# Platform documentation — index

**Status:** orientation. Start here when you are looking for a document and do not know
which one you want.

This is the index for the **platform** documentation: the device, the container, the CLI,
deploy and secrets. It lives in `DOCUMENTATION/` alongside the server's own operational
docs, and beside the older frontend and backend specifications in `documents/`, which it
does not replace. The device repository's documentation is in `Vyzorix-android/doc/`.

If you are new, read `VYZORIX_PLATFORM_OVERVIEW.md` first — it is one page and it makes
everything else read faster.

---

## Start here

| Document | Read it when |
|:--|:--|
| `VYZORIX_PLATFORM_OVERVIEW.md` | You are new, or you have lost sight of how the pieces fit. |
| `VYZORIX_GLOSSARY.md` | A word is unfamiliar, or a word seems to mean something other than what you expect. |
| `VYZORIX_DOCS_INDEX.md` | You are here. |

---

## By task

### I am setting up a device

| Document | What it gives you |
|:--|:--|
| `DEVICE_ONBOARDING.md` | The whole path: install, first run, pairing code, claim, config delivery. |
| `APK_DISTRIBUTION.md` | Getting the APK onto the phone, and how updates work after that. |
| `CONTAINER_FILESYSTEM.md` | What the device looks like once the container is up. |
| `CONTAINER_TERMINAL.md` | Opening a shell into the container. |

### I am deploying an application

| Document | What it gives you |
|:--|:--|
| `CLI_REFERENCE.md` | The `vyzorix` command: login, deploy, secrets, exit codes. |
| `MANIFEST_REFERENCE.md` | `vyzorix.toml`, field by field. |
| `DEPLOY_PIPELINE.md` | What happens to the manifest, stage by stage, and what a failure costs. |
| `DEPLOY_SERVER_API.md` | The endpoints a deploy touches. |
| `DEPLOY_CREDENTIALS.md` | CI tokens, so a pipeline can deploy without an operator. |
| `DEPLOY_SECRETS.md` | The encrypted side channel for values that must not be in the config. |
| `APP_EXPOSURE.md` | Reaching an app over the network. |

### I am working on the container

| Document | What it gives you |
|:--|:--|
| `CONTAINER_FILESYSTEM.md` | Every folder, host side and container side, and the bind rules. |
| `CONTAINER_ANDROID_APIS.md` | Calling Android APIs from inside the container, and the capability model. |
| `CONTAINER_TERMINAL.md` | The interactive shell, the PTY model and reattach. |
| `APP_RUNTIME.md` | The supervisor, `apps.json`, the `app-*` CLI. |

### I am working on the device↔server path

| Document | What it gives you |
|:--|:--|
| `DEVICE_ONBOARDING.md` | Enroll, the pairing code, identity and trust. |
| `DEVICE_COMMANDS_AND_SIGNING.md` | Commands, and the three signing domains. |
| `TELEMETRY_AND_EVENTS.md` | What the device reports, and what the server does with it. |
| `APK_DISTRIBUTION.md` | OTA updates and the distribution traps. |

### I am reviewing or changing the server

| Document | What it gives you |
|:--|:--|
| `DEPLOY_SERVER_API.md` | The deploy endpoints, their groups and their refusals. |
| `DEPLOY_CREDENTIALS.md` | CI tokens, and how authorisation is scoped to one device. |
| `DOCUMENTATION/` | The server's own operational docs: auth, risk, audit, WebSockets, metrics. |

---

## The complete list, newest first

| Document | One line |
|:--|:--|
| `VYZORIX_PLATFORM_OVERVIEW.md` | How the whole platform fits together; the four layers; the three workflows. |
| `VYZORIX_DOCS_INDEX.md` | This file. |
| `VYZORIX_GLOSSARY.md` | The platform vocabulary, defined. |
| `CLI_REFERENCE.md` | The `vyzorix` CLI: what it is, how to install it, every command. |
| `MANIFEST_REFERENCE.md` | `vyzorix.toml`, every field, and the rules behind them. |
| `DEPLOY_PIPELINE.md` | The deploy path, stage by stage, and the failure model. |
| `DEPLOY_SERVER_API.md` | The deploy endpoints and their authentication groups. |
| `DEPLOY_CREDENTIALS.md` | Device-scoped CI tokens: lifecycle and limits. |
| `DEPLOY_SECRETS.md` | The encrypted secret channel, end to end. |
| `DEVICE_ONBOARDING.md` | From a fresh APK to a registered, configurable device. |
| `APK_DISTRIBUTION.md` | First install, OTA updates, and the private-repo trap. |
| `DEVICE_COMMANDS_AND_SIGNING.md` | Commands, and the three signing domains. |
| `CONTAINER_FILESYSTEM.md` | Every folder and its purpose, host and container side. |
| `CONTAINER_ANDROID_APIS.md` | The bridge: Android APIs from inside the container. |
| `CONTAINER_TERMINAL.md` | The interactive shell, PTY allocation and reattach. |
| `APP_RUNTIME.md` | The supervisor and the `app-*` CLI. |
| `APP_EXPOSURE.md` | `loopback`, `lan`, `cloud`, and the token rule. |
| `TELEMETRY_AND_EVENTS.md` | Telemetry frames and device events. |

---

## Related documentation, elsewhere

| Where | What |
|:--|:--|
| `DOCUMENTATION/` | The server's operational docs: authentication, authorization, risk and confirmation, audit, WebSockets, alerting, metrics, deployment. |
| `GLOSSARY.md` | The *server's* vocabulary (auth, database, risk). Complementary to `VYZORIX_GLOSSARY.md`. |
| `ARCHITECTURE.md` | The server's stack, directory layout, request lifecycle and DI. |
| `COMMAND_EXECUTION.md` | The command lifecycle in depth (the signing summary is in `DEVICE_COMMANDS_AND_SIGNING.md`). |
| `WEBSOCKET_SYSTEM.md` | The WebSocket hub internals, in depth. |
| `Vyzorix-android/doc/` | The device repository's own docs (registration, filesystem access, OTA, layer 3, node). |

---

## A note on conventions

These documents follow a few conventions on purpose, and knowing them helps:

- **"The container" means the Debian userspace.** It used to be called "the engine"; the
  word was changed because a userspace `proot` container is exactly what it is. If you
  find "engine" in an older document, read it as "container".
- **Status lines are honest.** A document that describes something not yet built says so
  at the top. "Shipped" means it is on `main` and tested; "deferred" means it is not here
  and the reason is given.
- **Limits are stated, not implied.** Each document ends with what the feature does *not*
  do, because a limit a reader has to discover is a limit that costs them a day.
- **Code paths are named.** Where a document makes a claim about behaviour, it names the
  file that implements it, so the claim can be checked.
