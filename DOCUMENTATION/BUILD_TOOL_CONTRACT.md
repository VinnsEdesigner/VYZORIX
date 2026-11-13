# The build tool — design contract

**Status:** proposed (rev 2). This is the contract for `vyzorix-builder`, the tool that
turns a project's source into the digest-pinned artifacts a `vyzorix.toml` references.
**Author:** platform lead (design review).
**Related:** `DEPLOY_PIPELINE.md` (what happens to the artifact after it exists),
`MANIFEST_REFERENCE.md` §6 (the artifact reference this tool produces),
`CLI_REFERENCE.md` (the command surface), `DEPLOY_SERVER_API.md` (the endpoints),
`CONTAINER_FILESYSTEM.md` and `APP_RUNTIME.md` (where the artifact lands).

**Rev 2 changes (decisions taken):** the publish backend is an **OCI registry (GHCR)**
(§4.5); the builder is written in **C++17, not Go** (§4.6); artifacts carry **explicit,
unique filenames** (§4.7); the builder's repository is **strictly layered** (§5.1).

---

## 1. The problem, stated plainly

Today the pipeline is complete *after* an artifact exists:

```
 operator writes vyzorix.toml ──► CLI validates ──► server compiles+signs ──► device applies
        ▲
        └── every artifact URL and sha256 must already exist, produced by hand
```

Stage 2 of `DEPLOY_PIPELINE.md` says it outright: *"Building the artifact is the
operator's job, done somewhere else."* That "somewhere else" is, in practice, a
per-project pile of scripts — `Vfd/deploy/build.sh` plus `manifest.sh` — that encodes
the same non-obvious rules every time: cross-compile for aarch64, match the device's
glibc floor, resolve a shared-library closure, produce a deterministic tarball, compute
the digest, publish it at an **anonymous** URL, and render the manifest from that
digest.

The gap is that this body of knowledge is re-derived per app and is easy to get subtly
wrong (a binary that links a newer glibc fails **only on the device**; a private release
URL 404s **only on the device**). The tool exists to make that one, shared, tested path.

### 1.1 Non-goal restated from the manifest

The manifest is **not** a build system (`MANIFEST_REFERENCE.md` §11). This tool does not
change that. The build recipe is a **separate file**, `vyzorix.build.toml`, and the
manifest still references only `{url, sha256, arch, kind}`. The two files have different
owners, different lifecycles and different review audiences, and must not be merged.

---

## 2. Goals and non-goals

**Goals**

- One command builds a project into one or more **deployable artifacts**: tarballs at a
  public HTTPS URL, pinned by sha256, correctly tagged by `arch` and `kind`.
- Language coverage that is *open*: Go, Rust, C/C++, Node/TS (server and webroot),
  Python, JVM, plus a raw Containerfile escape hatch so "any project" means any project.
- Bake in the target constraints so the operator never restates them: the **Debian
  bookworm glibc floor**, the **anonymous-fetch** publishing rule, the **per-ABI**
  artifact rule, the **no-shell `cmd`** rule.
- Deterministic builds, so a digest is a function of the source, not the builder.
- The CLI **delegates** to the tool, then deploys the result. The operator runs one
  surface.

**Non-goals**

- **No device change.** The apply path already verifies, resumes, digests and installs
  atomically (`ArtifactFetcher.kt`, `DeployApplier.kt`). This is a producer-side feature.
- **No build on the control plane.** The server stays a Go binary on Fly; builds need
  arbitrary toolchains. See §4.3.
- **No runtime OCI on the device.** The image is an *encapsulation and store*; the
  device still fetches a tarball over plain HTTPS (§4.5).
- **No change to what "compile" means server-side.** `vyzorix_deploy_compile.go`
  compiles a *manifest into a signed config*; this tool *builds artifacts*. The words
  collide; the concepts must not.

---

## 3. The shape of the solution

```
 project source ──► vyzorix-builder ──► published artifacts ──► vyzorix deploy
 (any language)     (C++, Docker)       (public URL + sha256)   (existing pipeline)
        │                   │                     │                    │
        │ vyzorix.build.toml│                     │                    │
        └───────────────────┘                     └────────────────────┘
   the one new human input            the existing manifest, rendered from digests
```

Three actors:

1. **`vyzorix-builder`** — a new C++17 binary. Builds, packages, publishes.
2. **`vyzorix` (the CLI)** — gains `build`, which delegates to the builder, then renders
   the manifest, then can `deploy`. The CLI stays thin (no Docker, no toolchains).
3. **The server** — one additive change: expose the target device's ABI, and serve the
   recipe schema. No new build logic.

---

## 4. Load-bearing decisions

### D1 — The builder is a separate process, not a CLI subcommand's body

`vyzorix-builder` is its own project/repository, released separately. The CLI invokes it
as a **subprocess** over a JSON contract (§6.2).

*Why:* the CLI's release property is precious — CGO-free, one runner, five platforms, one
small static binary (`vyzorix-cli/README.md`, `release.yml`). Folding Docker plumbing and
language toolchains into it would bloat it and break that property. A process boundary
also means the builder can be swapped, containerised, or run on a build host without
touching the deploy CLI.

### D2 — The build recipe is a separate file, `vyzorix.build.toml`

