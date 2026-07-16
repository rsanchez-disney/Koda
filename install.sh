#!/bin/bash
# Koda installer — one-liner: curl -fsSL https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.sh | bash
# With Mouseketool: curl -fsSL https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.sh | bash -s -- --with-mouseketool
# Diagnose: curl -fsSL https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.sh | bash -s -- --diagnose
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

# Diagnostic mode
if [[ " $* " == *"--diagnose"* ]]; then
  echo ""
  echo "   🔍 Koda Install Diagnostics"
  echo "   ==========================="
  echo ""
  echo "   Shell: $SHELL ($BASH_VERSION)"
  echo "   OS: $OS  Arch: $ARCH"
  echo "   Install dir: $INSTALL_DIR"
  echo ""

  # Test DNS
  echo "   [1/4] DNS resolution for api.github.com..."
  if host api.github.com >/dev/null 2>&1; then
    IP=$(host api.github.com 2>/dev/null | grep "has address" | head -1 | awk '{print $NF}')
    echo "         ✓ Resolved to: $IP"
  elif nslookup api.github.com >/dev/null 2>&1; then
    echo "         ✓ DNS resolves (nslookup)"
  else
    echo "         ✗ FAILED — DNS blocked. Check proxy or VPN."
  fi

  # Test GitHub API
  echo "   [2/4] GitHub API (https://api.github.com/repos/$REPO/releases/latest)..."
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "https://api.github.com/repos/$REPO/releases/latest" --max-time 10 2>/dev/null || echo "000")
  if [ "$HTTP_CODE" = "200" ]; then
    TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
    echo "         ✓ HTTP 200 — Latest release: $TAG"
  elif [ "$HTTP_CODE" = "403" ]; then
    echo "         ✗ HTTP 403 — Rate limited."
    REMAINING=$(curl -s "https://api.github.com/rate_limit" 2>/dev/null | grep -A2 '"rate"' | grep '"remaining"' | sed 's/[^0-9]//g')
    echo "         → Remaining requests: ${REMAINING:-unknown}"
    echo "         → Wait 1 hour or set GITHUB_TOKEN env var."
  elif [ "$HTTP_CODE" = "000" ]; then
    echo "         ✗ Connection failed — network/proxy/firewall issue."
  else
    echo "         ✗ HTTP $HTTP_CODE"
  fi

  # Test direct download
  echo "   [3/4] Direct download redirect (https://github.com/$REPO/releases/latest/download/$BINARY)..."
  DL_CODE=$(curl -s -o /dev/null -w "%{http_code}" -L --max-redirs 5 --max-time 10 "https://github.com/$REPO/releases/latest/download/$BINARY" -r 0-0 2>/dev/null || echo "000")
  if [ "$DL_CODE" = "200" ] || [ "$DL_CODE" = "206" ]; then
    echo "         ✓ Download available (HTTP $DL_CODE)"
  else
    echo "         ✗ HTTP $DL_CODE — download may be blocked."
  fi

  # Test proxy
  echo "   [4/4] Proxy configuration..."
  if [ -n "$HTTPS_PROXY" ] || [ -n "$https_proxy" ]; then
    echo "         ⚠ Proxy detected: ${HTTPS_PROXY:-$https_proxy}"
  elif [ -n "$HTTP_PROXY" ] || [ -n "$http_proxy" ]; then
    echo "         ⚠ HTTP proxy: ${HTTP_PROXY:-$http_proxy}"
  else
    echo "         ✓ No proxy configured"
  fi

  echo ""
  echo "   Done. Share this output when reporting install issues."
  echo ""
  exit 0
fi

echo ""
echo "   Installing Koda..."
echo "   OS: $OS  Arch: $ARCH"
echo ""

# Find latest release tag — try API first, fall back to direct download
TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -n "$TAG" ]; then
  URL="https://github.com/$REPO/releases/download/$TAG/$BINARY"
  echo "   Version: $TAG"
else
  echo "   ⚠ GitHub API unavailable — falling back to direct download..."
  URL="https://github.com/$REPO/releases/latest/download/$BINARY"
  TAG="latest"
  echo "   Version: $TAG (will resolve on download)"
fi
echo ""

# Download
mkdir -p "$INSTALL_DIR"
TMP=$(mktemp)
trap "rm -f $TMP" EXIT

if ! curl -fsSL -o "$TMP" "$URL"; then
  echo ""
  echo "   ✗ Download failed."
  echo ""
  echo "   Troubleshooting:"
  echo "     1. Run with --diagnose for connectivity test:"
  echo "        curl -fsSL https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.sh | bash -s -- --diagnose"
  echo ""
  echo "     2. Try direct download in browser:"
  echo "        https://github.com/$REPO/releases/latest/download/$BINARY"
  echo ""
  echo "     3. If behind a proxy:"
  echo "        export HTTPS_PROXY=http://your-proxy:port"
  echo ""
  exit 1
fi

chmod +x "$TMP"
mv "$TMP" "$INSTALL_DIR/koda"

# Verify
if [ -x "$INSTALL_DIR/koda" ]; then
  echo "   ✓ Installed: $INSTALL_DIR/koda"
  "$INSTALL_DIR/koda" version 2>/dev/null || true
  echo ""
  if ! echo "$PATH" | tr ':' '\n' | grep -q "$INSTALL_DIR"; then
    echo "   Add to PATH:"
    echo "     export PATH=\"$INSTALL_DIR:\$PATH\""
    echo ""
  fi
else
  echo "   ✗ Installation failed."
  exit 1
fi

# Optional: install Mouseketool
if [[ " $* " == *"--with-mouseketool"* ]]; then
  echo "   📦 Installing Mouseketool..."
  "$INSTALL_DIR/koda" apps install mouseketool
fi
