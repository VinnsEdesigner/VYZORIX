# Distributing the APK, and OTA updates

**Status:** shipped. Server routes in the updater handlers; device side is
`cloud-connector/.../update/`.
**Related:** the device-side `Vyzorix-android/doc/OTA_UPDATE.md`,
`DEVICE_ONBOARDING.md` (installing the agent the first time),
`CONTAINER_FILESYSTEM.md` (the rootfs assets, which travel a similar path).

This document covers the two halves of getting the app onto devices: the **first
install** (sideload) and every **update after that** (OTA). It also records the one
distribution trap that has bitten this project twice, so it does not bite a third time.

---

## 1. Why there is no Play Store

Vyzorix pins `targetSdk = 28`, and that is a platform constraint, not a choice. On
Android 10 and later, an app targeting API 29+ **cannot `execve()` a binary from its
own data directory** — the SELinux domain `untrusted_app_29` and above lacks the
`execute_no_trans` rule that `untrusted_app_27` (the domain a `targetSdk 28` app gets)
has. The container's Debian binaries live in app storage, so the app must be in the
domain that may execute them.

There is no userspace workaround. The escape hatch is a rooted backend, which is a
different feature. The consequence is fixed and worth stating plainly: **distribution
is sideload / F-Droid / direct APK, permanently.** Play requires a recent API level and
raises that floor every year, so a `targetSdk 28` app can never ship there.

Android 14's minimum *installable* target is 23, so a `targetSdk 28` app still installs
on 14/15/16. The risk is a future Android raising the installable floor past 28.

---

## 2. First install

A user sideloads the APK: download it, open it, confirm the install from unknown
sources. The manifest declares `REQUEST_INSTALL_PACKAGES` for the *update* path below,
not for the first install.

After installing, the user opens the app once. Many OEMs block autostart until first
launch, so this is a real step, not a formality. Onboarding then proceeds as
`DEVICE_ONBOARDING.md` describes.

---

## 3. OTA updates

### 3.1 The flow

```
device connects ──▶ GET /v1/check-update?version_code=N
                      │
                      ├─ update_available=false ─▶ done
                      │
                      └─ update_available=true
                             │
                             ▼
                    GET /v1/apk/<apk_filename>   (tens of MB)
                             │
                             ▼
                    SHA-256 vs manifest ─ mismatch ─▶ refuse, delete
                             │ match
                             ▼
                    installer intent ─▶ user taps ─▶ installed
```

The check runs on every socket connect — the same trigger as telemetry — so a device
that has been offline checks as soon as it can reach the server. It is not a poll loop:
the check is cheap, but the download is not.

### 3.2 The device side

| Piece | What it does |
|:--|:--|
| `UpdateCheck` | The manifest DTO, mirroring the server's fields exactly. |
| `UpdateClient` | `check(versionCode)` and `download(check, file)`. The download **verifies the SHA-256** and refuses a mismatch or a missing hash. |
| `UpdateChecker` | The whole pass: check → download → verify → hand to the installer. Never throws; a failure means the device keeps its current build. |
| `ApkInstaller` | The seam for the install. `AndroidApkInstaller` launches the system installer; a test records that the APK arrived. |

`UpdateChecker` runs on its own daemon thread from the connection's `onConnected`, so a
64 MB download cannot stall the socket reader.

### 3.3 Why the install needs the user

A third-party app cannot install a package silently — that needs a system signature or
root. So the installer launches an `ACTION_VIEW` intent on the APK with the
package-archive MIME type, and the user confirms. **The mechanism is automatic; the
last tap is consent.** That is Android's rule, not a gap in the design.

The APK is exposed through a `FileProvider`, because the installer runs in another
process and cannot read the app's private storage, and a `file://` URI throws
`FileUriExposedException`. The provider serves only the staging directory
(`files/updates/`).

### 3.4 The server side

