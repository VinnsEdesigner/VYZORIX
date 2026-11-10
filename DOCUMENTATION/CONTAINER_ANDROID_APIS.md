# The bridge — reaching Android from inside the container

**Status:** shipped. Kotlin side: `android-core/.../bridge/`. Container side:
`debian-container/android-bin/`. The API list is `debian-container/android-bin/apis.txt`,
generated from `ApiCatalogue.kt`.
**Related:** `CONTAINER_FILESYSTEM.md` (where the socket lives),
`CONTAINER_TERMINAL.md` (the other thing that crosses this boundary), and
`AUTHORIZATION_ANALYSIS.md` (the scoped-RBAC engine behind §7).

This document explains how a process inside the Debian container calls an Android API —
and why the answer is "through a socket, with a capability token", not "through a
permission".

---

## 1. The problem

A process inside the container is a Debian program. It has no `Context`, no permissions,
and no way to call `BatteryManager`. Android APIs are reachable only from Layer 1's
process, which is the app.

So the container needs a door into Layer 1. That door is `android.sock`, a unix socket
served by Layer 1's bridge, and the container-side client is `android-bin`. One binary,
invoked under one name per API: `battery` is a symlink to `android-bin`, and running it
calls the bridge and prints the result.

---

## 2. The two halves

```
 inside the container                     Layer 1 (the app's process)
 ────────────────────                     ────────────────────────────
  battery  ──(symlink)──► android-bin
                              │
                              │  connect /tmp/android.sock
                              │  auth  (session token)
                              │  request  {api, args}
                              ▼
                          android.sock  ──►  ApiRouter  ──►  BatteryApi
                              ▲                                   │
                              └──────────  response  ◄────────────┘
```

- **`android-bin`** is the client: it reads the socket path and the token from the
  environment, authenticates once, and sends requests. It is C++, cross-compiled for the
  container's glibc, never the NDK.
- **The bridge** is Layer 1: it accepts the connection, authenticates the token, routes
  the request to the API's handler, and returns the result.

---

## 3. The wire contract

The frame shape is length-prefixed binary:

```
[4B length][1B type][payload]
```

`length` is the payload length, big-endian, capped at 1 MiB — a frame longer than that
is rejected rather than allocated, so a client cannot make the server allocate without
limit. The frame types are `Request`, `Response`, `PtyOutput`, `PtyInput`, `PtyCredit`
and `Event`; the terminal uses the PTY types, the API bridge uses request/response.

The Kotlin side is `BridgeProtocol.kt`; the C++ side is `bridge.cpp`. They must agree
byte for byte, and `json_test.cpp` pins the three response shapes the bridge sends — the
auth response, a success and an error — so a change on either side that breaks the
contract fails a test rather than a device.

The payload is JSON. The C++ side hand-rolls its JSON reader/writer rather than pulling a
dependency, because a general JSON library would be one more package that must exist in
the rootfs for every architecture. The surface is small and covered by tests; it is not a
general-purpose library and should not be grown into one.

---

## 4. Authentication: the capability token

The container does not present an Android permission, because permissions are a
Layer 1 concept the container cannot hold. Instead it presents a **session token** —
a per-session capability minted by Layer 1 — in its environment:

```
VYZORIX_BRIDGE_SOCKET=/tmp/android.sock
VYZORIX_BRIDGE_TOKEN=<per-session capability token>
```

`android-bin` reads both from the environment. There is no config file and no secret on
disk. The proot launcher puts them in the container's environment when it starts the
login shell.

The token is deliberately in the **inner** environment — the one `/usr/bin/env -i`
builds — not the outer one proot is started with. `env -i` clears the environment, so a
token passed only in the outer environment would be wiped before the shell ever saw it.
This is also why the token is an env var and not an argv entry: `ps` would show it in
argv.

A connection authenticates once (an `auth` request carrying the token) and then issues
requests. `auth` is the reserved name the bridge serves *before* authentication, so it
is deliberately absent from the API list and the router refuses to register it.

---

## 5. The API surface

`apis.txt` lists the API names — one per line — and it is the **single source of truth**.
One pass of `generator.cpp` over it produces both the generated catalog (what `--list`
prints and what the unknown-name check reads) and the symlinks, so they cannot disagree.
It is generated from `ApiCatalogue.kt` by `scripts/sync-apis-txt.py`, and
`ApiCatalogTest` fails the gate if the two drift.

The catalogue is large — over sixty APIs across device identity, storage, network,
power, sensors, connectivity, apps and settings. A representative sample:

```
battery  battery_history  thermal  power_save  load_average  memory_info
storage  storage_volumes  storage_all_files  external_storage  app_storage
network  network_interfaces  wifi_state  dns_servers  proxy_state  vpn_state
device_info  build_fingerprint  kernel_info  cpu_info  security_patch
installed_apps  package_info  default_apps  process_list  file_descriptors
location  phone_state  phone_state_detailed  sms_capability  sms_received
sensor_list  camera_list  audio_state  display_info  brightness  screen_state
usb_state  usb_attach  usb_detach  nfc_state  bluetooth_state  airplane_mode
clipboard_state  do_not_disturb  notification_policy  developer_options
device_owner  device_owner_capabilities  accessibility  input_method  locale_info
session  time_info  logcat  system_properties  mount_points  haptic_settings
```

Each name is both a symlink and a catalogue entry. `android-bin --list` prints them all.

---

## 6. Using it

Inside the container, from a login shell:

```bash
battery                          # prints the battery API's JSON result
battery --json '{}'              # explicit empty args
network iface=wlan0              # key=value pairs become the args object
android-bin --api battery        # the same call without the symlink
android-bin --list               # the API names this binary knows
android-bin --help
```

A name the router does not serve fails at call time with `unknown_api`. A handler the
router serves but that is missing from `apis.txt` is simply not reachable as a command —
which is why the drift test matters.

---

## 7. Who may call which API

The API bridge is a *capability* surface, and the capability model lives on the server.
Two axes decide whether a call is allowed, and both must pass:

- **Capability** — the API's declared tags (`data:*`, `op:*`, `protection:*`, `hw:*`,
  `fs:*`) must be satisfied by the caller's role.
- **Resource** — the caller must hold a grant for the API's action and scope.

The device never evaluates a tag and never learns a role. It receives a token carrying
the already-resolved allowlist, and checks only `api ∈ allowlist`. If the device
evaluated tags, a forged token could carry a role and grant itself everything; the token
must be the only truth because it is the cryptographically-bound artifact.

The device's own `@VyzorixApi` annotation is **documentation** — it lets a reader see an
API's capability next to its code, and lets a test assert the device's declared tags
match the server's catalog. It is not consulted at call time, so it cannot drift into
authority. The server-side policy that decides the allowlist lives in
`apps/api/internal/domain/permission`, and is described in
`AUTHORIZATION_ANALYSIS.md`.

---

## 8. The things it deliberately is not

- **Not a remote shell.** It exposes named APIs, not arbitrary Android calls. A caller
  cannot ask for "whatever `Context` method I like"; it asks for `battery` and gets the
  battery API's result.
- **Not the terminal.** The terminal is a separate path — a PTY allocated in Layer 1 and
  wrapped in a Debian shell. The bridge is request/response; the terminal is a stream.
  See `CONTAINER_TERMINAL.md`.
- **Not the cloud.** It is a local unix socket. Layers 2 and 3 never dial the cloud; only
  Layer 1 holds the WebSocket.
