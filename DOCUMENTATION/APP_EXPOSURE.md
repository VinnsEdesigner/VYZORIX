# Reaching an app — `expose`, ports and tokens

**Status:** `loopback` and `lan` shipped; `cloud` is deferred. The manifest field is
`expose` (see `MANIFEST_REFERENCE.md` §8); the runtime side is the supervisor's proxy and
`cloud-connector/.../deploy/ExposeTokenSource.kt`.
**Related:** `APP_RUNTIME.md` (the supervisor that exposes), `DEPLOY_PIPELINE.md` (how
`expose` gets there), `CONTAINER_FILESYSTEM.md` (the app's directory).

---

## 1. The question this answers

An app inside the container listens on a port. How does anyone reach it? The answer is a
single manifest field, `expose`, with three values and a deliberately conservative
default.

The design constraint behind it: **Layer 1 owns the socket.** The container cannot bind a
host port, and it should not — a relay that forwards to a loopback port inside the
container is safer and more testable than a process that reaches for the network. So the
proxy is a relay, not a server.

---

## 2. The three values

| `expose` | Reachable from | Requires | State |
|:--|:--|:--|:--|
| `loopback` | the device itself | nothing | **default** |
| `lan` | the local network | a `port`, and a token | supported |
| `cloud` | the internet, via the relay | the wildcard-DNS/TLS proxy | **refused in v1** |

The default is `loopback`, and that is a security default: an app is not reachable from
anywhere until someone decides it should be. `port` is required for anything but
`loopback`, because an app you want to reach has to be told where it listens.

---

## 3. `loopback` — the device itself

The app listens on `127.0.0.1:<port>` inside the container and is reachable only from the
device. This is the default, and it is the right choice for an app that is driven by the
terminal or by another app, not by a browser.

There is one important case where `loopback` is exactly what you want: a web console
served **by the device**. A console served from the device reaches the device's own
loopback with no configuration at all — the env schema defaults the API base URL to
`http://localhost:3000`, which is where the device listens. Serving same-origin also
removes CORS entirely and makes `127.0.0.1` a secure context, so a service worker and an
install prompt work without TLS.

---

## 4. `lan` — the local network, with a token

`lan` makes the app reachable from the local network. It requires a `port` and it
requires a token: **`0.0.0.0` without a token fails closed.** A proxy that forwards
unauthenticated traffic from the local network to an app is a way to expose an app's
data to everyone on the coffee-shop Wi-Fi, so the supervisor refuses to bind a
non-loopback address without a token, rather than binding it and hoping the app
authenticates.

The token is minted per exposure and is what a client presents to reach the app. It is
generated on the device (`ExposeTokenSource`), not delivered in the config — a token in
the config would be a token in five places, the same reasoning as secrets.

---

## 5. `cloud` — deferred, and why

`cloud` would make the app reachable from the internet through the cloud relay. It needs
the wildcard-DNS/TLS proxy that Layer 3 defers, and it is **refused in v1**: a manifest
that sets `expose = "cloud"` is rejected rather than silently downgraded. A silent
downgrade would leave a user believing an app is on the internet when it is not.

---

## 6. Where `expose` lives, and what it does

`expose` is a field on an app in the manifest, and it travels into the signed config and
into the registry. The supervisor reads it and configures the proxy accordingly:

- `loopback`: the app is left on the container's loopback; the relay forwards only from
  the device.
- `lan`: the relay binds the local interface and enforces the token.
- `cloud`: refused at compile time, so it never reaches the device.

`port` tells the relay where to forward. An app that never declares a port and never asks
to be exposed is simply run and supervised; reachability is opt-in.

---

## 7. The limits

- **No TLS for `lan`.** The token is the protection; a local-network deployment is
  expected to be on a network the user trusts. This is stated rather than implied.
- **No per-path or per-user authorization.** The token grants reach to the whole app.
  Fine-grained authorization is the app's job.
- **`cloud` is not here yet.** Until the wildcard proxy lands, an app reachable from the
  internet is not supported, and the manifest says so by refusing the value.
- **One port per app.** An app that needs several is several apps, or an app that fronts
  its own sub-services behind one port.
