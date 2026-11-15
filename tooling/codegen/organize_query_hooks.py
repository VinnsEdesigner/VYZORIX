#!/usr/bin/env python3
"""Post-process orval react-query output: rename generic tag files to
explicit Vyzorix-domain filenames.

orval generates files as <tag>/<tag>.ts (e.g., devices/devices.ts).
This script renames them to explicit names (e.g., device-management.ts)
and fixes the rest-bridge import path.

Run after: node node_modules/.pnpm/orval*/node_modules/orval/dist/bin/orval.mjs --config orval.config.js
"""
import json
import os
import re
import shutil

SOURCE = "apps/VyzoriX_web/src/generated-rq"
DEST = SOURCE  # orval now outputs directly to the web app; we rename in-place

# Map: orval tag name → explicit Vyzorix filename (without .ts)
# This gives every file a name that's self-explanatory without context.
TAG_RENAMES = {
    "admin": "admin-operations",
    "alerts": "alert-rules",
    "annotations": "dashboard-annotations",
    "api-keys": "user-api-keys",
    "auth": "auth-session",
    "channels": "websocket-channels",
    "client-credentials": "oauth-client-credentials",
    "commands": "device-commands",
    "config-versions": "config-version-history",
    "connections": "device-connections",
    "contact-points": "notification-contact-points",
    "dashboard": "dashboard-stats",
    "devices": "device-management",
    "diagnostics": "device-diagnostics",
    "inbox": "device-inbox",
    "invitations": "organization-invitations",
    "members": "organization-members",
    "mfa": "mfa-management",
    "organizations": "organization-management",
    "service-accounts": "service-accounts-management",
    "sessions": "auth-sessions",
    "settings": "user-settings",
    "telemetry": "device-telemetry",
    "updater": "ota-updater",
    "updates": "update-management",
    "usage-stats": "usage-statistics",
}

# orval's `query.mutationInvalidates` emits
#     queryClient.invalidateQueries({ queryKey: getGetXQueryKey() });
# for every invalidated query. That is correct only when the query key needs no
# arguments. A query keyed on a required path param (e.g. `imei`) declares
#     export const getGetXQueryKey = (imei: string) => ...
# so the generated call is a type error ("Expected 1 arguments, but got 0").
# Drop those calls from the auto-generated `onSuccess`; the hand-written
# mutation hooks re-issue the correct invalidations (they know the path params)
# via the generated `invalidateGetX` helpers.
_EMPTY_KEY_CALL = re.compile(
    r"[ \t]*queryClient\.invalidateQueries\(\{ queryKey: (get\w+QueryKey)\(\) \}\);[ \t]*\n?"
)
# A key function that requires at least one argument (does not take `() =>`).
_REQUIRED_ARG_KEY = re.compile(r"export const (get\w+QueryKey) = \(([^)]*)\) =>")
# The `if (!options?.skipInvalidation) { ... }` wrapper orval emits around the
# invalidate calls, once every call inside it has been removed.
_EMPTY_INVALIDATION_GUARD = re.compile(
    r"[ \t]*if \(!options\?\.skipInvalidation\) \{[ \t]*\n[ \t]*\}[ \t]*\n"
)


def fix_empty_invalidate_calls(content, key_owner=None, self_module=None):
    """Rewrite orval's `onSuccess` invalidations so the file compiles.

    orval's `mutationInvalidates` emits
        queryClient.invalidateQueries({ queryKey: getGetXQueryKey() });
    for every invalidated query, in *every* file of the mutation's tag. Three
    cases:

    1. the key is defined here and callable with no args -> keep as-is;
    2. the key is defined here but takes a required argument -> the `()` call is
       a type error, so drop it (a hand-written hook re-issues it with the path
       params it knows);
    3. the key is defined in another tag file -> orval does not import it, so the
       name is unresolvable. Import it from its owner module and keep the call
       (when it is callable with no args), rather than silently losing the
       invalidation.

    Returns `(content, imports)` where `imports` maps a module specifier to the
    set of key names to import from it. `key_owner` maps a key name to its module
    specifier relative to the generated-rq root.
    """
    key_owner = key_owner or {}

    # Every key function this file actually defines.
    defined = set(name for name, _ in _REQUIRED_ARG_KEY.findall(content))
    defined |= set(re.findall(r"export const (get\w+QueryKey) = \(\) =>", content))

    # Key functions whose first parameter is required (no `?`).
    required = set()
    for name, params in _REQUIRED_ARG_KEY.findall(content):
        first = params.split(",")[0].strip()
        if not first:
            continue  # `() =>` — no parameters, callable with no args
        param_name = first.split(":")[0].strip()
        if param_name and not param_name.endswith("?"):
            required.add(name)

    imports = {}

    def _sub(match):
        key_fn = match.group(1)
        # Case 2: a locally-defined key that needs an argument we cannot supply.
        if key_fn in required:
            return ""
        # Case 3: a key owned by another tag file — import it and keep the call.
        if key_fn not in defined:
            owner = key_owner.get(key_fn)
            if owner is None or owner == self_module:
                return ""  # unknown, or self (shouldn't happen) — drop, fail safe
            spec = "../" + owner  # owner is "tag/file", relative from "tag/file.ts"
            imports.setdefault(spec, set()).add(key_fn)
        return match.group(0)

    rewritten = _EMPTY_KEY_CALL.sub(_sub, content)
    # When every call inside the guard was removed, drop the empty guard too.
    rewritten = _EMPTY_INVALIDATION_GUARD.sub("", rewritten)
    # With the calls gone, a mutation-options function may no longer use its
    # `queryClient` parameter; rename it so `noUnusedParameters` stays happy.
    rewritten = _rename_unused_query_client(rewritten)
    return rewritten, imports


