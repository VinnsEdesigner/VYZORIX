# `vyzorix` — the command-line tool

**Status:** shipped. The CLI is its own repository, `github.com/VinnsEdesigner/vyzorix-cli`;
this document is the reference for it, kept beside the server because the CLI and the
server are two halves of one contract.
**Related:** `DEPLOY_PIPELINE.md` (what a deploy does, stage by stage),
`MANIFEST_REFERENCE.md` (the file it uploads), `DEPLOY_SERVER_API.md` (the endpoints the
CLI calls), and `DEPLOY_BUILD.md` (the `build` command and the tool it delegates to).

---

## 1. What it is

`vyzorix` is the user's handle on a Vyzorix server. It does three jobs:

1. **Signs you in** — proves a credential works and remembers it, so the next command
   does not have to repeat it.
2. **Deploys a manifest** — validates a `vyzorix.toml` against the server's own schema,
   uploads it, and reports what the server did.
3. **Manages secrets** — sets, lists and removes the encrypted values an app needs,
   without any value ever appearing on your terminal or in the manifest.

It deliberately does **not** compile anything. The server is authoritative: the CLI
pre-validates so a schema mistake is caught locally with a readable field path, but the
manifest it uploads is the file's bytes, unchanged. The CLI never re-serialises the
manifest, because a re-serialised file is a different file, and the digest and the
signature are computed over the bytes the server receives.

---

## 2. Installing

Two ways, both ending with `vyzorix` on your `PATH`.

**From a release (recommended).** The installer downloads the right binary for your
platform, verifies it against the release's `SHA256SUMS`, and only then puts it on
`PATH` — a corrupted or swapped download is refused rather than installed.

```sh
curl -fsSL https://raw.githubusercontent.com/VinnsEdesigner/vyzorix-cli/main/install.sh | sh
```

Set `VYZORIX_VERSION=v0.1.0` to pin a release, and `PREFIX` to install somewhere other
than `~/.local/bin`.

**From source**, with Go 1.27 or newer:

```sh
go install github.com/VinnsEdesigner/vyzorix-cli/cmd/vyzorix@latest
```

That lands in `$(go env GOPATH)/bin`.

Releases cover `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` and
`windows/amd64`. The build is CGO-free, so all five come from one runner.

Check what you have:

```sh
vyzorix version        # prints e.g. "vyzorix v0.1.0"; a working tree prints "dev"
```

---

## 3. Signing in

```sh
vyzorix login --server https://your-server --api-key KEY [--org ORG] [--device DEV]
```

