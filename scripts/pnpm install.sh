#!/usr/bin/env bash
#
# Install the workspace's pnpm dependencies, working around four environment
# traps that otherwise make the install fail in a way that looks like a hang.
#
#   1. NODE_ENV=production makes pnpm skip devDependencies entirely, so tsc,
#      vite and vitest are never linked. A prepare script that calls them then
#      dies with "sh: 1: tsc: not found".
#   2. --filter changes the dependency set and trips ERR_PNPM_INCLUDED_DEPS_CONFLICT.
#   3. CI=true makes pnpm default to --frozen-lockfile, so any package.json edit
#      trips ERR_PNPM_OUTDATED_LOCKFILE.
#   4. A prepare script calls bare `pnpm`, which is not on PATH where only
#      `corepack pnpm` exists.
#
# Usage: bash scripts/install.sh
#
set -euo pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT"

# Trap 1: devDependencies must be installed.
export NODE_ENV=development

# Keep Corepack from prompting, which stalls a non-interactive install.
export COREPACK_ENABLE_DOWNLOAD_PROMPT=0
export COREPACK_ENABLE_AUTOINSTALL=0
export PNPM_TELEMETRY=0

# Trap 4: give prepare scripts a `pnpm` to call. Only needed when pnpm is not
# already on PATH; the shim delegates to Corepack, which is.
SHIM_DIR=""
if ! command -v pnpm >/dev/null 2>&1; then
  SHIM_DIR=$(mktemp -d)
  printf '#!/bin/sh\nexec corepack pnpm "$@"\n' >"$SHIM_DIR/pnpm"
  chmod +x "$SHIM_DIR/pnpm"
  export PATH="$SHIM_DIR:$PATH"
  echo "pnpm not on PATH; using a Corepack shim in $SHIM_DIR"
  trap 'rm -rf "$SHIM_DIR"' EXIT
fi

# Trap 2: install at the root, never filtered.
# Trap 3: the lockfile must be allowed to update when a package.json changed.
echo "Installing dependencies (NODE_ENV=$NODE_ENV)..."
corepack pnpm install --no-frozen-lockfile "$@"

echo "Done."
