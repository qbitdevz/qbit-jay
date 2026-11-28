#!/usr/bin/env bash
# Rewrites the Go module/package path from upstream Engram to qbit-jay.
#
#   github.com/Gentleman-Programming/engram/v3  ->  github.com/qbitdevz/qbit-jay
#
# Usage: scripts/qbit/rewrite-module-path.sh
#
# Touches tracked text files that contain the upstream path (.go, .templ,
# go.mod, release/CI config, scripts, docs/INSTALLATION.md). It skips the
# fork's own documentation (odd/, docs/qbit*, UPSTREAM-DELTA.md) and these
# two scripts, which name the old path on purpose. Idempotent: a second run
# changes nothing. It does not regenerate templ or run go mod tidy; the
# caller does (see scripts/sync-upstream.sh). Requires GNU sed (Linux, WSL,
# Git Bash); BSD/macOS sed reads `-i` differently.
set -euo pipefail

OLD='github.com/Gentleman-Programming/engram/v3'
NEW='github.com/qbitdevz/qbit-jay'

cd "$(git rev-parse --show-toplevel)"

# git grep exits 1 when nothing matches and above 1 on a real error. Capture
# its output and status first: inside a process substitution the status
# would be lost and a failure would look like "nothing to rewrite".
status=0
files="$(git -c core.quotePath=false grep -lI -F "$OLD" -- . \
  ':!odd' ':!docs/qbit*' ':!UPSTREAM-DELTA.md' \
  ':!scripts/sync-upstream.sh' ':!scripts/qbit/rewrite-module-path.sh')" || status=$?
if [ "$status" -gt 1 ]; then
  echo "rewrite-module-path: git grep failed with status ${status}" >&2
  exit "$status"
fi

count=0
while IFS= read -r file; do
  [ -n "$file" ] && [ -f "$file" ] || continue
  sed -i "s#${OLD}#${NEW}#g" "$file"
  count=$((count + 1))
done <<< "$files"

echo "rewrite-module-path: updated ${count} file(s)"
