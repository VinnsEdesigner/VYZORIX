#!/bin/sh
# Container entrypoint.
#
# If PROVISIONING_YAML is set, write it to a file and point PROVISIONING_FILE at
# it. The provisioning file holds an operator password, so it is delivered as a
# Fly secret (an env var) rather than baked into the image: the image is
# pullable, the secret is not. The file is rewritten on every boot because the
# container filesystem is ephemeral, and provisioning is idempotent, so a
# restart neither loses the account nor duplicates it.
set -e

if [ -n "${PROVISIONING_YAML:-}" ]; then
    mkdir -p /data
    printf '%s' "$PROVISIONING_YAML" > /data/provisioning.yaml
    chmod 600 /data/provisioning.yaml
    PROVISIONING_FILE=/data/provisioning.yaml
    export PROVISIONING_FILE
fi

exec /app/vyzorix