Validated against a **published schema**, fetched from the server exactly like the
manifest schema (`GET /v1/deploy/schema/{version}` is the precedent; this is
`GET /v1/deploy/build-schema/{version}`). The builder parses TOML locally, fetches the
schema, validates, and refuses with a field path — the same local-first, server-
authoritative discipline as `MANIFEST_REFERENCE.md`.

### D3 — Language-agnostic core, first-class adapters

"Any project" still requires a recipe: no tool can infer that a repo is a C program
linking GStreamer versus a Vite SPA. So the recipe names, per artifact, an **adapter**.
Two mechanisms:

- **First-class adapters** for the common cases. Delivered: `golang`, `rust`,
  `cpp`, `node`, `node-webroot`, `deno`, `bun`, `zig`, `dotnet`, `ruby`, `php`,
  `python`, `jvm`. Each is a small plan plus a bundled, pinned Dockerfile; the
  builder writes that Dockerfile into the build context and builds it with the
  project, so the binary needs no `builders/` directory at run time.
- **A raw Containerfile adapter** — the escape hatch. The recipe points at a
  `Containerfile` that emits `/out/*.tar.gz`; the builder builds it and collects the
  result. This makes coverage genuinely open, and it is what
  `Vfd/deploy/bundle/Dockerfile.arm64` already is.

### D4 — Output is a tarball + sha256 at an anonymous URL

The manifest contract is literal: `url` (https), mandatory `sha256`, optional `arch`
and `kind` (`MANIFEST_REFERENCE.md` §6). The device fetches **anonymously** and resumes
with `Range`. Therefore the artifact is an archive, one per `(name, arch)`, and the URL
is never templated (`MANIFEST_REFERENCE.md` §6.2 — `${abi}` cannot work).

### D5 — The publish backend is an OCI registry, GHCR by default

*(Decision: use GHCR as the store.)* Publishing runs in two phases so an OCI image is
the encapsulation but the device contract stays HTTP:

1. **Build** the artifact(s) in a pinned bookworm container, producing
   `/out/<explicit-name>.tar.gz` (deterministic; §4.7).
2. **Ship** with `docker buildx build --platform linux/<arch> --push` to
   `ghcr.io/<namespace>/<repo>`. The image carries the tarball and a small metadata label
   set (source revision, recipe digest, glibc floor, artifact name/arch/kind).
3. **Extract** the exact tarball **deterministically** for the device path. Note the
   asymmetry: `docker buildx --output type=oci` is an *export format* (a tar of tars,
   with a manifest), not necessarily a plain tar of the deployable bytes; and Vfd found
   buildx's local exporter can stall. So the extraction is an ordinary
   `docker create` + `docker cp <container>:/out/<name>.tar.gz` / `docker save | extract`
   step — a normal docker operation, not an exotic exporter.
4. **Push the bytes as an OCI blob** to the same registry, so the device fetch address is
   a plain HTTPS URL served by the registry's blob endpoint:
   `https://ghcr.io/v2/<namespace>/<repo>/blobs/<sha256:…>`. The OCI digest for that blob
   **is** the `sha256` the manifest pins, so the signature covers exactly the bytes GHCR
   serves.

**Consequences that must be stated, not discovered:**

- **Public visibility is mandatory.** GHCR serves blobs anonymously **only for a public
  package**. A private package requires a bearer token, which the device cannot supply —
  it fetches with no credential. So either the GHCR package is public, or the deploy
  targets an object store (S3/R2 public prefix) instead. The builder MUST verify the blob
  is reachable **unauthenticated** before it reports success; a private package is a
  refusal, not a warning.
- **Content type.** GHCR's blob endpoint serves blobs with a generic content type; this
  is irrelevant to the device, which treats the body as opaque bytes and verifies the
  digest. It would matter only to a browser, which never fetches this.
- **Cloudflare-fronted ghcr.io.** `ghcr.io` is fronted by Cloudflare/Artifactory in
  places; the builder must send `Accept: application/vnd.oci.image.layer.v1.tar+gzip`
  (or `*/*`) and follow redirects, and must not assume the blob and the image manifest
  live on the same host.
- **Registry is not the only backend.** A `Store` interface keeps object stores and local
  output available (§6.4); GHCR is the default, not the only option. On a self-hosted
  server with no registry, the object-store backend is the honest path.

*Why GHCR:* it gives one namespace per project, immutable content-addressing (the blob
digest is the sha256), no bespoke hosting, and reuses the image as a build provenance
record — while the device stays on the tarball-over-HTTPS contract it already has.

### D6 — The target ABI is resolved from the server, not guessed

The device reports `Build.SUPPORTED_ABIS` at enroll (`DeviceFacts.kt`, stored on
`DeviceEntity.SupportedABIs`). The deploy identity endpoint currently returns only
`{id, name, tier}` (`vyzorix_identity_handler.go`). **Additive change:** return the
device's ABIs so `vyzorix build --device D` builds for exactly what the device runs.
`--arch` remains as an explicit override.

### D7 — Determinism is a requirement, not a nicety

Because the deploy is digest-pinned, a non-reproducible build produces digests that
drift, which produces `digest_mismatch` on the device. The builder MUST: pin base images
and toolchain versions; normalise the tar (sorted entries, fixed `mtime`, `uid/gid=0`,
`uname/gname` empty); and record the glibc floor it linked against.

