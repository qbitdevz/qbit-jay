# Upstream delta

qbit-jay is a thin fork of [Engram](https://github.com/Gentleman-Programming/engram).
This file lists every point where qbit-jay modifies upstream code, why, and the
test that covers it. Update it in every pull request that touches upstream code.

- Upstream remote: `https://github.com/Gentleman-Programming/engram`
- Starting commit: `11521f4f3776db6d94dc17c11731bad35b71c7bc` (`v3.7.2-92-g11521f4`)
- Last synchronized upstream commit: `11521f4f3776db6d94dc17c11731bad35b71c7bc`

## Modified upstream files

| File | Change | Reason | Covering test |
| --- | --- | --- | --- |
| `go.mod` and every `.go`, `.templ`, `*_templ.go` file that imports the module | Module path `github.com/Gentleman-Programming/engram/v3` → `github.com/qbitdevz/qbit-jay` (spec S10) | The fork is its own module, so `go install` and imports resolve to qbit-jay. Go identifiers, `mem_*` and the DB schema are unchanged (S11). | `go build ./...` and the full `go test ./...` suite; `internal/cloud/dashboard` `TestTemplGeneratedFilesAreCheckedIn` (regenerated templ) |
| `.perf-baseline.txt`, `scripts/perf-ratchet.sh` | Package path normalizes to `github.com/qbitdevz/qbit-jay/internal/store` | Keeps benchstat tables comparable across the path change | `scripts/perf-ratchet_test.go` (`qbitStorePackage` helper) |
| `.github/workflows/ci.yml`, `.github/scripts/*` | Go package path only; `github.repository == 'Gentleman-Programming/engram'` guards stay until a later task | Contract tests pin the package path | `.github/scripts/*.test.mjs` (`issue-refresh-router-workflow.test.mjs` hash updated) |
| `release_config_test.go`, `tools/release-candidate/*`, `internal/version/check.go` + test, `internal/setup/setup.go`, `docs/INSTALLATION.md` | Go package path only (including the `go install` hint) | Fixtures and hints must name the real module | `release_config_test.go`, `tools/release-candidate` tests, `internal/version/check_test.go` |

## Files added by qbit-jay

Additions do not conflict with upstream merges, but they are listed so the
fork's footprint stays visible.

| Path | Purpose |
| --- | --- |
| `UPSTREAM-DELTA.md` | This file |
| `docs/BRAND.md` | Where logo, palette and typography plug in |
| `docs/qbit/` | Fork documentation, including the performance baseline |
| `docs/qbit-jay — Especificación para el agente de código.md` | Project specification |
| `odd/tasks/` | ODD feature documents |
| `scripts/sync-upstream.sh` | Upstream sync flow (S13): fetch, merge on `sync/upstream-YYYY-MM-DD`, rewrite commit, templ, tidy, tests |
| `scripts/qbit/rewrite-module-path.sh` | Idempotent module path rewrite used by the sync script (`scripts/sync-upstream.sh rewrite`) |
| `scripts/qbit/baseline.sh` | E3 baseline: build size, test suite, store benchmarks |
| `scripts/qbit/e3latency/` | E3 baseline: black-box latency, startup and memory through MCP |
