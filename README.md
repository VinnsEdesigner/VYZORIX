<div align="center">

# Vyzorix

The server half of the Vyzorix platform: device fleets, signed command dispatch,
telemetry, OTA updates, and the deploy plane that ships applications to Android
devices running a Debian container.

Go · Gin · SQLite · GraphQL · WebSocket · React

</div>

---

The server manages Android device fleets: device registration, signed command
dispatch, telemetry, OTA updates, alerting, and a multi-tenant organization
model. It exposes a REST API, a GraphQL API, and a WebSocket stream.

It is one half of a two-repo platform. The other half is the device agent,
`Vyzorix-android`, which runs a Debian container on an unrooted phone and holds the
single WebSocket this server speaks to. The shared manifest schema
(`vyzorix-spec`) completes the set.

## The two CLIs

There are two, and the names matter:

- **The Vyzorix CLI** — the device-side tool. It runs **on the phone**, inside the
  container, and turns the device's APIs into commands (`camera`, `battery`,
  `wboot`, …). It ships with the agent, not from here. When an operator opens a
  terminal from the dashboard, this is what they land in.
- **The Deploy CLI** (`vyzorix-cli`, `vyzorix login` / `vyzorix deploy`) — the
  operator tool. It runs **on your machine or in CI**, validates a `vyzorix.toml`
  manifest against this server's schema, and pushes it to a device.

## The terminal, and what an operator may do

A terminal session is **scoped to the operator**. When one is opened, this server
resolves the operator's capability — their **role** intersected with the device's
**tier** and any scoped grant — and delivers it with the session, so the device
enforces it and the CLI shows it:

- the **identity** (operator, name, email, org, role and its human label, expiry);
- the **API allowlist** the session may call.

The device's `DeviceAPICatalog` is the authority for what a session can reach: an
API absent from it is unreachable to every role, so it mirrors the device's whole
surface. `ResolveSessionCapability` composes the role's tags, the scoped engine,
and the tier gate; an empty resolution is a refusal, never a full grant.

## Quick start

```bash
cd apps/api
make all      # install deps, build the Go binary and the frontend
make run      # start the server
```

The server listens on `PORT` (default `3000`). See `apps/api/Makefile` for every
target (`deps`, `build`, `test`, `lint`, `wire`, `dev`, ...).

Required environment variables (see `.env.example`):

- `DATABASE_URL` — SQLite path or Turso URL
- `JWT_SECRET`, `SESSION_SECRET` — 32+ characters each
- `SERVER_API_TOKEN` — protects `/healthz`
- at least one `API_KEY_*` — bootstrap API key
- `ALLOWED_ORIGINS` — your dashboard origin (empty means fail-closed)

## Layout

```
apps/
  api/              Go server (REST + GraphQL + WebSocket)
  VyzoriX_web/      React dashboard
packages/           Shared TypeScript packages
DOCUMENTATION/      Feature and operations documentation
```

## Documentation

New to the platform? Read the
[Platform Overview](./DOCUMENTATION/VYZORIX_PLATFORM_OVERVIEW.md) first — one page,
and everything else reads faster after it. The
[Platform Docs Index](./DOCUMENTATION/VYZORIX_DOCS_INDEX.md) groups every platform
document by task, and the
[Platform Glossary](./DOCUMENTATION/VYZORIX_GLOSSARY.md) defines the vocabulary.

| Platform | Documents |
|:-----|:----------|
| Start here | [Platform Overview](./DOCUMENTATION/VYZORIX_PLATFORM_OVERVIEW.md), [Docs Index](./DOCUMENTATION/VYZORIX_DOCS_INDEX.md), [Glossary](./DOCUMENTATION/VYZORIX_GLOSSARY.md) |
| Deploy | [Pipeline](./DOCUMENTATION/DEPLOY_PIPELINE.md), [Manifest Reference](./DOCUMENTATION/MANIFEST_REFERENCE.md), [Deploy CLI Reference](./DOCUMENTATION/CLI_REFERENCE.md), [Server API](./DOCUMENTATION/DEPLOY_SERVER_API.md), [Credentials](./DOCUMENTATION/DEPLOY_CREDENTIALS.md), [Secrets](./DOCUMENTATION/DEPLOY_SECRETS.md), [App Exposure](./DOCUMENTATION/APP_EXPOSURE.md) |
| Device | [Onboarding](./DOCUMENTATION/DEVICE_ONBOARDING.md), [APK & OTA](./DOCUMENTATION/APK_DISTRIBUTION.md), [Commands & Signing](./DOCUMENTATION/DEVICE_COMMANDS_AND_SIGNING.md), [Telemetry & Events](./DOCUMENTATION/TELEMETRY_AND_EVENTS.md) |
| Container | [Filesystem](./DOCUMENTATION/CONTAINER_FILESYSTEM.md), [Android APIs](./DOCUMENTATION/CONTAINER_ANDROID_APIS.md), [Terminal](./DOCUMENTATION/CONTAINER_TERMINAL.md), [App Runtime](./DOCUMENTATION/APP_RUNTIME.md) — the **Vyzorix CLI** (the device-side command tool the terminal lands you in) is documented in the device repo, `Vyzorix-android/doc/CLI.md`. |

The server's own operational documentation is indexed in
[DOCUMENTATION/README.md](./DOCUMENTATION/README.md).

| Area | Documents |
|:-----|:----------|
| Architecture | [ARCHITECTURE](./DOCUMENTATION/ARCHITECTURE.md), [GLOSSARY](./DOCUMENTATION/GLOSSARY.md) |
| API | [GRAPHQL_API](./DOCUMENTATION/GRAPHQL_API.md), [ERROR_CODES](./DOCUMENTATION/ERROR_CODES.md) |
| Auth & access | [AUTHENTICATION](./DOCUMENTATION/AUTHENTICATION.md), [API_KEYS](./DOCUMENTATION/API_KEYS.md), [ORGANIZATIONS](./DOCUMENTATION/ORGANIZATIONS.md), [SERVICE_ACCOUNTS](./DOCUMENTATION/SERVICE_ACCOUNTS.md) |
| Devices & commands | [DEVICE_MANAGEMENT](./DOCUMENTATION/DEVICE_MANAGEMENT.md), [COMMAND_EXECUTION](./DOCUMENTATION/COMMAND_EXECUTION.md), [RISK_AND_CONFIRMATION](./DOCUMENTATION/RISK_AND_CONFIRMATION.md) |
| Realtime | [REALTIME_COMMUNICATION](./DOCUMENTATION/REALTIME_COMMUNICATION.md), [WEBSOCKET_SYSTEM](./DOCUMENTATION/WEBSOCKET_SYSTEM.md), [LIVE_CHANNELS](./DOCUMENTATION/LIVE_CHANNELS.md) |
| Updates | [UPDATE_SYSTEM](./DOCUMENTATION/UPDATE_SYSTEM.md) |
| Operations | [DEPLOYMENT](./DOCUMENTATION/DEPLOYMENT.md), [SECURITY_HARDENING](./DOCUMENTATION/SECURITY_HARDENING.md), [METRICS](./DOCUMENTATION/METRICS.md), [BACKGROUND_WORKERS](./DOCUMENTATION/BACKGROUND_WORKERS.md) |
| Specs & setup | [SETUP-GUIDE](./SETUP-GUIDE.md), [Docker](./apps/api/README_DOCKER.md), [documents/](./documents/) |

## Contributing

`make lint` and `make test` (from `apps/api`) must pass before a change is
proposed. The CI gate runs `go build`, `go vet`, `golangci-lint`, `go test`, and
the OpenAPI/GraphQL drift checks.
