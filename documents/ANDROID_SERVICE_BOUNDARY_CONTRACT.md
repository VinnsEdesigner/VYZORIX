# Android Service Boundary Contract

**Status:** the policy is implemented and tested in `apps/api/internal/domain/permission`.
The delivery and kernel halves are not — see §12.
**Parent:** `ANDROID_SERVICE_ARCHITECTURE.md` §9 (Security and privilege model).
**Supersedes:** the tag-proposal sketch that circulated as a single `TierDef` table with
role and path policy merged.

This document defines one thing: **who decides what, and where the boundary is
enforced.** It exists because the first sketch of the tag model put the server's
`user`/`orgID` and the device's `path`/`Landlock`/`Seccomp` in a single function, and a
single function cannot run on either side. That is a boundary bug, not a code bug.

---

## 1. The problem the model must not repeat

Authorization asks two different questions:

- **Whether** an operator may use a capability at all. This needs a role, an
  organization, and the resource they are acting on. It is a *server* question.
- **Which concrete resource** the already-authorized action reaches. On the device there
  is exactly one resource — the device itself. So the device's question is not a scope
  evaluation, it is a *binding* lookup: which PTY channel, which USB fd.

The first sketch collapsed both into `CanExecuteUnified(user, device, tags, path, orgID)`.
The device has no `user` or `orgID`; the server has no `path`. Merging them produced the
fail-open behaviour the sketch needed to paper over: an empty scope list that "allows",
a `tier:1` tag that asserts what the device already knows, and a seccomp block that
listed `*` in `Allowed` *and* `mount` in `Blocked`.

**Rule: the server decides whether. The device enforces what was already decided.**

---

## 2. The three planes

| Plane | Owns | Never sees |
|:--|:--|:--|
| **Server** (`domain/permission`) | role → capability, resource scope, tier intersection, token minting | paths, fds, kernel policy |
| **Token** (issued once per session) | identity, resolved `apiAllowlist`, tier, expiry | roles, scopes |
| **Device** (bridge + launcher) | `api ∈ allowlist`, binding checks, closed-world binds | roles, orgs, tags |

The device never evaluates a tag and never learns a role. If it did, a forged token
could carry a role and grant itself everything; the token must be the only truth
because the token is the cryptographically-bound artefact.

---

## 3. The tag vocabulary

A tag is a `dimension:value` string. Tags carry **capability** dimensions only — the
*resource* axis stays in the scoped engine (§6), so there is one source of truth for it.

| Dimension | Values | Meaning |
|:--|:--|:--|
| `data` | `public`, `app`, `hardware`, `private` | data sensitivity |
| `op` | `read`, `write` | operation |
| `protection` | `normal`, `dangerous`, `signature` | Android permission class |
| `visibility` | `public`, `private` | who may see the result |
| `hw` | `usb` | hardware touched — extended when the camera/location/mic APIs land |
| `fs` | `app`, `sdcard`, `usb`, `tmp`, `system`, `engine` | filesystem zone |

Minimum tier is a field on the catalog entry (`MinTier`), not a tag: it is read
from the device record and compared, so it is never something a role is granted.

A tag must be required by at least one catalog entry or path policy. A tag nothing
names is inert: it makes a role appear to hold a capability no check consults. The
`hw` dimension therefore carries only `usb` today; camera/location/mic tags are added
**with** their APIs, not before. `TestNoInertTags` enforces this.

`fs:engine` is the app's own critical zone. It appears in **no** role grant and **no**
tier path policy, so it is unaddressable rather than merely un-granted (§10).

Tiers name a device's **state**, not a role and not an API class. They are read from the
device record and never asserted by a caller:

| Tier | Name | State |
|:--|:--|:--|
| 1 | unrooted | stock device: app-private storage, `UsbManager`-path USB |
| 2 | rooted | adds direct device-node access (`/dev/bus/usb`) and RW external storage |
| 3 | advanced | unrestricted: full filesystem and device access |

---

## 4. The predicate

One predicate, three operators, evaluated **only on the server**:

```
type Requirement struct { All, Any, Deny []Tag }

Requirement.SatisfiedBy(grant TagSet) bool:
    every tag in All  ∈ grant
    && (Any is empty || at least one tag in Any ∈ grant)
    && no tag in Deny ∈ grant
```

`All` is conjunction, `Any` is the disjunction the flat-set sketch could not express
(`UsbApi` works on tier 1 *or* tier 2), and `Deny` is the explicit exclusion that makes
a boundary readable without reading the complement of a grant.

---

## 5. The catalog — the server's single source of truth

Each device API has exactly one entry, in the permission package:

```
type ApiRequirement struct {
    Action  Action      // resource action for the scoped engine (§6)
    Scope   string      // resource scope prefix for the scoped engine (§6)
    Tags    Requirement // capability requirement (§4)
    MinTier Tier        // lowest device tier that can serve it
}
```

This is the *only* place an API's requirements are written. The device's `@VyzorixApi`
annotation, if it exists, is documentation — it is not consulted, so it cannot drift
into authority.

---

## 6. Two axes, both must pass

The catalog entry carries a capability requirement **and** a resource action+scope. They
compose; neither implies the other:

```
capability  = catalog[api].Tags.SatisfiedBy(roleTags(role))
resource    = scopedEvaluator(role, grants).Grants(catalog[api].Action, scopeOf(device))
allowed     = capability && resource && device.tier >= catalog[api].MinTier
```

