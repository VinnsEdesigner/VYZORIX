# Profile Preferences — Build Contract

**Status: PROVISIONAL — DELETE THIS DOCUMENT once every section below is implemented and merged.**
It exists only to pin the design while the work is split across sessions. It is not a durable
design record; the durable record is the code plus `AGENTS.md`.

Owner: client-side UI build. Last updated: 2026-11-18.

---

## 1. Why

The profile page has one preferences surface: the **organisation default preferences**
(admin → Default preferences), which sets the cascade default every member inherits. Members
have no way to express their own preferences, and the console is missing most of the settings a
daily operator expects (density, timestamps, notifications, privacy).

This contract adds a full **personal preferences** experience as tabs inside the profile page.

## 2. Naming (resolves the "two preference tabs" clash)

There are two different preference surfaces and they must not share a name:

| Surface | Route | Audience | Renamed to |
|---|---|---|---|
| Org cascade defaults (exists today) | `/admin/default-preferences` | admins | **Organisation defaults** (nav label + page title) |
| Personal preferences (this contract) | `/profile` → `Preferences` tab | every member | **Preferences** |

The account menu **Preferences** entry points at the personal Preferences tab
(`/profile?tab=preferences`). The account menu **Organization settings** entry points at
`/settings`; its `?tab=api-keys` link is unchanged. The admin surface stays reachable from
Administration → General → **Organisation defaults**.

## 3. Tab model

Preferences live as **tabs inside the profile page**, not separate routes. The profile route
becomes tab-aware: `/profile?tab=<id>`. Default tab: `profile`.

Tabs (in order):

1. `profile` — **Profile** (the existing profile content; unchanged)
2. `preferences` — **Preferences** (the personal preference groups below)

Additional tabs are added in later phases of this contract (Notifications, Privacy). Each is a
tab, never a new nav item.

