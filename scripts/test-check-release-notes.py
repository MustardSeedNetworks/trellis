#!/usr/bin/env python3
"""Self-test for check-release-notes.py: builds a throwaway repository and
proves the gate goes red for a missing entry, green for a clean entry, and
green for a commit type release-please is not configured to render."""

from __future__ import annotations

import importlib.util
import io
import json
import os
import subprocess
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("gate", HERE / "check-release-notes.py")
gate = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(gate)

CONFIG = {
    "changelog-sections": [
        {"type": "feat", "section": "Features"},
        {"type": "fix", "section": "Bug Fixes"},
        {"type": "chore", "section": "Miscellaneous"},
    ]
}

HEADER = """# Changelog

"""


class Repo:
    """A throwaway repository with two tags and a changelog the test writes."""

    def __init__(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.git("init", "--quiet", "--initial-branch=main")
        self.git("config", "user.email", "gate@example.com")
        self.git("config", "user.name", "Gate Test")
        config = self.root / ".github" / "release-please-config.json"
        config.parent.mkdir(parents=True)
        config.write_text(json.dumps(CONFIG))
        self.commit("chore: seed the tree (#1)")
        self.git("tag", "v0.1.0")

    def git(self, *args: str) -> None:
        subprocess.run(("git", *args), cwd=self.root, check=True, capture_output=True)

    def commit(self, subject: str) -> None:
        marker = self.root / "tree.txt"
        marker.write_text(subject)
        self.git("add", "-A")
        self.git("commit", "--quiet", "-m", subject)

    def changelog(self, body: str, tag: str | None = "v0.2.0") -> None:
        (self.root / "CHANGELOG.md").write_text(HEADER + body)
        self.git("add", "-A")
        self.git("commit", "--quiet", "-m", f"chore(main): release {gate.newest_version(body)}")
        if tag:
            self.git("tag", tag)

    def merge(self, subject: str) -> None:
        """A commit that lands after the release commit, as the queue does."""
        self.commit(subject)

    def run(self, *args: str) -> tuple[int, str]:
        previous = Path.cwd()
        os.chdir(self.root)
        try:
            captured = io.StringIO()
            with redirect_stdout(captured):
                code = gate.main(list(args))
            return code, captured.getvalue()
        finally:
            os.chdir(previous)

    def close(self) -> None:
        self.tmp.cleanup()


class CheckReleaseNotes(unittest.TestCase):
    def setUp(self) -> None:
        self.repo = Repo()
        self.addCleanup(self.repo.close)

    def test_missing_entry_is_red(self) -> None:
        self.repo.commit("fix: correct a real defect (#2)")
        self.repo.changelog("## [0.2.0] (2026-01-01)\n\n### Features\n\n* something else (#3)\n")
        code, output = self.repo.run()
        self.assertEqual(code, 1, output)
        self.assertIn("#2", output)

    def test_mentioned_entry_is_green(self) -> None:
        self.repo.commit("fix: correct a real defect (#2)")
        self.repo.changelog(
            "## [0.2.0] (2026-01-01)\n\n### Bug Fixes\n\n* correct a real defect (#2)\n"
        )
        code, output = self.repo.run()
        self.assertEqual(code, 0, output)

    def test_unrendered_type_is_green(self) -> None:
        """release-please omits a type its config does not name, so requiring
        an entry for one fails a release that is in fact correct."""
        self.repo.commit("build: change how the tree is packaged (#2)")
        self.repo.changelog("## [0.2.0] (2026-01-01)\n\n### Features\n\n* something else (#3)\n")
        code, output = self.repo.run()
        self.assertEqual(code, 0, output)

    def test_breaking_change_is_required_whatever_its_type(self) -> None:
        self.repo.commit("build!: drop a packaging format (#2)")
        self.repo.changelog("## [0.2.0] (2026-01-01)\n\n### Features\n\n* something else (#3)\n")
        code, output = self.repo.run()
        self.assertEqual(code, 1, output)
        self.assertIn("#2", output)


class PendingRelease(unittest.TestCase):
    """--pending checks the release before it is tagged.

    The post-tag check cannot stop the race it reports: by the time it runs,
    the tag exists and the changelog is permanently short. --pending runs on
    the merge-queue tree, where a swept-in commit can still eject the PR.
    """

    def setUp(self) -> None:
        self.repo = Repo()
        self.addCleanup(self.repo.close)

    def test_untagged_release_missing_a_pr_queued_ahead_is_red(self) -> None:
        """The release PR's own merge group: a PR queued ahead of it landed a
        commit its entry never lists, and the release commit sits on top."""
        self.repo.commit("fix: correct a real defect (#2)")
        self.repo.commit("feat: queued ahead of the release PR (#4)")
        self.repo.changelog(
            "## [0.2.0] (2026-01-01)\n\n### Bug Fixes\n\n* correct a real defect (#2)\n",
            tag=None,
        )
        code, output = self.repo.run("--pending")
        self.assertEqual(code, 1, output)
        self.assertIn("#4", output)

    def test_pr_queued_behind_a_short_release_is_green(self) -> None:
        """G-18: the release PR's group fails on #4; the PR queued behind it
        must not be ejected for the same entry."""
        self.repo.commit("fix: correct a real defect (#2)")
        self.repo.commit("feat: queued ahead of the release PR (#4)")
        self.repo.changelog(
            "## [0.2.0] (2026-01-01)\n\n### Bug Fixes\n\n* correct a real defect (#2)\n",
            tag=None,
        )
        self.repo.merge("feat: queued behind the release PR (#5)")
        code, output = self.repo.run("--pending")
        self.assertEqual(code, 0, output)
        self.assertIn("is not the v0.2.0 release commit", output)

    def test_untagged_section_without_its_release_commit_is_red(self) -> None:
        (self.repo.root / "CHANGELOG.md").write_text(
            HEADER + "## [0.2.0] (2026-01-01)\n\n### Bug Fixes\n\n* correct a real defect (#2)\n"
        )
        self.repo.commit("fix: correct a real defect (#2)")
        code, output = self.repo.run("--pending")
        self.assertEqual(code, 1, output)

    def test_commit_merged_after_an_untagged_release_is_green(self) -> None:
        """stem#1702: the release PR merged and the next PR's merge-group run
        started before release-please tagged it. That commit is not in the
        release; it belongs to the next one."""
        self.repo.commit("fix: correct a real defect (#2)")
        self.repo.changelog(
            "## [0.2.0] (2026-01-01)\n\n### Bug Fixes\n\n* correct a real defect (#2)\n",
            tag=None,
        )
        self.repo.merge("feat: queued behind the release PR (#4)")
        code, output = self.repo.run("--pending")
        self.assertEqual(code, 0, output)

    def test_untagged_release_that_describes_the_tree_is_green(self) -> None:
        self.repo.commit("fix: correct a real defect (#2)")
        self.repo.changelog(
            "## [0.2.0] (2026-01-01)\n\n### Bug Fixes\n\n* correct a real defect (#2)\n",
            tag=None,
        )
        code, output = self.repo.run("--pending")
        self.assertEqual(code, 0, output)

    def test_no_pending_release_is_green(self) -> None:
        self.repo.commit("fix: correct a real defect (#2)")
        self.repo.changelog(
            "## [0.2.0] (2026-01-01)\n\n### Bug Fixes\n\n* correct a real defect (#2)\n"
        )
        self.repo.merge("feat: ordinary work after the release (#4)")
        code, output = self.repo.run("--pending")
        self.assertEqual(code, 0, output)


class V0_24_161(unittest.TestCase):
    """The release stem#1197 names: the release PR for 0.24.161 was generated
    with #1475 and #1476, #1477 merged while it sat open, and the queue built
    the release commit on top of #1477. The tag shipped it unlisted."""

    ENTRY = (
        "## [0.24.161](https://github.com/MustardSeedNetworks/trellis/compare/"
        "v0.24.160...v0.24.161) (2026-09-30)\n\n\n### Miscellaneous\n\n"
        "* **deps:** bump toolchain to Go 1.27.1, Node 26.10.0, golangci-lint v2.14.0 "
        "([#1475](https://github.com/MustardSeedNetworks/trellis/issues/1475))\n"
        "* **deps:** update dependency @types/node to v26.6.2 "
        "([#1476](https://github.com/MustardSeedNetworks/trellis/issues/1476))\n"
    )

    def setUp(self) -> None:
        self.repo = Repo()
        self.addCleanup(self.repo.close)
        self.repo.git("tag", "v0.24.160")
        self.repo.commit("chore(deps): update dependency @types/node to v26.6.2 (#1476)")
        self.repo.commit(
            "chore(deps): bump toolchain to Go 1.27.1, Node 26.10.0, golangci-lint v2.14.0 (#1475)"
        )

    def test_merge_queue_tree_with_1477_is_red(self) -> None:
        self.repo.merge("fix(y1564): report a service that fails its acceptance criteria as failed (#1477)")
        self.repo.changelog(self.ENTRY, tag=None)
        code, output = self.repo.run("--pending")
        self.assertEqual(code, 1, output)
        self.assertIn("#1477", output)
        self.assertNotIn("#1475", output)
        self.assertNotIn("#1476", output)

    def test_release_pr_that_describes_its_tree_is_green(self) -> None:
        self.repo.changelog(self.ENTRY, tag=None)
        code, output = self.repo.run("--pending")
        self.assertEqual(code, 0, output)


if __name__ == "__main__":
    unittest.main()
