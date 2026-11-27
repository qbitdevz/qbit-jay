# Performance baseline (E3)

Every change in qbit-jay is compared against this baseline. This first version
measures **upstream Engram unchanged**, before any rename.

- Measured: 2026-11-26
- Commit: `11521f4f3776db6d94dc17c11731bad35b71c7bc` (`v3.7.2-92-g11521f4`)
- Status: **complete for E3 on Linux**. Test suite and race suite green on
  2026-11-27 after installing `jq` and `build-essential` in WSL; see
  [Test suite](#test-suite). Items outside E3 on Linux are listed in
  [Not measured or not passing](#not-measured-or-not-passing).

## Environment

| Item | Value |
| --- | --- |
| Host | Windows 11 with WSL2, distro Ubuntu 26.04, kernel `6.18.58.1-microsoft-standard-WSL2` |
| CPU | AMD Ryzen 7 7435HS, 16 threads |
| Memory | 15.5 GiB visible to WSL |
| Go | `go1.26.11 linux/amd64`, `GOTOOLCHAIN=local` |
| File system | ext4 inside WSL (the clone and the data directory are not on a Windows mount) |
| C compiler | none on 2026-11-26; GCC 15.2.0 (`build-essential`) from 2026-11-27 |

Numbers from another machine are not comparable with these. Compare a change
only against a baseline measured on the same machine in the same session.

## Build

| Measure | Value |
| --- | --- |
| Release binary (`CGO_ENABLED=0`, `-trimpath -ldflags '-s -w'`) | 22,962,338 bytes (21.9 MiB) |
| Default-flags binary (`CGO_ENABLED=0`) | 32,573,032 bytes (31.1 MiB) |
| Cold build, empty caches | 50.2 s |

The release binary is the reference for the size gate (growth of at most 10 %:
25,258,571 bytes).

## Test suite

Reference run, 2026-11-27 (`jq` 1.8.1, GCC 15.2.0, `make` 4.4.1 installed):

| Measure | Value |
| --- | --- |
| `go test -count=1 ./...` | exit 0 after 97.7 s |
| Packages passing | 35 |
| Packages failing | 0 |
| Failing tests | 0 |
| Packages without test files | 5 |
| `go test -race -count=1 ./...` | exit 0 after 741.7 s, 35 packages passing, 0 failing |

First run, 2026-11-26, without `jq` or a C compiler: `go test ./...` exited 1
after 112.7 s with 30 failing tests, all in `plugin` (the Claude Code and Codex
hook scripts need `jq`). With `jq` installed all of them pass, including the
Windows hook tests whose cause was not confirmed then.

GCC needs the C headers of `libc6-dev`: with `gcc` alone, cgo is enabled and
`runtime/cgo` fails to compile (`stdint.h: No such file or directory`).
Install `build-essential`, not just `gcc`.

## Store benchmarks

The benchmark expression of the upstream performance ratchet
(`scripts/perf-ratchet.sh`), `-benchtime=1s -count 10`, summarized by
`benchstat`. The corpus is the one the benchmarks build themselves.

| Benchmark | Time per operation | Memory per operation | Allocations |
| --- | --- | --- | --- |
| `ScanProject_Page5000` | 939.7 ms ± 15 % | 79.09 MiB | 428.1 k |
| `Search_AllMode_Hit` | 48.63 ms ± 7 % | 45.50 KiB | 478 |
| `Search_AnyMode_Hit` | 39.61 ms ± 3 % | 45.41 KiB | 481 |
| `Search_NoHit` | 574.3 µs ± 8 % | 11.14 KiB | 120 |
| `Search_TypeFilter` | 36.07 ms ± 5 % | 46.05 KiB | 481 |
| `Search_Limit20` | 48.23 ms ± 8 % | 79.69 KiB | 829 |
| `SearchContext_AllMode_Hit` | 47.77 ms ± 4 % | 45.47 KiB | 478 |

The table is the 2026-11-26 run. The 2026-11-27 run, same commit and machine,
gave `ScanProject_Page5000` 925.1 ms ± 5 %, `Search_AllMode_Hit` 53.06 ms,
`Search_AnyMode_Hit` 42.32 ms, `Search_NoHit` 672.3 µs, `Search_TypeFilter`
41.55 ms, `Search_Limit20` 55.57 ms and `SearchContext_AllMode_Hit` 52.72 ms,
with identical memory per operation. Up to 17 % apart between days: another
reason to compare candidate and baseline in the same session.

## Latency, startup and memory through MCP

Measured from outside the binary by `scripts/qbit/e3latency`, which speaks MCP
over stdio to `engram mcp --tools=agent`, as an agent does. One long-lived
server receives the whole workload; startup is timed on fresh processes
against the seeded database.

Workload: 2,300 saved memories (4,716,010 bytes of content, mean 2,050
bytes, 79 % with a `topic_key`), then 300 searches of one to three words (a
quarter with `match_mode=any`), then 100 context requests. The corpus is
synthetic and deterministic (seed `20261126`); it imitates the size and shape
of the audited database, not its language or its real queries. No call failed
in any run.

Three complete runs. The table shows the median of the three and, in
parentheses, the lowest and highest.

| Operation | Samples per run | p50 (ms) | p95 (ms) |
| --- | --- | --- | --- |
| `mem_save` | 2,300 | 42.7 (42.5–43.7) | 104.8 (65.0–108.6) |
| `mem_search` | 300 | 31.0 (29.0–31.2) | 43.1 (38.9–45.3) |
| `mem_context` | 100 | 56.3 (51.1–107.6) | 65.5 (59.7–128.2) |
| MCP startup, spawn to `initialize` response | 30 | 19.8 (19.8–38.7) | 25.0 (24.6–50.7) |
| CLI startup, `engram version` | 30 | 5.0 (4.8–6.9) | 8.2 (6.5–12.7) |

| Measure | Value |
| --- | --- |
| Peak resident memory after `initialize` (seeded database) | 19.9 MB |
| Peak resident memory after the workload | 35.6 MB |
| Data directory after the workload | 27.7 MB (5.9 times the saved content) |
| `mem_context` response | 7,658 bytes, about 1,914 tokens at 4 bytes per token |
| Mean `mem_search` response | 10,495 bytes |
| Tools in the `agent` profile | 23 |
| `tools/list` response | 30,063 bytes |

On an empty database, MCP startup was 11.6 ms p50 and 15.3 ms p95, with
16.0 MB of peak resident memory (one run of 30 starts).

### Run-to-run variation

The spread between runs is far larger than the 5 % of the latency and startup
gates. The first run was about twice as slow as the other two for
`mem_context` and MCP startup, and the p95 of `mem_save` ranged from 65 ms to
109 ms. A single run of the candidate compared against the numbers above
cannot show a 5 % difference. To decide a gate, measure baseline and candidate
alternately in the same session, several runs each, and compare the medians.

## Not measured or not passing

| Item | State | What it needs |
| --- | --- | --- |
| `make perf-check`, `make perf-baseline` | Not run through `make` | Nothing; `make` is now installed. The same benchmarks were run directly |
| Corpora of 10,000 and 100,000 memories | Not measured | Belongs to the evaluators of task T0.8 |
| Real database of the user | Not measured | A copy at `QBIT_JAY_EVAL_DB`; never committed |
| Windows and macOS | Not measured | CI |
| E1 (context) and E2 (search) quality | Not measured | Evaluators of task T0.8 |

## How to reproduce

From the repository, inside WSL:

```sh
# Build size, test suite and store benchmarks on a clean clone
bash scripts/qbit/baseline.sh "$PWD" <commit> "$HOME/qbit-jay-baseline/<name>"

# Latency, startup and memory through MCP
go run ./scripts/qbit/e3latency -bin "$HOME/qbit-jay-baseline/<name>/out/engram-release"
```

For the renamed fork, pass `-env-prefix QBIT_JAY` to `e3latency`.
