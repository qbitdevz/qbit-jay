# Parity checklist (Phase 0, T0.10)

Spec section 8.1: "Cada punto se da por cumplido cuando sus tests de upstream pasan con el binario renombrado y una prueba manual confirma el comportamiento."

Base: upstream Engram `11521f4`. Evidence comes from the green CI suites on `main` (Linux unit, race and e2e, macOS ARM64 native, Windows adapters and full suite, PostgreSQL integration, plugin tests), the feature documents under `odd/tasks/`, and manual checks run in temporary environments (the user chose temporary environments only while away; real-machine checks are listed as pending).

| # | Surface | Upstream tests on the renamed binary | Manual check | State |
| --- | --- | --- | --- | --- |
| 1 | CLI, every subcommand, named `qbit-jay` | `cmd/qbit-jay` suite green in CI | `--help`, `version`, `stats`, `init`, `import`, `doctor`, `setup` run in temp HOME (T0.4 L9, T0.5 L10) | Met |
| 2 | MCP stdio server with tool profiles (`--tools=agent` and others) | `internal/mcp` suite and the tool contract golden | gentle-ai `initialize` probe against `qbit-jay mcp --tools=agent` (T0.6 L6) | Met |
| 3 | `mem_*` tools, same parameters and responses | `tool-contract-v1.json` unchanged; agent contract regenerated only for description text (T0.4 L15) | MCP save, search, get, context over stdio (T0.7 L8) | Met |
| 4 | HTTP API, daemon, socket and tokens | `internal/server`, `cmd/qbit-jay` daemon tests | `/health` with `product:"qbit-jay"`, serve and doctor port checks (T0.5 L10) | Met |
| 5 | TUI, every function | `internal/tui` suite | rendered screens reviewed (T0.4 L14) | Met; real-terminal check pending |
| 6 | Hooks and plugins: Claude Code, Codex, OpenCode, Pi, Obsidian | `plugin` Go tests, Pi, OpenCode and Obsidian node tests, Windows adapter tests | Pi local package listed by `pi list` (T0.9b L21); others through tests only | Met by tests; end-to-end per agent pending |
| 7 | `setup` for every target, safe replacement of an existing Engram entry | `internal/setup` suite | cursor replace, backup, restore, edit guard, gentle-ai mode, `setup pi` (T0.5 L10, T0.9b L21) | Met for JSON agents and Pi; Claude Code, Codex and OpenCode end to end pending |
| 8 | Git Sync, Cloud (server, dashboard, autosync, audit log), Obsidian export | sync, cloud, cloudserver and PostgreSQL integration suites | dashboard rendered at 1440 and 390 px (T0.4 L14) | Met by tests; live Cloud deployment not exercised |
| 9 | `doctor`, project aliases and merges, `consolidate`, `conflicts`, `prune`, `rescue-ownership` | `cmd/qbit-jay` and `internal/diagnostic` suites | `doctor` in temp HOME | Met by tests |
| 10 | Protocol instructions, `memory-protocol` skills, agent contract | memory-protocol ownership tests, agent contract check | — | Met |
| 11 | Import from `~/.engram` and `ENGRAM_*` fallback | `internal/qbit/engramimport`, `internal/qbit/env` guard and precedence tests | import keeps the source hash, works with the source open, `--force` keeps the db on a corrupt source (T0.5 L10) | Met |
| 12 | gentle-ai compatibility (`GENTLE_AI_ENGRAM_BIN`; the spec said `ENGRAM_BIN`) | `internal/qbit/compat` (12 tests) | real gentle-ai 5.1.0 in a temp HOME (T0.6 L6) | Met with known gaps: G3 (mirrors over 50,000 bytes are truncated), G7 duplicate entries in gentle-ai's default setup mode |
| 13 | Windows, macOS and Linux (including WSL) | CI on ubuntu, macOS ARM64 and windows-2025; baseline in WSL | install scripts tested on Linux and Windows PowerShell 5.1 and 7 (T0.9a L9) | Met in CI; real-machine run pending |

## Known gaps carried into Phase 1

- G3 / H2: ODD mirrors over 50,000 bytes are stored truncated (`... [truncated]`). A test pins today's behavior.
- E2: Spanish queries against English memories get 0 % recall (`docs/qbit/BASELINE.md`).
- Real-machine checks: the user's WSL with gentle-ai and `eco-update`, Claude Code, Codex and OpenCode end to end, a real terminal for the TUI.
- Follow-ups recorded in `odd/tasks/qbit-jay-fase-0-rest.md` (telemetry degradation warnings, setup manifest atomicity, restore leaving the Pi plugin directory).
