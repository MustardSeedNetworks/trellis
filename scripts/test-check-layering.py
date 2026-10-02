#!/usr/bin/env python3
# SPDX-License-Identifier: BUSL-1.1
"""Self-tests for the Go layering gate."""

from __future__ import annotations

import subprocess
import tempfile
import unittest
from pathlib import Path

CHECKER = Path(__file__).with_name("check-layering.py")
M = "github.com/MustardSeedNetworks/trellis"


def go_file(*imports: str, header: str = "") -> str:
    specs = "\n".join(f'\t"{i}"' for i in imports)
    return f"{header}package p\n\nimport (\n\t\"fmt\"\n{specs}\n)\n\nfunc f() {{ fmt.Println() }}\n"


class LayeringTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.root = Path(self.temp_dir.name)
        self.write("core/wifi/wifi.go", go_file())
        self.write("core/survey/survey.go", go_file(f"{M}/core/wifi"))
        self.write("internal/capture/capture.go", go_file(f"{M}/core/wifi"))
        self.write("internal/api/api.go", go_file(f"{M}/core/survey", f"{M}/internal/capture"))
        self.write("cmd/trellisd/main.go", go_file(f"{M}/internal/api", f"{M}/internal/auth"))
        self.write("internal/auth/auth.go", go_file())

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def write(self, path: str, content: str) -> None:
        target = self.root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content, encoding="utf-8")

    def run_checker(self) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["python3", str(CHECKER), "--root", str(self.root)],
            capture_output=True,
            text=True,
            check=False,
        )

    def assert_fails(self, message: str) -> None:
        result = self.run_checker()
        self.assertEqual(1, result.returncode, result.stdout)
        self.assertIn(message, result.stdout)

    def test_layered_tree_passes(self) -> None:
        result = self.run_checker()
        self.assertEqual(0, result.returncode, result.stdout)

    def test_core_importing_the_daemon_fails(self) -> None:
        self.write("core/survey/survey.go", go_file(f"{M}/internal/api"))
        self.assert_fails("core/survey/survey.go: core may not import internal/api")

    def test_core_importing_capture_fails(self) -> None:
        self.write("core/wifi/wifi_test.go", go_file(f"{M}/internal/capture"))
        self.assert_fails("core/wifi/wifi_test.go: core may not import internal/capture")

    def test_capture_beyond_the_wifi_contract_fails(self) -> None:
        self.write("internal/capture/capture.go", go_file(f"{M}/core/survey"))
        self.assert_fails("internal/capture/capture.go: internal/capture may not import core/survey")

    def test_platform_tagged_file_is_checked(self) -> None:
        self.write(
            "internal/capture/capture_darwin.go",
            go_file(f"{M}/internal/auth", header="//go:build darwin && cgo\n\n"),
        )
        self.assert_fails("capture_darwin.go: internal/capture may not import internal/auth")

    def test_single_aliased_import_is_read(self) -> None:
        self.write("core/wifi/wifi.go", f'package p\n\nimport api "{M}/internal/api"\n\nvar _ = api.X\n')
        self.assert_fails("core/wifi/wifi.go: core may not import internal/api")

    def test_unlisted_package_fails(self) -> None:
        self.write("internal/engine/engine.go", go_file())
        self.assert_fails("package internal/engine is in no layer")

    def test_commented_and_string_imports_are_ignored(self) -> None:
        self.write(
            "core/wifi/wifi.go",
            f'package p\n\n// import "{M}/internal/api"\nimport "fmt"\n\n'
            f'var s = `import "{M}/internal/api"`\n\nfunc f() {{ fmt.Println(s) }}\n',
        )
        result = self.run_checker()
        self.assertEqual(0, result.returncode, result.stdout)

    def test_ui_and_testdata_are_skipped(self) -> None:
        self.write("ui/node_modules/x/x.go", go_file(f"{M}/internal/api"))
        self.write("core/survey/testdata/x.go", go_file(f"{M}/internal/api"))
        result = self.run_checker()
        self.assertEqual(0, result.returncode, result.stdout)


if __name__ == "__main__":
    unittest.main()
