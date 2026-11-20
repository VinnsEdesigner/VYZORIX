# Users & Service Accounts Pages — Design Analysis (Grafana reference → VYZORIX)

> **Version:** 1.0
> **Status:** Design analysis / pre-implementation
> **Created:** 2026-11-18
> **Scope:** Analyse Grafana's *Users* and *Service accounts* administration UI
> (fields, options, actions, layout, states) and map it onto the VYZORIX console
> design language and the VYZORIX server API.
> **Reference repos:** `VYZORIX/` (product) and `grafana-full/` (design source).
> **Architecture:** Layered — UI → Hooks → Domain → API (flow inward only).

---

## Table of Contents

1. [Why Grafana, and why not](#1-why-grafana-and-why-not)
2. [Grafana reference — Users](#2-grafana-reference--users)
3. [Grafana reference — Service accounts](#3-grafana-reference--service-accounts)
4. [VYZORIX design contract (the real target)](#4-vyzorix-design-contract-the-real-target)
5. [VYZORIX API surface (what actually exists)](#5-vyzorix-api-surface-what-actually-exists)
6. [Field-by-field mapping](#6-field-by-field-mapping)
7. [Proposed VYZORIX page designs](#7-proposed-vyzorix-page-designs)
8. [Backend gaps blocking the full design](#8-backend-gaps-blocking-the-full-design)
9. [Port plan & implementation order](#9-port-plan--implementation-order)

---

## 1. Why Grafana, and why not

Grafana is the reference for **information architecture and field/option
inventory**, not for pixels. Its two pages answer the same product questions
VYZORIX has:

- Who can log in? (users)
- What non-human identities exist? (service accounts)
- What is each one allowed to do? (roles/scopes)
- How do I revoke access? (disable / delete / revoke sessions / revoke tokens)

What we take: the **columns, the sections of a detail page, the action set, the
empty/disabled states, and the safety affordances** (confirm modals, "show the
secret once", lock icons for externally-managed rows).

What we do **not** take: Grafana's `filter-table` chrome, `Tag colorIndex`
colours, its blue `theme.v1.palette.blue95` links, or its Redux/`contextSrv`
permission plumbing. VYZORIX has its own token system
(`packages/ui/src/theme`), its own component library (`@vyzorix/ui`), and its
own auth store (`useAuthStore`) + TanStack Query data layer.

The VYZORIX visual contract is **Grafana's structure plus VYZORIX's own built UI**.
Grafana supplies the page structure, the field/option/action inventory and the
interaction patterns (`filter-table`, `.page-action-bar`, `.page-heading`,
inline Edit→Save rows). The visual language — tokens, spacing, badge treatment,
component behaviour — comes from what VYZORIX has already built:
`packages/ui/src/theme` is the token source of truth, and the pages under
`ui/pages/` (profile, login, admin-users, admin-service-accounts) are the house
style. Where Grafana and the built UI differ, **the built UI wins**.

`web/mocks/console/` is **not** a reference. It was the original visual mock and
is now superseded; its docs are stale and its chrome has been replaced.

---

## 2. Grafana reference — Users

### 2.1 Two distinct user pages (important distinction)

Grafana splits users into **Server Admin** and **Org** scopes. VYZORIX's console
is org-scoped, so it maps to the **Org** variant, but the Server Admin variant
carries fields worth knowing about.

| Concern | Server Admin (`/admin/users`) | Org (`/org/users`) |
|---|---|---|
| File | `public/app/features/admin/UserListAdminPage.tsx`, `Users/UsersTable.tsx` | `features/users/UsersListPage.tsx`, `admin/Users/OrgUsersTable.tsx` |
| Scope | All users on the instance | Users in one org |
| "Belongs to" column | yes (`OrgUnits` + shield for Grafana Admin) | no |
| Licensed role column | conditional | no |
| Role editable inline | no (edit page) | yes (`OrgRolePicker` / `UserRolePicker`) |
| Invites tab | no | yes (`Pending Invites (n)` radio) |
| Create user | yes (`admin/users/create`) | invite flow |

### 2.2 Users list — columns, in order

From `UsersTable.tsx` (server admin) and `OrgUsersTable.tsx` (org):

| # | Column | Source | Notes |
|---|---|---|---|
| 1 | avatar | `avatarUrl` | `Avatar` circle, empty header |
| 2 | **Login** | `login` | server admin: link to edit page; org: plain text |
| 3 | **Email** | `email` | |
| 4 | **Name** | `name` | |
| 5 | Belongs to | `orgs[]` | only when `user.orgs` present; 1 org → name, >1 → count + tooltip; shield icon if Grafana Admin |
| 6 | Licensed role | `licensedRole` | only when present; `'None'` → "Not assigned" + tooltip |
| 7 | **Last active** | `lastSeenAtAge` | `Never` when `lastSeenAt < created`; header tooltip "Time since user was seen using Grafana" |
| 8 | **Origin** | `authLabels[0]` | first auth label only (`TagBadge`) |
| 9 | Provisioned | `isProvisioned` | `Tag` "Provisioned" |
| 10 | *(disabled)* | `isDisabled` | `Tag` "Disabled", empty header |
| 11 | *(edit)* | — | `LinkButton` pen icon → `/admin/users/edit/:uid` |

Sortable: login, email, name, lastSeenAt.

### 2.3 Users list — toolbar / options

`UserListAdminPage.tsx`:

- **Search** `FilterInput`, placeholder *"Search user by login, email, or name."*, autofocus.
- **RadioButtonGroup** filter: `All users` | `Active last 30 days`
  (filter name `activeLast30Days`, boolean).
- Extension slot `extraFilters` (empty by default).
- **"New user"** `LinkButton` (primary), gated on `AccessControlAction.UsersCreate`.
- Empty state: `EmptyState` "No users found" (`variant="not-found"`).

Org variant (`UsersActionBar.tsx`): search + `Users | Pending Invites (n)` radio
(when invites > 0) + **Invite** link + optional external-management link button.

### 2.4 User detail page — sections

`UserAdminPage.tsx` composes four sections in a vertical stack, each gated by
permission:

**a) User information** (`UserProfile.tsx`) — `filter-table`, each row has an
inline `ConfirmButton` Edit→Save (or is locked):

| Field | Editable | Locked when |
|---|---|---|
| Numerical identifier | no (always locked) | — |
| Name | yes | `isExternal \|\| isProvisioned \|\| !UsersWrite` |
| Email | yes | same |
| Username | yes | same |
| Password | yes (`type=password`, blanked on edit) | `!UsersPasswordUpdate` |

Locked rows show italic *"Synced via {authSource}"* (LDAP / SCIM).

Actions: **Delete user** (destructive, `ConfirmModal`), **Disable user**
(secondary + confirm), or **Enable user** when disabled. All permission-gated.

**b) Permissions** (`UserPermissions.tsx`) — single row: **Grafana Admin** =
`RadioButtonGroup` Yes/No, rendered as shield + "Yes" when read-only; a
`ConfirmButton` "Change" gates editing. Locked for external users.

**c) Organizations** (`UserOrgs.tsx`) — table of org rows: org name, role
(`UserRolePicker`/`OrgRolePicker`), **Remove from organization**
(`ConfirmButton`, destructive). Below: **Add user to organization** button →
modal with `OrgPicker` + role picker.

**d) Sessions** (`UserSessions.tsx`) — table columns: **Last seen** (or "Now"
when active), **Logged on**, **IP address**, **Browser and OS**, **Identity
Provider** (`authModule` badge), **Force logout** per row (`ConfirmButton`,
destructive). Below: **Force logout from all devices** (secondary + confirm).
Gated on `UsersAuthTokenList` / `UsersLogout`.

**e) LDAP sync** (`UserLdapSyncInfo.tsx`) — only for LDAP users with the
`ldapsync` feature toggle. Skippable for VYZORIX.

### 2.5 Users — role model

- Basic org roles: `Admin | Editor | Viewer` (`OrgRole`); `None` excluded from
  the picker.
- Optional **RBAC roles** layered on top (`UserRolePicker`), enabled by
  `licensedAccessControlEnabled()`.
- Grafana Admin is a separate **server-admin boolean**, not an org role.

### 2.6 Users — safety affordances worth copying

- Destructive actions (delete, disable, force logout) are **always** behind a
  `ConfirmModal`.
- Edit-in-place per row with explicit Save/Cancel (`ConfirmButton`).
- A lock/italic message rather than hiding a field when the source of truth is
  external (LDAP/SCIM).
- "Never" is computed, not stored (`lastSeenAt < created`).

---

## 3. Grafana reference — Service accounts

### 3.1 Service accounts list

`ServiceAccountsListPage.tsx` + `ServiceAccountTable.tsx`.

**Page chrome:** `Page` with `navId="serviceaccounts"`, a **subtitle with a docs
link**, and an **"Add service account"** primary button in the page actions.

**Toolbar:** search `FilterInput` (placeholder *"Search service account by
name"*, maxWidth 50) + **RadioButtonGroup** state filter:
`All | With expired tokens | Disabled` (+ `Managed` when the
`externalServiceAccounts` toggle is on).

**Empty states (two different ones):**

- Nothing created yet → `EmptyState variant="call-to-action"`, message
  *"You haven't created any service accounts yet"*, body about scoped API
  permissions, and a large **Add service account** button (icon
  `key-skeleton-alt`).
- Created but filtered to zero → `EmptyState variant="not-found"`,
  *"No service accounts found"*.

**Table columns, in order:**

| # | Column | Cell |
|---|---|---|
| 1 | *(avatar)* | `Avatar`, links to detail |
| 2 | **Account** | name, `TextLink` → `/org/serviceaccounts/:uid` |
| 3 | **ID** | `login`, `TextLink` secondary |
| 4 | **Roles** | `UserRolePicker` (licensed) or `OrgRolePicker`; disabled when external/disabled |
| 5 | **Tokens** | `key-skeleton-alt` icon + count (`"No tokens"` when 0), link |
| 6 | *(actions)* | see below |

**Row actions:**

- **Add token** (secondary, `plus`) — only when `!tokens && !isDisabled`.
- **Enable** (`check`) / **Disable** (`ban`) — secondary, permission-gated.
- **Delete** (secondary, `trash-alt`, icon-only, aria-label) — permission-gated.
- External ("managed") accounts: everything replaced by a single disabled
  **lock** `IconButton` with tooltip *"This is a managed service account and
  cannot be modified"*.

**Modals:** delete (`ConfirmModal`, body names the account and its token count),
disable (`ConfirmModal`, body names the account), and `CreateTokenModal`.

**Skeleton loading:** each cell renders a `Skeleton` (circle for avatar) while
`isLoading`.

### 3.2 Service account detail page

`ServiceAccountPage.tsx` + `ServiceAccountProfile.tsx`.

**Page nav:** account name + avatar, subtitle *"Manage settings for an
individual service account."*

**Top-right action stack:** **Delete service account** (destructive) and
**Disable**/**Enable** (secondary). External accounts → single disabled lock
icon.

**a) Information** table (`ServiceAccountProfile.tsx`):

| Field | Editable | Notes |
|---|---|---|
| Numerical identifier | no | hidden when falsy |
| **Name** | yes (inline Save/Cancel) | disabled when `!write \|\| isDisabled` |
| **ID** | no | the `login` value |
| **Roles** | picker | `ServiceAccountRoleRow` |
| **Creation date** | no | timezone-formatted |
| Used by | no | external accounts only (`requiredBy` → plugin link) |

**b) Tokens** — section header with a right-aligned **Add service account
token** button. Table (`ServiceAccountTokensTable.tsx`):

| Column | Cell |
|---|---|
| **Name** | token name |
| **Expires** | `Never` / `Expires in Nd` (warning colour) / `Expired` (error colour + tooltip) |
| **Created** | formatted date |
| **Last used at** | formatted date, or `Never` |
| *(status)* | `Revoked` badge (error colour) + tooltip "publicly exposed, please rotate" |
| *(delete)* | `DeleteButton` per row, permission-gated |

Row text is dimmed (`text.secondary`) when expired or revoked.

**c) Permissions** (`ServiceAccountPermissions.tsx`) — the RBAC
`Permissions` component (add/remove fine-grained permissions). Only with
licensed access control. Out of scope for VYZORIX v1.

### 3.3 Create service account page

`ServiceAccountCreatePage.tsx` — a short form:

| Field | Required | Notes |
|---|---|---|
| **Display name** | yes | autofocus; error "Display name is required" |
| **Role** | no | `UserRolePicker` (licensed) or `OrgRolePicker`; default `Viewer` (or `None` when licensed) |

Submit → **Create**. On success: push to `/org/serviceaccounts/:uid`.

### 3.4 Create token modal — the important flow

`components/CreateTokenModal.tsx`. Two modes in one modal:

**Mode 1 — form:**

| Field | Notes |
|---|---|
| **Display name** | required, `description` "Name to easily identify the token", placeholder = auto-generated `${login}-${uuid}` |
| **Expiration** | `RadioButtonGroup`: `No expiration` \| `Set expiration date`. "No expiration" is **disabled** when a server token-expiry limit is configured |
| **Expiration date** | shown only for the custom option; `DatePickerWithInput`, min = tomorrow, max = tomorrow + `tokenExpirationDayLimit` |

Primary action **Generate token** (disabled when custom date invalid).

**Mode 2 — created:** read-only token `Input` + **Copy to clipboard**, and
**Copy to clipboard and close** / **Close**. Description warns: *"Copy the token
now as you will not be able to see it again."*

### 3.5 Service accounts — data shape (`ServiceAccountDTO`)

`id, uid, orgId, tokens (count), name, login, avatarUrl?, createdAt,
isDisabled, isExternal?, requiredBy?, teams[], role (OrgRole), roles? (RBAC)`.

---

## 4. VYZORIX design contract (the real target)

> **SUPERSEDED (2026-11-18).** The visual contract is now **Grafana's**, not the
> console mock. See §11. The mock is no longer a reference for these pages.
> The only thing carried over is the **neutral badge** rule, and even that is
> implemented as Grafana's plain gray text `Tag` — **no pill-with-dot**.

## 5. VYZORIX API surface (what actually exists)

### 5.1 Users (admin)

Go handler `apps/api/internal/api/handlers/auth/auth_admin.go`, mounted under
`/v1/auth/admin/users` (`auth_routes.go:207-211`). All require `super_admin`
**in the current org** (`op.IsSuperAdminIn(orgID)`).

| Method | Path | Notes |
|---|---|---|
| GET | `/v1/auth/admin/users` | **hardcoded `limit=20, offset=0`** — no pagination params, no search/filter/sort |
| POST | `/v1/auth/admin/users` | `RegisterRequest` (`email`, `name`, `password`, `role`) |
| GET | `/v1/auth/admin/users/:id` | |
| PATCH | `/v1/auth/admin/users/:id` | `UpdateUserRequest` (`email`, `name`, `role`, `mfa_enabled`, `email_verified`) |
| DELETE | `/v1/auth/admin/users/:id` | refuses self-delete and last super-admin |

Wire types (`packages/API_Client/src/generated/vyzorixUpdateServerAPI.schemas.ts`):

```ts
AdminUser            { id, email, name, role, mfa_enabled, email_verified, created_at, updated_at }
AdminUserListResult  { users: AdminUser[], total }
CreateUserRequest    { email, name, password, role }
UpdateUserRequest    { email, name, role, mfa_enabled, email_verified }
```

Generated hooks live in `apps/VyzoriX_web/src/generated-rq/admin/admin-operations.ts`
(`useGetAuthAdminUsers`, `getAuthAdminUsers`, … `getGetAuthAdminUsersUrl`).
**No hand-written hook layer exists yet** (no `hooks/admin/`).

**Missing vs Grafana:** avatar, login/username (VYZORIX uses `email` as the
identity — see §8), org membership list per user, last-active, auth origin
labels, provisioned flag, sessions-per-user, password-set endpoint, disable
(soft) endpoint, pagination/search/sort.

### 5.2 Users (org members + invitations)

| Method | Path | Handler |
|---|---|---|
| GET | `/v1/organizations/:id/members` | `memberHandler.List` |
| GET | `/v1/organizations/:id/members/:memberId` | |
| PATCH | `/v1/organizations/:id/members/:memberId` | role update |
| DELETE | `/v1/organizations/:id/members/:memberId` | remove |
| POST | `/v1/organizations/:id/members/:memberId/suspend` | |
| POST | `/v1/organizations/:id/members/:memberId/reinstate` | |
| POST | `/v1/organizations/:id/members/:memberId/transfer` | ownership transfer |
| GET | `/v1/organizations/:id/invitations` | `invitationHandler.ListByOrganization` |
| POST | `/v1/invitations` | create invite |
| GET | `/v1/invitations` | list by inviter |
| DELETE | `/v1/invitations/:id` | revoke invite |

`OrganizationMember` wire shape:

```ts
{ id, organization_id, user_id, role, status, user_name, user_email,
  invited_by, joined_at, removed_at }
```

`Invitation` wire shape:

```ts
{ id, organization_id, organization_name, email, role, status,
  invited_at, invited_by, inviter_name, expires_at, responded_at,
  responder_id, token }
```

Member `status` = lifecycle `active | suspended | removed`; invitation `status`
= `pending | approved | rejected | expired`. Roles:
`super_admin | admin | operator | viewer` (`OrganizationRole`, with
`RoleOperator` deliberately kept — see project memory).

Client: `generated-rq/members/organization-members.ts` and
`generated-rq/invitations/organization-invitations.ts` are **fully generated**
(list/get/remove/role/suspend/reinstate/transfer; list/create/delete invite).

### 5.3 Service accounts

Handler `apps/api/internal/api/handlers/serviceaccount/vyzorix_service_account_handler.go`,
mounted at `/v1/service-accounts` behind org-context + membership middleware.

| Method | Path | Response |
|---|---|---|
| GET | `/v1/service-accounts` | `{ service_accounts: ServiceAccount[] }` |
| POST | `/v1/service-accounts` | `ServiceAccount` (201) — body `{ name }` |
| DELETE | `/v1/service-accounts/:id` | `{ deleted: true }` |
| GET | `/v1/service-accounts/:id/tokens` | `{ tokens: ServiceAccountToken[] }` |
| POST | `/v1/service-accounts/:id/tokens` | `{ secret }` (201, **shown once**) |
| DELETE | `/v1/service-accounts/:id/tokens/:token_id` | `{ revoked: true }` |
| POST | `/v1/service-accounts/:id/tokens/:token_id/rotate` | `{ secret }` |

Wire types:

```ts
ServiceAccount        { id, org_id, name, enabled, created_at, updated_at }
ServiceAccountToken   { id, service_id, name, key_prefix, scopes[],
                        valid, created_at, expires_at?, revoked_at? }
CreateServiceAccountRequest      { name }
CreateServiceAccountTokenRequest { service_id, name, scopes[], expires_at }
RotateServiceAccountTokenRequest { name, scopes[], expires_at }
```

Scopes: `read | write | admin` (`serviceaccount.Scope`).

> **Careful:** the TS schema declares `expires_at` and `revoked_at`, but the Go
> handler **never populates them**, so they always arrive `undefined` on the
> wire. The type lies. Treat "Expires" / "Revoked" as unavailable until the
> handler is fixed (§8.1.2). There is no `last_used_at` in the schema at all.

**Bugs / mismatches found in the generated client:**

1. **Rotate path is wrong.** The client emits
   `POST /service-accounts/${id}/rotate`, but the server route is
   `/service-accounts/:id/tokens/:token_id/rotate`. The client must be
   regenerated (or hand-corrected) before rotate can work.
2. **Token fields are declared but never populated.** The server domain `Token`
   has `ExpiresAt`, `RevokedAt`, `LastUsedAt`, but `tokenJSON` only emits
   `id, service_id, name, key_prefix, scopes, valid, created_at`. So
   `expires_at`/`revoked_at` are always absent at runtime, and `last_used_at`
   is not even in the schema. The tokens table's "Expires / Last used /
   Revoked" columns are therefore **not renderable** until the handler maps
   those fields.
3. **No enable/disable endpoint.** The domain has `Enabled`, but there is no
   `PATCH /service-accounts/:id` — so Grafana's Enable/Disable row actions have
   **no backend** in VYZORIX.
4. **No update/rename endpoint.** Grafana's editable Name row needs
   `PATCH /service-accounts/:id`.
5. **No per-account token count in list.** Grafana shows a token count column;
   VYZORIX's list returns no count, so it requires an N+1 fetch or a server
   addition.
6. **No pagination/search** on the list (returns everything for the org).

---

## 6. Field-by-field mapping

Legend: **✅ maps** · **⚠️ partial** · **❌ no backend** · **🚫 drop** (not
applicable to VYZORIX).

### 6.1 Users list

| Grafana column | VYZORIX source | Verdict |
|---|---|---|
| avatar | — (VYZORIX avatar exists on `MeResult.avatar_url`, not on `AdminUser`) | ❌ |
| Login | `AdminUser.email` (email is the identity) | ⚠️ relabel to **Email** |
| Email | `AdminUser.email` | ✅ |
| Name | `AdminUser.name` | ✅ |
| Belongs to | — (admin list is not org-filtered; members endpoint has org) | ❌ |
| Licensed role | — (VYZORIX has no licensed RBAC roles) | 🚫 |
| Last active | — (`AdminUser` has no `last_seen_at`) | ❌ |
| Origin | — (no auth-label source) | ❌ |
| Provisioned | — (no provisioned flag) | ❌ |
| Disabled | — (no `is_disabled` on `AdminUser`) | ❌ |
| *(edit link)* | route `/admin/users/$userId` | ✅ (new route) |

**Mappable today:** Email, Name, Role, MFA, Email verified, Created, Updated,
edit link. Everything else needs backend work (§8).

### 6.2 Users list toolbar

| Grafana control | VYZORIX | Verdict |
|---|---|---|
| Search by login/email/name | needs server `q` param | ❌ (client-side only over 20 rows for now) |
| All / Active last 30 days | needs `last_seen_at` | ❌ |
| New user button | `POST /auth/admin/users` | ✅ |
| Empty state | `EmptyState` | ✅ |

### 6.3 User detail

| Grafana section/field | VYZORIX source | Verdict |
|---|---|---|
| Numerical identifier | `AdminUser.id` | ✅ |
| Name (editable) | `PATCH` `name` | ✅ |
| Email (editable) | `PATCH` `email` | ✅ |
| Username (editable) | — (no separate login) | 🚫 |
| Password (set) | — (no admin password-set endpoint) | ❌ |
| Grafana Admin yes/no | `AdminUser.role === 'super_admin'` (org role) | ⚠️ reinterpret |
| MFA enabled | `AdminUser.mfa_enabled` | ✅ (read + `PATCH`) |
| Email verified | `AdminUser.email_verified` | ✅ (read + `PATCH`) |
| Organizations list | members endpoint (per org, not per user) | ⚠️ needs join |
| Sessions + force logout | `/auth/sessions` are **self**-scoped only | ❌ (no admin session listing) |
| Delete user | `DELETE` | ✅ |
| Disable / Enable user | — | ❌ |
| LDAP sync info | — | 🚫 |

### 6.4 Service accounts list

| Grafana column | VYZORIX source | Verdict |
|---|---|---|
| avatar | — | ❌ |
| Account (name) | `ServiceAccount.name` | ✅ |
| ID | `ServiceAccount.id` | ✅ |
| Roles | — (VYZORIX SA has no role; it has token **scopes**) | ⚠️ replace with **Scopes** (derived from tokens) or drop |
| Tokens (count) | — (no count in list) | ❌ |
| Actions: Add token | `POST …/tokens` | ✅ |
| Actions: Enable/Disable | — | ❌ |
| Actions: Delete | `DELETE …/:id` | ✅ |
| Managed/external lock | — | 🚫 |
| State filter (All/expired/disabled/managed) | needs token expiry + enabled in list | ❌ |
| Search by name | client-side over full list | ✅ (client-side) |
| Empty state (CTA) | `EmptyState` | ✅ |
| Skeleton loading | `LoadingPlaceholder` | ✅ |

### 6.5 Service account detail

| Grafana section/field | VYZORIX source | Verdict |
|---|---|---|
| Numerical identifier | `ServiceAccount.id` | ✅ |
| Name (editable) | — (no PATCH) | ❌ |
| ID | `ServiceAccount.id` | ✅ |
| Roles | — | 🚫 |
| Creation date | `ServiceAccount.created_at` | ✅ |
| Enabled state | `ServiceAccount.enabled` | ✅ (read only) |
| Tokens table: Name | `ServiceAccountToken.name` | ✅ |
| Tokens table: Expires | `expires_at` declared but never populated | ❌ |
| Tokens table: Created | `ServiceAccountToken.created_at` | ✅ |
| Tokens table: Last used | — (not in the schema) | ❌ |
| Tokens table: Revoked badge | `revoked_at` declared but never populated | ❌ |
| Tokens table: Delete/Revoke | `DELETE …/tokens/:token_id` | ✅ |
| Add token → modal (name, expiration) | `POST …/tokens` with `name`, `scopes`, `expires_at` | ✅ |
| Create token "shown once" copy flow | `{ secret }` | ✅ |
| Rotate token | endpoint exists, **client path is wrong** | ⚠️ fix client |
| Permissions (RBAC) | — | 🚫 |

---

## 7. Proposed VYZORIX page designs

> **SUPERSEDED (2026-11-18).** This section was written before implementation and
> described porting the console mock's chrome. The mock is no longer the
> reference: the pages were built against Grafana's structure and VYZORIX's own
> tokens (§11), and the Users pages are being revised again (invite-only, no
> create-user button, no email/MFA/password editing). Treat §11 and the live
> `ui/pages/` code as the record; keep this section only for the reasoning about
> which Grafana fields have no backend.

Both pages keep the mock's chrome exactly and fill the table with what the API
can supply **now**, marking the rest as follow-ups. Where Grafana's field has no
backend, we **omit the column** rather than show an empty one.

### 7.1 `/admin/users` — Users list (v1)

```
Page title="Users" subTitle="Everyone in this organisation"
  actions: <Button variant="primary" icon="user-plus">New user</Button>   ← super_admin only

Toolbar (mock .log-toolbar):
  [search input: "Search by email or name…"]        (client-side, 20-row window)
  spacer
  <Text muted>"{n} users"</Text>

Table (mock .table):
  Email            Name          Role          MFA      Verified   Created        (actions)
  ada@acme.io      Ada Lovelace  <Badge>Admin</Badge>  Yes      Yes       2026-03-02     Edit
  ...

Empty: <EmptyState icon="users" message="No users yet" />
```

**Column set (v1):** Email · Name · Role · MFA · Email verified · Created ·
Edit. Role rendered as a **neutral `Badge`** (mock rule: state by word/dot, not
colour). MFA/Verified rendered as words, not coloured fills.

**Follow-up columns once the server exposes them:** Last active, Origin,
Provisioned, Disabled, avatar, "Belongs to".

### 7.2 `/admin/users/$userId` — User detail (v1)

```
Page title="{name}" subTitle="Manage settings for this user"   (breadcrumb: Users / {name})

Section "User information"   (Card, meta rows w/ inline edit)
  Numerical identifier   <id>                         (locked)
  Name                   [Ada Lovelace]  Edit→Save     (PATCH name)
  Email                  [ada@acme.io]   Edit→Save     (PATCH email)
  Email verified         [Yes|No]        Edit→Save     (PATCH email_verified)
  Multi-factor auth      [On|Off]        Edit→Save     (PATCH mfa_enabled)
  Role                   <Badge>{role}</Badge>
  Created / Updated      formatted dates

Section "Danger zone"    (Card)
  [Delete user]  destructive → ConfirmModal "Are you sure you want to delete {name}?"
```

Permission rule: the whole page requires `super_admin` in the current org;
delete is additionally blocked for self and for the last super-admin (server
enforces both — surface the server's message verbatim).

**Follow-ups:** Password set, Disable/Enable, Organizations membership,
Sessions + force logout, Last active / Origin / Provisioned.

### 7.3 `/admin/service-accounts` — Service accounts list (v1)

```
Page title="Service accounts" subTitle="Non-human identities for automation"
  actions: <Button variant="primary" icon="key">Add service account</Button>

Toolbar:
  [search input: "Search service accounts…"]   (client-side)
  spacer
  <Text muted>"{n} service accounts"</Text>

Table:
  Account                         ID            Enabled   Created        (actions)
  ci-deploy                       8f2a…c1       Yes       2026-06-01     Add token · Delete
  <div muted>created 2026-06-01</div>
```

**Column set (v1):** Account (name + created sub-line, matching the mock) · ID
(truncated, title=full) · Enabled (word/dot badge) · Created · actions.

**Actions (v1):** **Add token** (opens the create-token modal — this is the
primary flow) and **Delete** (ConfirmModal naming the account). *No Enable/Disable
until the PATCH endpoint exists.*

**"New service account" flow:** a small `Modal` (or inline panel) with one
`Field` "Display name" (required, autofocus) → `POST /service-accounts` → then
immediately offer the create-token modal (mirrors the real-world need: an account
with no token is useless). Grafana instead navigates to the detail page.

### 7.4 `/admin/service-accounts/$id` — Service account detail (v1)

```
Page title="{name}" subTitle="Manage settings for this service account"

Section "Information"  (Card, meta rows)
  Numerical identifier   <id>            (locked)
  Name                   <name>          (locked until PATCH exists)
  Enabled                Yes | No        (locked)
  Creation date          formatted

Section "Tokens"  (header row: h3 "Tokens" + right-aligned [Add service account token])
  Table: Name · Scopes (neutral badges) · Created · (actions: Revoke)
    ci-nightly   read write   2026-06-01   Revoke

Section "Danger zone"
  [Delete service account] destructive → ConfirmModal
```

**Create-token modal (the key flow), v1:**

```
Title: "Add service account token"
Field  Display name       required, description "Name to easily identify the token",
                          placeholder = auto-generated "<name>-<uuid>"
Field  Scopes             multi-select from read | write | admin  (VYZORIX-specific)
Field  Expiration         RadioButtonGroup: No expiration | Set expiration date
                          (no day-limit config in VYZORIX → both options available)
Field  Expiration date    shown for custom; date input, min = tomorrow
Button [Generate token]
── after create ──
Field  Token              read-only Input + [Copy to clipboard]
       description        "Copy the token now — you will not be able to see it again."
Button [Copy to clipboard and close]   [Close]
```

**Scopes are the VYZORIX equivalent of Grafana's Roles column.** Grafana's
service accounts carry an org role; VYZORIX's carry per-token scopes
(`read`/`write`/`admin`). So the list's "Roles" column becomes a **Scopes**
column (or is dropped from the list and shown on the token rows), and the detail
page's "Roles" row is replaced by the token scopes.

**Rotate:** expose a **Rotate** action per token row only after the client path
is fixed (§8.1) — otherwise the call 404s.

---

## 8. Backend gaps blocking the full design

> **Status: implemented (2026-11-18).** Everything in §8.1 and §8.2 is now done on
> the server, and §8.3 is done too (per-user organizations, per-user sessions with
> force-logout). The sections below are kept as the rationale; see §9 for the
> as-built summary.

Ordered by how much design they unlock. Each is a small, self-contained server
change.

### 8.1 Blocking bugs (fix first — no design work needed)

1. **Rotate URL mismatch.** Client: `POST /service-accounts/:id/rotate`;
   server: `POST /service-accounts/:id/tokens/:token_id/rotate`. Regenerate the
   client (`pnpm orval` / the repo's codegen) or hand-correct
   `generated-rq/service-accounts/service-accounts-management.ts`. *Rotate is
   unusable until this is fixed.*
2. **Token fields dropped by `tokenJSON`.** Add `ExpiresAt`, `RevokedAt`,
   `LastUsedAt` to `openapi.ServiceAccountToken` and map them in `tokenJSON`.
   Unlocks the Expires / Last used / Revoked columns and the token state filter.
3. **No token count on the list.** Either add `token_count` to
   `openapi.ServiceAccount` (one `COUNT(*)` join) or accept an N+1 client fetch.
   Unlocks the Tokens column and the "Add token vs Rotate" row-action logic.

### 8.2 Small endpoints that unblock whole rows

4. **`PATCH /v1/service-accounts/:id`** — rename + `enabled`. Unlocks the
   editable Name row and the Enable/Disable actions (list + detail). The domain
   already models `Enabled`; only the service method + route + schema are new.
5. **`GET /v1/auth/admin/users` pagination + search + sort.** Add
   `page`/`limit`/`q`/`sort` (the handler currently hardcodes `limit=20, offset=0`).
   Unlocks server-side search, paging, and the "Active last 30 days" filter.
6. **`POST /v1/auth/admin/users/:id/password`** — admin password set. Unlocks
   the Password row. (Grafana has this; VYZORIX does not.)
7. **`POST /v1/auth/admin/users/:id/disable`** + `…/enable` (or a soft-delete
   flag). Unlocks the Disable/Enable actions and the Disabled column.

### 8.3 Larger features (design deferred)

8. **Per-user org memberships** — a join of `users` × `organizations` ×
   `members`. Unlocks the Organizations section on the user detail page.
9. **Admin session listing per user** — `/auth/sessions` is self-scoped today.
   Unlocks the Sessions section and force-logout (a genuinely useful admin tool,
   and VYZORIX already has the session plumbing on the profile page).
10. **Auth origin / provisioned / avatar on `AdminUser`** — cosmetic but cheap
    once the schema is being touched for the above.

### 8.4 Deliberately out of scope

- Grafana's licensed RBAC roles / `UserRolePicker` — VYZORIX's model is the four
  org roles plus token scopes. **Do not port the role picker.**
- LDAP sync panel, external/managed service accounts, "Belongs to" multi-org
  display — no VYZORIX equivalent.
- Grafana's `Tag colorIndex` colour badges — replaced by the mock's neutral
  badge + semantic dot.

---

## 9. Port plan & implementation order

> **COMPLETED (2026-11-18).** Phases 0–5 were all implemented and verified; this
> plan is kept as history, not as instructions. Two items below are now **wrong**
> by later decision: Phase 2's "use the mock's `.table`/`.logs-search`/`.seg`" —
> the styling came from Grafana's `filter-table` instead — and Phase 3's
> "wire New user → a Modal form" — the Users pages are invite-only and have no
> create-user path. See §11 and the live `ui/pages/` code.

Follows the repo's layering: **UI → Hooks → Domain → API**.

### Phase 0 — unblock (server + client)
1. Fix the rotate URL (§8.1.1).
2. Emit token `expires_at` / `revoked_at` / `last_used_at` (§8.1.2).
3. Add `token_count` to the SA list (§8.1.3).
4. Add `PATCH /service-accounts/:id` (§8.2.4).

### Phase 1 — data layer (hooks)
5. `apps/VyzoriX_web/src/hooks/admin/use-admin-users.ts` — wrap
   `useGetAuthAdminUsers` / `useGetAuthAdminUsersId` / create / update / delete.
6. `apps/VyzoriX_web/src/hooks/organization/use-members.ts` — wrap
   `generated-rq/members/organization-members.ts` (list/role/remove/suspend/
   reinstate/transfer).
7. `apps/VyzoriX_web/src/hooks/organization/use-invitations.ts` — wrap
   `generated-rq/invitations/organization-invitations.ts`.
8. `apps/VyzoriX_web/src/hooks/service-accounts/use-service-accounts.ts` — wrap
   `generated-rq/service-accounts/service-accounts-management.ts` (list/create/
   delete/tokens/create-token/revoke-token/rotate-token).
9. Export the new folders from `hooks/index.ts`.

### Phase 2 — shared UI
10. Add a `Table` primitive to `@vyzorix/ui` **or** keep using the mock's
    `.table` styling via a shared `list-page` style module. The mock's table is
    the contract; a `Table` component is the clean way to reuse it across both
    pages (and the other admin pages that will follow).
11. Add a `SearchInput` (the mock's `.logs-search` pattern) and a
    `SegmentedFilter` (the mock's `.seg`) as shared components — both pages use
    them.
12. Add a `MetaRow` / `SectionCard` pair (the profile page already has private
    versions — promote them to `ui/components/shared/section` for reuse).

### Phase 3 — Users pages
13. Replace `routes/admin/users/index.tsx` (currently an `EmptyState` stub) with
    the list from §7.1.
14. Add `routes/admin/users/$userId.tsx` for the detail from §7.2.
15. Wire "New user" → a `Modal` form (Name, Email, Password, Role).

### Phase 4 — Service accounts pages
16. Rewrite `routes/admin/service-accounts/index.tsx` (currently an ad-hoc
    inline implementation with hardcoded hex colours — `#22c55e`/`#6b7280` — that
    **violate the neutral-badge rule**) to the design in §7.3.
17. Add `routes/admin/service-accounts/$id.tsx` for the detail from §7.4.
18. Build the create-token modal (§7.4) — the highest-value single component.

### Phase 5 — verification
19. `cd apps/VyzoriX_web && npx tsc --noEmit` and `npx vitest run`.
20. Add hook tests with MSW (follow `src/test/hooks/use-api-keys.test.ts`):
    real hooks + real client + MSW, no mocks. Assert flattened return shapes.
21. `python3 scripts/strip_comments.py --check` — client source must stay
    comment-clean.

### Cleanup notes for the implementer
- The existing `routes/admin/service-accounts/index.tsx` uses raw
  `styles.badge` with inline `backgroundColor`/`color` — **delete that pattern**;
  use the neutral `Badge` with a dot.
- The existing page also calls `postServiceAccountsIdRotate(sa.id)` on the
  account id — that is the broken rotate path *and* the wrong id (rotate takes a
  token id). Both must be corrected.
- `useCurrentOrganizationId()` is the org-scoping entry point
  (`hooks/_shared/use-current-context.ts`); every new query must be keyed with it
  so cache entries do not leak across orgs.
- The generated RQ hooks already carry `organizationScopedQueryKey` — prefer
  them over hand-rolled `useQuery` calls so org isolation is automatic.

---

## 10. As-built server changes (2026-11-18)

Everything below is implemented and verified (build, vet, lint, tests, and the
three drift gates are clean). It is the contract the client pages build on.

### 10.1 Service accounts

| Change | Where |
|---|---|
| Token wire shape now carries `expires_at`, `revoked_at`, `last_used_at` | `serviceAccountJSON`/`tokenJSON`, `tooling/schema/service-accounts.cue` |
| `token_count` on every account read (correlated subquery, never stored) | `vyzorix_service_account_storage.go` |
| `PATCH /v1/service-accounts/:id` — rename and/or enable/disable | handler `Update`, service `Update`, route in `server_routes.go` |
| Rotate path corrected to `/service-accounts/:id/tokens/{token}/rotate` (swagger annotation had been `/rotate`) | handler annotation |
| `ActionServiceAccountUpdated` audit action | `audit/audit_repository.go` |

### 10.2 Admin users

| Change | Where |
|---|---|
| `GET /v1/auth/admin/users` now takes `page`, `limit`, `q`, `sort`, `order`; returns `page`/`limit`/`total_pages` | handler `ListUsers`, `AuthService.ListAllUsers(ListUsersQuery)` |
| `AdminUser` gained `avatar_url`, `last_login_at`, `updated_at`, `disabled` | `dto.UserListResponse`, `adminUserFromEntity` |
| `POST /v1/auth/admin/users/:id/password` — policy + breach-checked, revokes sessions | handler `SetPassword`, `AuthService.SetUserPassword` |
| `POST /v1/auth/admin/users/:id/disable` and `.../enable` — stamps `disabled_at`, revokes sessions | handlers, `AuthService.SetUserDisabled` |
| `GET /v1/auth/admin/users/:id/organizations` — memberships with role + status | handler, `AuthService.ListUserOrganizations` |
| `GET /v1/auth/admin/users/:id/sessions` | handler, `AuthService.ListUserSessions` |
| `DELETE /v1/auth/admin/users/:id/sessions/:sessionId` — force logout one device | handler, `AuthService.RevokeUserSession` |
| `DELETE /v1/auth/admin/users/:id/sessions` — force logout all devices | handler, `AuthService.RevokeAllUserSessions` |
| Migration 96 adds `users.disabled_at` (NULL = active) | `096_user_disabled.go` |
| Disabled accounts are refused at login and at session validation (sessions deleted) | `auth_login_session.go` |

### 10.3 Defects found and fixed along the way

- **`UserRepository.Create` bound its values one position off.** The INSERT
  column list had 15 columns but the value list had 16 (`avatar_url` was in the
  values but not the columns, dropped when migration 085 rebuilt the table), so
  every user insert failed with `NOT NULL constraint failed: users.mfa_enabled`.
  Fixed by adding `avatar_url` to the column list; guarded by
  `TestUserRepo_CreatePersistsAvatar`.
- **Rotate was unreachable from the client.** The generated client emitted
  `POST /service-accounts/:id/rotate`; the server route is
  `/service-accounts/:id/tokens/:token_id/rotate`. The swagger annotation was
  also wrong, so the spec (and thus orval) reproduced the wrong path. Both
  fixed; the regenerated client now emits the correct path and takes a token id.
- The dead `Rotate` button in `routes/admin/service-accounts/index.tsx` (which
  called the broken path with an account id) was removed; the list page does not
  have a token id to rotate, so rotate belongs on the detail page.

### 10.4 Regeneration performed

`build_schemas.py` → `swag init` → `build_openapi3.py` → `codegen:api` +
`codegen:hooks`. New client artifacts: `updateServiceAccountRequest`,
`setUserPasswordRequest`, `adminUserOrganization(ListResult)`,
`adminUserSessionListResult`, `getAuthAdminUsersParams`.

### 10.5 Post-review fixes (two security holes found on audit)

A second pass found two places where the "disable" switch was cosmetic:

1. **`Service.Authenticate` never checked the owning account's `Enabled` flag.**
   The package doc promised "a disabled account's tokens stop authenticating", but
   only the *token* was checked. `PATCH /service-accounts/:id {enabled:false}`
   therefore disabled nothing. Fixed: `Authenticate` loads the account and rejects
   the token when it is disabled. Guarded by
   `TestService_DisabledAccountRejectsTokens` (verified to fail without the fix).
2. **`CreateSession` did not check `op.Disabled`.** Only `Login` had a guard, so
   `LoginWithTokens`, TOTP/MFA completion (`auth_totp_mfa.go`) and device
   recognition (`auth_device_recognition.go`) all still minted sessions for a
   disabled account. Fixed at `CreateSession` — the single choke point all four
   paths funnel through — and `LoginWithTokens` now maps `ErrUserDisabled` to 403
   instead of a generic 500. Guarded by `TestCreateSession_RefusesDisabledAccount`.

---

## 11. UI implementation — Grafana contract (2026-11-18)

The visual contract for these pages is **Grafana's**, not the console mock. The
mock is no longer a reference here.

### 11.1 What was copied from Grafana, and from where

| Copied | Grafana source |
|---|---|
| `filter-table` table styling | `packages/grafana-ui/src/themes/GlobalStyles/filterTable.ts` |
| `.page-action-bar` toolbar | `packages/grafana-ui/src/themes/GlobalStyles/page.ts` |
| `.page-heading` section headings | same file |
| Label/value detail rows (`filter-table form-inline`) | `admin/UserProfile.tsx`, `admin/UserSessions.tsx` |
| Inline Edit→Save per row | `admin/UserProfile.tsx` (`UserProfileRow`) |
| Neutral text badge | `Tag` (`packages/grafana-ui/src/components/Tags/Tag.tsx`) |
| Create-token modal, show-secret-once | `serviceaccounts/components/CreateTokenModal.tsx` |

**The `filter-table` characteristics that matter** (ported to
`ui/components/shared/table/table-styles.ts`):

- `border-collapse: separate`, and **no row rules at all** — rows are separated
  by zebra striping (`emphasize(background.primary, 0.02)`), not by lines.
- Fixed **30px** row height with `line-height: 30px`, so rows read as a dense
  list rather than a spacious grid.
- Headers left-aligned, nowrap, sharing the row height.
- Hover emphasises the row further (`emphasize(…, 0.05)`).

### 11.2 Badges: neutral gray text, no dot

`ui/components/shared/status-badge.tsx` renders the project's `Tag` — a plain
gray text badge with a hairline border and no fill. State is carried by the
**word alone** ("Active", "Disabled", "Revoked", "Expired", "Super admin").
There is deliberately **no coloured dot and no colour fill**, so a table of
statuses stays quiet and meaning survives for anyone who cannot distinguish
colours.

### 11.3 Files added

```
hooks/admin/use-admin-users.ts              data layer (list/detail/orgs/sessions + mutations)
hooks/service-accounts/use-service-accounts.ts
ui/components/shared/table/                 Grafana filter-table + page-action-bar
ui/components/shared/status-badge.tsx       neutral gray Tag
ui/components/shared/search-input/          inline search field
ui/components/shared/section/               SectionCard + MetaRow
ui/pages/admin-users/                       list + detail
ui/pages/admin-service-accounts/            list + detail + create-token modal
routes/admin/users/$userId.tsx              new detail route
routes/admin/service-accounts/$id.tsx       new detail route
```

### 11.4 Page shapes as built

**`/admin/users`** — page-action-bar (search · sort select · spacer · count),
then the filter-table: Email · Name · Role · Status · MFA · Last active ·
Created · Edit. Server-side search/sort/pagination (`page,limit,q,sort,order`).

**`/admin/users/$userId`** — Grafana's sectioned detail: **User information**
(inline Edit→Save on Name and Email), **Permissions** (role, MFA, email
verified), **Password** (Set password → modal), **Organizations**,
**Sessions** (per-row Force logout + Force logout from all devices), and
**Account state** (Delete / Disable / Enable). Self-edit of disable/delete is
blocked in the UI, matching the server's own guard.

**`/admin/service-accounts`** — filter-table: Account · ID · Tokens · Status ·
Created · Manage. Empty state offers "Add service account".

**`/admin/service-accounts/$id`** — **Information** (inline rename + Enable /
Disable, which now genuinely stops its tokens authenticating), **Tokens**
(Name · Prefix · Scopes · Expires · Created · Last used · Status · Rotate /
Revoke), and **Delete** (confirm modal naming the token count).

**Create-token modal** — Display name, Scopes (`read`/`write`/`admin`
checkboxes — the VYZORIX equivalent of Grafana's role picker), optional
Expiration date; on success the secret is shown **once** in a read-only field
with Copy to clipboard.

### 11.5 Verification

- `npx tsc --noEmit` clean; `npx eslint src` clean.
- `npx vitest run`: **850 tests / 79 files pass**.
- `scripts/strip_comments.py --check` — 0 changes in every new file.
- Route tree regenerated for the two new detail routes.
