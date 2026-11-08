# PRD: Vyzorix Deploy — declarative, authorised deployment of applications to devices

**Status:** requirements, for review. Not a contract.
**Scope decision:** this PRD covers the **deploy and authorisation layer**. The Layer 3
runtime it sits on (supervisor, `apps.json`, control socket, app storage) is specified
in `vyzorix-android/doc/LAYER_3_REQUIREMENTS.md` and is a **dependency**, not this work —
except where that document records a gap this feature cannot start without (§7).
**Repos:** `VinnsEdesigner/vyzorix` (server, this repo) and `VinnsEdesigner/Vyzorix-android`
(device agent).

---

## 1. Introduction

An operator writes a `vyzorix.toml` that declares what should run on a device, and
runs one command. The server validates it, binds it to a device and an authorising
member, **signs** it, and delivers it. The device verifies the signature against a
key it was built with, fetches the artifacts the manifest names, checks each against
its digest, and starts the apps.

The shape is deliberately the one a developer already knows — `fly.toml` and
`flyctl deploy` — with one difference that is not cosmetic: **a device is
intermittent.** Fly's control plane is online and authoritative, so every deploy is a
live API call and the server is the gate. A phone may be offline for days and must
still decide for itself whether a config is legitimate. So the authority is a
**signature the device can verify alone**, not a check the server performs at
delivery time. Everything below follows from that.

**It is not about one application.** The platform deploys *whatever the manifest
names*. The first tenant is the media downloader (Vfd: a C engine plus a web
console), and the second — included deliberately, see US-012 — is a trivial static
site. A platform with a single tenant is indistinguishable from a hardcoded feature.

---

## 2. Goals

- An operator can go from a `vyzorix.toml` in a repository to apps running on a
  device with one command.
- Deployment is **authorised**: only a `super_admin`, or a member holding an explicit
  grant, may deploy — and the device can be told *who* authorised a config.
- A config is **verifiable offline**: a device with no network decides for itself
  whether to apply it.
- A config **never carries a secret**, and every artifact it names **carries a
  digest**. Both are enforced on the server and on the device.
- The platform is **app-agnostic**: adding a new app is a manifest change, not a code
  change.
- A failed deployment **never leaves a device worse off**: the previous known-good
  set is retained and recoverable without a network.
- A web app can be **installed as a PWA**, and whether it is one — and how it is
  presented — is declared in the manifest.

---

## 3. The model, in one page

```
vyzorix.toml                      the operator's declaration (source of truth)
   │  vyzorix deploy --org ORG --device DEV
   ▼
server                            authenticate → authorise → bind → compile → SIGN
   │  config (signed, versioned, no secrets, every artifact hashed)
   ▼
device agent                      verify signature → fetch → verify digest → apply
   ├─ writes apps into /home/apps and the apps.json registry
   ├─ for a webroot artifact: generates site.webmanifest when pwa = true
   └─ reports device_ready {version}  → server records what actually landed
```

**The three things that make it trustworthy:**

1. **The config is signed by the server** and verified by the device against a pinned
   public key. The device never acts on an unsigned instruction, and never on a live
   socket command.
2. **The config carries references, not bytes.** Each artifact is `{url, sha256}`.
   The signature covers the digest, so the *transport* can be untrusted — an artifact
   may be fetched over plain HTTPS and still be known-good.
3. **A config carries no secret.** If it could, the delivery channel would inherit
   the problem the keypair model exists to remove. The server refuses to issue one and
   the device refuses to store one.

---

## 4. User Stories

### US-001: The `vyzorix.toml` schema and compiler
**Description:** As an operator, I want to declare what runs on a device in a file, so that the deployment is reviewable and version-controlled.

**Acceptance Criteria:**
- [ ] A `vyzorix.toml` with `[device]` (`id`, `org`, optional `name`) and one or more `[[app]]` blocks parses without error.
- [ ] Each `[[app]]` requires `name`; `cmd`, `port`, `expose`, `autostart`, `env`, `artifact[]` and `pwa` are optional with documented defaults.
- [ ] An unknown key is a **hard error**, not a warning — a typo'd setting must not silently do nothing.
- [ ] A `[[app.artifact]]` requires `name`, `url` and `sha256`; a missing `sha256` is refused with a message naming the artifact.
- [ ] The compiler output is deterministic: the same file compiles to byte-identical JSON.
- [ ] The compiler refuses a config that would carry a secret (see FR-8) and names the offending key.

