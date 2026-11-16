# Theme System — Contract

> Status: approved for implementation (2026-11-16)
> The single-theme model in `THEME.md` is superseded by this document.

This document is the contract for the multi-theme system: the styling
foundation, the eight themes, the registry, the preference model, and how the
client resolves and persists a theme.

---

## 1. Goal

A signed-in operator picks one of **eight themes** in their personal
preferences. The choice is stored server-side as the user default, can be set as
an **organisation default** that members inherit until they override it, and is
applied to the client with no flash on load.

## 2. Design decisions

| Concern | Decision |
|---|---|
| Styling | **Emotion** (`@emotion/css`, `@emotion/react`). No CSS framework. |
| Theme shape | `createTheme()` builds a **`VyzorixTheme` object** (colors, breakpoints, spacing, shape, typography, shadows, transitions, zIndex). |
| Theme catalogue | JSON definitions in `themeDefinitions/`, loaded into a `registry.ts`. |
| Delivery | Theme object through React context; `useStyles2((theme) => css({…}))`. |
| Colour model | A definition supplies a partial colour set; it is merged onto a dark/light base, and `getRichColor` derives the rest from each colour's `main`. A theme can be `{ mode, primary: { main } }`. |
| Preference storage | Backend: **user** preference (`user_settings.preferences.theme`) and **org** default (`organization_settings.theme`). |
| Validation | Backend `theme.IsValid(id)` against the eight ids; `400` on unknown. A test asserts the Go list and the frontend registry agree. |
| Apply + persist | Store resolves the id to a theme object and sets state; the hook PATCHes `/v1/me/preferences`. |
| `system` | A **mode** (`followSystem`), not one of the eight themes. |
| Anonymous | Out of scope: the console is authenticated-only. |

The theme object is injected once via `injectGlobalStyles(theme)` (document
background/foreground and a minimal reset); component styles live in each
component's Emotion block, so a theme switch is a context update rather than a
stylesheet fetch.

## 3. Styling foundation

**Decision: Emotion.**

- The theme is a plain object built by `createTheme` from a JSON definition and
  supplied through React context.
- Components style with `useStyles2((theme) => css({...}))`; the memoised
  creator caches per theme.
- `injectGlobalStyles(theme)` sets the document background/foreground and a
  minimal reset once per theme change.
- The `@vyzorix/ui` package owns the theme system; the app consumes it.

> **Foundation this establishes:** there was previously no styling engine wired
> at all. This gives the app a real one.

### 3.1 Theme contract

`createTheme` accepts a partial `NewThemeOptions`:

```
{ name, id, colors?, spacing?, shadows?, shape?, typography? }
```

`colors` is the only field every theme sets. It carries `mode`, the semantic rich colours (`primary`, `secondary`, `tertiary`, `accent`,
`info`, `error`, `success`, `warning`), plus `text`, `background`, `border`,
`gradients`, `action`, `scrollbar`. `getRichColor` fills `mainEmphasis`,
`contrastText`, `text`, `border`, `background` and the subtle variants from each
colour's `main`, so a definition stays small.

## 4. The eight themes

| id | Name | mode | Accent |
|---|---|---|---|
| `dark` | Vyzorix Dark | dark | rose-400 (default) |
| `light` | Vyzorix Light | light | rose-600 |
| `rose-500` | Rose 500 | light | rose-500 (high-contrast, brand-forward) |
| `burgundy` | Burgundy | dark | wine/burgundy |
| `matrix` | Matrix | dark | phosphor green |
| `tron` | Tron | dark | cyan/blue |
| `synthwave` | Synthwave | dark | magenta/blue |
| `zen` | Zen | light | sage green |

Plus a non-theme mode: **Follow system** (`system`), which resolves to `dark` or
`light` from `prefers-color-scheme`. The picker shows eight theme cards and a
follow-system toggle.

Adding a theme = one JSON definition + one registry entry + one id in the
backend allowlist. No other code changes.

## 5. Preference model

Three tiers:

```
effective theme = user preference  >  org default  >  "dark" (system default)
```

- **User default (personal).** `user_settings.preferences` JSON, key `theme`.
  Read/written via `GET`/`PATCH /v1/me/preferences`. This is the personal
  default the task asks for.
- **Org default (selectable per organisation).** `organization_settings.theme`.
  Set by an org admin; every member without a personal override inherits it.
  Read/written via the organisation settings endpoints. Defaults to **empty**,
  not `dark`, so an organisation with no explicit choice contributes nothing and
  the system default applies at the bottom.
- **System default.** `dark`, applied when neither tier is set.

The org value is a **default**, never a lock: a member who chooses a personal
theme keeps it. (A future "enforce org theme" flag is explicitly out of scope.)

