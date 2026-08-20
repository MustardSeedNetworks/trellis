#!/usr/bin/env python3
"""Theme contract for Trellis.

The shell components are copied between seed / stem / niac / trellis rather
than shared as a package, which ui/src/index.css states plainly: a component
copied from a sibling "compiles here unchanged". That claim was false for
spacing. PageHeader arrived using nine semantic classes and trellis defined
one of them, so eight resolved to nothing and every page header rendered with
no gaps and default type -- the page icon sat flush against its title.

Nothing could catch it. The classes are spelled correctly, they are correct in
the repo they came from, and a theme that is *defined* but not *rendered* is
exactly the failure the design rollout retired grep-based theme validation
over: asserting that palette strings appear in files cannot fail for the only
failure mode that matters.

So this checks the other direction: every semantic class the markup *uses* has
a definition behind it. It is deliberately narrow -- only the role-named scale
(`gap-default`, not Tailwind's `gap-3`), because a false positive is how a
gate gets ignored.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

UI_SRC = Path(__file__).resolve().parent.parent / "ui" / "src"

# A class of ours is named for its role rather than its value.
SEMANTIC = re.compile(
    r"^(?:gap|mb|mt|ml|pt|px|py|p|space-y)-"
    r"(?:tight|compact|default|comfortable|spacious|heading|content|section|inline|cell|xs|sm|lg|xl)$"
    r"|^(?:heading-[1-4]|body-small|body-large|kicker|kicker-accent|figure|panel|panel-hover"
    r"|target|stack|stack-xs|stack-sm|stack-lg|stack-xl|pad|pad-xs|pad-sm|pad-lg|pad-xl"
    r"|flex-between|flex-center)$"
)

CLASS_DEF = re.compile(r"^\s*\.([a-z0-9-]+)\s*\{", re.MULTILINE)
CLASS_USE = re.compile(r'className=(?:"([^"]*)"|\{`([^`]*)`\})')


def defined_classes() -> set[str]:
    css = "\n".join(p.read_text(encoding="utf-8") for p in UI_SRC.rglob("*.css"))
    return set(CLASS_DEF.findall(css))


def used_classes() -> dict[str, list[str]]:
    used: dict[str, list[str]] = {}
    for path in UI_SRC.rglob("*.tsx"):
        if "/gen/" in str(path):
            continue
        for quoted, templated in CLASS_USE.findall(path.read_text(encoding="utf-8")):
            for raw in f"{quoted} {templated}".split():
                token = raw.strip("`${}")
                if SEMANTIC.match(token):
                    used.setdefault(token, []).append(str(path.relative_to(UI_SRC)))
    return used


def main() -> int:
    if not UI_SRC.is_dir():
        print(f"no {UI_SRC} - nothing to check")
        return 0

    defined = defined_classes()
    used = used_classes()
    if not used:
        print("FAIL: matched no semantic classes at all - the contract is not looking at anything")
        return 1

    missing = {name: files for name, files in used.items() if name not in defined}
    if missing:
        print("FAIL: semantic classes used in markup with no definition in ui/src/theme:")
        for name, files in sorted(missing.items()):
            where = ", ".join(sorted(set(files))[:3])
            print(f"  .{name} - used in {len(files)} place(s): {where}")
        print("\nDefine them in ui/src/theme, matching the sibling repos' values.")
        return 1

    print(f"OK: all {len(used)} semantic classes used in markup are defined in the theme.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