### D8 — The glibc floor is asserted at build time, not discovered on the device

The device rootfs is Debian **bookworm**, glibc **2.36** (`build-rootfs.sh`). Anything
dynamically linked must be built against it or made static. The builder asserts the
**highest GLIBC symbol the binary needs** ≤ 2.36 and fails the build otherwise — the
failure Vfd found "appears nowhere but on the device."

### D9 — The CLI keeps `build` and `deploy` separate, with a convenience join

`vyzorix build` is expensive and publishes; `vyzorix deploy` is the signed config change.
They stay separate verbs. `vyzorix deploy --build` runs build-then-deploy. Like
`--dry-run`, expensive/irreversible acts are explicit.

### D10 — The builder is C++17, CMake, dependency-light

*(Decision: C++, not Go.)* This matches the house style of the device-side tooling
(`android-bin`, the supervisor, Vfd's `media-ingest-c` are C/C++ with hand-rolled wire
code) and keeps the builder buildable in the same bookworm images the artifacts target.

- **Build system:** CMake ≥ 3.25, C++17. A `viability` static library plus a thin
  `vyzorix-builder` binary.
- **Dependencies, deliberately few:** `libcurl` (HTTP: schema fetch, registry push, URL
  verification), `libarchive` (deterministic tar writing and reading), `fmt` (formatting)
  or `std::format` where the toolchain provides it. **Docker is driven as a subprocess**
  over the CLI, never libdocket — because the build *is* the container's job and the CLI
  is the stable interface.
