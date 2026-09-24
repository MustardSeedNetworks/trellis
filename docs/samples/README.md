# Coverage ramp report sample

[English PDF](coverage-ramp-en.pdf) exercises the production report generator
for UI-TRL-9 (trellis#484). Its synthetic points reuse the positions and RSSI
readings from `core/survey/report_heatmap_test.go`: -45, -57 and -69 dBm. These
are not site measurements. No floor plan is supplied; the product derives the
map extent from the points.

The operator threshold is deliberately **-60 dBm**, not the default. The map
and its key must show orange below that threshold and teal at/above it. The
key is generated from the exact scale used to paint the map. The sample
includes the cover, floor statistics and coverage map; optional summary and
recommendations are off to keep this acceptance artifact focused.

Regenerated on September 24, 2026 with the key-label fix for
[trellis#571](https://github.com/MustardSeedNetworks/trellis/issues/571). All
three pages were rendered and inspected: text is readable and unclipped, the
map is present, and its orange/teal boundary agrees with the key. The two
stops either side of the threshold now read `< -60 dBm` (orange) and
`-60 dBm` (teal); before the fix both read `-60 dBm`.

Regenerate from the repository root:

```sh
go run ./tools/report-sample > docs/samples/coverage-ramp-en.pdf
pdftoppm -png -r 120 docs/samples/coverage-ramp-en.pdf /tmp/coverage-ramp
pdftotext -layout docs/samples/coverage-ramp-en.pdf -
```

Inspect every rendered page for clipped text, missing maps, incorrect swatches
or labels. The generation date is supplied by the production generator, so
regeneration on a later date changes the PDF bytes.

## Language scope

The required policy is always the current UI language. This acceptance sample
uses an English UI context and therefore English output. The current report
API has no locale parameter and its strings are English; this artifact does
not establish French or other-language report support. That remains separate
from the coverage-ramp acceptance and must not be inferred from this sample.