_MUT_OPTIONS_PARAM = "(queryClient: QueryClient, options?:"


def _rename_unused_query_client(content):
    """Rename `queryClient` to `_queryClient` in mutation-options functions that
    no longer reference it after their invalidations were removed."""
    parts = re.split(r"(?=^export const )", content, flags=re.M)
    for i, part in enumerate(parts):
        if _MUT_OPTIONS_PARAM not in part:
            continue
        # The signature itself contributes one occurrence; anything more means
        # the body still uses it.
        if part.count("queryClient") > 1:
            continue
        parts[i] = part.replace(
            _MUT_OPTIONS_PARAM, "(_queryClient: QueryClient, options?:", 1
        )
    return "".join(parts)


def main():
    if not os.path.isdir(SOURCE):
        print(f"Source not found: {SOURCE}")
        return

    # The schemas file stays at the root level — don't touch it.
    schemas_src = os.path.join(SOURCE, "vyzorixUpdateServerAPI.schemas.ts")
    schemas_content = None
    if os.path.exists(schemas_src):
        with open(schemas_src) as f:
            schemas_content = f.read()

    # Collect all tag directories before renaming (can't rename while iterating)
    tag_dirs = []
    for name in sorted(os.listdir(SOURCE)):
        full = os.path.join(SOURCE, name)
        if os.path.isdir(full):
            tag_dirs.append((name, full))

    # Pass 1: map every query-key function to the module (tag/new_basename) that
    # defines it, so a mutation in one tag can import the keys it invalidates in
    # another. orval emits cross-tag invalidations but never the import.
    key_owner = {}
    for tag_name, tag_dir in tag_dirs:
        ts_files = [f for f in os.listdir(tag_dir) if f.endswith(".ts")]
        raw_name = f"{tag_name}.ts"
        old_filename = raw_name if raw_name in ts_files else (ts_files[0] if ts_files else None)
        if old_filename is None:
            continue
        new_basename = TAG_RENAMES.get(tag_name, tag_name)
        module = f"{tag_name}/{new_basename}"
        with open(os.path.join(tag_dir, old_filename)) as f:
            text = f.read()
        for name in re.findall(r"export const (get\w+QueryKey)\b", text):
            key_owner[name] = module

    index_lines = []

    for tag_name, tag_dir in tag_dirs:
        # Find the .ts file inside
        ts_files = [f for f in os.listdir(tag_dir) if f.endswith(".ts")]
        if not ts_files:
            continue

        # orval emits the raw file as `<tag>.ts`; prefer that as the source so a
        # stale already-renamed file left from a previous run can't be mistaken
        # for it.
        raw_name = f"{tag_name}.ts"
        old_filename = raw_name if raw_name in ts_files else ts_files[0]
        old_basename = old_filename.replace(".ts", "")
        new_basename = TAG_RENAMES.get(tag_name, tag_name)
        new_dir = os.path.join(SOURCE, tag_name)  # keep original folder name

        # Read old file
        old_path = os.path.join(tag_dir, old_filename)
        with open(old_path) as f:
            content = f.read()

        # Fix rest-bridge import path
        content = content.replace(
            "../../generated/rest-bridge",
            "../../../../packages/API_Client/src/generated/rest-bridge",
        )

        # Fix schemas import: point directly to @vyzorix/api-client instead
        # of the local duplicate vyzorixUpdateServerAPI.schemas.ts
        content = content.replace(
            "from '../vyzorixUpdateServerAPI.schemas'",
            "from '@vyzorix/api-client'",
        )

        # Fix auto-invalidation: drop calls whose key needs an argument, and
        # import (rather than drop) keys owned by another tag file.
        self_module = f"{tag_name}/{new_basename}"
        content, extra_imports = fix_empty_invalidate_calls(
            content, key_owner=key_owner, self_module=self_module
        )
        if extra_imports:
            block = "".join(
                "import { %s } from '%s';\n" % (", ".join(sorted(names)), spec)
                for spec, names in sorted(extra_imports.items())
            )
            # Insert after the last top-level import line.
            lines = content.split("\n")
            last = max(
                (i for i, l in enumerate(lines) if l.startswith("import ")),
                default=-1,
            )
            lines.insert(last + 1, block.rstrip("\n"))
            content = "\n".join(lines)

        # Create new dir and write file
        os.makedirs(new_dir, exist_ok=True)
        new_path = os.path.join(new_dir, f"{new_basename}.ts")
        with open(new_path, "w") as f:
            f.write(content)

        # Copy any additional files from old dir
        for item in os.listdir(tag_dir):
            if item.endswith(".ts") and item != old_filename:
                src = os.path.join(tag_dir, item)
                dst = os.path.join(new_dir, item)
                if not os.path.exists(dst):
                    shutil.copy2(src, dst)

        # Remove old file (folder stays, we write new file into it)
        if old_filename != f"{new_basename}.ts":
            os.remove(old_path)

        index_lines.append(f'export * from "./{tag_name}/{new_basename}";')
        print(f"  {tag_name}/{old_filename} → {tag_name}/{new_basename}.ts")

    # Write index.ts barrel
    index_path = os.path.join(SOURCE, "index.ts")
    with open(index_path, "w") as f:
        f.write("// Generated by orval + tooling/codegen/organize_query_hooks.py\n")
        f.write("// DO NOT EDIT MANUALLY — regenerate with:\n")
        f.write("//   node node_modules/.pnpm/orval*/node_modules/orval/dist/bin/orval.mjs --config orval.config.js\n")
        f.write("//   python3 tooling/codegen/organize_query_hooks.py\n\n")
        for line in sorted(index_lines):
            f.write(line + "\n")

    print(f"\nOrganized {len(index_lines)} hook files → {SOURCE}")

if __name__ == "__main__":
    main()
