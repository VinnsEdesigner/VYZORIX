#!/usr/bin/env python3
"""Fail when a mutating operation is missing from orval's mutationInvalidates.

orval generates cache invalidation only for the operation ids it is told about in
`orval.config.js` (`override.query.mutationInvalidates`). There is no default: a
mutation that is not listed emits no `invalidateQueries`, so the UI shows stale
data after a successful write until the user reloads.

That is not hypothetical. `patchServiceAccountsId` (the service-account
disable/enable toggle) was missing, so disabling an account fired a success toast
while the status badge stayed "Enabled" until a page reload.

This script derives the operation id of every mutating operation the same way
orval does — `method + path segments`, params kept verbatim, camelCased — and
fails if one is not named in `mutationInvalidates`. A mutation with genuinely
nothing to invalidate must be listed in IGNORED below with a reason, so the
decision is recorded rather than forgotten.

Usage:
    python3 tooling/codegen/check_mutation_invalidation.py [--list]

Exit codes: 0 clean; 1 missing entries (or an ignore that no longer applies).
"""

from __future__ import annotations

import json
import os
import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SPEC = REPO_ROOT / "apps/api/swag/openapi3.json"
ORVAL_CONFIG = REPO_ROOT / "orval.config.js"

MUTATING_METHODS = {"post", "put", "patch", "delete"}

# Operations that intentionally invalidate nothing. Each entry must say why, so
# the exemption is a recorded decision rather than an oversight.
IGNORED: dict[str, str] = {
    "postAuthLogout": "clears the session and navigates away; no cached data survives",
    "postAuthLogin": "the login response hydrates the auth store directly, not a query",
    "postAuthLoginTokens": "same as postAuthLogin: the caller stores the tokens itself",
    "postAuthRefresh": "refreshes tokens in place; no query data changes",
    "postAuthRegister": "creates an account and signs nobody in; nothing cached yet",
    "postAuthForgotPassword": "sends an email; no cached data changes",
    "postAuthResendPasswordReset": "sends an email; no cached data changes",
    "postAuthResendVerification": "sends an email; no cached data changes",
    "postAuthResetPassword": "changes a credential the client does not cache",
    "postAuthVerifyEmail": "flips a server-side flag the client re-reads on next load",
    "postAuthMfaVerify": "completes a challenge; the caller refetches /me itself",
    "postAuthMfaVerifyBackup": "completes a challenge; the caller refetches /me itself",
    "postAuthOauthGithub": "redirects out of the app",
    "postAuthOauthGoogle": "redirects out of the app",
    "postInviteTokenAccept": "acceptance is handled on a standalone page",
    "postInviteTokenReject": "rejection is handled on a standalone page",
    "postAuthMeNotificationsPushTest": "sends a test notification; no cached data changes",
    "postDeploy": "the deploy hooks invalidate by hand in their own onSuccess",
    "postProvisioningRepositoriesIdResourcesResourceIdPreview": "renders a diff and writes nothing, so no cached data changes",
    "postProvisioningWebhooksRepoID": "called by the git provider, not the console",
    "postConfigVersionsResourceVersionRestore": "not used by the console yet",
    "postV1DeviceClaim": "device-initiated; the console never calls it",
    "postV1DeviceEnroll": "device-initiated; the console never calls it",
}


def camel(parts: list[str]) -> str:
    out = parts[0]
    for part in parts[1:]:
        out += part[:1].upper() + part[1:]
    return out


def operation_id(method: str, path: str) -> str:
    """Derive orval's hook suffix, e.g. PATCH /service-accounts/{id} → patchServiceAccountsId."""
    segments = [s for s in path.split("/") if s]
    words: list[str] = []
    for seg in segments:
        param = re.fullmatch(r"\{(\w+)\}", seg)
        if param:
            # A path param keeps its own spelling, split on underscores:
            # {user_id} → UserId, {id} → Id.
            words.extend(p for p in re.split(r"[_-]", param.group(1)) if p)
        else:
            words.extend(p for p in re.split(r"[-_]", seg) if p)

    return method.lower() + camel([w[:1].upper() + w[1:] for w in words])


def configured_mutations() -> set[str]:
    text = ORVAL_CONFIG.read_text(encoding="utf-8")
    block = re.search(r"mutationInvalidates:\s*\[(.*?)\n\s*\],", text, re.S)
    if not block:
        raise SystemExit("could not find mutationInvalidates in orval.config.js")

    names: set[str] = set()
    for match in re.finditer(r"onMutations:\s*\[(.*?)\]", block.group(1), re.S):
        names.update(re.findall(r"'([^']+)'", match.group(1)))

    return names


