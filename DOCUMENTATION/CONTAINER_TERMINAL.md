# The container terminal

**Status:** shipped. Device: `cloud-connector/.../terminal/TerminalRelay.kt`,
`.../cloud/PtyFrame.kt`, the PTY broker in `android-core/.../capability/`, and the
session API in `android-core/.../bridge/apis/SessionApi.kt`. Server: the terminal route
under `/dashboard`.
**Related:** `CONTAINER_FILESYSTEM.md` (where the shell runs),
`CONTAINER_ANDROID_APIS.md` (the other thing that crosses the same boundary),
`WEBSOCKET_SYSTEM.md` (the transport).

This document explains how a user gets a shell inside the container, and the two
design decisions that make it work on a phone: the PTY is allocated **outside** proot,
and the shell **outlives** the socket.

---

## 1. The problem, and the two ideas

A terminal needs a PTY. The obvious way to give the container one is to let the Debian
shell open `/dev/ptmx` — but inside `proot` that is not guaranteed to work, and it is the
kind of thing that works on one device and not the next.

So the platform does two things differently:

1. **Layer 1 allocates the PTY and holds the master fd.** The container never opens
   `/dev/ptmx`. This is the fd-capability model: the privileged resource is acquired by
   the one layer that can, and passed down.
2. **The shell's lifetime is the session's, not the socket's.** A dropped WebSocket
   detaches the session and reattaches it, rather than killing the shell.

Both are the reason a terminal here is robust where a naive one is fragile.

---

## 2. The path a keystroke takes

```
 browser  ──WS──►  server  ──device WS──►  Layer 1        ──►  PTY master fd
                                             │                    │
                                             │  proot + bash      ▼
                                             └──────────────►  the container's shell
```

1. The user opens a terminal in the console.
2. The server mints a session id and sends the device a binary **OPEN** frame over the
   device WebSocket.
3. The device allocates a PTY (Layer 1's NDK code: `posix_openpt` → `grantpt` →
   `unlockpt` → open the slave), sets it as the controlling terminal, and execs the
   `proot` argv that runs a Debian login shell.
4. From then on, the two sides exchange **DATA** frames (keystrokes down, output up),
   **RESIZE** frames and a **CLOSE** frame, all multiplexed over the one device socket
   and all keyed by the session id.

`TerminalRelay` is that relay on the device: it decodes frames, drives the PTY sessions
through a port, and encodes the device's own frames back. It owns no socket and no PTY —
the socket is the caller's and the shell is the port's — so it is pure routing and can be
tested without either.

---

## 3. The PTY is allocated outside, and the fd is the capability

The NDK layer opens the PTY pair and holds the **master** fd; a tiny native launcher
does `setsid()` + `ioctl(slave, TIOCSCTTY, 0)` and `dup2`s the slave onto fds 0/1/2, then
`exec`s the `proot` argv. Layer 1 does all I/O on the master fd. Layer 2 never opens
`/dev/ptmx`.

This is not an optimisation; it is the design. The capability *is* the authorization —
whoever holds the master fd can drive the shell, and nobody else can — and it is
auditable. It also gives a clean privilege boundary: the container receives a working
terminal without being trusted to allocate one.

---

## 4. Detach and reattach — a shell that outlives the socket

This is the feature that makes the terminal usable on a phone, where the network comes
and goes.

When the WebSocket drops, the relay **detaches** every live session instead of closing
it:

- the shell keeps running;
- its output is buffered up to a bound;
- a reconnect re-attaches **by the same session id** — the browser sends `pty_open` with
  the id it used before, the server re-opens it, and the device re-binds the surviving
  shell rather than spawning a second one.

The buffer is bounded, because an unbounded one would grow with whatever the shell prints
while nobody is watching. When it overflows, the **oldest** bytes are dropped — a
terminal is a tail, and the most recent output is what a user needs.

A detached session is reaped after a device-side TTL. The server also has an idle TTL, as
a backstop for a session the user forgets; the device's is deliberately shorter,
because a phone should reclaim the memory of a shell whose client is not coming back
sooner rather than later.

### `tmux`, and why reattach is also possible *inside* the container

There are two layers of persistence, and they are different:

- the **relay's** detach/reattach keeps the *PTY session* alive across a socket drop;
- **`tmux`**, when it is installed (it is in the extended tier), keeps the *shell*
  alive across a session close, so a client that reconnects with the same tmux session
  name finds its shell still running.

`ProotLauncher.loginShellArgs` wraps the shell in `tmux new-session -A -s NAME` when
`tmux` is present: `-A` creates the session the first time and *attaches* to it
afterwards. The name is the caller's — the same across reconnects, different between
terminals — which is exactly what a terminal session id is. When `tmux` is absent, a
plain `bash --login` is used; a shell without reattach is better than no shell.

Two consequences worth knowing:

- `tmux` needs `TMUX_TMPDIR` set explicitly, because under proot `realpath` fails on a
  path with a trailing slash and tmux's compiled default (`/tmp/`) is dropped. Setting
  it to `/tmp` (no slash) makes the entry resolve.
- The `tmux` *server* outlives any single client and keeps the environment of the client
  that started it. A reconnect therefore reuses the original session's bridge token
  rather than the new one — the desired behaviour, but it means a token rotation does not
  reach an already-running shell.

---

## 5. Flow control

The server applies a credit window to output going *to* the user, but it never sends
credit back to the device (its own accounting governs input). So the device must not
stall waiting for an ACK that never comes: the reader's window is replenished locally as
output is handed to the socket, and a session whose socket write fails is **closed**
rather than left blocked. A blocked reader would hold the shell's output forever, which
looks like a hung terminal.

---

## 6. What the shell sees

The login shell is a Debian `bash --login`, run with `proot -0` (fake userspace root, so
`apt` and `dpkg` behave), with the bind list from `CONTAINER_FILESYSTEM.md` and two
environment variables from Layer 1:

```
VYZORIX_BRIDGE_SOCKET=/tmp/android.sock
VYZORIX_BRIDGE_TOKEN=<per-session capability token>
```

Those are what let the shell call Android APIs via `android-bin`
(`CONTAINER_ANDROID_APIS.md`). They are placed in the **inner** environment — the one
`/usr/bin/env -i` builds — not the outer one proot is started with, because `env -i`
clears the environment and a token passed only in the outer environment would be wiped.
The token is an env var rather than an argv entry so `ps` cannot show it.

---

## 7. Access, and the two things that gate a terminal

- **`shell.execute` is a separate grant.** The terminal route is gated on it, not on a
  general device-read permission, because a shell is the highest-consequence thing an
  user can open.
- **A recorded session is as sensitive as the terminal itself.** Reading a recording is
  gated the same way, because a recording contains everything that appeared on the
  terminal.

---

## 8. The limits

- **The terminal is a shell, not an app console.** It reaches the container's filesystem
  and the bridge APIs; it is not a way to reach an app's UI (that is `APP_EXPOSURE.md`).
- **`tmux` is optional.** Without it, a session still survives a socket drop (the relay
  detaches), but not a session close. `tmux` ships in the extended tier.
- **A detached session is reaped.** A closed tab does not hold a shell forever; the
  device TTL and the server idle TTL both bound it.
