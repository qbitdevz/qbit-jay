# Release Channels and Support Policy

This policy is the canonical source for choosing a qbit-jay release and planning an upgrade or rollback.

## Choose a release channel

| Channel | Intended use | Security support | Production guidance |
| --- | --- | --- | --- |
| Latest stable | General and production use | Receives security fixes | Recommended for production |
| Release candidate (RC) | Prerelease validation and feedback | Not guaranteed | Not universally suitable for production |
| Older release | Legacy or temporary compatibility needs | Does not receive security fixes | Upgrade to the latest stable release when practical |

Release candidates may expose functionality not yet available in the latest stable release. Operators who need that functionality accept prerelease risk, including the absence of guaranteed security support and universal production suitability.

## Upgrade deliberately

Before upgrading:

1. Choose the release channel deliberately.
2. Read the release notes and any migration notes for the target release.
3. Back up relevant state and configuration.
4. Validate compatibility with your environment and integrations.
5. Retain a known-good installation or artifact until the upgrade is accepted.

After installing the replacement binary, restart any already-running local
`qbit-jay serve` HTTP server through its existing supervisor or launcher. Verify the intended server's
`/health` version against the installed executable and recheck HTTP integrations;
a running process or successful local CLI check alone does not confirm that the
server was upgraded. Follow [After upgrading](./INSTALLATION.md#after-upgrading)
for endpoint, authentication, and service guidance.

## Rollback expectations

Rollback means restoring the known-good release and configuration, plus any required backup, using the documented procedure for the affected component. It does not promise an automated rollback path or behavior beyond that component's documentation. Feature or data migrations can constrain rollback; consult the release-specific notes before upgrading.

## Module metadata before tagging

The existing required **Unit Tests** job runs `go mod tidy -diff` with the same
Go toolchain as the release workflow, before build checks on pull requests,
merge groups and main pushes. Fix and commit any reported `go.mod`/`go.sum`
drift through normal review; release packaging retains its own non-mutating
check and never repairs tagged sources. This reuses the existing check/context,
not a new required status or publication approval.

## How releases are published

A `vX.Y.Z` tag on `qbitdevz/qbit-jay` runs [`release.yml`](../.github/workflows/release.yml): GoReleaser builds the archives and `checksums.txt`, publishes the GitHub Release, and the workflow attaches the Homebrew formula and the Scoop manifest. The workflow token cannot push to the tap and bucket repositories and no personal access token is stored, so the maintainer commits those two files by hand after each release. The steps are in [qbit/RELEASING.md](./qbit/RELEASING.md).

Releases are not code-signed. The install scripts verify the sha256 from `checksums.txt` before installing. If a release is wrong, fix forward with a new tag; never retarget a published tag.

## Related policies

- [Releasing qbit-jay](./qbit/RELEASING.md) for the pipeline, the install channels and the manual steps after a release.
- [Security policy](../SECURITY.md) for vulnerability reporting and supported-version security fixes.
- [Installation guide](./INSTALLATION.md) for installation methods.
- [Engram Cloud quickstart](./engram-cloud/quickstart.md) for Cloud deployment guidance.
