#!/usr/bin/env bash
# Token discipline for Trellis.
#
# Trellis had no gate: it also had no tokens until the MSN family theme landed,
# so there was nothing to be disciplined about. Everything here is ADVISORY for
# now. The product is five components against a Tailwind primitive palette, and
# the rewrite onto tokens happens when stem's shell is copied across; failing
# every commit until then would teach people to pass --no-verify.
#
# Promote these to blocking once the shell lands, matching seed/stem/niac-go,
# where the same colour rules already block.

set -uo pipefail

TARGET="${1:-ui/src}"
[ -d "$TARGET" ] || { echo "no $TARGET — nothing to check"; exit 0; }

# Stories, tests and mocks are excluded, as in the other three repos.
EXCLUDE_RE='\.(test|spec|stories|mock)\.(ts|tsx):'

warn() {
  local label="$1" hits="$2" hint="$3"
  [ -z "$hits" ] && return 0
  echo "[warn: $label] $(printf '%s\n' "$hits" | grep -c .) occurrence(s) — $hint"
}

warn RAW_PALETTE \
  "$(grep -rInE -- '-(gray|slate|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-[0-9]+' \
     "$TARGET" --include='*.tsx' --include='*.ts' 2>/dev/null | grep -vE "$EXCLUDE_RE" || true)" \
  'Use surface-*/text-*/status-*/brand-* tokens from ui/src/theme'

warn BARE_WHITE_BLACK \
  "$(grep -rInE -- '\b(bg|text|border|from|via|to|ring|fill|stroke)-(white|black)\b' \
     "$TARGET" --include='*.tsx' --include='*.ts' 2>/dev/null | grep -vE "$EXCLUDE_RE" || true)" \
  'Use bg-knob / bg-scrim / text-on-* / text-text-inverse'

# A colour literal is 6 or 8 hex digits, or a short form containing an a-f.
# Matching any '#' plus hex characters also matches issue references such as
# "#889", which is noise, and noise is how a gate gets ignored.
warn RAW_HEX \
  "$(grep -rInE -- '#([0-9a-fA-F]{6}([0-9a-fA-F]{2})?|[0-9a-fA-F]{0,3}[a-fA-F][0-9a-fA-F]{0,3})\b' \
     "$TARGET" --include='*.tsx' --include='*.ts' --include='*.css' 2>/dev/null \
     | grep -v "$TARGET/theme/" | grep -v "/assets/" | grep -vE ':[0-9]+:\s*(\*|//|/\*)' | grep -vE "$EXCLUDE_RE" || true)" \
  'Raw hex outside theme/ — define it in ui/src/theme and reference the token'

# The theme must actually be wired, or every token above resolves to nothing.
if [ -f "$TARGET/index.css" ]; then
  grep -q 'theme/msn-shared.css' "$TARGET/index.css" \
    || { echo "FAIL: index.css does not import theme/msn-shared.css"; exit 1; }
  grep -q 'theme/product-trellis.css' "$TARGET/index.css" \
    || { echo "FAIL: index.css does not import theme/product-trellis.css"; exit 1; }
fi

echo "OK: theme imports present; colour rules are advisory until the shell lands."
