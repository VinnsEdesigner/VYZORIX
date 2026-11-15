# The deploy pipeline, stage by stage

**Status:** shipped on both sides. The server compiles and signs; the device verifies
and applies. The CLI drives it.
**Related:** `MANIFEST_REFERENCE.md` (the input), `CLI_REFERENCE.md` (the command),
`DEPLOY_SERVER_API.md` (the endpoints), `DEPLOY_SECRETS.md` (the secret channel),
`DEPLOY_CREDENTIALS.md` (the CI path), `DEPLOY_BUILD.md` (producing the artifacts this
pipeline delivers). This is the authoritative walkthrough of the deploy path.

This document walks the whole path a deployment takes, from the file on the user's
laptop to apps running on a phone, and says at each stage **who decides** and **what
happens when it goes wrong**. The stages are numbered because the documentation and the
code both refer to them.

---

## 0. The one idea that shapes everything

Fly's model is: a CLI talks to an authoritative control plane, the control plane
decides, and the change lands. That model does not transfer, because **a device is
intermittent.** Fly's server is the gate at deploy time; a phone that has been offline
for a week cannot ask. So the authority cannot be a check the server performs when it
delivers.

It has to be something the device can verify **alone, later, with no network.** That is
a signature over a document the device can re-verify at any time. Everything below
follows from that one constraint.

---

## 1. The shape of a deploy

```
 user                     server                        device
 ────────                     ──────                        ──────
 vyzorix.toml
   │ vyzorix deploy
   ├──────────────────────────► POST /v1/deploy
   │                            authenticate  (session or deploy credential)
   │                            authorise     (super_admin, or a device.deploy grant)
   │                            bind          (device ∈ org, and registered)
   │                            compile       (toml → vyzorix.v config)
   │                            refuse        (secret, unhashed ref, cmd escape)
   │                            sign          (server key, version++)
   │                            persist       (current + last-good, keep 2)
   │◄────────────────────────── 202 {version, artifacts, authorizedBy, apps}
   │
   │                            ── device_config frame ─────────►  verify signature
   │                                                                 reject if secret
   │                                                                 fetch artifacts
   │                                                                 verify each sha256
   │                                                                 apply atomically
   │                                                                 write apps.json
   │                                                                 generate manifest
   │◄──────────────────────────  device_ready {version} ────────────  start apps
   │                            record applied version
```

A device that was offline re-fetches on reconnect: it compares the `version` it holds
to the one the server reports, and asks for the config when they differ (§5).

---

## 2. Stage 1 — the user writes a manifest

One file, `vyzorix.toml`, declaring the target device and the apps that should run on
it. It is the only artifact a human authors. The full field list is
`MANIFEST_REFERENCE.md`.

The important property of this stage is that the file is *declarative*. It says what
should run, not how to make it run. Building the artifact is the user's job, done
somewhere else, and the manifest references the result by URL and digest.

---

## 3. Stage 2 — the CLI validates locally

Before anything is uploaded, the CLI:

1. parses the TOML (a syntax error fails here, by name);
2. reconciles the flags against the file (a `--device` that disagrees with
   `device.id` is refused);
3. fetches the server's JSON Schema for the pinned version and validates the manifest
   against it;
4. prints a summary of what the file declares.

The point of validating locally is speed and clarity: a schema error becomes a
readable field path on the user's terminal, rather than a round trip that ends in a
refusal. But the local check is a convenience, **not** the authority — the server
re-validates everything it receives, because the server is what signs.

The CLI uploads the file's **bytes, unchanged**. It never re-serialises the manifest,
because the digest and the signature are computed over the bytes the server receives,
and a re-serialised file is a different file.

---

## 4. Stage 3 — the server authenticates and authorises

`POST /v1/deploy` runs on the tenant group, so the caller is authenticated by a session
cookie or a user API key. A device-scoped deploy credential is also accepted
(§7.4 of the contract), so CI can deploy with no user present.

Authorisation is a single predicate. A user may deploy to device `D` in org `O`
when **all** of the following hold:

```
  caller is authenticated
    AND D ∈ O                                     (binding)
    AND D.lifecycle == registered                 (not pending / deregistered)
    AND ( caller is a platform super_admin
          OR  ∃ grant g : g.subject == caller
                          ∧ g.action  ⊇ device.deploy
                          ∧ g.scope   ⊇ devices:D )
```

Three things about this predicate are worth stating plainly:

- **Both halves are required.** An unauthenticated caller is not helped by a grant, and
  a super-admin with no grant is still bound by `D ∈ O`.
- **`super_admin` is platform-global.** There is no per-org super-admin in this role
  model, and the predicate does not pretend there is. The org binding is what keeps it
  honest.
- **A device in another org returns `404`, not `403`.** A `403` would confirm the device
  exists, which is an existence leak across tenants.