`login` calls the server first to prove the credential works, and only writes the
config if it does — a bad key leaves nothing behind. On success it prints your
identity (name, role, org, and the device's tier if you named one) and saves the
server, the org and the credential to `~/.config/vyzorix/config.toml`, mode `0600`.

Two kinds of credential are accepted, and you must give exactly one:

| Credential | Flag | Environment | Who it is |
|:--|:--|:--|:--|
| User API key | `--api-key` | `VYZORIX_API_KEY` | A person, or a key acting for one. Carries a role and, once signed in, an org. |
| Deploy credential | `--deploy-credential` | `VYZORIX_DEPLOY_CREDENTIAL` | A CI token, scoped to a single device. No user behind it. |

Giving both is refused rather than guessed at: the server would otherwise have to
choose which identity you meant.

```sh
vyzorix whoami [--device DEV]     # show the saved identity again, without re-saving
vyzorix logout                    # forget the saved credential
```

`whoami` is the read-only twin of `login`: it resolves the credential and prints the
same identity, and is what you run in a script to confirm what a token reaches.

### Flags versus environment versus saved config

Every setting resolves in this order, first hit wins:

1. the flag on the command line,
2. the environment variable (`VYZORIX_SERVER`, `VYZORIX_ORG`, `VYZORIX_API_KEY`,
   `VYZORIX_DEPLOY_CREDENTIAL`),
3. the value saved by `login`.

So a saved config means `deploy` needs only the device, and a CI job overrides the
credential with an environment variable rather than editing a file.

---

## 4. Deploying

```sh
vyzorix deploy --device DEV [--org ORG] [--file vyzorix.toml] [--dry-run]
```

What happens, in order:

1. **Load** the manifest. A file that is not valid TOML fails here, by name.
2. **Reconcile** the flags with the file. If `vyzorix.toml` names a device and
   `--device` names a different one, the command refuses — otherwise it would validate
   one thing and upload another.
3. **Fetch the schema** from the server (`GET /v1/deploy/schema/{version}`) and
   validate the manifest against it. A schema error is reported with the offending
   field path, locally, before anything is uploaded.
4. **Print a summary** of what the manifest declares — the apps and their artifacts.
5. **Upload** the manifest to `POST /v1/deploy`.
6. **Report** the outcome: the version the server assigned, who authorised it, how many
   artifacts, and which apps are known to the catalog versus custom.

`--dry-run` stops after step 5's server-side equivalent: the server compiles and signs
but stores nothing and delivers nothing. It is the way to check that a manifest will be
accepted, and to see the version it *would* get, without touching a device.

Example output:

```
→ Deploying to d_01H...
  2 app(s)
    vfd
      engine  arm64-v8a  1.8 MB
      console  any  size unknown
    wiki
      site  any  size unknown
✓ Deployed version 7

  authorised by  op_01H...
  artifacts      3

  APP    SOURCE
  vfd    known
  wiki   custom
```

`SOURCE` is `known` when the app is in the server's catalog and `custom` when it is
not. `custom` is not an error — the catalog is a review aid, not a gate — but it is
worth noticing, because a custom app is one nobody has classified.

### When a deploy is refused

The server's refusal is printed verbatim, and the exit code is `1` (see §6). Typical
refusals:

| Refusal | What it means |
|:--|:--|
| a validation error naming a field | The manifest does not match the schema. The field is named. |
| `device.deploy is required for this device` | The credential is real but has no grant to deploy here. |
| device not found (`404`) | The device is not in your organization, or does not exist. A `404` rather than a `403` is deliberate: a `403` would confirm the device exists elsewhere. |
| `device is not registered` (`409`) | The device exists but is still pending, or has been deregistered. |
| a secret-scan refusal naming a key | `[app.env]` carried something that looks like a secret. See `DEPLOY_SECRETS.md`. |

---

## 5. Secrets

An app that is itself a server needs credentials. `[app.env]` is refused for those
(the config is signed, logged and served, so a secret in it is a secret in five
places), so secrets travel on their own channel, encrypted to the device. The CLI is
where you put them.

```sh
vyzorix secrets set   --device DEV <app> KEY=VALUE
vyzorix secrets list  --device DEV <app>
vyzorix secrets unset --device DEV <app> KEY
```

- `set` encrypts the value to the device's public key, server-side, and stores the
  ciphertext. The value may contain `=`; only the first one separates the key from the
  value, so a URL or a base64 blob is a valid value.
- `list` prints the **names and generations**, never a value. The device never receives
  a value over any user call; it fetches its own ciphertext and decrypts it locally.
- `unset` removes one key.

A value is never echoed back, never logged, and never returned by any `GET`. `set`
confirms with the *name*:

```
✓ Set DATABASE_URL in vfd
```

The full model — where the value is stored, which key it is wrapped to, what lands on
disk — is `DEPLOY_SECRETS.md`.

---

## 6. Exit codes

| Code | Meaning |
|:--|:--|
| `0` | Success. |
| `1` | The **server refused**. The server's reason is printed verbatim. |
| `2` | A **local** failure: bad flags, an unreadable file, an unreachable server. |

The distinction matters in a pipeline: `1` means the request reached the server and was
rejected on its merits, `2` means it never got that far. A CI job that retries on `2`
and fails the build on `1` is behaving correctly.

---

## 7. Environment variables

| Variable | Used by | Meaning |
|:--|:--|:--|
| `VYZORIX_SERVER` | all | Server base URL. |
| `VYZORIX_ORG` | all | Organization id. |
| `VYZORIX_API_KEY` | `login`, `deploy`, `secrets` | User API key. |
| `VYZORIX_DEPLOY_CREDENTIAL` | `login`, `deploy`, `secrets` | Device-scoped deploy token. |
| `VYZORIX_CONFIG` | all | Override the config file path (useful in CI and tests). |
| `NO_COLOR` / `FORCE_COLOR` | all | Colour control. `NO_COLOR` wins when set. |

---

## 8. The config file

`~/.config/vyzorix/config.toml`, mode `0600` because it holds a credential:

```toml
server = "https://your-server"
org = "org_01H..."
api_key = "..."            # or deploy_credential = "..."
```

Unset fields are omitted, so a later `login` does not have to carry a value it did not
set. `VYZORIX_CONFIG` points at a different file when you want the state somewhere
else.

---

## 9. Design notes worth knowing

- **The schema is fetched, not embedded.** The CLI validates against
  `GET /v1/deploy/schema/{version}` and pins the version. A manifest the CLI accepted
  is a manifest that server accepts; if the server has moved to a new schema version,
  the fetch is a `404` and the CLI refuses rather than validating against a shape the
  server no longer uses.
- **The wire types come from `vyzorix-spec`.** The CLI and the server share one
  declaration of the manifest, so neither can drift from the other.
- **The credential is proved before it is saved.** `login` calls `whoami` first, so a
  bad key leaves no config behind to confuse the next command.
- **Signing is HMAC over the request.** An API-key request carries `X-API-Key` plus a
  signature over method, path, nonce, timestamp and body. The signing secret is
  `sha512(fullKey)`, hex — the same derivation the server stores, so the CLI needs no
  separate secret. A deploy credential is sent as a bearer header instead, because it
  is already a device-scoped secret and has no user to sign as.
