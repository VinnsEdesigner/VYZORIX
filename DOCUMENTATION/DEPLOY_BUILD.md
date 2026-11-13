# Building artifacts for deploy

**Status:** shipped. `vyzorix-builder` builds and publishes; `vyzorix build` drives it.
**Related:** `BUILD_TOOL_CONTRACT.md` (the design), `DEPLOY_PIPELINE.md` (what happens
to the artifact after it exists), `MANIFEST_REFERENCE.md` §6 (the artifact reference),
`CLI_REFERENCE.md` (the command surface), `DEPLOY_CREDENTIALS.md` (the CI path).

This is the operator's guide to the producer side of a deploy: how to turn a project's
source into the digest-pinned tarballs a `vyzorix.toml` references. Read
`BUILD_TOOL_CONTRACT.md` for *why* the tool is shaped this way; read this for *how to
use it*.

---

## 1. The two files

| File | Owner | Purpose |
|:--|:--|:--|
| `vyzorix.build.toml` | the app developer | **how to build** the project into artifacts |
| `vyzorix.toml` | the operator | **what to deploy**: which artifact, pinned by digest |

They are separate on purpose (`BUILD_TOOL_CONTRACT.md` §1.1). The recipe changes when
the build changes; the manifest changes when the deployment changes. `vyzorix build`
reads the recipe, builds, and **renders** the manifest from the result — so the digests
in `vyzorix.toml` are never typed by hand.

---

## 2. The recipe, by example

```toml
[build]
version = "v1.2.0"
abi     = ["arm64-v8a"]      # omitted -> resolved from --device via the server
suite   = "bookworm"         # the glibc floor the artifact must run against

[[artifact]]
app  = "vfd"
name = "engine"
lang = "cpp"
path = "media-ingest-c"
build = "make"
cmd  = "/home/apps/vfd/engine/run.sh"
kind = "dir"
  [artifact.cpp]
  libs        = ["libcurl", "libsqlite3", "libgstreamer-1.0"]
  glibc_floor = "2.36"
  static      = ["ffmpeg"]

[[artifact]]
app   = "vfd"
name  = "console"
lang  = "node-webroot"
path  = "web"
build = "npm ci && npm run build"
dist  = "dist"
kind  = "webroot"

[[artifact]]
app  = "vfd"
name = "weird"
lang = "containerfile"
file = "deploy/Containerfile.arm64"
out  = "/out/weird.tar.gz"
arch = "arm64-v8a"
```

### Fields

**`[build]`** — settings shared by every artifact.

| Field | Meaning |
|:--|:--|
| `version` | the artifact version; the `v<version>` segment of every filename |
| `abi` | the target ABIs. Omitted, `vyzorix build --device D` resolves them from the device |
| `suite` | the Debian release whose glibc the artifact must run against (default `bookworm`) |
| `os` | the target OS ABI name (default `linux-android`) |

**`[[artifact]]`** — one artifact to build. Artifacts that share `app` become one
`[[app]]` block in the rendered manifest.

| Field | Meaning |
|:--|:--|
| `app` | the manifest app this artifact belongs to (kebab-case) |
| `name` | the artifact name (kebab-case); unique within the app |
| `lang` | the adapter: see §3 |
| `path` | the project-relative source directory (the docker build context) |
| `cmd` | the entrypoint the device runs; becomes the manifest's `cmd` |
| `kind` | `dir` (default) or `webroot` (architecture-free) |
| `build` | the build command; replaces the adapter's default |
| `dist` | the built output directory, for `node-webroot` |
| `file` | the Containerfile, for `lang = "containerfile"` |
| `out` | the archive path inside the image, for `lang = "containerfile"` |
| `arch` | one ABI, when the artifact is single-arch |
| `port`, `expose`, `autostart`, `purge`, `pwa`, `env` | app-level fields; copied to the manifest |

---

## 3. Adapters

One command per runtime; each is a small plan plus a pinned, bundled Dockerfile the
builder writes into the build context. The binary needs no `builders/` directory at run
time.

| `lang` | Produces |
|:--|:--|
| `golang` | a static binary (CGO off); nothing to resolve on the device |
| `rust` | a musl static binary |
| `cpp` | a cross-compiled binary plus its shared-library closure, glibc floor asserted |
| `node` | a Node server app plus production `node_modules` |
| `node-webroot` | a static site (architecture-free; the filename arch segment is `all`) |
| `deno` | one self-contained binary via `deno compile` |
| `bun` | one self-contained binary via `bun build --compile` |
| `zig` | a static binary via the zig cross compiler |
| `dotnet` | a self-contained publish; no runtime on the device |
| `ruby` | the app plus vendored gems |
| `php` | the app plus a vendored `vendor/` (no Composer on the device) |
| `python` | the app plus wheels for the target platform |
| `jvm` | a GraalVM native image by default; `[artifact.opts] jvm = "jar"` for a fat jar |
| `containerfile` | the escape hatch: your own Containerfile emits `/out/*.tar.gz` |

Adding a runtime is adding one file under `modules/adapter/` and one
`builders/<lang>.Dockerfile`. Nothing below the adapter layer learns a language exists.

---

## 4. The command

