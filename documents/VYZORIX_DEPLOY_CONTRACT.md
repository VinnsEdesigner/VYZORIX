# Vyzorix Deploy Contract

**Status:** design, agreed in outline. Nothing built except the secrets channel (§9b),
committed on a branch and not on `main`.
**Parent:** `VYZORIX_DEPLOY_REQUIREMENTS.md` (this repo, `documents/`) — the requirements
and the resolved decisions. This document is the *contract*: the exact shapes, the
rules that must not bend, and what is written where.
**Sibling:** `vyzorix-android/doc/REGISTRATION_CONTRACT.md` — how a device joins an
account. This contract begins where that one ends: the device is `registered`, holds a
keypair, and can verify a signed config.
**Dependency:** §9 of the registration contract is an **open decision** (retire HMAC
for the keypair, or keep HMAC with envelope-encrypted delivery). This contract assumes
the keypair, which is that document's recommendation, and says in §10 what changes if
the other is chosen.

This document defines one thing: **how a `vyzorix.toml` becomes apps running on a
device, and what makes that trustworthy.**

---

## 1. The problem the model must not repeat

The first shape of this feature was "deploy Vfd". That is wrong in a way that would
have been expensive to discover late: it makes the manifest a *config for one program*,
and every generalisation after that is a migration.

The correction is small to state and large in effect: **the manifest describes apps,
and Vfd is one of them.** The test is mechanical — a second app must be deployable with
no change to anything under `apps/api/`. If that is not true, the feature is hardcoded
and pretending otherwise.

There is a second trap, and it is the one that decides the whole design. The obvious
model is Fly's: a CLI talks to an authoritative control plane, the control plane
decides, and the change lands. That model does not transfer, because **a device is
intermittent.** Fly's server is the gate at deploy time; a phone that has been offline
for a week cannot ask. So the authority cannot be a check the server performs when it
delivers. It has to be **something the device can verify alone, later, with no network.**

That single constraint produces the three rules in §2, and everything else follows.

---

## 2. The one idea

**The operator declares; the server signs; the device verifies alone.**

Three rules, and they are not negotiable — each one closes a specific hole:

1. **A config is signed by the server and verified by the device** against a pinned
   public key. A device never acts on an unsigned document, and never on an instruction
   that arrived on a live socket. The socket *delivers* the config; it does not
   *authorise* it.
2. **A config carries references, never bytes.** Every artifact is `{url, sha256}`.
   The signature covers the digest, so the *transport* is untrusted by construction: an
   artifact may be fetched over plain HTTP and still be known-good.
3. **A config carries no secret.** If it could, the delivery channel would inherit the
   problem the keypair model exists to remove. The server refuses to issue one; the
   device refuses to store one. Both sides check, because one bad deploy must not be
   able to break the invariant.

Three consequences worth stating plainly, because they are why the rules earn their
keep:

- **Nothing secret is ever transmitted.** Same property the registration contract
  established for enrollment, now held for deploy.
- **The device can be offline for a week and still decide correctly.** It holds a
  signature and a pinned key; both are local.
- **The server stays authoritative.** The device reports facts and refusals; the
  operator decides what runs.

---

## 3. The flow

```
 operator                     server                        device
 ────────                     ──────                        ──────
 vyzorix.toml
   │ vyzorix deploy
   ├──────────────────────────► POST /v1/deploy
   │                            authenticate  (session)
   │                            authorise     (super_admin | grant)
   │                            bind          (device ∈ org)
   │                            compile       (toml → config)
   │                            refuse        (secret, unhashed ref, cmd escape)
   │                            sign          (server key, version++)
   │                            persist       (current + last good)
   │◄────────────────────────── 202 {version, artifacts, authorizedBy}
   │
   │                            ── device_config frame ─────────►  verify signature
   │                                                               reject if secret
   │                                                               fetch artifacts
   │                                                               verify each sha256
   │                                                               apply atomically
   │                                                               write apps.json
   │                                                               generate manifest
   │◄──────────────────────────  device_ready {version} ────────────  start apps
   │                            record applied version
```

A device that was offline re-fetches on reconnect: it compares the `version` it holds
to the one the server reports and asks for the config when they differ (§7).

---

## 4. The manifest — `vyzorix.toml`

The operator's file, and the only artifact a human writes.

```toml
[device]
id   = "d_01H..."            # required — the target
org  = "org_01H..."          # required — the owning organization
name = "field-ops-07"        # optional — a display name override

# One or more apps. Order is significant: it is the order the config records and
# the order the device applies.
[[app]]
name      = "vfd"                    # required, unique within the file
cmd       = "/home/apps/vfd/run.sh"  # required (see §8 for the rule)
port      = 8080                     # optional — the port the app listens on
autostart = true                     # optional, default true
expose    = "loopback"               # optional — loopback | lan | cloud
purge     = false                    # optional, default false — see below

  # Sub-tables and bare keys must precede the first [[app.artifact]]. Once TOML is
  # inside an array-of-tables element, a bare key or sub-table belongs to that
  # element, so `pwa = true` written after an [[app.artifact]] would set the
  # artifact's field, not the app's — and the manifest would be refused as an
  # unknown key. This is a real TOML rule, not a style choice.

  [app.env]
  MI_PORT = "8080"                   # configuration — safe to commit

  # Secrets are NAMES only, and the names are the KEYS of this table. The values
  # are set out of band (vyzorix secrets set) and never appear in this file. The
  # table form is deliberate: it is what lets the no-secret scan (§5.2) exempt
  # this one node by its position in the document, instead of pattern-matching a
  # field that is defined to hold secret names. See §9b.
  [app.secrets]
  DATABASE_URL = ""
  S3_SECRET_KEY = ""

  # A webroot may be installable, and that is a decision, not a default. The
  # boolean form (`pwa = true`) and the table form (`[app.pwa]`) are mutually
  # exclusive — both define the key `pwa` — so use exactly one. The table below
  # implies enabled.
    [app.pwa]                        # optional — overrides the generated defaults
    name             = "Vyzorix"
    short_name       = "Vyzorix"
    description      = "Download, transcode and deliver media."
    theme_color      = "#be123c"
    background_color = "#ededed"
    display          = "standalone"
    orientation      = "portrait"
    start_url        = "/"

  # Artifacts are references. `sha256` is mandatory. `arch` is optional and,
  # when present, restricts the artifact to one ABI — see §4.2.
  [[app.artifact]]
  name   = "engine"
  arch   = "arm64-v8a"
  url    = "https://github.com/VinnsEdesigner/vyzorix-engine-assets/releases/latest/download/vyzor-arm64.tar.gz"
  sha256 = "9f2c..."                 # arm64-v8a digest
  size   = 1882104                   # optional, informational

  [[app.artifact]]
  name   = "engine"
  arch   = "x86_64"
  url    = "https://github.com/VinnsEdesigner/vyzorix-engine-assets/releases/latest/download/vyzor-x86_64.tar.gz"
  sha256 = "b71e..."                 # x86_64 digest — same name, different arch

  [[app.artifact]]
  name   = "console"
  url    = "https://.../console.tar.gz"
  sha256 = "41ab..."
  kind   = "webroot"                 # optional — "webroot" unpacks and is served

[[app]]                              # a second app — this is what makes it a platform
name = "wiki"
cmd  = "/home/apps/wiki/run.sh"
port = 9000
expose = "lan"
pwa = true                           # the boolean form, for an app with no overrides

  [[app.artifact]]
  name   = "site"
  url    = "https://.../wiki.tar.gz"
  sha256 = "7b3d..."
  kind   = "webroot"
```

### 4.1 Field rules

