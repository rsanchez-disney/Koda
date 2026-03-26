#!/bin/bash
# Koda installer — one-liner: curl -fsSL https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.sh | bash
set -e

REPO="rsanchez-disney/Koda"
INSTALL_DIR="${KODA_INSTALL_DIR:-$HOME/.local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

BINARY="koda-${OS}-${ARCH}"

echo ""
echo "   Installing Koda..."
echo "   OS: $OS  Arch: $ARCH"
echo ""

# Find latest release tag
TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "$TAG" ]; then
  echo "Could not determine latest release."
  echo "Check: https://github.com/$REPO/releases"
  exit 1
fi

URL="https://github.com/$REPO/releases/download/$TAG/$BINARY"
echo "   Version: $TAG"
echo "   URL: $URL"
echo ""

# Download
mkdir -p "$INSTALL_DIR"
TMP=$(mktemp)
trap "rm -f $TMP" EXIT

curl -fsSL -o "$TMP" "$URL"

chmod +x "$TMP"
mv "$TMP" "$INSTALL_DIR/koda"

# Verify
if [ -x "$INSTALL_DIR/koda" ]; then
  echo "   Installed: $INSTALL_DIR/koda"
  "$INSTALL_DIR/koda" version 2>/dev/null || true
  echo ""
  if ! echo "$PATH" | tr ':' '\n' | grep -q "$INSTALL_DIR"; then
    echo "   Add to PATH:"
    echo "     export PATH=\"$INSTALL_DIR:\$PATH\""
    echo ""
  fi
else
  echo "   Installation failed."
  exit 1
fi
