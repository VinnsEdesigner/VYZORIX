# Device onboarding — from an APK to a device online

**Status:** shipped. Device side: `cloud-connector/.../enroll/`, `.../identity/`,
`.../cloud/EnrollmentClient.kt`; the app's start flow in `app/src/main/kotlin/.../agent/`.
Server side: the enroll handler and the device-signed group.
**Related:** the device-side `Vyzorix-android/doc/REGISTRATION_CONTRACT.md` (the
contract this narrates), `APK_DISTRIBUTION.md` (getting the APK onto the phone),
`DEVICE_COMMANDS_AND_SIGNING.md` (what the device may do once online),
`CONTAINER_FILESYSTEM.md` (what it looks like once the container is up).

This document walks the whole path a freshly installed device takes to become a known,
owned, configurable device in exactly one account — and it explains *why* the flow has
the shape it does, because the shape is the security.

---

## 1. The one idea

**The device proves who it is with a keypair it generates itself; the pairing code is
only a rendezvous; the config carries settings and never a secret.**

Three consequences, and they are the whole point:

- **No secret is ever transmitted.** The private key is generated on the device and
  never leaves it. There is nothing to intercept, on any channel.
- **The APK is generic.** One artifact, one release pipeline. Nothing user-specific is
  compiled in, so a downloaded APK is worth nothing on its own.
- **The server stays authoritative.** The device reports facts; the operator decides
  ownership and policy; the server assembles and signs the config.

This replaced an earlier flow where the server pushed a shared secret over FCM and the
device posted it back. That flow had three defects — a root secret over an
unauthenticated channel, a "confirm" step that proved nothing, and two secret paths
that hashed differently and could never match — and the keypair flow removes all three
by removing the secret.

---

## 2. What the operator does

From the operator's side the whole thing is three actions:

1. **Install the APK** on the phone (sideload — see `APK_DISTRIBUTION.md`).
2. **Open it once** and grant the accessibility service when the notification asks, so
   the agent can post its persistent notification.
3. **Claim the device**: read the pairing code from the notification, open the console's
   pair page, type the code, confirm the device is yours, and pick the organization and
   a name.

Everything else is automatic. There is no login on the device, no account entry, no
token to paste — by design, because every one of those is a step a user can get wrong.

---

## 3. Phase A — the device comes up, with no identity

1. First launch. The foreground service starts. There is no UI.
2. The device generates an **EC P-256 keypair** in the Android Keystore, once. The
   private key is non-exportable and never leaves the device. It also generates the
   **secret-decryption key** (`vyzorix.secret`, RSA-3072, `PURPOSE_DECRYPT`) used later
   for app secrets — see `DEPLOY_SECRETS.md`.
3. The device collects its facts: a **device-generated UUID** (the stable id), model,
   manufacturer, OS version, API level, build id, security patch level, supported ABIs,
   RAM, storage, device class, and a **tier hint** (unrooted / rooted / advanced).
   Everything is sent at enroll so the operator sees the whole picture at claim time.
4. The device `POST`s `{deviceUuid, publicKey, secretKey, deviceInfo}` to
   **`POST /v1/device/enroll`** — public, because there is nothing to authenticate
   *with* yet. This endpoint must be rate-limited per IP from the start: an
   unauthenticated endpoint that writes a row is a flood target.
5. The server creates a **pending** device record and returns a **pairing code**, its
   expiry, and a poll interval.
6. The device posts a **notification** (a system surface, not app UI):

   > Vyzorix setup — code `K7QM-3X2P`
   > Enter it at vyzorix.com/pair

The device then opens its WebSocket and waits.

---

## 4. Phase B — the operator claims it

7. The operator, already signed in to the console, opens `/pair` and types the code.
8. The server looks it up: valid, not expired, not already claimed.
9. The server shows the device's facts — *"Pixel 7a · Android 13 · 4 GB · added 40
   seconds ago"* — and the operator confirms it is theirs, picks the organization, a
   name, and the policy.
10. The server binds the device: owner, org, name, tier cap. It consumes the code and
    transitions **pending → registered**.
11. The server assembles `vyzorix.v` (the config) and signs it.

---

## 5. Phase C — the device receives its config

12. The device has been holding its WebSocket open through onboarding, so the config
    arrives as a `device_config` frame the moment it is approved. No push service
    needed.
13. The device verifies the server signature against the pinned key, applies the config,
    stores it, and is now **registered**.
