#!/usr/bin/env bash
#
# Start the local git server in the background, seeding the demo repository on first run.
#
# Kept separate from tooling/dev/supervise.sh on purpose: the supervisor owns exactly two
# children (api, proxy) with per-child backoff and crash-loop accounting, and threading a
# third through that is a change to the thing every other dev workflow depends on. This is a
# test fixture, not a service.
#
#   tooling/dev/git-server/start.sh          # start (seeds if missing)
#   tooling/dev/git-server/start.sh --reseed # recreate the demo content, then start
#   tooling/dev/git-server/start.sh stop
#
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT="${GIT_PORT:-8088}"
PIDFILE="$HERE/.server.pid"
LOG="$HERE/.server.log"

stop() {
  if [ -f "$PIDFILE" ]; then
    local pid
    pid="$(cat "$PIDFILE")"
    if kill -0 "$pid" 2>/dev/null; then
      kill -TERM "$pid" 2>/dev/null
      sleep 1
      kill -0 "$pid" 2>/dev/null && kill -KILL "$pid" 2>/dev/null
      echo "stopped git server (pid $pid)"
    fi
    rm -f "$PIDFILE"
  fi
}

case "${1:-start}" in
  stop) stop; exit 0 ;;
esac

stop

if [ "${1:-}" = "--reseed" ] || [ ! -d "$HERE/repos/device-config.git" ]; then
  python3 "$HERE/serve.py" --seed
fi

nohup python3 "$HERE/serve.py" --port "$PORT" >>"$LOG" 2>&1 &
echo $! > "$PIDFILE"
sleep 1

if ! kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "git server failed to start; see $LOG" >&2
  exit 1
fi

echo "git server pid $(cat "$PIDFILE") on http://127.0.0.1:$PORT/"
echo "  http://127.0.0.1:$PORT/device-config.git"