| Field | Type | Required | Default | Notes |
|:--|:--|:--|:--|:--|
| `device.id` | string | yes | — | Must match a device the caller's org owns. |
| `device.org` | string | yes | — | Checked against the caller's org. |
| `device.name` | string | no | server's name | Display only. |
| `app.name` | string | yes | — | `[a-z0-9-]{1,32}`, unique in the file. Becomes the directory name. |
| `app.cmd` | string | yes | — | §8. |
| `app.port` | int | no | none | 1–65535. Required when `expose != "loopback"`. |
| `app.autostart` | bool | no | `true` | Whether the supervisor starts it. |
| `app.expose` | enum | no | `loopback` | `loopback` \| `lan` \| `cloud`. `cloud` is refused in v1 (§10). |
| `app.purge` | bool | no | `false` | The only thing that deletes an app's data. |
| `app.env` | map | no | `{}` | String→string. **Configuration only** — a value that looks like a secret is refused (§5.2); a secret goes through `vyzorix secrets` (§9b). |
| `app.secrets` | table | no | `{}` | A table whose **keys** are the secret names this app expects; every value must be the empty string. Names only — never a value (§9b). The table form is required so the scan in §5.2 can exempt this node structurally. |
| `app.artifact[].name` | string | yes | — | Unique within the app **per `arch`** (an unqualified artifact is unique per app). |
| `app.artifact[].url` | string | yes | — | `https` only. No template variables: the URL is literal and the digest pins its bytes (§4.2). |
| `app.artifact[].sha256` | string | **yes** | — | 64 hex. Mandatory — see §5.3. |
| `app.artifact[].arch` | enum | no | — | `arm64-v8a` \| `armeabi-v7a` \| `x86_64`. When present, the artifact applies only to that ABI (§4.2). |
| `app.artifact[].size` | int | no | — | Informational; a mismatch is not an error. |
| `app.artifact[].kind` | enum | no | `dir` | `dir` \| `webroot`. |
| `app.pwa` | bool or table | no | **`false`** | The boolean and the `[app.pwa]` table are mutually exclusive; both define `pwa`. See §6. |

**An unknown key is a hard error.** A typo'd setting must not silently do nothing —
`autostart` misspelled as `auto_start` would otherwise produce an app that never
starts and no message saying why.

**`app.name` is the directory name and the registry key.** It is constrained so it
cannot be a path, and it cannot be changed without removing and re-adding the app —
renaming an app is not a supported operation in v1, because the data directory would
have to move and that is a data-loss surface.

### 4.2 Per-architecture artifacts

An artifact may be restricted to one ABI with `arch`. The device fetches only the
artifacts that apply to it — an artifact with no `arch` applies to every device; one
with an `arch` applies only when it equals the device's ABI (`arm64-v8a`,
`armeabi-v7a`, or `x86_64`).

**A URL is never a template.** An earlier draft wrote
`vyzor-${abi}.tar.gz` with one digest, which cannot work: if the device expands
`${abi}` the expanded URL is not what the server signed, and if it does not, the URL
is wrong. The digest is the whole reason the transport can be untrusted (§5.3), so
the URL must be literal and the digest must pin exactly those bytes. The same binary
for several ABIs is therefore *several* `[[app.artifact]]` entries — same `name`,
different `arch`, each with its own URL and digest.

Two consequences the compiler must enforce:

- **Uniqueness is per `(name, arch)`.** `engine` for `arm64-v8a` and `engine` for
  `x86_64` are the same artifact in two builds, not a duplicate. Two entries with the
  same `(name, arch)` — or two unqualified entries with the same `name` — are an
  error naming the artifact.
- **A device with no matching artifact is refused**, with `no_artifact_for_arch`, at
  apply time. It is a device-side check because only the device knows its ABI; the
  server cannot know whether the fleet it is deploying to holds an `armeabi-v7a`
  handset. Compiling a manifest whose every `engine` is `arm64-v8a` is legal — it is
  simply not deployable to a 32-bit device, and the refusal says so.

---

## 5. The compiled config

### 5.1 What the server produces

The manifest compiles to a **config** — the existing `vyzorix.v` document shape the
registration contract defines (`REGISTRATION_CONTRACT.md` §6, as rendered by
`RenderConfig`), extended with the apps the manifest declares. It is the thing that
is signed, delivered and stored; the `.toml` never leaves the operator's machine.

**The config is a `vyzorix.v` document, not JSON.** The registration contract's §6
shows the config as JSON for readability, but the shipped format — and the only one
libv signs and verifies — is the line format below. The JSON in that section is an
illustration of the *fields*; it is not a wire shape and nothing serialises it.

```text
' vyzorix.v — device configuration
' Generated by the server. Signed; a tampered field fails verification.
v device d_01H...
vv org org_01H...
vv name field-ops-07
vv tier 2
vv member op_01H...
vv cloud
vvv stream /v1/device/d_01H.../stream
vvv hb 60s
vvv backoff 1s>30s
vv features
vvv terminal true
vvv usb false
vvv apps true
vv apps
vvv app vfd
vvvv dir /home/apps/vfd
vvvv cmd /home/apps/vfd/run.sh
vvvv port 8080
vvvv autostart true
vvvv expose loopback
vvvv purge false
vvvv env MI_PORT=8080
vvvv secret DATABASE_URL
vvvv secret S3_SECRET_KEY
vvvv artifact engine
vvvvv arch arm64-v8a
vvvvv url https://.../vyzor-arm64.tar.gz
vvvvv sha256 9f2c...
vvvvv kind dir
vvvv artifact console
vvvvv url https://.../console.tar.gz
vvvvv sha256 41ab...
vvvvv kind webroot
vvvv pwa true
vvvvv theme_color #be123c
vvvvv display standalone
vvv app wiki
vvvv dir /home/apps/wiki
vvvv cmd /home/apps/wiki/run.sh
vvvv port 9000
vvvv expose lan
vvvv artifact site
vvvvv url https://.../wiki.tar.gz
vvvvv sha256 7b3d...
vvvvv kind webroot
vvvv pwa true
v version 7
v sig ecdsa-p256 <base64>
```

The nodes above are `key value`; `member`, `dir` and the two `secret` values are
injected by the server, not the file. **Comments are whole-line only and start with
`'`** — a `'` or `#` *after* a value is data, not a comment (libv keeps it as a token,
so `port 8080 # x` would carry `#` and `x` into the node). The renderer writes no
inline comments, which is why the example has none.

**How a list is expressed.** The format is a strict depth-based tree: depth is the
number of leading `v` characters, and a node is `key value` (or just `key` for a
container). It has no dedicated list syntax, so **a list is a repeated node of the
same key under one parent, and its order is document order.** This is verified, not
assumed: libv parses `vvv app vfd` / `vvv app wiki` as two sibling `app` children of
`apps`, both preserved, and `find("device.apps.app")` returns them in order. The
documented encoding for each repeated collection is:

| Collection | Encoding | Read as |
|:--|:--|:--|
| apps | repeated `app` under `apps`, name in the value token | `find("device.apps.app")` |
| artifacts | repeated `artifact` under one `app` | that app node's `artifact` children |
| env | repeated `env KEY=VALUE` under one `app` | that app node's `env` children |
| secrets | repeated `secret NAME` under one `app` | that app node's `secret` children |

**A reader must iterate the app nodes, not flatten.** `VNode.find` walks one level
per path segment, so `find("device.apps.app.port")` crosses *every* app and returns
all of their ports together. Reading an app's fields by absolute path would therefore
mix apps up. The rule is: `find("device.apps.app")` once, then read each child by
relative key within that node.

**Determinism.** The document is byte-identical for the same manifest: apps are
written in manifest order, and within an app the children are written in a fixed
field order (the order above), not map order. This is what makes "the same file
compiles to byte-identical output" (US-001) testable.

**Compatibility note — the one libv change this needs.** libv's `VNode` already
carries `children`, so repeated nodes parse and are addressable with no format
change. What does **not** exist yet is a reader that returns a collection for a
repeated node: the Android side has `VNode.find` but no `children(key)` helper, and
Go's generated reader is built for the flat registration config. §13.3 (`AppConfigReader.kt`)
budgets for that reader; the format itself does not change, so there is no version bump
and no risk to the existing registration config.

