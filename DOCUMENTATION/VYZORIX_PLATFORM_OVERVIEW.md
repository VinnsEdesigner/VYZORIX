# The Vyzorix Platform — how the pieces fit together

**Status:** orientation. This is the document to read first if you are new to Vyzorix,
or if you have been working on one corner of it and have lost sight of the whole.
**Audience:** engineers, operators and reviewers who need the shape of the system
before they need any single contract.

Every other document in this folder assumes you already know the vocabulary on this
page. Read it once and the rest will read much faster. If a later document disagrees
with this one about *structure*, that document wins — this is a map, not the terrain.

---

## 1. What Vyzorix is, in a paragraph

Vyzorix puts a real Linux userspace on an ordinary, unrooted Android phone and lets an
operator reach it from anywhere. The phone runs a Debian container under `proot`, a
long-lived agent holds a WebSocket to a server, and the server gives an operator a
terminal into that container, a live view of the device's health, and a way to
**deploy applications** into it. The whole thing is built so that the phone can be
offline for days and still behave correctly, because the authority to run something
travels *with* the thing — as a signature the phone verifies itself — rather than
being re-checked against a control plane the phone may not be able to reach.

The mental model is a small, self-contained fleet of Linux machines that happen to be
phones, managed with a workflow that looks like `flyctl` but never assumes the machine
is online.

---

## 2. The four layers

Everything in the device is organised into four layers. The numbering matters, because
the documentation and the code refer to it constantly.

```
┌──────────────────────────────────────────────────────────────────────────┐
│ Layer 4 — Cloud Connector     (Kotlin, inside Layer 1's process)          │
│   holds the one WebSocket to the server; enrolls, heartbeats, receives    │
│   commands, relays the terminal, reports telemetry, applies deploys       │
├──────────────────────────────────────────────────────────────────────────┤
│ Layer 3 — App Runtime         (C, inside the container)                   │
│   the supervisor: starts apps, restarts them on crash, exposes them;      │
│   the `app-*` CLI; `apps.json`; the app proxy                             │
├──────────────────────────────────────────────────────────────────────────┤
│ Layer 2 — Debian Container    (proot + a Debian rootfs)                   │
│   a full Debian userspace with a shell; `android-bin`, the in-container   │
│   client that talks to Layer 1's bridge socket                            │
├──────────────────────────────────────────────────────────────────────────┤
│ Layer 1 — Android Gateway     (Kotlin + NDK)                              │
│   the ONLY layer that touches Android; the foreground service, the boot   │
│   receiver, the watchdog, the PTY broker, the USB broker, `android.sock`, │
│   the API bridge, and the cloud WebSocket owner                           │
└──────────────────────────────────────────────────────────────────────────┘
```

Two rules make the layering load-bearing rather than decorative:

- **Only Layer 1 talks to Android.** Layers 2 and 3 have no Android APIs, no
  `Context`, no permissions. They reach the device through Layer 1's local socket.
- **Only Layer 1 talks to the cloud.** Layers 2 and 3 speak local unix sockets only.
  There is exactly one WebSocket, and Layer 1 owns it. A second client would mean two
  owners of one connection, which is the class of bug the design exists to avoid.

The server mirrors the same idea from the other end: it decides *whether* something may
happen (roles, scopes, tiers), and the device enforces *what* was already decided
(bindings, allowlists). Neither side does the other's job. The concrete form of that
split is in `CONTAINER_ANDROID_APIS.md` §7 (the capability model) and
`DEPLOY_PIPELINE.md` §4 (the authorisation predicate).

---

## 3. The two repositories, and what lives where

| Repository | What it is | Language |
|:--|:--|:--|
| `VYZORIX` (this one) | The server: API, console, deploy compilation and signing, secrets, telemetry, OTA. | Go, plus a TypeScript console |
| `Vyzorix-android` | The device agent: the four layers above, the rootfs build, the app runtime. | Kotlin (Gradle) and C++ |
| `vyzorix-cli` | The operator's command-line tool: `vyzorix login`, `deploy`, `secrets`. | Go |
| `vyzorix-spec` | The shared manifest schema and wire types, so the CLI and the server cannot drift. | Go |

