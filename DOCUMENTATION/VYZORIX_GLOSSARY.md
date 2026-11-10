# Glossary — the words this platform uses

**Status:** reference. This glossary covers the **platform** vocabulary — the device, the
container, deploys and the CLI. The server's own vocabulary (auth, risk, audit, the
database) is in `GLOSSARY.md`; the two are complements, not duplicates.

Read this if a document uses a word you are not sure about. Where a term has a precise
meaning that differs from the everyday one, the precise meaning is what matters and is
what is written here.

---

## The system as a whole

**Vyzorix** — The platform: a Debian userspace on an unrooted Android phone, reachable
from anywhere, with an operator console, a terminal and application deployment.

**Agent** — The app running on the device. "The agent" and "the app" mean the same thing;
the former is used when the device is the subject.

**Server** — The Go service and console in this repository. It compiles and signs
deploys, holds secrets, serves the API, and owns the WebSocket.

**CLI** — The `vyzorix` command-line tool (`vyzorix-cli`), the operator's handle on the
server.

**Spec** — `vyzorix-spec`, the shared module declaring the manifest shape and wire types,
so the CLI and the server cannot drift.

---

## The device layers

**Layer 1 — Android Gateway** — The only layer that touches Android. The foreground
service, the boot receiver, the watchdog, the PTY broker, the USB broker, `android.sock`,
the API bridge, and the one cloud WebSocket.

**Layer 2 — Debian Container** — The Debian userspace running under `proot`. (Formerly
called "the engine"; the word was changed because "engine" was vague and the thing is a
userspace container.)

**Layer 3 — App Runtime** — The supervisor, the `app-*` CLI, `apps.json` and the app
proxy, all inside the container.

**Layer 4 — Cloud Connector** — The Kotlin code that speaks to the server. It lives
*inside* Layer 1's process, not as a second client, because there is exactly one
WebSocket and one owner.

**The container** — The Debian userspace of Layer 2, as seen from inside. "Inside the
container" means "inside the `proot` namespace".

**proot** — The userspace `chroot` and `mount --bind` that presents the Debian rootfs as
`/`. It does **not** change the real uid; its `-0` flag fakes userspace root only.

**The rootfs** — The extracted Debian tree at `files/rootfs`. Per-architecture, replaced
wholesale on a container update.

**The overlay** — The writable prefix (`overlay/usr-local`, `overlay/opt`,
`overlay/etc`) that binds over `/usr/local`, `/opt` and `/etc/vyzorix`. It survives a
rootfs replacement.

**Tier** — A device's capability class: 1 unrooted, 2 rooted, 3 advanced. Used to *cap*
what may be granted; a device self-reports it and the server never raises the cap on the
report.

---

## The bridge and the terminal

**`android.sock`** — The unix socket Layer 1 serves so the container can call Android
APIs. The door.

**Bridge** — The Layer 1 side of `android.sock`: it authenticates the session token,
routes a request to the API's handler, and returns the result.

**`android-bin`** — The container-side client for the bridge. One binary, invoked under
one name per API.

**Capability token** — The per-session token `android-bin` presents to the bridge. It
carries the already-resolved API allowlist; the device never evaluates a role or a tag.

**PTY** — A pseudo-terminal. Allocated by Layer 1 (never by the container), whose master
fd is the capability that drives the shell.

**Detach / reattach** — The terminal's survival of a dropped WebSocket: the shell keeps
running, output is buffered (bounded), and a reconnect re-binds the same session.

---

## Deploy

**Manifest** — `vyzorix.toml`, the operator's declarative file. The only artifact a human
writes.

**Config** — The compiled, signed `vyzorix.v` document the device receives. The manifest
never leaves the operator's machine; the config is what travels.

**`vyzorix.v`** — The line-based, depth-encoded document format of the config. Depth is
the number of leading `v` characters.

**Artifact** — A referenced, digest-pinned blob an app is built from. A URL and a
mandatory `sha256`, never inline bytes.

**Digest** — The artifact's `sha256`. It makes the transport irrelevant: a substituted
byte is caught regardless of where the artifact is served from.

**Webroot** — An artifact `kind` that is unpacked and served as static files, rather than
run. A webroot with `pwa` enabled gets a generated web-app manifest.

**PWA** — A webroot that declares `pwa` (boolean or table) and therefore receives a
generated `site.webmanifest`. Off by default.

**`expose`** — How an app is reachable: `loopback` (default), `lan` (needs a token), or
`cloud` (refused in v1).

