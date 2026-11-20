#!/usr/bin/env bash
#
# Supervise the Vyzorix API and proxy in an environment without systemd.
#
#   tooling/dev/supervise.sh start      # daemonise both, return when healthy
#   tooling/dev/supervise.sh stop       # stop both, children included
#   tooling/dev/supervise.sh restart
#   tooling/dev/supervise.sh status
#   tooling/dev/supervise.sh logs [api|proxy]
#   tooling/dev/supervise.sh run        # stay in the foreground (Ctrl-C stops)
#
# Why this exists: systemd is not the init system in a container (PID 1 is the
# agent), so the units in tooling/systemd cannot run here. This script is the
# local equivalent: it owns both children in one process tree, reaps them, and
# restarts on exit with backoff.
#
# What it does that `nohup cmd &` does not:
#   - one supervisor, two children: stopping the supervisor stops both
#   - waits for each child, so exited children are reaped (no zombies)
#   - restarts a crashed child with exponential backoff and a crash-loop guard
#   - refuses to start a second copy (flock on the pidfile)
#   - health-checks after start, and fails the start if a port never answers

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUN_DIR="${RUN_DIR:-$REPO_ROOT/.local-dev}"
LOG_DIR="${LOG_DIR:-$RUN_DIR}"

API_BIN="${API_BIN:-$RUN_DIR/bin/vyzorix-api}"
API_ENV="${API_ENV:-$RUN_DIR/api.env}"
API_PORT="${API_PORT:-3000}"

PROXY_DIR="${PROXY_DIR:-$REPO_ROOT/apps/proxy-server}"
PROXY_ENTRY="${PROXY_ENTRY:-$PROXY_DIR/vyzorix-proxy-server.cjs}"
PROXY_PORT="${PROXY_PORT:-12000}"

SUPERVISOR_PID="$RUN_DIR/supervisor.pid"
SUPERVISOR_LOG="$LOG_DIR/supervisor.log"
API_PID="$RUN_DIR/api.pid"
PROXY_PID="$RUN_DIR/proxy.pid"

# Backoff: start at 1s, double to a ceiling, then give up after CRASH_LIMIT
# restarts inside CRASH_WINDOW seconds. A crash loop is a bug to surface, not a
# condition to hide behind an infinite restart.
BACKOFF_START="${BACKOFF_START:-1}"
BACKOFF_MAX="${BACKOFF_MAX:-30}"
CRASH_LIMIT="${CRASH_LIMIT:-8}"
CRASH_WINDOW="${CRASH_WINDOW:-120}"

HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-20}"

say()  { printf '[supervise] %s\n' "$*"; }
warn() { printf '[supervise] warn: %s\n' "$*" >&2; }
die()  { printf '[supervise] error: %s\n' "$*" >&2; exit 1; }

now() { date +%s; }

# alive <pid> — true when the pid exists and is not a zombie.
alive() {
  local pid="$1"
  [ -n "$pid" ] || return 1
  kill -0 "$pid" 2>/dev/null || return 1

  local state
  state="$(awk '{print $3}' "/proc/$pid/stat" 2>/dev/null)" || return 1
  [ "$state" != "Z" ]
}

read_pid() {
  local file="$1"
  [ -f "$file" ] || return 1
  local pid
  pid="$(tr -d '[:space:]' < "$file")"
  [ -n "$pid" ] || return 1
  printf '%s' "$pid"
}