### US-002: The server signs the compiled config
**Description:** As the platform, I want to sign the config so a device can verify it offline.

**Acceptance Criteria:**
- [ ] The compiled config is signed with the server's signing key — ECDSA P-256 over the
      `vyzorix.v` canonical string, via `ServerSigner.SignDocument`
      (`apps/api/internal/infrastructure/crypto/device_signing.go`). Not
      `documents/SIGNING_ARCHITECTURE.md`, which covers the three HMAC request/frame
      domains (A/B/C) and not the config.
- [ ] The signature covers the whole document, including every artifact digest.
- [ ] `deviceId`, `organizationId` and `memberId` are injected by the server and cannot be set by the file.
- [ ] A config signed for device A fails verification on device B (device binding is inside the signed payload).
- [ ] The config carries a monotonic `version`, incremented on every successful deploy to that device.

### US-003: `POST /v1/deploy` with authorisation
**Description:** As a `super_admin`, I want to deploy; as any other role, I want to be refused unless granted.

**Acceptance Criteria:**
- [ ] `super_admin` in the target org may deploy without a grant.
- [ ] Any other role is refused with 403 unless a `permission_grant` grants `device.deploy` on a scope covering that device.
- [ ] A deploy naming a device that is not in the caller's org is refused with 404, not 403 (no existence leak).
- [ ] The response reports the new `version`, the artifacts it will fetch, and the member who authorised it.
- [ ] The request is rejected if the device is not `approved` (registration not complete).

### US-004: A device-scoped deploy credential
**Description:** As a security owner, I want a deploy credential that reaches one device, so a compromise does not reach the fleet.

**Acceptance Criteria:**
- [ ] A deploy credential can be minted scoped to a single device.
- [ ] That credential is refused for any other device, even in the same org.
- [ ] The credential can be revoked, and a revoked credential is refused immediately.
- [ ] The credential is never logged, and never returned by any GET.

### US-005: Config delivery and version drift
**Description:** As an operator, I want a device that was offline to catch up by itself.

**Acceptance Criteria:**
- [ ] A device that is connected receives the config as a `device_config` frame.
- [ ] A device that reconnects compares its held `version` to the server's and re-fetches when they differ.
- [ ] A device that is refused (signature, secret, digest) keeps its previous config and reports the refusal with a reason.
- [ ] The server records the version the device reports as applied, and the console shows held-vs-target.

### US-006: Artifact fetch, verify and atomic install
**Description:** As an operator, I want a payload that fails verification to change nothing.

**Acceptance Criteria:**
- [ ] An artifact is downloaded to a temporary location, verified against its `sha256`, and only then installed.
- [ ] A digest mismatch aborts that artifact, leaves the previous copy in place, and reports a distinct error.
- [ ] The install is atomic (temp-then-rename), so an interrupted install cannot leave a half-written app.
- [ ] A download can be resumed; a resumed file is verified over its whole length, not just the new part.
- [ ] The previous known-good set is retained after a successful deploy, and can be restored without network access.

### US-007: Apps materialise into `/home/apps` and the registry
**Description:** As the runtime, I want the manifest to become the registry the supervisor reads.

**Acceptance Criteria:**
- [ ] Each `[[app]]` becomes a directory under `/home/apps/<name>` with its artifacts unpacked inside.
- [ ] `apps.json` is written atomically (temp-then-rename) and includes `name`, `dir`, `cmd`, `port`, `env`, `autostart` per app.
- [ ] An app removed from the manifest is stopped and its registry entry removed; its **data** is retained unless the manifest says otherwise.
- [ ] A `cmd` that is not confined to the app's own directory is refused (FR-9).
- [ ] The supervisor picks the change up without a restart (its existing `inotify` path).

### US-008: A webroot artifact, and a generated PWA manifest
**Description:** As an operator, I want to ship a web app and decide whether it installs as a PWA.

