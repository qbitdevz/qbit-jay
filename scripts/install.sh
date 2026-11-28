#!/bin/sh
# qbit-jay installer for Linux, macOS and WSL.
#
#   curl -fsSL https://raw.githubusercontent.com/qbitdevz/qbit-jay/main/scripts/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/qbitdevz/qbit-jay/main/scripts/install.sh | sh -s -- v0.1.0
#
# Downloads checksums.txt and the archive for this OS and architecture from a
# GitHub Release of qbitdevz/qbit-jay, verifies the archive's sha256 against
# checksums.txt, and only then installs the binary. Nothing is installed when
# the checksum does not match. It never calls sudo.
#
# Environment:
#   QBIT_JAY_VERSION           release tag to install (default: latest); the first
#                              argument does the same and wins.
#   QBIT_JAY_INSTALL_DIR       target directory (default: $HOME/.local/bin).
#   QBIT_JAY_INSTALL_BASE_URL  directory URL holding checksums.txt and the
#                              archive (https://, http:// or file://). Meant for
#                              mirrors and tests; replaces the GitHub Release URL.
#
# Limits: checksums.txt and the archive come from the same origin, so this
# detects corrupt or truncated downloads and mismatched assets, not a
# compromised release.

set -eu

REPO="qbitdevz/qbit-jay"
BIN="qbit-jay"

say() { printf '%s\n' "$*"; }
fail() {
	printf 'qbit-jay install: %s\n' "$*" >&2
	exit 1
}

version="${1:-${QBIT_JAY_VERSION:-latest}}"
install_dir="${QBIT_JAY_INSTALL_DIR:-}"
if [ -z "$install_dir" ]; then
	[ -n "${HOME:-}" ] || fail "HOME is not set; set QBIT_JAY_INSTALL_DIR"
	install_dir="$HOME/.local/bin"
fi

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=darwin ;;
MINGW* | MSYS* | CYGWIN*) fail "Windows detected; use scripts/install.ps1 (irm https://raw.githubusercontent.com/$REPO/main/scripts/install.ps1 | iex)" ;;
*) fail "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) fail "unsupported architecture: $(uname -m)" ;;
esac

if [ -n "${QBIT_JAY_INSTALL_BASE_URL:-}" ]; then
	base="${QBIT_JAY_INSTALL_BASE_URL%/}"
	if [ "$version" != "latest" ]; then
		say "QBIT_JAY_INSTALL_BASE_URL is set: installing whatever $base serves, not version $version"
	fi
elif [ "$version" = "latest" ]; then
	base="https://github.com/$REPO/releases/latest/download"
else
	case "$version" in
	v*) tag="$version" ;;
	*) tag="v$version" ;;
	esac
	case "$tag" in
	*[!A-Za-z0-9._+-]*) fail "invalid version: $version" ;;
	esac
	base="https://github.com/$REPO/releases/download/$tag"
fi

if command -v curl >/dev/null 2>&1; then
	download() { curl -fsSL --retry 2 -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
	download() { wget -q -O "$2" "$1"; }
else
	fail "curl or wget is required"
fi

if command -v sha256sum >/dev/null 2>&1; then
	sha256() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
	sha256() { shasum -a 256 "$1" | awk '{print $1}'; }
else
	fail "sha256sum or shasum is required to verify the download"
fi

tmp="$(mktemp -d 2>/dev/null || mktemp -d -t qbit-jay)" || fail "cannot create a temporary directory"
staged=""
cleanup() {
	rm -rf "$tmp"
	if [ -n "$staged" ]; then rm -f "$staged"; fi
}
trap cleanup EXIT
# A signal must stop the script, not resume it against a deleted temp dir.
trap 'exit 130' HUP INT TERM

say "Downloading checksums.txt ($base)"
download "$base/checksums.txt" "$tmp/checksums.txt" || fail "cannot download $base/checksums.txt"

# The archive name carries the version, so take it from checksums.txt.
matches="$(awk -v os="$os" -v arch="$arch" \
	'$2 ~ ("^" "qbit-jay_[0-9A-Za-z.+-]+_" os "_" arch "\\.tar\\.gz$") { print $2 }' "$tmp/checksums.txt")"
[ -n "$matches" ] || fail "no ${os}/${arch} archive listed in checksums.txt"
[ "$(printf '%s\n' "$matches" | wc -l | tr -d ' ')" = 1 ] || fail "more than one ${os}/${arch} archive listed in checksums.txt"
archive="$matches"
want="$(awk -v f="$archive" '$2 == f { print tolower($1) }' "$tmp/checksums.txt")"

say "Downloading $archive"
download "$base/$archive" "$tmp/$archive" || fail "cannot download $base/$archive"

got="$(sha256 "$tmp/$archive")"
if [ "$got" != "$want" ]; then
	fail "checksum mismatch for $archive (expected $want, got $got); nothing was installed"
fi
say "Checksum verified (sha256 $got)"

mkdir "$tmp/x"
# Extract only the binary, and only when the archive really contains it.
tar -tzf "$tmp/$archive" | grep -qx "$BIN" || fail "$archive does not contain $BIN"
tar -xzf "$tmp/$archive" -C "$tmp/x" "$BIN" || fail "cannot extract $archive"
[ -f "$tmp/x/$BIN" ] || fail "$BIN missing after extraction"

mkdir -p "$install_dir" || fail "cannot create $install_dir; set QBIT_JAY_INSTALL_DIR to a writable directory"
# Stage inside the target directory so the final rename is atomic.
staged="$install_dir/.$BIN.new.$$"
cp "$tmp/x/$BIN" "$staged" || fail "cannot write to $install_dir; set QBIT_JAY_INSTALL_DIR to a writable directory (this script never uses sudo)"
chmod 0755 "$staged"
mv -f "$staged" "$install_dir/$BIN" || fail "cannot install into $install_dir"
staged=""

say "Installed $install_dir/$BIN"
"$install_dir/$BIN" version || true

case ":${PATH:-}:" in
*":$install_dir:"*) ;;
*)
	say ""
	say "$install_dir is not on your PATH. Add it, for example:"
	say "  export PATH=\"$install_dir:\$PATH\""
	;;
esac
