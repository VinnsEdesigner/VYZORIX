# Supervisor (no systemd)

`supervise.sh` runs the API and the proxy under one parent process. It exists
because this container is not a systemd host — PID 1 is the agent, and
`/run/systemd/system` does not exist, so `tooling/systemd/` cannot run here.

Use `tooling/systemd/` on a real VM. Use this where systemd is unavailable.

## Commands

```bash
tooling/dev/supervise.sh start      # daemonise both, return when healthy
tooling/dev/supervise.sh stop       # stop both, children included
tooling/dev/supervise.sh restart
tooling/dev/supervise.sh status
tooling/dev/supervise.sh logs api   # or: proxy, sup
tooling/dev/supervise.sh run        # foreground; Ctrl-C stops both
```

`status` prints the supervisor and both children with their ports and whether
each port is actually accepting connections — a pid alone does not prove the
server is serving.

## What it does that `nohup cmd &` does not

- **One supervisor, two children.** Stopping the supervisor stops both, so the
  tree cannot drift into orphans.
- **Reaps exited children.** It `wait`s on each child, so a crashed process is
  collected rather than left as a zombie. The `nohup` workflow leaves zombies
  because nothing ever waits on them.
- **Restarts with backoff.** A crash restarts after 1s, 2s, 4s … capped at 30s.
- **Crash-loop guard.** After 8 restarts inside 120s it gives up and stops.
  A process that dies immediately, repeatedly, is a bug to surface — not a
  condition to hide behind an infinite restart loop.
- **Refuses a second copy.** `flock` on `.local-dev/supervisor.lock`.
- **Fails the start if a port never opens.** `start` returns non-zero rather
  than reporting success for a process that never bound.

## Configuration

Everything is an environment variable with a dev-friendly default:

| Variable | Default | Meaning |
|---|---|---|
| `RUN_DIR` | `.local-dev` | pidfiles, lock, default log and env location |
| `LOG_DIR` | `RUN_DIR` | where `api.log`, `proxy.log`, `supervisor.log` go |
| `API_BIN` | `RUN_DIR/bin/vyzorix-api` | API binary |
| `API_ENV` | `RUN_DIR/api.env` | API env file, sourced before exec |
| `API_PORT` | `3000` | health-check port |
| `PROXY_DIR` | `apps/proxy-server` | proxy working directory |
| `PROXY_PORT` | `12000` | proxy port |
| `CRASH_LIMIT` | `8` | restarts allowed inside the window |
| `CRASH_WINDOW` | `120` | seconds; a child that ran longer resets its count |
| `HEALTH_TIMEOUT` | `20` | seconds to wait for a port on start |

## Shutdown

Both binaries handle SIGTERM — the Go server drains in-flight requests
(`signal.NotifyContext`), the proxy closes its listener and flushes persisted
sessions. The supervisor sends SIGTERM to both, waits 10s, then escalates to
SIGKILL. If the supervisor itself is killed, `stop` also cleans up any child left
by pidfile.

## Files

```
.local-dev/supervisor.pid      supervisor pid
.local-dev/supervisor.lock     flock target
.local-dev/supervisor.log      supervisor's own log
.local-dev/api.pid             API child pid
.local-dev/proxy.pid           proxy child pid
.local-dev/api.log             API output
.local-dev/proxy.log           proxy output
```

## Note on the proxy's session store

`start`/`stop`/`restart` do not touch `apps/proxy-server/.sessions/`. Sessions
persist across a supervisor restart, so a browser keeps its login. If you want a
clean slate, remove that directory while the proxy is stopped.
