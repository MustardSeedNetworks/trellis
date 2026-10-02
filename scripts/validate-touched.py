#!/usr/bin/env python3
# SPDX-License-Identifier: BUSL-1.1
"""validate-touched.py — validate what the branch touched, not the whole tree.

The inner loop. `make test` runs every Go package and `make lint` lints all of
them three times over; a branch that edits one leaf package needs that
package, everything in the module that imports it (directly, transitively, or
from a test), and the gates that read the files it changed. `make test` still
runs once before the PR; this is what runs between edits.

The touched set is every path that differs between the merge base with
origin/main and the working tree, committed or not, plus untracked files.
Renames count as a delete and an add, so the package a file left is validated
too.

Every command is printed before it runs, and a failing step does not stop the
others: the exit status is non-zero when any step failed.

Run locally: make validate-touched. --dry-run prints the plan and runs nothing.
"""

from __future__ import annotations

import argparse
import fnmatch
import os
import subprocess
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path, PurePosixPath

ROOT = Path(__file__).resolve().parent.parent

# Fixed, not a flag: whatever a caller passes reaches the commands this runs.
BASE = "origin/main"

# A change to any of these can alter how every package builds or lints.
WHOLE_MODULE = ("go.mod", "go.sum", ".golangci.yml")

# `make lint` lints these again under the other GOOS values and the e2e tag,
# because their files are split by build constraint and the host's tags hide
# the rest. A touched package under one of them gets the same extra passes.
GOOS_SPLIT = ("internal/capture", "cmd/trellisd")
E2E_TAGGED = ("cmd/trellisd",)

# A change to any of these can alter how every UI test resolves or runs.
WHOLE_UI = (
    "ui/package.json",
    "ui/package-lock.json",
    "ui/vite.config.ts",
    "ui/vitest.config.ts",
    "ui/tsconfig*.json",
    "ui/src/test/*",
)

UI_SOURCE_SUFFIXES = {".ts", ".tsx", ".css", ".json"}

# The locale catalogues sit beside the Go code but the UI imports them
# (the @locales alias in ui/vite.config.ts).
UI_INPUTS = ("ui/src/*", "internal/i18n/locales/*")

# What `make check-generated` regenerates from, and what it compares against.
CONTRACT_INPUTS = ("proto/*", "buf.yaml", "buf.gen*.yaml", "gen/*", "ui/src/gen/*")

SELF = ("scripts/validate-touched.py", "scripts/test-validate-touched.py")


def matches_any(path: str, patterns: tuple[str, ...]) -> bool:
    return any(fnmatch.fnmatchcase(path, p) for p in patterns)


def under(rel_dir: str, roots: tuple[str, ...]) -> bool:
    return any(rel_dir == r or rel_dir.startswith(f"{r}/") for r in roots)


@dataclass(frozen=True)
class Gate:
    """A scripts/check-* gate and the paths it reads.

    Patterns are fnmatch patterns over repo-relative paths, where `*` also
    crosses `/`. The gate's own files (the script, its self-test, its baseline:
    `scripts/<name>-*`) are inputs of every gate implicitly.
    """

    script: str
    inputs: tuple[str, ...]
    # The GOOS the gate means something on, when not every host.
    only_on: str | None = None

    @property
    def name(self) -> str:
        return PurePosixPath(self.script).stem.removeprefix("check-")

    def own_files(self) -> tuple[str, ...]:
        return (
            self.script,
            f"scripts/test-check-{self.name}.*",
            f"scripts/{self.name}-*",
        )

    def reads(self, path: str) -> bool:
        return matches_any(path, self.inputs + self.own_files())

    def commands(self) -> list[list[str]]:
        cmds = []
        self_test = f"scripts/test-check-{self.name}.py"
        if (ROOT / self_test).exists():
            cmds.append(["python3", self_test])
        if self.script.endswith(".py"):
            cmds.append(["python3", self.script])
        else:
            cmds.append([f"./{self.script}"])
        return cmds


