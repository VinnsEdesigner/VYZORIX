/* mobile.js — the interaction layer for the mobile-first shell.
 *
 * Loaded AFTER app.js, so it only adds behaviour:
 *
 *  1. A brand header inside the nav (the full-screen dock needs one; the
 *     floating desktop menu had none).
 *  2. The breadcrumb opens a bottom sheet, which is where the org / device /
 *     page context menu lives on a phone.
 *  3. Table cells get a `data-label` from their column header, so the
 *     stacked-card table on a phone still says what each value is.
 *  4. The nav closes when a view is chosen, and on Escape.
 */
(() => {
  "use strict";
  const $ = (s) => document.querySelector(s);
  const $$ = (s) => Array.from(document.querySelectorAll(s));

  /* The MEGA panel is docked under the three bars, so the bars carry the
     brand and the panel shows none. Nothing to inject here. */

  /* ── 2. The bottom sheet ─────────────────────────────────── */
  let scrim = $(".sheet-scrim");
  if (!scrim) {
    scrim = document.createElement("div");
    scrim.className = "sheet-scrim";
    document.body.appendChild(scrim);
  }
  let sheet = $(".sheet");
  if (!sheet) {
    sheet = document.createElement("div");
    sheet.className = "sheet";
    sheet.innerHTML =
      '<div class="sheet__grab"></div><div class="sheet__title"></div>' +
      '<div class="sheet__sub"></div><div class="sheet__body"></div>';
    document.body.appendChild(sheet);
  }

  const closeSheet = () => {
    sheet.classList.remove("is-open");
    scrim.classList.remove("is-open");
  };
  const openSheet = (title, sub, items) => {
    sheet.querySelector(".sheet__title").textContent = title;
    const subEl = sheet.querySelector(".sheet__sub");
    subEl.textContent = sub || "";
    subEl.style.display = sub ? "" : "none";

    const body = sheet.querySelector(".sheet__body");
    body.innerHTML = items
      .map((it) => {
        if (it === null) return '<hr class="sheet__sep" />';
        const check = it.active ? '<span class="sheet__check">✓</span>' : "";
        return `<button class="sheet__item${it.danger ? " sheet__item--danger" : ""}" data-k="${it.k}">
          ${it.icon || ""}<span>${it.label}</span>${check}</button>`;
      })
      .join("");

    body.querySelectorAll(".sheet__item").forEach((b) =>
      b.addEventListener("click", () => {
        closeSheet();
        const it = items.find((x) => x && x.k === b.dataset.k);
        if (it && it.go) it.go();
      }),
    );

    sheet.classList.add("is-open");
    scrim.classList.add("is-open");
  };
  scrim.addEventListener("click", closeSheet);

  const ICON = {
    home: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M3 10.5L12 3l9 7.5V21H3z"/></svg>',
    device: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="6" y="2" width="12" height="20" rx="3"/><path d="M10.5 18h3"/></svg>',
    org: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M3 21h18M5 21V7l7-4 7 4v14"/></svg>',
    page: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M6 3h9l4 4v14H6z"/><path d="M15 3v4h4"/></svg>',
    star: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M12 3l2.6 5.6 6.4.8-4.6 4.4 1.1 6.2L12 17.8 6.5 20l1.1-6.2L3 9.4l6.4-.8z"/></svg>',
    settings: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3"/></svg>',
  };

  /* The breadcrumb: tap to open the context sheet. On a phone this is the
     primary way to switch org or page, since the bars have no room. */
  const crumbs = $(".crumbs");
  if (crumbs) {
    crumbs.setAttribute("role", "button");
    crumbs.setAttribute("tabindex", "0");
    crumbs.addEventListener("click", () => {
      const current = ($("#crumb") || {}).textContent || "Home";
      const active = ($$(".nav__item.is-active")[0] || {}).dataset?.view;
      const org = ($("#org-picker") || {}).value || "Acme Ops";

      openSheet("Context", "Acme Ops · " + current, [
        { k: "home", label: "Home", icon: ICON.home, go: () => pick("home"), active: active === "home" },
        { k: "devices", label: "Devices", icon: ICON.device, go: () => pick("devices"), active: active === "devices" },
        { k: "permissions", label: "Permissions", icon: ICON.org, go: () => pick("permissions"), active: active === "permissions" },
        { k: "telemetry", label: "Telemetry", icon: ICON.page, go: () => pick("telemetry"), active: active === "telemetry" },
        null,
        { k: "org", label: "Organization: " + org, icon: ICON.org, go: () => showToast("Switch organization", "Multi-org picker (mock)") },
        { k: "fav", label: "Add to favourites", icon: ICON.star, go: () => showToast("Added to favourites", current) },
        { k: "settings", label: "Page settings", icon: ICON.settings, go: () => showToast("Page settings", "Per-page preferences (mock)") },
      ]);
    });
    crumbs.addEventListener("keydown", (e) => {
      if (e.key === "Enter" || e.key === " ") { e.preventDefault(); crumbs.click(); }
    });
  }

  /* Choosing a view: reuse the mock's own nav buttons so the state stays in
     one place, then close the nav (it covers the screen on a phone). */
  const pick = (view) => {
    const btn = $$(".nav__item").find((b) => b.dataset.view === view);
    if (btn) btn.click();
    closeMenu();
  };

  /* ── 3. Table cells get a label from their column header ─── */
  const labelTables = () => {
    $$("table.table").forEach((t) => {
      const heads = Array.from(t.querySelectorAll("thead th")).map((th) =>
        th.textContent.trim(),
      );
      t.querySelectorAll("tbody tr").forEach((tr) => {
        Array.from(tr.children).forEach((td, i) => {
          if (heads[i] && !td.hasAttribute("data-label")) {
            td.setAttribute("data-label", heads[i]);
          }
        });
      });
    });
  };
  labelTables();
  /* Views are re-rendered wholesale, so re-label after any nav click. */
  document.addEventListener("click", () => setTimeout(labelTables, 0), true);

  /* ── 4. Nav behaviour ────────────────────────────────────── */
  const chromeEl = $(".chrome");
  const closeMenu = () => chromeEl && chromeEl.classList.remove("is-menu-open");
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") { closeSheet(); closeMenu(); }
  });


  /* ── 5. The panel "⋯" menus become bottom sheets ─────────────
     Grafana puts a small dropdown under the ⋯; on a phone a dropdown at the
     panel corner is cramped and often clipped. The same items go into the
     bottom sheet, which is full width and thumb-reachable. */
  const PANEL_MENU = (title) => [
    { k: "view", label: "View", icon: ICON.page, go: () => showToast("View", title) },
    { k: "edit", label: "Edit panel", icon: ICON.settings, go: () => showToast("Edit panel", title) },
    { k: "share", label: "Share", icon: ICON.star, go: () => showToast("Share", title) },
    null,
    { k: "inspect", label: "Inspect query", icon: ICON.page, go: () => showToast("Inspect", title) },
    { k: "export", label: "Export CSV", icon: ICON.page, go: () => showToast("Export", title) },
    { k: "remove", label: "Remove panel", icon: ICON.settings, danger: true, go: () => showToast("Removed", title) },
  ];
  document.addEventListener("click", (e) => {
    const m = e.target.closest(".panel__menu");
    if (!m) return;
    e.preventDefault();
    e.stopPropagation();
    const panel = m.closest(".panel");
    const title = (panel && panel.querySelector(".panel__title") || {}).textContent || "Panel";
    openSheet(title, "Panel actions", PANEL_MENU(title));
  }, true);

  /* ── 6. The user menu becomes a bottom sheet on a phone ──────
     The avatar's dropdown is anchored to the top-right corner; on a phone it
     is easier to reach from the bottom. */
  const userBtn = $("#user-btn");
  const userMenu = $("#user-menu");
  if (userBtn && userMenu) {
    const items = Array.from(userMenu.querySelectorAll(".user-menu__item"));
    const name = (userMenu.querySelector(".user-menu__name") || {}).textContent || "";
    const email = (userMenu.querySelector(".tiny") || {}).textContent || "";
    userBtn.addEventListener("click", (e) => {
      if (window.matchMedia("(min-width: 701px)").matches) return; // desktop keeps its dropdown
      e.preventDefault();
      e.stopPropagation();
      openSheet(name || "Account", email, items.map((it) => ({
        k: it.dataset.go || it.dataset.action || "x",
        label: it.textContent.trim(),
        icon: it.querySelector("svg") ? it.querySelector("svg").outerHTML : "",
        danger: it.classList.contains("user-menu__item--danger"),
        go: () => it.click(),
      })));
    }, true);
  }

  /* Expose the sheet helpers for the console, so other callers can reuse it. */
  window.vxSheet = { open: openSheet, close: closeSheet };
})();
