# Commands, and the three signing domains

**Status:** shipped. Server: `apps/api/internal/api/handlers/command/`,
`apps/api/internal/infrastructure/crypto/`, the middleware package. Device:
`cloud-connector/.../command/`.
**Related:** `COMMAND_EXECUTION.md` (the command lifecycle in depth),
`DEVICE_ONBOARDING.md` (how the device got its credentials), and
`AUTHENTICATION_SYSTEM_SERVER.md` (session and API-key auth).

This document explains how the server tells a device to *do* something, and how both
sides know a message is genuine. The second half matters more than the first: the
platform has three distinct signing schemes, and they look similar enough to be merged
by accident. They must not be.

---

## 1. Commands, in one paragraph

A **command** is a one-shot instruction — check for an update, run a shell command,
start or stop an app, reboot. The server creates it, signs it, and delivers it to the
device; the device verifies the signature, executes, and reports the result. Commands
have a small lifecycle and are audited at every step.

This is the *control* path. It is separate from the *deploy* path (`DEPLOY_PIPELINE.md`),
which delivers a whole config; a command is a single action, and it is the right tool
for "do this now" rather than "make it look like this".

---

## 2. The command lifecycle

```
pending → delivered → completed
                   ↘ → failed
pending → cancelled
delivered → cancelled
```

| State | Meaning |
|:--|:--|
| `pending` | Created, waiting to reach the device. |
| `delivered` | Sent over WebSocket (or FCM wake), awaiting a response. |
| `completed` | The device confirmed it executed successfully. |
| `failed` | The device reported failure, or the command timed out. |
| `cancelled` | An operator cancelled it before completion. |

The three terminal states cannot transition further. Commands are per-command, not
per-device: one device can have several in flight.

---

## 3. Creating and delivering a command

`POST /v1/device/:imei/command` is the entry point. The handler:

1. validates the request (device id, command, nonce);
2. checks the device belongs to the caller's organization;
3. runs the risk gate — classifies the command, checks MFA, consumes a confirmation
   token if the command is risky (`RISK_AND_CONFIRMATION.md`);
4. creates the command with status `pending`;
5. builds the `CommandFrame` (the wire format);
6. **signs the frame** with the device's key (§4, Domain C);
7. delivers it: over the WebSocket if the device is online, or as an FCM silent wake if
   not;
8. marks it `delivered` if it went over the WebSocket;
9. writes an audit entry.

### Delivery: WebSocket first, FCM second

```
if hub.Online(imei) { hub.Send(imei, frame) }     // fast path
else                { fcmNotifier.SendSilentWake() } // wake, device then pulls
```

The WebSocket is the primary path. FCM is a *wake*, not a delivery channel: it tells an
offline device to connect and fetch its pending commands (`GET
/v1/device/:imei/commands/pending`). The architecture is explicit that FCM is an
unauthenticated wake channel and not a command-authority channel, which is why nothing
sensitive travels over it.

A **command outbox** — a background worker — polls for pending commands and retries
delivery with exponential backoff, so a command created while a device was offline lands
when the device comes back.

---

## 4. The three signing domains

The platform has three HMAC signing schemes. They are **not** duplicates: each serves a
different trust boundary, with different headers, canonical formats and key sources.
Merging them would break the device API contract, so this section exists to make that
hard to do by accident.

| Domain | Direction | Who signs | Who verifies | Key source |
|:--|:--|:--|:--|:--|
| **A — Client request** | web app → server | the console | `session_signing.go` | the session's `SigningKey` |
| **B — Device REST** | device → server | the APK | `request_signing.go` | the device's `CommandSecretHash` |
| **C — Command frame** | server → device | the server | the APK | the device's `CommandSecretHash` |

### Domain A — the console signs its requests

**Scope:** all traffic from the web console to the server — REST, GraphQL, and the
WebSocket subscription handshake.