```bash
# Build for a device, resolving the ABI from the server, and render the manifest.
vyzorix build --device d_01H... --file vyzorix.build.toml --manifest-out vyzorix.toml

# Build and deploy in one step.
vyzorix deploy --device d_01H... --recipe vyzorix.build.toml --build

# Preview without Docker.
vyzorix-builder plan < request.json
```

The builder is found via `$VYZORIX_BUILDER`, else on `PATH`.

`vyzorix build` runs, in order:

1. read the recipe; resolve the target ABIs (the recipe's `abi`, or `--device` → the
   server's `whoami`);
2. validate the recipe against the served schema (`GET /v1/deploy/build-schema/v1`);
3. delegate to `vyzorix-builder`, which per artifact picks the adapter, builds in a
   pinned bookworm container, resolves the closure, asserts the glibc floor, writes a
   deterministic tarball, and publishes it;
4. render `vyzorix.toml` from the recipe + the result + the device/org;
5. `vyzorix deploy` runs the **existing** validate → sign → apply path, unchanged.

---

## 5. Publishing

The default backend is **GHCR**, as an OCI registry. The builder:

1. pushes the tarball as an **OCI blob**, whose digest **is** the `sha256` the manifest
   pins;
2. pushes a small **OCI image manifest** that references the blob as its single layer
   and records the artifact's provenance as labels — so the artifact is browsable in
   GHCR as an image with a tag, not an orphan blob;
3. verifies the blob answers **anonymously** (the device holds no credential).

The device fetches a plain HTTPS URL served by the registry's blob endpoint:

```
https://ghcr.io/v2/<namespace>/<repo>/blobs/sha256:<digest>
```

### Consequences that are refusals, not warnings

- **Public visibility is mandatory.** GHCR serves blobs anonymously only for a public
  package. A private package needs a token the device cannot supply, so the builder
  refuses a non-public publish.
- **The `local` backend cannot back a real deploy.** It returns a `file://` URL the
  device cannot fetch; it is allowed only under `--dry-run`.

### Other backends

| Backend | When |
|:--|:--|
| `composite` (recommended) | object store for the device, GHCR for provenance — see below |
| `ghcr` | a registry namespace exists; `GITHUB_TOKEN` with `packages:write` |
| `object` | S3/R2 public prefix; set `VYZORIX_BUILD_BASE_URL` and `VYZORIX_BUILD_UPLOAD_CMD` |
| `local` | dev and dry runs only |

### Why the device path is an object store, not GHCR

A `ghcr.io` blob URL is **not** a device-fetchable URL. It answers an unauthenticated
GET with a **401 challenge even for a public package**, and its redirected CDN URL is
**signed and expires in minutes**. The device fetches with a plain GET and no
credential, so it cannot use a registry blob URL.

The `composite` backend splits the two concerns:

```jsonc
"publish": {
  "backend": "composite",
  "deviceBackend": "object",   // the URL the manifest pins; a plain object URL
  "recordBackend": "ghcr",     // provenance: an OCI image with a tag
  "namespace": "owner", "repo": "app", "visibility": "public"
}
```

Set `VYZORIX_BUILD_BASE_URL` and `VYZORIX_BUILD_UPLOAD_CMD` for the object store. The
result's `url` is the object store's (what the device fetches); the `tag` is the GHCR
image (what a human browses). A record-store failure is a warning, not a refusal.

A backend that cannot return an anonymous URL is refused for a non-`--dry-run` build,
because the device cannot authenticate.

---

## 6. Determinism

A digest must be a function of the source, not the builder. The builder pins base images
and toolchain versions, normalises the tar (sorted entries, fixed `mtime`, `uid/gid=0`),
and records the glibc floor it linked against. Building the same source twice produces
the same digest; `vyzorix-builder`'s integration suite asserts this.

---

## 7. Filenames

Every artifact is named explicitly and uniquely:

```
<app>-<artifact>-<os>-<arch>-v<version>.tar.gz
```

For example `vfd-engine-linux-android-arm64-v8a-v1.2.0.tar.gz`. An architecture-free
artifact (a webroot) uses `all` for the arch segment. Two artifacts that would collide
on a filename are a **refusal naming both**, not a last-writer-wins.

---

## 8. CI

```yaml
- run: vyzorix build --device $DEVICE --publish ghcr --namespace $NS --repo $REPO --version v1.2.0
- run: vyzorix deploy --device $DEVICE --file vyzorix.toml
```

Use a **deploy credential** (`DEPLOY_CREDENTIALS.md`) rather than an operator key. The
job needs a token with `packages:write` for the GHCR push.

---

## 9. Troubleshooting

| Symptom | Cause |
|:--|:--|
| `does not match the server's schema` | the recipe has a field the schema rejects; the message names the path |
| `path "..." escapes the project directory` | a recipe `path`/`file` is absolute or climbs out with `..` |
| `the local backend cannot back a real deploy` | `--publish local` without `--dry-run` |
| `two artifacts produce the same filename` | two artifacts resolve to one name; give them distinct `name` or `app` |
| `the binary needs GLIBC_x but the target floor is GLIBC_y` | the artifact links a newer glibc than the device rootfs; pin the toolchain or link static |
| a blob 404s **only on the device** | the GHCR package is private; make it public |
