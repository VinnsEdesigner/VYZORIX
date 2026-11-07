#!/usr/bin/env sh
#
# Publish an APK: stage it for serving and write the version manifest the device
# checks against.
#
# Usage:
#   tooling/scripts/generate_version.sh <apk-file> <version> <version-code> <release-notes>
#
# Output paths are the ones the running server reads, not a separate copy:
#   $DATA_DIR/version.json   the manifest /v1/version and /v1/check-update serve
#   $BIN_DIR/<apk>           the APK /v1/apk/<name> serves
#
# Those default to apps/api/data and apps/api/bin, which match the server's own
# defaults (VYZORIX_API_DIR=./data, VYZORIX_BIN_DIR=./bin, relative to apps/api).
# Override with DATA_DIR / BIN_DIR when the server runs elsewhere. An earlier
# revision wrote `api/v1/version.json` and `bin/`, which the server never read — so
# the served manifest stayed a stub while this script reported success. The paths
# are the whole point of the script, hence the explicit defaults below.
#
# The manifest is what makes a download trustworthy: it carries the SHA-256 the
# device verifies the APK against, so the hash is computed here from the file being
# published rather than taken on trust.

set -eu

if [ "$#" -lt 4 ]; then
  echo "usage: $0 <apk-file> <version> <version-code> <release-notes>" >&2
  exit 2
fi

apk="$1"; version="$2"; code="$3"; notes="$4"

data_dir="${DATA_DIR:-apps/api/data}"
bin_dir="${BIN_DIR:-apps/api/bin}"

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)

# Validate before anything is written, so a bad APK leaves the published manifest
# untouched rather than pointing at a file that was never staged.
"$script_dir/validate_apk.sh" "$apk" >/dev/null

name=$(basename "$apk")
sha=$("$script_dir/compute_checksum.sh" "$apk")
size=$(wc -c < "$apk" | tr -d ' ')

mkdir -p "$repo_root/$bin_dir"
cp "$apk" "$repo_root/$bin_dir/$name"

mkdir -p "$repo_root/$data_dir"
cat > "$repo_root/$data_dir/version.json" <<JSON
{
  "version": "$version",
  "version_code": $code,
  "apk_filename": "$name",
  "apk_sha256": "$sha",
  "apk_size_bytes": $size,
  "release_notes": "$notes"
}
JSON

# Keep only the most recent APKs so the bin directory does not grow without bound.
"$script_dir/cleanup_old_apks.sh" 3 "$repo_root/$bin_dir"

echo "published $name ($version, code $code, $size bytes)"
echo "  manifest: $data_dir/version.json"
echo "  apk:      $bin_dir/$name"
echo "  sha256:   $sha"