- **JSON:** a small **vendored** reader/writer (nlohmann/json, vendored and pinned, or a
  hand-rolled pair mirroring the bridge's approach) rather than a system package that
  must exist in every build rootfs. The process-boundary contract is JSON (§6.2), so this
  is the one non-trivial dependency.
- **Testing:** a header + `ctest` unit suite (no framework dependency; the repo's existing
  C++ suites use a tiny custom harness).

### D11 — Artifact filenames are explicit and unique

*(Decision: no generic filenames.)* A filename must say what it is, for which platform,
at which version, so no two artifacts can collide and a human reading a registry or a
directory listing knows exactly what they have.

Pattern:

```
<app>-<artifact>-<os>-<arch>-v<version>.tar.gz
```

- `<app>` and `<artifact>` from the recipe (kebab-case, matching the manifest names).
- `<os>` is the target OS ABI: `linux-android` (the Debian-under-proot target; the single
  supported target in v1). It is explicit because "linux" alone is ambiguous next to a
  possible future host target.
- `<arch>` is the **Android ABI**, exact: `arm64-v8a` | `armeabi-v7a` | `x86_64`. For an
  architecture-free artifact (`kind = webroot`, or a static site) the segment is the
  literal `all`.
- `<version>` is the build version (`--version` or the recipe's `version`).

Examples:

```
vfd-engine-linux-android-arm64-v8a-v1.2.0.tar.gz
vfd-extractors-linux-android-arm64-v8a-v1.2.0.tar.gz
vfd-console-linux-android-all-v1.2.0.tar.gz
```

Rules:

- The name is **derived by the builder**, never typed by hand and never
  templated by the manifest. It is the artifact's identity in both the recipe output and
  the registry tag.
- Filenames must be **unique within a build**; a collision is a refusal naming both.
- The corresponding **OCI tag** (for humans browsing GHCR) is the same string without the
  `.tar.gz`; the durable, content-addressed reference is still the blob digest, which is
  what the manifest pins.

---

## 5. Repository structure

### 5.1 `vyzorix-builder` — new repository (C++17, strictly layered)

Dependencies point **downward only**. Nothing in `okfw/` or `layers/` calls `deploy/`; the
domain talks to abstractions via interfaces defined in `okfw/`, and the composition root
(`cmd/`) is the only place concrete adapters are wired to interfaces.

```
vyzorix-builder/
├── README.md
├── LICENSE
├── CMakeLists.txt                 # top level: options, toolchain, install
├── cmake/
│   └── VendoredDeps.cmake          # pinned third-party fetch/vendor (curl, archive, json)
├── okfw/                           # cross-cutting utilities (no domain knowledge)
│   ├── CMakeLists.txt
│   ├── include/okfw/
│   │   ├── result.hpp              # Result<T,E> — no exceptions across boundaries
│   │   ├── log.hpp                 # levelled logging
│   │   ├── subprocess.hpp          # run a program, capture out/err, exit code
│   │   └── jsonl.hpp               # thin over the vendored JSON reader/writer
│   └── src/{log,subprocess,jsonl}.cpp
├── domain/                         # vocabulary + policies, pure, no I/O
│   ├── CMakeLists.txt
│   ├── include/domain/
│   │   ├── recipe.hpp              # the parsed vyzorix.build.toml model
│   │   ├── target.hpp              # ABI -> {os, debarch, suite, libc_floor}
│   │   ├── artifact.hpp            # name, arch, kind, sha256, size, url
│   │   ├── naming.hpp              # the D11 filename/tag policy
│   │   └── errors.hpp              # refusals with a field path
│   └── src/{recipe,target,naming,errors}.cpp
├── layers/                         # the hexagon: each layer = interfaces + impls
│   ├── CMakeLists.txt
│   ├── schema/                     # the recipe schema port + fetch/validate impl
│   │   ├── include/layers/schema/{port.hpp,client.hpp,validator.hpp}
│   │   └── src/{client,validator}.cpp
│   ├── plan/                       # adapter port + registry
│   │   ├── include/layers/plan/{port.hpp,plan.hpp,registry.hpp}
│   │   └── src/registry.cpp
│   ├── adapter/                    # one file per language (the only place that grows)
│   │   ├── include/layers/adapter/{adapter.hpp,catalog.hpp}
│   │   └── src/{golang,rust,cpp,node_webroot,python,jvm,containerfile}.cpp
│   ├── build/                      # the docker-subprocess build engine
│   │   ├── include/layers/build/{engine.hpp,docker.hpp,image.hpp}
│   │   └── src/{engine,docker,image}.cpp
│   ├── package/                    # deterministic tar, digest, closure, glibc assert
│   │   ├── include/layers/package/{tarball.hpp,digest.hpp,closure.hpp,glibc.hpp}
│   │   └── src/{tarball,digest,closure,glibc}.cpp
│   └── publish/                    # Store port + backends (GHCR, object store, local)
│       ├── include/layers/publish/{port.hpp,store.hpp,ghcr.hpp,objectstore.hpp,local.hpp}
│       └── src/{ghcr,objectstore,local}.cpp
├── cmd/
│   └── vyzorix-builder/
│       ├── CMakeLists.txt
│       ├── main.cpp                # composition root: wire impls to interfaces
│       ├── cli.hpp                 # subcommands build|publish|plan
│       └── cli.cpp
├── builders/                       # pinned builder images per adapter
│   ├── bookworm.Dockerfile         # the shared base (matches the device glibc floor)
│   ├── go.Dockerfile
│   ├── rust.Dockerfile
│   ├── cpp.Dockerfile
│   ├── node.Dockerfile
│   ├── python.Dockerfile
│   └── jvm.Dockerfile
├── tests/
│   ├── unit/                       # ctest cases per layer
│   └── integration/               # real docker builds of tiny fixtures (opt-in)
└── .github/workflows/{ci.yml,release.yml}
```

**Layering rule, stated for the reviewer:** `okfw` ← `domain` ← `layers/*` ← `cmd`. A
`layers/x` module may depend on `domain` and `okfw` and on the **ports** of other layers,
but never on their concrete implementation headers. Only `cmd/` sees the concrete types.
This is what lets an adapter be tested without Docker and a Store be swapped without the
engine knowing.

### 5.2 `vyzorix-cli` — existing repository

**0 new packages, ~4 new files, ~4 edits.**

| Path | New/edit | Purpose |
|:--|:--|:--|
| `internal/deploy/vyzorix_build_command.go` | new | `build`: load recipe, resolve ABI, delegate, render manifest |
| `internal/deploy/vyzorix_build_delegate.go` | new | the process boundary: locate `vyzorix-builder`, JSON in/out |
| `internal/deploy/vyzorix_manifest_render.go` | new | render `vyzorix.toml` from build results + device/org |
| `internal/deploy/*_test.go` (2) | new | tests over the real package |
| `cmd/vyzorix/main.go` | edit | register `build`; add `--build` to `deploy` |
| `internal/deploy/vyzorix_identity_command.go` | edit | show `supportedAbis` in `whoami` |
| `README.md` | edit | document `vyzorix build` |

### 5.3 `vyzorix-spec` — shared module

**0 new packages, ~4 new files, one version bump** (`v0.1.0` → `v0.2.0`).

| Path | New/edit | Purpose |
|:--|:--|:--|
| `tooling/schema/build.cue` | new | the recipe, authored once (mirrors `deploy.cue`) |
| `tooling/codegen/build_build_types.py` | new | thin wrapper: `SCHEMA_DIR=schema SPEC_CUE=build.cue GO_OUT=build.go` |
| `tooling/codegen/build_build_schema.py` | new | thin wrapper: emits `schema/build.schema.json` |
| `build.go` | new (generated) | `BuildRecipe`, `BuildArtifact`, `AdapterKind` |
| `schema/build.schema.json` | new (generated) | the JSON Schema |
| `build_schema.go` | new | `BuildSchemaVersion`, `BuildSchema(version)` |
| `build_schema_test.go` | new | parse + unknown-version + type-decode tests |
| `README.md`, `.github/workflows/ci.yml` | edit | document `build.*`; extend the drift gate |

### 5.4 `VYZORIX` (server) — existing repository

**0 new Go packages, ~3 new files, ~5 edits, plus a swagger regen.**

| Path | New/edit | Purpose |
|:--|:--|:--|
| `apps/api/internal/api/handlers/deploy/vyzorix_identity_handler.go` | edit | add `SupportedABIs []string` to `deviceSummary` (mind `fieldalignment`) |
| `...identity_handler_test.go` | edit | assert the ABIs are present |
| `apps/api/internal/deploy/vyzorix_build_schema.go` | new | adapter over `vyzorixspec.BuildSchema` (mirrors `vyzorix_deploy_schema.go`) |
| `apps/api/internal/api/handlers/deploy/vyzorix_build_schema_handler.go` | new | `GET /v1/deploy/build-schema/{version}`, public |
| `...build_schema_handler_test.go` | new | compiles the served schema |
| `apps/api/internal/api/server_routes.go` | edit | register the route beside `setupDeploySchemaRoute` |
| `swag/*` | edit (regen) | the drift gate requires it |

The recipe schema source stays in `vyzorix-spec` (§5.3), the single source of truth; the
server embeds the generated JSON the same way it embeds the manifest schema.

### 5.5 `Vyzorix-android` (device) — existing repository

**Zero changes.** Explicitly. The device is already correct; this feature is producer-
side only.

### 5.6 Documentation — `VYZORIX/DOCUMENTATION`

| Path | New/edit | Purpose |
|:--|:--|:--|
| `BUILD_TOOL_CONTRACT.md` | new | this document |
| `DEPLOY_BUILD.md` | new | operator guide: recipe fields, adapters, GHCR publishing, example |
| `ADR_003_BUILD_TOOL.md` | new | decision record: C++ builder, OCI/GHCR store, tarball contract |
| `DEPLOY_PIPELINE.md`, `MANIFEST_REFERENCE.md`, `CLI_REFERENCE.md`, `VYZORIX_DOCS_INDEX.md` | edit | cross-references |

### 5.7 Totals

| Repository | New packages/modules | New files | Edits |
|:--|--:|--:|--:|
| `vyzorix-builder` (new repo) | 10 (C++ libs) | ~45 | 0 |
| `vyzorix-cli` | 0 | ~4 | ~4 |
| `vyzorix-spec` | 0 | ~6 | 2 |
| `VYZORIX` (server) | 0 | ~3 | ~7 (incl. swagger regen) |
| `Vyzorix-android` | 0 | 0 | 0 |
| docs (in `VYZORIX`) | — | 3 | 4 |
| **Total** | **10** | **~61** | **~17** |

---

## 6. The contracts

### 6.1 The adapter port (inside the builder)

```cpp
namespace vyzorix::plan {

struct Output {
    std::string name;   // artifact name, manifest-compatible
    std::string path;   // "/out/<explicit>.tar.gz" inside the image
    std::string arch;   // "arm64-v8a" | "armeabi-v7a" | "x86_64" | "all"
    std::string kind;   // "dir" | "webroot"
};

struct BuildPlan {
    std::string image;             // pinned builder image
    std::vector<std::string> args; // computed cross-toolchain args
    std::optional<std::string> containerfile; // the escape hatch
    std::vector<Output> outputs;
};

class Adapter {
public:
    virtual ~Adapter() = default;
    virtual std::string name() const = 0;
    virtual okfw::Result<BuildPlan, domain::Error>
        plan(const domain::RecipeArtifact&, const domain::Target&) const = 0;
};

}  // namespace vyzorix::plan
```

Adding a language = adding one `.cpp` that returns a `BuildPlan`. The core does not grow
branches.

### 6.2 The CLI ↔ builder process contract

The CLI writes a `BuildRequest` (JSON) to the builder's stdin and reads a `BuildResult`
(JSON) from stdout. The builder is found via `$VYZORIX_BUILDER`, else on `PATH`.

```jsonc
// BuildRequest → stdin
{
  "recipe":     "<vyzorix.build.toml bytes>",
  "schemaUrl":  "https://server/v1/deploy/build-schema/v1",
  "projectDir": "/abs/path/to/project",
  "abis":       ["arm64-v8a"],          // resolved from --device via the server
  "deviceId":   "d_01H...",
  "version":    "v1.2.0",
  "publish":    { "backend": "ghcr", "namespace": "owner", "repo": "vfd", "visibility": "public" },
  "dryRun":     false
}

// BuildResult → stdout
{
  "artifacts": [
    { "name": "engine", "arch": "arm64-v8a", "kind": "dir",
      "url": "https://ghcr.io/v2/owner/vfd/blobs/sha256:9f2c…",
      "sha256": "9f2c…", "size": 1882104,
      "file": "vfd-engine-linux-android-arm64-v8a-v1.2.0.tar.gz",
      "tag":  "owner/vfd:vfd-engine-linux-android-arm64-v8a-v1.2.0" }
  ]
}
```

The builder's result carries **artifacts only** (url, sha256, file, tag, size). It does
**not** render the manifest: the manifest's `[[app]]` block needs the recipe's app-level
fields *and* the CLI composes `[device]` (id, org) around them, which the builder never
sees. So the **CLI** renders `vyzorix.toml` from the recipe + the result (§5.2, §9), and
the single renderer is the one validated against the served manifest schema. A builder-side
renderer would be a second implementation that could drift from the manifest the server
accepts.

Exit codes mirror the CLI: `0` success, `1` refused (a builder refusal, printed verbatim),
`2` usage/local failure.

### 6.3 The recipe (`vyzorix.build.toml`), by example

```toml
[build]
version = "v1.2.0"
abi     = ["arm64-v8a"]      # omitted -> resolved from --device via the server
suite   = "bookworm"         # the glibc floor the artifact must run against

[[artifact]]
name = "engine"
lang = "cpp"
path = "media-ingest-c"
build = "make"
cmd  = "/home/apps/vfd/engine/run.sh"   # becomes the manifest's `cmd`
kind = "dir"
  [artifact.cpp]
  libs        = ["libcurl", "libsqlite3", "libgstreamer-1.0"]  # closure to bundle
  glibc_floor = "2.36"        # asserted
  static      = ["ffmpeg"]    # prefer static when the closure is pathological

[[artifact]]
name  = "console"
lang  = "node-webroot"
path  = "web"
build = "npm ci && npm run build"
dist  = "dist"
kind  = "webroot"             # architecture-free: filename arch segment is "all"

[[artifact]]
name = "weird"                # the escape hatch
lang = "containerfile"
file = "deploy/Containerfile.arm64"
out  = "/out/weird.tar.gz"
arch = "arm64-v8a"
```

### 6.4 The publish `Store` port

```cpp
namespace vyzorix::publish {

struct Published {
    std::string url;     // anonymous HTTPS
    std::string sha256;  // the content digest
    std::string file;    // the explicit filename (§4.7)
    std::string tag;     // the human-facing OCI tag, when the backend is a registry
};

class Store {
public:
    virtual ~Store() = default;
    // Put must verify the returned url is reachable without credentials, or refuse.
    virtual okfw::Result<Published, domain::Error>
        put(const std::string& key, const std::filesystem::path& file) = 0;
};

}  // namespace vyzorix::publish
```

Backends: `GhcrStore` (default; OCI blob push + anonymous verification), `ObjectStore`
(S3/R2 public prefix), `Local` (dev only — refused for a real deploy). A backend that
returns a non-anonymous URL is refused for a non-`dryRun` build, because the device
cannot authenticate.

### 6.5 The server changes (additive)

```jsonc
// GET /v1/deploy/whoami?device=d_01H...   (before → after)
{
  "device": { "id": "d_01H...", "name": "field-ops-07", "tier": "2",
              "supportedAbis": ["arm64-v8a", "armeabi-v7a"] },   // ← added
  "orgId": "org_01H...", "orgName": "...", "role": "operator", "...": "..."
}
```

```text
GET /v1/deploy/build-schema/{version}   → the recipe JSON Schema (public)
```

---

## 7. Wiring, end to end

```
 1. operator authors        vyzorix.build.toml  +  (optionally) vyzorix.toml
 2. vyzorix build           reads recipe; GET whoami?device=D → abis
                            validates recipe against GET /v1/deploy/build-schema/v1
                            writes BuildRequest → exec vyzorix-builder
 3. vyzorix-builder         per artifact: pick adapter → pinned bookworm image →
                            cross-compile for the ABI → resolve closure (readelf) →
                            assert glibc ≤ 2.36 → deterministic tar → explicit filename
 4. publish (GHCR)          docker buildx --platform linux/<arch> --push → extract exact
                            tarball → push as OCI blob → verify anonymous blob fetch
                            → url = https://ghcr.io/v2/<ns>/<repo>/blobs/<sha256:…>
 5. vyzorix-builder         emits BuildResult (artifacts: url, sha256, file, tag, size)
 6. vyzorix build           renders vyzorix.toml from the recipe + the result + device/org
 7. vyzorix deploy          the EXISTING path: validate → POST /v1/deploy →
                            compile → sign → device applies
```

Nothing in steps 1–6 touches the device or changes the signing model. Step 7 is
byte-for-byte the pipeline that exists today.

**CI wiring.** A repository's workflow:
`vyzorix build --device $DEVICE --publish ghcr --namespace $NS --repo $REPO --version v1.2.0`
then `vyzorix deploy --device $DEVICE --file vyzorix.toml` using a **deploy credential**
(`DEPLOY_CREDENTIALS.md`) rather than an operator key. The CI job needs `GITHUB_TOKEN`
with `packages:write` for the GHCR push.

---

## 8. Determinism, security, size

**Determinism.** Pinned images, pinned toolchains, normalised tar, recorded libc floor. A
digest must be a function of source; tested by building twice and asserting equal digests.

**Security — the builder runs untrusted code.** A project's build is arbitrary code
(postinstall scripts, Makefiles, Containerfiles). Therefore: builds run **inside the
pinned container**, never on the host; registry credentials come from the environment and
are never written into the artifact or the manifest; the recipe cannot name a host path
outside `projectDir`; and the anonymous-URL rule is a security property — it keeps the
artifact fetchable by a device that holds no secret.

**Size.** A phone's disk is scarce. Go/Rust/C/C++/GraalVM-native (one file) are preferred
over runtime-bundling (JRE, ruby, node). `native-image` is preferred over a bundled JRE.
The builder reports artifact size and MAY enforce a per-org ceiling later.

---

## 9. Test strategy

- **`vyzorix-builder`** — `ctest` unit suites per layer: recipe parse/validate; adapter
  `plan()` returns the right image/args per language (no Docker); `package` over real
  bytes (determinism, digest, glibc parse); `publish` against a fake store. One
  **integration** test per adapter builds a tiny fixture in Docker and asserts `/out`
  exists (skipped when Docker is absent).
- **`vyzorix-cli`** — the delegate boundary tested with a stub `vyzorix-builder` on
  `PATH`; the renderer tested by rendering known results and validating against the served
  schema, exactly as `vyzorix_deploy_schema_test.go` does.
- **`vyzorix-spec`** — parse + unknown-version + decode tests (parallel to
  `schema_test.go`); the CI drift gate extended to `build.*`.
- **server** — the identity test asserts the ABIs are present; the schema test compiles
  the served recipe schema.
- **End to end** — the Vfd case becomes the acceptance test.

No mocks for the real code paths: Docker is real in integration tests, the schema is the
served one, the device path is the shipped one.

---

## 10. Rollout

| Phase | Deliverable | Exit criteria |
|:--|:--|:--|
| **0 — spec** | `vyzorix-spec` v0.2.0 (`build.cue`, generated files, `BuildSchema`) | CLI and builder validate the same recipe |
| **1 — builder core** | `okfw`/`domain`, `plan`, `package`, `publish` (GHCR + local), `containerfile` + `golang` adapters | a Go fixture builds, publishes to GHCR, digests verify, blob fetches unauthenticated |
| **2 — CLI wiring** | `vyzorix build`, delegate, manifest renderer, ABI from `whoami` | `vyzorix build && vyzorix deploy` lands on a device |
| **3 — adapters** | `cpp`, `rust`, `node-webroot`, `python`, `jvm` | Vfd's engine + console build unmodified |
| **4 — parity & docs** | port `Vfd/deploy/*` to `vyzorix.build.toml`; `DEPLOY_BUILD.md` | Vfd deletes its hand-rolled scripts |

Phases 0–2 are load-bearing; 3–4 expand coverage without changing the contract.

---

## 11. Decisions on record

1. **Publish backend** — **OCI registry, GHCR default**, blob URL for the device; object
   store is the fallback where no registry exists. *(§4.5)*
2. **Builder language** — **C++17**, CMake, dependency-light, Docker via subprocess.
   *(§4.6)*
3. **Filenames** — **explicit and unique**: `<app>-<artifact>-<os>-<arch>-v<version>.tar.gz`.
   *(§4.7)*
4. **Repository structure** — **strictly layered**, dependencies point downward. *(§5.1)*

**Still open (need a call before phase 1):**

- Builder distribution: a released binary (requires Docker on the machine) **or** a
  container image the CLI `docker run`s? *Recommend both.*
- Size ceiling: enforce a per-org artifact limit, or report only? *Recommend report in
  v1.*
- GraalVM `native-image` versus a bundled JRE as the JVM default? *Recommend native-image,
  JRE opt-in.*
- Default resource caps (R4) and the build timeout (R2): the exact numbers are a
  deployment choice; the *requirement* is that both exist and are configurable.
  The builder now defaults the timeout to **3600s** (0 = default, negative = no
  bound) and takes caps from the request's `limits` block.

---

## 12. Operational requirements

The sections above say *what* the tool produces. This section says what must hold for it
to be **safe and usable in production** — the requirements a build tool needs that are not
implied by the artifact contract alone. Each is a numbered requirement (`R1`…), so the code
and the tests can cite it. Where the implementation is not yet at the requirement, the
requirement says so; that is the work list.

### 12.1 Process and liveness

- **R1 — No subprocess may hang the builder.** Draining a child's stdout and stderr must
  be concurrent (`poll()` on both), never sequential: a child that fills one pipe's buffer
  while not reading the other blocks on write, and a sequential parent blocks on read —
  a deadlock. `docker build` writes progress to **stderr**, so this is the common path.
- **R2 — Every external command has a timeout.** `docker build`, `curl`, `tar`, `readelf`
  and every other subprocess run under a wall-clock bound; on expiry the child is
  terminated (SIGTERM, then SIGKILL) and the build is refused, not left hanging. The
  default bound is generous (a real build may take minutes) and overridable.
- **R3 — The builder is cancellable.** SIGINT/SIGTERM to the builder terminates the build,
  removes the container/image it created, and exits non-zero. A cancelled build leaves no
  running container and no half-written artifact.

### 12.2 Isolation and resources

- **R4 — Builds are resource-capped.** The build container runs with a memory ceiling
  (`--memory`), a CPU ceiling (`--cpus`), and a PID ceiling (`--pids-limit`), so a
  runaway or hostile build cannot exhaust the host. The caps are configurable and have
  sane defaults.
- **R5 — Builds are disk-bounded.** A build cannot fill the host disk: a size ceiling on
  the build's output/scratch space is enforced, and exceeding it is a refusal naming the
  limit. A build's scratch and output are removed on both success and failure.
- **R6 — Concurrent builds do not collide.** Every build uses a unique working directory,
  image tag, and container name (per-process, not a fixed path/name), so two builds on one
  host — including two CI jobs — cannot clobber each other's image, container, or output.
- **R7 — The project is not mutated.** The builder may write only inside its own scratch
  space and inside the build container. In particular it must **not** overwrite a file the
  project already has (a project's own `.dockerignore` must be preserved, and any file the
  builder writes into the context must be restored or removed afterwards).

### 12.3 Determinism and provenance

- **R8 — Base images are content-pinned.** Each `FROM` resolves to a digest, not a
  floating tag (`rust:1-bookworm` is not a pin; `@sha256:…` is), so the same source builds
  the same bytes on two machines. A floating tag is a documented exception with a reason,
  not the default.
- **R9 — The target platform is explicit.** The build passes `--platform linux/<arch>` for
  the target ABI, so a multi-arch host builds the right architecture — including for the
  Containerfile escape hatch, which cannot otherwise request it.
- **R10 — Base images may be refreshed deliberately.** A `--pull` option (off by default,
  so builds are reproducible from cache) lets an operator pick up a re-published base
  image on purpose.
- **R11 — `suite` selects the glibc floor.** The recipe's `suite` maps to the libc floor
  the artifact is asserted against; changing `suite` changes the assertion. The floor is
  never a hardcoded constant that `suite` silently ignores.
- **R12 — The builder reports its own version, and is itself reproducible.** The binary
  reports the released version, and a given builder version plus a given source produce a
  given digest. The build records the builder version in the artifact's provenance.

### 12.4 Network, registry, and publishing

- **R13 — Published bytes are verified, not just the URL.** After publishing, the builder
  fetches the artifact and hashes what it receives, comparing to the pinned sha256. A 200
  with the wrong bytes is a refusal, because the device would fail `digest_mismatch`
  later. The fetch performs the OCI auth handshake when the store challenges with a 401,
  so the check mirrors what a real client must do. **Verified live:** a `ghcr.io` blob URL
  answers an unauthenticated GET with a **401 challenge even for a public package**, and
  its redirected CDN URL is **signed and expires in minutes** — so a registry blob URL is
  *not* a device-fetchable URL. The device's fetch path must therefore be an object store
  (§12.4 composite), not a registry blob endpoint.
- **R14 — Transient failures are retried.** Registry and network operations retry with
  backoff on transient errors (connection reset, 5xx, 429), and give up with a clear
  refusal after a bounded number of attempts. Retries are not applied to a definitive
  refusal (401/403/404).
- **R15 — Registry credentials are refreshed.** A bearer token that expires mid-push is
  re-minted and the request retried, rather than failing the build on a token that was
  valid when the build started.
- **R16 — Publish is idempotent.** Re-publishing the same bytes is a no-op (the blob
  already exists), not an error; a content-addressed store makes a re-run cheap and safe.
- **R23 — The device path and the provenance path may differ.** A registry is a good
  build-provenance record and a bad device fetch address (R13). The `composite` backend
  publishes the bytes to a **device store** (an object store: a plain, unsigned,
  credentialless URL) and records the build in a **record store** (GHCR: an OCI image with
  a tag). The manifest's `url` is the device store's; the `tag` is the record store's. A
  record-store failure is a warning, not a refusal — the device path already works.
- **R17 — Registry auth uses a per-run credential that is never persisted or logged.** A
  credential is passed to `curl` via a 0600 config file (never argv), is scoped to the one
  repo, and is removed when the run ends. See also R19.

### 12.5 Secrets and hygiene

- **R18 — Build-time secrets are supported and never baked in.** A project with private
  dependencies can authenticate during the build (a private Go module, an npm/pip/Composer
  registry) without the credential landing in a layer or in the artifact. Build secrets are
  passed as BuildKit secrets (`--secret`), never as build args, which are visible in image
  history.
- **R19 — Logs are redacted.** A credential, token, or signed URL never appears in the
  builder's output. Output that may contain one is redacted before it is logged, and the
  recipe cannot name a host path or environment variable outside the project.

### 12.6 Correctness of the artifact

- **R20 — The entrypoint exists.** The recipe's `cmd` is asserted to exist (and be
  executable) in the staged artifact before it is published, so a broken entrypoint is
  caught at build time rather than discovered on the device — the failure class this tool
  exists to prevent.
- **R21 — The recipe's `abi` agrees with what the server serves.** When the recipe sets
  `abi` and a device is named, the builder (or the CLI that drives it) checks the recipe's
  ABIs against the device's reported ABIs and refuses a mismatch, rather than building for
  an architecture the device does not run.
- **R22 — Staged-but-not-started is not "done".** An artifact is published only after the
  engine has packed it, asserted its floor (R11), and asserted its entrypoint (R20); the
  test strategy (§9) asserts each of these, not merely that `/out` exists.

### 12.7 Test obligations for the above

The requirements above are testable and must be tested (§9):

- R1–R3: a subprocess unit suite that fills each pipe and asserts no hang, and a timeout
  that kills a wedged child.
- R4–R6: an integration test that runs two builds concurrently and asserts distinct
  output, and a cap that is enforced.
- R8–R12: a plan-level assertion that every image is digest-pinned (or carries a
  documented exception) and that `--platform` is present; a `suite`→floor mapping test.
- R13: a store test against a fake registry that serves wrong bytes and asserts a refusal.
- R20: a package test that asserts a missing `cmd` is refused.

---

## 13. The one-line summary

The build tool is a **separate, C++17, Docker-driven producer** that turns a project plus
a `vyzorix.build.toml` into digest-pinned tarballs published to an **OCI registry (GHCR)**
and served to the device as anonymous blob URLs; the deploy CLI **delegates** to it,
**renders** the manifest from the result, and then runs the **unchanged** sign-and-apply
pipeline. The device is not touched.