`device.deploy` is a *new* action, granted explicitly or not at all. It is deliberately
not folded into `apps.write`, because `apps.*` is about *querying what an app is doing*,
not about changing what runs — folding deploy into it would make the permission that
lets a user *see* an app also let them *install* one.

---

## 5. Stage 4 — the server compiles the manifest

The manifest becomes a **config**: the `vyzorix.v` document the registration contract
defines, extended with the apps the manifest declares. The `.toml` never leaves the
user's machine; the config is what is signed, delivered and stored.

Compilation does three things beyond transcription:

1. **Injects the fields the file may not set.** `memberId` (the authorising user),
   `version`, and each app's `dir`.
2. **Runs the refusal checks** (§6).
3. **Assigns the next version.** The version is monotonic per device, so a device can
   tell "this is newer than what I hold" by the number alone.

The document is byte-identical for the same manifest: apps in manifest order, children
in a fixed field order, not map order. That is what makes "the same file compiles to
byte-identical output" testable.

The config is a line-based tree, not JSON:

```text
' vyzorix.v — device configuration
' Generated by the server. Signed; a tampered field fails verification.
v device d_01H...
vv org org_01H...
vv name field-ops-07
vv tier 2
vv member op_01H...
vv apps
vvv app vfd
vvvv dir /home/apps/vfd
vvvv cmd /home/apps/vfd/run.sh
vvvv port 8080
vvvv artifact engine
vvvvv arch arm64-v8a
vvvvv url https://.../vyzor-arm64.tar.gz
vvvvv sha256 9f2c...
v version 7
v sig ecdsa-p256 <base64>
```

Depth is the number of leading `v` characters; a node is `key value` (or just `key`
for a container). A list is a repeated node of the same key under one parent, in
document order. **A reader must iterate the app nodes, not flatten** — reading an app's
field by absolute path crosses every app and mixes them up.

---

## 6. Stage 5 — the refusals

The compiler refuses a config that would be unsafe, and names what it refused. These
checks are the load-bearing part of the contract; each one closes a specific hole.

| Refusal | Rule | Why it exists |
|:--|:--|:--|
| **Carries a secret** | `[app.env]` may not hold a secret-shaped key or value. | The config is signed, logged, served and displayed — a secret in it is a secret in five places. |
| **Unhashed reference** | Every artifact carries a `sha256`. | The signature covers the document; a URL without a digest trusts the host to be honest forever. |
| **`cmd` escape** | `cmd` must stay inside the app's own directory, with no shell metacharacters. | A signed manifest still runs code; the rule reduces it to code the manifest also shipped. |

The secret check is **structural, not a substring scan.** A scan for the word "secret"
across the whole document is what the shipped code did before apps existed, and it
cannot survive an `apps` section — the schema's own keys (`apps`, the `secret NAME`
node, `secret_keys`) contain those words by definition. So the check walks the parsed
document:

1. Schema keys the compiler *writes* are exempt by path, not by pattern.
2. Every `env` value is inspected for the shapes a secret takes (PEM, JWT, long
   hex/base64, `user:pass@`).
3. Every `env` key is inspected against `(?i)(password|passwd|secret|token|api[_-]?key|private[_-]?key|credential)`.
4. Every other node's value is inspected for the shapes in (2).
5. The `secret NAME` nodes are exempt — but only their *value*, and only because a name
   is a name. A `secret` node whose value looks like a secret is refused.

The consequence is that `[app.secrets] DATABASE_URL = ""` is accepted (it is a name)
while `[app.env] S3_SECRET_KEY = "AKIA…"` is refused (it is a value). The device
re-checks the same rule before storing, and reports `config_carries_secret` with the
key.

---

## 7. Stage 6 — the server signs and persists

The compiled config is signed with the server's key (the pinning story is
`DEVICE_ONBOARDING.md` §7 and the registration contract). The version is bumped, and
the result is persisted as **current** while the previous one is kept as **last-good**.
Retention is two, not N: the recovery path that matters is "put back what worked", one
prior version answers it, and disk is the scarce resource on a phone.

The signature is what makes the config trustworthy on a device that was offline when it
was issued. The device holds the server's public key, pinned in the app, and verifies
the signature itself.

---

## 8. Stage 7 — delivery

Delivery is a **push with a pull fallback**:

- If the device is connected right now, the server pushes a `device_config` frame over
  the WebSocket it already holds. This is the fast path and the common one.
- If the device is offline, nothing is pushed, and that is not an error. The device
  re-fetches its own config on reconnect: it asks the server for the current version,
  compares it to the one it holds, and pulls the document when they differ.

The pull path is what makes the intermittent-device model work. A deploy to an offline
device is a deploy that lands the next time the device comes up, with no retry loop and
no queue.