### 5.1 The cascade is resolved server-side

The client **cannot** perform `user ?? org ?? dark` itself: reading an
organisation's settings requires admin, so a non-admin member has no way to see
the org tier. The server therefore resolves the cascade and returns the
effective theme.

- `GetPreferences` deliberately **omits** `theme` when the user has not set one.
  An absent key means "inherit"; defaulting it to `dark` would make every user
  look like they had explicitly chosen dark and the org tier would never apply.
- `ClientSettingsService.ResolveTheme(ctx, userID, orgID)` is the merge:
  user preference, else the organisation's theme (via the `OrgThemeSource`
  interface, satisfied by the organisation settings service), else `dark`.
- `GET /v1/me/preferences` returns `PreferencesResult{preferences, theme}`,
  where `theme` is the resolved value. The org tier is read through
  `OrgThemeSource`, an interface declared in the auth package to avoid an
  import cycle (`application/organization` already imports `application/auth`).

## 6. Client resolution and persistence

**Store** (`src/stores/theme-store.ts`):

- State: `{ themeId, resolvedThemeId, theme, followSystem }`, where `theme` is the
  built `VyzorixTheme` object.
- `setTheme(id)` → validate against the registry, build the theme object, and
  mirror the id locally. The store performs **no** server write; persisting is
  the hook's job (below).
- `setFollowSystem(bool)` → toggles the `system` mode and re-resolves.
- The store never blocks paint: it builds the last-known theme immediately, then
  reconciles with the server value when it arrives.

**Local store** (`src/lib/local-store/`): a `user_prefs` table
(`user_id` PK, `theme`, `updated_at`) caches the **explicit** theme id — the same
value the `localStorage` mirror holds — so the first paint after a reload is
correct with zero network. It is not the resolved id: the mirror is written
synchronously on every change and the durable copy mirrors it. The store is
already scope-isolated per user; the theme is a plain non-secret value.

**Hooks** (`src/hooks/theme/`): `useTheme()` (read/resolve),
`useThemePreference()` (server query — adopts the server-resolved `theme`),
`useUpdateThemePreference()` (server mutation — applies locally first, then
PATCHes). A successful mutation updates the store and the cache.

**Anti-flash.** A small inline script in `__root.tsx` reads the cached theme id
from `localStorage` (the pre-hydration mirror) and sets `color-scheme` plus a
background colour on `<html>` before first paint, so the first frame is not a
white flash. React then injects the full theme object from context.

## 7. Backend contract

- **Validation.** `theme.IsValid(id)` against the eight ids (+ `system`).
  `PATCH /v1/me/preferences` and the org settings write return
  `400 {"error":{"code":"VALIDATION_FAILED"}}` for an unknown id. An empty string
  (or a nil value) clears the key so the next tier applies.
- **User endpoint.** `PreferencesResult{preferences, theme}`; `preferences`
  carries the raw preference map and `theme` is the server-resolved effective
  theme id.
- **Org field.** `organization_settings.theme TEXT NOT NULL DEFAULT ''`, added
  by migration **087** via `ALTER TABLE` (a new column cannot be added by editing
  the `CREATE TABLE` in migration 041 — that table already exists, so the
  `CREATE TABLE IF NOT EXISTS` is a no-op on every deployed database). Exposed on
  the existing organisation settings GET/PATCH; no new routes.
- The eight ids are the single source of truth; a Go test asserts the Go
  allowlist and the frontend registry agree on the id set.

## 8. Accessibility

Every theme MUST meet WCAG AA (4.5:1) for the primary text on the canvas
background and for `primary.contrastText` on `primary.main`. A registry test
computes contrast for each theme and fails the build otherwise.

## 9. Rollout

1. Emotion + theme core in `@vyzorix/ui` (`createTheme`, colours,
   spacing, shape, typography, shadows, transitions, zIndex). *(done)*
2. Registry + eight theme definitions (JSON) + `useStyles2`/`useTheme2`. *(done)*
3. Store + hooks + local-store `user_prefs` table + anti-flash script. *(done)*
4. Backend validation + org `theme` column + org endpoint field. *(done)*
5. Theme selector UI (card grid + follow-system toggle) in personal settings. *(pending — the settings page does not exist yet)*

## 10. Non-goals

- Per-device theme, theme enforcement/locking, custom user-defined themes,
  theme marketplace, motion/typography variants, swapping `<link>` stylesheets.

## 11. Acceptance criteria

- Eight themes selectable; switching applies instantly with no reload.
- Choice survives reload with no flash (cached) and is stored server-side
  (personal default).
- Org default is inherited by a member with no personal override; a personal
  choice wins.
- Unknown id rejected `400` by the server.
- Every theme passes the contrast test; the id-set test passes both sides.
