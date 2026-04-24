#!/usr/bin/env bash
set -euo pipefail

LICENSE_KEY="${MAXMIND_LICENSE_KEY:-}"

if [[ -z "$LICENSE_KEY" ]]; then
  echo "Error: MAXMIND_LICENSE_KEY is not set" >&2
  echo "Usage: MAXMIND_LICENSE_KEY=your_key $0" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="$SCRIPT_DIR/../data"
TMP_DIR="$(mktemp -d)"

cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

mkdir -p "$DATA_DIR"

download_db() {
  local edition="$1"
  local url="https://download.maxmind.com/app/geoip_download?edition_id=${edition}&license_key=${LICENSE_KEY}&suffix=tar.gz"

  echo "Downloading ${edition}..."
  curl -fsSL "$url" -o "$TMP_DIR/${edition}.tar.gz"

  echo "Extracting ${edition}..."
  tar -xzf "$TMP_DIR/${edition}.tar.gz" -C "$TMP_DIR"

  local mmdb
  mmdb="$(find "$TMP_DIR" -name "${edition}.mmdb" | head -1)"

  if [[ -z "$mmdb" ]]; then
    echo "Error: ${edition}.mmdb not found in archive" >&2
    exit 1
  fi

  cp "$mmdb" "$DATA_DIR/${edition}.mmdb"
  echo "Saved to data/${edition}.mmdb"
}

download_db "GeoLite2-City"
download_db "GeoLite2-ASN"

echo "Done."
