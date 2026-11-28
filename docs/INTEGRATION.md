[← Back to README](../README.md)

# Integration contract for configurators

Everything a tool that installs or configures qbit-jay (qbit-ai, gentle-ai) needs to know: the command line it may rely on, the output formats it may parse, where data lives, which variables it can set, which plugins exist per agent, and the ODD mirror convention. Each claim below is checked by [`internal/qbit/compat`](../internal/qbit/compat/gentleai_test.go) against the built binary, or marked as not verified.

Reference for gentle-ai: <https://github.com/Gentleman-Programming/gentle-ai> at commit `d64b1c1baa22cf34d3a8da29050d57103a07cf6a` (v5.1.0 line, checked 2026-11-28). File and line references below point to that commit.

## Quick path

| You are | Do this |
| --- | --- |
| **qbit-ai** (native) | Install `qbit-jay` by one of its channels, register the MCP server as `qbit-jay` with `qbit-jay mcp --tools=agent`, then run `qbit-jay setup <agent>` for the plugin. No `ENGRAM_*` names. |
| **gentle-ai** (compat) | Export `GENTLE_AI_ENGRAM_BIN=/absolute/path/to/qbit-jay` **permanently** (shell profile), then run `gentle-ai install` and `gentle-ai sync` as usual. Read [gentle-ai compat mode](#gentle-ai-compat-mode) first: two behaviors need a decision. |

The spec and older notes call the gentle-ai variable `ENGRAM_BIN`. In gentle-ai it is **`GENTLE_AI_ENGRAM_BIN`** (`internal/system/engram_bin.go:8`). `ENGRAM_BIN` is a different variable, read by the Pi package (see [Environment](#environment)).

## Binary interface

The command line gentle-ai runs against the Engram binary, and what qbit-jay does with it. "Test" names a test in `internal/qbit/compat`.

| Invocation | What the caller expects | gentle-ai | qbit-jay | Test |
| --- | --- | --- | --- | --- |
| `<bin> version` | Exit 0, non-empty trimmed stdout. Update checks take the first `\d+\.\d+(\.\d+)?` (or `dev`). | `components/engram/verify.go:81-120`, `cli/doctor_tool_probe.go:23-26`, `update/registry.go:45`, `update/detect.go:30-34,135-152`, `cli/run.go:691-703` | Prints `qbit-jay 0.1.0` (`qbit-jay dev` unstamped). Parsed as `0.1.0`. | `TestG1VersionOutputParses`, `TestG1DevVersionIsRecognized` |
| `<bin> setup --help` (5 s, stdin closed, stdout only) | Stdout contains `--protocol`. | `components/engram/setup.go:42-75`, `cli/run.go:3506-3513` | Yes, exit 0, no files written. | `TestG1SetupHelpAdvertisesProtocolFlag` |
| `<bin> setup <slug> [--protocol=full\|slim]` | Exit 0. A failure is ignored unless `GENTLE_AI_ENGRAM_SETUP_STRICT` is true. Slugs: `opencode`, `kilocode`, `claude-code`, `gemini-cli`, `codex`, `windsurf`. | `components/engram/setup.go:77-161`, `cli/run.go:3479-3547` | Accepted. `claude-code` and `codex` exit 1 when their CLI is missing (ignored in the default mode). See [duplicate entries](#gentle-ai-compat-mode). | manual (simulated run) |
| `<bin> instance-id` with `ENGRAM_NO_UPDATE_CHECK=1` and `ENGRAM_DATA_DIR=<tmp>` | Exit 0. Only stderr containing "unknown command" counts as unsupported. Writes only under the data dir. | `components/engram/verify.go:30-72` | Yes. `QBIT_JAY_DATA_DIR` would win if set. | `TestG1InstanceIDHonorsLegacyDataDir`, `TestG1InstanceIDPrefersQbitJayDataDir` |
| `<bin> mcp --tools=agent` (MCP server written into each agent's config) | Persisted as `command` + `args: ["mcp","--tools=agent"]` in JSON (`mcpServers`, `servers`, `mcp`), Codex TOML and Hermes YAML. | `components/engram/inject.go:96-162,1466-1530`, `filemerge/toml.go:25`, `filemerge/yaml.go:259-265` | Yes. | `TestG1MCPHandshakeSatisfiesDoctorProbe` |
| Stdio health probe: spawn the persisted command, send one `initialize`, expect exactly one valid response frame, close stdin, reject any later stdout frame. Deadline 15 s, or the MCP `timeout` of the agent. | `result.protocolVersion` (string), `result.capabilities` (object), `result.serverInfo.{name,version}` (non-empty strings), `id` 1, no `method`. | `components/engram/healthprobe.go:145-375`, `cli/doctor.go:717-800` | Yes. Verified with the copied validator and with the real `gentle-ai doctor` (`engram:reachable` ok). | `TestG1MCPHandshakeSatisfiesDoctorProbe` |
| HTTP health probe `GET $ENGRAM_BASE_URL/health` (only when `ENGRAM_BASE_URL` is set) | Any 2xx. | `cli/doctor.go:70,811-835` | `200 {"status":"ok","service":"engram",...}`. While starting it answers `503 {"status":"starting"}`. The `service` value `engram` is a kept protocol identifier (the Pi package also checks it). | `TestG1HTTPHealthEndpoint` |
| `engram` looked up on `PATH` | `VerifyInstalled`, doctor `tool:engram`, update checks and `upgrade` look for a binary literally named `engram`. | `verify.go:21`, `cli/doctor.go:39`, `doctor_tool_probe.go:24`, `update/detect.go:48` | No `engram` binary or alias is shipped (brand). Only `GENTLE_AI_ENGRAM_BIN` is honored where gentle-ai supports it. See findings. | simulated run |
| Tool names `mem_*` in the injected protocol and in Claude agent allowlists (`mcp__engram__mem_*`, `mcp__plugin_engram_engram__*`) | The 15 tools the protocol names exist in the `agent` profile. | `internal/assets/engram/protocol.md`, `legacyassets/sdd_agents.go:32`, `reviewassets/install.go:73` | Yes. | `TestG6AgentProfileServesToolsNamedByGentleAI` |

## MCP command and profiles

```text
qbit-jay mcp [--tools=PROFILE] [--project NAME]
```

| Profile | Tools | Use |
| --- | --- | --- |
| `agent` | 23 | What agents use. This is what configurators write. |
| `admin` | 6 | `mem_delete`, `mem_stats`, `mem_timeline`, `mem_merge_projects` (not in `agent`), plus `mem_judge_many` and `mem_session_finalize` (also in `agent`). |
| `all` (default) | 27 | Everything. |

Profiles combine (`--tools=agent,admin`) and individual tool names work (`--tools=mem_save,mem_search`). Stdout carries only JSON-RPC frames; logs go to stderr. A project is detected from the working directory unless `--project` or `QBIT_JAY_PROJECT` is set.

Tool names stay `mem_*`. The MCP **server name** is a configurator decision: qbit-ai registers `qbit-jay` (tools appear as `mcp__qbit-jay__mem_save`); gentle-ai registers `engram` (`mcp__engram__mem_save`), and that name is user configuration, not qbit-jay branding.

## Version and health

- `qbit-jay version` (also `--version`, `-v`) prints `qbit-jay <version>`, e.g. `qbit-jay 0.1.0`. Releases are stamped by GoReleaser; a source build prints `qbit-jay dev`.
- gentle-ai's slim protocol gate accepts only `^(engram )?v?X.Y.Z$` and `X.Y.Z >= 1.4.0` (`protocol.go:19-32,84-116`). qbit-jay fails both (different prefix, and 0.x is below the floor), so gentle-ai keeps the **full** protocol, its safe default. Nothing breaks; the cost is a longer Claude Code protocol section. Changing the version line is a conscious decision: `TestG1VersionOutputParses` pins it.
- Spec S16 (state the Engram version we derive from): keep the first line as it is. gentle-ai's update check takes the first version-like match, so a second line would not change what it parses, but that is untested.

## Data paths

| Item | Path |
| --- | --- |
| Data directory | `~/.qbit-jay` (`QBIT_JAY_DATA_DIR`, then `ENGRAM_DATA_DIR`) |
| Database | `<data dir>/qbit-jay.db` |
| Per-repository folder | `.qbit-jay/`; a repository that only has `.engram/` keeps using it; `.qbit-jay/` wins when both exist |
| Protocol mode per agent | `~/.qbit-jay/protocol-mode.json` (written by `setup --protocol`) |
| HTTP server | port 7437 (`QBIT_JAY_PORT`), same as Engram: a running Engram on that port is a conflict (Phase 0 T0.5) |

`~/.engram` is never read or modified by qbit-jay today; import from it is Phase 0 T0.5.

## Environment

Every variable is read as `QBIT_JAY_<NAME>` first, then `ENGRAM_<NAME>`; the new name wins. An empty `QBIT_JAY_` value falls back to a set `ENGRAM_` value. The full list is in `qbit-jay help`.

Variables outside that scheme, set or read by configurators:

| Variable | Read by | Meaning |
| --- | --- | --- |
| `GENTLE_AI_ENGRAM_BIN` | gentle-ai | Absolute path of the binary gentle-ai uses as Engram. Must exist and run `version`. `system/engram_bin.go:8`, `components/engram/caller_binary.go:16-48` |
| `GENTLE_AI_ENGRAM_SETUP_MODE` | gentle-ai | `off`, `opencode` or `supported` (default). `off` skips `<bin> setup`. `setup.go:78-106` |
| `GENTLE_AI_ENGRAM_SETUP_STRICT` | gentle-ai | `1`, `true`, `yes`, `on`: a failed `<bin> setup` fails the install |
| `ENGRAM_BIN` | Pi package (`gentle-engram`, `qbit-jay-pi`), gentle-ai Pi identity check | Binary the Pi package starts. qbit-jay's Pi plugin resolves `QBIT_JAY_BIN` > `ENGRAM_BIN` > `qbit-jay`. `verify.go:36`, `shellinstaller/user_engram_shared.go:39,175` |
| `ENGRAM_BASE_URL` | gentle-ai doctor | Switches the doctor to the HTTP probe |
| `ENGRAM_NO_UPDATE_CHECK=1` | set by gentle-ai on child processes | Honored. `verify.go:56`, `shellinstaller/user_engram_shared_unix.go:93` |

## Plugins per agent

`qbit-jay setup <agent>` installs the qbit-jay MCP registration and the plugin or hooks for that agent. They call `qbit-jay`, never `engram`. Details per agent: [Agent Setup](AGENT-SETUP.md).

| Agent | Command | What is installed |
| --- | --- | --- |
| Claude Code | `qbit-jay setup claude-code` | Plugin `qbit-jay@qbit-jay` (marketplace `qbitdevz/qbit-jay`) through the `claude` CLI; hooks call `qbit-jay` |
| Codex | `qbit-jay setup codex` | `[mcp_servers.qbit-jay]`, instruction and compact files, plugin `qbit-jay@qbit-jay` through the `codex` CLI |
| OpenCode | `qbit-jay setup opencode` | `~/.config/opencode/plugins/qbit-jay.ts` and the MCP entry |
| Pi | `qbit-jay setup pi` | Pi package `npm:gentle-engram` (renamed to `qbit-jay-pi` with the release work) |
| Gemini CLI, Windsurf, Kilo Code | `qbit-jay setup <slug>` | MCP entry, protocol block and, where the agent has one, a system file |
| Cursor, VS Code Copilot, Qwen, Kiro, Kimi, CommandCode, Antigravity | `qbit-jay setup <slug>` | MCP entry and rules (see Agent Setup) |
| Any MCP client | n/a | `qbit-jay mcp --tools=agent` |

`setup` writes only its own `qbit-jay` entries and never reads, rewrites or removes an `engram` entry (replacement with backup and `--restore` is Phase 0 T0.5).

## ODD mirror convention

For substantial work, ODD keeps `odd/tasks/{feature}.md` and mirrors the **whole** document in memory (contract: gentle-ai `internal/assets/skills/_shared/engram-convention.md`).

| Rule | qbit-jay behavior | Verified |
| --- | --- | --- |
| `mem_save` with the same `topic_key`, project and scope is an upsert | Same observation id; content replaced | `TestG2UpsertUpdateAndReadBack` |
| `mem_update` by id | Replaces content; needs `expected_project` | same |
| `mem_get_observation` returns what was saved | Returns the content in the `result` text, between a header line and a `Session:` trailer. Surrounding whitespace is trimmed on save (a trailing newline is not kept). | same |
| `capture_prompt: false` on mirrors | Accepted; a non-boolean value is rejected | `TestG2CapturePromptParameter` |
| `topic_key` `odd/{feature}/tasks` is found by topic and with `/` in the query | `mem_search` finds it for the full key, the feature name, `odd/` and `feature/tasks` | `TestG5SearchFindsMirrorsByTopic` |
| **G3: no silent truncation** | **Not met.** Content over 50,000 bytes is cut at 50,000 and ends with `... [truncated]`; `mem_save` reports it in `truncation` (`original_bytes`, `limit_bytes`, `truncated`) and `mem_update` applies the same limit. gentle-ai does not read that field, so a mirror above 50,000 bytes loses its tail on reread. | `TestG3LargeMirrorIsTruncatedAtFiftyThousandBytesTodayH2` pins it |

G3 is a finding for Phase 1 H2 (remove the cap or chunk mirrors). Until then a configurator should read `truncation.truncated` and keep mirrors under 50,000 bytes. When H2 lands, flip the G3 test to assert an exact round trip.

## gentle-ai compat mode

Facts from running gentle-ai 5.1.0 (built from the commit above) against the built qbit-jay in a temporary `HOME`. Nothing touched the real home.

**What works.** With `GENTLE_AI_ENGRAM_BIN` set, `gentle-ai install --component engram` skips brew, download and `go install`, runs `<bin> version` before writing, then writes an `engram` MCP entry whose command is the qbit-jay path in Gemini, Windsurf, Cursor, Claude (`~/.claude.json`) and Codex. `verify:engram:binary` and `verify:engram:version` pass. `gentle-ai doctor` reports `engram:reachable` ok through the real stdio handshake.

**What needs a decision.**

1. **Duplicate entries (G7).** In the default setup mode gentle-ai runs `<bin> setup <slug>`, and qbit-jay writes its own `qbit-jay` entry next to gentle-ai's `engram` entry. Observed: Gemini, Windsurf and Codex end with both entries, same command, so an agent sees every `mem_*` tool twice. Options: set `GENTLE_AI_ENGRAM_SETUP_MODE=off` (single `engram` entry; verified), at the cost of qbit-jay's plugins and hooks not being installed by gentle-ai (run `qbit-jay setup <agent>` yourself for those, which re-adds the `qbit-jay` entry); or accept the duplicate. **Do not remove the `engram` entry:** gentle-ai's Claude agents (`jd-judge-*`, `jd-fix-agent`) allow only `mcp__engram__mem_*` tools, so they stop working without it. T0.5 "replace Engram entries" must therefore not run in gentle-ai mode, or must keep `engram` as an alias of the qbit-jay command. This is open for the T0.5 design.
2. **`sync` without the variable.** `gentle-ai sync` keeps an absolute command for JSON agents but rewrote the Codex `[mcp_servers.engram]` command to a bare `engram` (observed). With the variable exported it restores the qbit-jay path. Hence "permanently" in the Quick path. `install` run without the variable would download and install upstream Engram (by code, `cli/run.go:3396-3430`; not run).
3. **`tool:engram` in `gentle-ai doctor`** fails ("engram not found in PATH") because the doctor looks up `engram` on `PATH` and ignores `GENTLE_AI_ENGRAM_BIN`. The `engram:reachable` check is the one that matters. Cosmetic.
4. **`gentle-ai update` and `upgrade`** detect Engram by running `engram version` from `PATH`. With no `engram` on `PATH` the status is "not installed" and `upgrade` omits it (`update/check.go:150-163`, `update/upgrade/executor.go:518-551`): qbit-jay is never reinstalled or replaced by upstream Engram. If a real `engram` is also on `PATH`, `upgrade` replaces that one. The update check itself could not be completed in the run because the GitHub API was rate limited.
5. **Pi.** gentle-ai installs `npm:gentle-engram` (required, not excludable: `agents/pi/adapter.go:33,66-92`) and prunes duplicates of it (`:716-744`). The package reads `ENGRAM_BIN`, so it runs qbit-jay when that is set. `qbit-jay setup pi` installs the same package name, so there is no conflict today. The `qbit-jay-pi` package name must not be added next to `gentle-engram`: Pi tool names are exclusive.
6. **Hooks and plugins that run `engram` by name (G7).** gentle-ai's own hooks call `gentle-ai`, not `engram` (Codex `hooks.json`, Claude `settings.json` observed). The hooks and plugins that call `engram` are Engram's own; qbit-jay ships its own, installed by `qbit-jay setup`, and they call `qbit-jay`. No `engram` alias is distributed.
7. **`eco-update`** is not in gentle-ai (no match in the repository) and not found on this machine. The only trace is the `eco-update:managed-block` markers in `~/.pi/agent/AGENTS.md`. Whether it can reinstall Engram or rewrite agent configs could not be checked. **Pending for the user:** run `eco-update` with `--dry-run` (or read its source) and check that it neither installs `engram` nor rewrites the `engram` MCP entries.

## Requirements G1 to G7

| # | Requirement | Verdict |
| --- | --- | --- |
| G1 | Same command line as gentle-ai invokes | **Met** for every invocation in the table; `ENGRAM_BIN` is `GENTLE_AI_ENGRAM_BIN` in gentle-ai |
| G2 | upsert by `topic_key`, `mem_update`, `mem_get_observation`, `capture_prompt` | **Met** (trailing whitespace trimmed) |
| G3 | No silent truncation of a mirror | **Not met** above 50,000 bytes; reported in the save response, not silent at the protocol level; Phase 1 H2 |
| G4 | Efficient versions | Phase 1 (H1, H3) |
| G5 | Search finds mirrors by topic and `/` | **Met** |
| G6 | Injected protocol text and tool names keep working | **Met** for tool names; the server name stays `engram` in gentle-ai mode |
| G7 | Hooks and plugins that call `engram` | qbit-jay's own plugins call `qbit-jay`; **open:** duplicate entries and the allowlist dependency above; `qbit-jay setup` replacement and restore is T0.5 |

## Reproduce

```text
go test -count=1 ./internal/qbit/compat/        # builds cmd/qbit-jay, runs every invocation above
```

The simulated gentle-ai run: build `cmd/gentle-ai` from the commit above, then run it with `env -i HOME=<tmp> GENTLE_AI_HOME=<tmp>/gai DO_NOT_TRACK=1 GENTLE_AI_ENGRAM_BIN=<qbit-jay> gentle-ai install --agent gemini-cli,windsurf,cursor,codex,claude-code --component engram`, then `sync` and `doctor` in the same `HOME`. OpenCode needs a working `opencode --version` and was not exercised.

## Not verified

- A run on the user's real machine and WSL (`GENTLE_AI_ENGRAM_BIN` with the Windows and WSL paths): left for the user.
- `eco-update` behavior (above).
- `qbit-jay setup claude-code` and `codex` end to end: the CLIs are not installed in the temporary environment.
- OpenCode through gentle-ai (needs the `opencode` CLI).
- The published `gentle-engram` npm package: only the source in `plugin/pi` was read.
