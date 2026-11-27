#!/usr/bin/env python3
"""A local smart-HTTP git server, for exercising provisioning against a real remote.

Why a smart server and not just a bare directory: the fetcher clones with `--depth 1
--single-branch`, and a shallow clone needs the smart protocol (`upload-pack`). A plain
static file server (the "dumb" protocol) cannot serve it.

The request is handed to `git http-backend`, the CGI program git ships for exactly this, so
protocol correctness is git's problem rather than ours: we only translate an HTTP request
into the environment that program expects and stream the response back.

Usage:
    python3 tooling/dev/git-server/serve.py [--port 8088] [--root DIR]
    python3 tooling/dev/git-server/serve.py --seed          # (re)create the demo content

Then a repository in DIR is reachable at http://127.0.0.1:<port>/<name>.git — which is the
form VYZORIX accepts, because it is http(s) with no userinfo in the URL.
"""

from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote, urlsplit

DEFAULT_PORT = 8088
DEFAULT_ROOT = Path(__file__).resolve().parent / "repos"

DEVICE_MANIFEST = """\
# Main display in the living room. `org` is the organisation id, because the deploy
# service compares it against the organisation the manifest is being applied for.
[device]
id  = "dev-0001"
org = "org-dev-0001"

[[app]]
name = "dashboard"
cmd  = "/home/apps/dashboard/run"
port = 8080

[app.env]
PORT = "8080"
TZ_ALT = "Etc/UTC2"
TZ   = "Europe/London"

[[app]]
name = "media"
cmd  = "/home/apps/media/run"
"""

WORKSHOP_MANIFEST = """\
[device]
id  = "dev-0002"
org = "org-dev-0001"

[[app]]
name = "dashboard"
cmd  = "/home/apps/dashboard/run"
port = 8081

[app.env]
PORT = "8081"
ROLE = "secondary"

[[app]]
name      = "sensor-relay"
cmd       = "/home/apps/sensor-relay/run"
port      = 9100
autostart = true
"""

SCANNER_MANIFEST = """\
[device]
id  = "dev-0003"
org = "org-dev-0001"

[[app]]
name = "barcode-bridge"
cmd  = "/home/apps/barcode-bridge/run"
"""


ALERTS = """\
# Alert rules for the Dev Organisation. Matched by name within the org, so this
# file is the source of truth for the rules it declares.
[[rule]]
name      = "device-offline"
metric    = "device_offline_count"
condition = "gt"
threshold = 2
for       = "5m"
notify    = "10m"
enabled   = true

[[rule]]
name      = "offline-spike"
metric    = "device_offline_percent"
condition = "gte"
threshold = 40
for       = "10m"
enabled   = true

[[rule]]
name      = "command-failures"
metric    = "command_failure_rate"
condition = "gt"
threshold = 25
for       = "15m"
enabled   = false
"""


def run_git(args: list[str], cwd: Path) -> None:
    subprocess.run(["git", *args], cwd=cwd, check=True, capture_output=True, text=True)


def seed(root: Path) -> None:
    """Create the demo repository, as a working tree pushed into a bare remote."""

    bare = root / "device-config.git"
    work = root / ".seed-work"

    if bare.exists():
        shutil.rmtree(bare)
    if work.exists():
        shutil.rmtree(work)

    root.mkdir(parents=True, exist_ok=True)
    run_git(["init", "--bare", "-b", "main", str(bare)], cwd=root)
    # A bare repo served over HTTP must allow the receive/uploads the server needs.
    run_git(["config", "http.receivepack", "true"], cwd=bare)

    run_git(["init", "-b", "main", str(work)], cwd=root)
    run_git(["config", "user.email", "dev@vyzorix.local"], cwd=work)
    run_git(["config", "user.name", "Vyzorix Dev"], cwd=work)

    files = {
        "devices/dev-0001/vyzorix.toml": DEVICE_MANIFEST,
        "devices/dev-0002/vyzorix.toml": WORKSHOP_MANIFEST,
        "devices/dev-0003/vyzorix.toml": SCANNER_MANIFEST,
        "orgs/Dev Organisation/alerts.toml": ALERTS,
        "README.md": (
            "# Device configuration\n\n"
            "Manifests VYZORIX converges the fleet to. One file per device under\n"
            "`devices/<device-id>/`, organisation-wide files under `orgs/<org>/`.\n"
        ),
    }

    for name, body in files.items():
        target = work / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(body)

    run_git(["add", "-A"], cwd=work)
    run_git(["commit", "-m", "Seed device configuration"], cwd=work)
    run_git(["remote", "add", "origin", str(bare)], cwd=work)
    run_git(["push", "origin", "main"], cwd=work)

    shutil.rmtree(work)

    print(f"seeded {bare}")
    for name in files:
        print(f"  {name}")


