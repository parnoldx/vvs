#!/bin/sh
# Install the latest vvs release binary from GitHub.
set -eu

REPO=parnoldx/vvs
DEST=${VVS_INSTALL_DIR:-$HOME/.local/bin}

os=$(uname -s) arch=$(uname -m)
case "$os" in Linux) os=linux ;; Darwin) os=darwin ;; *) echo "unsupported OS: $os" >&2; exit 1 ;; esac
case "$arch" in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) echo "unsupported arch: $arch" >&2; exit 1 ;; esac

asset="vvs-${os}-${arch}.tar.gz"
url="https://github.com/${REPO}/releases/latest/download/${asset}"

echo "Downloading ${url}"
mkdir -p "$DEST"
curl -fsSL "$url" | tar -xz -C "$DEST" vvs
echo "Installed to ${DEST}/vvs"
"$DEST/vvs" --version

case ":$PATH:" in *":$DEST:"*) ;; *) echo "Note: add ${DEST} to your PATH" ;; esac
