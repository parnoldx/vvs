#!/bin/sh
# Install the latest vvs release binary from GitHub.
set -eu

REPO=parnoldx/vvs
DEST=${VVS_INSTALL_DIR:-$HOME/.local/bin}

os=$(uname -s) arch=$(uname -m)
case "$os" in Linux) os=linux ;; Darwin) os=darwin ;; MINGW*|MSYS*|CYGWIN*) os=windows ;; *) echo "unsupported OS: $os" >&2; exit 1 ;; esac
case "$arch" in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) echo "unsupported arch: $arch" >&2; exit 1 ;; esac

tag=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
[ -n "$tag" ] || { echo "could not determine latest release" >&2; exit 1; }

asset="vvs-${os}-${arch}.tar.gz"
bin=vvs; [ "$os" = windows ] && bin=vvs.exe
url="https://github.com/${REPO}/releases/download/${tag}/${asset}"

echo "Downloading vvs ${tag} for ${os}/${arch}"
mkdir -p "$DEST"
curl -fsSL "$url" | tar -xz -C "$DEST" "$bin"
echo "Installed to ${DEST}/vvs"
"$DEST/$bin" --version

case ":$PATH:" in *":$DEST:"*) ;; *) echo "Note: add ${DEST} to your PATH" ;; esac