**Acceptance Criteria:**
- [ ] An artifact with `kind = "webroot"` is unpacked into the app's directory and served by that app.
- [ ] With `pwa` absent or `false`, **no** `site.webmanifest` is generated and the app is served as plain HTML.
- [ ] With `pwa = true`, a `site.webmanifest` is generated into the webroot before the app starts.
- [ ] With a `[app.pwa]` table, every declared field (`name`, `short_name`, `description`, `theme_color`, `background_color`, `display`, `orientation`, `start_url`, `icons`) overrides the generated default.
- [ ] A generated manifest is valid JSON and passes a manifest-schema check in the test suite.
- [ ] `display` accepts only `standalone | fullscreen | minimal-ui | browser`; anything else is a hard error.
- [ ] Changing `theme_color` in the manifest and re-deploying changes the served manifest without touching the app's own files.

### US-009: `vyzorix deploy` CLI
**Description:** As an operator, I want one command that does the whole thing.

**Acceptance Criteria:**
- [ ] `vyzorix deploy --org ORG --device DEV [--file vyzorix.toml]` compiles, validates, uploads and reports progress.
- [ ] The CLI pre-validates against the server's published schema and fails before uploading on a schema error.
- [ ] The CLI prints the new version, and the list of artifacts with sizes, before it uploads.
- [ ] A non-zero exit is returned on any refusal, with the server's reason verbatim.
- [ ] `--dry-run` performs compile + authorise and stops short of delivery.

### US-010: A Deploy view in the console
**Description:** As an operator, I want to see what a device runs and who authorised it.

**Acceptance Criteria:**
- [ ] The view lists, per device: target version, held version, last deploy time, and the member who authorised the last deploy.
- [ ] Each app is listed with its state, port, and reachability mode.
- [ ] A refused deploy shows the reason (signature, secret, digest, `cmd`).
- [ ] The view is read-only in v1 — no deploy from the console.

### US-011: The Layer 3 gaps this depends on
**Description:** As the platform, I need the runtime to exist before a manifest can target it.

**Acceptance Criteria:**
- [ ] `/home/apps` is bound into the container and survives a rootfs replacement (Layer 3 decision D2).
- [ ] The supervisor is started by Layer 1 before any `APP_*` command can arrive (decision D3).
- [ ] `APP_START` / `APP_STOP` reach the supervisor's control socket and report failure, never silence.
- [ ] An app's port is reachable per its `expose` mode: `loopback` default, `lan` only with a token, `cloud` deferred.

### US-012: A second app, to prove this is a platform
**Description:** As a lead engineer, I want a non-Vfd tenant, because one tenant hides hardcoding.

**Acceptance Criteria:**
- [ ] A second app is deployable from the same `vyzorix.toml` mechanism, with **no** platform code change.
- [ ] It is a static site with `kind = "webroot"` and `pwa = true`.
- [ ] Both apps run on one device concurrently, on different ports.
- [ ] Removing the second app from the manifest stops it and leaves the first running.
- [ ] The deploy of the second app is exercised in the integration test suite.

### US-013: A secret reaches an app, without entering the config
**Description:** As an operator running a server-shaped app, I want to set a credential, so that the app works without the secret ever living in a git repo or a signed document.

**Acceptance Criteria:**
- [ ] `vyzorix secrets set <app> KEY=VALUE --device <id>` stores the value encrypted to that device's key.
- [ ] `vyzorix secrets list <app> --device <id>` shows key names and a generation, and **never** a value.
- [ ] The signed config names the secret keys an app expects and carries **no value**.
- [ ] The app receives the secret as an environment variable at start.
- [ ] A secret value never appears in `apps.json`, in a log line, or in any `GET` response — asserted by a test.
- [ ] A device cannot fetch a secret its current signed config does not name.
- [ ] A secret that will not decrypt is reported and the app is **not started** with the variable absent.
- [ ] `secrets set`/`unset` require the same `device.deploy` grant as a deploy.

---

## 5. Functional Requirements

- **FR-1** `vyzorix.toml` is parsed and validated **server-side**; the server is authoritative.
- **FR-2** The schema is published as a versioned JSON Schema at a stable, versioned URL
  (`GET /v1/deploy/schema/{version}`), and the CLI validates against it before upload.