Three fields are **injected by the server and may not be set by the file**:
`memberId` (who authorised it — see §7), `version`, and each `dir`. A file that sets
`memberId` is refused, because a config that names its own authoriser proves nothing.

`features.apps` is `true` in any config that carries apps. The field
already exists as `DeviceConfig.AppsEnabled` in Go and `DeviceConfig.appsEnabled` on
the device; it is simply `false` today because nothing sets it. This is the feature
that gives it meaning, so the only change is that it becomes `true`.

### 5.2 The no-secret rule

The compiler refuses a config that would carry a secret, and **names the offending
key**. The rule is enforced by the check that already ships
(`ValidateConfigHasNoSecrets` in Go, `ConfigVerifier.assertNoSecrets` on the device),
and this section exists because the naive form of that check refuses a field this
contract requires.

**The check is structural, not a substring scan of every line.** A scan for the
tokens `secret`, `token`, `key`, … across the whole document is exactly what the
shipped code does today, and it cannot survive an `apps` section: the schema's own
keys — the `apps` container, the `secret NAME` node that names a secret *without*
carrying one, a `secret_keys` list in the registry — contain those words by
definition. A rule that refuses its own field names is not a rule; it is a bug with a
security rationale attached. So the check changes from "does any line contain a
forbidden word" to the following, and this is the definition both sides implement:

1. **Schema keys are exempt by path, not by pattern.** The compiler walks the parsed
   document. The nodes it *writes* — `apps`, `app`, `artifact`, `env`, `secret`,
   `pwa` and the fixed settings keys — are known keys and are never themselves
   inspected for a forbidden word. The exemption is a closed set of paths the
   compiler owns; it is not "any key that happens to look safe".
2. **Every `env` value is inspected** for the shapes a secret takes: a PEM block, a
   JWT (`eyJ…`), a 40+ char hex/base64 run, or a URL carrying `user:pass@`. An `env`
   value that looks like a secret is refused and the key is named.
3. **Every `env` key is inspected** against
   `(?i)(password|passwd|secret|token|api[_-]?key|private[_-]?key|credential)`, and a
   match is refused. This is the case the rule exists for: `TOKEN = "…"` in `env`.
4. **Every other node's value is inspected** for the shapes in (2), so a secret
   smuggled into an unexpected field is still caught.
5. **The `secret NAME` nodes are exempt** — but only their *value*, and only because
   a name is a name. A `secret` node whose value is one of the shapes in (2) — a JWT,
   a hex run, a PEM block — is refused, because a secret *name* is not 40 hex
   characters. The device re-checks this: it will not accept a node that carries
   what looks like a value.

The consequence is that `[app.secrets]` may name `DATABASE_URL` or `S3_SECRET_KEY`
freely — those are names, which is the whole point of the field — while
`[app.env] S3_SECRET_KEY = "AKIA…"` is still refused, because that is a value.

**This is a change to the shipped check, on both sides, and it is the load-bearing
part of this contract.** The current scanner would refuse the compiled document in
§5.1 outright. §13 lists the change on both sides; §12's definition of done adds the
case "a config naming a secret is accepted, and a config carrying one is refused" —
both, because a rule that refuses too much is as wrong as one that refuses too
little, and only testing both distinguishes them.

The device re-checks the same rule before storing, and reports
`config_carries_secret` with the key. Both sides check because the invariant is
load-bearing: a config that can carry a token collapses the model back to "a secret
travels over the wire".

A caller that genuinely needs a secret has **somewhere else to put it** — §9b defines it —
and that is the point of the refusal being absolute here. The answer is not "put it in the
config and hope"; it is a separate channel with its own encryption and its own
authorisation.

### 5.3 The no-unhashed-reference rule

**Every artifact reference carries a `sha256`.** A reference without one is refused,
and the message names the artifact.

The reason is precise: the signature covers the *document*. A `{url}` with no digest
means the bytes at that URL are trusted because the URL was signed — which is only
true if the URL is immutable and the host is honest. A digest makes the transport
irrelevant, so an artifact can be served from anywhere.

### 5.4 Why references and not bytes

Config is policy — about a kilobyte, changes often, needs an atomic apply. An artifact
is content — tens to hundreds of megabytes, changes rarely, needs a resumable
transfer. Different sizes, different lifetimes, different failure modes, so different
channels. This is the same split Fly uses (the control plane receives a digest; the
registry receives the image) and the same rule the existing OTA path already follows.

---

## 6. The PWA rule

**A webroot is not a PWA unless it says so.**

| `pwa` | What happens |
|:--|:--|
| absent | **no** `site.webmanifest` is generated |
| `false` | **no** `site.webmanifest` is generated |
| `true` | a manifest is generated from defaults |
| table | a manifest is generated, with every declared field overriding the default |

The negative case is the important one. A manifest changes how a browser presents and
installs a page — an app that is not a PWA must not be handed one by accident, and the
default is therefore `false`.

Generated defaults, modelled on the console's own working manifest:

```json
{
  "name": "<app.name>",
  "short_name": "<app.name>",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#ffffff",
  "theme_color": "#111111",
  "icons": []
}
```

`display` accepts only `standalone | fullscreen | minimal-ui | browser`; anything else
is a hard error, because an invalid value silently degrades to `browser` and the
operator would never know why the app is not installable.

**How `pwa` is carried in the config.** In the manifest it is `bool or table`; in the
compiled `vyzorix.v` it is written only when the webroot is a PWA, as one node whose
value is `true` or `false`, with the overriding fields as its children (§5.1):

```text
vvvv pwa true
vvvvv theme_color #be123c
vvvvv display standalone
```

A webroot with `pwa` absent or `false` has **no `pwa` node at all** — the compiler
does not write `pwa false`. That is deliberate: the absence is the negative case, and
a reader that sees no node must generate nothing. Only fields that *override* a
default are written, so the compiled document does not restate `start_url = "/"` when
the operator did not set it; the generator's defaults (§6) fill the rest. The `enabled`
boolean an earlier draft showed is not part of the shape — the node's presence is the
enablement, and there is no `enabled` key to disagree with it.

**Icons are not generated.** An icon is an image, and the platform does not invent
one. If `[app.pwa].icons` is absent the manifest carries an empty list and the app is
installable with a default icon — the honest outcome, and it keeps the generator from
emitting a reference to a file that does not exist.

The generated manifest is written into the webroot **before the app starts**, so the
app serves it as a static file. Nothing about the app's own bytes changes; the same
artifact serves every device.

### 6.1 Why this is a file write and not a build

The console reaches the device's own engine with no build-time configuration, and the
evidence is in this repo rather than in a build artefact: the env schema defaults
`VITE_API_BASE_URL` to `http://localhost:3000` (`packages/config/src/env/index.ts`).
The device's engine listens on `127.0.0.1:3000`, so a console served *by the device*
and a console whose base URL is unset both resolve to the same place. Serving
same-origin additionally removes CORS entirely and makes `127.0.0.1` a secure context,
so the service worker and the install prompt work without TLS.

> An earlier draft cited `Vfd/web/src/stores/connection-store.ts` falling back to
> `location.origin`. That file is not in this repo — it was part of the untracked
> experiment (§11 of the requirements) — so the claim cannot be checked here. It does
> not matter which mechanism holds: the default above already lands on the engine, so
> the conclusion is the same and rests on a file that exists.

So: ship the console once, generate the manifest per device, serve it same-origin.

---

## 7. Authorisation

### 7.1 The predicate

An operator may deploy to device `D` in org `O` when:

```
  caller is authenticated                                        (session, Domain A)
    AND D ∈ O                                                    (binding)
    AND D.lifecycle == registered                                (not pending/deregistered)
    AND ( caller is a platform super_admin
          OR  ∃ grant g : g.subject == caller
                          ∧ g.action  ⊇ device.deploy
                          ∧ g.scope   ⊇ devices:D )
```

