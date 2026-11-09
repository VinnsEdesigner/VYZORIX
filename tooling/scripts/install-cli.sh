#!/usr/bin/env bash
#
# Installs the vyzorix CLI from a GitHub release.
#
# The repository is private, so the release assets are not world-readable and the
# usual `curl -fsSL https://.../install.sh | sh` one-liner cannot work: an
# anonymous fetch gets a 404, and `go install module@latest` cannot resolve the
# module path at all. An authenticated download is the standard answer for a
# private repo, the same way a private npm package or a private Go module is
# consumed. This script uses, in order of preference:
#
#   1. `gh` if it is installed and authenticated (it handles the token for you),
#   2. a token in $GITHUB_TOKEN or $GH_TOKEN with curl.
#
# The download is verified against the release's SHA256SUMS before it is moved
# onto PATH, so a corrupted or swapped asset is refused rather than installed.
#
# Usage:
#   bash install-cli.sh                      # latest release, install to ~/.local/bin
#   VYZORIX_VERSION=v0.1.0 bash install-cli.sh
#   PREFIX=/usr/local/bin bash install-cli.sh
#
# From a clone of this repo, with no token at all, use the local build instead:
#   bash tooling/scripts/install-cli.sh
#
set -euo pipefail

REPO=${VYZORIX_REPO:-VinnsEdesigner/vyzorix-update-server}
PREFIX=${PREFIX:-"$HOME/.local/bin"}
VERSION=${VYZORIX_VERSION:-}

die() {
  echo "install: $*" >&2
  exit 1
}

# --- platform ---------------------------------------------------------------

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux) os=linux ;;
  darwin) os=darwin ;;
  *) die "unsupported OS '$os'; build from source with tooling/scripts/install-cli.sh" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) die "unsupported architecture '$arch'" ;;
esac

asset="vyzorix-$os-$arch"

# --- credentials ------------------------------------------------------------

# A token is required: the release is private. `gh` is preferred because it
# already holds the token and reads it from the keychain rather than the
# environment.
have_gh=0
if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
  have_gh=1
fi

token=${GITHUB_TOKEN:-${GH_TOKEN:-}}
if [ "$have_gh" -eq 0 ] && [ -z "$token" ]; then
  die "this repository is private, so a credential is required.
  Either authenticate gh:   gh auth login
  Or set a token:           GITHUB_TOKEN=... bash install-cli.sh
  Or build from a clone:    bash tooling/scripts/install-cli.sh"
fi

# --- download ---------------------------------------------------------------

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT

if [ "$have_gh" -eq 1 ]; then
  # `gh` resolves "latest" and handles the token; the tag is echoed back so the
  # version reported to the user is the one actually downloaded.
  if [ -n "$VERSION" ]; then
    gh release download "$VERSION" --repo "$REPO" --pattern "$asset" --pattern SHA256SUMS --dir "$workdir" --clobber
  else
    gh release download --repo "$REPO" --pattern "$asset" --pattern SHA256SUMS --dir "$workdir" --clobber
  fi
else
  api="https://api.github.com/repos/$REPO/releases"
  if [ -n "$VERSION" ]; then
    base="$api/tags/$VERSION"
  else
    base="$api/latest"
  fi

  command -v python3 >/dev/null 2>&1 || die "python3 is required for the token-based download; or authenticate gh: gh auth login"

  # Resolve the release once, then read the asset ids out of it. The asset API
  # returns the bytes directly for this Accept header, which is what makes an
  # authenticated private download work: following the browser_download_url
  # redirect would drop the Authorization header and land on a 404.
  release_json="$workdir/release.json"
  curl -fsSL -H "Authorization: Bearer $token" "$base" -o "$release_json" \
    || die "could not read the release metadata; is the token valid for $REPO?"

  download_named() {
    local name="$1" out="$2" id
    id=$(python3 -c '
import json, sys
name = sys.argv[1]
for a in json.load(open(sys.argv[2])).get("assets", []):
    if a["name"] == name:
        print(a["id"]); break
' "$name" "$release_json")
    [ -n "$id" ] || die "release does not contain asset '$name'"
    curl -fsSL -H "Authorization: Bearer $token" \
      -H "Accept: application/octet-stream" \
      "$api/assets/$id" -o "$out"
  }

  download_named "$asset" "$workdir/$asset"
  download_named SHA256SUMS "$workdir/SHA256SUMS"
fi

# --- verify -----------------------------------------------------------------

[ -f "$workdir/$asset" ] || die "download did not produce '$asset'"

(
  cd "$workdir"
  # sha256sum on Linux, shasum on macOS.
  if command -v sha256sum >/dev/null 2>&1; then
    grep " $asset\$" SHA256SUMS | sha256sum -c -
  else
    grep " $asset\$" SHA256SUMS | shasum -a 256 -c -
  fi
) || die "checksum verification failed; the download was refused"

# --- install ----------------------------------------------------------------

mkdir -p "$PREFIX"
install -m 0755 "$workdir/$asset" "$PREFIX/vyzorix"

echo "Installed vyzorix to $PREFIX/vyzorix"
"$PREFIX/vyzorix" version || true

case ":$PATH:" in
  *":$PREFIX:"*) ;;
  *)
    echo ""
    echo "Warning: $PREFIX is not on PATH. Add it with:"
    echo "  export PATH=\"$PREFIX:\$PATH\""
    ;;
esac
