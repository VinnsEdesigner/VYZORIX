# Frontend Local Store — Data at Rest

**Status:** Implemented
**Scope:** `apps/VyzoriX_web/src/lib/local-store`

---

## 1. What is stored

The local store is a cache of server data in the browser, held in SQLite
(WebAssembly) over OPFS. It may contain, per signed-in user:

- device records: id, name, organization id, online state, tier
- alert rules and contact points
- logs, telemetry samples, events and command history for a bounded window

It holds no credentials, no session tokens and no authorization decisions.

## 2. What is protected, and how

Every value written to the store is encoded before it reaches the database:

| Element | Treatment |
|---|---|
| Database file name | Derived: `vyzorix-<128-bit-hex>.db`, not a fixed name |
| Column values (ids, names, states) | AES-CTR, IV derived from the plaintext |
| Row payloads (the full JSON object) | AES-CTR, random IV |
| Metadata values (the sync cursor) | AES-CTR, random IV |
| Integer columns (`fetched_at`, `at`) | **Not encoded** — see §4 |

The key is derived with PBKDF2-SHA256 (100,000 iterations) from:

- the **scope** — the signed-in user's id, passed to the worker in the `open`
  request (a worker cannot read `localStorage`, so the caller supplies it)
- a **per-origin random salt** of 32 bytes, created once and held in
  `localStorage` under `vyzorix.localStore.salt`

Two different scopes therefore produce different database names and different
keys, so one browser profile can never read another user's cached data.

### Why the IV differs by column

A random IV is the right default, but a column used in a `WHERE ... = ?` needs a
*deterministic* one — otherwise the same input encodes differently each time and
never matches. So:

- **deterministic** (IV = HMAC-SHA256 of the plaintext, truncated to 16 bytes)
  for every column value, because those are the ones queried by equality
- **random** for payloads and metadata, which are never used as a lookup key

The trade-off is deliberate: deterministic encoding leaks equality — an observer
can see that two rows share a value without knowing what it is. That is
unavoidable if columns must be queryable, and it is why the values are encoded
rather than the payloads only.

## 3. Sign-out

`useLocalStore` resolves the store from the signed-in user's id and calls
`disposeLocalStore()` when the user becomes null. Sign-out therefore closes the
worker and clears the cached rows, so the next person to use the browser does
not inherit the previous user's data.

Disposal also runs on any identity change: a different user id disposes the old
store before opening the new one.

## 4. What this is NOT — encryption at rest

**This is obfuscation with a defined purpose, not encryption.** The key is
derived in the page, from a salt stored beside the data, by code the browser
runs. Stated plainly:

| Threat | Protected? |
|---|---|
| Another person using the same browser profile | **Yes** — different scope, different key and file name |
| Someone with the device (stolen laptop, shared disk) | **No** — the key is recoverable from the page |
| Script execution in this origin (XSS) | **No** — the attacker runs where the key is derived |
| The signed-in user reading their own cache | **No** — it is their data |

Anyone able to read the OPFS file *and* run script in this origin can recover
the key. The design assumes that if an attacker has script execution, the store
is the least of the problems — but the claim is not made that the data is
unreadable at rest.

Two specific consequences worth stating:

- **Integer columns are not encoded.** `fetched_at` and the bounded tables'
  `at` column are stored as plain integers, because they are range-queried for
  eviction and encoding them would make that impossible. They leak timing
  information — how many rows exist and when they arrived — but not content.
- **The row count and table names are visible.** Table names are fixed
  (`devices`, `logs`, …), so an observer with file access learns the shape of
  the cache even without reading values.

## 5. Migration from an earlier build

An earlier revision of this store used a fixed database name (`vyzorix.db`) and
stored values in plaintext. On the first writer open after this change, that
file is **deleted** (`dropPlaintextDatabase`), because leaving it would leave
unencoded data on disk after the store had been described as encoded.

The cost is a one-time cache miss: the store refills from the network on the
next read. Nothing is lost, because the store is a cache.

## 6. If stronger protection is needed

The honest options, in increasing cost:

1. **Do not cache the data.** The only protection that holds against a local
   attacker is not having the data.
2. **Shorten the window.** Reduce `BOUNDED_WINDOW_DAYS` so less is held.
3. **Key from a non-persisted secret.** Derive from something the page must
   fetch per session (a token held only in memory). The cache then dies with the
   session and is unreadable without the live token. This changes the store from
   a persistent cache to a per-session one — a real product decision, not a
   hardening tweak.

There is no configuration of the current design that makes the data unreadable
to someone with the device. That is a property of running in a browser, not a
gap in this implementation.
