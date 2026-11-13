# `vyzorix.toml` — the manifest, field by field

**Status:** shipped. The schema lives in the public module
`github.com/VinnsEdesigner/vyzorix-spec`; the server serves it at
`GET /v1/deploy/schema/{version}` and the CLI validates against that served copy.
**Related:** `DEPLOY_PIPELINE.md` (what happens to the file), `CLI_REFERENCE.md` (the
command that uploads it), and `DEPLOY_BUILD.md` (how the artifacts a manifest references
are built). This reference is the authoritative description of the manifest.

This is the reference for the one file a human writes. Read the top half once, then
keep the tables open while you write a manifest.

---

## 1. What the file is

`vyzorix.toml` declares **which applications should run on one device**, and what each
is built from. It is not a config for one program: the platform deploys whatever the
manifest names, and Vfd — the media downloader — is simply the first name in it.

The file is the only artifact a human authors. Everything else — the signed config, the
artifact fetches, the app registry — is produced from it.

---

## 2. A complete example

```toml
[device]
id   = "d_01H..."            # required — the target device
org  = "org_01H..."          # required — the owning organization
name = "field-ops-07"        # optional — a display-name override

# One or more apps. Order is significant: it is the order the config records and
# the order the device applies.
[[app]]
name      = "vfd"
cmd       = "/home/apps/vfd/run.sh"
port      = 8080
autostart = true
expose    = "loopback"
purge     = false

  # Sub-tables and bare keys must precede the first [[app.artifact]]. Once TOML is
  # inside an array-of-tables element, a bare key belongs to that element — so a
  # `pwa = true` written after an [[app.artifact]] would set the *artifact's* field,
  # not the app's, and the manifest would be refused as an unknown key. This is a
  # real TOML rule, not a style preference.

  [app.env]
  MI_PORT = "8080"           # configuration only — safe to commit

  [app.secrets]
  DATABASE_URL = ""          # names only; the values are set with `vyzorix secrets`
  S3_SECRET_KEY = ""

    [app.pwa]
    theme_color = "#be123c"

  [[app.artifact]]
  name   = "engine"
  arch   = "arm64-v8a"
  url    = "https://github.com/.../vyzor-arm64.tar.gz"
  sha256 = "9f2c..."
  size   = 1882104

  [[app.artifact]]
  name   = "engine"
  arch   = "x86_64"
  url    = "https://github.com/.../vyzor-x86_64.tar.gz"
  sha256 = "b71e..."

  [[app.artifact]]
  name   = "console"
  url    = "https://.../console.tar.gz"
  sha256 = "41ab..."
  kind   = "webroot"

[[app]]
name = "wiki"
cmd  = "/home/apps/wiki/run.sh"
port = 9000
expose = "lan"
pwa  = true                  # the boolean form, for an app with no overrides

  [[app.artifact]]
  name   = "site"
  url    = "https://.../wiki.tar.gz"
  sha256 = "7b3d..."
  kind   = "webroot"
```

---

## 3. `[device]`

| Field | Type | Required | Default | Notes |
|:--|:--|:--|:--|:--|
| `id` | string | yes | — | The target device. Must be a device the caller's org owns. |
| `org` | string | yes | — | Checked against the caller's org. |
| `name` | string | no | the server's name | Display only. |

The device id and org in the file are the *declaration*; the `--device` and `--org`
flags are the *invocation*. They must agree, or the CLI refuses — it will not validate
one thing and upload another.

---

## 4. `[[app]]`

| Field | Type | Required | Default | Notes |
|:--|:--|:--|:--|:--|
| `name` | string | yes | — | `[a-z0-9-]{1,32}`, unique in the file. Becomes the directory name and the registry key. |
| `cmd` | string | yes | — | What to run. See §7. |
| `port` | int | no | none | 1–65535. Required when `expose` is not `loopback`. |
| `autostart` | bool | no | `true` | Whether the supervisor starts it. |
| `expose` | enum | no | `loopback` | `loopback` \| `lan` \| `cloud`. `cloud` is refused in v1. |
| `purge` | bool | no | `false` | The only thing that deletes an app's data. |
| `env` | table | no | `{}` | Configuration only. A value that looks like a secret is refused. |
| `secrets` | table | no | `{}` | Keys are secret *names*; every value must be the empty string. |
| `pwa` | bool or table | no | `false` | Whether a webroot gets a generated web-app manifest. |

**An unknown key is a hard error.** A typo must not silently do nothing —
`auto_start` misspelled for `autostart` would otherwise produce an app that never
starts and no message explaining why.

**`name` is the directory name and the registry key.** It is constrained so it cannot
be a path, and it cannot be changed without removing and re-adding the app. Renaming an
app is not supported in v1: the data directory would have to move, and that is a
data-loss surface.

---

## 5. `[app.env]` and `[app.secrets]`

These two tables look similar and are opposites. The difference is the whole reason
the secrets channel exists.

```toml
[app.env]                    # configuration — safe to commit, travels in the config
LOG_LEVEL   = "info"
MI_PORT     = "8080"

[app.secrets]                # names only — values travel on a separate, encrypted channel
DATABASE_URL  = ""           # must be empty; a value here is refused
S3_SECRET_KEY = ""
```

- **`env`** values travel inside the signed config. Because the config is signed,
  logged, served by `GET /v1/deploy/{device}` and shown in the console, a secret in
  `env` is a secret in five places. The compiler refuses an `env` **key** that matches
  a secret-shaped name (`password`, `secret`, `token`, `api_key`, `private_key`,
  `credential`, …) and an `env` **value** that looks like a secret (a PEM block, a JWT,
  a long hex or base64 run, a URL with `user:pass@`).