The CLI and the server share one declaration of the manifest shape (`vyzorix-spec`),
and the CLI fetches the JSON Schema from the server at run time rather than embedding
it. That is why a self-hosted server is authoritative for its own clients, and why the
CLI and the server can never disagree about what a valid `vyzorix.toml` is.

---

## 4. The three things a person actually does

Almost all of the platform exists to support three workflows. Each has a document
that walks it end to end.

### 4.1 Install the agent and get a device online

You sideload an APK onto a phone. It comes up headless, generates a keypair, and
shows a short pairing code in a notification. You type that code into the web console,
confirm the device is yours, and pick an organization. The device receives its config
over the WebSocket it is already holding and starts running. No secret ever crosses the
wire; the pairing code is only a rendezvous.

→ `DEVICE_ONBOARDING.md`, and the device-side `Vyzorix-android/doc/REGISTRATION_CONTRACT.md`.

### 4.2 Open a terminal into the container

Once the device is online, an operator opens a terminal from the console. The session
is a PTY allocated on the device (Layer 1), wrapped in `proot` running a Debian login
shell (Layer 2), multiplexed by the cloud connector and streamed to the browser. If the
WebSocket drops, the shell survives inside `tmux` and reattaches when the client
reconnects.

→ `CONTAINER_TERMINAL.md`.

### 4.3 Deploy applications

An operator writes a `vyzorix.toml` that declares what should run, and runs
`vyzorix deploy`. The server validates it, checks that the caller may deploy to that
device, compiles it into a signed config, and delivers it. The device verifies the
signature offline, fetches the artifacts it names, checks each against its digest, and
installs them atomically. A failure changes nothing.

→ `DEPLOY_PIPELINE.md`, `CLI_REFERENCE.md` and `MANIFEST_REFERENCE.md`.

---

## 5. The data path, at a glance

```
   operator                    server                         device
   ────────                    ──────                         ──────
   vyzorix.toml
     │  vyzorix deploy
     ├────────────────────────► POST /v1/deploy
     │                           authenticate + authorise
     │                           compile + sign (server key)
     │                           persist (current + last-good)
     │◄───────────────────────── version, authoriser, artifacts
     │
     │                           ── push (if online) ────────► verify signature
     │                                                            fetch artifacts
     │                                                            verify sha256
     │                                                            apply atomically
     │◄───────────────────────── device_ready {version} ────────── start apps
```

The same shape — server decides, device verifies alone — is used for the terminal, for
telemetry and for OTA updates. Once you have seen it once, the rest is detail.

---

## 6. Vocabulary you will meet immediately

The short list. The long list is in `VYZORIX_GLOSSARY.md`.

| Term | Meaning |
|:--|:--|
| **The container** | The Debian userspace running under `proot` on the device. It used to be called "the engine"; it is a userspace container, and the word now matches the thing. |
| **Layer 1 / 2 / 3 / 4** | The four device layers above. |
| **`android.sock`** | The unix socket Layer 1 serves so the container can call Android APIs. |
| **`android-bin`** | The container-side client for `android.sock`; one binary, one name per API. |
| **The supervisor** | The C process inside the container that runs and watches apps (Layer 3). |
| **`apps.json`** | The supervisor's registry of what should run. |
| **`vyzorix.toml`** | The operator's manifest, the only file a human writes. |
| **The config** | The compiled, signed `vyzorix.v` document the device receives. |
| **Artifact** | A referenced, digest-pinned blob an app is built from (a tarball). |
| **Deploy credential** | A device-scoped CI token, so a pipeline can deploy without an operator session. |
| **Tier** | A device's capability class (unrooted / rooted / advanced), used to cap what may be granted. |

---

## 7. Where to go next

- **New here?** `VYZORIX_DOCS_INDEX.md` lists every document in this folder by task.
- **Setting up a device?** `DEVICE_ONBOARDING.md`, then `CONTAINER_FILESYSTEM.md`.
- **Deploying an app?** `DEPLOY_PIPELINE.md` and `CLI_REFERENCE.md`.
- **Understanding the container?** `CONTAINER_FILESYSTEM.md` and `CONTAINER_ANDROID_APIS.md`.
- **Changing the server?** `DEPLOY_SERVER_API.md` and `CONTAINER_ANDROID_APIS.md`.