GATES = (
    Gate("scripts/check-banned-vocabulary.py", ("*",)),
    # Only a darwin cgo build contains the CoreWLAN files; elsewhere the
    # stdlib's own cgo (net's resolver) is all it finds. CI runs it on macOS.
    Gate("scripts/check-cgo-confinement.py", ("*.go", "go.mod"), only_on="darwin"),
    Gate("scripts/check-component-variants.py", ("ui/src/*",)),
    Gate("scripts/check-file-size.sh", ("*.go", "ui/src/*")),
    Gate("scripts/check-layering.py", ("*.go",)),
    Gate("scripts/check-package-reachability.sh", ("*.go", "go.mod")),
    Gate("scripts/check-theme-contract.py", ("ui/src/*",)),
    Gate("scripts/check-token-discipline.sh", ("ui/src/*",)),
    Gate("scripts/check-tsconfig-flags.py", ("ui/tsconfig*.json",)),
)

# Gates with no file inputs to select on: check-stale-tests is a precondition
# the make target runs first.
NOT_INPUT_DRIVEN = {"scripts/check-stale-tests.sh"}


@dataclass(frozen=True)
class GoPackage:
    import_path: str
    rel_dir: str
    imports: frozenset[str]
    test_imports: frozenset[str]


@dataclass(frozen=True)
class Step:
    label: str
    cmd: list[str]
    cwd: str = "."


@dataclass
class Plan:
    steps: list[Step] = field(default_factory=list)
    notes: list[str] = field(default_factory=list)


def run_capture(cmd: list[str]) -> str:
    return subprocess.run(
        cmd, cwd=ROOT, check=True, capture_output=True, text=True
    ).stdout


def touched_files() -> list[str]:
    merge_base = run_capture(["git", "merge-base", BASE, "HEAD"]).strip()
    diff = run_capture(["git", "diff", "--name-only", "--no-renames", merge_base])
    untracked = run_capture(["git", "ls-files", "--others", "--exclude-standard"])
    return sorted(set(diff.split()) | set(untracked.split()))


def go_packages() -> dict[str, GoPackage]:
    """Every package in the module under the host's build tags, by import path."""
    sep = "\x1f"
    fmt = sep.join(
        [
            "{{.ImportPath}}",
            "{{.Dir}}",
            '{{join .Imports ","}}',
            '{{join .TestImports ","}}',
            '{{join .XTestImports ","}}',
        ]
    )
    pkgs = {}
    for line in run_capture(["go", "list", "-f", fmt, "./..."]).splitlines():
        path, directory, imports, test_imports, xtest_imports = line.split(sep)
        pkgs[path] = GoPackage(
            import_path=path,
            rel_dir=Path(directory).relative_to(ROOT).as_posix(),
            imports=frozenset(filter(None, imports.split(","))),
            test_imports=frozenset(
                filter(None, f"{test_imports},{xtest_imports}".split(","))
            ),
        )
    return pkgs


def owning_package(path: str, by_dir: dict[str, str]) -> str | None:
    """The package a changed file belongs to, if any.

    A .go file belongs to the package in its own directory. Any other file —
    an embedded asset, anything under testdata, including .go fixtures there —
    belongs to the nearest enclosing package, because its tests read it.
    """
    directory = PurePosixPath(path).parent
    if path.endswith(".go") and "testdata" not in directory.parts:
        return by_dir.get(directory.as_posix())
    while directory.as_posix() not in by_dir:
        if directory == PurePosixPath("."):
            return None
        directory = directory.parent
    return by_dir[directory.as_posix()]


def reverse_dependencies(changed: set[str], pkgs: dict[str, GoPackage]) -> set[str]:
    """Changed packages, every in-module package that imports one of them
    directly or transitively, and every package whose tests import any of those."""
    importers: dict[str, set[str]] = {p: set() for p in pkgs}
    for pkg in pkgs.values():
        for dep in pkg.imports & pkgs.keys():
            importers[dep].add(pkg.import_path)
    affected = set(changed)
    frontier = list(changed)
    while frontier:
        for importer in importers[frontier.pop()]:
            if importer not in affected:
                affected.add(importer)
                frontier.append(importer)
    affected |= {p.import_path for p in pkgs.values() if p.test_imports & affected}
    return affected


