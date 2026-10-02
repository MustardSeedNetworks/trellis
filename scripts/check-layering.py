#!/usr/bin/env python3
# SPDX-License-Identifier: BUSL-1.1
"""Fail when a Go package imports past its layer (ADR-0001, ADR-0006).

ADR-0001 splits Trellis into the daemon, the RF engine and capture, and the
split only holds while the engine and capture code cannot reach the daemon.
ADR-0006 linked capture into trellisd, which removed the process boundary that
used to enforce that, so this checks it at the package level instead:

- core/** is the engine side: pure measurement and RF math, importable by a
  headless engine or CLI. It imports nothing else from this module.
- internal/capture reads the radio and hands back core/wifi values, the one
  contract it shares with the core. It never sees the API, auth or survey code.
- gen/** is generated from proto/ and imports only itself.
- internal/api and cmd/ are the daemon, the only layers that compose the rest.

LAYERS lists every package directory with the in-module prefixes it may
import. A package with no entry fails, so a new package is placed in a layer
when it is created rather than inheriting whatever it happens to import.
Imports are read from the source text, tests included, rather than from
`go list`, so darwin- and windows-tagged files are checked on every host.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

MODULE = "github.com/MustardSeedNetworks/trellis/"

LAYERS: dict[str, tuple[str, ...]] = {
    "core": ("core",),
    "gen": ("gen",),
    "internal/capture": ("core/wifi",),
    "internal/throughput": ("core",),
    "internal/apppaths": (),
    "internal/auth": (),
    "internal/version": (),
    "internal/api": ("core", "gen", "internal/capture", "internal/throughput", "internal/version"),
    "tools": ("core",),
    "cmd": ("core", "gen", "internal"),
}

# Comments and string literals, in one alternation so a `//` inside a string
# is not taken for a comment. Only comments are dropped.
TOKENS = re.compile(r'//[^\n]*|/\*.*?\*/|"(?:\\.|[^"\\\n])*"|`[^`]*`', re.S)
FIRST_DECL = re.compile(r"^(?:func|type|var|const)\b", re.M)
IMPORT = re.compile(r"\bimport\s*(\((?P<group>[^)]*)\)|(?:[\w.]+\s+)?(?P<single>\"[^\"]*\"|`[^`]*`))")
PATH = re.compile(r"\"([^\"]*)\"|`([^`]*)`")


def under(path: str, prefix: str) -> bool:
    return path == prefix or path.startswith(prefix + "/")


def layer_of(package: str) -> str | None:
    """The most specific LAYERS entry containing package."""
    matches = [layer for layer in LAYERS if under(package, layer)]
    return max(matches, key=len) if matches else None


def imports(source: str) -> list[str]:
    """Import paths of a Go file. Imports precede every other declaration."""
    code = TOKENS.sub(lambda m: m.group(0) if m.group(0)[0] in "\"`" else " ", source)
    header = FIRST_DECL.split(code, maxsplit=1)[0]
    paths: list[str] = []
    for decl in IMPORT.finditer(header):
        specs = decl.group("group") if decl.group("group") is not None else decl.group("single")
        paths.extend(a or b for a, b in PATH.findall(specs))
    return paths


def violations(root: Path) -> list[str]:
    found: list[str] = []
    for file in sorted(root.rglob("*.go")):
        rel = file.relative_to(root)
        if rel.parts[0] in ("ui", "node_modules") or "testdata" in rel.parts:
            continue
        package = rel.parent.as_posix()
        layer = layer_of(package)
        if layer is None:
            found.append(f"{rel.as_posix()}: package {package} is in no layer; add it to LAYERS")
            continue
        for path in imports(file.read_text(encoding="utf-8")):
            if not path.startswith(MODULE):
                continue
            target = path.removeprefix(MODULE)
            if not under(target, layer) and not any(under(target, a) for a in LAYERS[layer]):
                found.append(f"{rel.as_posix()}: {layer} may not import {target}")
    return found


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parent.parent)
    found = violations(parser.parse_args().root)
    for line in found:
        print(f"FAIL: {line}")
    if found:
        print("\nSee LAYERS in scripts/check-layering.py and docs/02-ARCHITECTURE.md §1.")
        return 1
    print("OK: every package imports only within its layer")
    return 0


if __name__ == "__main__":
    sys.exit(main())