**Deploy credential** — A long-lived, device-scoped token so CI can deploy without an
operator session. Shown once, revocable, never returned by a `GET`.

**Version** — The server-assigned, monotonic number of a config. A device compares the
version it holds to the server's and pulls when it is behind.

**Last-good** — The previous config, retained alongside the current one. Retention is
two; there is no deeper history and no `undeploy`.

**Refusal** — A structured "no" with a machine-readable reason (`digest_mismatch`,
`cmd_escape`, `secret_undecryptable`, …). The refusal is the product: it says exactly why
a deploy did not apply.

---

## Secrets and signing

**Secret** — A value an app needs that must not be in the config (a database URL, a
token, a key). Set with `vyzorix secrets set`, encrypted to the device, delivered on its
own channel.

**Secret key (`vyzorix.secret`)** — The RSA-3072 Keystore keypair, `PURPOSE_DECRYPT`,
separate from the identity key, to which secrets are wrapped. The container cannot reach
it.

**Identity key** — The EC P-256 Keystore keypair, `PURPOSE_SIGN`, generated on first run.
The device's identity. The private half never leaves the device.

**Envelope** — The RSA-OAEP-SHA1 wrapping of a secret to the device's secret key. SHA-1
is a Keystore constraint, not a preference.

**HMAC domain** — One of the three signing schemes (client request, device REST, command
frame). They are distinct trust boundaries and must not be merged.

**Pinned key** — The server's public signing key, compiled into the app, used to verify
config and command signatures. What makes an offline device able to trust a config.

---

## Onboarding

**Enroll** — `POST /v1/device/enroll`. A device with no credential yet announces its
public key and its facts; the server creates a pending record and returns a pairing code.

**Pairing code** — The short Crockford-base32 code a device shows and an operator types
to claim it. Server-issued, single-use, TTL-bounded, rate-limited.

**Claim** — The operator action that binds a pending device to an owner and an org,
consuming the pairing code and triggering config delivery.

**Lifecycle** — A device's state: `pending`, `registered`, `deregistered`. There is no
separate `approved` state.

**Tier hint** — The device's self-reported capability class, sent at enroll and used to
cap, never to grant.

---

## The app runtime

**Supervisor** — The C process inside the container that runs and watches apps. Reads
`apps.json`, restarts on crash with backoff, serves the control socket, exposes apps.

**`apps.json`** — The registry: what should run, and the supervisor's `pid` /
`pid_start_ticks` for what is running.

**`app-*`** — The operator's CLI inside the container: `app-start`, `app-stop`,
`app-list`, and friends. One binary, one name per op.

**Control socket** — The supervisor's socket, `VYZORIX_APPS_SOCKET`
(`/home/apps/supervisor.sock` by default). One request per connection, JSON payload.

**Start-time token** — `/proc/<pid>/stat` field 22, read so a reused PID is not mistaken
for the supervisor's own process. `pid` and `pid_start_ticks` are authoritative only
together.

**`APP_START` / `APP_STOP`** — The signed commands (Layer 1 → supervisor) that start and
stop an app. Ordinary executors on the command seam.

---

## Telemetry and operations

**Telemetry** — The periodic report of a device's health (uptime, thermal, and reserved
fields). Push-only; a silent device is an invisible one.

**Device event** — A discrete thing that happened, stored as a ledger and queryable by
device, type and id. Distinct from telemetry, which is a time series.

**OTA** — Over-the-air update of the APK itself. Automatic up to the install, which the
user confirms (Android's rule).

**Download progress** — The device's periodic report of an in-flight APK download, held
in a 2-minute in-memory store so a stopped device reads as `downloading:false`.

---

## Security and permissions

**Boundary contract** — The rule that the server decides *whether* and the device
enforces *what*. The concrete form is in `CONTAINER_ANDROID_APIS.md` §7 and
`DEPLOY_PIPELINE.md` §4.

**Tag** — A `dimension:value` capability string (`data:private`, `op:read`, `hw:usb`).
Evaluated only on the server.

**Allowlist** — The resolved set of APIs a session may call, carried in the capability
token. The device checks `api ∈ allowlist` and nothing else.

**Closed world** — The property of the bind list: only the paths named are bound, so the
container cannot reach the rest by construction.

**Sandbox zone** — The app-private directories (the secret store, the recordings) that
must never be bound into the container. Asserted independently of the closed world.

**`fs:container`** — The zone tag for the app's own critical area. It appears in no role
grant and no tier policy, so it is unaddressable rather than merely un-granted.
