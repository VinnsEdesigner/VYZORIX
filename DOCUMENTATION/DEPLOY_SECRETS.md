# Secrets — an encrypted side channel to an app

**Status:** shipped on both sides. Server: `apps/api/internal/deploy/vyzorix_deploy_secrets.go`
and the secret store. Device: `cloud-connector/.../secret/` and the `.env` write in
`DeployApplier`.
**Related:** `MANIFEST_REFERENCE.md` §5 (the `[app.secrets]` table),
`CLI_REFERENCE.md` §5 (the commands), `DEPLOY_PIPELINE.md` §6 (why `env` refuses
secrets).

---

## 1. The problem this solves

An app that is itself a server needs credentials: a database URL, an API token, a
signing key. `[app.env]` is exactly where a user would put them, and the compiler
refuses it — rightly, because the config is signed, logged, served by
`GET /v1/deploy/{device}` and shown in a console, so a secret in it is a secret in five
places.

But a rule with no alternative is not a design. This is the alternative: **a secret
travels on its own channel, encrypted to the device, and is never in the signed config.**

---

## 2. The rule

**`[app.env]` is for configuration. A secret goes through `vyzorix secrets` and lands
in a file only the app reads.**

The compiler's refusal is unchanged and stays. What is added is somewhere for the
secret to go.

---

## 3. The journey of one secret

```
 vyzorix secrets set vfd DATABASE_URL=... --device d_01H...
   │
   ├─ server: encrypt to the DEVICE's public key      (RSA-OAEP-SHA1)
   │           store the CIPHERTEXT at rest            (AES-GCM at the store)
   │           record key, app, device — never the plaintext
   ▼
 device_config frame        the signed config: names the KEYS, not the values
   │
   ▼
 agent: sees a key it does not hold → asks → server returns CIPHERTEXT
        │
        ├─ decrypt with the device key                 (Keystore, PURPOSE_DECRYPT)
        ├─ write files/apps/<app>/.env, mode 0600
        └─ report secrets_ready {app, keys[]}
```

Two properties fall out of that shape and are the whole point:

- **The plaintext never exists on the server.** It is encrypted before it is stored,
  and decrypted only on the device. A server compromise that reaches the ciphertext
  store yields nothing for a device whose key the attacker does not have.
- **The config names the keys, not the values.** The device knows which secrets it is
  missing, and the console can show *which* secrets an app expects, without showing any
  of them.

---

## 4. Which key, and why not the identity key

A secret is encrypted to a key the agent holds **for exactly this purpose**: an
RSA-3072 keypair in the Android Keystore with `PURPOSE_DECRYPT`, generated on first run,
non-exportable, and **separate from the identity key**.

Three reasons the envelope is RSA rather than ECDH, and three reasons it is a second
key rather than the identity key:

1. **`PURPOSE_AGREE_KEY` is API 31**, and the app has `minSdk = 26`. ECDH in the
   Keystore is unavailable on Android 8–11. `PURPOSE_DECRYPT` is API 23, so RSA works on
   every device in range.
2. **The identity key cannot be reused.** It is generated with `PURPOSE_SIGN` only, and
   widening its purpose would change what that key *is*. More importantly, its public
   half is deliberately public — exposed to the container and sent at enroll — so
   encrypting a secret to it would rest confidentiality on a key the whole system knows.
3. **The container must not be able to decrypt.** proot runs with `-0`, so the
   container is *root inside*; anything the container can read, a compromised app can
   read. Decryption happens in the agent process, and only the plaintext `.env` reaches
   the container.

So there is a **second Keystore entry**, `vyzorix.secret`, whose public half is sent at
enroll exactly as the identity key's is. The agent never exposes it over the bridge —
there is no `SecretApi`.

### The OAEP digest is SHA-1, and that is a constraint, not a preference