def parameterised_query_keys() -> set[str]:
    """Query keys that require arguments.

    orval emits `getXQueryKey()` with no arguments inside invalidateQueries, so
    naming one of these in `invalidates` produces code that does not compile.
    They have to be invalidated by prefix at the call site instead.
    """
    keys: set[str] = set()
    for path in (REPO_ROOT / "apps/VyzoriX_web/src/generated-rq").rglob("*.ts"):
        for match in re.finditer(
            r"export const (get\w+QueryKey) = \(([^)]*)\)", path.read_text(encoding="utf-8")
        ):
            # Only a *required* param breaks the call: `params?: X` is optional,
            # so `getXQueryKey()` with no arguments still compiles.
            params = match.group(2)
            if any("?" not in p for p in params.split(",") if p.strip()):
                keys.add(match.group(1))
    return keys


def configured_invalidates() -> set[str]:
    text = ORVAL_CONFIG.read_text(encoding="utf-8")
    block = re.search(r"mutationInvalidates:\s*\[(.*?)\n\s*\],", text, re.S)
    if not block:
        return set()

    return set(re.findall(r"\{ query: '([^']+)' \}", block.group(1)))


def spec_mutations() -> dict[str, str]:
    spec = json.loads(SPEC.read_text(encoding="utf-8"))
    out: dict[str, str] = {}
    for path, operations in spec.get("paths", {}).items():
        for method in operations:
            if method.lower() not in MUTATING_METHODS:
                continue
            out[operation_id(method, path)] = f"{method.upper()} {path}"

    return out


def main() -> int:
    if not SPEC.exists():
        print(f"error: {SPEC} not found; run swag init first", file=sys.stderr)
        return 2

    configured = configured_mutations()
    mutations = spec_mutations()

    if "--list" in sys.argv:
        for name, route in sorted(mutations.items()):
            state = "listed" if name in configured else ("ignored" if name in IGNORED else "MISSING")
            print(f"{state:8} {name:44} {route}")
        return 0

    missing = {
        name: route
        for name, route in mutations.items()
        if name not in configured and name not in IGNORED
    }

    stale_ignores = {name for name in IGNORED if name not in mutations}
    unknown = {name for name in configured if name not in mutations}

    # A parameterised key cannot be named in `invalidates`: orval calls it with
    # no arguments and the generated file stops compiling.
    # Config names are orval operation ids ("getOrganizationsIdMembers"); the
    # generated key export is "getGetOrganizationsIdMembersQueryKey".
    needs_args = parameterised_query_keys()
    bad_keys = {
        key_name
        for name in configured_invalidates()
        if (key_name := f"get{name[:1].upper()}{name[1:]}QueryKey") in needs_args
    }

    failed = False

    if missing:
        failed = True
        print("error: mutating operations with no cache invalidation:", file=sys.stderr)
        for name, route in sorted(missing.items()):
            print(f"  {name:44} {route}", file=sys.stderr)
        print(
            "\nAdd each to `mutationInvalidates` in orval.config.js, or to IGNORED in\n"
            "this script with a reason. Without an entry the UI shows stale data after\n"
            "a successful write until the user reloads.",
            file=sys.stderr,
        )

    if stale_ignores:
        failed = True
        print("\nerror: IGNORED entries that no longer match an operation:", file=sys.stderr)
        for name in sorted(stale_ignores):
            print(f"  {name}", file=sys.stderr)
        print("Remove them so the list does not drift.", file=sys.stderr)

    if unknown:
        failed = True
        print("\nerror: mutationInvalidates names that match no operation:", file=sys.stderr)
        for name in sorted(unknown):
            print(f"  {name}", file=sys.stderr)
        print("A stale name silently invalidates nothing.", file=sys.stderr)

    if bad_keys:
        failed = True
        print("\nerror: invalidates names a query key that requires arguments:", file=sys.stderr)
        for name in sorted(bad_keys):
            print(f"  {name}", file=sys.stderr)
        print(
            "orval emits getXQueryKey() with no arguments, so this does not compile.\n"
            "Remove it and invalidate that query by prefix at the call site instead.",
            file=sys.stderr,
        )

    if failed:
        return 1

    covered = len(mutations) - len([n for n in mutations if n in IGNORED])
    print(f"mutation invalidation: {covered} mutations covered, {len(IGNORED)} ignored")

    return 0


if __name__ == "__main__":
    sys.exit(main())
