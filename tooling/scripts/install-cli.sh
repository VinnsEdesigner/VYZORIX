#!/usr/bin/env bash
#
# Builds and installs the vyzorix CLI.
#
# The CLI is not installable with `go install module@latest`: its module path
# (github.com/VinnsEdesigner/vyzorix) does not resolve, because the repository is
# private and named vyzorix-update-server. So the CLI is built from a clone of
# this repo and the binary is copied onto PATH.
#
# Usage:
#   bash tooling/scripts/install-cli.sh            # install to ~/.local/bin
#   PREFIX=/usr/local/bin bash tooling/scripts/install-cli.sh
#
set -euo pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)
PREFIX=${PREFIX:-"$HOME/.local/bin"}

mkdir -p "$PREFIX"

# sqlite3 is CGO, so the build needs CGO enabled; the API module the CLI imports
# links the same driver.
cd "$REPO_ROOT/apps/cli"
CGO_ENABLED=1 go build -o "$PREFIX/vyzorix" .

echo "Installed vyzorix to $PREFIX/vyzorix"
echo ""
echo "Next:"
echo "  vyzorix login --server https://your-server --api-key vxyz_..."
echo "  vyzorix whoami"

case ":$PATH:" in
  *":$PREFIX:"*) ;;
  *)
    echo ""
    echo "Warning: $PREFIX is not on PATH. Add it with:"
    echo "  export PATH=\"$PREFIX:\$PATH\""
    ;;
esac