The Android Keystore accepts only `MGF1-SHA1` for OAEP on a non-exportable key — a
spec with `MGF1ParameterSpec.SHA256` is refused outright. And Go's `rsa.EncryptOAEP`
uses the supplied hash for both the OAEP digest and MGF1, so the only pair both sides
can agree on is SHA-1 for both. This was **verified on the emulator**, not inferred: Go
wrapped a secret to the device's Keystore public key and the Keystore returned it.

SHA-1 in OAEP is the standard default and is not a practical weakness here — the
3072-bit modulus is the protection — but it is recorded because the obvious "harden it
to SHA-256" edit breaks every device, and a comment is cheaper than that outage.

---

## 5. What lands on disk

The plaintext goes to `files/apps/<app>/.env`, mode `0600`, and **not** into
`apps.json`. The registry is inside `/home/apps`, which is bound into the container and
readable by every app running there — a secret in it would hand one app another's
credential.

`apps.json` holds `env` (configuration, from the config) and a list of `secret_keys`
(names only). That distinction is the point: the registry is inspectable, so it must not
be worth inspecting.

The supervisor reads the `.env` at start and passes it to the process; it does not copy
it into the registry. See `CONTAINER_FILESYSTEM.md` for where these paths sit.

---

## 6. Rotation, and the failures that matter

- **A secret is replaced, not merged.** `secrets set` for an existing key overwrites the
  ciphertext and bumps the app's secret generation. The device re-fetches when the
  generation differs from the one it holds.
- **A wiped device has lost its Keystore key**, so it cannot decrypt anything sent
  before the wipe. It re-enrolls as a new identity and the operator re-sets the secret.
  There is no recovery of a secret encrypted to a key that no longer exists — by design.
- **A secret the device cannot decrypt is a refusal, not a silent skip.** If the
  ciphertext will not open, the agent reports `secret_undecryptable {app, key}` and
  **does not start the app**. Starting it with the variable absent is the failure mode
  that produces an app that looks up but talks to the wrong thing.

---

## 7. The commands and the endpoints

```
vyzorix secrets set   <app> KEY=VALUE --device <id>     # encrypt, store, bump generation
vyzorix secrets list  <app> --device <id>               # names + generation, never values
vyzorix secrets unset <app> KEY    --device <id>
```

| Endpoint | Group | Returns |
|:--|:--|:--|
| `POST /v1/deploy/{device}/secrets/{app}` | operator | nothing; stores the ciphertext |
| `GET /v1/deploy/{device}/secrets/{app}` | operator | names and generation **only** |
| `DELETE /v1/deploy/{device}/secrets/{app}/{key}` | operator | nothing; removes one |
| `GET /v1/device/{imei}/secrets/{app}/{key}` | device-signed | the **ciphertext**, to the device |

`set`/`unset` are authorised by `device.deploy` — the same grant as a deploy, because
writing a credential is the same kind of act as writing what runs.

A value is never logged, never returned by any `GET`, and never written into any state
the deploy feature reads back. `list` shows names, which is what an operator needs to
confirm the right thing is set.

---

## 8. What this protects, and what it does not

**Protects:**

- a secret never sits in a git repository;
- it never travels in the signed config;
- it never appears in a log or a console response;
- it is encrypted at rest on the server;
- it is encrypted in transit to a key only the agent holds — so a server compromise that
  reaches the ciphertext store does not yield a plaintext secret for a device whose key
  the attacker does not have.

**Does not protect:** the device itself. A rooted phone, or the operator's own container
running as fake-root, can read `files/apps/<app>/.env` once the app has started. This is
the same limit as every secret on a device the operator controls, and it is stated
rather than implied. The envelope defends the *transport and the store*, not a
compromised endpoint.

---

## 9. The limits, stated plainly

- **Secrets are per device, not shared.** A secret set for device A does not apply to
  device B, because it is encrypted to A's key. A fleet of ten means ten `secrets set`
  calls, or a loop.
- **A wiped device loses its secret key**, so old ciphertext is unrecoverable. The
  device re-enrolls and the operator re-sets the secret.
- **`app.secrets` names must be set before the app will start.** An app that expects a
  secret it does not have will not be started with the variable absent.
