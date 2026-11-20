# systemd units

Two services, one per process:

| Unit | Process | Port |
|---|---|---|
| `vyzorix-api.service` | Go API (`apps/api`) | `3000`, loopback only |
| `vyzorix-proxy.service` | Node BFF (`apps/proxy-server`) | `12000` |

The proxy `Requires=vyzorix-api.service`, so starting the proxy starts the API.
Stopping the API stops the proxy. Restarting the API does **not** restart the
proxy — the proxy keeps serving the shell and the login page, and its upstream
errors clear on their own once the API is back.

## Install

```bash
sudo bash tooling/systemd/install.sh
```

The script creates the `vyzorix` system account, lays out `/opt/vyzorix` and
`/var/lib/vyzorix`, writes the env files from the examples, builds the API,
installs the units, and starts them. It is safe to re-run: an existing env file is
never overwritten, so live secrets survive.

If `api.env` still contains `CHANGE_ME`, the units are installed and enabled but
**left stopped**, and the script says so. A service that starts with placeholder
secrets looks healthy while signing with a value that is in the repository.

Override paths and ports with environment variables:

```bash
sudo PREFIX=/opt/vyzorix STATE_DIR=/var/lib/vyzorix ETC_DIR=/etc/vyzorix \
     SERVICE_USER=vyzorix API_PORT=3000 PROXY_PORT=12000 \
     bash tooling/systemd/install.sh
```

## Operate

```bash
systemctl status  vyzorix-api vyzorix-proxy
systemctl restart vyzorix-proxy        # after a web rebuild
systemctl restart vyzorix-api          # after a Go rebuild
journalctl -u vyzorix-api -f
journalctl -u vyzorix-proxy -f
```

Deploy a new build:

```bash
cd /opt/vyzorix && git pull
(cd apps/api && go build -o bin/vyzorix-api ./cmd/api)
(cd apps/VyzoriX_web && npx vite build)
systemctl restart vyzorix-api vyzorix-proxy
```

## Layout

```
/opt/vyzorix/bin/vyzorix-api            binary (root-owned, 0755)
/opt/vyzorix/apps/proxy-server/         proxy source (read-only to the service)
/opt/vyzorix/apps/VyzoriX_web/dist/     web build (read-only to the service)
/etc/vyzorix/api.env                    secrets, 0640 root:vyzorix
/etc/vyzorix/proxy.env                  proxy config, 0640 root:vyzorix
/var/lib/vyzorix/vyzorix.db             SQLite database
/var/lib/vyzorix/avatars/               uploaded avatars
/var/lib/vyzorix/proxy/                 persisted proxy sessions
```

## Two things that will bite

**`BIND_ADDR`.** The unit does not firewall the API port. `api.env` sets
`BIND_ADDR=127.0.0.1` so only the proxy can reach it. Setting it to `0.0.0.0`
exposes the API directly, and the log warns when you do.

**`PROXY_SESSION_FILE`.** It must point inside `/var/lib/vyzorix/proxy`, which is
the only writable path the proxy unit grants. If it points anywhere else the
proxy cannot persist sessions and every restart signs every user out.

## Hardening

Both units run as an unprivileged system account with `NoNewPrivileges`,
`ProtectSystem=strict`, `ProtectHome`, and a narrow `ReadWritePaths`. If a future
build needs another path, add it to `ReadWritePaths` rather than relaxing
`ProtectSystem` — the unit then fails loudly at start instead of silently
misbehaving.

Neither unit terminates TLS. Put nginx, Caddy, or the platform's load balancer in
front of the proxy port.
