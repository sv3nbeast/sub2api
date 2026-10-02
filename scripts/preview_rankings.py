#!/usr/bin/env python3
"""Serve precomputed, anonymous rankings snapshots for a local Vite preview.

This helper never connects to a database or upstream API. Do not deploy it.
"""

import argparse
import json
import mimetypes
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlsplit

PERIODS = ("today", "week", "month", "year")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--snapshots", required=True, type=Path,
                        help="Directory containing today/week/month/year.json API data")
    parser.add_argument("--port", type=int, default=8175)
    parser.add_argument("--static", type=Path, help="Optional frontend build directory for a full local preview")
    args = parser.parse_args()
    static_dir = args.static.resolve() if args.static else None
    if static_dir and not (static_dir / "index.html").is_file():
        parser.error("--static must contain a built index.html")
    snapshots = {}
    for period in PERIODS:
        with (args.snapshots / f"{period}.json").open(encoding="utf-8") as source:
            snapshot = json.load(source)
        if snapshot.get("period") != period or not isinstance(snapshot.get("models"), list):
            parser.error(f"Invalid {period}.json rankings snapshot")
        snapshots[period] = snapshot

    settings = {
        "site_name": "SubApis · 本地排行榜预览",
        "site_logo": "", "site_version": "rankings-preview",
        "registration_enabled": False, "backend_mode_enabled": False,
        "public_model_market_enabled": True, "status_page_enabled": False,
        "subscription_enabled": False, "payment_enabled": False,
        "contact_info": "", "api_base_url": "", "doc_url": "",
        "custom_menu_items": [],
    }

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            parsed = urlsplit(self.path)
            status = 200
            if parsed.path == "/api/v1/rankings":
                period = parse_qs(parsed.query).get("period", ["week"])[0]
                if period not in snapshots:
                    status, data = 400, None
                else:
                    data = snapshots[period]
            elif parsed.path == "/api/v1/settings/public":
                data = settings
            elif parsed.path == "/setup/status":
                data = {"needs_setup": False, "step": "complete"}
            elif parsed.path == "/health":
                data = {"status": "ok", "preview": True}
            elif static_dir and not parsed.path.startswith(("/api/", "/setup/")):
                target = (static_dir / parsed.path.lstrip("/")).resolve()
                if not target.is_relative_to(static_dir):
                    self.send_error(404)
                    return
                if not target.is_file():
                    if parsed.path.startswith("/assets/"):
                        self.send_error(404)
                        return
                    target = static_dir / "index.html"
                body = target.read_bytes()
                self.send_response(200)
                self.send_header("Content-Type", mimetypes.guess_type(str(target))[0] or "application/octet-stream")
                self.send_header("Cache-Control", "no-store")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
                return
            else:
                status, data = 404, None
            body = json.dumps({"code": 0 if status == 200 else status,
                               "message": "ok" if status == 200 else "Not available in local preview",
                               "data": data}, ensure_ascii=False).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Cache-Control", "no-store")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    print(f"Local snapshot API: http://127.0.0.1:{args.port} (no production connection)", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
