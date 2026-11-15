# The deploy surface — server endpoints

**Status:** shipped. Handlers live in `apps/api/internal/api/handlers/deploy/`, the
domain in `apps/api/internal/deploy/`, and the routes are registered in
`apps/api/internal/api/server_routes.go` (`setupDeployRoutes`).
**Related:** `DEPLOY_PIPELINE.md` (what these endpoints do, in order),
`CLI_REFERENCE.md` (the client that calls most of them), `DEPLOY_SECRETS.md` and
`DEPLOY_CREDENTIALS.md` (the two side channels).

This is the endpoint reference for the deploy feature. Every route here is under the
server's `/v1` prefix. The CLI calls most of them; the device calls exactly two.

---

## 1. The groups, and who may call what

There are three audiences, and each has its own authentication. Getting this right is
most of understanding the surface.

| Group | Registered at | Authentication | Who calls it |
|:--|:--|:--|:--|
| **Deploy (user)** | `/v1/deploy` | tenant auth (session cookie or user API key) **or** a deploy credential, plus organization context | The CLI, and the console |
| **Public** | `/v1/deploy/schema/:version` | none | The CLI, before it holds any credential |
| **Device-signed** | `/v1/device/:imei/secrets/:app/:key` | the device's own HMAC/keypair | The device, and only the device |

The user group carries three pieces of middleware stacked:

1. `DeployCredentialAuth` — resolves a presented `X-Vyzorix-Deploy-Credential` to a
   device, so a CI token can reach this group with no session.
2. Organization context — every handler can read the org it is scoped to.
3. The tenant group's own auth (session or API key).

The device-signed route sits on a *different* group, because the device authenticates
with its own credential, not a user session — and a user has no device
private key, so the two authenticators must not be confused. See
`DEVICE_COMMANDS_AND_SIGNING.md`.

---

## 2. Deploying

### `POST /v1/deploy`

The one endpoint that changes anything. It authenticates, authorises, compiles, signs
and persists.

**Auth:** user session/API key, or a deploy credential (which pins the device).

**Status:** `202 Accepted` — the deploy is stored and will be applied by the device; a `202` rather than a `200` because nothing has run yet.

**Query:** `?dry_run=true` compiles and signs but stores and delivers nothing.

**Body:** the `vyzorix.toml` bytes, `Content-Type: application/toml`.

**Response (202 Accepted):** — the deploy is accepted and stored; the device applies it on its next connection. Nothing has run yet.

```json
{
  "version": 7,
  "authorizedBy": "op_01H...",
  "artifacts": 3,
  "dryRun": false,
  "document": "' vyzorix.v ...",     // the compiled, signed config
  "apps": [
    { "name": "vfd",  "custom": false },
    { "name": "wiki", "custom": true  }
  ]
}
```

- `version` is the server-assigned, monotonic version.
- `authorizedBy` is the user who authorised it (injected into the config as
  `memberId`).
- `custom` marks an app the catalog does not know. It is a review aid, not a gate.

**Refusals** (mapped centrally in `vyzorix_deploy_error.go`):

| Condition | Status | Body |
|:--|:--|:--|
| Not authenticated | `401` | authentication required |
| Device not in the caller's org | `404` | not found — never `403`, to avoid an existence leak |
| Device not registered | `409` | device is not registered |
| No `device.deploy` grant | `403` | device.deploy is required for this device |
| Manifest invalid | `422` | a validation error naming the field |
| Config would carry a secret | `422` | a validation error naming the key |
| A named secret is not set | `404` | structured |
| Bad secret / credential request | `400` | structured |

---

## 3. Identity

### `GET /v1/deploy/whoami`

Resolves the credential and reports who it is — the call `vyzorix login` makes to prove
a credential works before saving it.

**Query:** `?device=<id>` (optional) — include that device's tier.

**Response:**

```json
{
  "operatorName": "Ada",
  "email": "ada@example.com",
  "role": "admin",
  "orgId": "org_01H...",
  "orgName": "Acme",
  "device": { "id": "d_01H...", "name": "field-ops-07", "tier": "2" }
}
```

A deploy credential has no user behind it (it is a CI token, not a person), so
`operatorName` and `email` are empty. A credential also cannot be steered to a
different device: naming a device other than the credential's own returns `404`.

---

## 4. Reading the current config

### `GET /v1/deploy/:imei`

The current config's metadata and document, as the console shows it.

**Response:**

```json
{ "version": 7, "document": "' vyzorix.v ...", "appliedVersion": 7 }
```

### `GET /v1/deploy/:imei/document`

The signed config **verbatim**, `Content-Type: text/plain`, exactly as the device
fetches it. It is separate from the previous endpoint so a caller that only wants the
document does not receive the metadata wrapper, and so the bytes can be diffed against
what a device holds.

---

## 5. The manifest schema (public)

### `GET /v1/deploy/schema/:version`

Serves the manifest JSON Schema for a version, generated from `vyzorix-spec`.

**Auth:** none. The CLI fetches this **before** it holds any credential, so it must be
public. The schema carries no `$id` and every `$ref` in it is fragment-only, so the CLI
keys it by the URL it fetched from — which is why a self-hosted server needs no special
case.

**Responses:** `200` with the schema, or `404` for an unknown version. A `404` here is
how the CLI refuses to validate against a shape the server no longer uses.

---

## 6. The device fetches its own secret

### `GET /v1/device/:imei/secrets/:app/:key`

Returns the **ciphertext** of one secret to an authenticated device.

**Auth:** the device's own credential (this route is on the device-signed group, beside
`/v1/device/:imei/config`). It is here, not under `/v1/deploy`, because it is the
*device* that asks — the operator-facing `set`/`list`/`unset` live under `/v1/deploy`.

Two gates protect it:

1. The route is bound to `:imei` by the device's authentication, so a device cannot ask
   for another device's secret.
2. The handler checks the key against the **current signed config**: a device cannot ask
   for a secret its config does not name.

The response is ciphertext, wrapped to the device's key. The plaintext never exists on
the server. See `DEPLOY_SECRETS.md`.

---

## 7. The routes that are *not* deploy routes but complete the picture

| Route | Group | Why it matters here |
|:--|:--|:--|
| `GET /v1/device/:imei/config` | device-signed | The pull half of delivery. A device that was offline fetches its config by version here. |
| `GET /v1/device/:imei/stream` | device-signed | The WebSocket. The `device_config` push and the `device_ready` report travel over it. |

Both are documented in `DEVICE_COMMANDS_AND_SIGNING.md` and
`WEBSOCKET_SYSTEM.md`.

---

## 8. Wire shapes

Two DTOs are worth memorising, because they appear on both sides:

**The deploy response** (above) — what the CLI prints and what a console shows.

**The deploy report frame** — what the device sends after an apply, over the WebSocket:

```json
{ "type": "device_ready", "version": 7, "success": true, "reason": "", "detail": "" }
```

A refusal sends the same frame with `success = false`, a machine-readable `reason`
(§10 of `DEPLOY_PIPELINE.md`), and a `detail` that names the app or artifact. The
server's handler routes on `type` and records the applied version on success.
