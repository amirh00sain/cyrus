#!/usr/bin/env bash
# Downloads the IPv4 ranges registered in Iran into core/irdata/ir-cidrs.txt so
# that they are embedded into the build. Used by TUN mode on Linux/Windows to
# keep Iranian sites outside the tunnel. A failed download is not fatal: the
# build just ships without the list.
set -uo pipefail
DEST="$(cd "$(dirname "$0")/.." && pwd)/core/irdata/ir-cidrs.txt"
mkdir -p "$(dirname "$DEST")"
TMP="$(mktemp)"

try() {
  if curl -fsSL --retry 3 --max-time 60 -o "$TMP" "$1" && grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/[0-9]+' "$TMP"; then
    grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/[0-9]+' "$TMP" > "$DEST"
    echo "Iran IP list: $(wc -l < "$DEST") ranges from $1"
    return 0
  fi
  return 1
}

try "https://www.ipdeny.com/ipblocks/data/aggregated/ir-aggregated.zone" \
  || try "https://raw.githubusercontent.com/ipverse/rir-ip/master/country/ir/ipv4-aggregated.txt" \
  || echo "WARNING: could not download the Iran IP list; building without it"
exit 0
