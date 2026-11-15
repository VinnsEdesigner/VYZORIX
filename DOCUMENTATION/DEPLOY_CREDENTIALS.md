# Deploy credentials — CI without a user

**Status:** shipped. The surface is `apps/api/internal/api/handlers/deploy/vyzorix_deploy_credential_handler.go`
and `apps/api/internal/deploy/vyzorix_deploy_credential.go`.
**Related:** `DEPLOY_PIPELINE.md` §4 (authorisation), `CLI_REFERENCE.md` §3 (the
`--deploy-credential` flag), `DEPLOY_SECRETS.md` (the other side channel).

---

## 1. What a deploy credential is, and why it exists

The normal way to deploy is a user signing in, which is right for a person and
wrong for a pipeline. A CI job should not hold a person's API key, and it should not be
able to deploy to the whole fleet if it is compromised.

A **deploy credential** is a long-lived token scoped to exactly **one device**. It is
minted by a user, handed to a pipeline, and used in place of a session. It exists
so `vyzorix deploy` can run in CI with no human present and no broad credential.

The design borrows the reasoning behind an app-scoped Fly token: a compromised
credential should reach one device's config, not the fleet.

---

## 2. The properties, and why each one

| Property | What it means |
|:--|:--|
| **Scoped to one device** | Refused for any other device, even one in the same org. A credential cannot be steered. |
| **Revocable** | A revoked credential is refused immediately — there is no cache window to wait out. |
| **Shown once** | The plaintext token is returned exactly once, at mint time. No `GET` ever returns it again. |
| **Named** | A credential carries a name, so it can be revoked by intent rather than by its prefix. |
| **Displayed by prefix** | Every read returns the non-secret prefix and the metadata, so a user can tell credentials apart without holding any of them. |

The last two matter operationally: an audit that says "revoke the one the nightly job
uses" needs a name, and a list that showed full tokens would be a list worth stealing.

---

## 3. The lifecycle

```
  user mints ──► token returned ONCE ──► stored in CI secret ──► used by deploy
       │                                                                  │
       │  list (prefix, name, dates, never the token)                     │
       │                                                                  │
       └── revoke ────────────────────────────────────► refused immediately
```

### Minting

```
POST /v1/deploy/{imei}/credentials
{ "name": "nightly" }
```

**Auth:** the same `device.deploy` grant that authorises a deploy to that device. If
you may deploy to it, you may mint a credential for it — they are the same kind of act.

**Response (201):** the plaintext `token`, plus the metadata. This is the only time the
token appears.

### Listing

```
GET /v1/deploy/{imei}/credentials
```

Returns, per credential: `id`, `deviceId`, `name`, `prefix`, `createdBy`, `createdAt`,
and `lastUsedAt` / `revokedAt` when set. **Never the token.**

`lastUsedAt` is the field a user watches to answer "is this credential still in
use, or can I revoke it?".

### Revoking

```
DELETE /v1/deploy/{imei}/credentials/{id}
```

Immediate. A revoked credential fails authentication on its next use; there is no
grace period and no cache to expire.

---

## 4. How it authenticates

A deploy credential is presented in the `X-Vyzorix-Deploy-Credential` header. The
`DeployCredentialAuth` middleware resolves it to a device and puts that device on the
request, so every handler downstream is already bound to the right device.

This is different from a user API key, which authenticates by presenting the key
(`X-API-Key`) **and** an HMAC signature over the request. The reason for the difference
is that the two are different things: an API key is a person's or a key's credential and
signs as that identity; a deploy credential *is* the device scope, so presenting it is
the whole proof.

The CLI picks one or the other and refuses to be given both:

```sh
vyzorix login --server ... --deploy-credential "$TOKEN"
```

Giving both an API key and a deploy credential is refused rather than guessed at,
because the server would otherwise have to choose which identity the caller meant.

---

## 5. What a credential can and cannot do

**Can:** deploy to its one device, set/list/unset that device's secrets, read its
identity, read its config.

**Cannot:** deploy to any other device (refused, `404` for a device outside the scope);
mint or revoke credentials (that is a user action under `device.deploy` with a
session — a CI token should not be able to create more of itself); reach any other part
of the API.

The narrowness is the point. A pipeline needs exactly the ability to push to one device,
and that is exactly what it gets.

---

## 6. Operational notes

- **Store the token as a secret in your CI system**, never in the repository. It is
  returned once; if you lose it, revoke and mint a new one rather than hunting for it.
- **Prefer one credential per pipeline**, named for the pipeline, so revoking one does
  not disturb another.
- **Watch `lastUsedAt`.** A credential that has not been used in months is a candidate
  for revocation, and the list tells you without revealing anything.
- **A revoked credential is refused at once.** If a pipeline suddenly gets a `401`, the
  first thing to check is whether the credential was revoked — the second is whether the
  device was deregistered, which also invalidates the scope.
