[← Back to README](../README.md)

# Installation

- [Choose a release channel before installing](#choose-a-release-channel-before-installing)
- [Package managers and releases (coming later)](#package-managers-and-releases-coming-later)
- [Windows](#windows)
- [Install from source (macOS / Linux)](#install-from-source-macos--linux)
- [After upgrading](#after-upgrading)
- [Requirements](#requirements)
- [Data-directory filesystem safety](#data-directory-filesystem-safety)
- [Environment Variables](#environment-variables)
- [Windows Config Paths](#windows-config-paths)

---

## Choose a release channel before installing

qbit-jay has no stable release yet, so `go install ...@latest` resolves to the newest tagged version, or to the latest commit while no tag exists. The [Release Policy](./RELEASE-POLICY.md) is inherited from upstream and describes how channels, upgrades and rollback are meant to work once qbit-jay publishes releases.

---

## Package managers and releases (coming later)

qbit-jay has no published release yet. There is no Homebrew formula, Scoop bucket, npm package, install script or prebuilt binary, so the only supported install today is from source with Go (below). When releases exist, this page will list each channel here.

---

## Windows

**Option A: Install via `go install` (recommended)**

With Go installed, the binary is compiled locally from source. This does not guarantee protection from antivirus alerts or Windows security-policy blocking.

```powershell
go install github.com/qbitdevz/qbit-jay/cmd/qbit-jay@latest
# Binary goes to %GOPATH%\bin\qbit-jay.exe (typically %USERPROFILE%\go\bin\)
```

Ensure `%GOPATH%\bin` (or `%USERPROFILE%\go\bin`) is on your `PATH`.

**Option B: Build from source**

```powershell
git clone https://github.com/qbitdevz/qbit-jay.git
cd qbit-jay
go install ./cmd/qbit-jay
# Binary goes to %GOPATH%\bin\qbit-jay.exe (typically %USERPROFILE%\go\bin\)
```

> **Want a real version string instead of `dev`?**
>
> `go install` always stamps the binary as `dev`. To get a meaningful version, pick one of these — not both. Running them both leaves two binaries on disk and `qbit-jay version` keeps reporting `dev` because PATH still resolves to the `go install` build.
>
> **Option B1 — version-stamped `go install` (binary stays on PATH):**
>
> ```powershell
> $v = git describe --tags --always
> go install -ldflags="-X main.version=local-$v" ./cmd/qbit-jay
> ```
>
> **Option B2 — `go build` and move the result onto PATH:**
>
> ```powershell
> $v = git describe --tags --always
> go build -ldflags="-X main.version=local-$v" -o qbit-jay.exe ./cmd/qbit-jay
> Move-Item -Force qbit-jay.exe "$env:USERPROFILE\go\bin\qbit-jay.exe"
> ```
>
> After either option, `qbit-jay version` should print `local-<git-describe>` instead of `dev`.

> **Other Windows notes:**
> - Data is stored in `%USERPROFILE%\.qbit-jay\qbit-jay.db`
> - Override with `QBIT_JAY_DATA_DIR` environment variable
> - All core features work natively: CLI, MCP server, TUI, HTTP API, Git Sync
> - No WSL required for the core binary — it's a native Windows executable

---

## Windows troubleshooting: executable blocked before MCP starts

**Distinguish a missing executable from Windows rejecting an existing one.**
Smart App Control or another security policy can prevent `qbit-jay.exe` from
starting, including a locally compiled `go install` binary. Building locally is
not a guarantee against security-policy blocking, even though it is not a prebuilt download.

| Observation | What to check |
|-------------|---------------|
| The configured executable does not exist, or the client resolves a different file | Check the exact MCP executable path and the installation location; this is a path-resolution problem. |
| Windows explicitly reports that security policy blocked the existing executable | Treat this as an OS launch rejection, not an MCP protocol or argument error. |
| Git Bash reports `Permission denied` / exit 126, or an MCP client reports `EUNKNOWN: uv_spawn` | These errors alone do not identify Smart App Control; correlate them with the Windows launch error and security-policy evidence. |
| Authenticode status is `NotSigned` | This describes the file's signature, but does not by itself prove which policy blocked it. A signature status alone is not a policy diagnosis. |

Issue #1703 reports Windows 11 Smart App Control enabled, an unsigned
`qbit-jay.exe`, a PowerShell policy rejection, Git Bash exit 126, and a Claude Code
MCP `EUNKNOWN: uv_spawn` error. These are reported observations, not an
independently reproduced diagnosis for every similar startup failure.

If Windows rejects the executable before it starts, the MCP server cannot begin
its protocol exchange. Reinstalling or changing MCP arguments cannot be assumed
to resolve that rejection. Locally compiled binaries are separate from release
artifacts: signing a distributed release would not sign a local `go install`
build. This guidance does not establish the signing status of current downloads
or identify a signed workaround.

**For a support report, collect only the relevant evidence:**

- Installation route: `go install`, local source build, downloaded release, or installer.
- Exact executable path configured in the MCP client, and whether that file exists.
- Exact error from an already attempted direct launch of that same file, including the shell used; do not substitute only the MCP client's generic error.
- Authenticode signature status for that same file, plus any explicit Windows security-policy message or known Smart App Control state.

Redact personal path components when sharing evidence. Do not disable Smart App
Control, antivirus, or other security controls to troubleshoot this issue. On a
managed device, ask the security administrator to review the rejection under the
organization's policy rather than attempting a bypass.

---

## Install from source (macOS / Linux)

```bash
go install github.com/qbitdevz/qbit-jay/cmd/qbit-jay@latest
# Binary goes to $GOPATH/bin (typically ~/go/bin/)
```

Or build from a local clone:

```bash
git clone https://github.com/qbitdevz/qbit-jay.git
cd qbit-jay
go install ./cmd/qbit-jay
```

> **Want a real version string instead of `dev`?**
>
> `go install` always stamps the binary as `dev`. To get a meaningful version, pick one of these — not both. Running them both leaves two binaries on disk and `qbit-jay version` keeps reporting `dev` because PATH still resolves to the `go install` build.
>
> **Option 1 — version-stamped `go install` (binary stays on PATH):**
>
> ```bash
> go install -ldflags="-X main.version=local-$(git describe --tags --always)" ./cmd/qbit-jay
> ```
>
> **Option 2 — `go build` and move the result onto PATH:**
>
> ```bash
> go build -ldflags="-X main.version=local-$(git describe --tags --always)" -o qbit-jay ./cmd/qbit-jay
> mv qbit-jay "$(go env GOPATH)/bin/qbit-jay"
> ```
>
> After either option, `qbit-jay version` should print `local-<git-describe>` instead of `dev`.

---

## After upgrading

These steps apply to source installs, and to other channels once they exist. Keep the
backup and compatibility checks in the [release policy](./RELEASE-POLICY.md#upgrade-deliberately).

1. Confirm that your supervisor or launcher points to the updated executable.
   `qbit-jay version` reports the executable selected by your shell; a service may
   use a different absolute path.
2. Restart any already-running `qbit-jay serve` HTTP server through the supervisor or launcher
   that owns it, keeping the same service account and data-directory configuration.
   Use your configured [systemd service](../DOCS.md#using-systemd-linux),
   [launchd agent](../DOCS.md#using-launchd-macos), or
   [Windows scheduled task](../DOCS.md#using-windows-task-scheduler). For a manually
   started foreground `qbit-jay serve`, stop that process and start the updated
   executable with the same arguments and environment. Replacing a binary alone
   does not make an older process serve the new code. If a Windows task launches
   a separate child process, confirm that the previous server has actually stopped;
   cycling the task alone may leave that child running.
3. Compare the installed version with the `version` field returned by the HTTP
   server your client actually uses, then validate that integration again.

For a local server using the default port and no additional authentication,
read both versions with:

```sh
qbit-jay version
curl --fail --silent --show-error http://127.0.0.1:7437/health
```

Use the client's known endpoint, port, and required authentication if configured
differently. For a server on another host, compare against the executable you
upgraded on that host. A successful local response includes `"service":"engram"` and a
`"version"` field. The running version should match the release you just installed.
If it differs, check the launcher's executable path and restart the intended
server through its owner. A missing version or `dev` does not confirm which
release is running.

`qbit-jay cloud status` reporting `Local daemon: running` proves liveness only.
Its `Versions:` line compares the server's reported semantic release version
with the installed CLI and prints `Restart pending` when both are known and
differ. `qbit-jay ps` shows the same comparison for its identity-confirmed daemon.
Missing, malformed or development versions are `unknown`; build metadata is
ignored. This comparison does not authenticate a listener or restart anything.
Use the intended server's existing supervisor; `qbit-jay restart` manages only
the pidfile-tracked daemon. Successful local CLI, doctor, or direct-store MCP
operations do not confirm the version of a separate HTTP server.

---

## Requirements

- **Go 1.26+** to build from source
- That's it. No runtime dependencies.

The binary includes SQLite (via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — pure Go, no CGO). Works natively on **macOS**, **Linux**, and **Windows** (x86_64 and ARM64).

---

## Data-directory filesystem safety

qbit-jay uses persistent SQLite WAL and rejects known NFS and SMB/CIFS data directories before changing the SQLite files. Use a local disk for `QBIT_JAY_DATA_DIR`; an unknown filesystem remains compatible but is not proven local.

For local stores, qbit-jay sets SQLite's journal size limit to 64 MiB. SQLite
reclaims excess WAL space when it can safely reuse the file or close the last
connection. Active readers and writers can still retain more than 64 MiB while
they need those frames. qbit-jay keeps the WAL and shared-memory sidecar files.

If startup rejects a network data directory, stop **all** qbit-jay processes. Copy the complete `qbit-jay.db`, `qbit-jay.db-wal`, and `qbit-jay.db-shm` triplet to local storage, set `QBIT_JAY_DATA_DIR` to the absolute path of that local directory (relative paths are rejected), then start qbit-jay and run `qbit-jay doctor`. Check SQLite integrity with the command for your shell:

```bash
# POSIX shell or Git Bash
sqlite3 "$QBIT_JAY_DATA_DIR/qbit-jay.db" "PRAGMA integrity_check;"
```

```powershell
# PowerShell
sqlite3 (Join-Path $env:QBIT_JAY_DATA_DIR 'qbit-jay.db') 'PRAGMA integrity_check;'
```

qbit-jay does not auto-repair, quarantine, checkpoint, or use rollback journaling as a fallback.

---

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `QBIT_JAY_DATA_DIR` | qbit-jay CLI data directory. Empty or whitespace-only values use the platform default; nonblank values are used as provided. | `~/.qbit-jay` (Windows: `%USERPROFILE%\.qbit-jay`) |
| `QBIT_JAY_PORT` | HTTP server port. Use an unsigned decimal value from `1` through `65535`; invalid values fall back to `7437` in `qbit-jay serve` and Claude Bash hooks. | `7437` |
| `QBIT_JAY_SOCKET` | POSIX-only Unix-domain socket path. Run `qbit-jay serve --socket /path/to/qbit-jay.sock` (or set `QBIT_JAY_SOCKET`) to listen exclusively on the socket; do not combine it with an explicit TCP port. Claude Bash hooks use the same socket when the variable is exported and warn on stderr if socket transport cannot preserve memory capture. PowerShell remains TCP-only. | (unset) |
| `QBIT_JAY_STARTUP_PHASES` | Set to `1` to report startup phases on stderr during a shared foreground TCP serve. | (unset) |

### Diagnose a slow local server start

Set `QBIT_JAY_STARTUP_PHASES=1` for a planned foreground `qbit-jay serve` startup.
Each `QBIT_JAY_STARTUP_PHASE v1` line contains elapsed milliseconds and a fixed
phase label. If the early listener is allocated, the trace separates advisory
lock admission, SQLite connection setup and migration work. A
`*-retrying` marker records an advisory lock attempt that needs another try.
On Windows this indicates a lock conflict. A
`sqlite-*-lock-result` marker records a lock error returned by SQLite. SQLite
can resolve a lock wait inside the driver, so a long phase can have no lock
marker. `http-dispatch` marks handoff to the server loop. Use `GET /health`
to measure when the server becomes ready.

When reporting an incident, share the lines starting with
`QBIT_JAY_STARTUP_PHASE v1`. These contain only elapsed time and fixed labels.

---

## Windows Config Paths

When using `qbit-jay setup`, config files are written to platform-appropriate locations:

| Agent | macOS / Linux | Windows |
|-------|---------------|---------|
| OpenCode | `~/.config/opencode/` | `%APPDATA%\opencode\` |
| Gemini CLI | `~/.gemini/` | `%APPDATA%\gemini\` |
| Codex | `$CODEX_HOME/` when absolute, else `~/.codex/` | `%CODEX_HOME%\` when absolute, else `%USERPROFILE%\.codex\` |
| Claude Code | Managed by `claude` CLI | Managed by `claude` CLI |
| Antigravity CLI | `~/.gemini/config/mcp_config.json` + `~/.gemini/GEMINI.md` | `%APPDATA%\gemini\config\mcp_config.json` + `%APPDATA%\gemini\GEMINI.md` |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` + `.../memories/global_rules.md` | `%USERPROFILE%\.codeium\windsurf\...` |
| Qwen Code | `~/.qwen/settings.json` + `~/.qwen/QWEN.md` | `%USERPROFILE%\.qwen\...` |
| Kiro | `~/.kiro/settings/mcp.json` + `~/.kiro/steering/engram.md` | `%USERPROFILE%\.kiro\...` |
| Cursor | `~/.cursor/mcp.json` + `~/.cursor/rules/engram.mdc` | `%USERPROFILE%\.cursor\...` |
| VS Code Copilot | `~/.config/Code/User/mcp.json` + `.../prompts/engram.instructions.md` (macOS: `~/Library/Application Support/Code/User/`) | `%APPDATA%\Code\User\...` |
| Kilo Code | `~/.config/kilo/opencode.json` + `~/.config/kilo/AGENTS.md` | `%USERPROFILE%\.config\kilo\...` |
| Kimi Code | `~/.kimi-code/mcp.json` + `~/.kimi-code/AGENTS.md` | `%USERPROFILE%\.kimi-code\...` |
| CommandCode | `~/.commandcode/mcp.json` + `~/.commandcode/AGENTS.md` | `%USERPROFILE%\.commandcode\mcp.json` + `%USERPROFILE%\.commandcode\AGENTS.md` |
| Data directory | `~/.qbit-jay/` | `%USERPROFILE%\.qbit-jay\` |
