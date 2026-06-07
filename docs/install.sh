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

# 2. Get GitHub Token for Private Repo
if [ -z "$GITHUB_TOKEN" ]; then
    read -sp "🔑 This is a private tool. Please enter your GitHub Personal Access Token: " GITHUB_TOKEN < /dev/tty
    echo ""
fi

# 3. Download from Private Release
BINARY_NAME="hexyn-aws_${OS}_${ARCH}.tar.gz"
echo "📥 Fetching latest binary from $CORE_REPO..."
ASSET_INFO=$(curl -s -H "Authorization: Bearer $GITHUB_TOKEN" "https://api.github.com/repos/$CORE_REPO/releases/latest")
ASSET_ID=$(echo "$ASSET_INFO" | grep -B 20 "$BINARY_NAME" | grep "\"id\":" | head -n 1 | awk '{print $2}' | sed 's/[,"]//g')

if [ -z "$ASSET_ID" ]; then
    echo "❌ Could not find binary $BINARY_NAME in latest release. Check your token and repo name."
    exit 1
fi

curl -L -H "Authorization: Bearer $GITHUB_TOKEN" -H "Accept: application/octet-stream" \
    "https://api.github.com/repos/$CORE_REPO/releases/assets/$ASSET_ID" -o hexyn-aws.tar.gz

# 4. Install
tar -xzf hexyn-aws.tar.gz hexyn-aws
sudo mv hexyn-aws "$INSTALL_PATH"
sudo chmod +x "$INSTALL_PATH"
rm hexyn-aws.tar.gz

echo "✅ Success! Type 'hexyn-aws' to start."
