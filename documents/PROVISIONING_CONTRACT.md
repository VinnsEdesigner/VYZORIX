# Provisioning — Requirements Contract

Status: **proposed, not implemented**. Server-first. UI is phase 4.

## 1. Problem

Today VYZORIX provisioning is a **one-shot boot seed**: `PROVISIONING_FILE` is read
once at startup and applied (`application/provisioning`). There is no repository, no
connection, no sync, no drift detection, and no API to inspect it. An operator who
wants their configuration under version control has no path.

The user this is for: **someone self-hosting applications on their own devices**.
They keep their app deployment manifests in a git repo. They want VYZORIX to pull
that repo and converge their devices to it — and to tell them when a device has
drifted from it.

That is GitOps, and it is what Grafana's provisioning does for dashboards. The
analogue here is exact but the *resource* differs:

| Grafana | VYZORIX |
|---|---|
| Dashboard / folder | **Deploy manifest** (a `vyzorix.v` document targeting a device) |
| Dashboard folder | **Device group** (a named partition of devices) |
| Git repository | Git repository |
| Connection (provider credential) | Connection (provider credential) |
| Sync state per repo | Sync state per repo |

The mechanism transfers. The vocabulary does not, and copying Grafana's words
would produce a UI that describes something VYZORIX does not do.

## 2. What a "resource" is here — the load-bearing decision

A provisioned resource is **one deploy manifest for one device**. Concretely, the
file `devices/<id>/vyzorix.v` in the repository.

Consequences, each deliberate:

- **The device is part of the identity.** A manifest for `device-a` and one for
  `device-b` are two resources even if byte-identical, because applying either
  targets a different device. A resource is addressed by `(repo, path)`, and the
  path's device segment is what binds it.
- **A resource is not a database row the user edits.** It is a desired state. The
  server compiles, authorises and signs it through the existing `deploy` service,
  which already does exactly this for `POST /v1/deploy`.
- **Ownership is recorded, not inferred.** Every deploy applied by a repository
  is stamped with the owning repository. That stamp is what makes drift
  detectable and deletion safe. Without it, "which repo owns this config" is
  unanswerable and orphan cleanup is impossible.

Non-resources, explicitly: orgs, users, service accounts, API keys, alert rules,
contact points, annotations. They stay in the existing boot seed (§9). They are
not per-device and do not fit a per-device manifest.

## 3. Domain model

```
Connection 1───n Repository 1───n ProvisionedResource
                                    │
                                    └─ (device_id, path, repo_id)
```

### Connection

A stored git provider credential, shared by repositories.

- `id, org_id, name, type (github|gitlab|generic), host`
- Secret material: token, stored **encrypted at rest** using the existing
  `secretCipher` mechanism (`UserRepository.SetSecretCipher` is the precedent).
  The plaintext is write-only: no GET returns it, ever. This mirrors
  `deploycredential`, which stores only a hash and returns the plaintext once.
- `status`: `pending | ready | error`, with `last_checked_at`.
- Health is a **live check** (can we reach the host and authenticate?), not a
  cached flag. A credential that expires must go red without anyone pressing
  anything.

### Repository

- `id, org_id, connection_id, name, title, description`
- `type`: `github | gitlab | generic`
- `url, branch, path` (path = subdirectory to sync, default `/`)
- `sync`: `enabled bool`, `interval_seconds`, `target` (`device` | `group`)
- `status`: `sync_state`, `last_sync_at`, `last_error`, `stats[]`
- `sync_state`: `pending | success | warning | working | error | disabled`

One repository = one branch. Syncing two branches is two repositories. Grafana
does the same, and the alternative (a repo with N branch scopes) makes every
status field ambiguous.

### ProvisionedResource

- `id, repo_id, device_id, path`
- `version` — the signed config version produced by the deploy service
- `content_hash` — SHA-256 of the manifest bytes, so drift is detectable without
  re-signing
- `state`: `applied | drifted | missing | error`
- `last_applied_at, last_error`

`state` is derived, never set by hand:
- `applied` — the device's current config version matches the one this repo
  applied, and the file still exists in the repo at the same hash.
- `drifted` — the device's config was changed by something else (a manual
  `POST /v1/deploy`, a rollback) after this repo applied it.
- `missing` — the file was deleted from the repo but the device still runs the
  config this repo applied.
- `error` — the last apply failed.

## 4. API contract

All routes are **org-scoped** and guarded by scoped permissions (§6). Base
`/v1/provisioning`.