Tabs are implemented with the same `?tab=` search-param pattern the rest of the console already
uses (`pageTabs` in `app-nav.ts` + the route's search params). Adding `'/profile'` to `pageTabs`
is what makes the tab bar render.

## 4. Preference groups and options

Every preference below is a **personal** preference (per user). Where the organisation can also
set a value, the personal value is an **override**; unset means "inherit the organisation
default", and the UI must show that (an explicit "Default / inherit" choice).

### 4.1 Appearance & display (Phase 1)

| id | Label | Control | Values | Default |
|---|---|---|---|---|
| `density` | Interface density | segmented | `comfortable` \| `compact` \| `spacious` | `comfortable` |
| `textScale` | Text size | segmented | `small` (0.9) \| `default` (1) \| `large` (1.15) | `default` |
| `reduceMotion` | Reduce motion | switch | bool | `false` |
| `timeFormat` | Time format | segmented | `12h` \| `24h` | `24h` |
| `relativeTimestamps` | Relative timestamps | switch | bool | `true` |
| `byteUnits` | Byte units | segmented | `binary` (KiB) \| `decimal` (KB) | `binary` |
| `thousandsSeparator` | Thousands separator | switch | bool | `true` |

Behaviour notes:
- `density` scales the base spacing unit (8px → 6/8/11) applied through the app shell content
  padding and list rows.
- `textScale` sets the root font size on `document.documentElement` (14px × scale).
- `reduceMotion` injects a global rule that disables transitions/animations unless the OS already
  asks for reduced motion (in which case it is a no-op).
- `timeFormat`, `relativeTimestamps`, `byteUnits`, `thousandsSeparator` are **display
  formatters**: they must be read through a single `usePreferencesFormat()` helper so every
  timestamp/byte in the app formats consistently. Phase 1 wires the helper and applies it to the
  profile page's own dates plus the sessions list; other pages adopt it incrementally.

### 4.2 Behaviour & defaults (Phase 1)

| id | Label | Control | Values | Default |
|---|---|---|---|---|
| `landingPage` | Landing page after sign-in | combobox | `last` \| `home` \| `devices` \| `logs` | `last` |
| `defaultTimeRange` | Default time range | combobox | `15m` \| `1h` \| `6h` \| `24h` \| `7d` | `1h` |
| `tablePageSize` | Rows per page | segmented | `25` \| `50` \| `100` | `50` |
| `liveUpdates` | Live updates | switch | bool | `true` |
| `confirmDestructive` | Confirm destructive actions | switch | bool | `true` |

Behaviour notes:
- `landingPage` is consulted after a successful login/organisation select.
- `defaultTimeRange` seeds the time-range control on charts/logs/telemetry when the URL has no
  explicit range.
- `confirmDestructive` adds a confirmation dialog to destructive UI actions (device wipe, revoke
  session, delete API key, rotate secret).

### 4.3 Step-up verification for high-criticality actions (Phase 2 — design only in this contract)

**Intent.** Some actions are high-criticality (reveal or create an API key, revoke all sessions,
wipe a device, rotate a secret). When the user has enabled **fingerprint security** (a later
Security feature), those actions must require a **step-up**: re-verify the device fingerprint
(WebAuthn/passkey) before the action proceeds.

**Explicit non-goal.** This is **not** command-level gating. The terminal is a shell; individual
commands are governed by **server permissions, roles and scopes** and are intentionally not
mappable to client toggles. Step-up applies to **discrete high-criticality UI actions**, the
canonical example being **API keys**, not to shell commands.

Preference (Phase 2):

| id | Label | Control | Values | Default |
|---|---|---|---|---|
| `stepUpActions` | Require device verification for | multi-select | `api-keys` \| `sessions` \| `device-wipe` \| `secret-rotate` | `['api-keys']` |

Phase 2 is blocked on the fingerprint-security feature. Until then the control renders **disabled**
with a hint ("Enable fingerprint security to use this"), and no enforcement ships.

### 4.4 Notifications (Phase 3)

Personal notification routing, overriding the org defaults:

| id | Label | Control | Values | Default |
|---|---|---|---|---|
| `notifyEmail` | Email | switch | bool | inherit |
| `notifyInApp` | In-app | switch | bool | inherit |
| `notifyPush` | Web push | switch | bool | inherit |
| `notifySeverityFloor` | Minimum severity | combobox | `info` \| `warning` \| `critical` | inherit |
| `quietHours` | Quiet hours | time range | `{ start, end }` \| off | off |

Event-level toggles (`new-device-login`, `device-offline`, `threshold-breach`, `update-push`) are
Phase 3 and map onto the existing server `NotificationSettings`.

### 4.5 Privacy, security & sessions (Phase 4)

| id | Label | Control | Values | Default |
|---|---|---|---|---|
| `analyticsConsent` | Share usage analytics | switch | bool | current consent store value |
| `maskSensitive` | Mask sensitive data by default | switch | bool | `false` |
| `idleTimeoutMinutes` | Sign out after inactivity | combobox | `15` \| `30` \| `60` \| `never` | `30` |
| `rememberLastOrg` | Remember last organisation | switch | bool | `true` |

`analyticsConsent` reads/writes the existing `vyzor-consent-store`, not a new key.

### 4.6 Accessibility (Phase 4)

| id | Label | Control | Values | Default |
|---|---|---|---|---|
| `highContrast` | High contrast | switch | bool | `false` |
| `underlineLinks` | Always underline links | switch | bool | `false` |
| `alwaysShowFocus` | Always show focus rings | switch | bool | `false` |

## 5. Persistence contract

Two tiers, mirroring the theme:

1. **Local (source of truth for rendering).** A persisted zustand store
   `usePreferencesStore` (`vyzorix-preferences`), so the UI is correct before any network call.
   It holds every value in section 4.
2. **Server (durable, cross-device).** The existing per-user preferences payload
   (`GET/PATCH /v1/me/preferences`) gains the new keys under `preferences`. Keys are written in
   the same snake_case the server already uses for `date_format`/`week_start`.

Rules:
- Local wins on load; the server value is adopted when it arrives (same pattern as theme, and
  subject to the same "do not let a late async read clobber a settled value" rule).
- A change applies locally immediately, then PATCHes. A failed PATCH leaves the local value and
  surfaces a toast.
- Unset (inherit) is stored as an absent key, never an empty string.
- Only Phase 1 keys are added to the server payload in Phase 1. Later phases add theirs when
  implemented.

## 6. Components & files (Phase 1)

New:
- `src/stores/preferences-store.ts` — persisted store + defaults + setters.
- `src/lib/preferences/format.ts` — `formatTimestamp`, `formatBytes`, `formatNumber` reading the store.
- `src/hooks/preferences/use-display-preference.ts` — `useDisplayPreferences()` (density, text scale, motion side-effects) and `usePreferencesFormat()`.
- `src/ui/pages/profile/preferences-tab.tsx` — the Preferences tab content (Appearance, Behaviour groups).
- `src/ui/components/preferences/segmented-setting.tsx` — labelled segmented control row.

Changed:
- `src/routes/profile/index.tsx` — tab-aware (`?tab=`), renders Profile or Preferences.
- `src/ui/components/layout/app-nav.ts` — add `'/profile'` to `pageTabs`; rename the admin
  surface label to **Organisation defaults**.
- `src/ui/components/layout/account-menu.tsx` — Preferences → `/profile?tab=preferences`.
- `src/providers/theme-provider.tsx` (or a new provider) — apply `density`, `textScale`,
  `reduceMotion` side-effects globally.

## 7. Definition of done (per phase)

- `tsc --noEmit` clean, `vitest run` green, `vite build` clean.
- Client source is comment-free (`scripts/strip_comments.py --check` → 0 changes).
- Every control is keyboard-reachable and has an accessible name.
- New store and formatters covered by tests.
- When all phases are done: delete this document.

## 8. Open questions — RESOLVED

- **Server keys: allowlist chosen (senior decision).** The server no longer merges arbitrary keys.
  `apps/api/internal/application/auth/auth_preferences_schema.go` holds `preferenceSchema`, an
  explicit allowlist with per-key kind + allowed values. `UpdatePreferences` rejects an unknown key
  or an out-of-set value with `ErrValidationError` before persisting. Rationale: the client sends a
  flat map that is stored as JSON, so without a boundary any authenticated caller could grow the
  stored document without limit. An empty value clears an allowed key. Covered by
  `auth_preferences_schema_test.go` and `TestUpdatePreferences_RejectsUnknownKey`.
- **Drift between the two key lists is guarded — DONE.** The client's synced-key list lives in
  `apps/VyzoriX_web/src/test/lib/preferences-test-helpers.ts` (`PREFERENCE_KEYS_UNDER_TEST`) and is
  asserted against the runtime map by `preferences-sync-keys.test.ts`. The Go side
  (`auth_preferences_drift_test.go`) reads that same file and fails if the client syncs a key the
  server does not accept, or if a schema key is neither client-synced nor declared server-only
  (theme, timezone, language, the notification prefs, and the phase 2/4 keys the client keeps
  local). Mirrors the existing `theme_test.go` cross-language pattern.
- **Density affects the nav panel width too — DONE.** `useDisplayPreferences` sets
  `--vyz-nav-width` (compact 240 / comfortable 280 / spacious 320) and the mega-nav's open width
  reads it (`app-shell.tsx`). Below `sm` the nav still goes full-width.
- **Idle timeout reconciled — DONE.** `idleTimeoutMinutes` (0 = never, 15/30/60) is a preference;
  `use-auth-session.ts` derives the activity timeout from it (an explicit option still overrides for
  tests). `0` disables the inactivity sign-out entirely.

## 9. Phase status

- Phase 1 (Appearance, Behaviour): **done**, local + server sync.
- Phase 2 (step-up verification): **control shipped disabled**; enforcement blocked on the
  fingerprint-security feature.
- Phase 3 (Notifications): **next**.
- Phase 4 (Security & sessions, Accessibility): **done** (idle timeout, high contrast, underline
  links, always-show-focus).
