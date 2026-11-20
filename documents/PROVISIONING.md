# GitOps Provisioning

> **Status:** Server complete. Console screen is being built next.
> **Applies to:** Organisations that keep their device configuration in a git repository.

---

## What this is

Provisioning keeps your devices configured from a git repository.

Instead of editing each device's configuration in the console, you write it as a file
in a repository you already control. VYZORIX reads that repository, applies each file
to the device it names, and tells you when a device no longer matches what the
repository says. Your git history becomes the record of what changed, who changed it,
and when.

If you have used Grafana's dashboard provisioning, the idea is the same. The thing being
provisioned is different: here it is a device's configuration, not a dashboard.

## Why you would use it

- **Your configuration lives where you already work.** Review it, diff it, and revert it
  with the tools you use for every other file.
- **One repository, many devices.** A single change can update a fleet, and you can see
  the whole fleet's intended state in one place.
- **Drift is visible.** If someone changes a device outside the repository, VYZORIX
  shows that device as drifted rather than quietly leaving it alone.
- **Nothing disappears without you asking.** Removing a file from the repository does
  not wipe a device's configuration. You are told, and you decide.

## The one concept to understand

A **provisioned resource** is one configuration file for one device.

```
devices/
  living-room/
    vyzorix.toml
  workshop/
    vyzorix.toml
```

Each of those files is a separate resource, bound to the device named by its folder. Two
devices are two resources even when their files are identical, because applying either
one targets a different device.

The folder name is the device id. That is what ties a file to a device.

### What a file looks like

A minimal file names the device and the apps it runs:

```toml
[device]
id  = "living-room"
org = "my-organisation"

[[app]]
name = "dashboard"
cmd  = "/home/apps/dashboard/run"
```

Every field is checked before anything is applied. A misspelled key is an error, not a
setting that silently does nothing — the file is refused and the message names the key.

Two rules are worth knowing up front:

- **The `cmd` must live inside the app's own folder.** `/home/apps/dashboard/run` is
  accepted for an app named `dashboard`; a path outside it is refused, so one app cannot
  point at another's files.
- **The device id in the file must match its folder.** A file in `devices/living-room/`
  that names a different device is refused rather than applied somewhere unexpected.

Optional per-app settings — environment variables, exposed ports, autostart, secrets the
app needs, and a web-app manifest — are described in the manifest schema the console
links to.

## Setting it up

### 1. Add a connection

A connection is a stored git credential. It is shared by every repository in your
organisation that talks to the same provider.

You give it:

| Field | What it means |
|---|---|
| Name | A label you will recognise, such as `GitHub — platform team` |
| Provider | GitHub, GitLab, or Generic git |
| Host | Where the provider lives, for example `github.com` |
| Token | The credential VYZORIX uses to read the repository |

The token is **write-only**. Once saved it is never shown again, by any screen. If you
lose it, you replace it; you cannot read it back.

After saving, VYZORIX immediately checks whether it can reach the host and authenticate
with the token. A connection that fails that check is still saved, marked with the
reason, so you can fix the token rather than re-entering everything.

Two further options exist on a connection:

- **Check now** re-runs that health check at any time.
- **Write access** declares that the token may also be used to commit back to the
  repository. It is off by default, because reading is all that syncing needs. Turn it
  on only if you intend to edit files from the console.

### 2. Add a repository

A repository is one branch of one remote, with a schedule.

| Field | What it means |
|---|---|
| Name | A short identifier, unique in your organisation |
| Title and description | What a teammate sees in the list |
| URL | The git URL, `https` or `ssh` |
| Branch | The one branch this repository tracks |
| Path | A subfolder to read from, or `/` for the whole repository |
| Connection | The credential to read it with |
| Sync interval | How often to check for changes, minimum 60 seconds |
| Sync enabled | Whether the schedule runs at all |

VYZORIX checks the repository and branch are reachable with the connection's token
**before** saving. A typo in the URL is an error on the form, not a repository that
fails forever afterwards.

Two things worth knowing:

- **One branch per repository.** Tracking a second branch means adding a second
  repository. That keeps every status field unambiguous — "last synced" always refers to
  one branch.
- **A URL carrying a password is refused.** Put the credential in the connection, where
  it is protected and never returned. A token pasted into a URL would be stored in the
  clear and shown back to anyone who can read the repository.

### 3. See what it would do

Before the first sync, and at any time afterwards, you can ask what a sync would change.
This is a preview: it reads the repository, compares it to what VYZORIX already manages,
and reports the result **without touching a single device**.

Each file comes back as one of:

| Result | Meaning |
|---|---|
| Will be applied | A new file for a device this repository does not manage yet |
| Will be updated | The file changed |
| Unchanged | Nothing to do |
| Conflict | Another repository already manages this device |
| Missing | The file is no longer in the repository |

A **conflict** means two repositories both claim the same device. VYZORIX will not guess
which one is right. The second repository's file is refused and the message names the
repository that already owns the device.