- **FR-3** `deviceId`, `organizationId`, `memberId` are server-injected; the file may not set them.
- **FR-4** The compiled config is signed; the signature covers every field including artifact digests.
- **FR-5** The config carries a monotonic `version` per device.
- **FR-6** The config is delivered over the existing `device_config` frame; a reconnecting device re-fetches when versions differ.
- **FR-7** Every artifact reference is `{name, url, sha256, size?, kind?, arch?}`; `sha256` is mandatory. A URL is literal, never a template; the same binary for several ABIs is several references, each with its own digest and an `arch`.
- **FR-8** The compiler refuses a config carrying anything secret, and the device refuses to store one. Both sides check. The check is **structural** — it exempts the schema's own keys, including the `secret NAME` nodes that name a secret without carrying one — so a config that *names* a secret is accepted while one that *carries* a value is refused.
- **FR-9** `cmd` is confined to the target app's own directory under `/home/apps`; shell metacharacters and paths that escape it are refused.
- **FR-10** A curated catalog of known apps is offered in the console; a manifest naming an app outside the catalog is still permitted, but is marked as custom.
- **FR-11** Authorisation: `super_admin`, or a member with a `device.deploy` grant whose scope covers the device.
- **FR-12** A deploy credential may be scoped to a single device and revoked.
- **FR-13** Artifacts are fetched, verified by digest, and installed atomically; the previous set is retained.
- **FR-14** A `webroot` artifact may declare `pwa = true|false`, with an optional `[app.pwa]` table overriding generated defaults.
- **FR-15** An app removed from the manifest is stopped; its data is retained unless `purge = true` is declared for it.
- **FR-16** Every refusal is reported with a distinct, machine-readable reason (unsigned, bad-signature, secret, digest-mismatch, cmd-escape, unauthorized).
- **FR-17** Nothing in this feature is logged with a credential, a token, or a private key.
- **FR-18** `device.deploy` is a **new permission action**, separate from `apps.read`/`apps.write` — the grant that lets an operator see what an app is doing must not also let them install one.
- **FR-19** The app catalog is a `catalog_apps` section in the existing `provisioning` file, applied at boot like every other section.
- **FR-20** `[[app]]` gains `purge` (default `false`); it is the only thing that deletes an app's data, and it is recorded in the manifest so the deletion is reviewable.
- **FR-21** The device retains the current config and the previous known-good one (`keep_last = 2`), and restores the latter when an apply fails.
- **FR-22** The manifest schema is authored in CUE at `tooling/schema/deploy.cue`, generated for Go, and covered by the existing pre-commit drift gate.
- **FR-23** The deploy CLI is a new binary at `apps/cli`, sharing the API's generated types rather than re-declaring the wire shape.
- **FR-24** A secret value is never carried in a signed config; `[app.env]` is configuration and `[app.secrets]` is a table of **names** (a name is a key; the value is always empty). Values travel on their own channel, encrypted to the device's key. The table form is what lets the no-secret check (FR-8) exempt the node structurally instead of pattern-matching a field defined to hold secret names.
- **FR-25** The device holds a **second** Keystore key (`PURPOSE_DECRYPT`, RSA-3072) for secrets — separate from the identity key, never exposed over the bridge.
- **FR-26** A decrypted secret is written to `files/apps/<app>/.env` at mode `0600` and **never** into `apps.json`, which is readable by every app in the container.
- **FR-27** A device may fetch a secret's ciphertext only for a key the current signed config names for that app.

---

## 6. Non-Goals

- **No container images and no Docker.** The device is the runtime; the manifest names processes and artifacts, not OCI layers.
- **No deploy from the console in v1.** Read-only. (US-010)
- **No wildcard-DNS or TLS app proxy.** Layer 3 decision D7 defers it; `cloud` exposure is out of scope here.
- **No server-side app routes beyond what deploy needs.** The `apps.read`/`apps.write` scopes exist but no route checks them; Layer 3 decision D8 defers that work. Deploy uses `device.deploy` (FR-18), not `apps.write`.
- **No multi-device or group deploy in v1.** One device per deploy; org-level defaults are read but not applied fleet-wide.
- **No arbitrary code execution via the socket.** A device runs what a *signed* config names, never a live command.
- **No secret management UI in the console.** Secrets are set by the CLI and encrypted to a device; the console shows key *names* only, read-only. Editing a credential from a browser is a later feature.
- **No group or fleet secret.** A secret is encrypted to one device's key, so there is no shared secret in v1 (§10 of the contract).
- **No automatic rollback.** The previous known-good config is retained and restorable (FR-21); restoring it automatically when an app crash-loops is a later feature.
- **No offline first-deploy.** A device that has never been online has no pinned config and no artifacts, so it cannot be deployed to. Enrollment already requires one network round trip; every deploy after that is verifiable offline. This is a real product limit, not an oversight.
- **No `vyzorix undeploy`.** Removing an app from the manifest is how an app is removed (FR-15, FR-20); a separate destructive verb is out of scope.

