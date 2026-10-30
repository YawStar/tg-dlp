#!/usr/bin/env bash
#
# Install tg-dlp on Linux or macOS.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/YawStar/tg-dlp/main/scripts/install.sh | bash
#
# Environment overrides:
#   TGDLP_VERSION    release version (default: latest)
#   TGDLP_INSTALL_DIR install directory (default: /usr/local/bin)
#   TGDLP_REPO       GitHub repo (default: YawStar/tg-dlp)

set -euo pipefail

REPO="${TGDLP_REPO:-YawStar/tg-dlp}"
BINARY="tg-dlp"
VERSION="${TGDLP_VERSION:-latest}"
INSTALL_DIR="${TGDLP_INSTALL_DIR:-/usr/local/bin}"

# ---- Detect OS / arch ----
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    linux)   ;;
    darwin)  ;;
    *)
        echo "❌ Unsupported OS: $OS" >&2
        exit 1
        ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)
        echo "❌ Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

# ---- Resolve latest version ----
if [[ "$VERSION" == "latest" ]]; then
    echo "🔍 Fetching latest release..."
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep -E '"tag_name":' \
        | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
    if [[ -z "$VERSION" ]]; then
        echo "❌ Could not determine latest version." >&2
        exit 1
    fi
fi

VERSION_CLEAN="${VERSION#v}"
ARCHIVE="${BINARY}_${VERSION_CLEAN}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

echo "📦 Downloading ${ARCHIVE}..."
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fsSL "$URL" -o "${TMP_DIR}/${ARCHIVE}"

echo "📂 Extracting..."
tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$TMP_DIR"

BIN_PATH="$(find "$TMP_DIR" -type f -name "$BINARY" | head -n1)"
if [[ -z "$BIN_PATH" ]]; then
    echo "❌ Could not find $BINARY in the archive." >&2
    exit 1
fi

# ---- Install ----
if [[ ! -d "$INSTALL_DIR" ]]; then
    echo "📁 Creating $INSTALL_DIR..."
    mkdir -p "$INSTALL_DIR" 2>/dev/null || sudo mkdir -p "$INSTALL_DIR"
fi

if [[ -w "$INSTALL_DIR" ]]; then
    install -m 0755 "$BIN_PATH" "${INSTALL_DIR}/${BINARY}"
else
    echo "🔐 Requesting sudo to install to $INSTALL_DIR..."
    sudo install -m 0755 "$BIN_PATH" "${INSTALL_DIR}/${BINARY}"
fi

echo "✅ Installed to ${INSTALL_DIR}/${BINARY}"

# ---- Verify ----
if command -v "$BINARY" >/dev/null 2>&1; then
    "$BINARY" version
else
    echo "⚠️  $INSTALL_DIR is not in your PATH." >&2
    echo "    Add it with: export PATH=\"$INSTALL_DIR:\$PATH\""
fi