def lint_steps(golangci: str, dirs: list[str], host_goos: str) -> list[Step]:
    """The `make lint` passes, restricted to the given package directories."""
    run = [golangci, "run"]
    steps = [Step("golangci-lint", [*run, *dirs])]
    split = [d for d in dirs if under(d.removeprefix("./"), GOOS_SPLIT)]
    for goos in ("linux", "windows"):
        if split and goos != host_goos:
            steps.append(
                Step(f"golangci-lint GOOS={goos}", ["env", f"GOOS={goos}", *run, *split])
            )
    tagged = [d for d in dirs if under(d.removeprefix("./"), E2E_TAGGED)]
    if tagged:
        steps.append(
            Step("golangci-lint e2e", [*run, "--build-tags", "e2e", *tagged])
        )
    return steps


def plan_go(
    touched: list[str],
    pkgs: dict[str, GoPackage],
    plan: Plan,
    golangci: str,
    host_goos: str,
) -> None:
    whole = [f for f in touched if f in WHOLE_MODULE]
    if whole:
        plan.notes.append(f"Go: {', '.join(whole)} changed, so every package is in scope")
        dirs = sorted({f"./{p.rel_dir}" for p in pkgs.values()})
        plan.steps.extend(lint_steps(golangci, dirs, host_goos))
        plan.steps.append(Step("go test", ["go", "test", "-race", "./..."]))
        return
    by_dir = {p.rel_dir: p.import_path for p in pkgs.values()}
    changed = set()
    for path in touched:
        owner = owning_package(path, by_dir)
        if owner:
            changed.add(owner)
        elif path.endswith(".go"):
            plan.notes.append(f"Go: {path} is in no package under this host's build tags")
    if not changed:
        plan.notes.append("Go: no package touched, no Go lint or tests")
        return
    affected = reverse_dependencies(changed, pkgs)
    plan.notes.append(
        f"Go: {len(changed)} package(s) touched, "
        f"{len(affected) - len(changed)} reverse dependent(s)"
    )
    dirs = sorted(f"./{pkgs[p].rel_dir}" for p in changed)
    plan.steps.extend(lint_steps(golangci, dirs, host_goos))
    plan.steps.append(Step("go test", ["go", "test", "-race", *sorted(affected)]))


def plan_ui(touched: list[str], plan: Plan) -> None:
    whole = [f for f in touched if matches_any(f, WHOLE_UI)]
    sources = [
        f
        for f in touched
        if matches_any(f, UI_INPUTS) and PurePosixPath(f).suffix in UI_SOURCE_SUFFIXES
    ]
    deleted = [f for f in sources if not (ROOT / f).exists()]
    present = [os.path.relpath(f, "ui") for f in sources if f not in deleted]
    if whole or deleted:
        reason = whole or [f"{f} (deleted)" for f in deleted]
        plan.notes.append(f"UI: {', '.join(reason)} changed, so the whole Vitest suite runs")
        plan.steps.append(Step("vitest", ["npm", "test"], cwd="ui"))
    elif present:
        plan.steps.append(
            Step("vitest", ["npx", "vitest", "related", "--run", *present], cwd="ui")
        )
    else:
        plan.notes.append("UI: no UI source touched, no Vitest")

    # Biome and tsc cover ui/ as a whole, e2e specs and configs included.
    ui_files = [
        os.path.relpath(f, "ui")
        for f in touched
        if f.startswith("ui/")
        and PurePosixPath(f).suffix in UI_SOURCE_SUFFIXES
        and (ROOT / f).exists()
    ]
    if ui_files:
        plan.steps.append(
            Step(
                "biome",
                ["npx", "biome", "check", "--no-errors-on-unmatched", *ui_files],
                cwd="ui",
            )
        )
    if whole or any(PurePosixPath(f).suffix in {".ts", ".tsx"} for f in ui_files):
        plan.steps.append(Step("typecheck", ["npm", "run", "typecheck"], cwd="ui"))


