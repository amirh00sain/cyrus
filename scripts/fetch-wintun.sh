#!/usr/bin/env bash
# Downloads wintun.dll (https://www.wintun.net/) into core/wintun/ so that it is
# embedded into the Windows executable. Run before building for Windows.
set -euo pipefail
VERSION="${WINTUN_VERSION:-0.14.1}"
DEST="$(cd "$(dirname "$0")/.." && pwd)/core/wintun"
TMP="$(mktemp -d)"
mkdir -p "$DEST"
curl -fsSL -o "$TMP/wintun.zip" "https://www.wintun.net/builds/wintun-${VERSION}.zip"
sha256sum "$TMP/wintun.zip"
unzip -q "$TMP/wintun.zip" -d "$TMP"
cp "$TMP/wintun/bin/amd64/wintun.dll" "$DEST/wintun_amd64.dll"
cp "$TMP/wintun/bin/x86/wintun.dll"   "$DEST/wintun_386.dll"
ls -l "$DEST"
