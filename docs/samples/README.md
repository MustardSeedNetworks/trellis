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

Generated against the report implementation at `50b0fbb` (v0.2.86), which
includes the coverage-ramp change from PR #552. All three pages were rendered
and inspected on September 22, 2026: text is readable and unclipped, the map
is present, and its orange/teal boundary agrees with the key.

The existing renderer rounds the two threshold-adjacent stops to the same
`-60 dBm` label. Their different colors remain visible; the labeling ambiguity
is tracked in [trellis#571](https://github.com/MustardSeedNetworks/trellis/issues/571).
This sample does not fix that production behavior.

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
