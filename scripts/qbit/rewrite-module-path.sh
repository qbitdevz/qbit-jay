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
# caller does (see scripts/sync-upstream.sh).
set -euo pipefail

OLD='github.com/Gentleman-Programming/engram/v3'
NEW='github.com/qbitdevz/qbit-jay'

cd "$(git rev-parse --show-toplevel)"

count=0
while IFS= read -r -d '' file; do
  [ -f "$file" ] || continue
  sed -i "s#${OLD}#${NEW}#g" "$file"
  count=$((count + 1))
done < <(git grep -lIz -F "$OLD" -- . \
  ':!odd' ':!docs/qbit*' ':!UPSTREAM-DELTA.md' \
  ':!scripts/sync-upstream.sh' ':!scripts/qbit/rewrite-module-path.sh')

echo "rewrite-module-path: updated ${count} file(s)"
