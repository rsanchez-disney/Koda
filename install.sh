#!/bin/bash
# Koda installer — one-liner: curl -fsSL https://github.disney.com/raw/SANCR225/Koda/main/install.sh | bash
set -e

REPO="SANCR225/Koda"
GH_HOST="github.disney.com"
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
if command -v gh &>/dev/null; then
  TAG=$(gh release view --repo "$GH_HOST/$REPO" --json tagName -q .tagName 2>/dev/null || true)
fi

if [ -z "$TAG" ]; then
  # Fallback: GitHub API
  TAG=$(curl -fsSL "https://$GH_HOST/api/v3/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
fi

if [ -z "$TAG" ]; then
  echo "Could not determine latest release."
  echo "Try: gh release view --repo $GH_HOST/$REPO"
  exit 1
fi

URL="https://$GH_HOST/$REPO/releases/download/$TAG/$BINARY"
echo "   Version: $TAG"
echo "   URL: $URL"
echo ""

# Download
mkdir -p "$INSTALL_DIR"
TMP=$(mktemp)
trap "rm -f $TMP" EXIT

if command -v gh &>/dev/null; then
  gh release download "$TAG" --repo "$GH_HOST/$REPO" --pattern "$BINARY" --output "$TMP" 2>/dev/null || \
    curl -fsSL -o "$TMP" "$URL"
else
  curl -fsSL -o "$TMP" "$URL"
fi

chmod +x "$TMP"
mv "$TMP" "$INSTALL_DIR/koda"

# Verify
if [ -x "$INSTALL_DIR/koda" ]; then
  echo "   Installed: $INSTALL_DIR/koda"
  "$INSTALL_DIR/koda" version 2>/dev/null || true
  echo ""
  # Check PATH
  if ! echo "$PATH" | tr ':' '\n' | grep -q "$INSTALL_DIR"; then
    echo "   Add to PATH:"
    echo "     export PATH=\"$INSTALL_DIR:\$PATH\""
    echo ""
  fi
else
  echo "   Installation failed."
  exit 1
fi
