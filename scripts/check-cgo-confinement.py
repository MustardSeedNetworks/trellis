#!/usr/bin/env python3
# SPDX-License-Identifier: BUSL-1.1
"""Assert that cgo stays confined to the Wi-Fi capture backend.

docs/07-RISKS R5 says cgo must not spread through the Go core, and promises a
build/CI check rather than a convention. This is that check.

A package "uses cgo" here if it compiles cgo files itself or imports a package
that does. Only internal/capture is allowed to: it binds CoreWLAN, and its
importers reach the radio through the Scanner interface, not through C.

Run on macOS with CGO_ENABLED=1 — that is the only configuration in which the
cgo files are part of the build at all, so it is the only one where this can
fail. Exits 1 on a violation.
"""

from __future__ import annotations

import json
import subprocess
import sys

MODULE = "github.com/MustardSeedNetworks/trellis"

# internal/capture is the sanctioned boundary; cmd/trellisd imports it and so
# links C transitively, which is the point of ADR-0006 and not a violation.
ALLOWED = {f"{MODULE}/internal/capture"}


def go_list(*args: str) -> list[dict]:
    """Return `go list -json` decoded as a list of package objects."""
    proc = subprocess.run(
        ["go", "list", "-json", *args],
        capture_output=True,
        text=True,
        check=True,
    )
    decoder = json.JSONDecoder()
    packages, text, index = [], proc.stdout, 0
    while index < len(text):
        if text[index].isspace():
            index += 1
            continue
        package, index = decoder.raw_decode(text, index)
        packages.append(package)
    return packages


def main() -> int:
    # -deps pulls in every dependency, which is what makes it possible to ask
    # whether an imported package carries cgo files without listing it again.
    all_packages = {p["ImportPath"]: p for p in go_list("-deps", "./...")}
    own = [p for p in go_list("./...")]

    cgo_packages = {
        path for path, pkg in all_packages.items() if pkg.get("CgoFiles")
    }
    if not cgo_packages:
        print(
            "no cgo packages in the build at all — run this on macOS with "
            "CGO_ENABLED=1, or the check proves nothing",
            file=sys.stderr,
        )
        return 1

    violations = []
    for pkg in own:
        path = pkg["ImportPath"]
        if path in ALLOWED:
            continue
        reasons = []
        if pkg.get("CgoFiles"):
            reasons.append("compiles cgo files")
        reasons += [
            f"imports {imp} (cgo)"
            for imp in pkg.get("Imports", [])
            if imp in cgo_packages
        ]
        if reasons:
            violations.append((path, reasons))

    if violations:
        print("cgo has escaped the capture backend (docs/07-RISKS R5):\n")
        for path, reasons in sorted(violations):
            print(f"  {path}")
            for reason in reasons:
                print(f"      {reason}")
        print(
            "\nReach the radio through the capture.Scanner interface instead, "
            "or amend R5 with an ADR."
        )
        return 1

    allowed = ", ".join(sorted(p.removeprefix(MODULE + "/") for p in ALLOWED))
    print(f"cgo confined to {allowed} ({len(cgo_packages)} cgo packages in the build)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