14. The device reports `device_ready`.
15. The notification updates to *"Vyzorix is running."*

If the device went offline between A and B, it re-fetches its own state on reconnect.
Nothing is lost, because the config is versioned and the device pulls when it is behind.

---

## 6. The pairing code

Server-issued, because the server owns uniqueness and entropy.

| Property | Value | Why |
|:--|:--|:--|
| Alphabet | Crockford base32 (`0-9A-Z` minus `I L O U`) | No ambiguous characters; a human reads it off a notification and types it |
| Length | 8 characters, grouped `4-4` | ~40 bits. With the TTL and rate limit, guessing a live code is infeasible |
| TTL | 10 minutes | Short enough that a leaked code is stale, long enough to walk to a browser |
| Use | single | A claimed code is consumed; replay fails |
| Rate limit | per IP and per account | Brute force is bounded, not merely discouraged |

The threat this closes: an attacker who guesses a live code claims *someone else's*
device into *their* account and can then command it. Entropy alone is not the defence —
the TTL, the single-use rule and the rate limit are what make it adequate. This is worth
stating plainly rather than implying the step is authenticated: it is the one
unauthenticated step in the flow, by necessity.

---

## 7. Identity and trust

**What the device holds:**

- an EC P-256 private key in the Keystore, non-exportable, generated before any network
  call;
- the secret-decryption key (`vyzorix.secret`);
- the server's public signing key, **pinned in the app**, used to verify config and
  command signatures.

**What the server holds:**

- the device's public key, registered at enroll;
- the device record: uuid, owner, org, name, tier cap, config version.

**What travels:** the public keys once, at enroll, and the signed config. Nothing else.
No shared secret, at any point, in either direction.

**Device → server** is authenticated by signing the request with the device's private
key; the server verifies against the registered public key.
**Server → device** is authenticated by the server's signature, verified against the
pinned public key.

---

## 8. The device state machine

The lifecycle has exactly three states — `pending`, `registered`, `deregistered` —
reused from the existing model. There is no separate `approved` state: claiming a code
and delivering the config are both the single `pending → registered` transition, so
"claimed but not yet configured" does not need to be representable.

```
 (no record)
     │  POST /v1/device/enroll
     ▼
  PENDING ────── code expires ──────────► (record expires, device re-enrolls)
     │  operator claims the code, binds owner + org, config delivered
     ▼
 REGISTERED ──────── device offline mid-claim ─► re-fetches config on reconnect
     │
     │  DELETE /v1/device/:id
     ▼
DEREGISTERED (irreversible; 30-day retention)
```

A device that never gets claimed expires and re-enrolls with a fresh code. Nothing is
orphaned and nothing is ambiguous.

---

## 9. What the config gives the device

The delivered config (`vyzorix.v`) carries settings and features — terminal on/off, USB
on/off, apps on/off, the telemetry interval, the stream endpoint — and the tier cap.
The `tierCap` is the **server's cap**, intersected with the device's self-reported tier
at session open. A device that overstates its tier gets an allowlist it cannot serve —
harmless, because the fd is the real capability — but the server never *lowers* the cap
on a self-report.

The config never carries a secret. That invariant is enforced by a test on both sides,
not promised.

---

## 10. Where the container comes in

Onboarding makes the device *known*. The container is what makes it *useful*, and the
two are independent: a device can be registered and bridge-only (no container yet) if
the rootfs has not installed. The first boot of the service installs the container's
userspace — the Debian rootfs bundled in the APK — and from then on a terminal and
deploys work.

The install is idempotent and offline-capable: the core rootfs ships inside the APK, so
a device with no network still gets a working container. The optional `debian-extended`
tier (git, python3, tmux) downloads on demand, because a device that only wants a shell
should not carry ~108 MB it never runs. See `CONTAINER_FILESYSTEM.md`.

---

## 11. Honest limits

- **Tier is a self-report.** The server cannot verify root. It is used to *cap* what is
  granted, never to grant more than policy allows. The real control is the capability.
- **The pairing code is the only unauthenticated step**, by necessity.
- **A device with no network at first boot is bridge-only** until it can reach the
  server to enroll. Offline first-deploy is not supported; every deploy after enroll is.
- **Accessibility is a capability, not a registration gate.** It needs a trip to system
  Settings and is a large onboarding drop-off, so it is requested when something needs
  it, and the device degrades gracefully without it.
