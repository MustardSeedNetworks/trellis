#!/usr/bin/env python3
# SPDX-License-Identifier: BUSL-1.1
"""Self-test for validate-touched.py.

The selection is the whole point of the script: a reverse dependency it fails
to find is a test that silently stops running in the inner loop, and a gate
missing from its table is one that never runs there at all. Each half is
checked on a synthetic module, and the gate table against scripts/ itself.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "vt", Path(__file__).with_name("validate-touched.py")
)
vt = importlib.util.module_from_spec(spec)
sys.modules["vt"] = vt
spec.loader.exec_module(vt)

M = "example.com/m"


def pkg(path: str, imports: tuple[str, ...] = (), test_imports: tuple[str, ...] = ()):
    return vt.GoPackage(
        import_path=f"{M}/{path}",
        rel_dir=path,
        imports=frozenset(f"{M}/{i}" for i in imports) | {"fmt"},
        test_imports=frozenset(f"{M}/{i}" for i in test_imports),
    )


# leaf <- mid <- trellisd; harness's tests import trellisd; other is unrelated.
PKGS = {
    p.import_path: p
    for p in (
        pkg("core/leaf"),
        pkg("internal/mid", ("core/leaf",)),
        pkg("cmd/trellisd", ("internal/mid",)),
        pkg("internal/harness", test_imports=("cmd/trellisd",)),
        pkg("internal/other"),
        pkg("internal/capture"),
    )
}


def plan_for(touched: list[str], host_goos: str = "linux") -> vt.Plan:
    return vt.build_plan(touched, PKGS, "golangci-lint", "0.0.0", host_goos)


def steps(plan: vt.Plan, prefix: str) -> list[vt.Step]:
    return [s for s in plan.steps if s.label.startswith(prefix)]


def go_test_args(plan: vt.Plan) -> list[str]:
    tests = steps(plan, "go test")
    return tests[0].cmd[3:] if tests else []


def check_reverse_dependencies() -> list[str]:
    failures = []
    leaf = plan_for(["core/leaf/leaf.go"])
    got = set(go_test_args(leaf))
    want = {f"{M}/{p}" for p in ("core/leaf", "internal/mid", "cmd/trellisd", "internal/harness")}
    if got != want:
        failures.append(f"leaf change tested {sorted(got)}, want {sorted(want)}")
    if steps(leaf, "go test")[0].cmd[:3] != ["go", "test", "-race"]:
        failures.append("go test must run with -race")
    lint = [s.cmd for s in steps(leaf, "golangci-lint")]
    if lint != [["golangci-lint", "run", "./core/leaf"]]:
        failures.append(f"leaf change linted {lint}, want only the touched package")
    if go_test_args(plan_for(["internal/other/x_test.go"])) != [f"{M}/internal/other"]:
        failures.append("a package nothing imports must test alone")
    return failures


def check_lint_passes() -> list[str]:
    """The GOOS and e2e passes `make lint` adds follow the touched packages."""
    failures = []
    labels = [s.label for s in steps(plan_for(["internal/capture/capture_windows.go"]), "golangci")]
    if labels != ["golangci-lint", "golangci-lint GOOS=windows"]:
        failures.append(f"capture change on linux linted {labels}")
    darwin = [s.label for s in steps(plan_for(["internal/capture/a.go"], "darwin"), "golangci")]
    if darwin != ["golangci-lint", "golangci-lint GOOS=linux", "golangci-lint GOOS=windows"]:
        failures.append(f"capture change on darwin linted {darwin}")
    daemon = [s.label for s in steps(plan_for(["cmd/trellisd/main.go"]), "golangci")]
    if "golangci-lint e2e" not in daemon:
        failures.append(f"a trellisd change must lint the e2e build too, got {daemon}")
    return failures


def check_file_ownership() -> list[str]:
    by_dir = {p.rel_dir: p.import_path for p in PKGS.values()}
    cases = {
        "internal/mid/testdata/golden/a.json": f"{M}/internal/mid",
        "internal/mid/testdata/fixture/main.go": f"{M}/internal/mid",
        "internal/mid/assets/embedded.yaml": f"{M}/internal/mid",
        "core/leaf/leaf_test.go": f"{M}/core/leaf",
        "internal/nopkg/only_windows.go": None,
        "docs/README.md": None,
    }
    return [
        f"{path} owned by {vt.owning_package(path, by_dir)}, want {want}"
        for path, want in cases.items()
        if vt.owning_package(path, by_dir) != want
    ]


def check_scope_triggers() -> list[str]:
    failures = []
    docs = plan_for(["README.md"])
    if any(s.label in {"go test", "golangci-lint", "vitest"} for s in docs.steps):
        failures.append("a docs-only diff must run no Go or UI tests")
    if not steps(docs, "markdownlint"):
        failures.append("a docs diff must lint the markdown it changed")
    if go_test_args(plan_for(["go.mod"])) != ["./..."]:
        failures.append("go.mod must put every package in scope")
    ui = steps(plan_for(["ui/src/App.tsx"]), "vitest")
    if not ui or ui[0].cmd[-2:] != ["--run", "src/App.tsx"] or ui[0].cwd != "ui":
        failures.append(f"a UI source change must run vitest related on it, got {ui}")
    locale = steps(plan_for(["internal/i18n/locales/en/common.json"]), "vitest")
    if not locale or locale[0].cmd[-1] != "../internal/i18n/locales/en/common.json":
        failures.append(f"a locale change must run vitest related on it, got {locale}")
    whole = steps(plan_for(["ui/vitest.config.ts"]), "vitest")
    if not whole or whole[0].cmd != ["npm", "test"]:
        failures.append("a Vitest config change must run the whole suite")
    if not steps(plan_for(["proto/trellis/survey/v1/survey.proto"]), "check-generated"):
        failures.append("a .proto change must run check-generated")
    if not steps(plan_for(["scripts/validate-touched.py"]), "validate-touched self-test"):
        failures.append("a change to this script must run its self-test")
    return failures


def check_gate_table() -> list[str]:
    """Every scripts/check-* is either selectable or named as not input-driven."""
    failures = []
    scripts = {f"scripts/{p.name}" for p in (vt.ROOT / "scripts").glob("check-*")}
    known = {g.script for g in vt.GATES} | vt.NOT_INPUT_DRIVEN
    for missing in sorted(scripts - known):
        failures.append(f"{missing} is in neither GATES nor NOT_INPUT_DRIVEN")
    for stale in sorted(known - scripts):
        failures.append(f"{stale} is listed but does not exist")
    if steps(plan_for(["core/leaf/leaf.go"]), "gate cgo-confinement"):
        failures.append("cgo-confinement must not run off darwin (net links cgo there)")
    if not steps(plan_for(["core/leaf/leaf.go"], "darwin"), "gate cgo-confinement"):
        failures.append("cgo-confinement must run on darwin")
    gates = {s.label for s in plan_for(["scripts/package-reachability-baseline.txt"]).steps}
    if "gate package-reachability" not in gates:
        failures.append("a gate's own baseline must select the gate")
    return failures


def main() -> int:
    failures = (
        check_reverse_dependencies()
        + check_lint_passes()
        + check_file_ownership()
        + check_scope_triggers()
        + check_gate_table()
    )
    for failure in failures:
        print(f"FAIL: {failure}")
    if failures:
        return 1
    print("validate-touched self-test: ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