### 4. Sync

A sync is one pass over the repository. You can run one at any time, and the schedule
runs them for you.

Every sync produces a job you can open afterwards:

- When it started and how long it took
- How many files were applied, skipped, and failed
- One row per file, with the outcome and the reason

A single broken file does not stop the rest. If one file cannot be applied, that file
fails and the others still go ahead.

### 5. Watch for drift

Each resource shows a state:

| State | What it means |
|---|---|
| **Applied** | The device is running the configuration this repository applied |
| **Drifted** | The device was changed by something else after this repository applied it |
| **Missing** | The file was removed from the repository; the device keeps what it has |
| **Error** | The last attempt to apply it failed |

**Drifted** is the one to watch. It means the device no longer matches your repository,
usually because someone changed it in the console or a deploy was pushed another way.
VYZORIX shows it rather than silently correcting it, so you can decide whether the
repository or the device should change.

## What a sync will never do

These are guarantees, not settings:

- **A sync never deletes a device.** It changes configuration. Device lifecycle is not
  provisioning's business.
- **Removing a file does not wipe a device.** The resource is marked missing and the
  device keeps running what it has. Reverting is a separate, explicit action.
- **One device, one repository.** Two repositories cannot both manage the same device.
- **A failed file does not half-apply.** Each device's configuration is written whole or
  not at all.

## Reverting a device

When a file disappears from the repository, the device keeps its current configuration
and the resource is marked **missing**.

If you actually want the device rolled back, ask for it explicitly. A sync run with the
revert option restores each affected device's previous configuration. The preview tells
you this is what will happen, so you are never surprised by a rollback you did not
intend.

## Editing a file from the console

If a connection has **write access**, you can edit a provisioned file in the console and
commit it back to the repository.

You see a diff first. Nothing is committed until you approve it. Two modes exist:

- **Pull request** (default) — the change goes to a new branch and a pull request is
  opened, so a reviewer sees it before it lands. A provisioned file can change what a
  device runs, which deserves a second pair of eyes.
- **Direct** — the change commits straight to the repository's branch, for a
  single-maintainer repository where a pull request is ceremony.

If the repository changed while you were editing, the commit is refused rather than
overwriting someone else's work. You re-read the file and try again.

## Webhooks (optional)

By default a repository is checked on its schedule. If your provider can reach your
VYZORIX instance, you can also have a push trigger a sync immediately.

You give the repository a webhook secret, and your provider signs each push with it.
VYZORIX verifies that signature before doing anything. A request that does not verify is
refused. A push to a branch the repository does not track is acknowledged and ignored.

If you cannot expose a public endpoint — a self-hosted instance behind a firewall, for
example — leave webhooks off. The schedule still works.

## Secrets and app configuration

A provisioned file names the secrets an app needs; it never contains them.

Set the secret value separately in the console. VYZORIX encrypts it to the device's key,
so only that device can read it, and the value is never shown again. The device fetches
it only if its current configuration names that key.

## Permissions

Two permissions cover provisioning, both scoped to your organisation:

| Permission | What it allows |
|---|---|
| `provisioning.read` | View repositories, connections, resources, jobs, and history |
| `provisioning.write` | Create, edit, and delete them, and run syncs |

Both are granted to **admin** and **super admin** by default, and **not** to operator or
viewer. The reason is scope: a provisioned repository can rewrite any device's
configuration in your organisation, which is fleet-wide authority.

No permission, of any level, returns a stored token. Connections and secrets are
write-only.

## The audit trail

Every provisioning action is recorded: who created or changed a connection or
repository, who triggered a sync, and what it did. Each entry distinguishes a person,
a service account, the schedule, and the server itself — so "who changed this" has an
answer even when the change was automatic.

Deploys are recorded the same way. A change to what a device runs is logged with the
device, the version, and the person or credential behind it, whichever route it came
through.

## What happens to the existing boot seed

If your instance was set up from a provisioning file at startup, that file still applies
once at boot and still seeds organisations, users, service accounts, keys, alert rules,
and the app catalog.

It is not replaced by this feature, and it is not the same thing. Those resources are
shaped like a deployment — an organisation, a user — rather than shaped like a device.
Folding them into per-device files would be a false unification.

The two coexist. The boot seed sets up the instance; repositories keep the devices
configured.

## Current limitations

Stated plainly, so nobody plans around something that is not there:

- **The console screen is still being built.** The Provisioning page currently shows a
  placeholder. Everything described here works today through the API, but the point-and-click
  screen is the next step.
- **Pull only, by default.** Syncing reads from the repository. Writing back requires a
  connection with write access, and only edits made in the console use it.
- **One branch per repository.**
- **No automatic rollback.** Removing a file flags the resource; reverting is an explicit
  action.
- **Webhooks need a reachable endpoint.** Without one, the schedule is the only trigger.
- **No cross-organisation provisioning.** Everything is scoped to one organisation.