Both halves are required: an unauthenticated caller is not helped by a grant, and a
super-admin with no grant is still bound by `D ∈ O`.

**`super_admin` is platform-global, and the predicate says so.** An earlier draft
wrote `caller.role == super_admin IN O`, which the role model does not express:
`RoleSuperAdmin` is level 100 and "System-wide ownership" (`operator/role.go`), and
`risk_evaluator.go` documents it as bypassing org checks. There is no per-org
super-admin. So the first clause is "the caller is a super-admin, full stop", and the
org binding is what keeps it honest — a super-admin may deploy to any org's device,
which is what "system-wide ownership" means, and the `D ∈ O` conjunct is still
enforced rather than skipped. If a future release adds per-org admins, the clause
becomes a role check scoped to `O`; until then it is not, and pretending otherwise
would leave the predicate unimplementable.

**A device in another org returns `404`, not `403`.** A 403 would confirm the device
exists, which is an existence leak across tenants.

**How this lands in code.** The scoped engine already answers the grant half —
`ScopedPermissions.Grants(Action, scope)` with `MatchScope`'s trailing-`:*` wildcard,
so a grant on `devices:*` covers `devices:D` and one on `devices:D` covers only `D`.
The change is one new constant, `ActionDeviceDeploy = "device.deploy"`, in
`permission/scoped.go`, and a handler that calls `Grants` with scope `devices:<id>`.
The example grant in `provisioning.example.yaml` should move from `device.manage` on
`devices:*` to `device.deploy` on the specific device, so the file documents the
narrower grant this feature introduces.

### 7.2 Why `device.deploy` and not `apps.write`

Every existing action is a per-resource verb — `device.read`, `device.write`,
`device.assign`, `apps.read`, `apps.write` — and `apps.*` is about *querying what an
app is doing*, not about changing what runs. Folding deploy into `apps.write` would
mean the permission that lets an operator see an app's state also lets them install
one: a strictly larger grant than the name implies, on the highest-consequence action
in the system.

`device.deploy` is a new action. It is granted explicitly, or not at all.

### 7.3 What the device is told

The config carries `memberId` — who authorised it — and the device records it. The
console shows it (§10 of the requirements). A config that names its own authoriser is
refused (§5.1), so the field can only be set by the server, from the authenticated
session.

This is what makes "lower roles need permission from super admin" *verifiable after
the fact*: not a claim in a log the server could edit, but a field inside a document
the device signed a receipt for.

### 7.4 The deploy credential

A long-lived deploy credential may be minted scoped to a **single device**, and
revoked. It exists so CI can deploy without an operator session.

- scoped to one device — refused for any other, even in the same org;
- revocable, and a revoked credential is refused immediately;
- never logged, never returned by any `GET`.

This is the same reasoning as an app-scoped Fly token: a compromised credential should
reach one device's config, not the fleet.

---

## 8. The `cmd` rule

**`cmd` must resolve to a real file inside the app's own directory, and it is executed
without a shell.**

A manifest that can name any command is remote code execution with a signature. The
signature proves *who* asked; it says nothing about whether the thing asked for is
safe. So the rule is mechanical, and it is checked at compile time *and* at apply time:

1. The path must be **absolute** and start with `/home/apps/<app.name>/`.
2. It must contain **no shell metacharacters** (`; | & $ \` > < ( ) { } * ? ~ !` and
   newline).
3. After resolving symlinks, it must **still** be inside `/home/apps/<app.name>/`. A
   symlink pointing outside is refused — this is the check that makes rule 1 hold.
4. It must exist and be executable at apply time. The server cannot know this (the
   artifact has not been fetched yet), so the device checks it and reports
   `cmd_not_executable` rather than starting something that will fail.

The command is executed by `fork`/`exec` with the argv split on whitespace — no shell,
so a metacharacter that slipped past rule 2 has nothing to be interpreted by.

**`cmd` is checked twice on purpose.** The server refuses at compile time so the
operator gets a fast, clear error. The device re-checks at apply time because the
device is the thing that runs it, and a config could have been signed by a server that
was compromised, or by an older server that lacked the check.

### 8.1 The catalog

A `catalog_apps` section in the existing `provisioning` file names known apps. A
manifest naming an app in the catalog is ordinary; one naming anything else is
permitted but marked **custom** in the console.

The catalog is a convenience and a review aid, not a gate. It is seeded from the same
file that already seeds organizations and api keys, so it is reviewable in one place
and needs no new store.

---

## 9. Apply, and what happens when it fails

### 9.1 The apply sequence

1. **Verify** the signature against the pinned key. Refuse `unsigned`,
   `bad_signature`, `carries_secret`, `unhashed_reference`, `cmd_escape`.
2. **Fetch** each artifact to a temporary path, resuming a partial where one exists.
3. **Verify** each against its `sha256` — over the *whole* file, not just the new part
   of a resumed transfer.
4. **Install** atomically: unpack to `<dir>.new`, then `rename` over `<dir>`.
5. **Write** `apps.json` atomically (temp-then-rename), with `name`, `dir`, `cmd`,
   `port`, `env`, `autostart` per app.
6. **Generate** `site.webmanifest` for each `webroot` with `pwa` enabled.
7. **Signal** the supervisor (its `inotify` path picks the change up without a restart).
8. **Report** `device_ready {version}`.

### 9.2 Failure leaves the device working

A failed apply changes **nothing**:

- the current config and the previous known-good one are both retained (`keep_last = 2`);
- an artifact that fails its digest aborts *that artifact*, leaves the previous copy in
  place, and reports `digest_mismatch` naming the artifact;
- an interrupted install cannot leave a half-written app, because the install is a
  rename;
- a config that fails verification is refused before anything is fetched.

The device keeps running what it was running and reports the refusal with a distinct
reason. **The refusal is the product** — "your deploy did not apply, and here is
exactly why" is worth more than a silent retry.

Two is the right retention, not N: the recovery path that matters is "put back what
worked", one prior version answers it, and disk is the scarce resource on a phone.

### 9.3 What is removed

An app present in the old config and absent from the new one is **stopped and
deregistered**; its directory and data remain. Only `purge = true` in the manifest that
removes it deletes the data, and the deletion is therefore recorded in a document the
operator wrote and the server signed.

There is no `vyzorix undeploy`. A destructive verb with no reviewable artifact is how
data gets deleted by accident.

---

## 9b. Secrets — where an env var that *is* a secret goes

An app that is itself a server needs credentials: a database URL, an API token, a
signing key. `[app.env]` is exactly where a user would put them, and §5.2 refuses it.
That refusal is right — a config is signed, logged, served by `GET /v1/deploy/{device}`
and shown in a console, so a secret in it is a secret in five places — but a rule with
no alternative is not a design. This is the alternative.

### 9b.1 The rule

**`[app.env]` is for configuration. A secret travels on its own channel, encrypted to
the device, and is never in the signed config.**

The compiler's refusal (§5.2) is unchanged and stays. What is added is somewhere for
the secret to go.

### 9b.2 Where a secret is stored, and how it is delivered

Modelled on the one platform whose split this is — `fly.toml` carries no secrets,
`flyctl secrets set` writes to a separate encrypted store, and setting one *redeploys*
because the machine's environment changed. Vyzorix does the same, with one difference:
the device is intermittent, so the secret cannot be pushed at deploy time and assumed
received. It is delivered on demand, and re-delivered on every reconnect.

```
 vyzorix secrets set vfd DATABASE_URL=... --device d_01H...
   │
   ├─ server: encrypt to the DEVICE's public key      (RSA-OAEP-SHA1)
   │           store the CIPHERTEXT at rest            (secretstore, AES-GCM)
   │           record key, app, device — never the plaintext
   ▼
 device_config frame            the signed config: names the KEYS, not the values
   │
   ▼
 agent: sees a key it does not hold → asks → server returns CIPHERTEXT
        │
        ├─ decrypt with the device key                 (Keystore, PURPOSE_DECRYPT)
        ├─ write files/apps/<app>/.env, mode 0600
        └─ report secrets_ready {app, keys[]}
```

