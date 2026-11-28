# Releasing qbit-jay

How a release is cut, and the manual steps that remain because no personal access token exists (spec S12, S13; task T0.9). Everything here runs on the free tier of GitHub only.

## What the pipeline does

Pushing a tag `vX.Y.Z` to `qbitdevz/qbit-jay` triggers [`release.yml`](../../.github/workflows/release.yml), which uses only the workflow `GITHUB_TOKEN` (`contents: write`):

1. Checks out the tag, sets up Go, runs `go mod tidy -diff`.
2. Runs GoReleaser (`.goreleaser.yaml`) and publishes the GitHub Release `vX.Y.Z` with:
   - `qbit-jay_<version>_<os>_<arch>.tar.gz` (Linux, macOS) and `.zip` (Windows) for `amd64` and `arm64`,
   - `checksums.txt` (sha256 of every archive).
3. GoReleaser also writes the Homebrew formula and the Scoop manifest into `dist/` (`skip_upload: true`: the workflow token cannot push to other repositories, so it never tries), and the workflow attaches them to the same release as `qbit-jay.rb` and `qbit-jay.json`.

The workflow does nothing in forks (`github.repository == 'qbitdevz/qbit-jay'`). There is no npm, SignPath, Docker image or Homebrew token in the pipeline; the Pi plugin is embedded in the binary in a separate change.

Prerelease tags (`v0.2.0-rc.1`) are published as GitHub prereleases (`prerelease: auto`).

## Before tagging

- CI green on `main`; `go mod tidy -diff` clean.
- Dry run on a clone: `goreleaser release --snapshot --clean`, then run the installer against `dist/`:

  ```sh
  QBIT_JAY_INSTALL_BASE_URL=file://$PWD/dist QBIT_JAY_INSTALL_DIR=$(mktemp -d) sh scripts/install.sh
  ```

- Version numbering is qbit-jay's own, starting at `0.1.0`. `qbit-jay version` prints exactly `qbit-jay <semver>`; gentle-ai parses the first version it finds ([INTEGRATION.md](../INTEGRATION.md)), and whether a second line with the Engram base version would be ignored is not verified, so the base version lives in [UPSTREAM-DELTA.md](../../UPSTREAM-DELTA.md) and the release notes, not in the command output.

## Manual steps after the release (maintainer)

The tap and bucket repositories are published with your own `gh` login, once per release:

One-time setup (creates public repositories under `qbitdevz`; free):

```sh
gh repo create qbitdevz/homebrew-tap --public --description "Homebrew tap for qbit-jay"
gh repo create qbitdevz/scoop-bucket --public --description "Scoop bucket for qbit-jay"
```

Per release (replace `v0.1.0`):

```sh
tag=v0.1.0
tmp=$(mktemp -d) && cd "$tmp"
gh release download "$tag" -R qbitdevz/qbit-jay -p qbit-jay.rb -p qbit-jay.json

# Homebrew: Formula/qbit-jay.rb
gh repo clone qbitdevz/homebrew-tap && mkdir -p homebrew-tap/Formula
cp qbit-jay.rb homebrew-tap/Formula/qbit-jay.rb
git -C homebrew-tap add Formula/qbit-jay.rb && git -C homebrew-tap commit -m "qbit-jay $tag" && git -C homebrew-tap push

# Scoop: bucket/qbit-jay.json
gh repo clone qbitdevz/scoop-bucket && mkdir -p scoop-bucket/bucket
cp qbit-jay.json scoop-bucket/bucket/qbit-jay.json
git -C scoop-bucket add bucket/qbit-jay.json && git -C scoop-bucket commit -m "qbit-jay $tag" && git -C scoop-bucket push
```

Then check the channels:

```sh
brew install qbitdevz/tap/qbit-jay && qbit-jay version
scoop bucket add qbitdevz https://github.com/qbitdevz/scoop-bucket; scoop install qbit-jay
curl -fsSL https://raw.githubusercontent.com/qbitdevz/qbit-jay/main/scripts/install.sh | sh
```

If the tap or bucket is not updated, `go install` and the install scripts still work; only `brew` and `scoop` lag.

## Installers

| Channel | Command | Verification |
| --- | --- | --- |
| Linux, macOS, WSL | `curl -fsSL https://raw.githubusercontent.com/qbitdevz/qbit-jay/main/scripts/install.sh \| sh` | `scripts/install.sh` downloads `checksums.txt` and the archive, compares sha256, then installs to `~/.local/bin` (`QBIT_JAY_INSTALL_DIR`). Never uses `sudo`. |
| Windows | `irm https://raw.githubusercontent.com/qbitdevz/qbit-jay/main/scripts/install.ps1 \| iex` | `scripts/install.ps1` verifies the sha256 with .NET (works even when `Get-FileHash` is unavailable), installs to `%LOCALAPPDATA%\Programs\qbit-jay`, adds it to the user PATH. No administrator rights. |
| Go | `go install github.com/qbitdevz/qbit-jay/cmd/qbit-jay@latest` | Go module checksum database. Prints `dev` as version. |
| Homebrew | `brew install qbitdevz/tap/qbit-jay` | sha256 inside the formula. |
| Scoop | `scoop bucket add qbitdevz https://github.com/qbitdevz/scoop-bucket; scoop install qbit-jay` | `hash` inside the manifest. |

A specific version: `... | sh -s -- v0.1.0` (POSIX) or `$env:QBIT_JAY_VERSION='v0.1.0'` before the PowerShell line.

Limit: `checksums.txt` and the archives come from the same GitHub Release, so the scripts catch corrupt, truncated or mismatched downloads, not a compromised release. Archives are not code-signed (SignPath was removed: no certificate of our own).

## Known follow-ups

- GoReleaser reports `brews` as deprecated in favor of `homebrew_casks`. Casks are macOS-only, and the formula also serves Linux, so `brews` stays until GoReleaser removes it. Tested with GoReleaser v2.20.3; the workflow uses `~> v2`.
- The Cloud image is not published (no registry; spec Q6).
