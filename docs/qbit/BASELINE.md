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

## Evaluators E1, E2 and E3 (task T0.8)

Run by `scripts/qbit/eval-baseline.sh` (the `internal/qbit/eval` package,
`-tags eval`), alternating fork and upstream, two rounds, in one session on the
same machine as above (Ryzen 7 7435HS, WSL2 Ubuntu 26.04, Go 1.26.11, ext4,
both trees built from WSL-native clones with `core.autocrlf=false`).

- Upstream: `11521f4` unchanged. It has no eval package, so a temporary clone
  received a copy of it with two mechanical edits: the store import path
  (`github.com/qbitdevz/qbit-jay/` to `github.com/Gentleman-Programming/engram/v3/`)
  and `store.DatabaseFile` replaced by the literal `"engram.db"`. Nothing else.
- Fork: branch `feat/t08-evaluators` at `878b57e` (evaluators and script; the
  later merge of `origin/main` changes no code the evaluators call).
- Corpus: synthetic, deterministic, seed `20261128`, sizes 1,000 and 10,000
  memories (project filter, 100 k not run, see below). No real data.
- Same-session caveat: the numbers are comparable only with each other; run
  to run the latency varied by up to about 10 %.

### E1: context (audit method)

Hit rate is the share of sessions whose context contains at least one memory
the session needed; recall is the share of needed memories included.

| Size | Budget | Sessions | Hit rate | Recall | Mean bytes |
| --- | --- | --- | --- | --- | --- |
| 1,000 | 4,096 | 83 | 44.6 % | 36.7 % | 4,084 |
| 1,000 | 16,384 | 83 | 74.7 % | 65.7 % | 6,956 |
| 1,000 | unbounded | 83 | 74.7 % | 65.7 % | 6,956 |
| 10,000 | 4,096 | 842 | 30.4 % | 22.3 % | 4,095 |
| 10,000 | 16,384 | 842 | 61.4 % | 49.3 % | 7,040 |
| 10,000 | unbounded | 842 | 61.4 % | 49.3 % | 7,040 |

Fork and upstream are identical in every cell, in both rounds: the fork has
not yet changed retrieval. These are the numbers a change to `mem_context`
must beat.

### E2: search

450 queries (150 memories, three each: `same` paraphrase, `kw` keywords,
`cross` other language) through the real `SearchPreviewsContext`, limit 10.
Fixture: `internal/qbit/eval/testdata/e2_queries.json`, written once by an LLM
(Claude, as the coding agent of this task) after reading the 150 synthetic
memories, not from templates; `testdata/README.md` explains how to
regenerate it. Fork and upstream give identical results.

| Size | Mode | recall@5 | recall@10 | MRR | Zero results |
| --- | --- | --- | --- | --- | --- |
| 1,000 | all | 49.3 % | 49.3 % | 0.487 | 48.7 % |
| 1,000 | any | 39.3 % | 40.2 % | 0.377 | 0.0 % |
| 10,000 | all | 49.3 % | 49.3 % | 0.449 | 44.0 % |
| 10,000 | any | 37.1 % | 37.3 % | 0.329 | 0.0 % |

By kind at 10,000, `all` mode: `kw` 100 % recall@5 (MRR 0.906), `same` 43.3 %,
`cross` 4.7 % (76 % zero results). Spanish query against English memory:
0.0 % recall and 72.5 % zero results. The `any` mode never returns nothing
but ranks worse. Cross-language retrieval is the weak spot.

### E3: performance

Store level (`internal/qbit/eval`), median of two rounds is not meaningful at
two samples, so both rounds are shown (fork / upstream, ms):

| Size | Load s | DB MiB | Search p50 | Search p95 | Context p50 | Context p95 |
| --- | --- | --- | --- | --- | --- | --- |
| 1,000, round 1 | 3.1 / 3.5 | 5.0 / 5.0 | 5.00 / 5.35 | 9.55 / 10.28 | 2.12 / 2.52 | 3.42 / 3.96 |
| 1,000, round 2 | 3.2 / 3.4 | 5.0 / 5.0 | 5.60 / 5.31 | 11.40 / 10.56 | 2.12 / 2.14 | 3.59 / 3.16 |
| 10,000, round 1 | 42.9 / 38.4 | 38.3 / 38.3 | 17.63 / 16.94 | 36.44 / 32.90 | 14.30 / 12.77 | 21.21 / 19.24 |
| 10,000, round 2 | 35.2 / 37.6 | 38.3 / 38.3 | 17.75 / 17.11 | 37.67 / 35.20 | 12.93 / 13.17 | 19.77 / 19.54 |

Through MCP (`scripts/qbit/e3latency`, one run each, alternating fork then
upstream, 10,000 memories, 20,349,822 bytes of content, 300 searches, 100
contexts; the first sequential run in the session, so one sample per tree):

| Measure | Fork (`878b57e`) | Upstream (`11521f4`) |
| --- | --- | --- |
| `mem_save` p50 / p95 (ms) | 91.7 / 188.1 | 96.3 / 179.2 |
| `mem_search` p50 / p95 (ms) | 90.8 / 133.0 | 84.8 / 124.1 |
| `mem_context` p50 / p95 (ms) | 198.0 / 230.7 | 185.9 / 204.8 |
| MCP startup p50 / p95 (ms) | 32.7 / 40.3 | 32.8 / 35.3 |
| CLI startup p50 / p95 (ms) | 4.06 / 4.74 | 4.05 / 5.09 |
| Peak RSS after `initialize` | 19.7 MiB (20,220 kB) | 19.4 MiB (19,820 kB) |
| Peak RSS after workload | 36.7 MiB (37,592 kB) | 36.1 MiB (36,932 kB) |
| Data directory | 113,881,121 B | 113,860,641 B |
| `mem_context` response | 7,711 B | 7,709 B |
| `tools/list` response | 30,115 B | 30,063 B |

The differences between fork and upstream are within the run-to-run
variation already documented; they show no regression, and they do not prove
the absence of a 5 % one (see Run-to-run variation).

## Not measured or not passing

| Item | State | What it needs |
| --- | --- | --- |
| `make perf-check`, `make perf-baseline` | Not run through `make` | Nothing; `make` is now installed. The same benchmarks were run directly |
| Corpus of 100,000 memories | Not measured | One tree takes about 5 minutes at 10,000 (E1+E2+E3 store level, mostly loading and E1), so 100,000 is an hour or more per tree; opt in with `QBIT_JAY_EVAL_SIZES=1000,10000,100000` |
| E1 on a real database | Not measured | A copy at `QBIT_JAY_EVAL_DB`; never committed |
| MCP latency at 10,000 repeated | One run per tree | More alternating runs to decide a 5 % gate |
| Real database of the user | Not measured | A copy at `QBIT_JAY_EVAL_DB`; never committed |
| Windows and macOS | Not measured | CI |

## How to reproduce

From the repository, inside WSL:

```sh
# Build size, test suite and store benchmarks on a clean clone
bash scripts/qbit/baseline.sh "$PWD" <commit> "$HOME/qbit-jay-baseline/<name>"

# Latency, startup and memory through MCP
go run ./scripts/qbit/e3latency -bin "$HOME/qbit-jay-baseline/<name>/out/engram-release"
```

For the renamed fork, pass `-env-prefix QBIT_JAY` to `e3latency`.

```sh
# E1, E2 and E3 on upstream and fork, alternating (from WSL, not under /mnt/c)
bash scripts/qbit/eval-baseline.sh <upstream-repo> 11521f4 <fork-checkout> "$HOME/qbit-jay-baseline/eval"
```
