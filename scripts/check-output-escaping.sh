#!/usr/bin/env bash
# check-output-escaping.sh — output-escaping / XSS regression gate.
#
# Two checks, as in seed, stem and niac-go:
#   1. No raw inner-HTML injection prop in ui/src (the React XSS vector).
#   2. No fmt.Fprintf to an http.ResponseWriter that interpolates a value
#      (%s/%v/%q/%d). HTTP responses go through json.NewEncoder or
#      html/template, never a raw format string, so auto-escaping is never
#      bypassed. Trellis serves handlers from internal/ and cmd/trellisd; the
#      scan covers every non-test Go file under cmd, core and internal.
#
# Run locally: scripts/check-output-escaping.sh

set -uo pipefail

cd "$(dirname "$0")/.."

FAIL=0

# A character class keeps this file from containing the banned token itself.
INNER_HTML_RE='dangerously[S]etInnerHTML'

# Writer names a handler gives its http.ResponseWriter.
WRITER_RE='(w|rw|res|resp|writer)'

if [ -d ui/src ]; then
  DANGER=$(grep -rEn "$INNER_HTML_RE" ui/src \
    --include='*.tsx' --include='*.ts' 2>/dev/null \
    | grep -vE '\.(test|spec|stories)\.(ts|tsx):' || true)
  if [ -n "$DANGER" ]; then
    echo "[XSS] raw inner-HTML injection found in UI app code:"
    echo "$DANGER"
    echo "Use plain text/JSX. If HTML rendering is unavoidable, sanitize with a"
    echo "vetted sanitizer and justify it in review."
    echo
    FAIL=1
  fi
fi

HTTP_FMT=$(grep -rEn "fmt\.Fprintf\(${WRITER_RE},[^)]*%[svqd]" cmd core internal \
  --include='*.go' 2>/dev/null \
  | grep -v '_test\.go:' || true)
if [ -n "$HTTP_FMT" ]; then
  echo "[XSS] value-interpolating fmt.Fprintf to an http.ResponseWriter:"
  echo "$HTTP_FMT"
  echo "Use json.NewEncoder(w).Encode(...) or html/template."
  echo
  FAIL=1
fi

if [ "$FAIL" -ne 0 ]; then
  echo "FAIL: output-escaping gate."
  exit 1
fi

echo "OK: no raw inner-HTML injection; no value-interpolating Fprintf to a ResponseWriter."
