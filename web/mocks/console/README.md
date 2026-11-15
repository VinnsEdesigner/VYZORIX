# Vyzorix Console — design mock

A **standalone, dependency-free design mock** of the Vyzorix console (home, devices,
apps, permissions, telemetry, updates, alerts, logs, terminal, organizations, API keys,
settings, profile, and the Grafana-style Administration pages).

Operational pages are **scoped to an organisation**: pick one from the top-bar chip or
the in-page gate before devices, apps, logs, terminal, etc. will open. Home, profile,
settings and the Administration pages work without one.

It exists to lock the **visual language** before porting into the real apps
(`apps/Vyzorix_web`, `apps/Vyzorix_mobile`). The code here is throwaway — it is *not* the
product; it is a reference for the real implementation.

## Run

```bash
cd web/mocks/console
PORT=3000 python3 serve.py     # stdlib only, no deps
# open http://localhost:3000
```

## Design language

- Grafana-inspired three-row chrome: top bar (menu + search) → breadcrumb bar → in-page tabs
- Auto-hiding mega menu (unfurls from the left edge; no persistent rail)
- Neutral dark surface (`#0f0f0f`/`#171717`) with a **rose 400–700** accent
- Inter for all text, including technical (IDs, logs, terminal), 8px spacing grid, radii 4/6/10
- **Every badge is neutral** (a plain pill). Where a state matters it is carried by
  a small semantic **dot** inside the pill, or by the word — never by a coloured fill
- Icons: nav icons are Grafana's set (`icons.js`); all UI chrome icons are a
  cohesive 24px stroke set mounted from `data-ui` placeholders (`VyzIcons.ui`)

## Notes / caveats

- **Data is illustrative** — nothing here talks to the server.
- **`icons.js` embeds SVG path data extracted from Grafana's icon set**
  (`grafana/public/img/icons`, which derives from Unicons / Grafana custom icons).
  Grafana is AGPL-3.0 and Unicons has its own license — **verify licensing before
  shipping these icons in a distributed build**, or swap for an in-house/iconify set.
- Palette/typography tokens live in `theme.css`; they are the intended source of truth
  when this is ported into `packages/config` + `packages/ui`.