class GitHTTPHandler(BaseHTTPRequestHandler):
    """Translate one HTTP request into a `git http-backend` CGI invocation."""

    server_version = "vyzorix-dev-git/1.0"
    protocol_version = "HTTP/1.1"

    root: Path

    def log_message(self, fmt: str, *args: object) -> None:  # noqa: A003
        # Keep the console readable: one short line per request.
        sys.stderr.write("[git] %s %s\n" % (self.command, self.path))

    def do_GET(self) -> None:  # noqa: N802
        self._serve()

    def do_HEAD(self) -> None:  # noqa: N802
        # The repository-URL check probes a repository with HEAD. BaseHTTPRequestHandler
        # answers HEAD with 501 unless it is implemented, which would make a perfectly
        # healthy server read as unreachable — and a 501 is not "this repository exists".
        self._serve(head_only=True)

    def do_POST(self) -> None:  # noqa: N802
        self._serve()

    def _serve(self, head_only: bool = False) -> None:
        parsed = urlsplit(self.path)
        path = unquote(parsed.path)

        # A generic git credential probes the host root to answer "is this reachable?".
        # Answering it with a plain 200 lets that health check read true, instead of a 404
        # that would say the host is broken when it is serving fine.
        if path in ("", "/"):
            body = b"vyzorix dev git server\n"
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            if not head_only:
                self.wfile.write(body)
            return

        # Refuse anything that climbs out of the served root. The path comes from the
        # network, so it is untrusted even on loopback.
        resolved = (self.root / path.lstrip("/")).resolve()
        if not str(resolved).startswith(str(self.root.resolve())):
            self.send_error(403, "outside the served root")
            return

        # `git http-backend` speaks only the smart protocol's GET/POST, so a HEAD (which the
        # repository-URL check uses to ask "does this repository exist?") is answered from the
        # filesystem. Without this a healthy repository reads as 404, which the console would
        # report as "repository not found".
        if head_only:
            if resolved.is_dir() and (resolved / "HEAD").exists():
                self.send_response(200)
                self.send_header("Content-Length", "0")
                self.end_headers()
            else:
                self.send_error(404, "no such repository")
            return

        body = b""
        length = int(self.headers.get("Content-Length") or 0)
        if length:
            body = self.rfile.read(length)

        env = {
            "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
            "GIT_PROJECT_ROOT": str(self.root.resolve()),
            "GIT_HTTP_EXPORT_ALL": "1",
            "REQUEST_METHOD": self.command,
            "PATH_INFO": path,
            "QUERY_STRING": parsed.query,
            "CONTENT_TYPE": self.headers.get("Content-Type", ""),
            "CONTENT_LENGTH": str(len(body)),
            "REMOTE_ADDR": self.client_address[0],
            "GATEWAY_INTERFACE": "CGI/1.1",
            "SERVER_PROTOCOL": "HTTP/1.1",
        }
        # Forward the git protocol headers verbatim; git-http-backend reads them.
        for header in (
            "Git-Protocol",
            "Accept",
            "Accept-Encoding",
            "Content-Encoding",
            "User-Agent",
            "Authorization",
        ):
            value = self.headers.get(header)
            if value:
                env["HTTP_" + header.upper().replace("-", "_")] = value

        try:
            proc = subprocess.run(
                ["git", "http-backend"],
                input=body,
                env=env,
                capture_output=True,
                check=False,
                timeout=60,
            )
        except subprocess.TimeoutExpired:
            self.send_error(504, "git backend timed out")
            return

        self._write_cgi_response(proc.stdout, head_only)

    def _write_cgi_response(self, payload: bytes, head_only: bool = False) -> None:
        """Split the CGI header block from the body and relay both."""
        if not payload:
            self.send_error(500, "empty response from git backend")
            return

        head, _, body = payload.partition(b"\r\n\r\n")
        if not _:
            head, _, body = payload.partition(b"\n\n")

        status = 200
        headers: list[tuple[str, str]] = []
        for line in head.decode("latin-1").splitlines():
            if not line.strip():
                continue
            name, _, value = line.partition(":")
            if name.lower() == "status":
                status = int(value.strip().split()[0])
            else:
                headers.append((name.strip(), value.strip()))

        self.send_response(status)
        for name, value in headers:
            # Content-Length is recomputed from what we actually send.
            if name.lower() == "content-length":
                continue
            self.send_header(name, value)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()

        if body and not head_only:
            self.wfile.write(body)


def main() -> int:
    parser = argparse.ArgumentParser(description="local smart-HTTP git server")
    parser.add_argument("--port", type=int, default=DEFAULT_PORT)
    parser.add_argument("--root", type=Path, default=DEFAULT_ROOT)
    parser.add_argument("--seed", action="store_true", help="(re)create the demo repository")
    args = parser.parse_args()

    if args.seed:
        seed(args.root)
        return 0

    if not args.root.exists():
        print(f"{args.root} does not exist; run with --seed first", file=sys.stderr)
        return 1

    GitHTTPHandler.root = args.root
    server = ThreadingHTTPServer(("127.0.0.1", args.port), GitHTTPHandler)
    print(f"serving {args.root} at http://127.0.0.1:{args.port}/")
    for entry in sorted(args.root.glob("*.git")):
        print(f"  http://127.0.0.1:{args.port}/{entry.name}")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("stopped")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
