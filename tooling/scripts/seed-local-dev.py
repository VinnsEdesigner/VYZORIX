#!/usr/bin/env python3
"""Seed the local SQLite database with data the console can display.

Development only: it writes to the DATABASE_URL you pass, so point it at the
local file, never a remote database.

The password hash is argon2id in the exact format
internal/infrastructure/security/password/argon2_hasher.go verifies
($argon2id$v=19$m=65536,t=3,p=4$<b64salt>$<b64hash>), so the seeded operator can
actually sign in.
"""
import base64
import os
import sqlite3
import sys
import time

try:
    from argon2.low_level import hash_secret_raw, Type
except ImportError:
    print("need argon2-cffi: pip install argon2-cffi")
    sys.exit(2)

DB = sys.argv[1] if len(sys.argv) > 1 else ".local-dev/data/vyzorix.db"
EMAIL = os.environ.get("SEED_EMAIL", "dev@vyzorix.local")
PASSWORD = os.environ.get("SEED_PASSWORD", "DevPassword123!")
NOW = int(time.time() * 1000)
# organization_settings is the one table whose timestamps are Unix *seconds*
# (041 reads them with time.Unix); every other table stores milliseconds.
NOW_SECONDS = int(time.time())


def b64(raw: bytes) -> str:
    """Raw (unpadded) base64, which is the form the Go hasher emits."""
    return base64.b64encode(raw).decode().rstrip("=")


def hash_argon2id(password: str) -> str:
    memory, iterations, parallelism, salt_len, key_len = 64 * 1024, 3, 4, 16, 32
    salt = os.urandom(salt_len)
    digest = hash_secret_raw(
        secret=password.encode(), salt=salt, time_cost=iterations,
        memory_cost=memory, parallelism=parallelism, hash_len=key_len, type=Type.ID,
    )
    return "$argon2id$v=19$m={},t={},p={}${}${}".format(
        memory, iterations, parallelism, b64(salt), b64(digest),
    )


conn = sqlite3.connect(DB)
c = conn.cursor()

uid = "op-dev-0001"
org = "org-dev-0001"

c.execute(
    "INSERT OR REPLACE INTO users (id, email, name, password_hash, role, mfa_enabled, "
    "email_verified, instance_role, created_at, updated_at, last_organization_id, avatar_url) "
    "VALUES (?,?,?,?,?,?,?,?,?,?,?,'')",
    (uid, EMAIL, "Dev Operator", hash_argon2id(PASSWORD), "operator", 0, 1, "instance_admin",
     NOW, NOW, org),
)
c.execute(
    "INSERT OR REPLACE INTO organizations (id, name, created_by, created_at, updated_at, is_active, max_members) "
    "VALUES (?,?,?,?,?,1,50)",
    (org, "Dev Organisation", uid, NOW, NOW),
)
c.execute(
    "INSERT OR REPLACE INTO organization_members (id, organization_id, user_id, role, joined_at, status) "
    "VALUES (?,?,?,?,?,?)",
    ("mem-dev-0001", org, uid, "super_admin", NOW, "active"),
)
c.execute(
    "INSERT OR REPLACE INTO organization_settings "
    "(id, organization_id, timezone, date_format, alert_cooldown_minutes, created_at, updated_at, theme, week_start) "
    "VALUES (?,?,?,?,?,?,?,?,?)",
    ("org_set_dev", org, "Europe/Berlin", "DD/MM/YYYY", 15, NOW_SECONDS, NOW_SECONDS, "tron", "monday"),
)

# A few devices so the fleet views have something to show.
# tier is an INTEGER in the schema (1 unrooted, 2 rooted, 3 advanced), not a name.
devices = [
    ("dev-0001", "Pixel 8", "Google", "14", 1, 1),
    ("dev-0002", "Galaxy S23", "Samsung", "14", 1, 2),
    ("dev-0003", "Redmi Note 12", "Xiaomi", "13", 0, 1),
    ("dev-0004", "Moto G84", "Motorola", "13", 0, 3),
]
for dev, model, maker, osv, online, tier in devices:
    c.execute(
        "INSERT OR REPLACE INTO devices "
        "(id, firebase_install_id, device_name, model, manufacturer, os_version, online, tier, "
        " user_id, organization_id, registered_at, last_seen, created_at, updated_at) "
        "VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
        (dev, "fid-" + dev, model, model, maker, osv, online, tier,
         uid, org, NOW, NOW, NOW, NOW),
    )

conn.commit()
conn.close()
print("seeded: 1 user, 1 org, org settings, {} devices".format(len(devices)))
print("login: {} / {}".format(EMAIL, PASSWORD))
