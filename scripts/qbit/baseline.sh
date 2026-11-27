#!/usr/bin/env bash
# E3 baseline, stage 1: build size, full test suite and the store benchmarks of
# the performance ratchet, measured on a clean clone inside the Linux
# filesystem so that Windows-mounted I/O does not distort the numbers.
#
# Usage: scripts/qbit/baseline.sh <source-repo-or-url> <git-ref> <work-dir>
#
# Results are written to <work-dir>/out. The clone lives in <work-dir>/src and
# is recreated on every run.
set -uo pipefail
export LC_ALL=C

src="${1:?source repository path or URL}"
ref="${2:?git ref to measure}"
work="${3:?work directory}"
go_root="${QBIT_JAY_GOROOT:-$HOME/sdk/go1.26.11}"
count="${QBIT_JAY_BENCH_COUNT:-10}"
benches="${QBIT_JAY_BENCHES:-^(Benchmark(Search|SearchContext)_|BenchmarkScanProject_Page5000$)}"
benchstat_version="v0.0.0-20250909190841-7e13e04d9366"

export PATH="${go_root}/bin:${PATH}"
export GOTOOLCHAIN=local

out="${work}/out"
clone="${work}/src"
rm -rf "${clone}" "${out}"
mkdir -p "${out}"

git -c safe.directory='*' clone --quiet --no-hardlinks "${src}" "${clone}" || exit 1
cd "${clone}" || exit 1
git checkout --quiet --detach "${ref}" || exit 1

cmd_dir="$(find cmd -mindepth 1 -maxdepth 1 -type d | sort | head -n 1)"
bin_name="$(basename "${cmd_dir}")"

{
	printf 'date_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	printf 'commit=%s\n' "$(git rev-parse HEAD)"
	printf 'describe=%s\n' "$(git describe --tags --always)"
	printf 'go=%s\n' "$(go version)"
	printf 'kernel=%s\n' "$(uname -r)"
	printf 'cpu=%s\n' "$(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2- | sed 's/^ //')"
	printf 'threads=%s\n' "$(nproc)"
	printf 'mem_kb=%s\n' "$(awk '/MemTotal/ {print $2}' /proc/meminfo)"
	printf 'fs=%s\n' "$(stat -f -c %T "${clone}")"
	printf 'cc=%s\n' "$(command -v gcc || command -v cc || echo none)"
	printf 'command=%s\n' "${cmd_dir}"
} >"${out}/env.txt"

# Build: default flags and the release flags (stripped, CGO-free).
start=$(date +%s.%N)
CGO_ENABLED=0 go build -o "${out}/${bin_name}-default" "./${cmd_dir}" 2>"${out}/build.log"
build_rc=$?
end=$(date +%s.%N)
CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -o "${out}/${bin_name}-release" "./${cmd_dir}" 2>>"${out}/build.log"
release_rc=$?
{
	printf 'build_rc=%s\n' "${build_rc}"
	printf 'release_build_rc=%s\n' "${release_rc}"
	printf 'cold_build_seconds=%s\n' "$(awk -v a="${start}" -v b="${end}" 'BEGIN { printf "%.1f", b - a }')"
	printf 'size_default_bytes=%s\n' "$(stat -c %s "${out}/${bin_name}-default" 2>/dev/null || echo NA)"
	printf 'size_release_bytes=%s\n' "$(stat -c %s "${out}/${bin_name}-release" 2>/dev/null || echo NA)"
} >"${out}/build.txt"

# Full test suite, as upstream CI runs it (`go test ./...`).
start=$(date +%s.%N)
go test -count=1 -timeout=30m ./... >"${out}/test.log" 2>&1
test_rc=$?
end=$(date +%s.%N)
{
	printf 'test_rc=%s\n' "${test_rc}"
	printf 'test_seconds=%s\n' "$(awk -v a="${start}" -v b="${end}" 'BEGIN { printf "%.1f", b - a }')"
	printf 'packages_ok=%s\n' "$(grep -c '^ok ' "${out}/test.log")"
	printf 'packages_failed=%s\n' "$(grep -c '^FAIL	' "${out}/test.log")"
	printf 'packages_no_tests=%s\n' "$(grep -c 'no test files' "${out}/test.log")"
	printf 'tests_failed=%s\n' "$(grep -c '^--- FAIL' "${out}/test.log")"
} >"${out}/test.txt"

# Race suite needs CGO and a C compiler.
if command -v gcc >/dev/null 2>&1 || command -v cc >/dev/null 2>&1; then
	start=$(date +%s.%N)
	go test -race -count=1 -timeout=40m ./... >"${out}/race.log" 2>&1
	race_rc=$?
	end=$(date +%s.%N)
	{
		printf 'race_rc=%s\n' "${race_rc}"
		printf 'race_seconds=%s\n' "$(awk -v a="${start}" -v b="${end}" 'BEGIN { printf "%.1f", b - a }')"
		printf 'race_packages_failed=%s\n' "$(grep -c '^FAIL	' "${out}/race.log")"
	} >"${out}/race.txt"
else
	printf 'race_rc=not-run\nreason=no C compiler available\n' >"${out}/race.txt"
fi

# Store benchmarks: the same expression the upstream performance ratchet uses.
go test -run '^$' -bench "${benches}" -benchmem -benchtime=1s -count "${count}" ./internal/store >"${out}/bench.txt" 2>"${out}/bench.err"
printf 'bench_rc=%s\n' "$?" >"${out}/bench.rc"
go run "golang.org/x/perf/cmd/benchstat@${benchstat_version}" "${out}/bench.txt" >"${out}/benchstat.txt" 2>"${out}/benchstat.err"

cat "${out}/env.txt" "${out}/build.txt" "${out}/test.txt" "${out}/race.txt" "${out}/bench.rc"
