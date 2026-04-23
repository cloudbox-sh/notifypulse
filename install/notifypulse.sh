#!/bin/sh
# Notifypulse CLI installer for Linux / macOS.
#
# Usage:
#   curl -sSL https://get.cloudbox.sh/notifypulse | sh
#   curl -sSL https://get.cloudbox.sh/notifypulse | INSTALL_VERSION=v0.2.0 sh
#   curl -sSL https://get.cloudbox.sh/notifypulse | INSTALL_DIR=$HOME/bin sh
#
# Environment:
#   INSTALL_VERSION  pin a specific tag (e.g. v0.2.0). Default: latest release.
#   INSTALL_DIR      install target. Auto: /usr/local/bin (with sudo) or $HOME/.local/bin.
#
# The script downloads the matching GoReleaser tarball, verifies the SHA-256
# against the published checksums.txt, and extracts the binary into INSTALL_DIR.

set -eu

OWNER="cloudbox-sh"
REPO="notifypulse"
BIN="notifypulse"

info() { printf '\033[1;35m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[1;31mxx\033[0m %s\n' "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || err "missing required tool: $1"
}

need uname
need tar
need mkdir

# Fetcher: curl (preferred) or wget.
if command -v curl >/dev/null 2>&1; then
  FETCH="curl -fsSL"
  FETCH_OUT="curl -fsSL -o"
elif command -v wget >/dev/null 2>&1; then
  FETCH="wget -qO-"
  FETCH_OUT="wget -qO"
else
  err "curl or wget is required"
fi

# SHA-256 verifier: shasum -a 256, sha256sum, or openssl.
if command -v sha256sum >/dev/null 2>&1; then
  SHA_CMD="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  SHA_CMD="shasum -a 256"
elif command -v openssl >/dev/null 2>&1; then
  SHA_CMD="openssl dgst -sha256 -r"
else
  err "sha256sum / shasum / openssl is required for checksum verification"
fi

# Detect OS.
OS=$(uname -s)
case "$OS" in
  Linux)  OS_TAG="linux" ;;
  Darwin) OS_TAG="darwin" ;;
  *) err "unsupported OS: $OS (use the Windows installer on Windows)" ;;
esac

# Detect arch.
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH_TAG="x86_64" ;;
  aarch64|arm64) ARCH_TAG="arm64" ;;
  *) err "unsupported architecture: $ARCH" ;;
esac

# Resolve version.
VERSION="${INSTALL_VERSION:-}"
if [ -z "$VERSION" ]; then
  info "resolving latest release tag"
  VERSION=$(
    $FETCH "https://api.github.com/repos/$OWNER/$REPO/releases/latest" |
      sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1
  )
  [ -n "$VERSION" ] || err "could not determine latest release tag"
fi
# Strip leading 'v' for the archive name template.
VERSION_NO_V="${VERSION#v}"

ARCHIVE="${BIN}_${VERSION_NO_V}_${OS_TAG}_${ARCH_TAG}.tar.gz"
CHECKSUMS="checksums.txt"
BASE_URL="https://github.com/$OWNER/$REPO/releases/download/$VERSION"

# Pick install dir.
if [ -z "${INSTALL_DIR:-}" ]; then
  if [ -w /usr/local/bin ]; then
    INSTALL_DIR=/usr/local/bin
    SUDO=""
  elif command -v sudo >/dev/null 2>&1 && [ -d /usr/local/bin ]; then
    INSTALL_DIR=/usr/local/bin
    SUDO="sudo"
  else
    INSTALL_DIR="${HOME}/.local/bin"
    SUDO=""
    mkdir -p "$INSTALL_DIR"
  fi
else
  SUDO=""
  mkdir -p "$INSTALL_DIR"
fi

TMP=$(mktemp -d 2>/dev/null || mktemp -d -t notifypulse)
trap 'rm -rf "$TMP"' EXIT INT TERM

info "downloading $ARCHIVE ($VERSION)"
$FETCH_OUT "$TMP/$ARCHIVE" "$BASE_URL/$ARCHIVE" || err "download failed"

info "downloading $CHECKSUMS"
$FETCH_OUT "$TMP/$CHECKSUMS" "$BASE_URL/$CHECKSUMS" || err "checksum download failed"

info "verifying SHA-256"
EXPECTED=$(grep " $ARCHIVE\$" "$TMP/$CHECKSUMS" | awk '{print $1}')
[ -n "$EXPECTED" ] || err "archive not listed in checksums.txt"
ACTUAL=$($SHA_CMD "$TMP/$ARCHIVE" | awk '{print $1}')
[ "$EXPECTED" = "$ACTUAL" ] || err "checksum mismatch: expected $EXPECTED, got $ACTUAL"

info "extracting"
tar -xzf "$TMP/$ARCHIVE" -C "$TMP" "$BIN"
chmod +x "$TMP/$BIN"

info "installing to $INSTALL_DIR"
$SUDO mv "$TMP/$BIN" "$INSTALL_DIR/$BIN"

info "installed $BIN $VERSION to $INSTALL_DIR/$BIN"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    warn "$INSTALL_DIR is not in your PATH. Add it to your shell profile:"
    printf '    export PATH="%s:$PATH"\n' "$INSTALL_DIR"
    ;;
esac

"$INSTALL_DIR/$BIN" version || true
