#!/usr/bin/env bash
# Syncs qbit-jay with upstream Engram (spec 6.1).
#
# Usage:
#   scripts/sync-upstream.sh            full sync flow (below)
#   scripts/sync-upstream.sh rewrite    only rewrite the Go module path in place
#
# Flow:
#   1. refuse to run on a dirty working tree
#   2. fetch the `upstream` remote
#   3. create sync/upstream-YYYY-MM-DD from main
#   4. merge upstream/main (merge, never rebase)
#   5. rewrite github.com/Gentleman-Programming/engram/v3 ->
#      github.com/qbitdevz/qbit-jay, in its own commit
#   6. regenerate templ (as `make templ` does), go mod tidy (commit if they change anything)
#   7. run the tests
#
# If the merge conflicts only on imports, take upstream's version of the file
# (git checkout --theirs <file>; git add <file>), finish the merge, then run
# `scripts/sync-upstream.sh rewrite` and commit the result.
#
# Environment: SYNC_BASE_BRANCH (default main), SYNC_UPSTREAM_REMOTE (default
# upstream), SYNC_BRANCH (default sync/upstream-<today>), SYNC_SKIP_TESTS=1.
set -euo pipefail

BASE="${SYNC_BASE_BRANCH:-main}"
REMOTE="${SYNC_UPSTREAM_REMOTE:-upstream}"
BRANCH="${SYNC_BRANCH:-sync/upstream-$(date +%Y-%m-%d)}"

cd "$(git rev-parse --show-toplevel)"

if [ "${1:-}" = "rewrite" ] && [ "$#" -eq 1 ]; then
  exec bash scripts/qbit/rewrite-module-path.sh
fi
if [ "$#" -gt 0 ]; then
  echo "usage: scripts/sync-upstream.sh [rewrite]" >&2
  exit 2
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "sync-upstream: working tree is not clean; commit or stash first" >&2
  exit 1
fi
if git show-ref --verify --quiet "refs/heads/${BRANCH}"; then
  echo "sync-upstream: branch ${BRANCH} already exists" >&2
  exit 1
fi

git fetch "$REMOTE"
git switch -c "$BRANCH" "$BASE"

if ! git merge --no-ff -m "chore(sync): merge ${REMOTE}/main" "${REMOTE}/main"; then
  echo "sync-upstream: merge conflicts. Resolve them (import-only conflicts: take upstream's file)," >&2
  echo "finish the merge, then run: scripts/sync-upstream.sh rewrite" >&2
  exit 1
fi

bash scripts/qbit/rewrite-module-path.sh
if [ -n "$(git status --porcelain)" ]; then
  git add -A
  git commit -m "chore(sync): rewrite Go module path to qbit-jay"
fi

go tool templ generate -path ./internal/cloud/dashboard
go mod tidy
if [ -n "$(git status --porcelain)" ]; then
  git add -A
  git commit -m "chore(sync): regenerate templ and tidy modules"
fi

if [ "${SYNC_SKIP_TESTS:-0}" != "1" ]; then
  go test -count=1 ./...
fi
echo "sync-upstream: done on ${BRANCH}; review, then open a pull request to ${BASE}"
