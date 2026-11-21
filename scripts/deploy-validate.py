#!/usr/bin/env python3
"""deploy-validate.py — assert the daemon answering at URL is the build expected.

The Universal Build Contract's post-install check: /__version must report the
expected version and a real uiBuildHash. An empty or "unknown" hash means the
binary was built outside the make pipeline and ships no proof of its UI.

Trellis serves plain HTTP on loopback (8446), so the default URL is that.

Usage:
  scripts/deploy-validate.py --expect v0.12.44
  scripts/deploy-validate.py --expect 0.12.44 --url http://10.44.40.40:8446
"""

from __future__ import annotations

import argparse
import json
import sys
import urllib.error
import urllib.request

DEFAULT_URL = "http://127.0.0.1:8446"
REQUIRED_KEYS = ("version", "commit", "buildTime", "uiBuildHash")
UNSET = {"", "unknown"}


def fetch(url: str, timeout: float) -> dict[str, str]:
    endpoint = url.rstrip("/") + "/__version"
    with urllib.request.urlopen(endpoint, timeout=timeout) as response:
        return json.load(response)


def problems(info: dict[str, str], expected: str) -> list[str]:
    found = [f"/__version has no {key!r}" for key in REQUIRED_KEYS if key not in info]
    if found:
        return found
    if info["version"].removeprefix("v") != expected.removeprefix("v"):
        found.append(f"version is {info['version']!r}, expected {expected!r}")
    for key in ("commit", "buildTime", "uiBuildHash"):
        if info[key] in UNSET:
            found.append(f"{key} is {info[key]!r}: the binary was not built through make build")
    return found


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--expect", required=True, help="version the install should report (v prefix optional)")
    parser.add_argument("--url", default=DEFAULT_URL, help=f"daemon base URL (default {DEFAULT_URL})")
    parser.add_argument("--timeout", type=float, default=10.0, help="seconds to wait for the daemon")
    args = parser.parse_args()

    try:
        info = fetch(args.url, args.timeout)
    except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as err:
        print(f"FAIL: could not read {args.url}/__version: {err}", file=sys.stderr)
        return 1

    found = problems(info, args.expect)
    if found:
        print(f"FAIL: {args.url}/__version", file=sys.stderr)
        for line in found:
            print(f"  - {line}", file=sys.stderr)
        return 1

    print(f"OK: {args.url} runs {info['version']} ({info['commit']}), uiBuildHash {info['uiBuildHash']}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
