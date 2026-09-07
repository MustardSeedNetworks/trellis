#!/bin/bash
# scripts/linklive-fetch-oracle.sh
# Fetches NetAlly's own decode of the AirMapper surveys held in Link-Live, so
# tools/linklive-oracle can compare it with this repo's import of the same
# archives. See docs/12-CROSS-PRODUCT-ORACLE.md.
#
# Writes <out>/index.json (analysis id -> metadata) and
# <out>/<analysis id>.json.gz (the decoded measurements). Nothing it writes
# belongs in the repo: these are the vendor's surveys, and this repo is public.
#
# The token lives in ~/.linklive/token.env, refreshed by
# ~/.linklive/refresh.sh. Link-Live answers 426 well before the token's own
# expiry, so a refresh is forced here rather than waited for.

set -euo pipefail

OUT=${1:-/tmp/ll-oracle}
TOKEN_ENV=${LINKLIVE_TOKEN_ENV:-$HOME/.linklive/token.env}
REFRESH=${LINKLIVE_REFRESH:-$HOME/.linklive/refresh.sh}
API=https://link-live.com/v1

if [ ! -r "$TOKEN_ENV" ]; then
    echo "no Link-Live token at $TOKEN_ENV" >&2
    exit 2
fi
[ -x "$REFRESH" ] && bash "$REFRESH" --force >/dev/null
# shellcheck source=/dev/null
set -a && . "$TOKEN_ENV" && set +a

mkdir -p "$OUT"
curl -fsS -m 60 -H "Authorization: Access $LINKLIVE_ACCESS_TOKEN" \
    "$API/admin/heatmap" -o "$OUT/heatmap.json"

# The heatmap listing carries both the metadata and, per record, a presigned
# link to the decoded survey. Both come out of one response so the links are
# still valid when they are followed.
python3 - "$OUT" <<'PY'
import json, os, subprocess, sys

out = sys.argv[1]
records = json.load(open(os.path.join(out, "heatmap.json")))
index = {}
for record in records:
    href = record.get("airmapperProcessedHref")
    if not href:
        continue
    index[record["_id"]] = {
        "fileName": record.get("fileName"),
        "plan": record.get("floorPlanFilename"),
        "pts": record.get("surveyPointCount"),
        "unit": record.get("unitType"),
        "mode": record.get("surveyMode"),
    }
    path = os.path.join(out, record["_id"] + ".json.gz")
    if os.path.exists(path) and os.path.getsize(path) > 0:
        continue
    subprocess.run(["curl", "-fsSL", "-m", "300", "-o", path, href], check=True)

json.dump(index, open(os.path.join(out, "index.json"), "w"), indent=1)
print(f"{len(index)} decoded surveys in {out}")
PY