- **`secrets`** is a table whose **keys are names** and whose values must be empty. The
  names travel in the config; the values are set with `vyzorix secrets set` and are
  encrypted to the device. The table form is required so the secret scan can exempt
  this node structurally, by its position, rather than by pattern-matching a field that
  is *defined* to hold a secret name.

The full model is `DEPLOY_SECRETS.md`.

---

## 6. `[[app.artifact]]`

An artifact is a **reference**, never inline bytes: a URL and a mandatory digest. The
device fetches it, checks it against the digest, and unpacks it.

| Field | Type | Required | Default | Notes |
|:--|:--|:--|:--|:--|
| `name` | string | yes | — | Unique within the app **per `arch`**. |
| `url` | string | yes | — | `https` only. Literal — no template variables. |
| `sha256` | string | **yes** | — | 64 hex characters. Mandatory. |
| `arch` | enum | no | — | `arm64-v8a` \| `armeabi-v7a` \| `x86_64`. Restricts the artifact to one ABI. |
| `size` | int | no | — | Informational; a mismatch is not an error. |
| `kind` | enum | no | `dir` | `dir` \| `webroot`. `webroot` is unpacked and served. |

### 6.1 Why the digest is mandatory

The signature covers the *document*. A `url` with no digest means the bytes at that URL
are trusted because the URL was signed — which is only true if the URL is immutable and
the host is honest. A digest makes the transport irrelevant: the artifact can be served
from anywhere, over anything, and a substituted byte is caught. So a reference without
a `sha256` is refused, and the message names the artifact.

### 6.2 Per-architecture artifacts

A URL is never a template. An earlier draft wrote `vyzor-${abi}.tar.gz` with one
digest, which cannot work: if the device expands `${abi}`, the expanded URL is not what
the server signed; if it does not, the URL is wrong. The same binary for several ABIs is
therefore **several** `[[app.artifact]]` entries — same `name`, different `arch`, each
with its own literal URL and its own digest.

Consequences the compiler enforces:

- **Uniqueness is per `(name, arch)`.** `engine` for `arm64-v8a` and `engine` for
  `x86_64` are one artifact in two builds, not a duplicate. Two entries with the same
  `(name, arch)`, or two unqualified entries with the same `name`, are an error naming
  the artifact.
- **A device with no matching artifact is refused** at apply time with
  `no_artifact_for_arch`. It is a device-side check because only the device knows its
  ABI; the server cannot know whether the fleet holds a 32-bit handset. Compiling a
  manifest whose every `engine` is `arm64-v8a` is legal — it is simply not deployable to
  a 32-bit device, and the refusal says so.

---

## 7. The `cmd` rule

`cmd` must resolve to a real file inside the app's own directory, and it is executed
without a shell.

A manifest that can name any command is remote code execution with a signature. The
signature proves *who* asked; it says nothing about whether the thing asked for is
safe. So the rule is mechanical, and it is checked at compile time **and** at apply
time:

1. The path must be absolute and start with `/home/apps/<app.name>/`.
2. It must contain no shell metacharacters (`; | & $ \` > < ( ) { } * ? ~ !` and
   newline).
3. After resolving symlinks, it must **still** be inside `/home/apps/<app.name>/`. A
   symlink pointing outside is refused.
4. It must exist and be executable at apply time. The server cannot know this (the
   artifact has not been fetched yet), so the device checks it and reports
   `cmd_not_executable` rather than starting something that will fail.

The command is executed by `fork`/`exec` with the argv split on whitespace — no shell,
so a metacharacter that slipped past rule 2 has nothing to interpret it.

---

## 8. `expose` and `port`

| `expose` | Meaning | Needs |
|:--|:--|:--|
| `loopback` | Reachable only from the device itself (default). | nothing |
| `lan` | Reachable from the local network. | a `port`, and a token (the supervisor fails closed on `0.0.0.0` without one) |
| `cloud` | Reachable through the cloud relay. | **refused in v1** — it needs the wildcard-DNS/TLS proxy that is deferred |

`port` is the port the app listens on inside the container. It is required when
`expose` is anything but `loopback`. See `APP_EXPOSURE.md`.

---

## 9. `pwa` and webroots

**A webroot is not a PWA unless it says so.**

| `pwa` | What happens |
|:--|:--|
| absent | **no** `site.webmanifest` is generated |
| `false` | **no** `site.webmanifest` is generated |
| `true` | a manifest is generated from defaults |
| table | a manifest is generated, with every declared field overriding the default |

The boolean form and the `[app.pwa]` table are mutually exclusive — both define the key
`pwa` — so use exactly one. The table implies enabled. Overridable fields include
`name`, `short_name`, `description`, `theme_color`, `background_color`, `display`,
`orientation` and `start_url`.

The manifest is written into the webroot **before the app starts**, so the app serves
it as a static file; nothing about the app's own bytes changes.

---

## 10. What is injected, and what is refused

Three fields are **injected by the server and may not be set by the file**:

- `memberId` — who authorised the config (see `DEPLOY_PIPELINE.md` §6);
- `version` — assigned by the server;
- each app's `dir` — derived from the app name.

A file that sets `memberId` is refused, because a config that names its own authoriser
proves nothing.

---

## 11. What the manifest is *not*

- **Not a secret carrier.** `[app.secrets]` names secrets; `[app.env]` is refused for
  values that look like them. There is no field where a secret belongs.
- **Not a fleet description.** One manifest targets one device. A second device is a
  second deploy.
- **Not a build system.** It references artifacts by digest; it does not build them.
- **Not a rollback tool.** The last good config is retained and restorable, but there
  is no `undeploy` and no "roll back three deploys" — version history is two.
