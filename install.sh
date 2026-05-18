#!/usr/bin/env bash
# install.sh — install code-outline-graph-go from GitHub releases
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/rushikeshsakharleofficial/gocode-outline-graph/main/install.sh | bash
#   bash install.sh [--prefix /usr/local] [--version v1.1.0]

set -euo pipefail

REPO="rushikeshsakharleofficial/gocode-outline-graph"
BINARY="code-outline-graph-go"
PREFIX="${PREFIX:-/usr/local}"
VERSION=""

# Parse args
while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix) PREFIX="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    *) echo "Unknown argument: $1"; exit 1 ;;
  esac
done

# Detect OS and arch
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  mingw*|msys*|cygwin*|windows*) OS="windows" ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported arch: $ARCH"; exit 1 ;;
esac

# Fetch latest version if not specified
if [[ -z "$VERSION" ]]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\(.*\)".*/\1/')"
fi
[[ -z "$VERSION" ]] && { echo "Could not determine latest version"; exit 1; }
VER="${VERSION#v}"  # strip leading v for filenames

# Construct download URL
EXT="tar.gz"
[[ "$OS" == "windows" ]] && EXT="zip"
FILENAME="${BINARY}_${VER}_${OS}_${ARCH}.${EXT}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

# Download
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Downloading ${BINARY} ${VERSION} for ${OS}/${ARCH}..."
curl -fsSL -o "${TMP}/${FILENAME}" "${URL}"

# Verify checksum
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
curl -fsSL -o "${TMP}/checksums.txt" "${CHECKSUM_URL}"
(cd "$TMP" && grep "${FILENAME}" checksums.txt | sha256sum --check --status) \
  || { echo "Checksum verification failed"; exit 1; }

# Extract
mkdir -p "${TMP}/bin"
if [[ "$EXT" == "zip" ]]; then
  unzip -q "${TMP}/${FILENAME}" -d "${TMP}/bin"
else
  tar -xzf "${TMP}/${FILENAME}" -C "${TMP}/bin"
fi

# Install
INSTALL_DIR="${PREFIX}/bin"
mkdir -p "${INSTALL_DIR}"
BIN_SRC="$(find "${TMP}/bin" -name "${BINARY}" -o -name "${BINARY}.exe" | head -1)"
[[ -z "$BIN_SRC" ]] && { echo "Binary not found in archive"; exit 1; }
install -m755 "${BIN_SRC}" "${INSTALL_DIR}/${BINARY}"

echo ""
echo "Installed ${BINARY} ${VERSION} → ${INSTALL_DIR}/${BINARY}"
echo ""
echo "Quick start:"
echo "  ${BINARY} build .       # index current project"
echo "  ${BINARY} serve         # start MCP server"
echo "  ${BINARY} install       # write MCP config for Claude/editors"
