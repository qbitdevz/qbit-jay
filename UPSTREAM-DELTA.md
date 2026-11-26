# Upstream delta

qbit-jay is a thin fork of [Engram](https://github.com/Gentleman-Programming/engram).
This file lists every point where qbit-jay modifies upstream code, why, and the
test that covers it. Update it in every pull request that touches upstream code.

- Upstream remote: `https://github.com/Gentleman-Programming/engram`
- Starting commit: `11521f4f3776db6d94dc17c11731bad35b71c7bc` (`v3.7.2-92-g11521f4`)
- Last synchronized upstream commit: `11521f4f3776db6d94dc17c11731bad35b71c7bc`

## Modified upstream files

None yet. The tree at the starting commit is unchanged.

| File | Change | Reason | Covering test |
| --- | --- | --- | --- |
| — | — | — | — |

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
| `scripts/qbit/baseline.sh` | E3 baseline: build size, test suite, store benchmarks |
| `scripts/qbit/e3latency/` | E3 baseline: black-box latency, startup and memory through MCP |