The config carries the **names of the secrets** each app expects — one `secret NAME`
node per name (§5.1), e.g. `vvvv secret DATABASE_URL` — and never a value. That is
what lets the device know it is missing one, and what lets the console show *which*
secrets an app expects without showing any of them. The node's value is a name, which
is why §5.2 exempts it from the forbidden-word scan while still refusing a node whose
value *looks* like a secret.

### 9b.3 The envelope, and why this key and not the other

**A secret is encrypted to a key the agent holds for exactly this purpose** — an
RSA-3072 keypair in the Android Keystore with `PURPOSE_DECRYPT`, generated on first
run, non-exportable, and *separate from the identity key*.

Three reasons the envelope is RSA-OAEP rather than ECDH:

1. **`PURPOSE_AGREE_KEY` is API 31**, and this app has `minSdk = 26`. ECDH in the
   Keystore is unavailable on Android 8–11, which the app supports. `PURPOSE_DECRYPT`
   is API 23, so RSA works on every device in range.
2. **The identity key cannot be reused.** It is generated with `PURPOSE_SIGN` only
   (`KeystoreDeviceIdentity`), and widening its purpose would change what that key
   *is* — a signing key that can also decrypt is a worse key. More importantly, the
   identity key's public half is deliberately public: `IdentityApi` exposes it to the
   container, and it is sent to the server at enroll. Encrypting a secret to a key
   whose public half is handed out freely is fine cryptographically, but it means the
   secret's confidentiality rests on a key the whole system already knows.
3. **The container must not be able to decrypt.** `IdentityApi` says of the identity
   key: *"this API has no way to reach it."* The secret key needs the same property
   for the same reason — proot runs with `-0`, so the container is **root inside**,
   and anything the container can read, a compromised app can read. The decryption
   happens in the agent process, and only the *plaintext* env file reaches the
   container.

So: a **second Keystore entry**, `vyzorix.secret`, `PURPOSE_DECRYPT`, generated on
first run alongside the identity key, whose public half is sent at enroll exactly as
the identity key's is. The agent never exposes it over the bridge; there is no
`SecretApi`.

**The OAEP digest is SHA-1, and that is a constraint the Keystore imposes.** The
probe run for this spike established it on API 28:

```
OAEP-SHA256/MGF1-SHA256  REJECTED  InvalidAlgorithmParameterException:
                                   Unsupported MGF1 digest: SHA-256. Only SHA-1 supported
OAEP-SHA256/MGF1-SHA1    SUPPORTED (no stdlib producer — see below)
OAEP-SHA1/MGF1-SHA1      SUPPORTED
```

Two consequences, and neither is a preference:

1. **The Keystore accepts only MGF1-SHA1.** A spec with `MGF1ParameterSpec.SHA256`
   is refused outright, so "SHA-256 OAEP" is not an option on a non-exportable
   Keystore key.
2. **Go cannot produce the mixed pair.** `rsa.EncryptOAEP` uses the supplied hash
   for both the OAEP digest and MGF1, so the only pair both Go and the Keystore can
   agree on is SHA-1 for both — which is also OAEP's default and what
   `EncryptOAEP(sha1.New())` emits.

So the envelope is `RSA-OAEP-SHA1`, and the constant the two sides compare is
`"RSA-OAEP-SHA1"`. This was **verified on the emulator, not inferred**: Go wrapped a
secret to the device's Keystore public key and the Keystore returned it —
`DECRYPTED=postgres://vyzorix:s3cr3t@db.internal:5432/app`. SHA-1 in OAEP is the
standard default and is not a practical weakness here (the 3072-bit modulus is the
protection), but it is recorded because the obvious "harden it to SHA-256" edit
breaks every device, and a comment is cheaper than that outage.

### 9b.4 What lands on disk, and why not `apps.json`

The plaintext goes to `files/apps/<app>/.env`, mode `0600`, and **not** into
`apps.json`. The registry is inside `/home/apps`, which is bound into the container and
readable by every app running there — putting a secret in it would hand one app
another's credential. The supervisor already passes env to a process (Layer 3 §4.1 S8,
`app-env`); it reads the `.env` at start and does not copy it into the registry.

`apps.json` holds `env` (configuration, from the config) and a list of `secret_keys`
(names only). That distinction is the whole point: the registry is inspectable, so it
must not be worth inspecting.

### 9b.5 Rotation, and the failure that matters

- **A secret is replaced, not merged.** `secrets set` for an existing key overwrites the
  ciphertext and bumps the app's secret generation. The device re-fetches when the
  generation differs from the one it holds.
- **A device that has been wiped** has lost its Keystore key, so it cannot decrypt
  anything sent before the wipe. It re-enrolls (a new identity, per
  `REGISTRATION_CONTRACT.md` §5) and the operator re-sets the secret. There is no
  recovery of a secret encrypted to a key that no longer exists — by design, and worth
  stating rather than discovering.
- **A secret the device cannot decrypt is a refusal, not a silent skip.** If the
  ciphertext will not open, the agent reports `secret_undecryptable {app, key}` and
  **does not start the app**. Starting it with the variable absent is the failure mode
  that produces an app that looks up but talks to the wrong thing.

### 9b.6 The CLI and the API

```
vyzorix secrets set   <app> KEY=VALUE --device <id>     # encrypt, store, bump generation
vyzorix secrets list  <app> --device <id>               # names + generation, never values
vyzorix secrets unset <app> KEY    --device <id>
```

- `GET /v1/deploy/{device}/secrets/{app}` returns **names and generation only**.
- `GET /v1/deploy/{device}/secrets/{app}/{key}` returns **ciphertext** to an
  authenticated device, and only for a key the current signed config names. A device
  cannot ask for a secret its config does not reference.
- `set`/`unset` are authorised by `device.deploy` (§7.1) — the same grant, because
  writing a credential is the same kind of act as writing what runs.
- **A value is never logged, never returned by any `GET`, and never written into any
  state the deploy feature reads back.** `list` shows names, which is what an
  operator needs to confirm the right thing is set.

### 9b.7 What this does and does not protect

**Protects:** a secret never sits in a git repo, never travels in the signed config,
never appears in a log or a console response, is encrypted at rest on the server, and
is encrypted in transit to a key only the agent holds — so a server compromise that
reaches the *ciphertext store* does not yield a plaintext secret for a device whose key
it does not have.

**Does not protect:** the device itself. A rooted phone, or the operator's own
container running as fake-root, can read `files/apps/<app>/.env` once the app has
started. This is the same limit as every secret on a device the operator controls, and
it is stated rather than implied. The envelope defends the *transport and the store*,
not a compromised endpoint.


---

## 10. Honest limits

- **Offline first-deploy is not supported.** A device that has never been online has no
  pinned config and no artifacts. Enrollment already costs one round trip
  (`REGISTRATION_CONTRACT.md` §3 Phase A); every deploy after that is verifiable
  offline. This is a product limit, not an oversight, and a user will hit it.
- **`cmd` is still a code-execution surface.** §8 constrains it to something the
  manifest also shipped, which is a real reduction — but a signed manifest still runs
  code on someone's phone. The remaining control is *who may sign* (super_admin or an
  explicit grant), and that is a policy, not a sandbox.
- **No automatic rollback.** The last good config is retained and restorable; restoring
  it automatically when an app crash-loops is a later feature. A device can therefore
  be left running a config whose app immediately dies, and it will report the crash
  loop rather than heal itself.
- **The signature decision is inherited, not made here.** If the registration contract
  keeps HMAC (§9 there) instead of the keypair, then the config is signed with a shared
  secret and delivered envelope-encrypted to the device's public key. Everything else
  in this contract is unchanged; §2's rule 1 becomes "signed with a shared secret the
  device holds", which is weaker — the secret must exist on both sides — and that is
  the trade that document weighs.
