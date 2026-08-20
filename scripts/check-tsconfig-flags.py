#!/usr/bin/env python3
"""TypeScript strictness contract for the MSN family.

The four products keep their own `ui/tsconfig.app.json` — there is no master
repo — so the flag sets drift silently. They did: for a while seed and stem
set `isolatedModules` without `verbatimModuleSyntax`, trellis the reverse, and
niac had `verbatimModuleSyntax: false` written out explicitly. Nothing failed,
because a missing strictness flag never announces itself; it just quietly
checks less than its sibling does.

This asserts the flags this repo has agreed to carry, with the value each must
have. It deliberately does *not* try to compare repos at runtime — a gate that
reaches into a sibling checkout only works on a machine that has one. The list
below is the contract; changing it is a decision someone makes on purpose.

Adding a flag: land it in all four repos, then add it here in all four.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

CONFIG = Path(__file__).resolve().parent.parent / "ui" / "tsconfig.app.json"

# Flag -> required value. Keep in step with the sibling repos' copies.
REQUIRED: dict[str, bool] = {
    "strict": True,
    "isolatedModules": True,
    "verbatimModuleSyntax": True,
    "erasableSyntaxOnly": True,
    "noUncheckedIndexedAccess": True,
    "noUnusedLocals": True,
    "noUnusedParameters": True,
    "noFallthroughCasesInSwitch": True,
    "noUncheckedSideEffectImports": True,
}

# Not yet agreed anywhere; listed so its absence reads as pending rather than
# forgotten. See the modernization plan's Step 6.
PENDING: tuple[str, ...] = ("exactOptionalPropertyTypes",)


def load_options() -> dict[str, object]:
    text = CONFIG.read_text(encoding="utf-8")
    # tsconfig allows comments and trailing commas; JSON does not.
    text = re.sub(r"/\*.*?\*/", "", text, flags=re.DOTALL)
    text = re.sub(r"//[^\n]*", "", text)
    text = re.sub(r",(\s*[}\]])", r"\1", text)
    return json.loads(text).get("compilerOptions", {})


def main() -> int:
    if not CONFIG.is_file():
        print(f"no {CONFIG} - nothing to check")
        return 0

    options = load_options()
    problems: list[str] = []

    for flag, want in REQUIRED.items():
        if flag not in options:
            problems.append(f"  {flag} is missing - the fleet sets it to {str(want).lower()}")
        elif options[flag] != want:
            problems.append(
                f"  {flag} is {json.dumps(options[flag])} - the fleet sets it to {str(want).lower()}"
            )

    if problems:
        print("FAIL: ui/tsconfig.app.json has drifted from the fleet's strictness contract:")
        print("\n".join(problems))
        print("\nIf the change is intended, land it in seed, stem, trellis and niac-go,")
        print("then update REQUIRED in each repo's scripts/check-tsconfig-flags.py.")
        return 1

    pending = [flag for flag in PENDING if flag in options]
    if pending:
        print(f"note: {', '.join(pending)} is set here but not yet agreed fleet-wide")

    print(f"OK: all {len(REQUIRED)} agreed strictness flags are set.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
