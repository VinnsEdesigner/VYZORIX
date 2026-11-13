# ADR-003: A separate C++17 build tool, publishing to an OCI registry

**Status**: Accepted
**Date**: 2026-11-13
**Related**: `DEPLOY_BUILD.md` (the operator guide), `ADR-002` (the house style for
decisions on record).

## Context

The deploy pipeline was complete **after** an artifact exists: the manifest references
`{url, sha256, arch, kind}`, and something else has to produce those bytes. In practice
that "something else" was a per-project pile of scripts (`Vfd/deploy/build.sh` plus
`manifest.sh`) that re-derived the same non-obvious rules every time: cross-compile for
aarch64, match the device's glibc floor, resolve a shared-library closure, produce a
deterministic tarball, compute the digest, publish it at an anonymous URL, and render
the manifest from that digest.

That body of knowledge was easy to get subtly wrong, and the failures were invisible
until they reached the device: a binary that links a newer glibc fails **only on the
device**; a private release URL 404s **only on the device**. We needed one shared,
tested producer path.

Three questions had to be answered, and each is a decision on record.

## Decision 1 — The builder is a separate process, written in C++17

`vyzorix-builder` is its own repository, released separately, and the CLI invokes it as a
subprocess over a JSON contract.

**Why a separate process.** The CLI's release property is precious: CGO-free, one
runner, five platforms, one small static binary. Folding Docker plumbing and language
toolchains into it would bloat it and break that property. A process boundary also lets
the builder be swapped, containerised, or run on a build host without touching the
deploy CLI.

**Why C++17, not Go.** It matches the house style of the device-side tooling
(`android-bin`, the supervisor, Vfd's `media-ingest-c` are C/C++ with hand-rolled wire
code), and it builds in the same bookworm images the artifacts target — so the builder
itself can be built by the same class of image it drives. Dependencies are deliberately
few: `libcurl` for HTTP, a small vendored JSON reader/writer, and Docker driven as a
**subprocess** over its CLI (the build *is* the container's job, and the CLI is the
stable interface).

## Decision 2 — The publish backend is an OCI registry, GHCR by default

Publishing runs in two phases so an OCI image is the encapsulation but the device
contract stays plain HTTP:

1. **Build** in a pinned bookworm container, producing a deterministic
   `/out/<explicit-name>.tar.gz`.
2. **Push the bytes as an OCI blob** to GHCR, and a small OCI image manifest that
   references the blob as its single layer and carries the artifact's provenance as
   labels. The OCI digest for the blob **is** the `sha256` the manifest pins, so the
   signature covers exactly the bytes the registry serves.
3. **Verify the blob answers anonymously** before reporting success. The device fetches
   with no credential, so a private package is a **refusal, not a warning**.

**Why GHCR.** One namespace per project, immutable content-addressing (the blob digest
*is* the sha256), no bespoke hosting, and the image doubles as a build-provenance
record — while the device stays on the tarball-over-HTTPS contract it already has.

**Consequences, stated not discovered.**

- **Public visibility is mandatory.** GHCR serves blobs anonymously only for a public
  package; a private package requires a bearer token the device cannot supply. So either
  the package is public, or the deploy targets an object store instead.
- **The registry is not the only backend.** A `Store` port keeps object stores (S3/R2
  public prefix) and local output available; GHCR is the default, not the only option.
  On a self-hosted server with no registry, the object-store backend is the honest path.
- **The device is not touched.** The image is an encapsulation and store; the device
  still fetches a tarball over HTTPS and verifies the digest.

## Decision 3 — Artifacts carry explicit, unique filenames

A filename says what it is, for which platform, at which version:

```
<app>-<artifact>-<os>-<arch>-v<version>.tar.gz
```

`<os>` is the target OS ABI (`linux-android`); `<arch>` is the exact Android ABI
(`arm64-v8a` | `armeabi-v7a` | `x86_64`), or the literal `all` for an architecture-free
artifact. The name is **derived by the builder**, never typed by hand and never
templated by the manifest; two artifacts that would collide are a refusal naming both.
The OCI tag is the same string without the `.tar.gz`; the durable, content-addressed
reference is still the blob digest.

**Why.** A generic filename (`app.tar.gz`) makes a registry or a directory listing
unreadable and lets two artifacts silently overwrite one another.

## Decision 4 — The manifest is rendered by the CLI, not the builder

An earlier draft had the builder return a `manifestFragment`. That was wrong: the
builder's result carries **artifacts only** (url, sha256, file, tag, size), and the
**CLI** renders `vyzorix.toml`.

**Why.** The manifest's `[[app]]` block needs the recipe's app-level fields *and* the
CLI composes `[device]` (id, org) around them, which the builder never sees. More
importantly, the CLI is the side that validates the rendered manifest against the served
schema; a builder-side renderer would be a second implementation that could drift from
the manifest the server accepts.

## Consequences

- **Positive.** One shared, tested producer path replaces per-project scripts. The
  digest a manifest pins is the digest the registry serves, because they are the same
  bytes. The builder is separately releasable and testable without the CLI.
- **Negative.** A second toolchain to build and release (`vyzorix-builder`), and a
  process boundary that must be kept in sync with the CLI's `BuildRequest`/`BuildResult`
  JSON. Both are covered by the contract's tests: the CLI's delegate test uses a real
  stub builder on `PATH`, and the builder's integration suite asserts the plan contract
  for every adapter.
- **Neutral.** The device is unchanged; the sign-and-apply pipeline is byte-for-byte
  what it was.