---

## 7. Design Considerations

**Why references, not bytes.** Config is policy (~1 KB, changes often); an artifact is
content (tens to hundreds of MB, changes rarely). They have different delivery needs —
atomic apply versus resumable transfer — and putting the digest inside the signature is
what lets the transport be untrusted. This is the same split Fly uses (the API receives
a digest; the registry receives the bytes) and the same rule the existing OTA path
already follows.

**Why the PWA toggle is per-app.** "Is this installable" is a property of a webroot,
not of a device. `pwa = false` must generate *nothing* — an app that is not a PWA must
not be given a manifest by accident, because a manifest changes how the browser
presents and installs it.

**Reuse over invention.** The console already ships a working `site.webmanifest` and a
hand-written service worker; the generator's defaults are modelled on it. The device
already has `ConfigVerifier`, `PinnedServerKeys`, and an OTA path that checks a digest
before installing — the artifact fetcher generalises that rather than replacing it.

**The `cmd` field is the sharp edge.** A manifest that can name any command is remote
code execution with a signature. Confining `cmd` to the app's own directory (FR-9) means
a manifest can only run something it also shipped; the catalog (FR-10) makes the common
case safe by construction.

---

## 8. Technical Considerations

- **Trust anchor:** the device's pinned server public key (`PinnedServerKeys`, a
  rotatable set). Key rotation needs an overlap window — a device offline for months
  may only know an older key.
- **Signing:** ECDSA P-256 via `ServerSigner` over the `vyzorix.v` canonical string
  (`device_signing.go`); libv is the verifier on the device side. `SIGNING_ARCHITECTURE.md`
  describes the three HMAC domains for requests and frames, not this one.
- **Storage:** `files/apps` bound to `/home/apps` (Layer 3 decision D2, already
  implemented in `ProotLauncher.binds`). The supervisor's state must not live in the rootfs.
- **Artifact host:** GitHub releases, as the rootfs and the APK already use — public,
  so an unauthenticated device can fetch it.
- **Schema:** one shared schema; the CLI validates against it, the server enforces it.
- **Recovery:** a device that fails to start its apps must still be reachable — the
  bridge socket and the cloud link are Layer 1 and must not depend on Layer 3.
- **Verification surfaces to test:** an unsigned config, a tampered digest, a config
  carrying a secret, a `cmd` that escapes the app directory, a lower role without a
  grant, and a failed artifact leaving the previous set intact.

---

## 9. Success Metrics

- A new app is added to a device with **no platform code change** — manifest only.
- Time from `vyzorix deploy` to apps running is under a minute on a warm connection.
- Zero deployments where a device applies an unsigned, secret-bearing, or
  digest-mismatched config (verifiable from the refusal logs).
- A failed deploy leaves the device serving its previous version, measured by the
  device continuing to answer after an induced artifact failure.
- The second app (US-012) ships without touching anything under `apps/api/`.

---

## 10. Decisions (resolved)

These were open; each is now settled, with the evidence it rests on.

1. **Grant naming — a new action, `device.deploy`.**
   The existing actions are per-resource verbs (`device.read`, `device.write`,
   `device.delete`, `device.assign`, `apps.read`, `apps.write`) and `apps.*` is about
   *querying what an app is doing*, not about changing a device's contents. Folding
   deploy into `apps.write` would mean the permission that lets an operator read an
   app's state also lets them install one — a strictly larger grant than the name
   implies. Deploy is the highest-consequence action in this system (it changes what
   code runs), so it gets its own action and is granted explicitly.