# port_open <port> — true when something accepts a TCP connection there.
port_open() {
  local port="$1"
  (exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null
}

wait_healthy() {
  local port="$1" name="$2" deadline
  deadline=$(( $(now) + HEALTH_TIMEOUT ))
  while [ "$(now)" -lt "$deadline" ]; do
    if port_open "$port"; then
      say "$name healthy on :$port"
      return 0
    fi
    sleep 0.3
  done
  warn "$name did not open :$port within ${HEALTH_TIMEOUT}s"
  return 1
}

# --- supervisor loop -------------------------------------------------------
# Runs in the foreground; `start` daemonises this same function.

supervise() {
  local stopping=0
  local api_pid="" proxy_pid=""
  local api_fails=0 proxy_fails=0
  local api_backoff="$BACKOFF_START" proxy_backoff="$BACKOFF_START"

  on_signal() {
    stopping=1
    say "signal received; stopping children"
    # SIGTERM lets both binaries drain (Go signal.NotifyContext, proxy SIGTERM
    # handler). Escalate to KILL only if a child overruns the grace period.
    for pid in "$proxy_pid" "$api_pid"; do
      [ -n "$pid" ] && alive "$pid" && kill -TERM "$pid" 2>/dev/null
    done
  }

  trap on_signal TERM INT

  start_api() {
    if [ ! -x "$API_BIN" ]; then
      warn "API binary missing or not executable: $API_BIN"
      return 1
    fi
    if [ ! -f "$API_ENV" ]; then
      warn "API env missing: $API_ENV"
      return 1
    fi

    # Load the env file in a subshell, then exec so the pid we record IS the
    # server: a `bash -c` wrapper would make $! the wrapper, and killing it
    # would leave the real server orphaned.
    (
      set -a
      # shellcheck disable=SC1090  # path is configuration, not a literal
      . "$API_ENV"
      set +a
      exec "$API_BIN"
    ) >> "$LOG_DIR/api.log" 2>&1 &
    api_pid=$!
    printf '%s' "$api_pid" > "$API_PID"
    say "api started pid=$api_pid"
  }

  start_proxy() {
    if [ ! -f "$PROXY_ENTRY" ]; then
      warn "proxy entry missing: $PROXY_ENTRY"
      return 1
    fi

    (
      cd "$PROXY_DIR" || exit 1
      PROXY_PORT="$PROXY_PORT" \
      API_TARGET="http://127.0.0.1:$API_PORT" \
      NODE_ENV="${NODE_ENV:-development}" \
      exec node "$PROXY_ENTRY"
    ) >> "$LOG_DIR/proxy.log" 2>&1 &
    proxy_pid=$!
    printf '%s' "$proxy_pid" > "$PROXY_PID"
    say "proxy started pid=$proxy_pid"
  }

  # Restart accounting: a crash inside the window counts; a child that ran
  # longer than the window resets its counter, so an occasional restart never
  # trips the guard.
  note_exit() {
    local which="$1" started="$2" uptime
    uptime=$(( $(now) - started ))

    if [ "$which" = "api" ]; then
      if [ "$uptime" -ge "$CRASH_WINDOW" ]; then
        api_fails=0
        api_backoff="$BACKOFF_START"
      fi
      api_fails=$(( api_fails + 1 ))
    else
      if [ "$uptime" -ge "$CRASH_WINDOW" ]; then
        proxy_fails=0
        proxy_backoff="$BACKOFF_START"
      fi
      proxy_fails=$(( proxy_fails + 1 ))
    fi
  }

  local api_started proxy_started

  say "supervisor pid=$$ starting children"
  start_api || return 1
  api_started=$(now)

  # The proxy needs the API's port, not the API's readiness: it proxies lazily
  # and serves the shell before the API answers.
  wait_healthy "$API_PORT" api >/dev/null 2>&1 || warn "api slow to bind :$API_PORT; starting proxy anyway"
  start_proxy || return 1
  proxy_started=$(now)

  while [ "$stopping" -eq 0 ]; do
    sleep 1
    [ "$stopping" -eq 1 ] && break

    if ! alive "$api_pid"; then
      wait "$api_pid" 2>/dev/null
      note_exit api "$api_started"
      say "api exited (fail $api_fails)"

      if [ "$api_fails" -ge "$CRASH_LIMIT" ]; then
        warn "api crashed $api_fails times inside ${CRASH_WINDOW}s; giving up"
        stopping=1
        break
      fi
      say "restarting api in ${api_backoff}s"
      sleep "$api_backoff"
      api_backoff=$(( api_backoff * 2 ))
      [ "$api_backoff" -gt "$BACKOFF_MAX" ] && api_backoff="$BACKOFF_MAX"
      start_api && api_started=$(now)
    fi

    if ! alive "$proxy_pid"; then
      wait "$proxy_pid" 2>/dev/null
      note_exit proxy "$proxy_started"
      say "proxy exited (fail $proxy_fails)"

      if [ "$proxy_fails" -ge "$CRASH_LIMIT" ]; then
        warn "proxy crashed $proxy_fails times inside ${CRASH_WINDOW}s; giving up"
        stopping=1
        break
      fi
      say "restarting proxy in ${proxy_backoff}s"
      sleep "$proxy_backoff"
      proxy_backoff=$(( proxy_backoff * 2 ))
      [ "$proxy_backoff" -gt "$BACKOFF_MAX" ] && proxy_backoff="$BACKOFF_MAX"
      start_proxy && proxy_started=$(now)
    fi
  done

  # Final shutdown: give each child the grace period, then insist.
  say "stopping children"
  for pid in "$proxy_pid" "$api_pid"; do
    [ -n "$pid" ] && alive "$pid" && kill -TERM "$pid" 2>/dev/null
  done

  local deadline
  deadline=$(( $(now) + 10 ))
  while [ "$(now)" -lt "$deadline" ]; do
    local left=0
    for pid in "$proxy_pid" "$api_pid"; do
      alive "$pid" && left=1
    done
    [ "$left" -eq 0 ] && break
    sleep 0.3
  done

  for pid in "$proxy_pid" "$api_pid"; do
    if [ -n "$pid" ] && alive "$pid"; then
      warn "pid $pid ignored SIGTERM; sending SIGKILL"
      kill -KILL "$pid" 2>/dev/null
    fi
  done

  # Reap whatever is left so nothing is left as a zombie.
  for pid in "$api_pid" "$proxy_pid"; do
    [ -n "$pid" ] && wait "$pid" 2>/dev/null
  done

  rm -f "$API_PID" "$PROXY_PID" "$SUPERVISOR_PID"
  say "stopped"
}

# --- commands --------------------------------------------------------------

cmd_start() {
  mkdir -p "$RUN_DIR"

  local existing
  if existing="$(read_pid "$SUPERVISOR_PID")" && alive "$existing"; then
    say "already running (supervisor pid=$existing)"
    cmd_status
    return 0
  fi

  # Hold the lock for the supervisor's lifetime so a second `start` cannot race.
  exec 9>"$RUN_DIR/supervisor.lock"
  if ! flock -n 9; then
    die "another supervisor holds the lock"
  fi

  say "starting supervisor (logs: $SUPERVISOR_LOG)"
  setsid nohup "$0" run >> "$SUPERVISOR_LOG" 2>&1 < /dev/null &
  local pid=$!
  printf '%s' "$pid" > "$SUPERVISOR_PID"

  wait_healthy "$API_PORT" api || die "api failed to start; see $LOG_DIR/api.log"
  wait_healthy "$PROXY_PORT" proxy || die "proxy failed to start; see $LOG_DIR/proxy.log"

  say "up (supervisor pid=$pid)"
}

cmd_stop() {
  local pid
  if ! pid="$(read_pid "$SUPERVISOR_PID")"; then
    warn "no pidfile at $SUPERVISOR_PID; nothing to stop"
    return 0
  fi

  if ! alive "$pid"; then
    warn "supervisor $pid is not running; clearing stale pidfile"
    rm -f "$SUPERVISOR_PID" "$API_PID" "$PROXY_PID"
    return 0
  fi

  say "stopping supervisor pid=$pid"
  kill -TERM "$pid" 2>/dev/null

  local deadline
  deadline=$(( $(now) + 20 ))
  while [ "$(now)" -lt "$deadline" ]; do
    alive "$pid" || { say "stopped"; return 0; }
    sleep 0.3
  done

  warn "supervisor did not exit; sending SIGKILL"
  kill -KILL "$pid" 2>/dev/null
  sleep 0.5

  # The supervisor reaps its own children, but if it was SIGKILLed it could not.
  # Clean up anything it left behind by pidfile.
  for file in "$PROXY_PID" "$API_PID"; do
    local child
    if child="$(read_pid "$file")" && alive "$child"; then
      warn "killing orphaned child $child"
      kill -TERM "$child" 2>/dev/null
      sleep 1
      alive "$child" && kill -KILL "$child" 2>/dev/null
    fi
  done

  rm -f "$SUPERVISOR_PID" "$API_PID" "$PROXY_PID"
}

cmd_status() {
  local sup api proxy state
  sup="$(read_pid "$SUPERVISOR_PID" || true)"
  api="$(read_pid "$API_PID" || true)"
  proxy="$(read_pid "$PROXY_PID" || true)"

  printf '%-12s %-8s %-6s %s\n' SERVICE PID PORT STATE
  printf '%-12s %-8s %-6s %s\n' supervisor "${sup:-—}" '—' "$(state_of "$sup" "")"
  printf '%-12s %-8s %-6s %s\n' api "${api:-—}" "$API_PORT" "$(state_of "$api" "$API_PORT")"
  printf '%-12s %-8s %-6s %s\n' proxy "${proxy:-—}" "$PROXY_PORT" "$(state_of "$proxy" "$PROXY_PORT")"
}

# state_of <pid> <port> — "running (listening)" / "running" / "stopped".
# An explicit function rather than `A && B || C`, which reports B when A fails
# and C when B fails, conflating the two.
state_of() {
  local pid="$1" port="$2" out
  if alive "$pid"; then
    out="running"
    if [ -n "$port" ] && port_open "$port"; then
      out="$out (listening)"
    fi
  else
    out="stopped"
  fi
  printf '%s' "$out"
}

cmd_logs() {
  case "${1:-api}" in
    api)   tail -n 60 -f "$LOG_DIR/api.log" ;;
    proxy) tail -n 60 -f "$LOG_DIR/proxy.log" ;;
    sup)   tail -n 60 -f "$SUPERVISOR_LOG" ;;
    *)     die "logs [api|proxy|sup]" ;;
  esac
}

cmd_restart() {
  cmd_stop
  sleep 1
  cmd_start
}

usage() {
  sed -n '3,20p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

case "${1:-}" in
  start)   cmd_start ;;
  stop)    cmd_stop ;;
  restart) cmd_restart ;;
  status)  cmd_status ;;
  logs)    shift; cmd_logs "${1:-api}" ;;
  run)     supervise ;;
  *)       usage; exit 2 ;;
esac
