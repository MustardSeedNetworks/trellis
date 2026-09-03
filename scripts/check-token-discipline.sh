#!/usr/bin/env bash
# Token discipline for Trellis.
#
# Blocking, matching seed/stem/niac-go. It began advisory because Trellis had
# no theme tokens to be disciplined about; the MSN family shell has since
# landed and every component reads ui/src/theme, so a raw palette class, a
# bare white/black or a hex literal outside theme/ is now a defect (#145).
#
set -uo pipefail

# CI runs this from ui/ (the frontend job's working directory) and people run
# it from the repo root; a wrong guess must be an error, not an empty pass.
if [ -d "ui/src" ]; then
  TARGET="ui/src"
elif [ -d "src" ] && [ -f "package.json" ]; then
  TARGET="src"
else
  echo "ERROR: cannot find ui/src — run from repo root or ui/ directory" >&2
  exit 2
fi

# Stories, tests and mocks are excluded, as in the other three repos.
EXCLUDE_RE='\.(test|spec|stories|mock)\.(ts|tsx):'

FAILED=0
fail() {
  local label="$1" hits="$2" hint="$3"
  [ -z "$hits" ] && return 0
  echo "[FAIL: $label] $(printf '%s\n' "$hits" | grep -c .) occurrence(s) — $hint"
  printf '%s\n' "$hits"
  FAILED=1
}

fail RAW_PALETTE \
  "$(grep -rInE -- '-(gray|slate|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-[0-9]+' \
     "$TARGET" --include='*.tsx' --include='*.ts' 2>/dev/null | grep -vE "$EXCLUDE_RE" || true)" \
  'Use surface-*/text-*/status-*/brand-* tokens from ui/src/theme'

fail BARE_WHITE_BLACK \
  "$(grep -rInE -- '\b(bg|text|border|from|via|to|ring|fill|stroke)-(white|black)\b' \
     "$TARGET" --include='*.tsx' --include='*.ts' 2>/dev/null | grep -vE "$EXCLUDE_RE" || true)" \
  'Use bg-knob / bg-scrim / text-on-* / text-text-inverse'

# A colour literal is 6 or 8 hex digits, or a short form containing an a-f.
# Matching any '#' plus hex characters also matches issue references such as
# "#889", which is noise, and noise is how a gate gets ignored.
fail RAW_HEX \
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

[ "$FAILED" -eq 0 ] || exit 1
echo "OK: theme imports present; no raw palette, bare white/black or hex outside theme/."
