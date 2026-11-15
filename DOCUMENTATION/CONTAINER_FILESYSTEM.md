# The container filesystem — every folder, and what it is for

**Status:** shipped. The bind list is `ProotLauncher.binds()` in
`android-core/.../container/ProotLauncher.kt`; the host paths are under the app's private
storage. This document is the map.
**Related:** the device-side `Vyzorix-android/doc/DEVICE_FILESYSTEM_ACCESS.md` (the
measured platform limits behind these choices), `CONTAINER_ANDROID_APIS.md` (the socket
one of these folders carries), `APP_RUNTIME.md` (what lives under `/home/apps`),
`DEPLOY_SECRETS.md` (the `.env` file).

This is the document to read when you are standing inside the container and asking "what
is this folder, why can I see it, and is it safe to write here?". It covers the host
side, the container side, and the rules that decide what crosses between them.

---

## 1. The two filesystems

There are two filesystems in play, and keeping them straight is most of the confusion:

- **The host side** is the Android app's private storage,
  `/data/user/0/<pkg>/files/` — call it `files/`. It holds the container's userspace,
  the writable overlay, the app storage, the secret store and the recordings.
- **The container side** is the Debian rootfs, presented by `proot` as `/`. It has its
  own `/bin`, `/usr`, `/lib`, `/etc` — a real Debian tree.

`proot` is a userspace `chroot` with bind mounts. The bind list is the **only** thing
that crosses from `files/` into the container's `/`, and it is a *closed world*: only
the paths named in `ProotLauncher.binds()` are bound, and nothing else is reachable by
construction. That list is the app's allowlist.

---

## 2. The host side, under `files/`

```
files/
├── rootfs/            the Debian userspace (Layer 2) — the container's own /
├── overlay/           writable prefix that survives a rootfs replacement
│   ├── usr-local/       -> /usr/local   (binds over it)
│   ├── opt/             -> /opt         (binds over it)
│   └── etc/             -> /etc/vyzorix (config that survives a rootfs swap)
├── apps/              app storage -> /home/apps (Layer 3); survives a rootfs swap
│   └── <app>/           one app: its files, its `.env`, its unpacked artifacts
├── proot-tmp/         proot's scratch dir (it writes a loader before every exec)
├── secrets/           the app's secret store — NEVER bound
├── recordings/        terminal recordings — NEVER bound
├── updates/           a downloaded APK staged for the installer
├── android.sock       the bridge socket (the door into Layer 1)
├── extended.installed  a marker: the optional extended tier is complete
└── (config, logs)
```

Each of these has a reason:

| Path | What it is | Why it is here |
|:--|:--|:--|
| `rootfs/` | The extracted Debian tree. | The container's `/`. Replaced wholesale on a container update. |
| `overlay/` | A writable prefix, seeded with `usr-local/`, `opt/`, `etc/`. | `proot` has no `mount`, so there is no overlayfs; the overlay is a set of binds. It survives a rootfs replacement, so on-device installs are not lost when the base is swapped. |
| `apps/` | App storage, bound to `/home/apps`. | **Layer 2 is disposable; Layer 3's storage is not.** An app's files and data must survive a container update, so they live outside the rootfs. |
| `proot-tmp/` | proot's `PROOT_TMP_DIR`. | proot writes a temporary loader before every exec, and `P_tmpdir` (`/data/local/tmp`) is not writable by an app. Without this, the first exec fails with `can't create temporary file`. |
| `secrets/` | The app's secret store. | Holds secrets. **Never bound** — see §5. |
| `recordings/` | Terminal recordings. | Hold session content. **Never bound** — see §5. |
| `updates/` | An APK staged for the installer. | The installer runs in another process and cannot read private storage; this is what the `FileProvider` serves. |
| `android.sock` | The bridge socket. | The door the container reaches Layer 1 through. The **file** is bound, not the directory that holds the secret store. |

---

## 3. What is bound, and where it appears inside

`ProotLauncher.binds()` is the closed world. This is the current list:

| Host | In the container | Why |
|:--|:--|:--|
| `/dev`, `/proc`, `/sys` | same | Kernel interfaces the container needs. |
| `rootfs/etc/resolv.conf` | `/etc/resolv.conf` | DNS; without it there is no name resolution inside. |
| `overlay/usr-local` | `/usr/local` | Writable overlay that survives a rootfs swap. |
| `overlay/opt` | `/opt` | Same. |
| `overlay/etc` | `/etc/vyzorix` | Config that survives a rootfs swap. The target is a directory the rootfs does not ship, so the overlay is the only thing there. |
| `apps/` | `/home/apps` | App storage (Layer 3). |
| shared-storage root | `/sdcard` | The folders a stock device exposes (Download, DCIM, Pictures, Music, Movies, Documents, `Android/media`). |
| `android.sock` | `/tmp/android.sock` | The API bridge. |
| `/system`, `/vendor`, `/product`, `/odm`, `/oem` (those present) | `/android/<name>` | The device's read-only trees, under a **prefix** so they cannot shadow Debian's `/system` etc. |

Two decisions in that table are worth their own paragraph.

### The overlay is bound over `/usr/local` and `/opt`, never the whole tree

`overlay/usr-local` and `overlay/opt` bind *over* the rootfs's `/usr/local` and `/opt`,
so a container update — which replaces `files/rootfs` — leaves anything installed into
those directories in place. The overlay's `etc` binds to the container's `/etc/vyzorix`,
a directory the rootfs does not ship, so the overlay is the only thing there.

The overlay root itself is never bound, and neither is `files/`. Only the three named
subdirectories cross.

### The Android trees are under a prefix, not at their host paths

The container's `/` is the Debian rootfs, which has its own `/bin`, `/usr`, `/lib`,
`/etc`. Binding a host tree at `/system` would be harmless only by luck, and anything
landing on a shared top-level name would silently shadow the Debian filesystem. So the
Android trees appear under `/android`: `/system` becomes `/android/system`. A prefix
makes the boundary explicit — these are the device's files, read-only, over there.

They are **not** runnable inside the container: `/system/lib64` is bionic and the guest
is glibc Debian. This is for inspection and for the odd host file a tool wants to read,
not for executing Android binaries.

---

## 4. What each container-side folder is for

Inside the container you will meet these:

| Path | What it is | Writable? |
|:--|:--|:--|
| `/` | The Debian rootfs (`files/rootfs`). Replaced on a container update. | Avoid; changes here are lost on update. |
| `/usr/local`, `/opt` | The writable overlay. On-device installs belong here. | Yes — survives a container update. |
| `/etc/vyzorix` | Config from the overlay. Survives a container update. | Yes. |
| `/home/apps` | App storage (`files/apps`). Each app gets `/home/apps/<name>/`. | Yes — this is where apps live and keep state. |
| `/home/apps/<app>/.env` | The app's secrets, mode `0600`, written by the agent. | Read; do not copy elsewhere. |
| `/home/apps/supervisor.sock` | The supervisor's control socket (default). | n/a |
| `/tmp/android.sock` | The bridge socket (the bound file). | n/a |
| `/sdcard` | The device's shared storage. | Yes, within the app's own permissions. |
| `/android/...` | The device's read-only trees, under a prefix. | No. |
| `/tmp` | Inside the container. `tmux` uses `/tmp/tmux-0/`. | Yes. |

The rule of thumb: **install into the overlay (`/usr/local`, `/opt`), keep state under
`/home/apps`, and treat `/` as disposable.**

---

## 5. What is never bound, and the second check

The app's own private tree (`files/`, i.e. `/data/user/0/<pkg>`) is **not** bound as a
whole. It holds the rootfs, the overlay, the secret store and the recordings, so binding
it would hand the container the keys to itself. Only named subdirectories cross (`apps`,
the socket) plus one file out of the rootfs for DNS.

Two independent guarantees enforce this:

1. **The closed-world list.** Only the paths named in `binds()` are bound, so the
   container cannot reach the rest by construction — not by remembering to exclude it.
2. **The sandbox-zone assertion.** `ProotLauncher.isSandboxZonePath()` refuses any path
   under the secret store or the recordings, even if one were added to the bind list by
   mistake. The `binds()` function asserts this invariant before returning, and
   `ProotLauncherTest` asserts it too.

Two checks, so one mistake does not open the zone. The server-side policy agrees: the
`fs:container` zone appears in no role grant and no tier policy, so it is unaddressable
rather than merely un-granted.

---

## 6. A bind cannot grant more than the app already has

This is the sentence that makes the whole model safe, and it is worth stating in full:

**`proot` does not change the real uid.** A container process runs as the app's own uid,
in the app's SELinux domain, under the same Unix permissions. The bind changes what the
container can *see*, never what the app may *touch*. So `/sdcard/Android/data`,
`/sdcard/Android/obb` and other apps' collections stay denied to the container exactly as
they are denied to the app.

`proot`'s `-0` flag fakes *userspace* root, which is what makes `apt` and `dpkg` behave —
but it is a lie told to `getuid()`, not a privilege change the kernel honours. This is
why the container can `apt install` into the overlay and still cannot read another app's
data.

---

## 7. The rootfs, and the two things that look like bugs

The Debian tree at `files/rootfs` is extracted from a tarball bundled in the APK (the
core tier) or downloaded on demand (the extended tier). Two facts about it will look like
bugs the first time:

1. **`/bin`, `/sbin`, `/lib` are symlinks to `usr/`.** Debian's merged-usr layout.
   `bin/bash` is really `usr/bin/bash`. `File(rootfs, "bin/bash").exists()` follows
   symlinks, so this is fine — but a `tar -t | grep '^./bin/bash$'` will not find it.
2. **`/opt` and `/usr/local` are deliberately empty.** The overlay binds over them.
   Content shipped there would be shadowed and look like a bug.

The rootfs is **per-architecture**: `arm64`, `armeabi-v7a` (`armhf`) and `x86_64`. A
rootfs for one architecture cannot run on another — the failure is a `bin/bash` that
exists and cannot execute, which reads as a corrupt rootfs rather than a wrong-arch one.
The extended tier always downloads; the core tier is bundled, so a device with no network
still gets a working container.

---

## 8. Sizes and the storage check

Measured, not estimated: `debian-core` is about **46 MB** compressed (~145 MB extracted)
and `debian-extended` about **108 MB** compressed. A device with both carries roughly
**300 MB** in `files/rootfs`. Budget for it in any storage check, and never assume `/data`
has room without asking `StatFs` — a phone's free space is not the user's to assume.

---

## 9. Quick reference: where do I put a file?

| I want to… | Put it in… | Because |
|:--|:--|:--|
| install a tool that survives a container update | `/usr/local/...` | the overlay binds there |
| keep an app's data | `/home/apps/<app>/...` | app storage survives updates |
| write configuration that survives an update | `/etc/vyzorix/...` | the overlay's `etc` binds there |
| read a device file | `/android/system/...` | the read-only trees are under the prefix |
| read or write a user's downloads | `/sdcard/...` | shared storage is bound, within the app's permissions |
| store a secret | nowhere in the container by hand — use `vyzorix secrets set` | the agent writes `.env` mode `0600`; nothing else is safe |