def plan_docs(touched: list[str], plan: Plan, markdownlint: str) -> None:
    docs = [f for f in touched if f.endswith(".md") and (ROOT / f).exists()]
    if docs:
        cmd = ["npx", "--yes", f"markdownlint-cli2@{markdownlint}", *docs]
        plan.steps.append(Step("markdownlint", cmd))


def plan_gates(touched: list[str], plan: Plan, host_goos: str) -> None:
    for gate in GATES:
        if not any(gate.reads(f) for f in touched):
            continue
        if gate.only_on and gate.only_on != host_goos:
            plan.notes.append(f"gate {gate.name}: runs only on {gate.only_on}, skipped")
        else:
            plan.steps.extend(Step(f"gate {gate.name}", cmd) for cmd in gate.commands())
    if any(matches_any(f, CONTRACT_INPUTS) for f in touched):
        plan.steps.append(Step("check-generated", ["make", "check-generated"]))
    if any(f in SELF for f in touched):
        plan.steps.append(
            Step("validate-touched self-test", ["python3", "scripts/test-validate-touched.py"])
        )


def build_plan(
    touched: list[str],
    pkgs: dict[str, GoPackage],
    golangci: str,
    markdownlint: str,
    host_goos: str,
) -> Plan:
    plan = Plan()
    plan_go(touched, pkgs, plan, golangci, host_goos)
    plan_ui(touched, plan)
    plan_docs(touched, plan, markdownlint)
    plan_gates(touched, plan, host_goos)
    return plan


def make_pin(makefile: str, name: str) -> str:
    """A `NAME := value` pin, read where Renovate and the lint targets read it."""
    for line in (ROOT / makefile).read_text().splitlines():
        key, _, value = line.partition(":=")
        if key.strip() == name:
            return value.strip()
    sys.exit(f"validate-touched: {makefile} pins no {name}")


def golangci_binary() -> str:
    """The GOPATH golangci-lint, refused unless it is the version CI pins."""
    want = make_pin("Makefile", "GOLANGCI_LINT_VERSION").removeprefix("v")
    binary = str(Path(run_capture(["go", "env", "GOPATH"]).strip()) / "bin" / "golangci-lint")
    try:
        version = run_capture([binary, "version"])
    except (OSError, subprocess.CalledProcessError):
        version = ""
    if f"version {want} " not in version:
        sys.exit(
            f"validate-touched: {binary} is not golangci-lint {want}; "
            "`make golangci-lint` installs the pinned version"
        )
    return binary


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--dry-run", action="store_true", help="print the plan, run nothing")
    args = parser.parse_args()

    touched = touched_files()
    print(f"validate-touched: {len(touched)} path(s) differ from {BASE}")
    if not touched:
        return 0
    plan = build_plan(
        touched,
        go_packages(),
        "golangci-lint" if args.dry_run else golangci_binary(),
        make_pin("mk/lint.mk", "MARKDOWNLINT_CLI2_VERSION"),
        run_capture(["go", "env", "GOOS"]).strip(),
    )
    for note in plan.notes:
        print(f"  {note}")

    failed = []
    started_all = time.monotonic()
    for step in plan.steps:
        where = "" if step.cwd == "." else f"(cd {step.cwd}) "
        print(f"\n+ {where}{' '.join(step.cmd)}", flush=True)
        if args.dry_run:
            continue
        started = time.monotonic()
        status = subprocess.run(step.cmd, cwd=ROOT / step.cwd, check=False).returncode
        print(f"  [{step.label}: exit {status}, {time.monotonic() - started:.1f} s]")
        if status != 0:
            failed.append(step.label)
    elapsed = f"{time.monotonic() - started_all:.1f} s"
    if failed:
        print(f"\nvalidate-touched: FAILED in {elapsed}: {', '.join(failed)}")
        return 1
    verdict = "planned" if args.dry_run else f"passed in {elapsed}"
    print(f"\nvalidate-touched: {len(plan.steps)} step(s) {verdict}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
