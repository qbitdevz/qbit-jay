# qbit-jay — Fase 0: parity and measurement

Feature document for Phase 0 of `docs/qbit-jay — Especificación para el agente de código.md` (section 11).
Traceability: no business contract (`bcdd/contrato.md`) exists; T0.1–T0.3 are internal technical work
(environment, scaffolding, measurement) and fall under that exemption of the Working Agreement.
Current authorization covers T0.1–T0.3 only. Path of this document: `odd/tasks/qbit-jay-fase-0.md`.

## Specs

- **S1** (T0.1) "**T0.1 Entorno y repositorio.** Go 1.26.11 en WSL. Importar upstream con historial completo, crear `qbitdev/qbit-jay` (owner a confirmar), remoto `upstream`. Entregable: repositorio con `main` = commit de upstream de partida y CI de tests en verde."
- **S2** (6.1) "Importar el historial completo de upstream (no un squash) para que las fusiones futuras sean limpias. Remoto `upstream` = `https://github.com/Gentleman-Programming/engram`."
- **S3** (T0.2) "**T0.2 Andamiaje.** `odd/tasks/` del proyecto, `UPSTREAM-DELTA.md`, `docs/qbit/`, `docs/BRAND.md`."
- **S4** (6.4) "`UPSTREAM-DELTA.md` en la raíz: lista de cada punto donde qbit-jay modifica código de upstream, con el motivo y el test que lo cubre. Se actualiza en cada PR."
- **S5** (8.4) "Dejar documentado en `docs/BRAND.md` dónde se enchufan logo, paleta y tipografía."
- **S6** (T0.3) "**T0.3 Línea base de upstream.** E3 sobre upstream sin cambios."
- **S7** (10.2) "E3 Rendimiento | Latencia, arranque, memoria y tamaño | Benchmarks de upstream + nuevos | p50/p95, RSS, tamaño del binario y de la base" and "E3: reutilizar `make perf-baseline` y `make perf-check` y los benchmarks de `internal/store`."
- **S8** (10.4) "guardar los resultados en `docs/qbit/BASELINE.md`. Todas las mejoras se comparan contra esa línea base."
- **S9** (section 2, Tests gate) "Pasa la suite completa de upstream, incluida `go test -race`"
- **S10** (user, L1) "Antes de renombrar nada, muéstrame la línea base medida sobre Engram sin cambios."

Assumptions (not user specs):

- A1: starting commit is upstream `main` HEAD at import time, `11521f4f3776db6d94dc17c11731bad35b71c7bc` (same commit the spec analysed).
- A2: Go 1.26.11 is installed without sudo under `~/sdk/go1.26.11` in the WSL `Ubuntu` distro; no shell profile is modified.
- A3: the baseline is measured on a clone inside the WSL Linux filesystem, not on `/mnt/d`, so drvfs I/O does not distort numbers. The harness script makes this repeatable for the fork.

## Tasks

- [ ] T0.1 Environment and repository — S1, S2 — inline — commit: (upstream history; no own commit)
  - [x] T0.1a Go 1.26.11 in WSL
  - [x] T0.1b Import upstream with full history, remote `upstream`, `main` = starting commit
  - [ ] T0.1c Create GitHub repository and get test CI green — blocked: owner decision + publishing consent
- [x] T0.2 Scaffolding — S3, S4, S5 — inline — commit: on branch `chore/fase-0-scaffolding-baseline`
- [ ] T0.3 Upstream baseline (E3) — S6, S7, S8, S9, S10 — inline — commit: same branch. Partial: S9 open (race suite not run; 30 `plugin` tests fail without `jq`)
- [ ] T0.4–T0.10 — not authorized yet (see spec section 11)

## Log

- L1 (user, 2026-11-26): "Lee entero el md antes de tocar código. Empieza por la Fase 0, tareas T0.1 a T0.3, trabajando con ODD. Antes de renombrar nada, muéstrame la línea base medida sobre Engram sin cambios."
- L2 (user, 2026-11-26): "si necesitas bajar los repos de engram, gentle-ai y gentle-shell hazlo"
- L3 (evidence, 2026-11-26): host is Windows 11 + WSL2 `Ubuntu` 26.04 (user `qubit`), AMD Ryzen 7 7435HS, 16 threads. Windows has Go 1.27.4; WSL had no Go, gcc or make, and sudo needs a password. `git ls-remote` shows upstream `main` = `11521f4f`.
- L4 (evidence, 2026-11-26): GitHub login `qbitdev` exists as a user account; the `gh` sessions on this machine are `QuBiit0` (active) and `qbitdevz`. `qbitdev/qbit-jay` does not exist. Owner must be confirmed by the user before T0.1c and before the module path change in T0.4.
- L5 (evidence, 2026-11-26): Go 1.26.11 installed in WSL at `~/sdk/go1.26.11`, checksum verified against go.dev. Upstream imported: 1,428 commits, not shallow, `main` = `11521f4f` (`v3.7.2-92-g11521f4`), remote `upstream` set.
- L6 (evidence, 2026-11-26): baseline measured on unchanged upstream, results in `docs/qbit/BASELINE.md`. Release binary 22,962,338 bytes; `go test ./...` exits 1 (34 packages ok, 30 failing tests, all in `plugin`, WSL lacks `jq`); race suite not run (no C compiler, sudo needs a password). MCP medians of three runs: `mem_save` p95 104.8 ms, `mem_search` p95 43.1 ms, `mem_context` p95 65.5 ms, startup p95 25.0 ms.
- L7 (decision, 2026-11-26): run-to-run spread (up to 2x) exceeds the 5 % latency gates, so gates must be decided with alternating baseline/candidate runs in one session, not against the stored numbers. To review with the user, as section 2 asks.
- L8 (discovery, 2026-11-26): MCP write and read tools reject an explicit `project` the server has no context for (`unknown_project`), even when the working directory has that name; the harness omits `project` and relies on detection.