---

## 9. Stage 8 — the device verifies and applies

The device runs the same sequence every time, and the order matters:

1. **Verify** the signature against the pinned key. Refuse `unsigned`,
   `bad_signature`, `carries_secret`, `unhashed_reference`, `cmd_escape`.
2. **Fetch** each artifact to a temporary path, resuming a partial transfer where one
   exists.
3. **Verify** each artifact against its `sha256` — over the *whole* file, not just the
   newly received part of a resumed transfer.
4. **Install** atomically: unpack to `<dir>.new`, then `rename` over `<dir>`. A rename
   is atomic, so an interrupted install cannot leave a half-written app.
5. **Write** `apps.json` atomically (temp-then-rename) with `name`, `dir`, `cmd`,
   `port`, `env`, `autostart` per app.
6. **Generate** `site.webmanifest` for each webroot with `pwa` enabled.
7. **Signal** the supervisor, whose `inotify` path picks the change up without a
   restart.
8. **Report** `device_ready {version}`.

The device pings the supervisor **first**. A config that lands with no supervisor
running would leave every app silently stopped, so an unreachable supervisor is a
refusal (`supervisor_unreachable`), not a proceed-and-hope.

---

## 10. Stage 9 — the outcome, and what a failure costs

A successful apply reports `device_ready {version}` with `success = true`, and the
server records the applied version.

A failed apply changes **nothing**:

- the current and previous configs are both retained;
- an artifact that fails its digest aborts *that artifact*, leaves the previous copy in
  place, and reports `digest_mismatch` naming the artifact;
- an interrupted install cannot leave a half-written app, because the install is a
  rename;
- a config that fails verification is refused before anything is fetched.

The device keeps running what it was running, and reports the refusal with a distinct
reason. **The refusal is the product** — "your deploy did not apply, and here is exactly
why" is worth more than a silent retry.

The refusal reasons a device can report, and what each means:

| Reason | Meaning |
|:--|:--|
| `unsigned` / `bad_signature` | The config did not verify against the pinned key. |
| `carries_secret` | A node carried what looks like a secret value. |
| `unhashed_reference` | An artifact had no digest. |
| `cmd_escape` | `cmd` resolved outside the app's directory, or had metacharacters. |
| `cmd_not_executable` | `cmd` did not exist or was not executable after install. |
| `no_artifact_for_arch` | No artifact matched this device's ABI. |
| `digest_mismatch` | A fetched artifact did not match its `sha256`. |
| `supervisor_unreachable` | The app runtime did not answer a ping. |
| `secret_undecryptable` | A secret would not open; the app is **not** started. |

---

## 11. What is removed, and why there is no `undeploy`

An app present in the old config and absent from the new one is **stopped and
deregistered**; its directory and data remain. Only `purge = true` in the manifest that
removes it deletes the data — and that deletion is therefore recorded in a document the
user wrote and the server signed.

There is deliberately no `vyzorix undeploy`. A destructive verb with no reviewable
artifact is how data gets deleted by accident.

---

## 12. Per-app state, not per-device

One device runs several apps, and they fail independently, so the state machine is per
app:

```
 (absent)
     │  appears in a signed config
     ▼
  STOPPED ──── autostart ────► STARTING ──► RUNNING
     │                            │            │
     │                            │ crash      │ exit 0 (one-shot)
     │                            ▼            ▼
     │                         BACKOFF      STOPPED
     │                            │
     │                            │ failures > N
     │                            ▼
     │                         FAILED ──► (reported; no auto-heal)
     │
     │  absent from the next signed config
     ▼
 (stopped, deregistered; data retained unless purge)
```

`BACKOFF` is the supervisor's exponential restart, capped. `FAILED` is reported to the
server so the console can show it; nothing restarts it automatically. See
`APP_RUNTIME.md` for the supervisor itself.

---

## 13. Honest limits

- **Offline first-deploy is not supported.** A device that has never been online has no
  pinned config and no artifacts. Enrollment already costs one round trip; every deploy
  after that is verifiable offline.
- **`cmd` is still a code-execution surface.** The rules reduce it to code the manifest
  also shipped, but a signed manifest still runs code on someone's phone. The remaining
  control is *who may sign*.
- **No automatic rollback.** The last good config is retained and restorable; restoring
  it automatically when an app crash-loops is a later feature.
- **`expose = "cloud"` is refused in v1.** It needs the wildcard-DNS/TLS proxy that is
  deferred. `loopback` and `lan` are supported.
- **No group or fleet deploy.** One device per deploy.
- **Version history is two.** "Roll back three deploys" is not possible.
- **Secrets are per device, not shared.** A secret set for device A does not apply to
  device B, because it is encrypted to A's key. A fleet of ten means ten calls, or a
  loop.
