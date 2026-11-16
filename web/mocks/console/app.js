/* app.js — Vyzorix UI mock. Plain JS, no build step.
 *
 * Views mirror the real VYZORIX surface (REST + GraphQL):
 * dashboard, devices, commands, telemetry, updates (OTA), alerts, logs,
 * organizations, api-keys, settings. Data here is mock/illustrative.
 */

const $ = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];
const el = (html) => { const t = document.createElement("template"); t.innerHTML = html.trim(); return t.content.firstElementChild; };

// ---------------------------------------------------------------- mock data
const DEVICES = [
  { imei: "354481100123451", model: "Vyzorix AR-1", fw: "0.4.2", status: "online",  battery: 87, charging: false, thermal: "nominal", risk: 12, rssi: -58, org: "Acme Ops", tier: "extended", container: "debian-core 12.4", lastSeen: "now",        uptime: "6d 4h", storage: 38 },
  { imei: "354481100123452", model: "Vyzorix AR-1", fw: "0.4.2", status: "online",  battery: 64, charging: true,  thermal: "warm",    risk: 38, rssi: -71, org: "Acme Ops", tier: "extended", container: "debian-core 12.4", lastSeen: "12s ago",    uptime: "2d 19h", storage: 61 },
  { imei: "354481100123453", model: "Vyzorix AR-2", fw: "0.3.9", status: "offline", battery: 4,  charging: false, thermal: "—",       risk: 91, rssi: 0,   org: "Northwind", tier: "base", container: "debian-core 12.1", lastSeen: "22m ago",   uptime: "—",      storage: 88 },
  { imei: "354481100123454", model: "Vyzorix AR-2", fw: "0.4.1", status: "online",  battery: 52, charging: false, thermal: "hot",     risk: 66, rssi: -83, org: "Northwind", tier: "extended", container: "debian-core 12.4", lastSeen: "4s ago",     uptime: "0d 6h", storage: 47 },
  { imei: "354481100123455", model: "Vyzorix AR-1", fw: "0.4.2", status: "online",  battery: 95, charging: false, thermal: "nominal", risk: 5,  rssi: -49, org: "Acme Ops", tier: "extended", container: "debian-core 12.4", lastSeen: "now",        uptime: "11d 2h", storage: 22 },
  { imei: "354481100123456", model: "Vyzorix AR-2", fw: "0.3.9", status: "offline", battery: 22, charging: false, thermal: "—",       risk: 74, rssi: 0,   org: "Globex", tier: "base", container: "debian-core 12.1", lastSeen: "1h 12m ago", uptime: "—",      storage: 73 },
];

// ---------------------------------------------------------------- permissions
// Organization membership: a user joined to an org with a role. Roles are
// super_admin / admin / operator / viewer (ORGANIZATIONS.md). Permissions are
// derived from the role, so this page grants a role and reviews what it yields.
// ---------------------------------------------------------------- permissions
// The capability catalog, modelled on the real server (apps/api/internal/domain/
// permission/): a permission is an (action, scope) pair, and a role's defaults
// are a strict superset ladder viewer ⊂ operator ⊂ admin ⊂ super_admin, failing
// closed to viewer. The UI must never invent a rule the server would not apply,
// so this table mirrors defaults.go exactly.
const SCOPES = { devices: "devices:*", org: "org:*", updates: "updates:*", telemetry: "telemetry:*", diagnostics: "diagnostics:*", inbox: "inbox:*" };

// Base actions → the scope each applies to, and a human label + where it shows.
// `gate` names the UI surface the capability protects, so a page can ask
// "am I allowed here" without hardcoding role names.
const ACTIONS = [
  { action: "device.read",        scope: "devices",     label: "View devices & telemetry",        group: "Devices" },
  { action: "device.write",       scope: "devices",     label: "Register / edit devices",          group: "Devices" },
  { action: "device.delete",      scope: "devices",     label: "Deregister devices",               group: "Devices" },
  { action: "device.assign",      scope: "devices",     label: "Transfer / assign devices",        group: "Devices" },
  { action: "command.execute",    scope: "devices",     label: "Run actions on devices",           group: "Operations" },
  { action: "shell.execute",      scope: "devices",     label: "Open a terminal on a device",      group: "Operations" },
  { action: "usb.attach",         scope: "devices",     label: "Attach USB devices",               group: "Operations" },
  { action: "apps.read",          scope: "devices",     label: "View supervised apps",             group: "Apps" },
  { action: "apps.write",         scope: "devices",     label: "Start / stop / deploy apps",       group: "Apps" },
  { action: "telemetry.read",     scope: "telemetry",   label: "View telemetry",                   group: "Observability" },
  { action: "telemetry.write",    scope: "telemetry",   label: "Configure telemetry",              group: "Observability" },
  { action: "diagnostics.read",   scope: "diagnostics", label: "View diagnostics",                 group: "Observability" },
  { action: "logs.read",          scope: "devices",     label: "Read logs",                        group: "Observability" },
  { action: "alert.read",         scope: "org",         label: "View alerts",                      group: "Observability" },
  { action: "alert.write",        scope: "org",         label: "Manage alert rules",               group: "Observability" },
  { action: "update.read",        scope: "updates",     label: "View releases",                    group: "Operations" },
  { action: "update.write",       scope: "updates",     label: "Push updates",                     group: "Operations" },
  { action: "update.delete",      scope: "updates",     label: "Delete releases",                  group: "Operations" },
  { action: "members.read",       scope: "org",         label: "View members",                     group: "Access" },
  { action: "members.write",      scope: "org",         label: "Invite / change roles",            group: "Access" },
  { action: "members.delete",     scope: "org",         label: "Remove members",                   group: "Access" },
  { action: "keys.manage",        scope: "org",         label: "Manage API keys",                  group: "Access" },
  { action: "audit.read",         scope: "org",         label: "Read the audit log",               group: "Access" },
  { action: "settings.read",      scope: "org",         label: "View settings",                    group: "Access" },
  { action: "settings.write",     scope: "org",         label: "Change organization settings",     group: "Access" },
  { action: "admin.lockout",      scope: "org",         label: "Lock out users",               group: "Administration" },
  { action: "admin.impersonate",  scope: "org",         label: "Impersonate users",            group: "Administration" },
];

// The role defaults, copied from defaults.go: each level is the level below plus
// its own additions. An unknown role falls back to viewer (fail closed).
const ROLE_GRANTS = {
  viewer: ["device.read", "update.read", "settings.read", "telemetry.read", "diagnostics.read", "logs.read", "alert.read", "apps.read"],
  operator: ["device.write", "command.execute", "apps.write"],
  admin: ["device.delete", "device.assign", "update.write", "update.delete", "settings.write", "keys.manage", "audit.read", "telemetry.write", "members.read", "members.write", "members.delete", "alert.write", "shell.execute", "usb.attach"],
  super_admin: ["admin.lockout", "admin.impersonate"],
};
const ROLES = ["viewer", "operator", "admin", "super_admin"];

// Effective capability set for a role: the union of every level up to and
// including it. Mirrors UnionScopes(role, custom) with custom = [].
function capabilitiesForRole(role) {
  const idx = ROLES.indexOf(role);
  const upto = idx < 0 ? ["viewer"] : ROLES.slice(0, idx + 1);
  const set = new Set();
  upto.forEach((r) => (ROLE_GRANTS[r] || []).forEach((a) => set.add(a)));
  return set;
}
// The current user's effective capabilities in the active organisation.
// This is the client-side mirror of `GET /v1/me/capabilities`; the UI gates on
// this and nothing else — never on a role string.
function currentRole() {
  const o = ORG_BY_SLUG[ACTIVE_ORG];
  return o ? { role: o.role === "Owner" ? "super_admin" : o.role.toLowerCase() } : { role: "viewer" };
}
// The user's role in the active organisation, or the previewed role.
function currentRoleName() {
  return VIEWER_ROLE || currentRole().role;
}
function currentCaps() {
  return capabilitiesForRole(currentRoleName());
}
// can(action) — the one place the UI asks. Scope defaults to the action's own
// scope; pass one to ask about a narrower resource (devices:imei:X).
function can(action) {
  return currentCaps().has(action);
}

// Which capability a page requires to open at all. A view not listed is open to
// any member (Home, Profile — those are personal, not org-scoped). This is the
// route-level gate; it applies only where a role context exists.
const VIEW_REQUIRES = {
  devices: "device.read",
  apps: "apps.read",
  telemetry: "telemetry.read",
  alerts: "alert.read",
  logs: "logs.read",
  terminal: "shell.execute",
  updates: "update.read",
  permissions: "members.read",
  "api-keys": "keys.manage",
  settings: "settings.read",
};
function canView(view) {
  const a = VIEW_REQUIRES[view];
  return !a || can(a);
}
function requiredAction(view) {
  const a = VIEW_REQUIRES[view];
  return a ? ACTIONS.find((x) => x.action === a) : null;
}

// A gated control: when the user lacks `action`, render the label disabled
// with a reason instead of hiding it — a control you cannot use is clearer than
// one that is simply missing. `opts.id` keeps the control's wiring id.
function canDo(action, label, cls, opts) {
  const ok = can(action);
  const reason = (ACTIONS.find((x) => x.action === action) || {}).label || action;
  const id = opts && opts.id ? ` id="${opts.id}"` : "";
  const title = ok ? "" : ` title="Your role can't ${reason.toLowerCase()}"`;
  return `<button class="btn ${cls || "btn--primary"}${ok ? "" : " is-disabled"}"${id}${ok ? "" : ' disabled aria-disabled="true"'}${title}>${label}</button>`;
}

const MEMBERS = [
  { name: "Oscar Porter",  email: "op@acme.io",    role: "super_admin", status: "active",  mfa: true,  last: "12:41",       joined: "2026-03-02" },
  { name: "Nina Grzelak",  email: "nina@acme.io",  role: "admin",       status: "active",  mfa: true,  last: "2 hours ago", joined: "2026-04-18" },
  { name: "Sam Okafor",    email: "sam@acme.io",   role: "operator",    status: "active",  mfa: false, last: "Yesterday",   joined: "2026-05-30" },
  { name: "Lee Chan",      email: "lee@globex.io", role: "viewer",      status: "active",  mfa: false, last: "12 days ago", joined: "2026-06-11" },
  { name: "svc-ci",        email: "—",             role: "operator",    status: "service", mfa: false, last: "12:38",       joined: "2026-06-01" },
  { name: "Dana Feld",     email: "dana@acme.io",  role: "viewer",      status: "suspended", mfa: false, last: "2026-09-02", joined: "2026-07-22" },
];

// The review matrix is derived from the same catalog, so the page that explains
// permissions cannot drift from the gate that enforces them.
const CAPABILITIES = ACTIONS.map((a) => ({
  key: a.action,
  label: a.label,
  group: a.group,
  roles: ROLES.filter((r) => capabilitiesForRole(r).has(a.action)),
}));

// The user's role can be previewed as any role, so the gated states can be
// designed without signing in as each. null = use the org's real role.
let VIEWER_ROLE = null;

const INVITATIONS = [
  { email: "ravi@acme.io",   role: "operator", invited: "2026-11-12", by: "Oscar Porter", expires: "in 5 days",  status: "pending" },
  { email: "maya@acme.io",   role: "viewer",   invited: "2026-11-09", by: "Nina Grzelak", expires: "in 2 days",  status: "pending" },
  { email: "theo@acme.io",   role: "admin",    invited: "2026-10-30", by: "Oscar Porter", expires: "expired",    status: "expired" },
  { email: "jules@acme.io",  role: "operator", invited: "2026-10-21", by: "Nina Grzelak", expires: "—",          status: "accepted" },
];

const PERM_AUDIT = [
  { at: "12:38",       actor: "Oscar Porter", action: "granted role",  target: "svc-ci",    detail: "operator",          by: "super_admin" },
  { at: "Yesterday",   actor: "Oscar Porter", action: "revoked role",  target: "jules@acme.io", detail: "admin → viewer", by: "super_admin" },
  { at: "2026-11-12",  actor: "Oscar Porter", action: "invited",       target: "ravi@acme.io",  detail: "operator",      by: "super_admin" },
  { at: "2026-11-09",  actor: "Nina Grzelak", action: "invited",       target: "maya@acme.io",  detail: "viewer",        by: "admin" },
  { at: "2026-09-02",  actor: "system",       action: "suspended",     target: "dana@acme.io",  detail: "no login for 60 days", by: "—" },
  { at: "2026-07-22",  actor: "Oscar Porter", action: "changed MFA",   target: "sam@acme.io",   detail: "required → optional", by: "super_admin" },
];

const ALERTS = [
  { sev: "err",  rule: "battery_critical",  device: "354481100123453", msg: "Battery 4% — below 10% threshold", at: "12:40", state: "firing" },
  { sev: "err",  rule: "device_offline",    device: "354481100123456", msg: "No telemetry for 22m",             at: "12:39", state: "firing" },
  { sev: "warn", rule: "thermal_high",      device: "354481100123454", msg: "Thermal zone 'hot' for 6m",         at: "12:37", state: "firing" },
  { sev: "warn", rule: "risk_score",        device: "354481100123452", msg: "Risk 38 (> 30)",                    at: "12:31", state: "resolved" },
  { sev: "info", rule: "fw_update_available", device: "354481100123454", msg: "0.4.2 available (on 0.4.1)",       at: "12:20", state: "resolved" },
];

const LOGS = [
  { t: "12:41:22.114", lvl: "info",  svc: "ws-hub",     msg: "device connected",                          imei: "354481100123451", req: "—",          trace: "9f2a1c4b8e7d0a31", span: "hub.connect",   dur: "—",     region: "eu-west-1", host: "api-3", fields: { org: "Acme Ops", session: "ws_7c41a9", rssi: "-58 dBm" } },
  { t: "12:41:12.660", lvl: "info",  svc: "http",       msg: "POST /graphql 200",                         imei: "—",               req: "req_9f2a1b", trace: "9f2a1c4b8e7d0a30", span: "http.request",  dur: "24ms",  region: "eu-west-1", host: "api-1", fields: { method: "POST", path: "/graphql", status: 200, bytes: "1.4 KB" } },
  { t: "12:41:08.902", lvl: "info",  svc: "command",    msg: "dispatch cmd_8f21 reboot → 35…451",          imei: "354481100123451", req: "req_9f2a1c", trace: "9f2a1c4b8e7d0a2f", span: "cmd.dispatch",  dur: "18ms",  region: "eu-west-1", host: "api-3", fields: { cmd: "cmd_8f21", type: "reboot", hmac: "verified" } },
  { t: "12:41:05.221", lvl: "info",  svc: "http",       msg: "GET /v1/device/354481100123452/status 200",  imei: "354481100123452", req: "req_9f2a17", trace: "9f2a1c4b8e7d0a2e", span: "http.request",  dur: "6ms",   region: "eu-west-1", host: "api-2", fields: { method: "GET", path: "/v1/device/…/status", status: 200 } },
  { t: "12:40:55.031", lvl: "warn",  svc: "rate-limit", msg: "bucket exhausted for 10.4.2.19",             imei: "—",               req: "req_9f2a19", trace: "9f2a1c4b8e7d0a2d", span: "ratelimit.check", dur: "1ms", region: "eu-west-1", host: "api-1", fields: { bucket: "10.4.2.19", limit: "120/min", retry_after: "12s" } },
  { t: "12:39:12.004", lvl: "warn",  svc: "auth",       msg: "failed login for op@globex.io (attempt 2/5)", imei: "—",              req: "req_9f29ff", trace: "9f2a1c4b8e7d0a2c", span: "auth.login",    dur: "12ms",  region: "eu-west-1", host: "api-1", fields: { user: "op@globex.io", attempt: "2/5", ip: "203.0.113.7" } },
  { t: "12:38:44.010", lvl: "error", svc: "db",         msg: "sqlite busy: retrying WAL write (1/3)",      imei: "—",               req: "req_9f29fe", trace: "9f2a1c4b8e7d0a2b", span: "db.write",      dur: "140ms", region: "eu-west-1", host: "api-1", fields: { db: "telemetry.sqlite", op: "wal_write", retry: "1/3" } },
  { t: "12:38:02.331", lvl: "info",  svc: "ws-hub",     msg: "device disconnected",                        imei: "354481100123456", req: "—",          trace: "9f2a1c4b8e7d0a2a", span: "hub.disconnect", dur: "—",    region: "eu-west-1", host: "api-3", fields: { org: "Globex", reason: "timeout", after: "22m" } },
  { t: "12:36:19.887", lvl: "debug", svc: "cache",      msg: "evicted 128 stale entries (ttl)",            imei: "—",               req: "—",          trace: "9f2a1c4b8e7d0a29", span: "cache.evict",   dur: "2ms",   region: "eu-west-1", host: "api-2", fields: { evicted: 128, policy: "ttl", ttl: "300s" } },
  { t: "12:36:02.114", lvl: "info",  svc: "command",    msg: "acked cmd_8f20 start_capture by 35…452",     imei: "354481100123452", req: "req_9f29f1", trace: "9f2a1c4b8e7d0a28", span: "cmd.ack",       dur: "31ms",  region: "eu-west-1", host: "api-3", fields: { cmd: "cmd_8f20", type: "start_capture" } },
  { t: "12:35:40.400", lvl: "error", svc: "hmac",       msg: "signature mismatch cmd_8f1e (device 35…453)", imei: "354481100123453", req: "req_9f2a04", trace: "9f2a1c4b8e7d0a27", span: "hmac.verify",  dur: "3ms",   region: "eu-west-1", host: "api-1", fields: { cmd: "cmd_8f1e", algo: "HMAC-SHA256", expected: "a91f…", got: "0c44…" } },
  { t: "12:33:48.559", lvl: "warn",  svc: "ota",        msg: "apk checksum slow (2.1s) for canary",        imei: "—",               req: "req_9f29e0", trace: "9f2a1c4b8e7d0a26", span: "ota.checksum",  dur: "2100ms", region: "eu-west-1", host: "api-2", fields: { channel: "canary", version: "0.4.2", sha256: "3b7e…" } },
  { t: "12:20:11.220", lvl: "info",  svc: "ota",        msg: "manifest served v0.4.2 (312 KB)",            imei: "—",               req: "req_9f2a01", trace: "9f2a1c4b8e7d0a25", span: "ota.manifest",  dur: "9ms",   region: "eu-west-1", host: "api-2", fields: { version: "0.4.2", size: "312 KB", channel: "stable" } },
  { t: "12:19:58.007", lvl: "debug", svc: "graphql",    msg: "subscription telemetryStream opened (org acme)", imei: "—",            req: "req_9f2a00", trace: "9f2a1c4b8e7d0a24", span: "gql.subscribe", dur: "—",     region: "eu-west-1", host: "api-3", fields: { org: "acme", sub: "telemetryStream", ops: 1 } },
  { t: "12:18:44.512", lvl: "info",  svc: "deploy",     msg: "document signed for 35…451 (version 12)",    imei: "354481100123451", req: "req_9f29f7", trace: "9f2a1c4b8e7d0a23", span: "deploy.sign",   dur: "8ms",   region: "eu-west-1", host: "api-1", fields: { version: 12, member: "oscar.porter", algo: "Ed25519" } },
  { t: "12:17:31.088", lvl: "debug", svc: "cache",      msg: "warm 42 keys from origin",                   imei: "—",               req: "—",          trace: "9f2a1c4b8e7d0a22", span: "cache.warm",    dur: "44ms",  region: "eu-west-1", host: "api-2", fields: { keys: 42, source: "origin" } },
  { t: "12:16:05.770", lvl: "error", svc: "ws-hub",     msg: "backpressure: dropped 3 output frames",      imei: "354481100123454", req: "—",          trace: "9f2a1c4b8e7d0a21", span: "hub.outbox",    dur: "—",     region: "eu-west-1", host: "api-3", fields: { dropped: 3, outbox: 256, session: "pty_2f90be" } },
];

// ---------------------------------------------------------------- terminal
// One entry per PTY session. Mirrors the server's session model: a session id,
// the device it runs on, a connection state (live / reattached / detached), the
// resolved capability, and the transcript. See DOCUMENTATION/CONTAINER_TERMINAL.md.
const TERM_SESSIONS = [
  {
    id: "pty_7c41a9", tab: "451", imei: "354481100123451", model: "Vyzorix AR-1", fw: "0.4.2",
    state: "live", opened: "12:38:02", idle: "0s", cols: 96, rows: 26,
    user: "oscar.porter", org: "Acme Ops", role: "Owner", grant: "shell.execute",
    tier: "extended", expires: "in 1h 58m", recording: "on · 41 KB", tmux: "451",
    transcript: [
      ["banner", "Vyzorix container terminal · proot -0 · Debian GNU/Linux 12 (bookworm) aarch64"],
      ["banner", "session pty_7c41a9 · device 354481100123451 · grant shell.execute · user oscar.porter"],
      ["banner", "detach / reattach: on (tmux \"451\") · idle timeout 15m"],
      ["blank", ""],
      ["prompt", "<i>root@vyzorix:/#</i> uname -a"],
      ["out", "Linux vyzorix 6.6.30-android14-gki aarch64 GNU/Linux"],
      ["prompt", "<i>root@vyzorix:/#</i> cat /etc/vyzorix/container.conf"],
      ["out", "VYZORIX_CONTAINER=ar-1"],
      ["out", "VYZORIX_BRIDGE_SOCKET=/tmp/android.sock"],
      ["out", "VYZORIX_BRIDGE_TOKEN=<span class=\"term-secret\">••••••••</span>  (per-session capability)"],
      ["prompt", "<i>root@vyzorix:/#</i> android-bin capabilities --tier extended"],
      ["ok", "camera.capture          granted"],
      ["ok", "location.read           granted"],
      ["ok", "notifications.post      granted"],
      ["err", "telephony.call          denied    tier \"base\" &lt; \"extended\""],
      ["prompt", "<i>root@vyzorix:/#</i> ls -lh /data/captures"],
      ["out", "-rw-r--r-- 1 root root  40K Nov 14 12:39 capture_8f20.pcm"],
      ["out", "-rw-r--r-- 1 root root  39K Nov 14 12:41 capture_8f21.pcm"],
      ["prompt", "<i>root@vyzorix:/#</i> df -h /"],
      ["out", "Filesystem      Size  Used Avail Use% Mounted on"],
      ["out", "/dev/block/dm-4  48G  6.2G   41G  14% /"],
      ["prompt", "<i>root@vyzorix:/#</i> "],
    ],
  },
  {
    id: "pty_2f90be", tab: "454", imei: "354481100123454", model: "Vyzorix AR-2", fw: "0.4.1",
    state: "reattached", opened: "12:31:44", idle: "3s", cols: 96, rows: 26,
    user: "oscar.porter", org: "Acme Ops", role: "Owner", grant: "shell.execute",
    tier: "extended", expires: "in 1h 51m", recording: "on · 12 KB", tmux: "454",
    transcript: [
      ["banner", "reattached to pty_2f90be after a 4.2s socket drop — shell preserved, output buffered"],
      ["banner", "session pty_2f90be · device 354481100123454 · tmux \"454\""],
      ["blank", ""],
      ["prompt", "<i>root@vyzorix:/#</i> systemctl status vyzorix-agent --no-pager"],
      ["out", "● vyzorix-agent.service - Vyzorix device agent"],
      ["out", "     Loaded: loaded (/lib/systemd/system/vyzorix-agent.service; enabled)"],
      ["out", "     Active: active (running) since Fri 12:31:40 UTC; 6min ago"],
      ["prompt", "<i>root@vyzorix:/#</i> journalctl -u vyzorix-agent -n 3 --no-pager"],
      ["out", "12:31:44 agent bridge listening on /tmp/android.sock"],
      ["warn", "12:33:02 thermal zone 'hot' — throttling camera pipeline"],
      ["out", "12:38:02 heartbeat ok · rssi -83 dBm · battery 52%"],
      ["prompt", "<i>root@vyzorix:/#</i> "],
    ],
  },
  {
    id: "pty_1a08d3", tab: "453", imei: "354481100123453", model: "Vyzorix AR-2", fw: "0.3.9",
    state: "detached", opened: "12:22:10", idle: "22m", cols: 80, rows: 24,
    user: "nina.grzelak", org: "Acme Ops", role: "Editor", grant: "shell.execute",
    tier: "base", expires: "expired", recording: "on · 88 KB", tmux: "453",
    transcript: [
      ["banner", "session pty_1a08d3 detached 12:22:10 — device offline (no telemetry for 22m)"],
      ["banner", "output buffered on the device up to the bound · reattach by the same session id"],
      ["blank", ""],
      ["warn", "device 354481100123453 unreachable — 0 frames in 22m"],
      ["err", "relay: session kept alive; shell still running (tmux \"453\")"],
      ["banner", "press Reconnect to reattach, or close the tab to reap the shell"],
    ],
  },
];