The console signs every outgoing request with the session's signing key (issued at
login/MFA time) and sends `X-Vyzorix-Timestamp`, `X-Vyzorix-Nonce` and
`X-Vyzorix-Signature`. The canonical string is newline-delimited:
`METHOD\nPATH\nNONCE\nTIMESTAMP\nBODY`, HMAC-SHA512, base64 output. The server verifies
in `SessionSignatureMiddleware`, which runs *after* cookie/API-key auth and uses the
session's own key. Nonces are cached to reject replays within the window.

### Domain B — the device signs its REST calls

**Scope:** REST calls from the APK to the server — enroll, telemetry upload, config
fetch, command ack.

The device sends `X-Client-ID`, `X-Timestamp` and `X-Signature` (the signature is
`t={unix},v1={hmac_hex}`). The server resolves the device by `X-Client-ID`, retrieves
the device's `CommandSecretHash`, and verifies. Device request bodies are
**AES-256-GCM encrypted** as well, which browser requests are not. Replay cache keyed by
client id + timestamp.

### Domain C — the server signs the command frame

**Scope:** command frames over the WebSocket, server → device. The console is not
involved — it dispatches over REST (Domain A) and the server signs the outbound frame.

The server generates a nonce and timestamp, computes `HMAC-SHA512(frame, deviceSecret)`,
and puts the signature in the frame. The device verifies before executing, which is what
prevents command injection. The canonical string is pipe-delimited:
`dispatchId|deviceId|command|ts_ms|nonce|args`.

All three dispatch paths sign before delivery — the direct execute handler, the outbox,
and the update push service — so there is no path that sends an unsigned frame.

### Why not one scheme

- **Different trust boundaries.** The console authenticates with a session; a device
  authenticates with an HMAC key. One scheme would conflate the two models.
- **Different directions.** A and B are inbound (need middleware); C is outbound (needs
  a signer called at dispatch).
- **Different key sources.** Sessions have a `SigningKey`; devices have a
  `CommandSecretHash`; API keys will get their own. One key store does not fit all three.
- **Different canonical formats.** Newline-delimited, comma-delimited `t=,v1=`, and
  pipe-delimited, each chosen for its transport and validation needs.

---

## 5. Where the device's key comes from

The device's `CommandSecretHash` is a deterministic `SHA-256` of the plaintext command
secret issued at registration. Both sides derive the same HMAC key from it, and the
server **never stores the plaintext** — it stores the hash. That is what lets the server
sign a frame it cannot decrypt a secret for, and it is the same "nothing to intercept"
principle as the enrollment keypair.

This is also the seam the registration contract retires: the keypair flow replaces the
transmitted shared secret with a keypair that never moves. Until that retirement lands
on every path, the HMAC domains above are how the device and server authenticate each
other.

---

## 6. Risk, audit and cancellation

- **Risk gate.** A command is classified before it is created. If it needs confirmation
  and no valid token is presented, the handler returns `425 Too Early` and never creates
  the command. See `RISK_AND_CONFIRMATION.md`.
- **Audit.** Every execution attempt — success, blocked or failure — writes an audit
  entry with the command, device, risk tier, result and the state transition. See
  `AUDIT_SYSTEM.md`.
- **Status.** `GET /v1/command/:dispatchId/status` returns the current state, scoped to
  the caller's org.
- **Retry.** `POST /v1/command/:dispatchId/retry` creates a *new* command with the same
  parameters; the original stays in its terminal state.
- **Cancellation.** `DELETE /v1/command/:dispatchId` cancels a pending or delivered
  command, and notifies the device if it is online.

---

## 7. The app commands specifically

The app runtime adds two commands on top of the generic set:

- `APP_START` — start a named app;
- `APP_STOP` — stop a named app.

They arrive as signed commands on the WebSocket, are executed by Layer 1's dispatcher,
and reach the supervisor's control socket. They are ordinary executors on the same seam
as `SHELL_EXEC`. See `APP_RUNTIME.md` for what the supervisor does with them, and
`CONTAINER_ANDROID_APIS.md` for the API bridge that is a different surface entirely
(called *by* the container, rather than *sent to* it).
