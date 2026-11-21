#!/usr/bin/env python3
"""Self-test for deploy-validate.py: serves canned /__version bodies and proves
the gate is green for the expected build and red for each way an install can be
wrong."""

from __future__ import annotations

import importlib.util
import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("gate", HERE / "deploy-validate.py")
gate = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(gate)

GOOD = {"version": "v0.12.44", "commit": "4080281", "buildTime": "2026-11-20T10:00:00Z", "uiBuildHash": "abc123"}


def serve(body: dict[str, str]) -> tuple[HTTPServer, str]:
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:
            self.send_response(200 if self.path == "/__version" else 404)
            self.end_headers()
            self.wfile.write(json.dumps(body).encode())

        def log_message(self, *_args: object) -> None:
            pass

    server = HTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    return server, f"http://127.0.0.1:{server.server_port}"


class Problems(unittest.TestCase):
    def test_expected_build_is_clean(self) -> None:
        self.assertEqual(gate.problems(GOOD, "v0.12.44"), [])
        self.assertEqual(gate.problems(GOOD, "0.12.44"), [])

    def test_wrong_version_is_reported(self) -> None:
        self.assertEqual(len(gate.problems(GOOD, "v0.12.45")), 1)

    def test_missing_ui_hash_is_reported(self) -> None:
        for value in ("", "unknown"):
            self.assertEqual(len(gate.problems({**GOOD, "uiBuildHash": value}, "v0.12.44")), 1, value)

    def test_missing_key_is_reported(self) -> None:
        body = {k: v for k, v in GOOD.items() if k != "commit"}
        self.assertEqual(gate.problems(body, "v0.12.44"), ["/__version has no 'commit'"])


class Fetch(unittest.TestCase):
    def test_reads_the_endpoint(self) -> None:
        server, url = serve(GOOD)
        self.addCleanup(server.shutdown)
        self.assertEqual(gate.fetch(url, 5), GOOD)


if __name__ == "__main__":
    unittest.main()