const TERM_RECORDINGS = [
  { id: "rec_7c41a9", session: "pty_7c41a9", imei: "354481100123451", user: "oscar.porter", dur: "3m 42s", size: "41 KB", at: "12:41", state: "recording" },
  { id: "rec_2f90be", session: "pty_2f90be", imei: "354481100123454", user: "oscar.porter", dur: "1m 08s", size: "12 KB", at: "12:32", state: "complete" },
  { id: "rec_1a08d3", session: "pty_1a08d3", imei: "354481100123453", user: "nina.grzelak", dur: "9m 51s", size: "88 KB", at: "12:22", state: "detached" },
  { id: "rec_5b02ef", session: "pty_5b02ef", imei: "354481100123452", user: "sam.okafor",   dur: "6m 17s", size: "63 KB", at: "11:58", state: "complete" },
];

// ---------------------------------------------------------------- updates
// Versions are synced from GitHub releases; there is no in-console upload (the
// APK is built and released outside the console). A "push" tells devices to
// download, verify the SHA-256 and hand the APK to the system installer — the
// last tap is the user's. See DOCUMENTATION/APK_DISTRIBUTION.md, UPDATE_SYSTEM.md.
const UPDATES = [
  { version: "0.2.9", code: 19, type: "stable", filename: "vyzorix-app-v0.2.9.apk", size: "64.1 MB", sha: "9f2a1c4b8e7d0a31c6f4b2e8a1d05c73e6b9f4a2d8c10e5b7a3f6d9c2e14b8a70", released: "2026-11-11", latest: true, notes: "PTY detach/reattach hardening; tmux session naming." },
  { version: "0.2.8", code: 18, type: "beta", filename: "vyzorix-app-v0.2.8.apk", size: "63.4 MB", sha: "3b7e0c1d9a4f2e6b8c05d3a7f1e9b4c2a6d80f3e5b1c9a7d4f2e6b0c8a3d5f19", released: "2026-11-04", latest: false, notes: "Bridge socket retry on cold start." },
  { version: "0.2.7", code: 17, type: "stable", filename: "vyzorix-app-v0.2.7.apk", size: "63.1 MB", sha: "a91f4e2c7b0d5a3f8e6c1b9d4a07f2e5c8b3d6a1f9e4c7b2d5a0f8e3c6b1d4a9", released: "2026-10-27", latest: false, notes: "Deploy document signature check on apply." },
  { version: "0.2.6", code: 16, type: "internal", filename: "vyzorix-app-v0.2.6.apk", size: "62.8 MB", sha: "0c44b8a1f6e3d9c2b5a7f4e1d8c0b3a6f9e2d5c8b1a4f7e0d3c6b9a2f5e8d1c4", released: "2026-10-18", latest: false, notes: "Telemetry frame batching; rootfs asset pin." },
];

const UPDATE_PUSHES = [
  { id: "push_4b91", version: "0.2.9", type: "immediate", state: "in_progress", devices: 12, done: 8, failed: 0, started: "12:38", by: "oscar.porter" },
  { id: "push_4b88", version: "0.2.8", type: "scheduled", state: "completed",   devices: 6,  done: 6, failed: 0, started: "11:02", by: "nina.grzelak" },
  { id: "push_4b80", version: "0.2.7", type: "deferred",  state: "completed",   devices: 9,  done: 8, failed: 1, started: "2026-10-28", by: "oscar.porter" },
  { id: "push_4b7c", version: "0.2.6", type: "immediate", state: "cancelled",   devices: 4,  done: 1, failed: 0, started: "2026-10-18", by: "sam.okafor" },
];

// The per-device update view: the latest version (or the one being pushed) with
// each device's check/download/verify/install state. This is what replaces the
// adoption framing — one version, device by device.
const UPDATE_DEVICES = [
  { imei: "354481100123451", model: "Vyzorix AR-1", current: 19, latest: 19, state: "up_to_date", progress: 100, at: "12:41" },
  { imei: "354481100123454", model: "Vyzorix AR-2", current: 18, latest: 19, state: "installing",  progress: 78,  at: "12:40" },
  { imei: "354481100123452", model: "Vyzorix AR-1", current: 18, latest: 19, state: "downloading", progress: 42,  at: "12:40" },
  { imei: "354481100123455", model: "Vyzorix AR-1", current: 19, latest: 19, state: "up_to_date", progress: 100, at: "12:39" },
  { imei: "354481100123456", model: "Vyzorix AR-2", current: 16, latest: 19, state: "update_available", progress: 0, at: "11:58" },
  { imei: "354481100123453", model: "Vyzorix AR-2", current: 16, latest: 19, state: "offline", progress: 0, at: "12:22" },
];

// Live view state: which sessions are open and which tab is active. Mutated by
// the terminal's own controls so the view survives a re-render.
let TERM_LIST = TERM_SESSIONS.slice();
let TERM_ACTIVE = TERM_LIST[0].id;

const TERM_SEED_OUTPUT = [
  "cd /data && ls",
  "Vyzorix container shell — type 'help' for android-bin, or 'exit' to end the session.",
];

// ---------------------------------------------------------------- container files
// The container's filesystem, exactly as `proot` presents it: a real Debian tree
// at `/`, with the host trees bound in at the paths `ProotLauncher.binds()` names.
// See DOCUMENTATION/CONTAINER_FILESYSTEM.md. `root: true` marks a node whose
// changes are lost on a container update (the rootfs); the overlay and
// /home/apps survive one.
const CONTAINER_FS = {
  name: "/", type: "dir", note: "Debian rootfs (files/rootfs)", root: true, children: [
    { name: "bin", type: "dir", note: "Debian coreutils, base system" },
    { name: "boot", type: "dir" },
    { name: "dev", type: "dir", note: "bound from the device", bind: true },
    { name: "etc", type: "dir", children: [
      { name: "vyzorix", type: "dir", note: "overlay config — survives a rootfs swap", writable: true, children: [
        { name: "container.conf", type: "file", size: "312 B", mode: "-rw-r--r--" },
        { name: "agent.toml", type: "file", size: "1.1 KB", mode: "-rw-r--r--" },
      ] },
      { name: "resolv.conf", type: "file", size: "34 B", mode: "-rw-r--r--", note: "bound from the rootfs" },
      { name: "passwd", type: "file", size: "1.9 KB", mode: "-rw-r--r--" },
      { name: "apt", type: "dir", children: [
        { name: "sources.list", type: "file", size: "2.4 KB", mode: "-rw-r--r--" },
      ] },
    ] },
    { name: "home", type: "dir", children: [
      { name: "apps", type: "dir", note: "Layer 3 app storage — survives a rootfs swap", writable: true, children: [
        { name: "audio-router", type: "dir", children: [
          { name: "app.json", type: "file", size: "840 B", mode: "-rw-r--r--" },
          { name: ".env", type: "file", size: "512 B", mode: "-rw-------", secret: true, note: "app secrets, 0600 — do not copy" },
          { name: "logs", type: "dir", children: [
            { name: "router.log", type: "file", size: "18.2 KB", mode: "-rw-r--r--" },
          ] },
        ] },
        { name: "supervisor.sock", type: "file", size: "0 B", mode: "srw-------", note: "supervisor control socket" },
      ] },
    ] },
    { name: "lib", type: "dir" }, { name: "lib64", type: "dir" },
    { name: "mnt", type: "dir" }, { name: "media", type: "dir" },
    { name: "opt", type: "dir", note: "overlay — on-device installs belong here", writable: true, children: [
      { name: "vyzorix-tools", type: "dir", children: [
        { name: "README", type: "file", size: "1.2 KB", mode: "-rw-r--r--" },
      ] },
    ] },
    { name: "proc", type: "dir", note: "bound from the device", bind: true },
    { name: "root", type: "dir" },
    { name: "run", type: "dir" },
    { name: "sbin", type: "dir" },
    { name: "sdcard", type: "dir", note: "device shared storage (Download, DCIM, …)", bind: true, children: [
      { name: "Download", type: "dir", children: [{ name: "capture_8f21.pcm", type: "file", size: "39 KB", mode: "-rw-r--r--" }] },
      { name: "DCIM", type: "dir" }, { name: "Pictures", type: "dir" },
      { name: "Music", type: "dir" }, { name: "Movies", type: "dir" },
      { name: "Documents", type: "dir" }, { name: "Android", type: "dir", children: [
        { name: "media", type: "dir", children: [{ name: "com.vyzorix", type: "dir" }] },
      ] },
    ] },
    { name: "srv", type: "dir" }, { name: "sys", type: "dir", note: "bound from the device", bind: true },
    { name: "tmp", type: "dir", children: [
      { name: "android.sock", type: "file", size: "0 B", mode: "srw-------", note: "the bridge socket — the door into Layer 1" },
      { name: "tmux-0", type: "dir", note: "tmux's socket dir for the shell" },
    ] },
    { name: "usr", type: "dir", children: [
      { name: "bin", type: "dir" }, { name: "lib", type: "dir" }, { name: "share", type: "dir" },
      { name: "local", type: "dir", note: "overlay — survives a rootfs swap", writable: true, children: [
        { name: "bin", type: "dir", children: [{ name: "android-bin", type: "file", size: "2.4 MB", mode: "-rwxr-xr-x", note: "in-container client for the bridge" }] },
      ] },
    ] },
    { name: "var", type: "dir", children: [
      { name: "log", type: "dir", children: [{ name: "apt", type: "dir" }] },
      { name: "lib", type: "dir" },
    ] },
    { name: "android", type: "dir", note: "the device's read-only trees, under a prefix", readOnly: true, children: [
      { name: "system", type: "dir" }, { name: "vendor", type: "dir" }, { name: "product", type: "dir" },
    ] },
  ],
};

const ORGS = [
  { slug: "acme-ops",  name: "Acme Ops",  plan: "Scale", members: 14, devices: 128, apps: 6, role: "Owner",  status: "active",    region: "eu-west-1", owner: "Oscar Porter" },
  { slug: "northwind", name: "Northwind", plan: "Pro",   members: 6,  devices: 42,  apps: 3, role: "Admin",  status: "active",    region: "eu-west-1", owner: "Nina Grzelak" },
  { slug: "globex",    name: "Globex",    plan: "Free",  members: 2,  devices: 9,   apps: 0, role: "Viewer", status: "suspended", region: "us-east-1", owner: "Lee Chan" },
];
const ORG_BY_SLUG = Object.fromEntries(ORGS.map((o) => [o.slug, o]));

// ---------------------------------------------------------------- apps
// Apps are what the console hosts inside a device's container (Layer 3). A KiCad
// build ported to the browser and made Vyzorix-aware is the flagship example.
// Status/actions mirror the supervisor's ops: start / stop / restart / logs /
// expose (APP_RUNTIME.md, APP_EXPOSURE.md). None are deployed by default.
const APP_TEMPLATES = [
  { id: "kicad",   name: "KiCad (browser)", tag: "eda",     icon: "chip",   desc: "KiCad's schematic & PCB editor, rebuilt to run in the browser and talk to a Vyzorix device — open, review and edit boards from the console.", expose: "lan", port: 8080 },
  { id: "router",  name: "Audio router",    tag: "audio",   icon: "wave",   desc: "Route the device's audio streams between capture and output.", expose: "loopback", port: 4700 },
  { id: "files",   name: "File browser",    tag: "storage", icon: "folder", desc: "Browse and transfer the container's filesystem over the console.", expose: "lan", port: 8081 },
  { id: "studio",  name: "Sensor studio",   tag: "data",    icon: "chart",  desc: "Live plots of telemetry frames with export.", expose: "loopback", port: 4900 },
  { id: "custom",  name: "Custom app",      tag: "—",       icon: "box",    desc: "Deploy any app from a manifest: a command, a port and a tier.", expose: "loopback", port: 3000 },
];

const APPS = [
  { id: "app_kicad", name: "KiCad (browser)", version: "8.0.4-vx1", template: "kicad", device: "354481100123451", status: "running", expose: "lan", port: 8080, tier: "extended", uptime: "3d 2h", url: "http://127.0.0.1:8080/?token=…" },
  { id: "app_router", name: "Audio router", version: "1.4.0", template: "router", device: "354481100123451", status: "running", expose: "loopback", port: 4700, tier: "base", uptime: "6d 4h", url: "—" },
  { id: "app_files", name: "File browser", version: "0.9.2", template: "files", device: "354481100123454", status: "stopped", expose: "lan", port: 8081, tier: "base", uptime: "—", url: "—" },
  { id: "app_studio", name: "Sensor studio", version: "0.3.0", template: "studio", device: "354481100123454", status: "crashed", expose: "loopback", port: 4900, tier: "extended", uptime: "—", url: "—" },
];

// ---------------------------------------------------------------- home
// The logged-in homepage layout:
//   HomePageFrame  -> time-based greeting + "Welcome to <edition>." subtitle
//   HeaderActions  -> "Firing alerts" / "Active incidents" pills
//   HomeGrid(2)    -> a Dashboards tabbed card + an Alerts & incidents tabbed card
//   GetStarted     -> a grid of "Recommended getting started guides" cards
// Each guide carries the capability it needs, so a role that cannot do the
// thing does not see a card inviting them to try.
const HOME_GUIDES = [
  { id: "device",   icon: "device",   color: "#fb7185", title: "Register a device", desc: "Onboard an Android phone and connect it to your organisation.", cta: "Register", go: "devices",     action: "device.write" },
  { id: "app",      icon: "box",      color: "#73bf69", title: "Deploy an app",     desc: "Host an app — like the browser KiCad — inside a device's container.", cta: "Deploy", go: "apps", action: "apps.write" },
  { id: "terminal", icon: "terminal", color: "#ff9830", title: "Open a terminal",   desc: "Get a shell inside the container, gated by shell.execute.", cta: "Open", go: "terminal",       action: "shell.execute" },
  { id: "update",   icon: "rocket",   color: "#b877d9", title: "Push an update",    desc: "Roll the latest signed build out to your devices.", cta: "Push", go: "updates",               action: "update.write" },
  { id: "alert",    icon: "bell",     color: "#fade2a", title: "Set up an alert",   desc: "Fire on battery, thermal, risk and offline.", cta: "Create", go: "alerts",                 action: "alert.write" },
  { id: "member",   icon: "user",     color: "#fb7185", title: "Invite a teammate", desc: "Grant access to this organisation with a role.", cta: "Invite", go: "permissions",           action: "members.write" },
];

const HOME_DASH_TABS = [
  { id: "recent", label: "Recent", items: [
    { name: "Overview", kind: "Dashboard", folder: "General",    at: "12:41", go: "home" },
    { name: "OTA adoption",   kind: "Dashboard", folder: "Operations", at: "12:38", go: "updates" },
    { name: "Devices",        kind: "Page",      folder: "Devices",    at: "12:36", go: "devices" },
  ] },
  { id: "most-used", label: "Most used", items: [
    { name: "Overview", kind: "Dashboard", folder: "General",    go: "home" },
    { name: "Logs",           kind: "Page",      folder: "Operations", go: "logs" },
    { name: "Terminal",       kind: "Page",      folder: "Operations", go: "terminal" },
  ] },
  { id: "starred", label: "Starred", items: [
    { name: "Overview", kind: "Dashboard", folder: "General",    go: "home" },
    { name: "Alert activity", kind: "Dashboard", folder: "Operations", go: "alerts" },
  ] },
];

const HOME_ALERT_TABS = [
  { id: "firing", label: "Firing alerts", items: [
    { sev: "critical", rule: "battery_critical", device: "35…453", at: "12:40" },
    { sev: "critical", rule: "device_offline",   device: "35…456", at: "12:39" },
    { sev: "high",     rule: "thermal_high",     device: "35…454", at: "12:37" },
  ] },
  { id: "incidents", label: "Incidents", items: [
    { sev: "critical", rule: "Northwind devices offline",     device: "2 devices", at: "12:39" },
    { sev: "high",     rule: "Thermal throttling on 35…454",  device: "1 device",  at: "12:37" },
  ] },
];

const SESSIONS = [
  { active: true,  seen: "Now",          loggedOn: "September 12, 2026", ip: "10.4.2.19",    browser: "Chrome 153", os: "Linux",           idp: "vyzorix" },
  { active: false, seen: "2 hours ago",  loggedOn: "September 12, 2026", ip: "10.4.2.51",    browser: "Safari 18",  os: "macOS 15.1",    idp: "google" },
  { active: false, seen: "Yesterday",    loggedOn: "September 11, 2026", ip: "203.0.113.7",  browser: "Firefox 132", os: "Windows 11",   idp: "github" },
  { active: false, seen: "3 days ago",   loggedOn: "September 9, 2026",  ip: "198.51.100.24", browser: "Chrome 152", os: "Android 15",   idp: "google" },
];

const KEYS = [
  { id: "key_9c2a", client: "ci-deploy",       scopes: "devices:write, ota:read", created: "2026-08-01", last: "12:38", status: "active" },
  { id: "key_7b10", client: "readonly", scopes: "metrics:read",            created: "2026-07-14", last: "12:41", status: "active" },
  { id: "key_5d33", client: "legacy-sync",     scopes: "devices:read",            created: "2026-03-02", last: "—",     status: "revoked" },
];

// ---------------------------------------------------------------- helpers
const sparkline = (values, color) => window.VyzCharts.sparkline(values, { color });
const statusDot = (s) => `<span class="dot dot--${s === "online" ? "ok" : "off"}"></span>`;
// Badges are neutral everywhere. `pill(label, dot)` adds a semantic dot so a
// state is still readable without a coloured pill; omit `dot` for a plain pill.
function pill(label, dot) {
  return `<span class="badge">${dot ? `<span class="dot dot--${dot}" style="margin-right:4px"></span>` : ""}${label}</span>`;
}
const riskBadge = (r) => r > 70 ? pill(r, "err") : r > 35 ? pill(r, "warn") : pill(r, "ok");
const sevBadge = (s) => s === "err" ? pill("error", "err") : s === "warn" ? pill("warning", "warn") : pill("info", "ok");
function bar(pct, tone) { return `<div class="bar bar--${tone}"><i style="width:${pct}%"></i></div>`; }

function statPanel(title, value, label, delta, tone, spark) {
  return `<div class="panel"><div class="panel__head"><span class="panel__title">${title}</span><span class="panel__menu" data-ui="ellipsis" data-ui-size="16"></span></div>
    <div class="panel__body">
      <div class="stat__value">${value}</div>
      <div class="stat__label">${label}</div>
      ${delta ? `<div class="stat__delta ${tone}">${delta}</div>` : ""}
      ${spark ? `<div class="stat__chart">${sparkline(spark, tone === "down" ? "#f2495c" : "#73bf69")}</div>` : ""}
    </div></div>`;
}

