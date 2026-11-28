#!/usr/bin/env bash
# E1/E2/E3 baseline: runs the evaluators of internal/qbit/eval on upstream
# Engram unchanged and on the fork, alternating, on the same machine.
#
# Usage: scripts/qbit/eval-baseline.sh <upstream-repo-or-url> <upstream-ref> <fork-checkout> <work-dir>
#
# Upstream does not contain the eval package, so a temporary clone of
# <upstream-ref> receives a copy of it with two mechanical adaptations: the
# import path of the store, and the database file name (the fork exports
# store.DatabaseFile, upstream hard-codes "engram.db"). Nothing else changes
# and the upstream clone is never pushed. The fork is measured from a
# git-free copy of <fork-checkout> so that the run also works from a worktree
# whose .git points at a Windows path.
#
# Results (text log and JSON per run) go to <work-dir>/out. Run it from the
# Linux filesystem (WSL: not under /mnt/c). Environment:
#   QBIT_JAY_GOROOT     Go toolchain root (default $HOME/sdk/go1.26.11)
#   QBIT_JAY_EVAL_SIZES corpus sizes (default 1000,10000; add 100000 to opt in)
#   QBIT_JAY_EVAL_ROUNDS alternating rounds, fork first then upstream (default 2)
set -uo pipefail
export LC_ALL=C

up_src="${1:?upstream repository path or URL}"
up_ref="${2:?upstream git ref}"
fork_src="${3:?fork checkout}"
work="${4:?work directory}"
go_root="${QBIT_JAY_GOROOT:-$HOME/sdk/go1.26.11}"
sizes="${QBIT_JAY_EVAL_SIZES:-1000,10000}"
rounds="${QBIT_JAY_EVAL_ROUNDS:-2}"

export PATH="${go_root}/bin:${PATH}"
export GOTOOLCHAIN=local
mkdir -p "${work}"
work="$(cd "${work}" && pwd)"
out="${work}/out"
mkdir -p "${out}" "${work}/tmp"
export TMPDIR="${work}/tmp" # the corpora are written here: keep it off tmpfs

up="${work}/upstream"
fork="${work}/fork"
if [ ! -d "${up}/.git" ]; then
	git -c safe.directory='*' clone --quiet --no-hardlinks "${up_src}" "${up}" || exit 1
fi
git -c safe.directory='*' -C "${up}" checkout --quiet --detach "${up_ref}" || exit 1

mkdir -p "${fork}"
(cd "${fork_src}" && tar --exclude=.git -cf - .) | tar -xf - -C "${fork}" || exit 1

# The eval package into the upstream clone, adapted.
mkdir -p "${up}/internal/qbit"
cp -r "${fork}/internal/qbit/eval" "${up}/internal/qbit/"
sed -i \
	-e 's#github.com/qbitdevz/qbit-jay/#github.com/Gentleman-Programming/engram/v3/#' \
	-e 's#store\.DatabaseFile#"engram.db"#' \
	"${up}"/internal/qbit/eval/*.go

up_commit="$(git -c safe.directory='*' -C "${up}" rev-parse --short HEAD)"
fork_commit="${QBIT_JAY_EVAL_COMMIT:-$(git -c safe.directory='*' -C "${fork_src}" rev-parse --short HEAD 2>/dev/null || echo unknown)}"

n=0
for _ in $(seq "${rounds}"); do
	for tree in fork upstream; do
		n=$((n + 1))
		dir="${fork}"
		commit="${fork_commit}"
		if [ "${tree}" = upstream ]; then
			dir="${up}"
			commit="${up_commit}"
		fi
		echo "run ${n}: ${tree} (${commit}), sizes ${sizes}"
		(cd "${dir}" && QBIT_JAY_EVAL_SIZES="${sizes}" QBIT_JAY_EVAL_LABEL="${tree}" \
			QBIT_JAY_EVAL_COMMIT="${commit}" QBIT_JAY_EVAL_JSON="${out}/${tree}-${n}.json" \
			go test -tags eval -count=1 -timeout 240m ./internal/qbit/eval/ -run TestEval -v) \
			>"${out}/${tree}-${n}.log" 2>&1
		echo "  exit $? -> ${out}/${tree}-${n}.log"
	done
done