- **`expose = "cloud"` is refused in v1.** It needs the wildcard-DNS/TLS proxy that
  Layer 3 decision D7 defers. `loopback` and `lan` are supported; `lan` requires a
  token (Layer 3 §4.4 P2 — `0.0.0.0` without a token fails closed).
- **No group or fleet deploy.** One device per deploy. Org-level defaults are read but
  not applied fleet-wide; a second device is a second deploy.
- **Version history is two, so "roll back three deploys" is not possible.** Stated
  because it is a reasonable thing to want and it is not here.
- **Key rotation needs an overlap window.** A device offline for months may only know an
  older pinned key; the server must accept a signature from any key in the set until
  the device has re-fetched.
- **The secret envelope defends the transport and the store, not the device.** Once an
  app has started, its secret is a `0600` file the app itself can read — and proot runs
  with `-0`, so the container is root inside. A rooted phone, or a compromised app, can
  read it. This is the limit of any secret on an endpoint the operator controls (§9b.7).
- **A wiped device loses its secret key, so old ciphertext is unrecoverable.** The
  Keystore entry does not survive a reinstall; the device re-enrolls as a new identity
  and the operator re-sets the secret. Stated because it is a real operational step, not
  an edge case.
- **Secrets are per device, not shared.** A secret set for device A does not apply to
  device B, because it is encrypted to A's key. A fleet of ten devices means ten
  `secrets set` calls, or a loop — there is no group secret in v1.

---

## 11. State machine

Per **app**, not per device — one device runs several, and they fail independently.
The supervisor already models this; the states below are what the registry holds.

```
 (absent)
     │  appears in a signed config
     ▼
  STOPPED ─────────── autostart ─────────► STARTING ──► RUNNING
     │                                        │            │
     │                                        │ crash      │ exit 0 (one-shot)
     │                                        ▼            ▼
     │                                     BACKOFF      STOPPED
     │                                        │
     │                                        │ failures > N
     │                                        ▼
     │                                     FAILED ──────► (reported; no auto-heal)
     │
     │  absent from the next signed config
     ▼
 (stopped, deregistered; data retained unless purge)
```

`BACKOFF` is the supervisor's existing exponential restart (Layer 3 §4.1 S3), capped.
`FAILED` is reported to the server so the console can show it; nothing restarts it
automatically (§10).

---

## 12. Definition of done

1. A `vyzorix.toml` with two apps compiles, and both run on one device, with no change
   under `apps/api/`.
2. A config is signed and the device verifies it; an unsigned config is refused.
3. A config carrying a secret is refused, **and a config naming one is accepted** — by
   a test on both sides. The second half is the one the shipped scanner fails today:
   it refuses the word `secret` anywhere, which refuses the schema's own `secret NAME`
   node. Both cases are asserted, because a rule that refuses too much is as wrong as
   one that refuses too little.
4. An artifact with a wrong `sha256` is refused, and the previous copy is untouched.
5. A `cmd` outside the app's own directory is refused, including via a symlink.
6. A lower role without a grant is refused; `super_admin` is not; another org's device
   is `404`.
7. A device that was offline re-fetches on reconnect and applies the newer version.
8. `pwa = false` generates **no** manifest; `pwa = true` generates a valid one; a
   `[app.pwa]` override changes the served file.
9. A failed apply leaves the device running its previous config, demonstrated by
   inducing an artifact failure and observing the device still answer.
10. A secret set with `vyzorix secrets set` reaches the app as an env var, and **never**
    appears in the signed config, in `apps.json`, in a log, or in any `GET` response —
    asserted by a test that greps all four.
11. A secret the device cannot decrypt is refused, and the app is **not started** with
    the variable absent.
12. `./gradlew gate` passes; the server gate passes; the flow is exercised end to end
    against the real server on the emulator.

---

## 13. Inventory — every file, and which requirement puts it there

Counts are estimates, sized so the work is plannable. The repo's rule holds: a new file
must earn its name, so existing files are modified in place wherever possible.

**How to read this.** *New* files are ones that do not exist today. *Touch* files exist
and change. Every entry names the requirement it serves, so a file with no requirement
is visibly unnecessary and a requirement with no file is visibly unbuilt. §13.5 is the
reverse map — requirement → files — which is the check that nothing is missing.

Two facts the first draft of this section got wrong, because it never opened the code:

- **Layer 3 is built, not greenfield.** `app-runtime/supervisor/` (C++), the `app-*`
  CLI (`app-runtime/cli-wrappers/`), `native/lib/include/vyzorix/app_registry.h`,
  `app_meta_store.h` and `android-core/.../Layer3Installer.kt` all exist. Deploy
  *writes into* that runtime; it does not build it. §13.6 lists what is left.
- **`secretstore` exists.** `apps/api/internal/infrastructure/security/secretstore/`
  is the encrypted-at-rest store the secret ciphertext goes into. Deploy reuses it;
  it is not a new file.

### 13.1 Server — new files (`vyzorix-update-server`)

| New file | ~LOC | Requirement | Purpose |
|:--|--:|:--|:--|
| `tooling/schema/deploy.cue` | 120 | FR-22, FR-2 | The manifest schema. Source of truth; Go generated from it, JSON Schema published. |
| `apps/api/internal/deploy/vyzorix_deploy_manifest.go` | 160 | US-001, FR-1 | Parse and validate `vyzorix.toml` against the generated types. Unknown key ⇒ error. |
| `apps/api/internal/deploy/vyzorix_deploy_compile.go` | 190 | US-001, FR-3, FR-4 | Manifest → config. Injects `memberId`/`version`/`dir`; enforces the `cmd` rule (§8). |
| `apps/api/internal/deploy/vyzorix_deploy_render.go` | 120 | US-001, §5.1 | The `vyzorix.v` writer: repeated `app`/`artifact`/`env`/`secret` nodes in fixed field order. The determinism surface, so it is tested on its own. |
| `apps/api/internal/deploy/vyzorix_deploy_secret_scan.go` | 90 | FR-8, §5.2 | The structural no-secret walk: exempt schema paths, inspect every other value, inspect `env` keys. |
| `apps/api/internal/deploy/vyzorix_deploy_signer.go` | 90 | US-002, FR-4, FR-5 | Sign the compiled config with `ServerSigner.SignDocument`; increment the version. |
| `apps/api/internal/deploy/vyzorix_deploy_store.go` | 130 | FR-21, §13 note | Persist current + last-good per device (`keep_last = 2`). **Not** the `configversion` store. |
| `apps/api/internal/deploy/vyzorix_deploy_authentication.go` | 110 | US-003, FR-11, §7.1 | The predicate: super-admin, or a `device.deploy` grant whose scope covers the device. |
| `apps/api/internal/deploy/vyzorix_deploy_secrets.go` | 200 | US-013, FR-24–27, §9b | `crypto.WrapSecret` to the device key; store ciphertext via `secretstore`; generation counter; the "only keys the config names" check. |
| `apps/api/internal/deploy/vyzorix_deploy_catalog.go` | 80 | FR-10, FR-19 | The `catalog_apps` model and the "custom vs known" marking. |
| `apps/api/internal/api/handlers/deploy/vyzorix_deploy_config_handler.go` | 170 | US-003, US-005, US-009 | `POST /v1/deploy`, `GET /v1/deploy/{device}`, `GET /v1/deploy/schema/{version}`, dry-run. |
| `apps/api/internal/api/handlers/deploy/vyzorix_deploy_credential_handler.go` | 110 | US-004, FR-12 | Mint / list / revoke a device-scoped deploy credential. |
| `apps/api/internal/api/handlers/deploy/vyzorix_app_secret_handler.go` | 130 | US-013, §9b.6 | `GET` names+generation; `GET` ciphertext to an authenticated device. |
| `apps/api/internal/infrastructure/storage/078_device_deploy_config.go` | 70 | FR-21 | The per-device signed-config table (current + last-good). |
| `apps/api/internal/infrastructure/storage/079_device_deploy_credential.go` | 70 | FR-12 | The deploy-credential table. |
| `apps/api/internal/infrastructure/storage/080_device_app_secret.go` | 70 | US-013 | The secret ciphertext table: `(device, app, key, generation, ciphertext)`. |
| `apps/cli/vyzorix_cli_main.go` | 120 | US-009, FR-23 | The `vyzorix` entry point. |
| `apps/cli/internal/deploy/vyzorix_deploy_command.go` | 180 | US-009 | `deploy`, `--dry-run`, validate, progress, exit codes. |
| `apps/cli/internal/deploy/vyzorix_secrets_command.go` | 100 | US-013, §9b.6 | `secrets set/list/unset`. |
| `apps/cli/internal/deploy/vyzorix_manifest_validate.go` | 90 | FR-2, US-009 | Validate against the published JSON Schema before upload. |