// ---------------------------------------------------------------- views
const VIEWS = {
  home() {
    const h = new Date().getHours();
    const greeting = h >= 5 && h <= 11 ? "Good morning." : h >= 12 && h <= 17 ? "Good afternoon." : "Good evening.";
    const org = ORG_BY_SLUG[ACTIVE_ORG] || null;
    const firing = HOME_ALERT_TABS[0].items.length;
    const incidents = HOME_ALERT_TABS[1].items.length;

    // A tabbed card: a tab bar over a
    // fixed-height body, plus a footer that links out.
    const tabbedCard = (title, icon, tabs, idPrefix, footer) => `
      <section class="panel g-card">
        <div class="g-card__head">
          <span class="panel__title"><span data-ui="${icon}" data-ui-size="16"></span> ${title}</span>
        </div>
        <div class="g-tabs" data-g-tabs="${idPrefix}">
          ${tabs.map((t, i) => `<button class="g-tab${i === 0 ? " is-active" : ""}" data-g-tab="${t.id}">${t.label}<span class="g-tab__n">${t.items.length}</span></button>`).join("")}
        </div>
        <div class="g-card__body">
          ${tabs.map((t, i) => `<ul class="g-rows" data-g-panel="${t.id}"${i === 0 ? "" : " hidden"}>${t.items.map((it) => rowFor(it, idPrefix)).join("")}</ul>`).join("")}
        </div>
        <div class="g-card__foot">${footer}</div>
      </section>`;

    const rowFor = (it, idPrefix) => {
      if (idPrefix === "dash") {
        return `<li class="g-row" data-go="${it.go}">
          <span class="g-row__icon" data-ui="${it.kind === "Page" ? "book" : "chart"}" data-ui-size="16"></span>
          <span class="g-row__name">${it.name}</span>
          <span class="g-row__folder">${it.folder}</span>
          ${it.at ? `<span class="g-row__at muted">${it.at}</span>` : ""}
        </li>`;
      }
      return `<li class="g-row" data-go="alerts">
        <span class="g-row__sev g-row__sev--${it.sev}"></span>
        <span class="g-row__name mono">${it.rule}</span>
        <span class="g-row__folder">${it.device}</span>
        <span class="g-row__at muted">${it.at}</span>
      </li>`;
    };

    return `
      <div class="g-home">
        <header class="g-home__greet">
          <h1>${greeting}</h1>
          <p>Welcome to ${org ? org.name : "Vyzorix"}. ${org ? `You're ${currentRoleName() === "admin" || currentRoleName() === "operator" ? "an" : "a"} <b>${currentRoleName().replace("_", " ")}</b> here.` : "Select an organisation to begin."}</p>
        </header>

        <div class="g-home__header">
          ${can("alert.read") ? `<button class="g-pill" data-go="alerts"><span data-ui="bell" data-ui-size="14"></span> Firing alerts <b>${firing}</b></button>
          <button class="g-pill" data-go="alerts"><span data-ui="bell" data-ui-size="14"></span> Active incidents <b>${incidents}</b></button>` : ""}
        </div>

        <div class="g-home__ask">
          <span class="g-home__ask-icon" data-ui="help" data-ui-size="16"></span>
          <input class="g-home__ask-input" placeholder="Ask Vyzorix about your devices, or search for a device, app or dashboard…" />
          <button class="btn btn--primary tiny">Ask</button>
        </div>

        <div class="g-home__grid">
          ${tabbedCard("Dashboards", "chart", HOME_DASH_TABS, "dash", '<button class="btn btn--ghost tiny" data-go="home">View all dashboards</button>')}
          ${can("alert.read") ? tabbedCard("Alerts & incidents", "bell", HOME_ALERT_TABS, "alerts", '<button class="btn btn--ghost tiny" data-go="alerts">View all alerts</button>') : ""}
        </div>

        <section class="g-getstarted">
          ${HOME_GUIDES.some((g) => !g.action || can(g.action)) ? '<h3 class="g-getstarted__title">Recommended getting started guides</h3>' : ""}
          <div class="g-guides">
            ${HOME_GUIDES.filter((g) => !g.action || can(g.action)).map((g) => `<button class="g-guide" data-go="${g.go}" style="--guide:${g.color}">
              <span class="g-guide__glow"></span>
              <span class="g-guide__icon" data-ui="${g.icon}" data-ui-size="22"></span>
              <span class="g-guide__title">${g.title}</span>
              <span class="g-guide__desc">${g.desc}</span>
              <span class="g-guide__cta">${g.cta} <span data-ui="chevron" data-ui-size="14"></span></span>
            </button>`).join("")}
          </div>
        </section>

        <section class="g-account">
          <h3 class="g-getstarted__title">Your account</h3>
          <div class="g-account__grid">
            <div class="panel g-account__card">
              <div class="g-account__me">
                <span class="g-account__avatar">OP</span>
                <div class="g-account__id">
                  <div class="g-account__name">Oscar Porter</div>
                  <div class="tiny muted">op@acme.io</div>
                  <div class="g-account__pills">
                    ${pill("Owner · " + (org ? org.name : "—"), "ok")}
                    ${pill("MFA on", "ok")}
                    ${pill("Google linked", "off")}
                  </div>
                </div>
                <button class="btn btn--ghost tiny" data-go="profile">Edit profile</button>
              </div>
              <dl class="g-account__meta">
                <div><dt>Organizations</dt><dd>${ORGS.length} <span class="muted">· ${org ? org.name : "none selected"}</span></dd></div>
                <div><dt>Member since</dt><dd>2026-03-02</dd></div>
                <div><dt>Last login</dt><dd>12:41 · 10.4.2.19</dd></div>
                <div><dt>Timezone</dt><dd>UTC</dd></div>
              </dl>
            </div>

            <div class="panel g-account__card">
              <div class="g-account__cardhead"><span class="panel__title">Usage</span><span class="panel__subtitle">this organisation</span></div>
              <div class="g-usage">
                <div class="g-usage__row"><span class="g-usage__k">Devices</span><span class="g-usage__v">${org ? org.devices : 0} <span class="muted">/ 500</span></span></div>
                ${bar(org ? Math.round((org.devices / 500) * 100) : 0, "info")}
                <div class="g-usage__row"><span class="g-usage__k">Seats</span><span class="g-usage__v">${org ? org.members : 0} <span class="muted">/ 20</span></span></div>
                ${bar(org ? Math.round((org.members / 20) * 100) : 0, "ok")}
                <div class="g-usage__row"><span class="g-usage__k">Apps deployed</span><span class="g-usage__v">${APPS.length}</span></div>
                ${bar(Math.min(100, APPS.length * 10), "warn")}
              </div>
            </div>

            <div class="panel g-account__card">
              <div class="g-account__cardhead"><span class="panel__title">Sessions</span><span class="panel__subtitle">${SESSIONS.length} signed-in devices</span><button class="btn btn--ghost tiny" data-go="profile">Manage</button></div>
              <ul class="g-rows">
                ${SESSIONS.slice(0, 3).map((s) => `<li class="g-row" data-go="profile">
                  <span class="dot dot--${s.active ? "ok" : "off"}"></span>
                  <span class="g-row__name">${s.browser} · ${s.os}</span>
                  <span class="g-row__folder mono">${s.ip}</span>
                  <span class="g-row__at muted">${s.seen}</span>
                </li>`).join("")}
              </ul>
            </div>
          </div>
        </section>
      </div>`;
  },

  apps() {
    const running = APPS.filter((a) => a.status === "running").length;
    const stopped = APPS.filter((a) => a.status === "stopped").length;
    const crashed = APPS.filter((a) => a.status === "crashed").length;
    const exposed = APPS.filter((a) => a.expose !== "loopback").length;
    const appIcon = (t) => `<span class="app-icon" data-ui="${t.icon}" data-ui-size="18"></span>`;
    const templateById = (id) => APP_TEMPLATES.find((t) => t.id === id) || APP_TEMPLATES[0];

    const rows = APPS.map((a) => {
      const t = templateById(a.template);
      return `<article class="app-card" data-status="${a.status}">
        <header class="app-card__head">
          ${appIcon(t)}
          <span class="app-card__id">
            <span class="app-card__name">${a.name}</span>
            <span class="tiny muted mono">${a.version}</span>
          </span>
          ${appStatusPill(a.status)}
        </header>
        <div class="app-card__meta">
          <span class="tiny muted">on <span class="mono">${a.device.slice(-6)}</span></span>
          <span class="tiny muted">·</span>
          <span class="tiny muted">${a.expose === "loopback" ? "loopback only" : `${a.expose} · :${a.port}`}</span>
          <span class="tiny muted">·</span>
          <span class="tiny muted">${a.tier} tier</span>
        </div>
        <footer class="app-card__actions">
          ${a.status === "running"
            ? `<button class="btn btn--ghost tiny" data-app-act="stop" data-app="${a.id}">Stop</button><button class="btn btn--ghost tiny" data-app-act="restart" data-app="${a.id}">Restart</button>`
            : `<button class="btn btn--ghost tiny" data-app-act="start" data-app="${a.id}">Start</button>`}
          <button class="btn btn--ghost tiny" data-app-act="logs" data-app="${a.id}">Logs</button>
          ${a.status === "running" && a.expose !== "loopback" ? `<button class="btn btn--primary tiny" data-app-act="open" data-app="${a.id}">Open</button>` : ""}
        </footer>
      </article>`;
    }).join("");

    const templates = APP_TEMPLATES.map((t) => `<article class="app-tmpl" data-deploy="${t.id}">
      ${appIcon(t)}
      <div class="app-tmpl__id">
        <span class="app-tmpl__name">${t.name}${t.tag !== "—" ? ` <span class="badge">${t.tag}</span>` : ""}</span>
        <span class="app-tmpl__desc">${t.desc}</span>
      </div>
      <button class="btn btn--primary tiny" data-deploy="${t.id}">Deploy</button>
    </article>`).join("");

    return `
      <div class="page-head">
        <div><h1>Apps</h1><p>Applications hosted inside a device's container — start, expose and open them from the console</p></div>
        <div class="page-head__actions">
          <button class="btn btn--ghost">From manifest</button>
          ${canDo("apps.write", "+ Deploy an app", "btn--primary", { id: "app-deploy-focus" })}
        </div>
      </div>

      <div class="grid grid--4">
        ${statPanel("Running", String(running), "of " + APPS.length + " deployed", "", "")}
        ${statPanel("Stopped", String(stopped), "not serving", "", "")}
        ${statPanel("Crashed", String(crashed), "needs a restart", "", "")}
        ${statPanel("Exposed", String(exposed), "reachable beyond loopback", "", "")}
      </div>

      <div class="panel mt2">
        <div class="panel__head"><span class="panel__title">Deployed apps</span><span class="panel__subtitle">${APPS.length} in this organisation</span><span class="panel__menu" data-ui="ellipsis" data-ui-size="16"></span></div>
        <div class="panel__body">
          ${APPS.length ? `<div class="app-grid">${rows}</div>` : `<div class="home-empty">No apps deployed yet — deploy one from the catalog below.</div>`}
        </div>
      </div>

      <div class="panel mt2">
        <div class="panel__head"><span class="panel__title">Catalog</span><span class="panel__subtitle">deploy an app onto a device</span></div>
        <div class="panel__body">
          <div class="app-tmpls">${templates}</div>
        </div>
      </div>

      <div class="panel mt2">
        <div class="panel__head"><span class="panel__title">Exposure</span><span class="panel__subtitle">how an app is reached</span></div>
        <div class="panel__body">
          <div class="term-notes">
            <div class="term-note">
              <div class="term-note__h"><span class="badge">loopback</span> <b>the device itself</b></div>
              <p>The default. The app listens on <code class="code">127.0.0.1</code> inside the container and is reachable only from the device — right for an app driven by the terminal or by another app.</p>
            </div>
            <div class="term-note">
              <div class="term-note__h"><span class="badge">lan</span> <b>the local network, with a token</b></div>
              <p>Reachable from the local network, but it needs a <code class="code">port</code> and a token. The supervisor refuses to bind a non-loopback address without a token rather than binding it and hoping.</p>
            </div>
            <div class="term-note">
              <div class="term-note__h"><span class="badge">cloud</span> <b>deferred in v1</b></div>
              <p>Reaching an app from the internet needs the wildcard-DNS/TLS relay. A manifest that asks for it is <i>refused</i>, not silently downgraded, so a user is never misled.</p>
            </div>
            <div class="term-note">
              <div class="term-note__h"><span class="badge">runtime</span> <b>the supervisor keeps it alive</b></div>
              <p>One process inside the container starts every <code class="code">autostart</code> app, restarts a crash with capped backoff, and forwards each app's output to a rotated log.</p>
            </div>
          </div>
        </div>
      </div>`;
  },

  devices() {
    const online = DEVICES.filter((d) => d.status === "online").length;
    const attention = DEVICES.filter((d) => d.risk > 60 || d.battery < 15).length;
    const stale = DEVICES.filter((d) => d.fw !== "0.4.2").length;
    const orgs = [...new Set(DEVICES.map((d) => d.org))];
    return `
      <div class="page-head">
        <div><h1>Devices</h1><p>Every device the container runs on — health, firmware, tier and org</p></div>
        <div class="page-head__actions">
          <button class="btn btn--ghost">Tags</button>
          ${canDo("device.write", "+ Register device")}
        </div>
      </div>

      <div class="grid grid--4">
        ${statPanel("Online", `${online}<span style="font-size:18px;color:var(--text-secondary)">/${DEVICES.length}</span>`, "reporting in last 5 min", "", "")}
        ${statPanel("Needs attention", String(attention), "high risk or low battery", "", "")}
        ${statPanel("Firmware drift", String(stale), "not on the latest build", "", "")}
        ${statPanel("Organizations", String(orgs.length), orgs.join(" · "), "", "")}
      </div>

      <div class="dev-toolbar mt2">
        <div class="logs-search">
          <span data-ui="search" data-ui-size="14"></span>
          <input class="logs-search__input" id="dev-search" placeholder="Filter by IMEI, model or org…" />
        </div>
        <div class="seg" id="dev-status">
          <button class="seg__btn is-active" data-status="all">All</button>
          <button class="seg__btn" data-status="online">Online</button>
          <button class="seg__btn" data-status="offline">Offline</button>
        </div>
        <span class="topbar__spacer"></span>
        <span class="tiny muted" id="dev-count">${DEVICES.length} devices</span>
      </div>

      <div class="dev-grid" id="dev-grid">${DEVICES.map(deviceCard).join("")}</div>`;
  },

  permissions() {
    const active = MEMBERS.filter((m) => m.status === "active").length;
    const admins = MEMBERS.filter((m) => m.role === "admin" || m.role === "super_admin").length;
    const pending = INVITATIONS.filter((i) => i.status === "pending").length;
    const noMfa = MEMBERS.filter((m) => !m.mfa && m.status === "active").length;

    const memberRows = MEMBERS.map((m) => `<tr data-role="${m.role}" data-text="${(m.name + " " + m.email).toLowerCase()}" data-status="${m.status}">
      <td>
        <div class="perm-member">
          <span class="perm-member__name">${m.name}</span>
          <span class="tiny muted">${m.email}</span>
        </div>
      </td>
      <td>${rolePill(m.role)}</td>
      <td>${memberStatusPill(m.status)}</td>
      <td>${m.mfa ? pill("on", "ok") : pill("off", "warn")}</td>
      <td class="muted">${m.last}</td>
      <td class="row-actions">
        <button class="btn btn--ghost tiny" data-perm="change" data-who="${m.name}">Change role</button>
        <button class="btn btn--ghost tiny" data-perm="revoke" data-who="${m.name}">Revoke</button>
      </td>
    </tr>`).join("");

    const matrixRows = CAPABILITIES.map((c) => `<tr>
      <td><span class="perm-cap__label">${c.label}</span><span class="perm-cap__key mono">${c.key}</span></td>
      ${ROLES.map((r) => `<td class="perm-cap__cell">${c.roles.includes(r) ? '<span class="perm-yes" title="granted">✓</span>' : '<span class="perm-no" title="not granted">·</span>'}</td>`).join("")}
    </tr>`).join("");

    return `
      <div class="page-head">
        <div><h1>Permissions</h1><p>Who is in this organization, what role they hold, and what that role can do</p></div>
        <div class="page-head__actions">
          <button class="btn btn--ghost">Export audit</button>
          ${canDo("members.write", "+ Invite member", "btn--primary", { id: "perm-invite" })}
        </div>
      </div>

      <div class="grid grid--4">
        ${statPanel("Active members", String(active), "of " + MEMBERS.length + " in this org", "", "")}
        ${statPanel("Admins", String(admins), "admin or super_admin", "", "")}
        ${statPanel("Pending invites", String(pending), "awaiting acceptance", "", "")}
        ${statPanel("Without MFA", String(noMfa), "active members, MFA off", "", "")}
      </div>

      <div class="perm-panel" data-panel="members">
        <div class="panel mt2">
          <div class="panel__head">
            <span class="panel__title">Members</span>
            <span class="panel__subtitle">Acme Ops · role-based access</span>
            <span class="panel__menu" data-ui="ellipsis" data-ui-size="16"></span>
          </div>
          <div class="log-toolbar">
            <div class="logs-search">
              <span data-ui="search" data-ui-size="14"></span>
              <input class="logs-search__input" id="perm-search" placeholder="Filter by name or email…" />
            </div>
            <div class="seg" id="perm-roles">
              <button class="seg__btn is-active" data-role="all">All</button>
              ${ROLES.map((r) => `<button class="seg__btn" data-role="${r}">${r}</button>`).join("")}
            </div>
            <span class="topbar__spacer"></span>
            <span class="tiny muted" id="perm-count">${MEMBERS.length} members</span>
          </div>
          <div class="panel__body" style="padding-top:0">
            <table class="table">
              <thead><tr><th>Member</th><th>Role</th><th>Status</th><th>MFA</th><th>Last active</th><th></th></tr></thead>
              <tbody id="perm-rows">${memberRows}</tbody>
            </table>
          </div>
        </div>
      </div>

      <div class="perm-panel" data-panel="review" hidden>
        <div class="panel">
          <div class="panel__head"><span class="panel__title">Role capabilities</span><span class="panel__subtitle">what each role is allowed to do — permissions are derived from the role</span></div>
          <div class="panel__body" style="padding-top:8px">
            <table class="table perm-cap">
              <thead><tr><th>Capability</th>${ROLES.map((r) => `<th class="perm-cap__cell">${r.replace("_", " ")}</th>`).join("")}</tr></thead>
              <tbody>${matrixRows}</tbody>
            </table>
          </div>
        </div>
      </div>

      <div class="perm-panel" data-panel="invitations" hidden>
        <div class="panel">
          <div class="panel__head"><span class="panel__title">Invitations</span><span class="panel__subtitle">an invitation creates a membership with the chosen role when accepted</span></div>
          <div class="panel__body" style="padding-top:8px">
            <table class="table">
              <thead><tr><th>Email</th><th>Role</th><th>Invited</th><th>By</th><th>Expires</th><th>Status</th><th></th></tr></thead>
              <tbody>${INVITATIONS.map((i) => `<tr>
                <td>${i.email}</td>
                <td>${rolePill(i.role)}</td>
                <td class="muted">${i.invited}</td>
                <td class="muted">${i.by}</td>
                <td class="muted">${i.expires}</td>
                <td>${inviteStatusPill(i.status)}</td>
                <td class="row-actions">${i.status === "pending" ? '<button class="btn btn--ghost tiny">Resend</button><button class="btn btn--ghost tiny">Revoke</button>' : '<button class="btn btn--ghost tiny">View</button>'}</td>
              </tr>`).join("")}</tbody>
            </table>
          </div>
        </div>
      </div>

      <div class="perm-panel" data-panel="audit" hidden>
        <div class="panel">
          <div class="panel__head"><span class="panel__title">Access audit</span><span class="panel__subtitle">every grant, revoke and invitation is recorded</span></div>
          <div class="panel__body" style="padding-top:8px">
            <table class="table">
              <thead><tr><th>When</th><th>Actor</th><th>Action</th><th>Target</th><th>Detail</th><th>By role</th></tr></thead>
              <tbody>${PERM_AUDIT.map((a) => `<tr>
                <td class="muted">${a.at}</td>
                <td>${a.actor}</td>
                <td>${a.action}</td>
                <td class="mono">${a.target}</td>
                <td class="muted">${a.detail}</td>
                <td class="muted">${a.by}</td>
              </tr>`).join("")}</tbody>
            </table>
          </div>
        </div>
      </div>`;
  },

  telemetry() {
    return `
      <div class="page-head"><div><h1>Telemetry</h1><p>WebSocket stream · history &amp; export</p></div>
        <div class="page-head__actions"><button class="btn btn--ghost">Export CSV</button><button class="btn btn--primary">Open live stream</button></div></div>
      <div class="grid grid--2">
        <div class="panel"><div class="panel__head"><span class="panel__title">RSSI</span><span class="panel__subtitle">354481100123451 · dBm</span></div>
          <div class="panel__body"><div class="chart-host" id="chart-rssi"></div></div></div>
        <div class="panel"><div class="panel__head"><span class="panel__title">Battery</span><span class="panel__subtitle">354481100123451 · %</span></div>
          <div class="panel__body"><div class="chart-host" id="chart-batt"></div></div></div>
      </div>
      <div class="panel mt2"><div class="panel__head"><span class="panel__title">Live frames</span><span class="panel__subtitle">telemetryStream subscription</span></div>
        <div class="panel__body" style="padding-top:8px">
          <table class="table"><thead><tr><th>Time</th><th>IMEI</th><th>RSSI</th><th>Battery</th><th>Thermal</th></tr></thead>
          <tbody>${DEVICES.slice(0,4).map((d,i)=>`<tr><td class="mono muted">12:41:${(22-i*3).toString().padStart(2,"0")}</td><td class="mono">${d.imei}</td><td>${d.rssi||"—"} dBm</td><td>${d.battery}%</td><td>${d.thermal}</td></tr>`).join("")}</tbody></table>
        </div></div>
      <div class="panel mt2"><div class="panel__head"><span class="panel__title">RSSI heatmap</span><span class="panel__subtitle">per device · last 10 min</span></div>
        <div class="panel__body"><div class="chart-host" id="chart-heat"></div></div></div>`;
  },

  updates() {
    const latest = UPDATES.find((u) => u.latest) || UPDATES[0];
    const onLatest = UPDATE_DEVICES.filter((d) => d.current === latest.code).length;
    const pending = UPDATE_DEVICES.filter((d) => d.current !== latest.code && d.state !== "offline").length;
    return `
      <div class="page-head">
        <div><h1>Updates</h1><p>Release versions synced from GitHub · push installs to devices</p></div>
        <div class="page-head__actions">
          <button class="btn btn--ghost">Sync now</button>
          ${canDo("update.write", "Push update")}
        </div>
      </div>

      <div class="grid grid--4">
        ${statPanel("Latest version", latest.version, `version code ${latest.code} · ${latest.type}`, "", "")}
        ${statPanel("Devices on latest", `${onLatest}<span style="font-size:18px;color:var(--text-secondary)">/${UPDATE_DEVICES.length}</span>`, "reporting current build", "", "")}
        ${statPanel("Pending installs", String(pending), "downloading or awaiting install", "", "")}
        ${statPanel("Last sync", "12:38", "from github.com/VinnsEdesigner", "", "")}
      </div>

      <div class="upd-tabpanel" data-panel="overview">
        <div class="grid grid--3 mt2">
          <div class="panel" style="grid-column: span 2">
            <div class="panel__head"><span class="panel__title">Device update state</span><span class="panel__subtitle">each device's check → download → verify → install</span></div>
            <div class="panel__body"><div class="chart-host" id="chart-updstates"></div></div>
          </div>
          <div class="panel">
            <div class="panel__head"><span class="panel__title">Latest release</span><span class="panel__subtitle">${latest.filename}</span></div>
            <div class="panel__body">
              <div class="rel-meta">
                <div class="rel-meta__row"><span class="rel-meta__k">Version</span><span class="mono">${latest.version} (code ${latest.code})</span></div>
                <div class="rel-meta__row"><span class="rel-meta__k">Type</span><span>${latest.type}</span></div>
                <div class="rel-meta__row"><span class="rel-meta__k">Size</span><span>${latest.size}</span></div>
                <div class="rel-meta__row"><span class="rel-meta__k">Released</span><span>${latest.released}</span></div>
                <div class="rel-meta__row"><span class="rel-meta__k">SHA-256</span><span class="mono rel-meta__sha" title="${latest.sha}">${latest.sha.slice(0, 24)}…</span></div>
              </div>
              <p class="rel-notes">${latest.notes}</p>
            </div>
          </div>
        </div>

        <div class="panel mt2">
          <div class="panel__head"><span class="panel__title">Devices</span><span class="panel__subtitle">per-device update status · the last install tap is the user's</span><span class="panel__menu" data-ui="ellipsis" data-ui-size="16"></span></div>
          <div class="panel__body" style="padding-top:8px">
            <table class="table">
              <thead><tr><th>Device</th><th>Model</th><th>Current</th><th>Latest</th><th>State</th><th style="min-width:150px">Progress</th><th>Last seen</th><th></th></tr></thead>
              <tbody>${UPDATE_DEVICES.map((d) => `<tr>
                <td class="mono">${d.imei}</td>
                <td>${d.model}</td>
                <td class="mono">${d.current}</td>
                <td class="mono">${d.latest}</td>
                <td>${updateStateBadge(d.state)}</td>
                <td>${updateProgress(d)}</td>
                <td class="muted">${d.at}</td>
                <td class="row-actions"><button class="btn btn--ghost tiny">Push</button></td>
              </tr>`).join("")}</tbody>
            </table>
          </div>
        </div>
      </div>

      <div class="upd-tabpanel" data-panel="versions" hidden>
        <div class="panel">
          <div class="panel__head"><span class="panel__title">Release versions</span><span class="panel__subtitle">synced from GitHub releases · not uploaded here</span><span class="panel__menu" data-ui="ellipsis" data-ui-size="16"></span></div>
          <div class="panel__body" style="padding-top:8px">
            <table class="table">
              <thead><tr><th>Version</th><th>Type</th><th>APK</th><th>Size</th><th>SHA-256</th><th>Released</th><th>Notes</th><th></th></tr></thead>
              <tbody>${UPDATES.map((u) => `<tr>
                <td class="mono">${u.version}${u.latest ? ' <span class="badge">latest</span>' : ""}</td>
                <td><span class="badge">${u.type}</span></td>
                <td class="mono">${u.filename}</td>
                <td class="muted">${u.size}</td>
                <td class="mono muted" title="${u.sha}">${u.sha.slice(0, 10)}…</td>
                <td class="muted">${u.released}</td>
                <td class="muted upd-notes">${u.notes}</td>
                <td class="row-actions">${u.latest ? (can('update.write') ? '<button class="btn btn--primary tiny">Push to devices</button>' : '<button class="btn btn--ghost tiny" disabled>Push</button>') : '<button class="btn btn--ghost tiny">Push</button>'}</td>
              </tr>`).join("")}</tbody>
            </table>
          </div>
        </div>
      </div>

      <div class="upd-tabpanel" data-panel="pushes" hidden>
        <div class="panel">
          <div class="panel__head"><span class="panel__title">Push history</span><span class="panel__subtitle">update pushes · status reported back by each device</span><span class="panel__menu" data-ui="ellipsis" data-ui-size="16"></span></div>
          <div class="panel__body" style="padding-top:8px">
            <table class="table">
              <thead><tr><th>Push</th><th>Version</th><th>Install</th><th>State</th><th style="min-width:150px">Devices</th><th>Started</th><th>By</th></tr></thead>
              <tbody>${UPDATE_PUSHES.map((p) => `<tr>
                <td class="mono">${p.id}</td>
                <td class="mono">${p.version}</td>
                <td>${p.type}</td>
                <td>${pushStateBadge(p.state)}</td>
                <td>${bar(Math.round((p.done / p.devices) * 100), p.failed ? "warn" : p.state === "completed" ? "ok" : "info")}<span class="tiny muted">${p.done}/${p.devices}${p.failed ? ` · ${p.failed} failed` : ""}</span></td>
                <td class="muted">${p.started}</td>
                <td class="muted">${p.by}</td>
              </tr>`).join("")}</tbody>
            </table>
          </div>
        </div>
      </div>`;
  },

  alerts() {
    return `
      <div class="page-head"><div><h1>Alerts</h1><p>Threshold-based rule engine · contact points · history</p></div>
        <div class="page-head__actions"><button class="btn btn--ghost">Contact points</button><button class="btn btn--primary">+ New rule</button></div></div>
      <div class="grid grid--4">
        ${statPanel("Firing", "3", "active now", "", "")}
        ${statPanel("Rules", "12", "enabled", "", "")}
        ${statPanel("MTTA", "2m 10s", "mean time to ack", "▲", "up", [3,3,2.6,2.4,2.3,2.2,2.1])}
        ${statPanel("Notifications", "34", "sent today", "", "")}
      </div>
      <div class="grid grid--2 mt2">
        <div class="panel"><div class="panel__head"><span class="panel__title">Alerts by rule (7d)</span><span class="panel__subtitle">firing count</span></div>
          <div class="panel__body"><div class="chart-host" id="chart-alerts"></div></div></div>
        <div class="panel"><div class="panel__head"><span class="panel__title">Mean time to ack</span><span class="panel__subtitle">minutes</span></div>
          <div class="panel__body"><div class="chart-host" id="chart-mtta"></div></div></div>
      </div>
      <div class="panel mt2"><div class="panel__head"><span class="panel__title">Alert rules</span><span class="panel__menu" data-ui="ellipsis" data-ui-size="16"></span></div>
        <div class="panel__body" style="padding-top:8px">
          <table class="table"><thead><tr><th>Severity</th><th>Rule</th><th>Device</th><th>Message</th><th>State</th><th>Since</th></tr></thead>
          <tbody>${ALERTS.map(a => `<tr>
            <td>${sevBadge(a.sev)}</td><td class="mono">${a.rule}</td><td class="mono">${a.device}</td>
            <td>${a.msg}</td>
            <td>${a.state === "firing" ? pill("firing", "err") : pill("resolved", "ok")}</td>
            <td class="muted">${a.at}</td></tr>`).join("")}</tbody></table>
        </div></div>`;
  },

  logs() {
    const counts = LOGS.reduce((a, l) => ((a[l.lvl] = (a[l.lvl] || 0) + 1), a), {});
    const services = [...new Set(LOGS.map((l) => l.svc))].sort();
    const fields = [...new Set(LOGS.flatMap((l) => Object.keys(l.fields || {})))].sort();
    return `
      <div class="page-head">
        <div><h1>Logs</h1><p>Structured server logs · query, facet and tail the stream</p></div>
        <div class="page-head__actions">
          <span class="badge">${counts.error || 0} error</span>
          <span class="badge">${counts.warn || 0} warn</span>
          <button class="btn btn--ghost">Export</button>
          <button class="btn btn--primary"><span class="dot dot--ok" style="margin-right:4px"></span>Live tail</button>
        </div>
      </div>

      <div class="log-tabpanel" data-panel="entries">
        <div class="logs-console">
          <aside class="logs-facets">
            <div class="logs-facet">
              <div class="logs-facet__head"><span>Levels</span></div>
              <div class="logs-facet__body" id="facet-levels">
                <button class="logs-facet__row is-on" data-facet="level" data-val="error"><span class="dot dot--err"></span>error<span class="logs-facet__n">${counts.error || 0}</span></button>
                <button class="logs-facet__row is-on" data-facet="level" data-val="warn"><span class="dot dot--warn"></span>warn<span class="logs-facet__n">${counts.warn || 0}</span></button>
                <button class="logs-facet__row is-on" data-facet="level" data-val="info"><span class="dot dot--ok"></span>info<span class="logs-facet__n">${counts.info || 0}</span></button>
                <button class="logs-facet__row is-on" data-facet="level" data-val="debug"><span class="dot dot--off"></span>debug<span class="logs-facet__n">${counts.debug || 0}</span></button>
              </div>
            </div>
            <div class="logs-facet">
              <div class="logs-facet__head"><span>Service</span></div>
              <div class="logs-facet__body" id="facet-services">
                ${services.map((s) => `<button class="logs-facet__row" data-facet="svc" data-val="${s}"><span class="logs-facet__sq"></span>${s}<span class="logs-facet__n">${LOGS.filter((l) => l.svc === s).length}</span></button>`).join("")}
              </div>
            </div>
            <div class="logs-facet">
              <div class="logs-facet__head"><span>Fields</span><span class="tiny muted">${fields.length} detected</span></div>
              <div class="logs-facet__body logs-facet__body--chips">
                ${fields.map((f) => `<button class="logs-fieldchip" data-facet="field" data-val="${f}">${f}</button>`).join("")}
              </div>
            </div>
          </aside>

          <section class="logs-main">
            <div class="logs-toolbar">
              <div class="logs-search">
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><line x1="21" y1="21" x2="16.7" y2="16.7"/></svg>
                <input class="logs-search__input" id="log-search" placeholder="Filter logs (message, service, trace, field…)…" />
                <kbd>/</kbd>
              </div>
              <button class="btn btn--ghost tiny" id="log-wrap" aria-pressed="true">Wrap</button>
              <button class="btn btn--ghost tiny" id="log-expand">Expand all</button>
              <div class="topbar__spacer"></div>
              <span class="tiny muted" id="log-count">${LOGS.length} of ${LOGS.length} entries</span>
            </div>

            <div class="logs-histogram">
              <div class="logs-histogram__head">
                <span class="tiny muted" id="log-range">12:20 → 12:41 · last 22 min</span>
                <div class="chart-legend">
                  <span><i style="background:#f2495c"></i>Error</span>
                  <span><i style="background:#ff9830"></i>Warn</span>
                  <span><i style="background:#fb7185"></i>Info</span>
                  <span><i style="background:#5b5b5b"></i>Debug</span>
                </div>
                <button class="btn btn--ghost tiny" id="log-range-clear" hidden>Clear range</button>
              </div>
              <div class="logs-histogram__bars" id="log-histogram">${logHistogramHTML()}</div>
            </div>

            <div class="logs-table">
              <div class="logs-head logs-row">
                <span class="logs-cell logs-cell--time">Time</span>
                <span class="logs-cell logs-cell--lvl">Level</span>
                <span class="logs-cell logs-cell--svc">Service</span>
                <span class="logs-cell logs-cell--msg">Message</span>
                <span class="logs-cell logs-cell--imei">Device</span>
                <span class="logs-cell logs-cell--trace">Trace</span>
                <span class="logs-cell logs-cell--dur">Dur</span>
              </div>
              <div class="logs-stream" id="log-view">${logRows(LOGS)}</div>
            </div>
          </section>
        </div>
      </div>

      <div class="log-tabpanel" data-panel="volume" hidden>
        <div class="panel">
          <div class="panel__head"><span class="panel__title">Log volume</span><span class="panel__subtitle">stacked by level · last 22 min</span></div>
          <div class="panel__body">
            <div class="chart-legend" style="padding-top:0">
              <span><i style="background:#f2495c"></i>Error</span>
              <span><i style="background:#ff9830"></i>Warn</span>
              <span><i style="background:#fb7185"></i>Info</span>
              <span><i style="background:#5b5b5b"></i>Debug</span>
            </div>
            <div class="chart-host" id="chart-loghist"></div>
          </div>
        </div>
        <div class="grid grid--3 mt2">
          ${statPanel("Errors", String(counts.error || 0), "last 60 min", "", "")}
          ${statPanel("Warnings", String(counts.warn || 0), "last 60 min", "", "")}
          ${statPanel("Total", String(LOGS.length), "entries retained", "", "")}
        </div>
      </div>`;
  },

  terminal() {
    if (!TERM_LIST.length) {
      return `
        <div class="page-head"><div><h1>Terminal</h1><p>Container shell · PTY over the device WebSocket · gated by <code class="code">shell.execute</code></p></div></div>
        <div class="panel"><div class="panel__body">
          <div class="empty-state">
            <div class="empty-state__icon">${window.VyzIcons.icon("terminal", 34)}</div>
            <div class="empty-state__title">No open sessions</div>
            <div class="empty-state__desc">Open a shell on a device to debug it in place. The PTY is allocated on the device and the shell outlives the socket.</div>
            ${canDo("shell.execute", "+ New session")}
          </div>
        </div></div>`;
    }
    const active = TERM_LIST.find((s) => s.id === TERM_ACTIVE) || TERM_LIST[0];
    const detached = active.state === "detached";

    return `
      <div class="term-tabpanel" data-panel="session">
        <div class="term-screenwrap">
          <div class="term-sessionbar">
            <div class="term-tabs" id="term-tabs" role="tablist">
              ${TERM_LIST.map((s) => `
                <button class="term-tab${s.id === active.id ? " is-active" : ""}" data-sess="${s.id}" role="tab" aria-selected="${s.id === active.id}">
                  <span class="dot dot--${termDot(s.state)}"></span>
                  <span class="term-tab__dev">${s.tab}</span>
                  <span class="term-tab__state">${s.state}</span>
                  <span class="term-tab__x" data-close="${s.id}" title="Close session">✕</span>
                </button>`).join("")}
            </div>
            ${canDo("shell.execute", "+ New session", "btn--ghost tiny")}
          </div>

          <div class="term" data-state="${active.state}">
            <header class="term__bar">
              <div class="term__bar-id">
                <span class="dot dot--${termDot(active.state)}"></span>
                <span class="term__bar-op">${active.user}</span><span class="term__bar-at">@</span><span class="term__bar-host">vyzorix</span>
                <span class="term__bar-sep">·</span>
                <span class="term__bar-dev mono">${active.imei}</span>
              </div>
              <div class="term__bar-actions">
                <button class="term__icon" data-term-act="search" title="Find in buffer" aria-label="Find">${termIcon("search")}</button>
                <button class="term__icon" data-term-act="copy" title="Copy buffer" aria-label="Copy">${termIcon("copy")}</button>
                <button class="term__icon" data-term-act="clear" title="Clear buffer" aria-label="Clear">${termIcon("clear")}</button>
                <button class="term__icon term__icon--rec" data-term-act="record" title="Record session" aria-label="Record">${termIcon("record")}</button>
                <span class="term__bar-div"></span>
                <button class="term__icon" id="term-expand" title="Full screen" aria-label="Toggle full screen">${termIcon("expand")}</button>
              </div>
            </header>

            <div class="term__meta">
              <div class="term__meta-row">
                <span class="term__meta-item"><span class="term__meta-k">Device</span>${active.model} · fw ${active.fw} · <span class="mono">${active.imei}</span></span>
                <span class="term__meta-item"><span class="term__meta-k">User</span>${active.user} · ${active.role}</span>
              </div>
              <div class="term__meta-row">
                <span class="term__meta-item"><span class="term__meta-k">Grant</span><span class="term__grant">${active.grant}</span></span>
                <span class="term__meta-item"><span class="term__meta-k">Tier</span>${active.tier}</span>
                <span class="term__meta-item"><span class="term__meta-k">Expires</span>${active.expires === "expired" ? '<span class="term__expired">expired</span>' : active.expires}</span>
                <span class="term__meta-item"><span class="term__meta-k">tmux</span><span class="mono">"${active.tmux}"</span></span>
              </div>
            </div>

            <div class="term__screen mono" id="term-screen" tabindex="0" aria-label="Terminal buffer">
              <div class="term__lines" id="term-lines">${termLines(active)}</div>
              <textarea class="term__input" id="term-input" rows="1" autocapitalize="off" autocomplete="off" autocorrect="off" spellcheck="false" aria-label="Terminal input"></textarea>
              ${detached ? `
                <div class="term__reconnect">
                  <div class="term__reconnect-card">
                    <div class="term__reconnect-title">Session detached</div>
                    <div class="term__reconnect-desc">The socket dropped. The shell is still running on the device under <b>tmux&nbsp;"${active.tmux}"</b>; reattach to the same session id to resume.</div>
                    <button class="btn btn--primary" id="term-reconnect">Reconnect</button>
                  </div>
                </div>` : ""}
            </div>

            <div class="term__keybar" id="term-keybar" role="toolbar" aria-label="On-screen keys">
              ${termKeybar()}
            </div>

            <footer class="term__status">
              <span class="term__status-item"><span class="dot dot--${termDot(active.state)}"></span>${active.state}</span>
              <span class="term__status-item">tmux "${active.tmux}"</span>
              <span class="term__status-item">${active.grant}</span>
              <span class="term__spacer"></span>
              <span class="term__status-item">idle ${active.idle}</span>
              <span class="term__status-item">rec ${active.recording}</span>
              <span class="term__status-item" id="term-size">${active.cols}×${active.rows}</span>
            </footer>
          </div>
        </div>
      </div>

      <div class="term-tabpanel" data-panel="files" hidden>
        <div class="fm">
          <aside class="fm__tree">
            <div class="fm__tree-head"><span>Container filesystem</span><span class="tiny muted">proot · /</span></div>
            <div class="fm__tree-body" id="fm-tree">${fmTreeHTML(CONTAINER_FS, [], 0)}</div>
          </aside>
          <section class="fm__main">
            <div class="fm__pathbar">
              <nav class="fm__crumbs" id="fm-crumbs">${fmCrumbsHTML(FM_PATH)}</nav>
              <span class="fm__spacer"></span>
              <span class="tiny muted" id="fm-note"></span>
            </div>
            <div class="fm__list" id="fm-list">${fmListHTML(FM_PATH)}</div>
          </section>
        </div>
      </div>

      <div class="term-tabpanel" data-panel="recordings" hidden>
        <div class="panel">
          <div class="panel__head"><span class="panel__title">Session recordings</span><span class="panel__subtitle">a recording is as sensitive as the terminal — gated the same way</span></div>
          <div class="panel__body" style="padding-top:8px">
            <table class="table">
              <thead><tr><th>Recording</th><th>Session</th><th>Device</th><th>Operator</th><th>Duration</th><th>Size</th><th>State</th><th></th></tr></thead>
              <tbody>${TERM_RECORDINGS.map((r) => `<tr>
                <td class="mono">${r.id}</td>
                <td class="mono">${r.session}</td>
                <td class="mono">${r.imei}</td>
                <td>${r.user}</td>
                <td class="muted">${r.dur}</td>
                <td class="muted">${r.size}</td>
                <td>${termPill({ recording: "err", detached: "warn", complete: "ok" }[r.state] || "off", r.state)}</td>
                <td class="row-actions"><button class="btn btn--ghost tiny">Play</button><button class="btn btn--ghost tiny">Download</button></td>
              </tr>`).join("")}</tbody>
            </table>
          </div>
        </div>
      </div>

      <div class="term-tabpanel" data-panel="limits" hidden>
        <div class="grid grid--3">
          ${statPanel("Idle timeout", "15m", "server closes a silent session", "", "")}
          ${statPanel("Session TTL", "2h", "absolute life of a session", "", "")}
          ${statPanel("Output outbox", "256", "frames buffered before drop", "", "")}
        </div>
        <div class="panel mt2">
          <div class="panel__head"><span class="panel__title">How a session is gated &amp; kept alive</span></div>
          <div class="panel__body">
            <div class="term-notes">
              <div class="term-note">
                <div class="term-note__h"><span class="badge">access</span> <b>shell.execute</b></div>
                <p>A shell is the highest-consequence capability on a device, so the terminal route is gated on the <code class="code">shell.execute</code> grant — not on device-read. Opening a session and reading its recording are gated the same way.</p>
              </div>
              <div class="term-note">
                <div class="term-note__h"><span class="badge">detach</span> <b>the shell outlives the socket</b></div>
                <p>When the WebSocket drops the relay detaches the session instead of closing it: the shell keeps running and its output is buffered up to a bound (oldest bytes dropped first — a terminal is a tail). A reconnect re-attaches by the same session id.</p>
              </div>
              <div class="term-note">
                <div class="term-note__h"><span class="badge">persistence</span> <b>two layers</b></div>
                <p>The relay's detach/reattach keeps the PTY session alive across a socket drop. <code class="code">tmux</code>, when present, keeps the shell alive across a session close — reconnect with the same session name and the shell is still there.</p>
              </div>
              <div class="term-note">
                <div class="term-note__h"><span class="badge">pty</span> <b>allocated on the device</b></div>
                <p>The container never opens <code class="code">/dev/ptmx</code>. The device's Layer 1 allocates the PTY, holds the master fd and execs the <code class="code">proot</code> login shell — the fd <i>is</i> the capability, and it is auditable.</p>
              </div>
            </div>
          </div>
        </div>
      </div>`;
  },

  orgs() {
    const totalDevices = ORGS.reduce((a, o) => a + o.devices, 0);
    const totalMembers = ORGS.reduce((a, o) => a + o.members, 0);
    const suspended = ORGS.filter((o) => o.status !== "active").length;
    const PLAN_LIMIT = { Scale: 500, Pro: 100, Free: 10 };
    const initials = (n) => n.split(/\s+/).map((w) => w[0]).join("").slice(0, 2).toUpperCase();
    return `
      <div class="page-head">
        <div><h1>Organizations</h1><p>Your workspaces — switch between them, or manage members, quotas and settings</p></div>
        <div class="page-head__actions">
          <button class="btn btn--ghost">Invitations</button>
          <button class="btn btn--primary">+ New organization</button>
        </div>
      </div>

      <div class="grid grid--4">
        ${statPanel("Organizations", String(ORGS.length), "you belong to", "", "")}
        ${statPanel("Total devices", String(totalDevices), "across all orgs", "", "")}
        ${statPanel("Members", String(totalMembers), "seats in use", "", "")}
        ${statPanel("Suspended", String(suspended), "needs attention", "", "")}
      </div>

      <div class="org-grid mt2">
        ${ORGS.map((o) => {
          const limit = PLAN_LIMIT[o.plan] || 100;
          const pct = Math.round((o.devices / limit) * 100);
          const tone = pct > 85 ? "err" : pct > 60 ? "warn" : "ok";
          const isActive = o.slug === ACTIVE_ORG;
          return `<div class="panel org-card${isActive ? " is-active" : ""}">
            <div class="org-card__top">
              <div class="org-avatar">${initials(o.name)}</div>
              <div class="org-card__id">
                <div class="org-name">${o.name}${isActive ? ' <span class="badge">current</span>' : ""}</div>
                <div class="tiny muted">${o.plan} plan · your role: ${o.role}</div>
              </div>
              ${o.status === "active" ? pill("active", "ok") : pill("suspended", "warn")}
            </div>
            <div class="org-card__stats">
              <div><div class="tiny muted">Members</div><b>${o.members}</b></div>
              <div><div class="tiny muted">Devices</div><b>${o.devices}</b></div>
              <div><div class="tiny muted">Apps</div><b>${o.apps}</b></div>
            </div>
            <div class="org-card__usage">
              <div class="flex between tiny"><span class="muted">Device quota</span><span>${o.devices} / ${limit}</span></div>
              <div style="margin-top:6px">${bar(pct, tone)}</div>
            </div>
            <div class="org-card__meta tiny muted">Region ${o.region} · owner ${o.owner}</div>
            <div class="org-card__actions">
              ${o.status === "active" && !isActive
                ? `<button class="btn btn--primary" data-pick-org="${o.slug}">Switch to</button>`
                : `<button class="btn btn--ghost" data-go="permissions">Members</button>`}
              <button class="btn btn--ghost" data-go="settings">Settings</button>
            </div>
          </div>`;
        }).join("")}
      </div>

      <div class="panel mt2">
        <div class="panel__head"><span class="panel__title">How organisations work</span><span class="panel__subtitle">the tenant boundary</span></div>
        <div class="panel__body">
          <div class="term-notes">
            <div class="term-note">
              <div class="term-note__h"><span class="badge">tenant</span> <b>everything is org-scoped</b></div>
              <p>Devices, apps, updates, API keys and telemetry all belong to an organisation. Switching organisation changes what every operational page shows.</p>
            </div>
            <div class="term-note">
              <div class="term-note__h"><span class="badge">operator</span> <b>one identity, many orgs</b></div>
              <p>A user is a global identity that joins organisations with a <b>role</b>. The same person can be an owner here and a viewer elsewhere.</p>
            </div>
            <div class="term-note">
              <div class="term-note__h"><span class="badge">invite</span> <b>joining is by invitation</b></div>
              <p>An admin invites an email with a role; accepting creates the membership. Invitations expire and can be revoked.</p>
            </div>
            <div class="term-note">
              <div class="term-note__h"><span class="badge">quota</span> <b>the plan sets the ceiling</b></div>
              <p>Each plan caps devices and seats. A suspended organisation keeps its data but cannot be opened until it is restored.</p>
            </div>
          </div>
        </div>
      </div>`;
  },

  "api-keys"() {
    const active = KEYS.filter((k) => k.status === "active").length;
    const revoked = KEYS.filter((k) => k.status !== "active").length;
    const mask = (id) => `${id.slice(0, 6)}••••••••${id.slice(-2)}`;
    return `
      <div class="page-head">
        <div><h1>API Keys</h1><p>Service accounts · scoped tokens · rotation</p></div>
        <div class="page-head__actions">
          <button class="btn btn--ghost">Audit log</button>
          ${canDo("keys.manage", "+ Create key")}
        </div>
      </div>

      <div class="grid grid--4">
        ${statPanel("Active keys", String(active), "in use right now", "", "")}
        ${statPanel("Revoked", String(revoked), "retained for audit", "", "")}
        ${statPanel("Requests (24h)", "12.4k", "across all keys", "▲ 8%", "up", [9, 10, 10.5, 11, 11.6, 12, 12.4])}
        ${statPanel("Rotation due", "1", "older than 90 days", "", "")}
      </div>

      <div class="grid grid--2 mt2">
        <div class="panel"><div class="panel__head"><span class="panel__title">Requests per key</span><span class="panel__subtitle">last 24h</span></div>
          <div class="panel__body"><div class="chart-host" id="chart-keyusage"></div></div></div>
        <div class="panel"><div class="panel__head"><span class="panel__title">Authentication</span><span class="panel__subtitle">success vs rejected · 24h</span></div>
          <div class="panel__body"><div class="chart-host" id="chart-auth"></div></div></div>
      </div>

      <div class="panel mt2">
        <div class="panel__head"><span class="panel__title">Keys</span><span class="panel__subtitle">${KEYS.length} total</span><span class="panel__menu" data-ui="ellipsis" data-ui-size="16"></span></div>
        <div class="panel__body" style="padding-top:8px">
          <table class="table">
            <thead><tr><th>Key ID</th><th>Client</th><th>Scopes</th><th>Created</th><th>Last used</th><th>Status</th><th></th></tr></thead>
            <tbody>${KEYS.map((k) => `<tr>
              <td>
                <span class="mono key-id">${mask(k.id)}</span>
                <button class="icon-btn key-reveal" data-key="${k.id}" title="Reveal">${eyeIcon()}</button>
              </td>
              <td><b>${k.client}</b></td>
              <td>${k.scopes.split(",").map((s) => `<span class="badge scope">${s.trim()}</span>`).join(" ")}</td>
              <td class="muted">${k.created}</td>
              <td class="muted">${k.last}</td>
              <td>${k.status === "active" ? pill("active", "ok") : pill("revoked", "err")}</td>
              <td class="row-actions">
                <button class="btn btn--ghost tiny">Rotate</button>
                <button class="btn btn--ghost tiny key-revoke${k.status !== "active" ? " is-disabled" : ""}">${k.status === "active" ? "Revoke" : "—"}</button>
              </td>
            </tr>`).join("")}</tbody>
          </table>
        </div>
      </div>`;
  },

  profile() {
    return `
      <div class="profile">
        <aside class="profile-card panel">
          <div class="profile-card__avatar">OP</div>
          <div class="profile-card__name">Oscar Porter</div>
          <div class="profile-card__role">Owner · Acme Ops</div>
          <div class="flex" style="justify-content:center;gap:6px;margin-top:6px">
            <span class="badge">MFA on</span>
            <span class="badge">Google linked</span>
          </div>
          <div class="profile-card__meta">
            <div class="flex between"><span class="muted">Email</span><span>op@acme.io</span></div>
            <div class="flex between"><span class="muted">Phone</span><span>+44 •••• 2210</span></div>
            <div class="flex between"><span class="muted">Member since</span><span>2026-03-02</span></div>
            <div class="flex between"><span class="muted">Last login</span><span>12:41</span></div>
          </div>
          <div class="profile-card__session">
            <div class="tiny muted" style="margin-bottom:4px">Current session</div>
            <div class="flex between"><span class="muted">IP address</span><span class="mono">10.4.2.19</span></div>
            <div class="flex between"><span class="muted">Browser &amp; OS</span><span>Chrome 153 · Linux</span></div>
          </div>
          <div class="profile-card__actions">
            <button class="btn btn--primary" id="profile-upload">Upload photo</button>
            <button class="btn btn--ghost">Copy user ID</button>
          </div>
        </aside>

        <div class="profile-main">
          <div class="profile-panel" data-panel="info">
            <div class="grid grid--4">
              ${statPanel("Sessions", "3", "active devices", "", "")}
              ${statPanel("Commands sent", "1.2k", "all time", "▲ 40", "up", [900, 980, 1040, 1090, 1140, 1180, 1200])}
              ${statPanel("Actions (30d)", "486", "audited events", "", "")}
              ${statPanel("Alerts acked", "112", "mean 2m 10s", "", "")}
            </div>

            <div class="panel mt2">
              <div class="panel__head"><span class="panel__title">Profile</span><span class="panel__subtitle">Your user identity</span></div>
              <div class="panel__body">
                ${field("Display name", "Shown to other users in the organization", input("p-name", "Oscar Porter"))}
                ${field("Email", "Used for sign-in and notifications", input("p-email", "op@acme.io"))}
                ${field("Job title", "Optional — shown on your profile", input("p-title", "Operations Lead"))}
                ${field("Timezone", "Applied to your timestamps", select("p-tz", ["UTC", "Europe/London", "America/New_York"], "UTC"))}
                ${field("Language", "Console display language", select("p-lang", ["English", "Deutsch", "Français"], "English"))}
              </div>
            </div>

            <div class="panel mt2">
              <div class="panel__head"><span class="panel__title">Security</span></div>
              <div class="panel__body">
                ${field("Password", "Last changed 42 days ago", `<button class="btn btn--ghost">Change password</button>`)}
                ${field("Multi-factor auth", "TOTP required on every login", switchEl("p-mfa", true))}
                ${field("Backup codes", "8 of 10 remaining", `<button class="btn btn--ghost">Regenerate</button>`)}
                ${field("Active sessions", "Revoke every other signed-in device", `<button class="btn btn--danger">Revoke all others</button>`)}
              </div>
            </div>

            <div class="panel mt2">
              <div class="panel__head"><span class="panel__title">API tokens</span><span class="panel__subtitle">Personal access tokens</span></div>
              <div class="panel__body" style="padding-top:8px">
                <table class="table">
                  <thead><tr><th>Name</th><th>Scope</th><th>Created</th><th>Last used</th><th></th></tr></thead>
                  <tbody>
                    <tr><td><b>cli-laptop</b></td><td><span class="badge scope">full</span></td><td class="muted">2026-06-01</td><td class="muted">12:30</td><td class="row-actions"><button class="btn btn--ghost tiny">Revoke</button></td></tr>
                    <tr><td><b>ci-readonly</b></td><td><span class="badge scope">metrics:read</span></td><td class="muted">2026-07-14</td><td class="muted">11:58</td><td class="row-actions"><button class="btn btn--ghost tiny">Revoke</button></td></tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <div class="profile-panel" data-panel="preferences" hidden>
            <div class="panel">
              <div class="panel__head"><span class="panel__title">Preferences</span><span class="panel__subtitle">Your org &amp; UI defaults</span></div>
              <div class="panel__body">
                ${field("Interface theme", "Appearance of the console — built-in themes", themeSelect())}
                ${field("Home Dashboard", "Landing dashboard when you sign in", select("pref-home", ["Default dashboard", "None", "Overview", "OTA adoption", "Alert activity"], "Overview"))}
                ${field("Timezone", "Used for dashboards and alert timestamps", select("pref-timezone", ["Default", "Browser time", "UTC", "Europe/London", "America/New_York"], "Browser time"))}
                ${field("Week start", "First day of the week", select("pref-week", ["Default", "Monday", "Sunday", "Saturday"], "Monday"))}
                ${field("Language", "Console display language", select("pref-language", ["Default", "English", "Deutsch", "Français", "Español", "Português"], "English"))}
                <div class="field">
                  <div class="field__label"></div>
                  <div class="field__control"><button class="btn btn--primary" id="prefs-save">Save preferences</button></div>
                </div>
              </div>
            </div>
          </div>
          <div class="profile-panel" data-panel="sessions" hidden>
            <div class="panel">
              <div class="panel__head"><span class="panel__title">Sessions</span><span class="panel__subtitle">Signed-in devices for this account</span></div>
              <div class="panel__body" style="padding-top:8px">
                <table class="table">
                  <thead><tr><th>Last seen</th><th>Logged on</th><th>IP address</th><th>Browser &amp; OS</th><th>Identity Provider</th><th></th></tr></thead>
                  <tbody>${SESSIONS.map((s) => `<tr>
                    <td>${s.active ? pill("Now", "ok") : `<span class="muted">${s.seen}</span>`}</td>
                    <td class="muted">${s.loggedOn}</td>
                    <td class="mono">${s.ip}</td>
                    <td>${s.browser} on ${s.os}</td>
                    <td><span class="badge scope">${s.idp}</span></td>
                    <td class="row-actions"><button class="btn btn--ghost tiny session-revoke${s.active ? " is-current" : ""}">${s.active ? "This device" : "Revoke"}</button></td>
                  </tr>`).join("")}</tbody>
                </table>
              </div>
            </div>
          </div>
        </div>
      </div>`;
  },

  // ----- Administration sub-pages -----

  "admin-defaultprefs"() {
    return `
      <div class="page-head">
        <div><h1>Default preferences</h1><p>Manage preferences across an organization</p></div>
        <div class="page-head__actions"><button class="btn btn--primary" id="prefs-save">Save preferences</button></div>
      </div>
      <div class="admin-form">
        <div class="panel">
          <div class="panel__head"><span class="panel__title">Organization profile</span></div>
          <div class="panel__body">
            ${field("Organization name", "The display name of this organization", input("org-name", "Acme Ops"))}
          </div>
        </div>
        <div class="panel mt2">
          <div class="panel__head"><span class="panel__title">Preferences</span></div>
          <div class="panel__body">
            ${field("Home dashboard", "Default landing dashboard for this organization", select("home-dash", ["None", "Overview", "OTA adoption", "Alert activity"], "Overview"))}
            ${field("Timezone", "Used for dashboards and alert timestamps", select("prefs-tz", ["Browser time", "UTC", "Europe/London", "America/New_York"], "Browser time"))}
            ${field("Week start", "First day of the week", select("week-start", ["Monday", "Sunday", "Saturday"], "Monday"))}
            ${field("Default theme", "Appearance for new sessions", select("theme", ["Dark", "Light", "System"], "Dark"))}
          </div>
        </div>
      </div>`;
  },

  "admin-provisioning"() {
    return `
      <div class="page-head">
        <div><h1>Provisioning</h1><p>Manage your provisioning repositories and connections</p></div>
        <div class="page-head__actions"><button class="btn btn--primary">Connect repository</button></div>
      </div>
      <div class="prov-panel" data-panel="repos">
        <div class="panel"><div class="panel__body">
          <div class="empty-state">
            <div class="empty-state__icon">${plugIcon()}</div>
            <div class="empty-state__title">No repositories yet</div>
            <div class="empty-state__desc">Connect a Git repository to manage your resources as code.</div>
            <button class="btn btn--primary">Connect repository</button>
          </div>
        </div></div>
      </div>
      <div class="prov-panel" data-panel="connections" hidden>
        <div class="panel"><div class="panel__body" style="padding-top:8px">
          <table class="table"><thead><tr><th>Name</th><th>Type</th><th>Status</th><th></th></tr></thead>
          <tbody>
            <tr><td><b>github-acme</b></td><td>GitHub</td><td>${pill("connected", "ok")}</td><td class="row-actions"><button class="btn btn--ghost tiny">Edit</button></td></tr>
          </tbody></table>
        </div></div>
      </div>
      <div class="prov-panel" data-panel="getting-started" hidden>
        <div class="panel"><div class="panel__body">
          <ol class="steps">
            <li><b>Connect a repository</b><span class="muted"> — link a Git provider account.</span></li>
            <li><b>Choose a branch</b><span class="muted"> — the source of truth for your resources.</span></li>
            <li><b>Set up a sync</b><span class="muted"> — pull changes on a schedule or via webhook.</span></li>
          </ol>
        </div></div>
      </div>`;
  },

  "admin-users"() {
    const USERS = [
      { login: "oscar.porter", email: "op@acme.io", name: "Oscar Porter", orgs: "Acme Ops", role: "Admin", last: "12:41", origin: "Local", prov: false },
      { login: "nina.grzelak", email: "nina@acme.io", name: "Nina Grzelak", orgs: "Acme Ops", role: "Editor", last: "2 hours ago", origin: "Google", prov: false },
      { login: "sam.okafor", email: "sam@acme.io", name: "Sam Okafor", orgs: "Acme Ops · Northwind", role: "Viewer", last: "Yesterday", origin: "GitHub", prov: true },
      { login: "lee.chan", email: "lee@globex.io", name: "Lee Chan", orgs: "Globex", role: "Viewer", last: "12 days ago", origin: "Local", prov: false },
      { login: "svc-ci", email: "—", name: "ci pipeline", orgs: "Acme Ops", role: "Editor", last: "Never", origin: "Service account", prov: true },
    ];
    return `
      <div class="page-head">
        <div><h1>Users</h1><p>Manage console users</p></div>
        <div class="page-head__actions"><button class="btn btn--primary">New user</button></div>
      </div>
      <div class="panel">
        <div class="log-toolbar">
          <div class="logs-search">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><line x1="21" y1="21" x2="16.7" y2="16.7"/></svg>
            <input class="logs-search__input" id="user-search" placeholder="Search users by login, email or name…" />
          </div>
          <div class="seg" id="user-role">
            <button class="seg__btn is-active" data-role="all">All</button>
            <button class="seg__btn" data-role="Admin">Admin</button>
            <button class="seg__btn" data-role="Editor">Editor</button>
            <button class="seg__btn" data-role="Viewer">Viewer</button>
          </div>
          <div class="topbar__spacer"></div>
          <span class="tiny muted" id="user-count">${USERS.length} users</span>
        </div>
        <div class="panel__body" style="padding-top:0">
          <table class="table">
            <thead><tr><th>Login</th><th>Email</th><th>Name</th><th>Belongs to</th><th>Role</th><th>Last active</th><th>Origin</th><th>Provisioned</th></tr></thead>
            <tbody id="user-rows">${USERS.map((u) => userRow(u)).join("")}</tbody>
          </table>
        </div>
      </div>`;
  },

  "admin-serviceaccounts"() {
    const SAS = [
      { name: "ci-deploy", id: 12, roles: "Editor", tokens: 2, created: "2026-06-01" },
      { name: "readonly", id: 9, roles: "Viewer", tokens: 1, created: "2026-07-14" },
      { name: "legacy-sync", id: 4, roles: "Editor, Viewer", tokens: 0, created: "2026-03-02" },
    ];
    return `
      <div class="page-head">
        <div><h1>Service accounts</h1><p>Use service accounts to run automated workloads</p></div>
        <div class="page-head__actions"><button class="btn btn--primary">Add service account</button></div>
      </div>
      <div class="panel">
        <div class="log-toolbar">
          <div class="logs-search">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><line x1="21" y1="21" x2="16.7" y2="16.7"/></svg>
            <input class="logs-search__input" placeholder="Search service accounts…" />
          </div>
          <div class="topbar__spacer"></div>
          <span class="tiny muted">${SAS.length} service accounts</span>
        </div>
        <div class="panel__body" style="padding-top:0">
          <table class="table">
            <thead><tr><th></th><th>Account</th><th>ID</th><th>Roles</th><th>Tokens</th><th></th></tr></thead>
            <tbody>${SAS.map((s) => `<tr>
              <td><input type="checkbox" class="cb" /></td>
              <td><b>${s.name}</b><div class="tiny muted">created ${s.created}</div></td>
              <td class="mono">${s.id}</td>
              <td>${s.roles.split(",").map((r) => `<span class="badge scope">${r.trim()}</span>`).join(" ")}</td>
              <td>${s.tokens > 0 ? `<span class="badge">${s.tokens}</span>` : '<span class="muted">0</span>'}</td>
              <td class="row-actions"><button class="btn btn--ghost tiny">Edit</button><button class="btn btn--ghost tiny">Delete</button></td>
            </tr>`).join("")}</tbody>
          </table>
        </div>
      </div>`;
  },

  "admin-auth"() {
    const PROVIDERS = [
      ["SSO", "Sign in with your organisation account", true],
      ["GitHub", "Use GitHub OAuth for authentication", false],
      ["GitLab", "Use GitLab OAuth for authentication", false],
      ["Google", "Use Google OAuth for authentication", true],
      ["Azure AD", "Use Azure Active Directory OAuth", false],
      ["Okta", "Use Okta OAuth for authentication", false],
      ["Generic OAuth", "Configure any OAuth2 provider", false],
      ["SAML", "Sign in via a SAML identity provider", false],
      ["LDAP", "Authenticate users against an LDAP directory", false],
    ];
    return `
      <div class="page-head">
        <div><h1>Authentication</h1><p>Manage your auth settings and configure single sign-on</p></div>
      </div>
      <div class="provider-grid">
        ${PROVIDERS.map(([name, desc, on]) => `<div class="panel provider-card">
          <div class="provider-card__icon">${lockIcon()}</div>
          <div class="provider-card__name">${name}</div>
          <div class="provider-card__desc">${desc}</div>
          <div class="provider-card__foot">
            ${on ? pill("Enabled", "ok") : '<span class="badge">Not enabled</span>'}
            <button class="btn btn--ghost tiny">Configure</button>
          </div>
        </div>`).join("")}
      </div>`;
  },

  settings() {
    const sections = [
      ["general", "General", "Instance identity, region and data retention"],
      ["connection", "Connection", "Server endpoint, WebSocket and polling"],
      ["access", "Operators & Access", "Accounts, MFA and session policy"],
      ["alerting", "Alerting", "Thresholds that drive the alert engine"],
      ["notifications", "Notifications", "FCM, webhook and email delivery"],
      ["danger", "Danger zone", "Destructive organization-wide actions"],
    ];
    const nav = sections.map(([id, label, desc], i) =>
      `<button class="set-nav__item${i === 0 ? " is-active" : ""}" data-sec="${id}">
        <span class="set-nav__label">${label}</span><span class="set-nav__desc">${desc}</span></button>`).join("");

    const body = {
      general: `
        ${field("Instance name", "Shown in the top bar and notification emails", input("instance-name", "Acme Vyzorix"))}
        ${field("Region", "Primary data region — cannot be changed after provisioning", select("region", ["eu-west-1", "us-east-1", "ap-south-1"], "eu-west-1"))}
        ${field("Timezone", "Used for timestamps in the audit log", select("tz", ["UTC", "Europe/London", "America/New_York"], "UTC"))}
        ${field("Telemetry retention", "How long device telemetry frames are kept", select("retention", ["7 days", "30 days", "90 days", "1 year"], "30 days"))}`,
      connection: `
        ${field("Server URL", "GraphQL + REST base endpoint (read-only)", `<code class="code">https://api.vyzorix.io</code>`)}
        ${field("WebSocket", "Real-time telemetry + command channel", `${pill("connected", "ok")} <span class="tiny muted">wss://api.vyzorix.io/ws</span>`)}
        ${field("Auto-reconnect", "Reconnect the WebSocket automatically on drop", switchEl("autoreconnect", true))}
        ${field("Poll interval", "Fallback REST polling when the socket is down", select("poll", ["5s", "15s", "30s", "60s"], "15s"))}`,
      access: `
        ${field("Operator account", "Your login identity", `<span>op@acme.io</span> <span class="badge">Owner</span>`)}
        ${field("Multi-factor auth", "Require TOTP on every user login", switchEl("mfa", true))}
        ${field("Google OAuth", "Link your Google workspace identity", `${pill("linked", "ok")} <button class="btn btn--ghost">Unlink</button>`)}
        ${field("Session timeout", "Sign users out after inactivity", select("session", ["1 hour", "8 hours", "24 hours"], "8 hours"))}
        ${field("IP allow-list", "Restrict console access to these CIDRs", `<input class="input" value="10.0.0.0/8, 203.0.113.0/24" />`)}`,
      alerting: `
        ${field("Battery low", "Fire an alert when a device drops below this level", `<input class="input input--sm" value="10" /> <span class="muted">%</span>`)}
        ${field("Offline after", "Mark a device offline after this silence", `<input class="input input--sm" value="15" /> <span class="muted">min</span>`)}
        ${field("Risk score", "Fire when the device risk score exceeds", `<input class="input input--sm" value="30" />`)}
        ${field("Notify on resolve", "Send a notification when an alert resolves", switchEl("onresolve", false))}`,
      notifications: `
        ${field("FCM push", "Push alerts to the VyzorixAudioRouter app via Firebase", switchEl("fcm", true))}
        ${field("Webhook", "POST alert payloads to your endpoint", `<input class="input" value="https://hooks.acme.io/vyzorix" /> <button class="btn btn--ghost">Test</button>`)}
        ${field("Email digest", "Summary of open alerts and device health", select("digest", ["Off", "Daily", "Weekly"], "Daily"))}
        ${field("Escalation contact", "Secondary contact for critical alerts", input("escalation", "oncall@acme.io"))}`,
      danger: `
        <div class="field field--danger">${fieldRow("Deregister all devices", "Remove every device from this organization. Recordings and telemetry are deleted.", `${canDo("device.delete", "Deregister all", "btn--danger")}`)}</div>
        <div class="field field--danger">${fieldRow("Rotate all API keys", "Immediately revoke and re-issue every service-account key.", `${canDo("keys.manage", "Rotate keys", "btn--danger")}`)}</div>
        <div class="field field--danger">${fieldRow("Delete organization", "Permanently delete this organization and all its data. This cannot be undone.", `${canDo("settings.write", "Delete organization", "btn--danger")}`)}</div>`,
    };

    return `
      <div class="page-head">
        <div><h1>Settings</h1><p>Instance, connection, access, alerting &amp; notifications</p></div>
        <div class="page-head__actions">
          <button class="btn btn--ghost" id="set-discard">Discard</button>
          ${canDo("settings.write", "Save changes", "btn--primary", { id: "set-save" })}
        </div>
      </div>
      <div class="settings">
        <div class="set-body" id="set-body">
          ${sections.map(([id, label], i) => `<section class="set-section${i === 0 ? " is-active" : ""}" data-sec="${id}"><h2 class="set-h">${label}</h2>${body[id]}</section>`).join("")}
        </div>
      </div>`;
  },
};

function input(id, value) { return `<input class="input" id="${id}" value="${value}" />`; }
function select(id, opts, value) {
  return `<select class="select" id="${id}">${opts.map((o) => `<option${o === value ? " selected" : ""}>${o}</option>`).join("")}</select>`;
}
function switchEl(id, on) {
  return `<button class="switch${on ? " is-on" : ""}" id="${id}" role="switch" aria-checked="${on}"><span class="switch__dot"></span></button>`;
}

// The selectable themes (mirrors the registry in packages/ui).
function themeSelect() {
  const builtIn = ["System preference", "Dark", "Light"];
  const extras = [
    "Deuteranopia/Protanopia Dark", "Deuteranopia/Protanopia Light", "Tritanopia Dark", "Tritanopia Light",
    "Desert bloom", "Gilded grove", "Gloom", "Sapphire dusk", "Tron",
  ];
  return `<select class="select" id="pref-theme">
    <option>Default</option>
    <optgroup label="Built-in">${builtIn.map((t) => `<option>${t}</option>`).join("")}</optgroup>
    <optgroup label="Experimental">${extras.map((t) => `<option>${t}</option>`).join("")}</optgroup>
  </select>`;
}
function fieldRow(label, desc, control) {
  return `<div class="field__label"><div class="field__title">${label}</div><div class="field__desc">${desc}</div></div><div class="field__control">${control}</div>`;
}
function field(label, desc, control) { return `<div class="field">${fieldRow(label, desc, control)}</div>`; }

function deviceCard(d) {
  const online = d.status === "online";
  const batteryTone = d.battery < 15 ? "err" : d.battery < 40 ? "warn" : "ok";
  const thermalTone = d.thermal === "hot" ? "err" : d.thermal === "warm" ? "warn" : "off";
  const signal = d.rssi === 0 ? "—" : `${d.rssi} dBm`;
  return `<article class="dev-card" data-status="${d.status}" data-text="${(d.imei + " " + d.model + " " + d.org).toLowerCase()}">
    <header class="dev-card__head">
      <span class="dev-card__id">
        <span class="dot dot--${online ? "ok" : "off"}"></span>
        <span class="dev-card__imei mono">${d.imei}</span>
      </span>
      <span class="badge">${online ? "online" : "offline"}</span>
    </header>
    <div class="dev-card__model">${d.model}<span class="muted"> · ${d.org}</span></div>

    <div class="dev-card__metrics">
      <div class="dev-metric"><span class="dev-metric__k">Battery</span><span class="dev-metric__v">${d.battery}%${d.charging ? " ⚡" : ""}</span>${bar(d.battery, batteryTone)}</div>
      <div class="dev-metric"><span class="dev-metric__k">Thermal</span><span class="dev-metric__v">${d.thermal}</span><span class="dev-metric__dot"><span class="dot dot--${thermalTone}"></span></span></div>
      <div class="dev-metric"><span class="dev-metric__k">Signal</span><span class="dev-metric__v mono">${signal}</span></div>
      <div class="dev-metric"><span class="dev-metric__k">Risk</span><span class="dev-metric__v">${riskBadge(d.risk)}</span></div>
    </div>

    <dl class="dev-card__facts">
      <div><dt>Firmware</dt><dd class="mono">${d.fw}${d.fw !== "0.4.2" ? ' <span class="badge">update</span>' : ""}</dd></div>
      <div><dt>Container</dt><dd class="mono">${d.container}</dd></div>
      <div><dt>Tier</dt><dd>${d.tier}</dd></div>
      <div><dt>Uptime</dt><dd>${d.uptime}</dd></div>
      <div><dt>Storage</dt><dd>${d.storage}%</dd></div>
      <div><dt>Last seen</dt><dd class="${online ? "" : "muted"}">${d.lastSeen}</dd></div>
    </dl>

    <footer class="dev-card__actions">
      <button class="btn btn--ghost tiny">Inspect</button>
      <button class="btn btn--ghost tiny">Terminal</button>
      <button class="btn btn--ghost tiny">Run action</button>
    </footer>
  </article>`;
}

// ---------------------------------------------------------------- app helpers
const APP_STATUS = {
  running: { dot: "ok",   label: "running" },
  stopped: { dot: "off",  label: "stopped" },
  crashed: { dot: "err",  label: "crashed" },
  starting: { dot: "warn", label: "starting" },
};
function appDot(status) { return (APP_STATUS[status] || { dot: "off" }).dot; }
function appStatusPill(status) {
  const s = APP_STATUS[status] || { dot: "off", label: status };
  return pill(s.label, s.dot);
}

// ---------------------------------------------------------------- permission helpers
// A role is the source of truth for permissions, so it gets a pill with a dot
// that reflects how much it can do (viewer < operator < admin < super_admin).
const ROLE_DOT = { viewer: "off", operator: "warn", admin: "ok", super_admin: "ok" };
function rolePill(role) { return pill(role.replace("_", " "), ROLE_DOT[role] || "off"); }
const MEMBER_STATUS_DOT = { active: "ok", service: "off", suspended: "warn", invited: "warn" };
function memberStatusPill(s) { return pill(s, MEMBER_STATUS_DOT[s] || "off"); }
const INVITE_DOT = { pending: "warn", accepted: "ok", expired: "off", revoked: "off" };
function inviteStatusPill(s) { return pill(s, INVITE_DOT[s] || "off"); }

// A per-device update state. Only the semantic dot carries colour; the pill is
// neutral, matching the terminal's treatment.
const UPDATE_STATE = {
  up_to_date:       { label: "up to date",       dot: "ok" },
  update_available: { label: "update available", dot: "warn" },
  downloading:      { label: "downloading",      dot: "warn" },
  installing:       { label: "installing",       dot: "warn" },
  failed:           { label: "failed",           dot: "err" },
  offline:          { label: "offline",          dot: "off" },
};
function updateStateBadge(state) {
  const s = UPDATE_STATE[state] || { label: state, dot: "off" };
  return `<span class="badge"><span class="dot dot--${s.dot}" style="margin-right:4px"></span>${s.label}</span>`;
}
function updateProgress(d) {
  if (d.state === "offline") return '<span class="tiny muted">—</span>';
  if (d.state === "up_to_date") return `${bar(100, "ok")}<span class="tiny muted">current</span>`;
  if (d.state === "update_available") return `${bar(0, "warn")}<span class="tiny muted">awaiting push</span>`;
  return `${bar(d.progress, "info")}<span class="tiny muted">${d.progress}%</span>`;
}
function pushStateBadge(state) {
  const dot = { in_progress: "warn", completed: "ok", failed: "err", cancelled: "off" }[state] || "off";
  return `<span class="badge"><span class="dot dot--${dot}" style="margin-right:4px"></span>${state.replace("_", " ")}</span>`;
}

function eyeIcon() { return window.VyzIcons.ui("eye", 14); }

function userRow(u) {
  const roleDot = u.role === "Admin" ? "err" : u.role === "Editor" ? "warn" : "off";
  return `<tr data-role="${u.role}" data-text="${(u.login + " " + u.email + " " + u.name).toLowerCase()}">
    <td><b>${u.login}</b></td>
    <td class="muted">${u.email}</td>
    <td>${u.name}</td>
    <td class="muted">${u.orgs}</td>
    <td>${pill(u.role, roleDot)}</td>
    <td class="muted">${u.last}</td>
    <td>${u.origin}</td>
    <td>${u.prov ? '<span class="badge">yes</span>' : '<span class="muted">no</span>'}</td>
  </tr>`;
}
function plugIcon() { return window.VyzIcons.ui("plug", 34); }
function lockIcon() { return window.VyzIcons.ui("lock", 22); }

// One entry per log line: a fixed-grid row plus a hidden detail panel that the
// row toggles. The grid columns are mirrored by .logs-head so the header aligns.
// Level is shown by the badge text only — the row itself stays neutral.
function logRows(rows) {
  if (!rows.length) return `<div class="log-empty">No matching log entries.</div>`;
  return rows.map((l) => {
    const meta = {
      time: l.t, level: l.lvl, service: l.svc, device: l.imei, request: l.req,
      trace: l.trace, span: l.span, duration: l.dur, region: l.region, host: l.host,
    };
    const fields = Object.entries(l.fields || {});
    return `<div class="logs-entry">
      <div class="logs-row" data-row tabindex="0">
        <span class="logs-cell logs-cell--time mono">${l.t}</span>
        <span class="logs-cell logs-cell--lvl"><span class="logs-badge logs-badge--${l.lvl}">${l.lvl}</span></span>
        <span class="logs-cell logs-cell--svc mono">${l.svc}</span>
        <span class="logs-cell logs-cell--msg">${l.msg}</span>
        <span class="logs-cell logs-cell--imei mono">${l.imei === "—" ? '<span class="muted">—</span>' : l.imei.slice(-6)}</span>
        <span class="logs-cell logs-cell--trace mono">${l.trace ? l.trace.slice(0, 8) + "…" : '<span class="muted">—</span>'}</span>
        <span class="logs-cell logs-cell--dur mono">${l.dur}</span>
      </div>
      <div class="logs-detail" hidden>
        <div class="logs-detail__grid">
          ${Object.entries(meta).map(([k, v]) => `<div class="logs-detail__k">${k}</div><div class="logs-detail__v mono">${v === "—" ? '<span class="muted">—</span>' : v}</div>`).join("")}
        </div>
        ${fields.length ? `<div class="logs-detail__fields">${fields.map(([k, v]) => `<span class="logs-field"><b>${k}</b>${v}</span>`).join("")}</div>` : ""}
        <div class="logs-detail__actions">
          <button class="btn btn--ghost tiny" data-detail="copy">Copy JSON</button>
          <button class="btn btn--ghost tiny" data-detail="svc">Filter by service</button>
          <button class="btn btn--ghost tiny" data-detail="trace">Show trace</button>
        </div>
      </div>
    </div>`;
  }).join("");
}

// Interactive log-volume histogram: one clickable column per minute, segmented
// by level. Clicking a column filters the stream to that minute (a scrubber).
function logHistogramHTML() {
  const levels = ["error", "warn", "info", "debug"];
  const colors = { error: "#f2495c", warn: "#ff9830", info: "#fb7185", debug: "#5b5b5b" };
  const buckets = logHistogramBuckets();
  const totals = buckets.map((b) => levels.reduce((a, l) => a + (b[l] || 0), 0));
  const max = Math.max(1, ...totals);
  return buckets.map((b) => {
    const total = levels.reduce((a, l) => a + (b[l] || 0), 0);
    const segs = levels
      .map((l) => (b[l] ? `<i class="logs-hb__seg" style="height:${(b[l] / max) * 100}%;background:${colors[l]}"></i>` : ""))
      .join("");
    return `<button class="logs-hb" data-bucket="${b.label}" title="${b.label} · ${total} entries" aria-label="${b.label}, ${total} entries">${segs}</button>`;
  }).join("");
}

// ---------------------------------------------------------------- terminal helpers
function termDot(state) {
  return state === "live" ? "ok" : state === "reattached" ? "warn" : "off";
}
// A neutral pill with only a semantic status dot — no accent fills.
function termStatePill(state) {
  const label = state === "live" ? "connected" : state === "reattached" ? "reattached" : "detached";
  return termPill(termDot(state), label);
}
function termIcon(name) {
  const d = {
    search: '<circle cx="11" cy="11" r="7"/><line x1="21" y1="21" x2="16.7" y2="16.7"/>',
    copy: '<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/>',
    clear: '<path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14"/>',
    record: '<circle cx="12" cy="12" r="6"/>',
    expand: '<path d="M8 3H5a2 2 0 0 0-2 2v3M16 3h3a2 2 0 0 1 2 2v3M8 21H5a2 2 0 0 1-2-2v-3M16 21h3a2 2 0 0 0 2-2v-3"/>',
    collapse: '<path d="M3 8h3a2 2 0 0 0 2-2V3M21 8h-3a2 2 0 0 1-2-2V3M3 16h3a2 2 0 0 1 2 2v3M21 16h-3a2 2 0 0 0-2 2v3"/>',
  }[name];
  return `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">${d}</svg>`;
}
// A neutral pill with a semantic dot. `tone` is ok / warn / err / off.
function termPill(tone, label) {
  return `<span class="badge"><span class="dot dot--${tone}" style="margin-right:4px"></span>${label}</span>`;
}
// The on-screen key bar: the keys a soft keyboard cannot send. `k` is the key id;
// `lit` (when present) is the literal text a printable key inserts.
const TERM_KEYS = [
  { k: "ctrl", label: "Ctrl" }, { k: "alt", label: "Alt" }, { k: "esc", label: "Esc" },
  { k: "tab", label: "Tab" }, { k: "btab", label: "⇧Tab" }, { sep: true },
  { k: "up", label: "↑", icon: true }, { k: "down", label: "↓", icon: true },
  { k: "left", label: "←", icon: true }, { k: "right", label: "→", icon: true }, { sep: true },
  { k: "home", label: "Home" }, { k: "end", label: "End" }, { k: "pgup", label: "PgUp" }, { k: "pgdn", label: "PgDn" }, { sep: true },
  { k: "pipe", label: "|", lit: "|" }, { k: "tilde", label: "~", lit: "~" },
  { k: "slash", label: "/", lit: "/" }, { k: "dash", label: "-", lit: "-" }, { sep: true },
  { k: "c-c", label: "^C" }, { k: "c-d", label: "^D" }, { k: "c-l", label: "^L" }, { k: "c-z", label: "^Z" },
];
function termKeybar() {
  return TERM_KEYS.map((key) => key.sep
    ? '<span class="term-keybar__sep"></span>'
    : `<button class="term-key${key.icon ? " term-key--icon" : ""}" data-key="${key.k}"${key.lit ? ` data-lit="${key.lit}"` : ""} type="button">${key.label}</button>`
  ).join("");
}
function termLines(session) {
  const last = session.transcript.length - 1;
  return session.transcript.map(([kind, text], i) => {
    if (kind === "blank") return `<div class="term-line term-line--blank"></div>`;
    const caret = (i === last && kind === "prompt") ? '<span class="term-caret"></span>' : "";
    return `<div class="term-line term-line--${kind}">${text}${caret}</div>`;
  }).join("");
}

// ---------------------------------------------------------------- file manager
// The File Manager mirrors the container's filesystem exactly: every folder the
// container sees appears here at the same path. It is a view of the same tree the
// terminal walks, not a separate store.
let FM_PATH = ["sdcard"];
// Which tree folders are expanded. A folder's subtree is collapsed until it is
// clicked; a second click collapses it again. The root is always open.
let FM_EXPANDED = new Set([""]);

// Resolve a path (array of segments) to its node in the tree.
function fmNode(path) {
  let node = CONTAINER_FS;
  for (const seg of path) {
    if (!node.children) return null;
    node = node.children.find((c) => c.name === seg);
    if (!node) return null;
  }
  return node;
}
function fmFullPath(path) { return "/" + path.join("/"); }

const FM_CHEVRON = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg>';

function fmTreeHTML(node, path, depth) {
  const isDir = node.type === "dir";
  const kids = isDir ? (node.children || []) : [];
  const key = path.join("/");
  const expanded = isDir && FM_EXPANDED.has(key);
  const cls = ["fm-node"];
  if (isDir) cls.push("fm-node--dir");
  if (node.bind) cls.push("fm-node--bind");
  if (node.writable) cls.push("fm-node--writable");
  if (node.secret) cls.push("fm-node--secret");
  if (node.root) cls.push("fm-node--root");
  const caret = isDir
    ? `<span class="fm-node__caret${expanded ? " is-open" : ""}${kids.length ? "" : " is-empty"}">${kids.length ? FM_CHEVRON : ""}</span>`
    : '<span class="fm-node__caret is-empty"></span>';
  const glyph = isDir
    ? `<span class="fm-node__glyph fm-node__glyph--dir">${window.VyzIcons.icon("folder", 14)}</span>`
    : '<span class="fm-node__glyph fm-node__glyph--file"></span>';
  const head = `<button class="${cls.join(" ")}" data-fm-dir="${key}" title="/${key}" style="padding-left:${6 + depth * 12}px">${caret}${glyph}<span class="fm-node__name">${node.name}</span></button>`;
  if (!isDir) return head;
  const body = `<div class="fm-node__kids"${expanded ? "" : " hidden"}>${kids.map((k) => fmTreeHTML(k, path.concat(k.name), depth + 1)).join("")}</div>`;
  return head + body;
}
function fmTreeRender() {
  const host = document.getElementById("fm-tree");
  if (host) host.innerHTML = fmTreeHTML(CONTAINER_FS, [], 0);
}

function fmCrumbsHTML(path) {
  const parts = [`<button class="fm-crumb" data-fm-path="">/</button>`];
  path.forEach((seg, i) => {
    parts.push(`<span class="fm-crumb-sep">›</span>`);
    parts.push(`<button class="fm-crumb" data-fm-path="${path.slice(0, i + 1).join("/")}">${seg}</button>`);
  });
  return parts.join("");
}

function fmKind(n) {
  if (n.type === "dir") return "directory";
  if (n.mode && n.mode.startsWith("s")) return "socket";
  if (n.mode && n.mode.startsWith("l")) return "symlink";
  return (n.name.split(".").pop() || "file").toUpperCase() + " file";
}
function fmOwner(n) { return n.secret ? "root:root 0600" : n.writable ? "root:root 0755" : "root:root 0644"; }

function fmListHTML(path) {
  const node = fmNode(path);
  if (!node || !node.children || !node.children.length) {
    return `<div class="fm-empty">${node && node.type === "file" ? `${node.name} · ${node.size || ""}` : "This directory is empty."}</div>`;
  }
  const rows = node.children
    .slice()
    .sort((a, b) => (a.type === b.type ? a.name.localeCompare(b.name) : a.type === "dir" ? -1 : 1))
    .map((c) => {
      const isDir = c.type === "dir";
      const glyph = isDir
        ? `<span class="fm-row__glyph fm-row__glyph--dir">${window.VyzIcons.icon("folder", 15)}</span>`
        : `<span class="fm-row__glyph fm-row__glyph--file"></span>`;
      const tags = [
        c.bind ? '<span class="fm-tag">bind</span>' : "",
        c.writable ? '<span class="fm-tag">writable</span>' : "",
        c.readOnly ? '<span class="fm-tag">read-only</span>' : "",
        c.secret ? '<span class="fm-tag fm-tag--warn">secrets</span>' : "",
        c.root ? '<span class="fm-tag">rootfs</span>' : "",
      ].join("");
      return `<tr class="${isDir ? "fm-row--dir" : ""}" ${isDir ? `data-fm-open="${path.concat(c.name).join("/")}"` : ""}>
        <td class="fm-row__name">${glyph}<span>${c.name}</span>${tags}</td>
        <td class="muted">${fmKind(c)}</td>
        <td class="mono muted">${c.size || (isDir ? "—" : "0 B")}</td>
        <td class="mono muted">${fmOwner(c)}</td>
        <td class="row-actions">${isDir ? `<button class="btn btn--ghost tiny" data-fm-open="${path.concat(c.name).join("/")}">Open</button>` : `<button class="btn btn--ghost tiny">Open</button><button class="btn btn--ghost tiny">Download</button>`}</td>
      </tr>`;
    })
    .join("");
  const note = node.note ? `<span class="fm-note">${node.note}</span>` : "";
  return `<table class="fm-table">
    <thead><tr><th>Name</th><th>Type</th><th>Size</th><th>Permissions</th><th></th></tr></thead>
    <tbody>${rows}</tbody>
  </table>${note ? `<div class="fm-list-note">${note}</div>` : ""}`;
}

// ---------------------------------------------------------------- router
// CURRENT_TAB tracks the active in-page tab (third bar), so a nav child click
// can select a tab and the page tabs stay in sync with the nav.
let CURRENT_VIEW = "home";
let CURRENT_TAB = null;
// The active organisation. Operational pages (devices, apps, logs, terminal, …)
// need one; administration, authentication, home and profile do not.
let ACTIVE_ORG = "acme-ops";

// Pages that work without an organisation selected. These are either personal
// (profile, preferences) or the place you *choose* an organisation (orgs).
// Organisation settings are NOT here — they are org-scoped.
const ORG_FREE_VIEWS = new Set(["home", "profile", "orgs",
  "admin-defaultprefs", "admin-provisioning", "admin-users", "admin-serviceaccounts", "admin-auth"]);
function needsOrg(view) { return !ORG_FREE_VIEWS.has(view); }

// The gate shown when an operational page is opened with no organisation.
function orgGateHTML() {
  const suspended = ORGS.filter((o) => o.status !== "active").length;
  return `
    <div class="page-head">
      <div><h1>Select an organisation</h1><p>Devices, apps, logs and the rest are scoped to an organisation. Choose one to continue.</p></div>
    </div>
    <div class="org-picker">
      ${ORGS.map((o) => `<button class="org-pick${o.status === "active" ? "" : " is-disabled"}" data-pick-org="${o.slug}"${o.status === "active" ? "" : " disabled"}>
        <span class="org-pick__avatar">${o.name.split(" ").map((w) => w[0]).join("").slice(0, 2).toUpperCase()}</span>
        <span class="org-pick__id">
          <span class="org-pick__name">${o.name}</span>
          <span class="tiny muted">${o.plan} plan · your role: ${o.role}</span>
        </span>
        <span class="org-pick__stats tiny muted">${o.devices} devices · ${o.members} members${o.status !== "active" ? " · suspended" : ""}</span>
        <span class="org-pick__go">›</span>
      </button>`).join("")}
    </div>
    ${suspended ? `<p class="tiny muted" style="margin-top:12px">${suspended} organisation${suspended === 1 ? " is" : "s are"} suspended and cannot be opened.</p>` : ""}`;
}

function render(view, tab) {
  const fn = VIEWS[view] ?? VIEWS.home;
  CURRENT_VIEW = view;
  const tabs = PAGE_TABS[view] || [["overview", "Overview"]];
  CURRENT_TAB = (tab && tabs.some(([id]) => id === tab)) ? tab : tabs[0][0];
  // Three states, in order: no organisation → pick one; organisation but no
  // capability → refuse; otherwise render.
  const gated = needsOrg(view) && !ACTIVE_ORG;
  const denied = !gated && !canView(view);
  $("#content").innerHTML = gated ? orgGateHTML() : denied ? deniedHTML(view) : fn();
  // Views are re-rendered wholesale; mount the UI icon placeholders each time.
  if (window.VyzIcons && window.VyzIcons.mountUi) window.VyzIcons.mountUi($("#content"));
  $$(".nav__item").forEach((b) => {
    const isChild = b.classList.contains("nav__item--child");
    b.classList.toggle("is-active", isChild
      ? b.dataset.view === view && b.dataset.tab === CURRENT_TAB
      : b.dataset.view === view);
  });
  $$(".nav__group-head").forEach((h) => h.classList.toggle("is-active", h.dataset.view === view));
  // Keep the active group expanded so its children are visible.
  $$(".nav__group").forEach((g) => {
    const head = g.querySelector(".nav__group-head");
    if (head && head.dataset.view === view) { g.classList.remove("is-collapsed"); head.setAttribute("aria-expanded", "true"); }
  });
  applyNavGate();
  renderOrgChip();
  renderRoleChip();
  renderCrumbs(view);
  $(".content").scrollTop = 0;
  if (!gated && !denied) mountCharts(view);
  renderPageTabs(view, gated || denied);
  applyContentMode();
}

// Hide the nav items the current role cannot open. This is the nav-level gate;
// it runs after every render (and after a role switch).
function applyNavGate() {
  $$(".nav__item").forEach((b) => {
    const v = b.dataset.view;
    b.hidden = !canView(v);
  });
  // A group head whose children are all hidden is hidden too.
  $$(".nav__group").forEach((g) => {
    const head = g.querySelector(".nav__group-head");
    const kids = g.querySelectorAll(".nav__item");
    const anyVisible = [...kids].some((k) => !k.hidden);
    g.hidden = !anyVisible;
  });
  // Section labels with nothing under them.
  $$(".nav__section").forEach((sec) => {
    const items = sec.querySelectorAll(".nav__item, .nav__group");
    sec.hidden = ![...items].some((i) => !i.hidden);
  });
}

// The refusal shown when a member opens a page their role cannot see.
function deniedHTML(view) {
  const a = requiredAction(view);
  const label = a ? a.label : "this page";
  return `
    <div class="denied">
      <div class="denied__icon" data-ui="lock" data-ui-size="28"></div>
      <h1 class="denied__title">You don't have access to this page</h1>
      <p class="denied__desc">Opening it needs the <span class="mono">${a ? a.action : "—"}</span> capability${a ? ` (${label})` : ""}, which your role in this organisation does not hold.</p>
      <div class="denied__pills">${pill("Your role: " + currentRoleName().replace("_", " "), "off")}</div>
      <button class="btn btn--primary" data-go="home">Back to home</button>
    </div>`;
}

// The top-bar organisation switcher. Reads the global org state so it is the one
// place a user changes context.
function renderOrgChip() {
  const host = document.getElementById("org-chip");
  if (!host) return;
  const o = ORG_BY_SLUG[ACTIVE_ORG];
  host.innerHTML = o
    ? `<span class="org-chip__avatar">${o.name.split(" ").map((w) => w[0]).join("").slice(0, 2).toUpperCase()}</span>
       <span class="org-chip__name">${o.name}</span>
       <span class="org-chip__caret" data-ui="chevron" data-ui-size="14"></span>`
    : `<span class="org-chip__name muted">No organisation</span>
       <span class="org-chip__caret" data-ui="chevron" data-ui-size="14"></span>`;
  if (window.VyzIcons && window.VyzIcons.mountUi) window.VyzIcons.mountUi(host);
}

// The role-preview chip. The whole console gates on the effective capability set,
// so flipping the role here re-gates the nav, the routes and every control — the
// fastest way to design and check the not-granted states.
function renderRoleChip() {
  const host = document.getElementById("role-chip");
  if (!host) return;
  const role = currentRoleName();
  const previewing = VIEWER_ROLE && VIEWER_ROLE !== currentRole().role;
  host.innerHTML = `<span class="role-chip__label">${previewing ? "Previewing" : "Role"}</span>
    <span class="role-chip__role">${role.replace("_", " ")}</span>
    <span class="role-chip__caret" data-ui="chevron" data-ui-size="14"></span>`;
  host.classList.toggle("is-preview", !!previewing);
  if (window.VyzIcons && window.VyzIcons.mountUi) window.VyzIcons.mountUi(host);
}

// The breadcrumb is `View / Tab` — no "Vyzorix" root. It tracks the active page
// tab so it reads like a path (e.g. `Terminal / Session`).
function renderCrumbs(view) {
  const crumb = document.getElementById("crumb");
  const sep = document.getElementById("crumb-sep");
  const tabEl = document.getElementById("crumb-tab");
  if (!crumb) return;
  const label = {
    home: "Home", devices: "Devices", apps: "Apps", permissions: "Permissions", telemetry: "Telemetry",
    updates: "Updates", alerts: "Alerts", logs: "Logs", terminal: "Terminal", orgs: "Organizations",
    "api-keys": "API Keys", settings: "Settings", profile: "Profile",
    "admin-defaultprefs": "Administration › Default preferences",
    "admin-provisioning": "Administration › Provisioning",
    "admin-users": "Administration › Users",
    "admin-serviceaccounts": "Administration › Service accounts",
    "admin-auth": "Administration › Authentication",
  }[view] || "Home";
  crumb.textContent = label;
  const tabs = PAGE_TABS[view] || [];
  const tabLabel = (tabs.find(([id]) => id === CURRENT_TAB) || [])[1];
  // Only show a tab segment when the page actually has named tabs (not the lone
  // "Overview"), so simple pages stay a single word.
  const showTab = tabs.length > 1 && tabLabel;
  if (sep) sep.hidden = !showTab;
  if (tabEl) { tabEl.hidden = !showTab; tabEl.textContent = showTab ? tabLabel : ""; }
}

// The terminal session fills the whole content area (no page padding) so it reads
// as a full-screen terminal; every other view keeps the normal padded layout.
function applyContentMode() {
  const c = document.querySelector(".content");
  if (c) c.classList.toggle("is-term", CURRENT_VIEW === "terminal" && CURRENT_TAB === "session");
  // Leaving the terminal drops the edge-to-edge override.
  if (!(CURRENT_VIEW === "terminal" && CURRENT_TAB === "session")) document.body.classList.remove("is-term-full");
}

// Per-view in-page tabs shown in the third bar (tabbar).
const PAGE_TABS = {
  home: [["overview", "Overview"]],
  devices: [["overview", "Overview"]],
  apps: [["deployed", "Deployed"], ["catalog", "Catalog"], ["exposure", "Exposure"]],
  permissions: [["members", "Members"], ["review", "Review"], ["invitations", "Invitations"], ["audit", "Audit"]],
  telemetry: [["overview", "Overview"]],
  updates: [["overview", "Overview"], ["versions", "Release versions"], ["pushes", "Push history"]],
  alerts: [["overview", "Overview"]],
  logs: [["entries", "Entries"], ["volume", "Volume"]],
  terminal: [["session", "Session"], ["files", "Files"], ["recordings", "Recordings"], ["limits", "Access & limits"]],
  orgs: [["overview", "Overview"]],
  "api-keys": [["overview", "Overview"]],
  settings: [["general", "General"], ["connection", "Connection"], ["access", "Operators & Access"], ["alerting", "Alerting"], ["notifications", "Notifications"], ["danger", "Danger zone"]],
  profile: [["info", "Profile"], ["preferences", "Preferences"], ["sessions", "Sessions"]],
  "admin-defaultprefs": [["overview", "Overview"]],
  "admin-provisioning": [["repos", "Repositories"], ["connections", "Connections"], ["getting-started", "Get started"]],
  "admin-users": [["overview", "Overview"]],
  "admin-serviceaccounts": [["overview", "Overview"]],
  "admin-auth": [["overview", "Overview"]],
};

function renderPageTabs(view, gated) {
  const host = document.getElementById("page-tabs");
  if (!host) return;
  // A gated page has no in-page tabs — it is just the organisation picker.
  if (gated) { host.innerHTML = ""; return; }
  const tabs = PAGE_TABS[view] || [["overview", "Overview"]];
  host.innerHTML = tabs
    .map(([id, label]) => `<button class="appbar__tab${id === CURRENT_TAB ? " is-active" : ""}" data-tab="${id}">${label}</button>`)
    .join("");
  host.querySelectorAll(".appbar__tab").forEach((b) => b.addEventListener("click", () => {
    CURRENT_TAB = b.dataset.tab;
    host.querySelectorAll(".appbar__tab").forEach((x) => x.classList.toggle("is-active", x === b));
    syncNavChildren();
    activatePageTab(view, b.dataset.tab);
    renderCrumbs(view);
    applyContentMode();
  }));
  // Ensure the selected tab's panel is the visible one.
  activatePageTab(view, CURRENT_TAB);
}

// Mirror the active tab into the nav's child items (Logs / Terminal groups).
function syncNavChildren() {
  $$(".nav__item--child").forEach((b) => {
    b.classList.toggle("is-active", b.dataset.view === CURRENT_VIEW && b.dataset.tab === CURRENT_TAB);
  });
}

// Route a page tab (third bar) to the matching in-page panel.
function activatePageTab(view, tab) {
  const show = (panelSel, attr) => {
    const panels = document.querySelectorAll(panelSel);
    if (!panels.length) return false;
    panels.forEach((p) => { p.hidden = p.dataset[attr] !== tab; });
    return true;
  };
  const showClass = (sectionSel, attr) => {
    const sections = document.querySelectorAll(sectionSel);
    if (!sections.length) return false;
    sections.forEach((s) => s.classList.toggle("is-active", s.dataset[attr] === tab));
    return true;
  };
  if (view === "settings") return showClass(".set-section", "sec");
  if (view === "logs") {
    const ok = show(".log-tabpanel", "panel");
    if (ok && tab === "volume") {
      // Render the histogram once its panel is visible so it sizes correctly.
      const h = document.getElementById("chart-loghist");
      if (h && window.VyzCharts) window.VyzCharts.histogram(h, { height: 240, buckets: logHistogramBuckets() });
    }
    return ok;
  }
  if (view === "profile") return show(".profile-panel", "panel");
  if (view === "terminal") return show(".term-tabpanel", "panel");
  if (view === "permissions") return show(".perm-panel", "panel");
  if (view === "updates") return show(".upd-tabpanel", "panel");
  if (view === "admin-provisioning") return show(".prov-panel", "panel");
}

// Shared histogram data (log volume) used by both the page tab and the initial mount.
function logHistogramBuckets() {
  const hist = [];
  for (let i = 20; i < 42; i++) {
    const b = { label: `12:${(i < 10 ? "0" : "") + i}` };
    b.info = i % 6 === 0 ? 3 : i % 3 === 0 ? 2 : 1;
    b.warn = i % 7 === 0 ? 2 : i % 5 === 0 ? 1 : 0;
    b.error = i === 35 || i === 38 ? 1 : 0;
    b.debug = i % 4 === 0 ? 1 : 0;
    hist.push(b);
  }
  return hist;
}

// Mount SVG charts into the freshly rendered view (after layout is available).
function mountCharts(view) {
  const C = window.VyzCharts;
  const host = (id) => document.getElementById(id);
  const t = (n) => Array.from({ length: n }, (_, i) => `${9 + Math.floor(i / 2)}:${i % 2 ? "30" : "00"}`);

  if (view === "permissions") {
    // Members filter: role segmented control + text search.
    let role = "all";
    let query = "";
    const apply = () => {
      let n = 0;
      document.querySelectorAll("#perm-rows tr").forEach((tr) => {
        const ok = (role === "all" || tr.dataset.role === role) && (!query || tr.dataset.text.includes(query));
        tr.hidden = !ok;
        if (ok) n++;
      });
      const count = document.getElementById("perm-count");
      if (count) count.textContent = `${n} of ${MEMBERS.length} members`;
    };
    const seg = document.getElementById("perm-roles");
    if (seg) seg.addEventListener("click", (e) => {
      const b = e.target.closest(".seg__btn");
      if (!b) return;
      role = b.dataset.role;
      [...seg.querySelectorAll(".seg__btn")].forEach((x) => x.classList.toggle("is-active", x === b));
      apply();
    });
    const search = document.getElementById("perm-search");
    if (search) search.addEventListener("input", () => { query = search.value.trim().toLowerCase(); apply(); });

    // Grant / revoke: a change of role or a removal, both audited.
    const rows = document.getElementById("perm-rows");
    if (rows) rows.addEventListener("click", (e) => {
      const b = e.target.closest("[data-perm]");
      if (!b) return;
      const who = b.dataset.who;
      if (b.dataset.perm === "revoke") {
        const tr = b.closest("tr");
        if (tr) tr.classList.add("is-removed");
        b.disabled = true;
        showToast("Access revoked", `${who} removed from the organization · audited`);
      } else {
        showToast("Change role", `Choose a new role for ${who} (mock)`);
      }
    });
    const invite = document.getElementById("perm-invite");
    if (invite) invite.addEventListener("click", () => showToast("Invite member", "Send an invitation with a role (mock)"));
  }

  if (view === "home") {
    // Each home tabbed card switches its own tab bar + body, like the
    // HomeTabbedCard.
    document.querySelectorAll("[data-g-tabs]").forEach((bar) => {
      bar.addEventListener("click", (e) => {
        const b = e.target.closest("[data-g-tab]");
        if (!b) return;
        const id = b.dataset.gTab;
        [...bar.querySelectorAll("[data-g-tab]")].forEach((x) => x.classList.toggle("is-active", x === b));
        const card = bar.closest(".g-card");
        card.querySelectorAll("[data-g-panel]").forEach((p) => { p.hidden = p.dataset.gPanel !== id; });
      });
    });
    const ask = document.querySelector(".g-home__ask-input");
    if (ask) ask.addEventListener("keydown", (e) => {
      if (e.key === "Enter") { e.preventDefault(); showToast("Ask Vyzorix", ask.value.trim() || "Ask about your devices (mock)"); }
    });
  }

  if (view === "devices") {
    // Client-side filter: status segmented control + text search over the cards.
    let status = "all";
    let query = "";
    const apply = () => {
      let n = 0;
      document.querySelectorAll(".dev-card").forEach((c) => {
        const ok = (status === "all" || c.dataset.status === status) && (!query || c.dataset.text.includes(query));
        c.hidden = !ok;
        if (ok) n++;
      });
      const count = document.getElementById("dev-count");
      if (count) count.textContent = `${n} of ${DEVICES.length} devices`;
    };
    const seg = document.getElementById("dev-status");
    if (seg) seg.addEventListener("click", (e) => {
      const b = e.target.closest(".seg__btn");
      if (!b) return;
      status = b.dataset.status;
      [...seg.querySelectorAll(".seg__btn")].forEach((x) => x.classList.toggle("is-active", x === b));
      apply();
    });
    const search = document.getElementById("dev-search");
    if (search) search.addEventListener("input", () => { query = search.value.trim().toLowerCase(); apply(); });
  }

  if (view === "telemetry") {
    if (host("chart-rssi")) C.lineChart(host("chart-rssi"), {
      height: 200, unit: " dBm", legend: false,
      xLabels: ["12:00", "12:10", "12:20", "12:30", "12:40"],
      series: [{ name: "RSSI", color: "#fb7185", values: [-70, -66, -62, -64, -60, -58, -59, -57, -55, -56, -54, -58, -60, -57] }],
    });
    if (host("chart-batt")) C.lineChart(host("chart-batt"), {
      height: 200, unit: "%", legend: false,
      xLabels: ["12:00", "12:10", "12:20", "12:30", "12:40"],
      series: [{ name: "Battery", color: "#73bf69", values: [92, 91, 90, 90, 89, 88, 88, 87, 87, 86, 86, 87, 87, 87] }],
    });
    if (host("chart-heat")) C.heatmap(host("chart-heat"), {
      rows: 4, cols: 12,
      rowLabels: ["451", "452", "453", "454"],
      colLabels: ["12:00", "", "12:10", "", "12:20", "", "12:30", "", "12:40", "", "12:50", ""],
      values: [
        [0.1, 0.15, 0.2, 0.25, 0.2, 0.3, 0.35, 0.3, 0.25, 0.2, 0.15, 0.1],
        [0.2, 0.3, 0.4, 0.5, 0.55, 0.6, 0.5, 0.45, 0.4, 0.35, 0.3, 0.25],
        [0.6, 0.7, 0.85, 0.9, 0.8, 0.7, 0.6, 0.5, 0.45, 0.4, 0.35, 0.3],
        [0.4, 0.5, 0.6, 0.7, 0.75, 0.8, 0.7, 0.6, 0.5, 0.45, 0.4, 0.35],
      ],
    });
  }

  if (view === "updates") {
    if (host("chart-updstates")) C.barChart(host("chart-updstates"), {
      height: 200,
      data: [
        { label: "up to date", value: UPDATE_DEVICES.filter((d) => d.state === "up_to_date").length, color: "#5b5b5b" },
        { label: "available", value: UPDATE_DEVICES.filter((d) => d.state === "update_available").length, color: "#ff9830" },
        { label: "downloading", value: UPDATE_DEVICES.filter((d) => d.state === "downloading").length, color: "#fade2a" },
        { label: "installing", value: UPDATE_DEVICES.filter((d) => d.state === "installing").length, color: "#fb7185" },
        { label: "offline", value: UPDATE_DEVICES.filter((d) => d.state === "offline").length, color: "#414141" },
      ],
    });
  }

  if (view === "alerts") {
    if (host("chart-alerts")) C.barChart(host("chart-alerts"), {
      height: 200,
      data: [
        { label: "batt", value: 9, color: "#f2495c" },
        { label: "offline", value: 7, color: "#ff9830" },
        { label: "thermal", value: 6, color: "#fade2a" },
        { label: "risk", value: 5, color: "#fb7185" },
        { label: "fw", value: 4, color: "#73bf69" },
      ],
    });
    if (host("chart-mtta")) C.lineChart(host("chart-mtta"), {
      height: 200, unit: "m", xLabels: ["12:00", "12:10", "12:20", "12:30", "12:40"],
      series: [{ name: "MTTA", color: "#b877d9", values: [3, 2.8, 2.6, 2.4, 2.4, 2.3, 2.2, 2.2, 2.1, 2.1, 2.1, 2.1] }],
    });
  }

  if (view === "logs") {
    if (host("chart-loghist")) C.histogram(host("chart-loghist"), { height: 240, buckets: logHistogramBuckets() });

    // --- Query state -------------------------------------------------------
    const state = {
      levels: new Set(["error", "warn", "info", "debug"]),
      svc: null,          // service facet (single-select)
      field: null,        // required field name
      bucket: null,       // histogram range (a minute label)
      query: "",
      wrap: true,
      expanded: false,
    };

    const logText = (l) =>
      [l.msg, l.svc, l.imei, l.req, l.trace, l.span, l.host, l.region,
       ...Object.entries(l.fields || {}).flat()].join(" ").toLowerCase();

    const apply = () => {
      const rows = LOGS.filter((l) =>
        state.levels.has(l.lvl) &&
        (!state.svc || l.svc === state.svc) &&
        (!state.field || (l.fields && state.field in l.fields)) &&
        (!state.bucket || l.t.startsWith(state.bucket)) &&
        (!state.query || logText(l).includes(state.query)),
      );
      const view = document.getElementById("log-view");
      if (view) view.innerHTML = logRows(rows);
      if (view && state.expanded) view.querySelectorAll(".logs-detail").forEach((d) => { d.hidden = false; });
      const count = document.getElementById("log-count");
      if (count) count.textContent = `${rows.length} of ${LOGS.length} entries`;
      const range = document.getElementById("log-range");
      if (range) {
        range.textContent = state.bucket
          ? `range ${state.bucket} → ${state.bucket} · 1 min`
          : "12:20 → 12:41 · last 22 min";
      }
      const clear = document.getElementById("log-range-clear");
      if (clear) clear.hidden = !state.bucket;
      document.querySelectorAll(".logs-hb").forEach((b) => b.classList.toggle("is-selected", b.dataset.bucket === state.bucket));
    };

    // --- Facets ------------------------------------------------------------
    document.querySelectorAll(".logs-facet__row[data-facet='level']").forEach((b) => b.addEventListener("click", () => {
      const v = b.dataset.val;
      if (state.levels.has(v)) { state.levels.delete(v); b.classList.remove("is-on"); }
      else { state.levels.add(v); b.classList.add("is-on"); }
      apply();
    }));
    document.querySelectorAll(".logs-facet__row[data-facet='svc']").forEach((b) => b.addEventListener("click", () => {
      state.svc = state.svc === b.dataset.val ? null : b.dataset.val;
      document.querySelectorAll(".logs-facet__row[data-facet='svc']").forEach((x) => x.classList.toggle("is-on", x === b && state.svc));
      apply();
    }));
    document.querySelectorAll("[data-facet='field']").forEach((b) => b.addEventListener("click", () => {
      state.field = state.field === b.dataset.val ? null : b.dataset.val;
      document.querySelectorAll("[data-facet='field']").forEach((x) => x.classList.toggle("is-on", x === b && state.field));
      apply();
    }));

    // --- Toolbar -----------------------------------------------------------
    const search = document.getElementById("log-search");
    if (search) search.addEventListener("input", () => { state.query = search.value.trim().toLowerCase(); apply(); });

    const wrapBtn = document.getElementById("log-wrap");
    if (wrapBtn) wrapBtn.addEventListener("click", () => {
      state.wrap = !state.wrap;
      wrapBtn.setAttribute("aria-pressed", String(state.wrap));
      const view = document.getElementById("log-view");
      if (view) view.classList.toggle("is-nowrap", !state.wrap);
    });

    let expanded = false;
    const expandBtn = document.getElementById("log-expand");
    if (expandBtn) expandBtn.addEventListener("click", () => {
      expanded = !expanded;
      state.expanded = expanded;
      expandBtn.textContent = expanded ? "Collapse all" : "Expand all";
      document.querySelectorAll(".logs-entry").forEach((e) => {
        const d = e.querySelector(".logs-detail");
        if (d) d.hidden = !expanded;
      });
    });

    // --- Histogram scrubber ------------------------------------------------
    const hist = document.getElementById("log-histogram");
    if (hist) hist.addEventListener("click", (e) => {
      const b = e.target.closest(".logs-hb");
      if (!b) return;
      state.bucket = state.bucket === b.dataset.bucket ? null : b.dataset.bucket;
      apply();
    });
    const clearRange = document.getElementById("log-range-clear");
    if (clearRange) clearRange.addEventListener("click", () => { state.bucket = null; apply(); });

    // --- Row expand + detail actions --------------------------------------
    const view = document.getElementById("log-view");
    if (view) {
      view.addEventListener("click", (e) => {
        const act = e.target.closest("[data-detail]");
        if (act) {
          e.stopPropagation();
          const a = act.dataset.detail;
          showToast(
            a === "copy" ? "Copied" : a === "svc" ? "Filtered" : "Trace",
            a === "copy" ? "Entry copied as JSON" : a === "svc" ? "Filtered by this service" : "Opened the trace view (mock)",
          );
          return;
        }
        const row = e.target.closest("[data-row]");
        if (!row) return;
        const detail = row.parentElement.querySelector(".logs-detail");
        if (detail) detail.hidden = !detail.hidden;
      });
      view.addEventListener("keydown", (e) => {
        if (e.key !== "Enter" && e.key !== " ") return;
        const row = e.target.closest("[data-row]");
        if (!row) return;
        e.preventDefault();
        const detail = row.parentElement.querySelector(".logs-detail");
        if (detail) detail.hidden = !detail.hidden;
      });
    }

    apply();
  }

  if (view === "terminal") {
    const active = TERM_LIST.find((s) => s.id === TERM_ACTIVE) || TERM_LIST[0];
    const screen = document.getElementById("term-screen");
    const lines = document.getElementById("term-lines");
    if (screen) screen.scrollTop = screen.scrollHeight;

    const tabs = document.getElementById("term-tabs");
    if (tabs) tabs.addEventListener("click", (e) => {
      const close = e.target.closest("[data-close]");
      if (close) {
        const id = close.dataset.close;
        TERM_LIST = TERM_LIST.filter((s) => s.id !== id);
        if (TERM_ACTIVE === id) TERM_ACTIVE = TERM_LIST[0] ? TERM_LIST[0].id : "";
        render("terminal");
        showToast("Session closed", `Shell reaped for ${id}`);
        return;
      }
      const tab = e.target.closest(".term-tab");
      if (tab) { TERM_ACTIVE = tab.dataset.sess; render("terminal"); }
    });

    document.querySelectorAll("[data-term-act]").forEach((b) => b.addEventListener("click", () => {
      const act = b.dataset.termAct;
      if (act === "clear") {
        if (lines) lines.innerHTML = "";
        showToast("Buffer cleared", "The visible transcript was cleared");
      } else if (act === "copy") {
        const txt = active.transcript.map(([, t]) => t.replace(/<[^>]+>/g, "")).join("\n");
        if (navigator.clipboard) navigator.clipboard.writeText(txt).catch(() => {});
        showToast("Copied", "Session transcript copied to the clipboard");
      } else if (act === "record") {
        b.classList.toggle("is-recording");
        showToast("Recording", b.classList.contains("is-recording") ? "Recording this session" : "Recording stopped");
      } else {
        showToast("Find in buffer", "Search the scrollback (mock)");
      }
    }));

    const reconnect = document.getElementById("term-reconnect");
    if (reconnect) reconnect.addEventListener("click", () => {
      const s = TERM_LIST.find((x) => x.id === TERM_ACTIVE);
      if (s) s.state = "reattached";
      render("terminal");
      showToast("Reattached", "Bound to the surviving shell by session id");
    });

    const newBtn = document.getElementById("term-new");
    if (newBtn) newBtn.addEventListener("click", () => {
      const id = "pty_" + Math.random().toString(16).slice(2, 8);
      const dev = DEVICES.find((d) => d.status === "online" && !TERM_LIST.some((s) => s.imei === d.imei)) || DEVICES[0];
      TERM_LIST.push({
        id, tab: dev.imei.slice(-3), imei: dev.imei, model: dev.model, fw: dev.fw,
        state: "live", opened: "now", idle: "0s", cols: 96, rows: 26,
        user: "oscar.porter", org: "Acme Ops", role: "Owner", grant: "shell.execute",
        tier: "extended", expires: "in 2h 00m", recording: "off", tmux: dev.imei.slice(-3),
        transcript: [
          ["banner", "Vyzorix container terminal · proot -0 · Debian GNU/Linux 12 (bookworm) aarch64"],
          ["banner", `session ${id} · device ${dev.imei} · grant shell.execute · user oscar.porter`],
          ["blank", ""],
          ["prompt", "<i>root@vyzorix:/#</i> "],
        ],
      });
      TERM_ACTIVE = id;
      render("terminal");
      showToast("Session opened", `New PTY allocated on ${dev.imei}`);
    });

    if (screen && active && active.state !== "detached") {
      const input = document.getElementById("term-input");
      const promptLine = () => lines && lines.querySelector(".term-line--prompt:last-child");
      const atBottom = () => screen.scrollHeight - screen.scrollTop - screen.clientHeight < 8;
      const scroll = () => { if (atBottom()) screen.scrollTop = screen.scrollHeight; };
      // Only the line the user is typing on keeps a caret, so pressing Enter
      // does not leave a trail of blinking cursors behind it.
      const moveCaret = () => {
        if (!lines) return;
        const old = lines.querySelector(".term-caret");
        if (old) old.remove();
        const last = promptLine();
        if (last) last.insertAdjacentHTML("beforeend", '<span class="term-caret"></span>');
      };

      const write = (text) => {
        if (!lines) return;
        let last = promptLine();
        if (!last) {
          lines.insertAdjacentHTML("beforeend", '<div class="term-line term-line--prompt"><i>root@vyzorix:/#</i> <span class="term-typed"></span></div>');
          last = promptLine();
        }
        let typed = last.querySelector(".term-typed");
        if (!typed) { typed = document.createElement("span"); typed.className = "term-typed"; last.appendChild(typed); }
        typed.textContent += text;
        moveCaret();
        scroll();
      };
      const newline = () => {
        if (!lines) return;
        lines.insertAdjacentHTML("beforeend", '<div class="term-line term-line--prompt"><i>root@vyzorix:/#</i> <span class="term-typed"></span></div>');
        moveCaret();
        scroll();
      };

      // A soft keyboard cannot send the keys a shell needs, so a hidden input
      // proxies keystrokes from the device keyboard and the on-screen key bar
      // sends everything else.
      const press = (e) => {
        if (e.key === "Enter") { e.preventDefault(); newline(); }
        else if (e.key === "Backspace") {
          e.preventDefault();
          const t = promptLine() && promptLine().querySelector(".term-typed");
          if (t && t.textContent) t.textContent = t.textContent.slice(0, -1);
        }
      };
      const focusInput = () => { if (input) { input.value = ""; input.focus(); } };
      screen.addEventListener("click", focusInput);
      if (input) input.addEventListener("keydown", press);
      input && input.addEventListener("input", () => {
        const v = input.value;
        if (v) { write(v); input.value = ""; }
      });

      // On-screen key bar: a printable key inserts its literal; a control key
      // writes a token like ^C and is shown on the buffer.
      const keybar = document.getElementById("term-keybar");
      if (keybar) keybar.addEventListener("click", (e) => {
        const b = e.target.closest("[data-key]");
        if (!b) return;
        e.preventDefault();
        if (b.dataset.lit !== undefined) { write(b.dataset.lit); return; }
        const label = b.textContent.trim();
        write((/^\^/.test(label) ? label : `<${label}>`).toLowerCase());
        scroll();
        focusInput();
      });
    }

    // Full-screen toggle: expands the session edge to edge (and back).
    const expand = document.getElementById("term-expand");
    if (expand) expand.addEventListener("click", () => {
      const on = document.body.classList.toggle("is-term-full");
      expand.innerHTML = termIcon(on ? "collapse" : "expand");
      expand.title = on ? "Exit full screen" : "Full screen";
      expand.setAttribute("aria-label", expand.title);
      const s = document.getElementById("term-screen");
      if (s) s.scrollTop = s.scrollHeight;
    });

    // File manager: navigate the container's filesystem. The tree expands a
    // folder on click and collapses it on the next click; the listing follows the
    // selected folder.
    const fmHost = document.getElementById("fm-list");
    if (fmHost) {
      const panel = document.querySelector('.term-tabpanel[data-panel="files"]');

      const renderList = () => {
        document.getElementById("fm-crumbs").innerHTML = fmCrumbsHTML(FM_PATH);
        document.getElementById("fm-list").innerHTML = fmListHTML(FM_PATH);
        const node = fmNode(FM_PATH);
        const note = document.getElementById("fm-note");
        if (note) note.textContent = node && node.note ? node.note : "";
        document.querySelectorAll(".fm-node").forEach((n) => n.classList.toggle("is-active", n.dataset.fmDir === FM_PATH.join("/")));
        const active = document.querySelector(".fm-node.is-active");
        if (active && active.scrollIntoView) active.scrollIntoView({ block: "nearest" });
      };

      // Open a folder in the listing, and reveal it in the tree (expanding each
      // ancestor) so the two stay in step.
      const openFolder = (path) => {
        FM_PATH = path;
        let acc = [];
        FM_EXPANDED.add("");
        path.forEach((seg) => { FM_EXPANDED.add(acc.join("/")); acc.push(seg); });
        FM_EXPANDED.add(path.join("/"));
        fmTreeRender();
        renderList();
      };

      // Toggle a folder in the tree: expand on the first click, collapse on the
      // second, and select it in the listing.
      const toggleTree = (key) => {
        const path = key.split("/").filter(Boolean);
        if (FM_EXPANDED.has(key)) FM_EXPANDED.delete(key);
        else FM_EXPANDED.add(key);
        FM_PATH = path;
        fmTreeRender();
        renderList();
      };

      panel.addEventListener("click", (e) => {
        const open = e.target.closest("[data-fm-open]");
        if (open) { openFolder(open.dataset.fmOpen.split("/").filter(Boolean)); return; }
        const crumb = e.target.closest("[data-fm-path]");
        if (crumb) { openFolder(crumb.dataset.fmPath.split("/").filter(Boolean)); return; }
        const node = e.target.closest("[data-fm-dir]");
        if (node) { toggleTree(node.dataset.fmDir); return; }
      });

      fmTreeRender();
      renderList();
    }
  }

  if (view === "api-keys") {
    if (host("chart-keyusage")) C.barChart(host("chart-keyusage"), {
      height: 220, unit: "",
      data: [
        { label: "ci-deploy", value: 6200, color: "#fb7185" },
        { label: "vyzorix", value: 5400, color: "#73bf69" },
        { label: "legacy", value: 800, color: "#5b5b5b" },
      ],
    });
    if (host("chart-auth")) C.donutChart(host("chart-auth"), {
      height: 200, centerValue: "99.4%", centerLabel: "success rate",
      segments: [
        { label: "Accepted", value: 12344, color: "#73bf69" },
        { label: "Rejected", value: 78, color: "#f2495c" },
      ],
    });
    // Reveal key ids on demand.
    document.querySelectorAll(".key-reveal").forEach((b) => b.addEventListener("click", () => {
      const span = b.previousElementSibling;
      const full = b.dataset.key;
      const masked = full.slice(0, 6) + "••••••••" + full.slice(-2);
      span.textContent = span.textContent.includes("•") ? full : masked;
    }));
  }

  if (view === "profile") {
    document.querySelectorAll(".switch").forEach((s) => s.addEventListener("click", () => {
      const on = s.classList.toggle("is-on");
      s.setAttribute("aria-checked", String(on));
    }));
    const save = document.getElementById("prefs-save");
    if (save) save.addEventListener("click", () => showToast("Preferences saved", "Your preferences were updated"));
    document.querySelectorAll(".session-revoke").forEach((b) => {
      if (b.classList.contains("is-current")) { b.disabled = true; b.style.opacity = "0.5"; b.style.pointerEvents = "none"; return; }
      b.addEventListener("click", () => {
        const tr = b.closest("tr");
        if (tr) tr.style.opacity = "0.4";
        b.disabled = true;
        showToast("Session revoked", "That signed-in device has been signed out");
      });
    });
  }

  if (view === "admin-users") {
    let role = "all", q = "";
    const apply = () => {
      let n = 0;
      document.querySelectorAll("#user-rows tr").forEach((tr) => {
        const ok = (role === "all" || tr.dataset.role === role) && (!q || tr.dataset.text.includes(q));
        tr.hidden = !ok; if (ok) n++;
      });
      const c = document.getElementById("user-count"); if (c) c.textContent = `${n} users`;
    };
    const seg = document.getElementById("user-role");
    if (seg) seg.addEventListener("click", (e) => {
      const b = e.target.closest(".seg__btn"); if (!b) return;
      role = b.dataset.role;
      [...seg.querySelectorAll(".seg__btn")].forEach((x) => x.classList.toggle("is-active", x === b));
      apply();
    });
    const s = document.getElementById("user-search");
    if (s) s.addEventListener("input", () => { q = s.value.trim().toLowerCase(); apply(); });
  }

  if (view === "admin-defaultprefs") {
    const save = document.getElementById("prefs-save");
    if (save) save.addEventListener("click", () => showToast("Preferences saved", "Organization preferences updated"));
  }

  if (view === "settings") {
    // Section switching is driven by the third-bar page tabs; just wire the switches.
    document.querySelectorAll(".switch").forEach((s) => s.addEventListener("click", () => {
      const on = s.classList.toggle("is-on");
      s.setAttribute("aria-checked", String(on));
    }));
    const save = document.getElementById("set-save");
    if (save) save.addEventListener("click", () => {
      const t = document.getElementById("toast");
      if (t) { t.querySelector("b").textContent = "Settings saved"; t.querySelector(".muted").textContent = "Organization preferences updated"; t.classList.add("is-on"); }
    });
  }
}

// Collapsible nav groups. Like the Administration groups (General / Users and
// access), a head only toggles; its children navigate.
document.querySelectorAll(".nav__group-head").forEach((h) => h.addEventListener("click", () => {
  const g = h.closest(".nav__group");
  const collapsed = g.classList.toggle("is-collapsed");
  h.setAttribute("aria-expanded", String(!collapsed));
}));

$$(".nav__item").forEach((b) =>
  b.addEventListener("click", () => {
    // A child item (Logs / Terminal) carries a data-tab and selects that tab.
    render(b.dataset.view, b.dataset.tab);
    setMenu(false); // auto-hide the sidebar once a menu item is chosen
  }),
);
render("home");

// ------------------------------------------------------------------ org context
// One delegated listener handles everything the views emit as data-* hooks:
// navigation (data-go), the organisation picker (data-pick-org), and app actions
// (data-app-act / data-deploy). This keeps the views declarative.
const setOrg = (slug) => {
  ACTIVE_ORG = slug;
  const o = ORG_BY_SLUG[slug];
  renderOrgChip();
  render(CURRENT_VIEW, CURRENT_TAB);
  showToast("Switched organisation", o ? `Now working in ${o.name}` : "Organisation cleared");
};
const setRole = (role) => {
  VIEWER_ROLE = role || null;
  renderRoleChip();
  render(CURRENT_VIEW, CURRENT_TAB);
  showToast("Role preview", role ? `Now viewing as ${role.replace("_", " ")}` : "Back to your real role");
};
const openRoleMenu = () => {
  const real = currentRole().role;
  const items = [
    { k: "real", label: `My real role (${real.replace("_", " ")})`, icon: window.VyzIcons.ui("user", 16), active: !VIEWER_ROLE, go: () => setRole(null) },
    null,
    ...ROLES.slice().reverse().map((r) => ({
      k: r, label: r.replace("_", " "),
      sub: `${capabilitiesForRole(r).size} capabilities`,
      icon: window.VyzIcons.ui(ROLE_DOT[r] === "ok" ? "check" : "circle", 16),
      active: currentRoleName() === r, go: () => setRole(r),
    })),
  ];
  if (window.vxSheet) window.vxSheet.open("Preview as a role", "Re-gates the whole console", items);
};
const openOrgMenu = () => {  const items = [
    ...ORGS.map((o) => ({
      k: o.slug, label: o.name, sub: `${o.plan} · ${o.role}${o.status !== "active" ? " · suspended" : ""}`,
      icon: `<span class="dot dot--${o.status === "active" ? "ok" : "off"}"></span>`,
      active: o.slug === ACTIVE_ORG, disabled: o.status !== "active",
      go: () => setOrg(o.slug),
    })),
    null,
    { k: "all", label: "All organisations", icon: window.VyzIcons.ui("orgs", 16), go: () => render("orgs") },
    { k: "none", label: "Clear selection", icon: window.VyzIcons.ui("close", 16), go: () => setOrg("") },
  ];
  if (window.vxSheet) window.vxSheet.open("Switch organisation", "Scopes every operational page", items);
};

document.addEventListener("click", (e) => {
  // Navigation from any element carrying data-go.
  const go = e.target.closest("[data-go]");
  if (go) { render(go.dataset.go); return; }

  // Organisation picker (the gate).
  const pick = e.target.closest("[data-pick-org]");
  if (pick) { setOrg(pick.dataset.pickOrg); return; }

  // App actions.
  const act = e.target.closest("[data-app-act]");
  if (act) {
    const app = APPS.find((a) => a.id === act.dataset.app);
    const kind = act.dataset.appAct;
    if (kind === "open") showToast("Opening app", `${app.name} at ${app.url} (mock)`);
    else if (kind === "logs") showToast("App logs", `${app.name} · tailing the supervisor log (mock)`);
    else if (kind === "start") { app.status = "running"; render("apps"); showToast("App started", `${app.name} is running`); }
    else if (kind === "stop") { app.status = "stopped"; render("apps"); showToast("App stopped", `${app.name} stopped`); }
    else if (kind === "restart") { app.status = "running"; render("apps"); showToast("App restarted", `${app.name} restarted`); }
    return;
  }

  // Deploy a template.
  const dep = e.target.closest("[data-deploy]");
  if (dep) {
    const t = APP_TEMPLATES.find((x) => x.id === dep.dataset.deploy);
    showToast("Deploy app", `Deploying ${t.name} to a device (mock)`);
    return;
  }
});

// The top-bar organisation chip opens the switcher.
const orgChip = document.getElementById("org-chip");
if (orgChip) orgChip.addEventListener("click", (e) => { e.stopPropagation(); openOrgMenu(); });
const roleChip = document.getElementById("role-chip");
if (roleChip) roleChip.addEventListener("click", (e) => { e.stopPropagation(); openRoleMenu(); });

// Second-bar navigation tabs are rendered per view (see renderPageTabs) and wired
// there; only the global search is wired here.
const globalSearch = document.getElementById("global-search");
if (globalSearch) globalSearch.addEventListener("keydown", (e) => {
  if (e.key !== "Enter") return;
  const q = globalSearch.value.trim();
  if (!q) return;
  const v = /alert/i.test(q) ? "alerts" : /log/i.test(q) ? "logs" : /device|imei/i.test(q) ? "devices" : /update|ota/i.test(q) ? "updates" : "home";
  render(v);
  showToast("Search", `Showing results for “${q}”`);
});

// Inject the UI icon SVGs into every [data-icon] placeholder.
if (window.VyzIcons) { window.VyzIcons.mountIcons(); window.VyzIcons.mountUi(); }

// Mega-menu overlay (auto-hiding drawer): open/close, no icon rail.
const chromeEl = $(".chrome");
const setMenu = (open) => chromeEl.classList.toggle("is-menu-open", open);
const isMenuOpen = () => chromeEl.classList.contains("is-menu-open");

$("#menu-toggle").addEventListener("click", () => setMenu(!isMenuOpen()));
$("#menu-close").addEventListener("click", () => setMenu(false));
$(".scrim").addEventListener("click", () => setMenu(false));
const customiseBtn = document.getElementById("menu-customise");
if (customiseBtn) customiseBtn.addEventListener("click", () => showToast("Customise navigation", "Reorder or hide menu items (mock)"));
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") {
    setMenu(false); setUserMenu(false);
    // Escape also exits the terminal's edge-to-edge full screen.
    if (document.body.classList.contains("is-term-full")) {
      document.body.classList.remove("is-term-full");
      const ex = document.getElementById("term-expand");
      if (ex) { ex.innerHTML = termIcon("expand"); ex.title = "Full screen"; }
    }
  }
});

// Top-bar user menu (profile / preferences / sign out).
const userEl = document.getElementById("user");
const userBtn = document.getElementById("user-btn");
const userMenu = document.getElementById("user-menu");
const setUserMenu = (open) => {
  userMenu.hidden = !open;
  userBtn.setAttribute("aria-expanded", String(open));
};
userBtn.addEventListener("click", (e) => { e.stopPropagation(); setUserMenu(userMenu.hidden); });
document.addEventListener("click", (e) => { if (!userEl.contains(e.target)) setUserMenu(false); });
userMenu.addEventListener("click", (e) => {
  const item = e.target.closest(".user-menu__item");
  if (!item) return;
  setUserMenu(false);
  if (item.dataset.go) render(item.dataset.go, item.dataset.tab);
  else if (item.dataset.action === "signout") showToast("Signed out", "You have been signed out (mock)");
});

function showToast(title, body) {
  const t = document.getElementById("toast");
  if (!t) return;
  t.querySelector("b").textContent = title;
  t.querySelector(".muted").textContent = body;
  t.classList.add("is-on");
  clearTimeout(showToast._t);
  showToast._t = setTimeout(() => t.classList.remove("is-on"), 2600);
}