### Connections

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/connections` | list, with `status` and `last_checked_at` |
| `POST` | `/connections` | create; returns the id, **never** the secret |
| `PATCH` | `/connections/{id}` | rename / rotate the token |
| `DELETE` | `/connections/{id}` | 409 if any repository still references it |
| `POST` | `/connections/{id}/check` | force a health check, returns the result |

### Repositories

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/repositories` | list with `status`, `stats` |
| `POST` | `/repositories` | create; validates access to the remote before storing |
| `GET` | `/repositories/{id}` | one, with status |
| `PATCH` | `/repositories/{id}` | edit branch/path/sync settings |
| `DELETE` | `/repositories/{id}` | `?keep_resources=true` releases ownership instead of reverting |
| `POST` | `/repositories/{id}/sync` | trigger a sync now; returns a job id |
| `GET` | `/repositories/{id}/resources` | list provisioned resources with `state` |
| `GET` | `/repositories/{id}/resources/{resourceId}/document` | the signed document |
| `GET` | `/repositories/{id}/jobs/{jobId}` | sync job progress |

### Instance-level

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/v1/instance/provisioning` | is a boot seed configured, path, last applied, per-kind counts |

`POST /repositories` **must** verify the remote before storing: clone-check the
URL and branch with the connection's credential. Storing a repository that cannot
be reached turns a typo into a permanently red row.

## 5. Sync engine

A sync is one job, recorded, with progress:

1. **Fetch** — clone/fetch the branch into a temp dir. Failure ⇒ job `error`,
   repository `sync_state = error`, `last_error` set.
2. **Discover** — walk `path`, collect every `vyzorix.v`. A malformed manifest is
   a per-resource error, not a job failure: one bad file must not block the rest.
3. **Diff** — compare against `provisioned_resources` for this repo:
   new / changed (hash differs) / unchanged / missing (file gone).
4. **Apply** — for each new or changed manifest, call the **existing**
   `deploy.DeployService.Deploy(ctx, DeployRequest)` with the manifest bytes
   (`apps/api/internal/deploy/vyzorix_deploy_service.go:82`). It already performs
   compile → authorise → sign → persist → push and is covered by roundtrip and
   refusal tests. Provisioning **must not** reimplement any of that; it only
   decides *which* manifests to apply and records the outcome.
   `DryRun` on the same request is what powers the preview step.
5. **Reconcile missing** — for each resource whose file disappeared, mark it
   `missing`. **Do not auto-revert by default.** Deleting a device's config
   because a file moved is a destructive surprise. `missing` is surfaced, and
   reverting is an explicit action (`?revert_missing=true` on sync).
6. **Record** — write the job result, update `stats[]`, set `last_sync_at`.

### Invariants

- **A sync never deletes a device.** It changes configs. Device lifecycle is not
  provisioning's business.
- **Two repositories must not own the same device.** Enforced at repository
  creation and at apply time: if device D is already owned by repo A and repo B
  tries to apply to D, B's resource goes to `error` with a clear message. Silent
  last-writer-wins would make drift unexplainable.
- **A failed apply never leaves a half-written config.** The deploy service is
  already atomic per device; the job records the failure and continues with the
  other resources.
- **Sync is idempotent.** Re-running with no repo changes performs no writes.

### Scheduling

Interval-based, per repository, minimum 60s. A manual sync resets the timer. No
webhooks in v1 — they require public reachability that a self-hoster may not have,
and Grafana's own "public access" setup screen exists precisely because of that.
Webhook support is a later addition, not a v1 requirement.

## 6. Permissions

Two new **org-scoped** actions:

- `provisioning.read` — view repositories, connections, resources, jobs.
- `provisioning.write` — create/edit/delete repositories and connections, trigger syncs.

Granted by default to `admin` and `super_admin`; **not** to `operator` or `viewer`.
A provisioned repository can rewrite any device's config, so write access to it is
equivalent to fleet-wide deploy. That is admin authority, not operator authority.

Secrets are never returned by any route, under any permission. A connection's
token is write-only.

## 7. UI contract (phase 4, after the server is proven)

### 7.1 Tabs

Three tabs, matching the existing `pageTabs` declaration:

- **Repositories** — a `Card` per repository: provider icon, title, sync badge,
  resource counts as buttons, last-sync relative time, then `View` / `Sync` /
  `Settings`. Primary action: **Connect repository**.
- **Connections** — a list of stored credentials with a health badge and a
  `•••` → `OverlaySheet` menu (Check now / Edit / Delete), consistent with the
  token rows. Primary action: **Add connection**.
- **Get started** — explains what a provisioned resource is *for VYZORIX* (a
  per-device deploy manifest), the directory layout, and a copyable example.
  No action button.

Empty state is **Get started**, not an empty table — a new install has nothing to
list and should be taught, not shown a void.

### 7.2 The connect wizard — five steps

Grafana's wizard is the model, and the step *sequence* transfers because both
products go: authenticate → point at a location → choose what to sync → run the
sync → configure the schedule. The step *content* differs because the resource
differs (deploy manifests, not dashboards).

| # | Step | What the user does | Server call |
|---|---|---|---|
| 1 | **Connect** | Choose a provider (GitHub / GitLab / Generic git) and paste a token, or reuse an existing connection. | `POST /connections`, `POST /connections/{id}/check` |
| 2 | **Configure repository** | Repository URL, branch, path, and a display title. | `POST /repositories` (validates the remote before storing) |
| 3 | **Choose what to synchronize** | Pick the sync target and see what was discovered. | `POST /repositories` returns the discovery summary |
| 4 | **Synchronize** | A **dry run** of every manifest, then apply. Live per-resource progress. | `POST /repositories/{id}/sync?dry_run=true`, then without |
| 5 | **Choose additional settings** | Sync interval, read-only mode. Finish. | `PATCH /repositories/{id}` |

Step 1 is **skipped** when the user launched the wizard from an existing
connection (`Add connection` already created it). Grafana does the same for local
repositories, where there is no auth step at all. A skipped step is marked
complete, never merely hidden, so the stepper's numbering stays honest.

#### Step 3 — the step that is genuinely VYZORIX-specific

Grafana asks "bootstrap: which dashboards do you want to migrate?". VYZORIX asks
**"which devices will this repository own?"** — because a manifest is bound to a
device, this is where ownership is decided and where the one-owner rule is
enforced.

It shows the discovered manifests grouped by device, and for each one:

- **will be applied** — new manifest for a device this repo does not yet own
- **will be updated** — hash changed
- **unchanged**
- **conflict** — the device is already owned by another repository; the row is
  disabled with the owner named. This is the `UNIQUE(device_id)` constraint
  surfaced *before* the write, not as a 409 after it.

#### Step 4 — dry run is mandatory

The preview is not a nicety. This is the only screen where a user sees the effect
on their fleet before it happens, so **Next is disabled until the dry run has
run and returned**. The results render per resource: `would apply v12`,
`unchanged`, `would fail: <parse error>`.

Apply is a separate, explicit action on the same screen. A dry run must never be
silently promoted to a real one.

#### Step 5 — settings

- **Sync interval** (seconds, minimum 60).
- **Read-only** — when on, VYZORIX never writes back to the repository. Relevant
  only once push-back exists; until then the toggle is present but explains that
  it has no effect yet rather than pretending to work.

No PR-workflow / push-to-branch options in v1: those are write-back features, and
v1 provisioning only pulls. Offering a toggle for a capability that does not
exist is worse than omitting it.

#### Wizard behaviour

- **Stepper** on the left: numbered steps, completed steps show a check, the
  active step is highlighted, connectors between. A vertical list, as in Grafana.
- **Navigation**: `Previous` / `Next`. `Next` is disabled while the step's
  required fields are invalid or its async work is running. `Previous` discards
  nothing — the form state persists across steps.
- **Cancelling** mid-wizard must **delete the partially created repository and
  connection**. Grafana does exactly this (`useWizardCancellation` →
  `onRepositoryDeletion`). Leaving a half-configured repository behind means a
  red row the user never intended to create.
- **Resuming**: the wizard is not resumable across reloads in v1. State lives in
  the form, and abandoning the tab cancels as above. Persisting half-finished
  wizards needs a draft concept that is not worth its complexity yet.
- **Failure at any step** shows the server's error inline on that step and keeps
  the user there. It never advances with a broken prerequisite.

### 7.3 Reuse

The stepper, `•••` sheet menu, `OverlaySheet`, `ConfirmModal`, `InfoBox`, and the
`Page`/`Tabs` chrome already exist and must be reused. The only new UI primitives
are the stepper (a vertical numbered list) and the per-resource preview table.

### 7.4 Every async state gets a component

**Rule: no inline async-state ternaries in a page or container.** Each state that
a user can be left in is a named component in its own file, and the container
renders it by name.

States that must exist as components, per surface:

| Surface | Components |
|---|---|
| Repository list | `RepositoryListLoading`, `RepositoryListEmpty`, `RepositoryListError`, `RepositoryList` |
| Repository detail | `RepositoryLoading`, `RepositoryError`, `RepositoryHealthCard`, `RepositorySyncBadge` |
| Sync | `SyncIdle`, `SyncRunning`, `SyncSucceeded`, `SyncPartialFailure`, `SyncFailed` |
| Resource row | `ResourceApplied`, `ResourceDrifted`, `ResourceMissing`, `ResourceError` |
| Connection | `ConnectionPending`, `ConnectionReady`, `ConnectionError`, `ConnectionUnreachable` |
| Wizard step | `StepIdle`, `StepValidating`, `StepRunning`, `StepFailed`, `StepSucceeded` |

Why this is a requirement and not a preference: a sync feature's difficulty is not
its happy path, it is the state space. `{isLoading ? <Spinner/> : err ? <Alert/> :
rows.length ? <Table/> : <Empty/>}` hides four states in one line, and by the
fifth state the line is unreadable and untested. A named component per state is
individually testable, individually styleable, and reviewable at a glance.

Naming: `<Surface><State>` — the surface first, so the set sorts together in the
directory listing.

Exceptions: a two-state toggle inside an already-named component (a button's
`loading` prop) does not need its own file. The rule targets *page-level* async
states, not every boolean.


## 8. What happens to the existing boot seed

**Keep it, narrow it.** `PROVISIONING_FILE` continues to seed orgs, users, groups,
service accounts, keys, grants, alert rules, contact points, annotations, catalog
apps. It **stops** being the only story and gains a read API
(`GET /v1/instance/provisioning`) so the UI can show it.

Reason: those resources are deployment-shaped (an org, a user), not
device-shaped. Folding them into a per-device manifest would be a false
unification. The two coexist: the boot seed bootstraps the instance, repositories
converge the devices.

No rewrite of `application/provisioning` is required. It is called once at boot
and stays that way.

## 9. Schema

Migration 101 creates five tables. Constraints carry three rules that must not
depend on application code remembering to check.

| Table | Purpose |
|---|---|
| `provisioning_connections` | a stored git credential; `token_ciphertext` is write-only |
| `provisioning_repositories` | one branch of one remote, synced on an interval |
| `provisioning_resources` | one manifest, bound to one device |
| `provisioning_jobs` | one sync run, with aggregate counters |
| `provisioning_job_results` | one manifest's outcome within one job |

### Constraints, and why each is a constraint

**`CREATE UNIQUE INDEX ... ON provisioning_resources(device_id) WHERE released_at IS NULL`**
— at most one *owning* repository per device. It is **partial** deliberately: a
plain `UNIQUE(device_id)` also satisfies "one owner" but makes re-adoption
impossible forever, because a released row still exists. `released_at` marks a
device the repository gave up without reverting, freeing it for another
repository to adopt. Verified: swapping the partial index for a plain
`UNIQUE(device_id)` makes adoption fail with `UNIQUE constraint failed`.

**`UNIQUE(repo_id, path)`** — a repository holds one resource per manifest path.

**`FOREIGN KEY (connection_id) REFERENCES provisioning_connections(id) ON DELETE RESTRICT`**
— a connection in use cannot be deleted. The service also checks and returns a
409 with a helpful message, but the schema is the guarantee.

**`FOREIGN KEY (repo_id) REFERENCES provisioning_repositories(id) ON DELETE CASCADE`**
on resources, jobs, and (via job) job results — deleting a repository cannot
leave orphaned ownership rows that would block adoption permanently.

**`FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE`** — an
organisation's provisioning rows go with it.

### Why not finalizers

Grafana uses Kubernetes-style finalizers: a delete sets a timestamp, a controller
runs cleanup, then removes the finalizer. That exists because Grafana's repository
owns *live objects* (dashboards) that may need API calls to clean up, and because
Kubernetes has a controller loop to drive it.

VYZORIX has neither. Cleanup here is pure SQL: delete rows. A finalizer would add
a state machine (`deleting` state, a reconciler, a way to force-release a stuck
finalizer) to solve a problem the database solves in one `ON DELETE`. The one
thing finalizers buy — "tell the user why the delete is blocked" — the service
already does with a 409 that names the owning repository.

## 10. Failure modes that must be handled explicitly

| Failure | Required behaviour |
|---|---|
| Credential expired | Connection `error`; dependent repos `error` with "connection unavailable"; **no** config changes |
| Branch force-pushed | Resources whose hash vanished go `missing`; nothing auto-reverts |
| Manifest invalid | That resource `error` with the parse message; others still apply |
| Device deleted from fleet | Resource `error` "device not found"; the repo stays healthy |
| Device not in the repo's org | Resource `error`; never cross-org |
| Two repos claim one device | Second repo's resource `error`, message names the owner |
| Sync already running | 409 on a second manual trigger; scheduled run is skipped, not queued |
| Server restart mid-sync | Job left `working`; on boot, jobs older than the interval are marked `error` "interrupted" |

## 11. Scope

### 11.1 v1 — pull-only GitOps

In scope:

- Org-scoped connections and repositories.
- Pull on an interval; manual sync.
- Deploy manifests only: `devices/<device-id>/vyzorix.v`.
- Dry run before apply.
- Drift detection (`applied` / `drifted` / `missing` / `error`).
- Sync job history.
- The five-step wizard.

Out of scope for v1, with reasons:

| Excluded | Why |
|---|---|
| Push-back to git (edit a provisioned config in the UI) | Needs PR workflow, branch-per-edit, commit signing, conflict resolution — roughly doubles the feature (§14.1) |
| In-UI editing of provisioned resources | Same; a provisioned config is owned by the repo, editing it in the UI invites divergence |
| Webhooks / push-based sync | Requires public reachability a self-hoster may not have |
| Multiple branches per repository | Makes every status field ambiguous; two branches = two repositories |
| Auto-revert on file deletion | A force-push would silently wipe a device; explicit action only |
| One device owned by two repositories | Makes drift unexplainable; enforced by `UNIQUE(device_id)` |
| Cross-org provisioning | Violates the tenancy boundary |
| Migrating existing configs into git | Nothing to migrate: configs are already produced from manifests |

### 11.2 v2 — full bidirectional GitOps

Sequenced by value, not by size. Each item is independently shippable.

**v2.1 — In-UI editing with push-back.** The largest piece and the reason
Grafana's feature is 28k lines. Editing a provisioned manifest in VYZORIX and
committing it back to the repository. Requires:
- Branch-per-edit with a generated branch name.
- A PR workflow mode and a direct-to-branch mode.
- Commit message templating.
- Push credentials with write scope on the connection (a read-only token is not
  enough; the connection gains a `write_enabled` flag).
- A preview of the exact diff before pushing.
- Conflict handling when the remote moved while the user was editing.

**v2.2 — Full discovery.** Today the sync walks for `vyzorix.v` files. Full
discovery means enumerating what the repository contains *before* deciding what to
sync: a tree view of devices and manifests, showing which are managed by this
repo and which are not. This is what makes step 3 of the wizard rich instead of a
flat list.

**v2.3 — Webhooks.** Provider webhooks so a push triggers a sync instead of
waiting for the interval. Needs a public endpoint, a per-provider secret, and a
verification path. Falls back to the interval when unreachable.

**v2.4 — Additional resource kinds.** Only if a second kind of resource becomes
per-device-shaped. Orgs, users, and alerts stay in the boot seed — they are
deployment-shaped, and forcing them into a per-device manifest is a false
unification (§8).

**v2.5 — Instance-level connections.** One provider credential shared by every
org, for a single-tenant self-hoster who does not want to re-enter a token per
org. An addition to §12.3, not a replacement.

**v2.6 — Repository health over time.** Sync duration, failure streaks, a
timeline. Needs the job table to retain history rather than the latest run only.

### 11.3 What never comes

- Provisioning a device from two repositories. The constraint is load-bearing.
- Silent auto-revert on deletion.
- Storing a connection token in plaintext, or returning it from any GET.

## 12. Decisions taken

All four questions are answered; these are now requirements, not options.

1. **Repo layout — one device per file.** `devices/<device-id>/vyzorix.v`. One
   file is one resource, bound to one device. It matches the deploy API's
   one-device-per-call shape and keeps `content_hash` unambiguous. A manifest
   targeting several devices is explicitly out of scope (§11).
2. **Deletion policy — flag only.** A manifest deleted from the repo moves its
   resource to `missing`. **Nothing reverts automatically.** Reverting is an
   explicit action (`?revert_missing=true`). A force-push must never silently
   wipe a device's config.
3. **Connection scope — org-scoped.** A connection belongs to an organisation and
   is reusable by that org's repositories. Instance-level connections are a later
   addition, not v1.
4. **Secret storage — reuse `secretCipher`.** The same mechanism already guarding
   the Slack bot token (`UserRepository.SetSecretCipher`). One encryption story
   in the codebase, not two.

Additionally, the connect flow is a **five-step wizard** (§7.2), matching
Grafana's sequence.

## 13. Sequencing

1. Schema + domain (`provisioning_connection`, `provisioning_repository`,
   `provisioning_resource`) with tests.
2. Connection CRUD + health check; secret encryption.
3. Repository CRUD + remote validation.
4. Sync engine, dry run first, then apply; jobs.
5. Permissions + route guards + contract test.
6. UI: tabs, cards, wizard.
7. Delete the boot seed's device-adjacent parts **only if** §8's split proves
   redundant in practice — not before.

## 14. Resource kinds — a second layout beside device manifests

Status: **required.** Supersedes the v2.4 note in §11.2, which deferred this.

### 14.1 The problem this solves

§8 kept organisations, users, service accounts, keys, alert rules, contact points,
and annotations in the boot seed, on the grounds that they are deployment-shaped
rather than device-shaped. That reasoning is about **identity**, and it holds.

The consequence was not intended: **an alert rule can only be changed by editing a
file on the server and restarting it.** No review, no diff, no history, no drift
reporting. For a product whose claim is "your configuration lives in git", that is
the largest remaining inconsistency.

The fix is not to force org-shaped things into per-device files. It is a **second
resource kind** with its own directory layout, sharing the repository, connection,
sync job, and drift model.

### 14.2 The layout

```
devices/<device-id>/vyzorix.toml     → one device's config        (kind: device)
orgs/<org>/alerts.toml               → alert rules                 (kind: alerts)
orgs/<org>/contact-points.toml       → notification targets        (kind: contact_points)
orgs/<org>/annotations.toml          → annotations                 (kind: annotations)
orgs/<org>/groups.toml               → device groups + membership  (kind: groups)
```

Rules:

- **The directory decides the kind.** `devices/` and `orgs/` are the two anchors. A
  file under neither is reported as an error naming the file, not silently ignored —
  the same rule `deviceFromPath` already applies to a manifest outside `devices/`.
- **`orgs/<org>` names the organisation.** `<org>` is matched against the
  organisation's **name**, not its id, because a name is what a person writes in a
  file. Resolution is org-scoped: a repository in org A cannot reach org B by
  naming it, so a file naming an organisation the repository does not belong to is
  an error for that resource.
- **One file per kind per organisation.** Two `alerts.toml` files for one org is a
  duplicate and refused, mirroring the one-manifest-per-device rule.
- **A resource is addressed by `(repo, path)`** exactly as a device manifest is, so
  the existing `UNIQUE(repo_id, path)` constraint carries it unchanged.

### 14.3 What each kind contains

**Implementation status:** `alerts.toml` is implemented end to end. The other three
kinds are recognised by discovery — the file is classified and its organisation
checked — but report `no applier is configured for <kind> files` until their applier
lands. That failure is deliberate: a `groups.toml` that silently did nothing would be
worse than one that says why.

An applied org file is a **resource row like any other**: it carries `kind`, a content
hash, a state, and one owning repository, so it appears in the resource list, counts in
`stats[]`, and is released by `keep_resources` exactly as a device manifest is. Its
`device_id` is null. The device plan ignores org rows, so an applied `alerts.toml` is
never reported `missing` by the device half of a sync.

Every org file is a list of named objects. Each object is matched to an existing
record **by name within its organisation**, so a file can create, update, and
(only explicitly) delete.

**`alerts.toml`**

```toml
[[rule]]
name      = "device-offline"
metric    = "device_offline_count"
condition = "gt"
threshold = 0
for       = "5m"
notify    = "5m"
enabled   = true
```

Maps to `alert.Rule` (name, metric, condition, threshold, `for`, notify interval,
enabled). `webhook_url` is **refused** in a provisioned rule: a secret-bearing URL
belongs in a contact point, which already stores it protected.

**`contact-points.toml`**

```toml
[[contact_point]]
name    = "oncall-slack"
channel = "slack"
# secret material is referenced, never embedded
secret_ref = "slack-oncall"
```

`secret_ref` names a value stored in the existing secret store, never the value
itself. A contact point whose `secret_ref` does not resolve fails **at apply time**
with a message naming the missing secret (§17).

**`annotations.toml`**

```toml
[[annotation]]
title = "Deploy freeze"
text  = "No deploys during the audit window."
start = "2026-12-01T00:00:00Z"
end   = "2026-12-03T00:00:00Z"
tags  = ["change-control"]
```

**`groups.toml`**

```toml
[[group]]
name = "warehouse"