2. **Catalog source — a server-side list, seeded from `provisioning`.**
   The repo already has a declarative, idempotent, ordered provisioning file
   (`provisioning.example.yaml`, applied at boot via `PROVISIONING_FILE`) whose
   sections include `organizations` and `api_keys`. The catalog is one more section
   there (`catalog_apps`), so it is reviewable in the same place as everything else
   and needs no new store. A manifest naming an app outside the catalog is still
   permitted (FR-10) but is marked custom — the catalog is a convenience, not a gate.

3. **`purge` semantics — per-app, and never implicit.**
   `[[app]]` gains `purge = false` by default. Removing an app from the manifest stops
   it and removes its registry entry; its data survives (FR-15). Deleting data is
   destructive and irreversible, so it needs an explicit field in the manifest that
   does it — not a separate verb, because a verb would let data be deleted without a
   reviewable artifact recording that it was asked for. `vyzorix undeploy` is out of
   scope for v1.

4. **Version history depth — the last good config plus the current one.**
   Two, not N. The recovery path that matters is "the deploy just failed, put back
   what was working", and one prior good version answers it. Keeping N invites a
   rollback picker, which is a product feature this PRD does not include, and a
   device's disk is the scarce resource. The retention is a count, not a policy:
   `keep_last = 2`.

5. **Offline first-deploy — not supported, and stated as a limit.**
   A device with no pinned config and no artifacts cannot be deployed to. It must be
   online at least once (the enrollment flow already requires this: it POSTs to
   `/v1/device/enroll`). What *is* supported is every subsequent deploy being
   verifiable offline. This is recorded in Non-Goals rather than left implicit,
   because it is a real product limitation and a user will hit it.

6. **Schema ownership — CUE in `tooling/schema/`, generated for Go, published for the CLI.**
   The repo already does this: `tooling/schema/*.cue` is the source, Go is generated
   from it, and a **pre-commit gate fails on drift**. Adding
   `tooling/schema/deploy.cue` means the manifest schema gets the same treatment as
   every other schema in the repo, including the drift check. The CLI does not read
   CUE: the server publishes the generated JSON Schema at a stable, versioned URL
   (`/v1/deploy/schema/{version}`) and the CLI
   validates against that (FR-2), so the CLI needs no CUE toolchain.

7. **Deploy CLI location — `apps/cli`, a new binary.**
   There is no CLI today (`apps/` holds `api`, `proxy-server`, and two web/mobile
   apps), so this is a new entry point. It goes in `apps/cli` so it shares the API's
   generated types rather than re-declaring the wire shape — the repo's convention is
   that generated types are the single source (AGENTS.md: "comes from the spec via
   orval, NOT from CUE" for TS; the same rule for Go).

---

## 11. Evidence from the existing experiment

An earlier experiment (`apps/api/public/vfd/`, untracked) dropped a built copy of the
Vfd console — including its `site.webmanifest` and `sw.js` — into the server's static
directory. It was never wired up, and it is useful precisely for what it proves and
what it does not.

**What it proves.** Its `index.html` contains **no baked engine URL** — only font
links. It does not need one: the env schema defaults `VITE_API_BASE_URL` to
`http://localhost:3000` (`packages/config/src/env/index.ts`), which is where the
device's engine listens. So a console served *by the device* reaches the device's own
engine **with no build-time configuration at all**. That is the whole
device-served-PWA idea working, and it is why US-008 is a file write rather than a
build system.

> The draft attributed the fallback to `connection-store` resolving
> `location.origin`. That file is not in this repo — it belonged to the untracked
> experiment — so the mechanism could not be checked here. The default above is in
> the repo, it lands on the same engine, and it is what this section now rests on.

**What it does not prove.** Nothing serves `public/` — no `http.FileServer`, no
`embed`, and no reference to the path anywhere in the Go tree. The files are inert.
And a manifest shipped *inside an artifact* cannot be customised per device, which is
exactly what US-008 requires, so the generator is still needed.

**Two consequences for this design:**

- **Same-origin serving removes the CORS and secure-context problems at once.** The
  engine already has `--allow-origin`; same-origin needs no CORS at all, and
  `127.0.0.1` counts as a secure context, so the service worker and the install prompt
  work without TLS. `expose = "lan"` is the case that needs a token (FR-11, P2 of the
  Layer 3 proxy requirements).
- **A `webroot` artifact needs no per-device build.** Ship the console once, generate
  the manifest on the device, and the same bytes serve every device.