**New package: one** (`internal/deploy`). The handlers, storage and CLI are new files
in existing packages or new leaf packages.

**`device.deploy` is a base action and is in no action set.** `action_sets.go` expands
`device.manage` into `device.read/write/delete/assign`; deploy must **not** be added to
it, or the aggregate the contract exists to avoid (observe ⇒ install) reappears one
level up. It is granted explicitly, per §7.2.

### 13.2 Server — existing files to touch

| Existing file | Change | Requirement |
|:--|:--|:--|
| `apps/api/internal/domain/permission/scoped.go` | Add `ActionDeviceDeploy = "device.deploy"`. | FR-18, §7.2 |
| `apps/api/internal/api/server_routes.go` | Register the `/v1/deploy` group and its routes. | US-003, US-013 |
| `apps/api/internal/api/api_server.go` | Construct the deploy handler and service; wire the signer, store, secretstore and catalog. | US-003 |
| `apps/api/internal/api/server_handlers.go` | The handler struct fields the routes need. | US-003 |
| `apps/api/internal/application/device/config_service.go` | Replace `ValidateConfigHasNoSecrets`'s line scan with the structural walk (`vyzorix_deploy_secret_scan.go`) for configs that carry apps; `BuildConfig` emits the `apps` section and `features.apps = true`. | FR-8, §5.1–5.2 |
| `apps/api/internal/api/handlers/inbox/enroll_handler.go` | Deliver the apps-carrying config; carry the device's **secret** public key into the device record at enroll. | US-005, §9b.3 |
| `apps/api/cmd/api/provisioning_adapters.go` | The `catalog_apps` section adapter. | FR-19, §8.1 |
| `apps/api/internal/application/provisioning/vyzorix_boot_provisioning.go` | Add `CatalogApps []CatalogAppConfig` to `Config` and apply it at boot. | FR-19 |
| `provisioning.example.yaml` | Document the `catalog_apps` section; move the example grant to `device.deploy` on a specific device. | FR-19, §7.1 |
| `apps/api/openapi/schemas.go` | **Generated** from `tooling/schema/deploy.cue` by `tooling/codegen/build_schemas.py`; never hand-edited. | FR-22 |
| `apps/api/swag/*` (`docs.go`, `swagger.json`, `openapi3.json`, `swagger.yaml`) | **Generated** by `swag init` after the handlers land; the swagger drift gate compares them. | FR-22 |
| `e2e/selectors/vyzorix-selectors.cjs` | **Generated** by `tooling/codegen/build_selectors.py`; new deploy endpoints add selectors. | US-010 |
| `packages/API_Client/src/generated/**` | **Generated** by orval (`pnpm codegen:api`, `codegen:hooks`); the CLI and console consume the deploy types rather than re-declaring them. | FR-23 |
| `apps/VyzoriX_web/src/ui/pages/` (new `deploy/` page), `src/routes/` | The read-only Deploy view. | US-010 |

The four generated rows are why the CUE and swagger drift gates exist: the source is
edited, the artefacts are regenerated, and a stale artefact fails the gate. Nothing in
those rows is hand-written.

### 13.3 Device — new files (`Vyzorix-android`)

| New file | ~LOC | Requirement | Purpose |
|:--|--:|:--|:--|
| `cloud-connector/src/main/kotlin/com/vyzorix/connector/cloud/DeployApplier.kt` | 240 | US-006, US-007, §9.1 | Verify, fetch, verify, install atomically, write the registry. |
| `.../cloud/ArtifactFetcher.kt` | 180 | US-006, FR-13 | Resumable fetch + whole-file digest check. Generalises the OTA fetcher. |
| `.../cloud/PwaManifest.kt` | 120 | US-008, FR-14 | Generate `site.webmanifest`; refuse an invalid `display`. |
| `.../cloud/AppConfigReader.kt` | 130 | §5.1 | Read the repeated `app`/`artifact`/`env`/`secret` nodes out of the verified `VDocument`, iterating app nodes rather than flattening. |
| `.../cloud/DeployReporter.kt` | 90 | US-005, FR-16 | Report `device_ready {version}` and every refusal reason. |
| `.../cloud/AppRegistryWriter.kt` | 110 | US-007, §13 note | Write `apps.json` atomically in the supervisor's shape. **Not** a copy of the config — see the note below. |
| `.../secret/SecretKey.kt` | 110 | FR-25, §9b.3 | The second Keystore entry: `vyzorix.secret`, RSA-3072, `PURPOSE_DECRYPT`, generated on first run. Never exposed over the bridge. |
| `.../secret/SecretStore.kt` | 140 | US-013, FR-26 | Fetch ciphertext, decrypt, write `files/apps/<app>/.env` at 0600; refuse to start an app whose secret will not open. |

`.../secret/SecretEnvelope.kt` (90) is already written and proven in the spike, so it is
counted here as done rather than new work.

**`apps.json` is not the config.** The registry the supervisor reads
(`native/lib/include/vyzorix/app_registry.h`) holds `{name, dir, cmd, port, env,
autostart, pid, pidStartTicks}` and **nothing else** — no `expose`, no `secret_keys`.
`expose` and its token live in the supervisor's separate meta store
(`app_meta_store.h`), and the secret is a `0600` `.env` the supervisor reads at start
(§9b.4). So `AppRegistryWriter` writes the registry shape and does *not* put secrets or
exposure metadata into it; the earlier draft's "write apps.json with expose and
secret_keys" would have produced a file the supervisor silently ignores those fields in.

### 13.4 Device — existing files to touch

| Existing file | Change | Requirement |
|:--|:--|:--|
| `cloud-connector/src/main/kotlin/com/vyzorix/connector/config/ConfigVerifier.kt` | Replace the line scanner in `assertNoSecrets` with the structural walk of §5.2; read the apps section. | FR-8, §5.1 |
| `.../config/ConfigStore.kt` | Retain current + last-good; restore on failure. | FR-21 |
| `.../CloudConnectionFactory.kt` | Route a delivered `device_config` to `DeployApplier`; construct the deploy components. | US-005 |
| `.../command/AppControlClient.kt` | Unchanged as a client; the deploy path signals the supervisor through the same socket (its `inotify` re-read). | US-007 |
| `cloud-connector/build.gradle.kts` | No new dependency expected; the connector already has libv, OkHttp and serialization. Listed so a reviewer checks rather than assumes. | — |
| `app/build.gradle.kts` | Bundle the `secret-envelope` test vector into `androidTest` assets; any new permissions. | US-013 |
| `android-core/src/main/kotlin/com/vyzorix/engine/ProotLauncher.kt` | No change needed — `/home/apps` is already bound and the supervisor is already launched. Listed to say so explicitly. | §13.6 |

### 13.5 Requirement → files (the completeness check)