| Route | Auth | What it serves |
|:--|:--|:--|
| `GET /v1/check-update?version_code=N` | public | `update_available` plus the manifest fields, flattened |
| `GET /v1/version` | public | the manifest verbatim |
| `GET`/`HEAD /v1/apk/<name>` | public | the APK, with the manifest hash checked before serving |
| `POST /v1/download-progress` | public | the device's progress report; **stored**, not just logged |
| `GET /v1/download-progress/:deviceId` | public | the latest progress for a device, or `downloading:false` |

All of these are registered both with and without the `/api` prefix, because the proxy
strips `/api`.

### 3.5 Download progress

The device reports progress as it downloads, fired when the whole-percent changes (a
64 MB download in 8 KB reads would otherwise be thousands of requests). A user
reads it back with `GET /v1/download-progress/:deviceId`.

The report is stored in an in-memory store keyed by device, with a 2-minute TTL — a
device that stops reporting (crashed, killed, offline) reads as `downloading:false`
rather than as a frozen percentage. The store is deliberately not durable: "how far
along is this download right now" is meaningless after a restart, and the device
re-reports on its next tick.

The percent is derived from `bytesLoaded`/`totalBytes` so the two cannot disagree; the
`progress` field the device sends is accepted but not trusted over them.

---

## 4. Publishing a build

```
tooling/scripts/generate_version.sh <apk> <version> <version-code> "<notes>"
```

It writes the two paths the **running server reads** — `apps/api/data/version.json` and
`apps/api/bin/<apk>` — not a separate copy. An earlier revision wrote `api/v1/version.json`
and `bin/`, which the server never read, so the served manifest stayed a stub while the
script reported success. Override with `DATA_DIR` / `BIN_DIR` when the server runs
elsewhere.

The manifest carries the SHA-256, so a device refuses an APK that does not match it.

---

## 5. The distribution trap (read this before wiring a device to GitHub)

The APK release lives in the **public** `VinnsEdesigner/gz-_ASSETS` repository:

```
https://github.com/VinnsEdesigner/gz-_ASSETS/releases/tag/app-v0.1.0
https://github.com/VinnsEdesigner/gz-_ASSETS/releases/download/app-v0.1.0/vyzorix-app-v0.1.0.apk
```

That download is **anonymous** (verified: HTTP 200, 64 MB, no token). It has to be: the
app's own repository is **private**, and GitHub serves a private repository's release
assets only to authenticated callers — so a device, which has no token, gets a `404`
from it. This is the same trap the container rootfs assets hit.

A device is therefore **not** wired to GitHub: it downloads from the **server's**
`/v1/apk/<name>` route. The public release exists for the case where a device, or an
user, should fetch the APK without the server in the path.

### The `make_latest: false` trap

The container rootfs is fetched via `releases/latest/download/...`. So a new release in
`gz-_ASSETS` must **not** become "latest" — if it did,
`releases/latest/download/debian-core-*.tar.gz` would `404` and every fresh install would
fail to fetch its userspace. The APK release is therefore created with
`"make_latest": "false"`, and `latest` still resolves to the container assets. This was
verified after publishing, and it is worth verifying after every publish.

---

## 6. Verified live

Against a server on the host and the Android 9 emulator:

- `GET /v1/check-update?version_code=1` → `update_available:true`, a manifest with a
  real 64 MB size and a real SHA-256.
- `UpdateLiveE2ETest` (2 tests, 0 skipped) runs the production `UpdateClient` and
  `UpdateChecker` from the device: the check, the 64 MB download and the checksum
  verification all happen, and the verified APK reaches the install step. The second
  test proves a device already on the latest code does **not** download.
- `HEAD /v1/apk/<name>` → 200 (it answered 400 before the HEAD routes were registered).

---

## 7. The limits

- **The last tap is the user's.** Automatic download, automatic verification, manual
  install. A silent install needs a system signature or root.
- **No staged rollout, no channels.** The server serves one latest version.
- **The private-repository trap is permanent** while the app repository is private: any
  asset a device must fetch anonymously has to live in a public repository.
