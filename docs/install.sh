#!/bin/bash
set -e

# Configuration
CORE_REPO="flockyn/hexyn-aws"
INSTALL_PATH="/usr/local/bin/hexyn-aws"

echo "🚀 Starting Hexyn AWS installation..."

# 1. Detect OS and Architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "❌ Unsupported architecture: $ARCH"; exit 1 ;;
esac

# 2. Resolve the matching asset from the latest public release
BINARY_NAME="hexyn-aws_${OS}_${ARCH}.tar.gz"
echo "📥 Fetching latest binary ($BINARY_NAME) from $CORE_REPO..."
DOWNLOAD_URL=$(curl -fsSL "https://api.github.com/repos/$CORE_REPO/releases/latest" \
    | grep "browser_download_url" \
    | grep "$BINARY_NAME" \
    | head -n 1 \
    | cut -d '"' -f 4)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "❌ Could not find $BINARY_NAME in the latest release."
    exit 1
fi

# 3. Download and install
curl -fsSL "$DOWNLOAD_URL" -o hexyn-aws.tar.gz
tar -xzf hexyn-aws.tar.gz hexyn-aws
sudo mv hexyn-aws "$INSTALL_PATH"
sudo chmod +x "$INSTALL_PATH"
rm hexyn-aws.tar.gz

echo "✅ Success! Type 'hexyn-aws' to start."