| Requirement | Server files | Device files |
|:--|:--|:--|
| US-001 manifest + compiler | `deploy.cue`, `vyzorix_deploy_manifest.go`, `vyzorix_deploy_compile.go`, `vyzorix_deploy_render.go` | — |
| US-002 signing | `vyzorix_deploy_signer.go` (`ServerSigner`) | `ConfigVerifier` (verify) |
| US-003 `POST /v1/deploy` | `vyzorix_deploy_config_handler.go`, `vyzorix_deploy_authentication.go`, `server_routes.go` | — |
| US-004 deploy credential | `vyzorix_deploy_credential_handler.go`, `079_device_deploy_credential.go` | — |
| US-005 delivery + drift | `vyzorix_deploy_config_handler.go`, `enroll_handler.go` | `ConfigStore`, `DeployReporter`, `CloudConnectionFactory` |
| US-006 fetch/verify/install | — | `ArtifactFetcher`, `DeployApplier` |
| US-007 `/home/apps` + registry | — | `AppRegistryWriter`, `DeployApplier` |
| US-008 webroot + PWA | `vyzorix_deploy_compile.go` | `PwaManifest` |
| US-009 CLI | `apps/cli/**` | — |
| US-010 console view | `apps/VyzoriX_web/src/ui/pages/deploy/` | — |
| US-011 Layer 3 gaps | — | see §13.6 |
| US-012 second app | (no platform file — that is the test) | (no platform file) |
| US-013 secrets | `vyzorix_deploy_secrets.go`, `vyzorix_app_secret_handler.go`, `080_device_app_secret.go` | `SecretKey`, `SecretEnvelope`, `SecretStore` |
| FR-2 published schema | `deploy.cue` → `schemas.go`, `build_schemas.py`; `vyzorix_deploy_config_handler.go` serves it | `vyzorix_manifest_validate.go` (CLI) |
| FR-8 no-secret | `vyzorix_deploy_secret_scan.go` | `ConfigVerifier` |
| FR-9 `cmd` rule | `vyzorix_deploy_compile.go` | `DeployApplier` (re-checks) |
| FR-16 refusals | `vyzorix_deploy_config_handler.go` | `DeployReporter` |
| FR-17/FR-24 never logged | every deploy file (a test greps) | `SecretStore` (a test greps) |
| FR-18 `device.deploy` | `scoped.go`, `vyzorix_deploy_authentication.go` | — |
| FR-19 catalog | `vyzorix_deploy_catalog.go`, `provisioning_adapters.go`, `vyzorix_boot_provisioning.go` | — |
| FR-20 `purge` | `vyzorix_deploy_compile.go` | `DeployApplier` |
| FR-21 `keep_last = 2` | `vyzorix_deploy_store.go`, `078_device_deploy_config.go` | `ConfigStore` |
| FR-22 CUE + drift | `deploy.cue`, `schemas.go`, `swag/*`, `selectors.cjs` | — |
| FR-23 CLI shares types | `apps/cli/**`, orval output | — |
| FR-25–27 secret envelope | `vyzorix_deploy_secrets.go` | `SecretKey`, `SecretEnvelope`, `SecretStore` |

US-012 has no platform file on purpose: "a second app deploys with no platform change"
is a property, and the moment a file is added to make it work, the requirement has
failed. Its exercise is a test (`13.7`), not an artefact.

### 13.6 Layer 3 — what is already built, and the one gap left

Layer 3 is **not** greenfield, and the contract must not re-budget it. Already in the
tree:

- **The supervisor** — `app-runtime/supervisor/` (C++): registry read, autostart,
  crash backoff, PID start-ticks validation, log rotation, control socket, `inotify`
  re-read of the registry.
- **The `app-*` CLI** — `app-runtime/cli-wrappers/app_tool.cpp`, with ops `start`,
  `stop`, `restart`, `list`, `logs`, `env`, `create`, `remove`, `ping`, `expose`,
  `unexpose`.
- **The registry and meta stores** — `native/lib/include/vyzorix/app_registry.h`,
  `app_meta_store.h` (exposure + token, mode 0600).
- **The installer and launch** — `android-core/.../Layer3Installer.kt` installs the
  supervisor and CLI; `ProotLauncher.supervisorArgs` starts it with the registry path
  and control socket; `binds()` already maps `files/apps:/home/apps`.
- **The command path** — `CloudConnectionFactory` registers `APP_START`, `APP_STOP`
  and `APP_RESTART` via `AppCommands`/`AppControlClient`.

The **one** gap deploy still carries: nothing verifies the supervisor is *running*
before an apply writes the registry. If it is absent the config still lands and the
apps simply do not start, which is a silent failure. Deploy's `DeployApplier` should
probe the control socket (`ping`) and report a distinct reason when it is unreachable —
a small addition, and the only Layer 3 work this contract owns.

### 13.7 Tests

**Server: ~8 files, ~60 cases.** Load-bearing: the refusals — secret, unhashed ref,
`cmd` escape, cross-org (404), no-grant (403) — plus the structural no-secret pair
(names accepted, values refused) and the two-app compile. The envelope tests
(`secret_envelope_test.go`, `secret_envelope_vector_test.go`) already exist and pass.

**Device: ~8 files, ~50 cases.** Load-bearing: a tampered digest leaves the previous
copy; `pwa = false` writes no file; a `cmd` symlink escape is refused; a failed apply
keeps the running config; a ciphertext will not decrypt with the wrong key; a secret
never reaches `apps.json` or a log; the compiled §5.1 document parses and verifies; and
the two-app end-to-end from US-012. `SecretEnvelopeInteropTest` and
`SecretEnvelopeKeystoreTest` already exist and pass.

### 13.8 Rough size

Server ~1,900 LOC + CLI 400 + tests; device ~1,100 LOC + tests. Comparable to the
registration contract's own estimate, and the same shape: most of the value is in the
refusals and the crypto, which are cheap to write and expensive to omit.

---

## 14. The spike, and why it is first

**Build the compiler, the signer, the verifier — and the secret envelope — and nothing
else.**

It is first because it is the load-bearing invariant. Every other part of this contract
can be wrong in a way that is visible and fixable; a config that verifies when it
should not is the failure that makes the rest pointless. A round-trip test — compile a
manifest, sign it, verify it, then tamper with one byte and verify again — exercises
the whole trust model in a few hundred lines, with no device, no network and no
artifacts.

The envelope is in the spike for the same reason and at the same cost: RSA-OAEP wrap
in Go, unwrap in Kotlin, is a handful of lines, and it is the one piece of new
cryptography this feature introduces. Getting it wrong is silent — a secret that
decrypts to garbage is worse than one that refuses — so it is worth proving before
anything is built on top of it. **It has been run, and it changed the design**: the
Android Keystore accepts only MGF1-SHA1, so the envelope is `RSA-OAEP-SHA1` rather
than the SHA-256 the first draft specified (§9b.3). That is exactly the kind of thing
this spike is for.

Specifically, the spike is done when:

1. A manifest compiles to a deterministic config.
2. The config is signed, and the signature verifies against the pinned key.
3. Flipping one byte anywhere in the config fails verification.
4. A config carrying a secret is refused at compile time **and** at verify time, **and
   a config naming a secret is accepted** — the second half is what the shipped scanner
   fails, and it is the half that decides whether §5.2's rewrite is correct.
5. A reference without `sha256` is refused.
6. A `cmd` outside `/home/apps/<app>/` is refused, including through a symlink.
7. **A two-app document parses with libv and reads back as two apps with their own
   fields** — the round trip that proves the §5.1 encoding, and the one an earlier
   draft of this contract could not pass because it had no encoding at all.
8. The same tests run on both sides — Go signs, Kotlin verifies — because a signature
   that only verifies in one language is not a signature.
9. **A secret wrapped in Go unwraps in Kotlin**, and a ciphertext wrapped to key A
   fails to unwrap with key B. **And unwraps with a real Android Keystore key** — the
   host JVM is not the Keystore, and only the device answers whether the parameters
   are accepted. This item is why the digest is SHA-1.

Only then is it worth fetching an artifact. The fetch is the part everyone wants to
build first and the part that teaches the least.

Two of these were added after a review found the first draft could not pass them
(items 4 and 7). They are in the spike rather than the test suite because both are
format decisions, and a format decision discovered late is a migration.