The resource axis reuses the existing scoped-RBAC engine (`scoped.go`, `evaluator.go`)
unchanged. Tags are the *new* axis; they do not replace it. This is why the model needs
no rewrite of the seven issues `AUTHORIZATION_ANALYSIS.md` closed.

---

## 7. Resolution, and the fail-open fix

The server resolves a session's allowlist once, at open:

```
apiAllowlist(role, tier) =
    { api : catalog[api].Tags.SatisfiedBy(DefaultTagsForRole(role))
            && tier >= catalog[api].MinTier }
```

Two properties are load-bearing:

1. **Deny by default.** An empty requirement is satisfied by an empty grant; an unknown
   API is not in the catalog and therefore not in the allowlist. There is no
   "empty means everything" case — that is the single most dangerous line the device
   bridge currently contains (`SessionToken.permits`), and this model forbids it.
2. **Tier is read, never asserted.** Tier comes from the device record. An unrooted
   device can never receive a root-only API in its allowlist, because resolution
   intersects on `MinTier`.

---

## 8. The token

```
SessionToken {
    sessionId, deviceId, operatorId
    apiAllowlist []Action   // resolved by §7
    tier         Tier
    expiresAt    int64
}
```

No role. No scope. No tags. The token is the capability, per architecture §9.1 — for
anything with an fd (PTY, USB), the fd is the real revocable grant and closing it
revokes access. The allowlist is a filter on top of the capability, not the capability.

---

## 9. The device check

The device's entire authorization surface is one pure, total function of the token:

```
api ∈ token.apiAllowlist
&& token.deviceId == self
&& !expired
&& (for PTY frames) channel ∈ channelsOwnedBy(token.sessionId)
```

The last clause is the *binding* dimension: it is the device's real resource axis, and
it is set membership against a fact the device minted itself, not a scope the server
sent. `pty_output` is already scoped this way; `pty_input`, `session.close` and
`session.resize` are not, and must be.

---

## 10. The filesystem boundary

**This lives on the device, not here.** The server has no filesystem, so it cannot
enforce a bind rule; a server-side path model would be policy with nothing to apply
to. An earlier revision of this contract carried one (`PathPolicy`, `PathBindable`,
`fs:*` tags) and it was removed — it had no consumer, and it had drifted: it listed
`rootfs`, `overlay` and `android.sock` as a zone to protect, when the rootfs is the
container's own root (`-r rootfs`) and the socket is the door §2.2 exists to open.

Bind rules are owned by `ProotLauncher` in the Android repo, in one place:

1. **Closed world.** `ProotLauncher.binds()` names every path that may be bound:
   `/dev`, `/proc`, `/sys`, the container's `resolv.conf`, the overlay's `usr-local`
   and `opt` subdirectories, and the socket. Nothing else is bindable.
2. **Engine-zone assertion.** `ProotLauncher.isEngineZonePath()` refuses any path
   under the secret store or the recordings, as an independent second check. Two
   guarantees, so one mistake does not open the zone.

What must never be bound is anything holding a secret or a recording. The rootfs and
the socket are deliberately *not* in that set, for the reasons above.

---

## 11. Honest limits

The kernel features are depth, not the floor. Stated plainly so the model is not
believed to be stronger than it is:

- **Landlock is opportunistic.** Kernel 5.13+; the C22's Android 13 kernel may not have
  it. The policy invariant (§10) is the primary control.
- **Seccomp cannot block `ptrace`.** proot *is* a ptrace tracer, so the filter is a
  denylist (`mount`, `init_module`, `bpf`, `kexec_load`) and a denylist is weaker than
  an allowlist. It cannot be both — the sketch's `Allowed: ["*"]` + `Blocked: [...]`
  was not a policy.
- **SELinux domains per tier require policy you do not control** on a `user` build. In
  practice the tiers collapse to one domain and differentiation rests on binds,
  seccomp, and the token.

---

## 12. Not yet wired (state, not intent)

Read this before treating any section above as live. The policy is complete; three
things are not connected, and each is named so it is not mistaken for done.

### 12.1 The allowlist is resolved but never delivered

`ResolveSessionCapability` runs at PTY open, and the result is recorded on the
session (`PTYSession.SetCapability`). Nothing sends it to the device:
`PTYSession.Capability()` has no reader.

The device therefore still mints its own allowlist from its local API list
(`router.names()`), i.e. every API it serves. The delivery path is the device's
cloud WebSocket, which is Build 4. Until then the server-side decision is made,
audited and observable, but not enforced on the device.

### 12.2 Device identity is unset, and expiry is unset

The §9 clause `token.deviceId == self` needs the device to know its own ID, which
arrives with registration (Build 3). The check is implemented and skipped while the
ID is null, with a warning logged once at bridge start so the gap is visible rather
than assumed.

`expiresAt` is passed as `0` at the one call site, so a resolved capability does not
expire. It becomes meaningful when the token is delivered (12.1).

### 12.3 Kernel features do not exist

No Landlock, no seccomp filter, no per-tier SELinux domain is implemented on the
device. §11 describes them as depth rather than the floor, and that is the honest
reading: the controls that exist today are the token check, the closed-world binds,
and the fd as the revocable capability. The kernel features are a later layer.