[[group.member]]
device = "scanner-01"
[[group.member]]
device = "scanner-02"
```

Membership is **declarative**: the file states the complete membership of a group
it owns. A device absent from the file is removed from that group. This is the one
place a provisioned file removes a relationship, and it is safe because membership
is not a destructive change to the device — it changes grouping, not configuration.

### 14.4 Ownership and drift, per kind

- **Ownership is recorded the same way** for every kind: an applied object is
  stamped with the owning repository. Two repositories cannot own the same object
  (`UNIQUE(kind, target, name) WHERE released_at IS NULL`), mirroring the
  one-owner-per-device rule.
- **Drift is detectable per object.** Each object records the hash of the
  declaration that produced it. A rule edited in the console after provisioning
  shows as `drifted`, exactly as a device config does.
- **Deletion is flag-only**, for every kind. Removing `alerts.toml` marks its
  objects `missing`; it does not delete the rules. Reverting is explicit, as §12.2
  requires for devices.

### 14.5 What stays in the boot seed

**Users, service accounts, API keys, and permission grants.** These are identity,
and identity-as-code can grant authority. A bad `alerts.toml` produces a wrong
alert; a bad `grants.toml` produces an admin. If identity-as-code is ever added it
must be opt-in per organisation, preview-mandatory, and unable to delete a user
(disable only). It is explicitly **not** part of this section.

The boot seed continues to apply once at boot and remains the way to create the
first organisation and its first admin. §8 stands.

## 15. Selective sync — per-repository includes and excludes

Status: **required.**

### 15.1 Problem

One repository syncs everything under its configured path. A team running a staging
repository and a production repository over the same tree, or one repository holding
manifests for several teams, cannot say "only this part".

### 15.2 Requirements

- Two new repository fields, `include` and `exclude`: **lists of glob patterns**,
  matched against the resource path relative to the repository's `path`.
- **Exclude wins over include.** A path matching both is excluded; the safe reading
  of an ambiguous rule is "do not touch it".
- **Empty include means everything** under `path`. Empty exclude means nothing
  excluded.
- **Filtering happens at discovery**, before the plan is built, so an excluded file
  is not fetched, not planned, and not reported. It is not "planned then skipped".
- **A file excluded after it was applied is not `missing`.** It is outside this
  repository's declared scope, which is different from deleted. Its resource moves
  to a new state, `ignored`, so the difference is visible. Nothing reverts.
- **Globs are validated on save.** A malformed pattern is a `400` on the repository,
  not a sync that silently matches nothing.

### 15.3 Pattern semantics

`path.Match` semantics, with one deliberate addition:

- `*` does not cross `/`, `?` is one character, and `**` is **not** special.
- **A pattern ending in `/*` means the whole subtree.** `orgs/*` covers
  `orgs/acme/alerts.toml` *and* anything deeper, because that is how a person writes
  "everything under orgs/". Plain `path.Match` would match one level only and silently
  exclude nothing, which is the quiet no-op this filter must not have.
- Patterns are matched against the resource path with a leading slash normalised away
  (`devices/a/vyzorix.toml`). A pattern with no wildcard matches only that exact path.

This is deliberately the simplest thing that works; a full glob library is not
warranted for a config filter.

## 16. Sync a single device

Status: **required.**

### 16.1 Problem

A sync is whole-repository. After one device fails to apply, the common operational
request is "re-apply just that device", and today the only answer is a full pass
over every file.

### 16.2 Requirements

- `POST /v1/provisioning/repositories/{id}/resources/{resourceId}/sync` runs the
  normal fetch, discovery, and plan, then **applies only the one resource**.
- **The preview and the real run still use the same plan.** The endpoint does not
  get a second code path: it filters the plan to one item and runs the existing
  apply.
- **The job records the whole plan but applies one item.** Results for the other
  items are `skipped`, so the job shows what was known and what was done. A job
  that applied one file must not claim the rest were unchanged.
- **It still takes the per-device lock** and the same `provisioning.write`
  permission. A single-device sync is not a lesser authorisation.
- **The resource must belong to the repository in the path.** A resource id from
  another repository is a `404`, not a cross-repository apply.
- **It does not stamp the whole repository's sync state**, because it did not sync
  the repository. It records a job and the resource's own state.

## 17. Secret references in git

Status: **required.**

### 17.1 Problem

A manifest names the secrets an app needs; it never contains them, and that must
stay true. But nothing checks the secret **exists** until the device asks for it,
which is the worst possible moment to discover a missing value.

### 17.2 Requirements

- A manifest may declare, per app, the secret keys the app requires. It already
  can, as `[app.secrets]` keys. This is now **checked at apply time**.
- **Apply fails for that resource when a required secret is not set**, with a
  message naming the app and key. The rest of the fleet still applies, per §5.
- **The check is a read of the secret store, not the value.** Nothing about the
  value reaches the config, the job, or the log.
- **A secret set later does not retroactively apply.** The next sync applies it,
  which is the existing model; the value is fetched by the device, not pushed.
- **An undeclared secret is not an error.** A device may hold a secret its current
  manifest does not name; it is simply not fetched, which is the existing gate.

### 17.3 Client requirements

- The resource detail view lists each declared secret and whether it is set, using
  the existing secret-list endpoint. It never shows a value.
- A resource whose apply failed for a missing secret shows which key is missing,
  with a link to set it.

## 18. Client requirements for this phase

The console screen (phase 4 of §7) must not assume every resource is a device.

### 18.1 Resource kind is a first-class field

- The resource list carries `kind` and filters by it: **Devices / Alerts / Contact
  points / Annotations / Groups**.
- The wizard's step 3 groups discovered resources by kind, then by device or
  organisation. A device manifest shows "will be applied to device X"; an
  `alerts.toml` shows "will update 4 alert rules in organisation Y".
- A resource row links to its own detail view; a device row keeps its current one.

### 18.2 Selective sync in the connect wizard

- Step 2 (configure repository) gains **include** and **exclude** as pattern lists,
  with a live count of matched files from the discovery endpoint.
- The preview (step 4) shows the effect of the filters, so a user sees what is out
  of scope before syncing.

### 18.3 Per-device sync

- The resource detail view has **Sync this device**, next to the repository-level
  Sync. It is disabled while a repository sync runs, and says so.
- Its job result is shown inline, not only in the jobs list.

### 18.4 Secret readiness

- As §17.3: declared secrets and whether each is set, never a value.

### 18.5 The rule that still holds

§7.4 applies unchanged: every async state is a named component, never an inline
ternary. The new states this phase adds are `ResourceIgnored`, `SecretReady`,
`SecretMissing`, and `DeviceSyncRunning`.

## 19. Sequencing for this phase

Each step is independently shippable and testable.

1. **Selective sync** — repository fields, discovery filter, `ignored` state, save
   validation.
2. **Per-device sync** — one endpoint, filtered plan, job semantics.
3. **Secret references** — apply-time check, resource detail readiness.
4. **Resource kinds** — schema, discovery, the four appliers, ownership and drift
   per kind.
5. **Client** — kind filter, wizard filters, per-device sync, secret readiness.

Steps 1–3 do not touch the resource model. Step 4 is the largest and lands last
because it adds a dimension the others do not depend on.

## 20. Repository browsing

Status: **implemented.** The console can read the repository without a git client.

### 20.1 The problem this solves

§7 makes the console the place a person manages provisioning. Every other view there
answers a question about *state* — what was applied, what drifted, what failed. None of
them answers the question a person asks next: **what is actually in the repository?**

Without that, diagnosing a failed file means cloning the repository somewhere else. That
is a worse experience than the API it replaces, and it makes the console a partial view of
the very thing it claims to manage.

### 20.2 Requirements

- **Three read routes**, all under `provisioning.read`:
  - `GET .../repositories/{id}/tree?dir=` — one directory, `dir` empty meaning the root.
  - `GET .../repositories/{id}/file?path=` — one file, with the commit it came from.
  - `GET .../repositories/{id}/history?limit=` — recent commits, newest first.
- **Reading works on a read-only connection.** A credential without push scope must
  still browse: seeing what is in a repository is harmless, and an operator who cannot
  push should not be blind.
- **The token comes from the repository's own connection.** The caller never supplies a
  credential; there is no route that accepts one.
- **A path is confined to the checkout.** A `..` segment is refused with `400`, not
  resolved — the path arrives from the network.
- **A file read carries its commit.** The response names the commit, author, and time, so
  a subsequent edit can be based on that revision and the existing conflict check
  (base-commit mismatch) still means something.
- **A large file is truncated and says so**, rather than being streamed whole through
  JSON.
- **History is a bounded window** (50 commits). A full clone of a large repository is
  minutes of work to answer "show me the log", so the clone is bounded and the response
  reports the depth it covered.
- **Editing is not browsing.** An edit still goes through the resource's preview and push
  routes, which carry the write scope and the base-commit check. A file that is not a
  provisioned resource has no edit path at all.

### 20.3 Client

- The **Code** tab reads a source's repository: a Files view (tree with breadcrumbs, and a
  file view with commit metadata) and a **History** list.
- An **Edit** button appears only when the connection has write access *and* the file is a
  provisioned resource. Otherwise the file is read-only, and the UI says why rather than
  offering a dead button.

## 21. Device evidence — a state is a claim about a device

Status: **implemented.** Supersedes the "applied on successful deploy" behaviour.

### 21.1 The problem this solves

§3 defines `applied` as "the device is running the config this repository applied". An
earlier implementation stamped `applied` whenever the deploy service returned success —
which only proves the server compiled, signed and stored a document. The device may have
been offline, unregistered, or simply not polled yet.

A state read from a database row is therefore not evidence. It is a stored guess. Reporting
it as fact is the same class of error the `drifted` state exists to prevent, in the
opposite direction.

### 21.2 Requirements

- **A deploy records `pending`, not `applied`.** Only a device report can move a resource
  to `applied`.
- **The report must be current.** A device that is connected now is current. A device that
  is not connected counts while its last report is inside the offline window — the same
  threshold diagnostics uses (`DIAG_OFFLINE_THRESHOLD_MINUTES`), so no two features
  disagree about when a device stopped being reachable. A report older than that is
  **stale**: the resource stays `pending` and its reason says the device is offline with a
  stale report, which is a different fact from "has not reported yet".
- **A device report moves the row in both directions.** `pending → applied` when the device
  reports the version this repository applied; `applied → pending` when it reports a
  different one (a rollback, or a deploy from elsewhere). Drift is *not* inferred here:
  drift is "someone changed the device after this source applied it", which only the
  sync's own comparison establishes.
- **Presence is part of the resource payload.** Each resource carries `device_online`,
  `device_last_seen`, `device_reported_at`, and `device_reported_version`. They are absent
  for an org resource (no device) and for a device the server could not read.
- **Absence means unknown, never offline.** A failed device read must not be rendered as a
  down device. This is the same rule §10 already applies to a failed provider check.
- **A nil reader fails closed.** With no confirmation reader wired, every applied resource
  records `pending` rather than assuming success.

### 21.3 Client

- A resource's state is shown next to its presence: online, offline (with last-seen), or
  reachability unknown, and the version the device reports.
- The presence line is its own component, per §7.4, and never an inline ternary.
