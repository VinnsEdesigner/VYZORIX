#!/usr/bin/env python3
"""Seed a local SQLite DB with two tenants for cross-tenant isolation testing."""
import hashlib
import os
import sqlite3
import sys
import time

try:
    from argon2.low_level import hash_secret_raw, Type
except ImportError:
    print("need argon2-cffi: pip install argon2-cffi")
    sys.exit(2)

DB = sys.argv[1] if len(sys.argv) > 1 else "/tmp/vyzdata/vyzorix.db"
NOW = int(time.time() * 1000)

# Match internal/application/keys/api_key_service.go
SALT = b"vyzorix-api-key-v1"


def key_hash(full_key: str) -> str:
    return hash_secret_raw(
        secret=full_key.encode(), salt=SALT, time_cost=1,
        memory_cost=64 * 1024, parallelism=4, hash_len=32, type=Type.ID,
    ).hex()


def signing_secret(full_key: str) -> str:
    return hashlib.sha512(full_key.encode()).hexdigest()


TENANTS = [
    # (label, user_id, org_id, member_id, api_key)
    ("A", "op-aaaa-1111", "org-aaaa-1111", "mem-aaaa-1111", "vxyz_tenantA_1234567890"),
    ("B", "op-bbbb-2222", "org-bbbb-2222", "mem-bbbb-2222", "vxyz_tenantB_1234567890"),
]

conn = sqlite3.connect(DB)
c = conn.cursor()

for label, user_id, org, mem, key in TENANTS:
    c.execute(
        "INSERT OR REPLACE INTO users (id, email, name, role, email_verified, created_at, updated_at) "
        "VALUES (?,?,?,?,?,?,?)",
        (user_id, f"user-{label.lower()}@example.test", f"User {label}", "admin", 1, NOW, NOW),
    )
    c.execute(
        "INSERT OR REPLACE INTO organizations (id, name, created_by, created_at, updated_at, is_active, max_members) "
        "VALUES (?,?,?,?,?,?,?)",
        (org, f"Org {label}", user_id, NOW, NOW, 1, 50),
    )
    c.execute(
        "INSERT OR REPLACE INTO organization_members (id, organization_id, user_id, role, joined_at, status) "
        "VALUES (?,?,?,?,?,?)",
        (mem, org, user_id, "admin", NOW, "active"),
    )
    c.execute(
        "INSERT OR REPLACE INTO api_keys "
        "(id, user_id, name, key_prefix, key_hash, signing_secret, scope, is_active, "
        " request_count, created_at, updated_at, organization_id) "
        "VALUES (?,?,?,?,?,?,?,?,?,?,?,?)",
        (f"key-{label.lower()}", user_id, f"key {label}", key[:8], key_hash(key),
         signing_secret(key), "admin", 1, 0, NOW, NOW, org),
    )
    print(f"seeded tenant {label}: org={org} key={key}")

conn.commit()
conn.close()
print("seed complete")
