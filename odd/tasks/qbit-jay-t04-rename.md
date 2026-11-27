# qbit-jay — T0.4 rename to qbit-jay with full identity

Task T0.4 of Phase 0 in `docs/qbit-jay — Especificación para el agente de código.md` (sections 6.1, 6.2, 8.3, 8.4, 11).
Traceability: no business contract exists; T0.4 is internal technical work (fork identity), exempt under the Working Agreement.
Delivery: every work unit on its own branch and pull request to `main` (L2). Path: `odd/tasks/qbit-jay-t04-rename.md`.

## Specs

- **S1** (11, T0.4) "**T0.4 Renombrado** (sección 6.2): binario, comando, textos, carpeta de datos, variables con respaldo `ENGRAM_*`, nombre del servidor MCP en el setup, plugins y paquetes, retirada de los assets de Engram, README con atribución, tokens de tema provisionales (8.4). Los tests que comprueban textos de marca se adaptan sin perder su intención. Incluye el cambio del path del módulo Go a github.com/qbitdev/qbit-jay (Q2) y el script de sincronización de 6.1. Entregable: `qbit-jay` compila y pasa toda la suite."
- **S2** (6.2) "Binario y comando: `qbit-jay` (carpeta `cmd/qbit-jay`). Textos de ayuda, mensajes y errores."
- **S3** (6.2) "Nombre del servidor MCP que escribe el setup: `qbit-jay`."
- **S4** (6.2) "Carpeta de datos: `~/.qbit-jay` (Windows: `%USERPROFILE%\.qbit-jay`)."
- **S5** (6.2) "Variables de entorno: prefijo `QBIT_JAY_*`, **leyendo también `ENGRAM_*` como respaldo** por compatibilidad (la nueva gana si están las dos)."
- **S6** (6.2) "TUI, interfaz web, README, documentación, `assets/` (fuera el logo y las imágenes de Engram)." and "\"Engram Cloud\" → \"qbit-jay Cloud\"."
- **S7** (6.2) "Paquetes y plugins: el paquete Pi `gentle-engram`, el plugin de Claude Code y su entrada en el marketplace, el plugin de OpenCode, el de Codex y el de Obsidian (id del plugin incluido)."
- **S8** (6.2) "Comprobación de actualizaciones: `internal/version/check.go` consulta las releases de `Gentleman-Programming/engram` y sugiere el `go install` de upstream. Debe apuntar a las releases de qbitdev."
- **S9** (6.2) "Scripts de hooks y plugins que llaman al binario por su nombre: `plugin/claude-code/scripts/*.sh` (`engram hook ...`, `engram serve`), `plugin/codex/hooks`, el plugin de Pi (por defecto ejecuta `engram` del PATH salvo que se defina `ENGRAM_BIN`) y el de OpenCode. En qbit-jay llaman a `qbit-jay`. No se distribuye ningún ejecutable ni alias llamado `engram`: la política de marca protege también ese nombre como identificador de CLI."
- **S10** (6.2, Q2, corrected by L3) "**Path del módulo Go** (decisión Q2): de `github.com/Gentleman-Programming/engram/v3` a `github.com/qbitdev/qbit-jay`." Owner corrected by the user: "mi repo es qbitdevz no qbitdev ya que ese estaba ocuado" → `github.com/qbitdevz/qbit-jay`.
- **S11** (6.2) "**No se renombra** ...: Los nombres de herramientas `mem_*`, sus parámetros y la forma de sus respuestas. El esquema de la base de datos. Identificadores internos de Go (paquetes, tipos, funciones)."
- **S12** (6.2) "**Atribución:** mantener la licencia MIT con el copyright original y añadir el de qbitdev. En el README, una línea del tipo \"qbit-jay está basado en Engram, de Gentleman Programming\" con enlace, sin logos."
- **S13** (6.1) "**Script de sincronización** `scripts/sync-upstream.sh` ...: trae upstream, fusiona en la rama `sync/...`, reescribe el path `github.com/Gentleman-Programming/engram/v3` → `github.com/qbitdev/qbit-jay` en `.go`, `go.mod`, `.templ`, scripts y configuración de release, regenera templ, ejecuta `go mod tidy` y los tests. La reescritura va en un commit separado. Si un conflicto es solo de imports, se toma la versión de upstream y se vuelve a ejecutar la reescritura."
- **S14** (8.4) "Crear una única fuente de tokens de tema (`internal/qbit/theme`): colores para claro y oscuro, tipografía, espaciados. La TUI (Lip Gloss) y la web (variables CSS) leen de ella." Identity is now decided (PR #15, direction 03), so the tokens are the final ones from `design/system.md`, not provisional.
- **S15** (8.3) "Se conserva entera. Solo cambian la marca y el tema (8.4)."
- **S16** (user, L1) "el punto 4 esta bien.. full identidad, readme, tui, dashboard. etc." and L4 (earlier): "aprovechemos que hoy engram esta maduro y bien diseñado. nos apalanquemos y lo mejoremos aun mas".
- **S17** (6.4) "`UPSTREAM-DELTA.md` en la raíz: lista de cada punto donde qbit-jay modifica código de upstream, con el motivo y el test que lo cubre. Se actualiza en cada PR."

Assumptions (not user specs):

- A1: releases, Homebrew tap, Scoop, npm scope and the Docker image belong to T0.9; T0.4 only renames the names and URLs they read, without publishing anything.
- A2: migration from `~/.engram` (copy, `import --from-engram`, setup replacement) is T0.5; T0.4 does not touch existing Engram data.
- A3: work is split into chained PRs (module path, binary/env/data dir, plugins, visible identity) so each stays reviewable.

## Tasks

- [x] T1 Inventory of rename surfaces with counts and risk — S1-S11 — explore — commit: none (L5)
- [ ] T2 PR 1, module path and sync script: `scripts/sync-upstream.sh` (own commit), then the mechanical rewrite to `github.com/qbitdevz/qbit-jay` in `.go`, `go.mod`, `.templ` + regenerated templ, goreleaser, CI guards and their contract tests, perf scripts — S10, S13, S11, S17 — worker — Risk: item 3 — branch `chore/t04-module-path`
- [ ] T3 PR 2, binary, command, data dir, env helper: `cmd/qbit-jay`, help/usage/errors, `~/.qbit-jay` + `qbit-jay.db`, one env lookup helper `QBIT_JAY_*` winning over `ENGRAM_*` for every production variable, project folder `.qbit-jay/` reading `.engram/` when present — S2, S4, S5, S11, S17, D1, D2 — worker — Risk: items 1 and 3
- [ ] T4 PR 3, MCP setup and Claude Code plugin: server name `qbit-jay` for every agent, Codex TOML, marketplace and plugin.json, permissions `mcp__qbit-jay__*` — S3, S7, S9, D4 — worker — Risk: item 3
- [ ] T5 PR 4, Pi, OpenCode, Codex and Obsidian packages (Obsidian id `qbit-jay`), hooks calling `qbit-jay` — S7, S9, D3 — worker — Risk: item 3
- [ ] T6 PR 5, visible identity: `internal/qbit/theme` from `design/system.md`, TUI and dashboard read it, brand strings, "qbit-jay Cloud", update check against `qbitdevz/qbit-jay` — S6, S8, S14, S15, S16 — worker — Risk: item 6 (visual)
- [ ] T7 PR 6, docs and attribution: README with the qbit-jay banner and attribution line, LICENSE second copyright, docs, `UPSTREAM-DELTA.md`, removal of Engram assets — S6, S12, S17 — worker — Risk: none expected

Decisions (user, L6): D1 project folder `.qbit-jay/`, reading `.engram/` if it exists; D2 database file `qbit-jay.db`; D3 Obsidian plugin id `qbit-jay`; D4 setup writes `mcp__qbit-jay__*` permissions, removing Engram's is T0.5.

## Log

- L1 (user, 2026-11-27): "mergalo a main. pushea y abri un pr aparte.. todo lo que hagas siempre con pr asi va quedando un historial de todo. el punto 4 esta bien.. full identidad, readme, tui, dashboard. etc."
- L2 (decision, 2026-11-27): every change goes through a branch and PR (L1). PR #14 merged to `main` as `fbcc495`; identity PR #15 open.
- L3 (user, 2026-11-27): "mi repo es qbitdevz no qbitdev ya que ese estaba ocuado"
- L5 (evidence, 2026-11-27): T1 inventory: module path in 300+ lines (four generated `*_templ.go`, templ v0.3.1001 pinned as a go tool); `cmd/engram` ~150 files; data dir at `cmd/engram/main.go:800,807`, `engram.db` at `main.go:4354,4361`; ~30 production `ENGRAM_*` variables read by ~70 scattered `os.Getenv` calls, no helper; MCP server name in `internal/setup/agents.go`, `codex_toml.go:244-257`, `setup.go:103-104`, `claude_native_bundle_windows.go`; CI guards pin `Gentleman-Programming/engram` (`ci.yml:122,680`, `.github/scripts/ci-technical-contracts.test.mjs`); keep stored or protocol identifiers (`engram-cloud` health body, sync provenance strings, `engram-managed-claude-v1` hash domain, `mem_*`, DB schema). Proposed six chained PRs (T2-T7).
- L6 (user, 2026-11-27): ".qbit-jay/ y lee .engram/ si existe"; "qbit-jay.db"; "id nuevo qbit-jay"; "Setup escribe los permisos nuevos".
- L4 (user, 2026-11-27, earlier): "algo que no se si te dije es que aprovechemos que hoy engram esta maduro y bien diseñado. nos apalanquemos y lo mejoremos aun mas"
