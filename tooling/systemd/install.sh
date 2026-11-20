#!/usr/bin/env bash
#
# Install the Vyzorix API and proxy as systemd services.
#
#   sudo bash tooling/systemd/install.sh
#
# Override with environment variables, not flags:
#   sudo PREFIX=/opt/vyzorix STATE_DIR=/var/lib/vyzorix \
#        ETC_DIR=/etc/vyzorix SERVICE_USER=vyzorix \
#        API_PORT=3000 PROXY_PORT=12000 \
#        bash tooling/systemd/install.sh
#
# This script does the parts systemd cannot do for itself: create the service
# account, lay out the directories, and refuse to start with placeholder secrets.
# It never overwrites an existing env file — a re-run keeps the live secrets.

set -euo pipefail

PREFIX="${PREFIX:-/opt/vyzorix}"
STATE_DIR="${STATE_DIR:-/var/lib/vyzorix}"
ETC_DIR="${ETC_DIR:-/etc/vyzorix}"
SERVICE_USER="${SERVICE_USER:-vyzorix}"
API_PORT="${API_PORT:-3000}"
PROXY_PORT="${PROXY_PORT:-12000}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
UNIT_SRC="$REPO_ROOT/tooling/systemd"

say()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m warn\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31merror\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "run as root (sudo bash tooling/systemd/install.sh)"

command -v systemctl >/dev/null || die "systemctl not found; this host does not use systemd"
[ -d /run/systemd/system ] || die "systemd is not the running init system on this host"

# --- service account -------------------------------------------------------
# A system account with no shell and no home: it exists only to own the process.
if ! id "$SERVICE_USER" >/dev/null 2>&1; then
  say "creating service account $SERVICE_USER"
  useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER"
else
  say "service account $SERVICE_USER already exists"
fi

# --- directories -----------------------------------------------------------
say "creating directories"
install -d -o root -g root -m 0755 "$PREFIX" "$PREFIX/bin"
install -d -o root -g root -m 0755 "$ETC_DIR"
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0750 "$STATE_DIR"
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0750 "$STATE_DIR/avatars"
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0750 "$STATE_DIR/proxy"

# --- env files -------------------------------------------------------------
# Installed once, then owned by the operator. Re-running must not clobber live
# secrets, so an existing file is left alone.
install_env() {
  local name="$1"
  local target="$ETC_DIR/$name"

  if [ -f "$target" ]; then
    say "$target exists; leaving it untouched"
    return
  fi

  install -o root -g "$SERVICE_USER" -m 0640 "$UNIT_SRC/$name.example" "$target"
  say "wrote $target from the example"
}

install_env api.env
install_env proxy.env

# --- placeholder guard -----------------------------------------------------
# A unit that starts with CHANGE_ME secrets is worse than one that does not
# start: it looks healthy while signing with a value that is in the repository.
if grep -q 'CHANGE_ME' "$ETC_DIR/api.env"; then
  warn "$ETC_DIR/api.env still contains CHANGE_ME."
  warn "Generate real values first: openssl rand -hex 32"
  warn "The services will be installed but left stopped."
  SECRETS_READY=0
else
  SECRETS_READY=1
fi

# --- binaries --------------------------------------------------------------
say "building the API binary"
if command -v go >/dev/null; then
  ( cd "$REPO_ROOT/apps/api" && go build -o "$PREFIX/bin/vyzorix-api" ./cmd/api )
  chmod 0755 "$PREFIX/bin/vyzorix-api"
else
  warn "go not found; copy a prebuilt binary to $PREFIX/bin/vyzorix-api"
fi

# --- web build -------------------------------------------------------------
# The proxy serves dist/client and loads dist/server/server.js. Without it the
# proxy still boots but serves no app, so say so plainly.
if [ -f "$REPO_ROOT/apps/VyzoriX_web/dist/server/server.js" ]; then
  say "web build present"
else
  warn "no web build at apps/VyzoriX_web/dist — run: (cd apps/VyzoriX_web && npx vite build)"
fi

# --- units -----------------------------------------------------------------
say "installing units"
install -o root -g root -m 0644 "$UNIT_SRC/vyzorix-api.service" /etc/systemd/system/vyzorix-api.service
install -o root -g root -m 0644 "$UNIT_SRC/vyzorix-proxy.service" /etc/systemd/system/vyzorix-proxy.service
systemctl daemon-reload

if [ "$SECRETS_READY" -eq 1 ]; then
  say "enabling and starting"
  systemctl enable --now vyzorix-api.service
  systemctl enable --now vyzorix-proxy.service

  sleep 2
  systemctl --no-pager --lines=0 status vyzorix-api.service vyzorix-proxy.service || true

  say "health check"
  if curl -fsS --max-time 5 "http://127.0.0.1:${API_PORT}/health" >/dev/null; then
    say "API healthy"
  else
    warn "API did not answer /health yet; check: journalctl -u vyzorix-api -n 50"
  fi
else
  systemctl enable vyzorix-api.service vyzorix-proxy.service
fi

cat <<EOF

Installed.

  config   $ETC_DIR/api.env, $ETC_DIR/proxy.env
  data     $STATE_DIR
  binary   $PREFIX/bin/vyzorix-api
  logs     journalctl -u vyzorix-api -f
           journalctl -u vyzorix-proxy -f

  start    systemctl start vyzorix-api vyzorix-proxy
  stop     systemctl stop vyzorix-proxy vyzorix-api
  restart  systemctl restart vyzorix-proxy
  status   systemctl status vyzorix-api vyzorix-proxy

The proxy binds ${PROXY_PORT} on all interfaces; put TLS in front of it.
The API binds 127.0.0.1:${API_PORT} (BIND_ADDR in api.env) — keep it there.
EOF
