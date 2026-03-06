#!/usr/bin/env bash
set -e

REPO="rishi-anand/claude-watch"
INSTALL_DIR="${CLAUDE_WATCH_INSTALL_DIR:-$HOME/.local/bin}"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux)  os="linux" ;;
  Darwin) os="darwin" ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

ASSET="claude-watch-${os}-${arch}"
VERSION="${CLAUDE_WATCH_VERSION:-latest}"

if [ "$VERSION" = "latest" ]; then
  DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
fi

echo "Detected: ${os}/${arch}"
echo "Downloading claude-watch from ${DOWNLOAD_URL}"

mkdir -p "$INSTALL_DIR"
curl -fsSL "$DOWNLOAD_URL" -o "$INSTALL_DIR/claude-watch"
chmod +x "$INSTALL_DIR/claude-watch"

echo ""
echo "Installed to: $INSTALL_DIR/claude-watch"

echo ""
echo "To make claude-watch available in your shell, add $INSTALL_DIR to your PATH."
echo ""
echo "  For zsh:"
echo "    echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.zshrc && source ~/.zshrc"
echo ""
echo "  For bash:"
echo "    echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
echo ""
echo "Then run: claude-watch serve"
