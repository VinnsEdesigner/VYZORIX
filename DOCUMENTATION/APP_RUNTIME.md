# The app runtime — the supervisor and `app-*`

**Status:** shipped. Source is `app-runtime/supervisor/` (C) and
`app-runtime/cli-wrappers/` in `Vyzorix-android`; the device's apply path is
`cloud-connector/.../deploy/`.
**Related:** `DEPLOY_PIPELINE.md` (how apps get here), `APP_EXPOSURE.md` (reaching them
over the network), `CONTAINER_FILESYSTEM.md` (where they live), the device-side
`Vyzorix-android/doc/LAYER_3_REQUIREMENTS.md`.

This is Layer 3: the thing that runs *applications* inside the container and keeps them
running. If Layer 2 gave the device a Debian userspace with a shell, Layer 3 turns that
userspace from "somewhere you can type" into "somewhere things run".

---

## 1. What the supervisor is

One C process inside the container that:

- reads the registry (`apps.json`) and starts every app marked `autostart`;
- runs each app in its own process group;
- restarts a crashed app with exponential backoff, capped;
- watches the app directory with `inotify`, so a changed registry is picked up without a
  restart;
- forwards each app's stdout/stderr to a bounded, rotated log;
- serves a control socket so a CLI (and Layer 1) can start, stop, restart, list and
  inspect apps;
- never dials the cloud and never talks to Android.

It is a daemon, not a terminal. Layer 1 starts it inside the container with the same
`proot` argv shape as a login shell, minus the shell — `env -i` to clear the environment,
then the supervisor binary, with its `--apps`, `--logs` and `--socket` paths passed
explicitly so the layout is stated in one place.

---

## 2. The registry — `apps.json`

The registry is the single source of truth for what runs. Each entry carries:

```
{ name, dir, cmd, port, env, autostart, pid, pid_start_ticks }
```

- `name`, `dir`, `cmd`, `port`, `env`, `autostart` come from the signed config, written
  by the device's apply path.
- `pid` and `pid_start_ticks` are written by the supervisor and are authoritative **only
  together** (§4).

`env` holds *configuration* only. A list of `secret_keys` names the secrets an app
expects (names, never values); the values live in the app's `.env` file, mode `0600`,
which the supervisor reads at start and passes to the process. The registry is inside
`/home/apps`, which is bound into the container and readable by every app there, so it
must not be worth inspecting — which is why a secret is never in it. See
`DEPLOY_SECRETS.md`.

---

## 3. The control protocol

The control socket speaks the repo's framed shape: `[4B length][1B type][payload]`, one
request per connection. A client connects, sends one frame, reads one reply, and closes —
enough for a CLI, with no session state and no lifecycle to leak. The payload is JSON:

```
request : {"op": "start", "name": "flasher"}
reply   : {"ok": true, "result": {...}}  |  {"ok": false, "error": "..."}
```

The `op` set is deliberately closed — `start`, `stop`, `restart`, `list`, `logs`, `env`,
`create`, `remove`, `ping`, `expose`, `unexpose`. An unknown op is refused, not ignored,
so a CLI and a supervisor from different builds fail loudly rather than silently doing
nothing.

The socket path is `VYZORIX_APPS_SOCKET`, defaulting to the layout the architecture
fixes (`/home/apps/supervisor.sock`).

---

## 4. Restart and PID handling

Two details make the supervisor correct rather than merely working:

- **PID validation is by start-time token, not bare PID.** A bare PID is reused by the
  kernel after a process exits, so a supervisor that trusts a bare PID can mistake a
  *different* process for the one it started and "restart" the wrong thing. The
  supervisor reads the start-time token (`/proc/<pid>/stat` field 22) and treats `pid`
  and `pid_start_ticks` as authoritative only together.
- **Each app runs in its own process group.** A crashing app cannot take the supervisor
  down with it, and terminating an app means signalling its group.

Backoff is exponential and capped, so a permanently broken app does not spin. An app
that exits 0 when it is one-shot is **not** restarted.

---

## 5. The `app-*` CLI

One binary, invoked under many names, exactly like `android-bin`: the program's basename
selects the op.

```
app-start NAME       start an app
app-stop NAME        stop it
app-restart NAME     stop then start
app-list             list apps and their state
app-logs NAME        the tail of an app's log
app-env NAME         the environment an app runs with
app-create NAME CMD  declare an app
app-remove NAME      forget an app
app-expose NAME      how to reach an app (bind + token rule)
app-unexpose NAME    stop exposing an app
```

Each prints machine-readable output and exits non-zero on failure, so it works from a
terminal and from a script.

**Why C++ and not a bash wrapper:** the reply is JSON and the framing is binary. A shell
script would need `head -c`, `dd` and a JSON parser — three ways to be subtly wrong — for
something the shared library already does correctly.

---

## 6. How a deploy reaches the supervisor

The device's apply path (`DeployApplier`) does the following, in order:

1. **Ping the supervisor first.** A config that lands with no supervisor running would
   leave every app silently stopped, so an unreachable supervisor is a refusal
   (`supervisor_unreachable`), not a proceed-and-hope.
2. **Fetch and verify every artifact** into staging before touching any live directory.
3. **Resolve every secret** before touching the live tree — a secret that will not open
   must abort with the device unchanged.
4. **Install atomically**: a per-artifact rename of the staged directory over
   `<app>/<artifact>`, which keeps the app's *other* files (its data, its `.env`) intact.
   Replacing the whole app directory would delete exactly the state that is retained.
5. **Write `apps.json` atomically.**
6. **Signal the supervisor**, whose `inotify` path picks the change up.

A failure at any step leaves the previous config running — see `DEPLOY_PIPELINE.md` §10.

---

## 7. The app lifecycle

Per app, not per device:

```
 (absent) ── appears in a signed config ──► STOPPED ── autostart ──► STARTING ──► RUNNING
                                                │                                 │
                                                │                     crash ──────┤ exit 0
                                                ▼                                 ▼
                                             BACKOFF ◄──────────────────────────  STOPPED
                                                │
                                                │ failures > N
                                                ▼
                                             FAILED  (reported; no auto-heal)
```

`FAILED` is reported to the server so the console can show it; nothing restarts it
automatically. An app absent from the next signed config is stopped and deregistered,
but its directory and data remain — only `purge = true` deletes them.

---

## 8. Reaching an app

An app listens on a port inside the container. How a user reaches it depends on
`expose` in the manifest: `loopback` (the device itself), `lan` (the local network, with
a token), or `cloud` (refused in v1). Layer 1 owns the socket, so the proxy is a relay,
not a server. The details are `APP_EXPOSURE.md`.

---

## 9. Testability, and why it is built the way it is

The supervisor is split so its logic can be tested on a host without forking anything.
Everything that touches the outside world — spawning, killing, reading the clock,
sleeping, reading start-time tokens — goes through a `Host` interface. Production uses
the system implementation; tests use a fake. So "restart with backoff", "a reused PID is
not mistaken for our process" and "rotation at the size cap" are exercised without
waiting on real time or real processes.

The control protocol's request handler is likewise pure with respect to the socket: it
takes a JSON request and returns a JSON reply, so the op semantics are pinned without a
socket at all.
