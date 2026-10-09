#!/usr/bin/env bash
set -euo pipefail

REPO="lukaculjak/mak-cli"
BINARY="mak"
INSTALL_DIR="/usr/local/bin"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)       ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  darwin|linux) ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name":' \
  | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Failed to fetch latest version from GitHub." >&2
  exit 1
fi

FILENAME="mak_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$FILENAME"
CHECKSUM_URL="https://github.com/$REPO/releases/download/$VERSION/checksums.txt"

echo "Installing mak $VERSION ($OS/$ARCH)..."

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fsSL "$URL" -o "$TMP_DIR/$FILENAME"
curl -fsSL "$CHECKSUM_URL" -o "$TMP_DIR/checksums.txt"

EXPECTED_CHECKSUM=$(awk -v filename="$FILENAME" '$2 == filename || $2 == "*" filename { print $1; exit }' "$TMP_DIR/checksums.txt")
if [ -z "$EXPECTED_CHECKSUM" ]; then
  echo "Could not find a checksum for $FILENAME." >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL_CHECKSUM=$(sha256sum "$TMP_DIR/$FILENAME" | awk '{print $1}')
else
  ACTUAL_CHECKSUM=$(shasum -a 256 "$TMP_DIR/$FILENAME" | awk '{print $1}')
fi

if [ "$ACTUAL_CHECKSUM" != "$EXPECTED_CHECKSUM" ]; then
  echo "Checksum verification failed for $FILENAME." >&2
  exit 1
fi

tar -xzf "$TMP_DIR/$FILENAME" -C "$TMP_DIR"

if [ ! -f "$TMP_DIR/$BINARY" ]; then
  echo "Release archive does not contain the mak binary." >&2
  exit 1
fi

if [ -w "$INSTALL_DIR" ]; then
  install -m 0755 "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
else
  sudo install -m 0755 "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo ""
echo "mak installed to $INSTALL_DIR/$BINARY"
case "${SHELL:-}" in
  */zsh)
    if ! "$INSTALL_DIR/$BINARY" completion zsh --install; then
      echo "Could not enable zsh completion. Retry with: mak completion zsh --install" >&2
    fi
    ;;
esac
echo "Run 'mak --help' to get started."
