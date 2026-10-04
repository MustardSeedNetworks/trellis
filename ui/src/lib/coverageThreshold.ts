/**
 * The dead-zone analysis an operator tunes on Coverage and a report is judged
 * against. One definition for both pages: a report analysed at a threshold the
 * Coverage page never offered would disagree with the map it was printed from
 * (trellis#509).
 */

/**
 * Metrics the dead-zone analysis can speak about. Download throughput is not
 * one of them: `GetCoverage` and `GenerateReport` refuse a metric they have no
 * rule for rather than answering about signal strength under another heading.
 */
export const COVERAGE_METRICS = ['rssi', 'snr'] as const;

export type CoverageMetric = (typeof COVERAGE_METRICS)[number];

/* RSSI, SNR, dBm and dB are glossary terms the gate requires verbatim in every
   locale, so they are not translated. */
export const COVERAGE_METRIC_COPY: Record<CoverageMetric, { glossary: string; unit: string }> = {
  rssi: { glossary: 'RSSI', unit: 'dBm' },
  snr: { glossary: 'SNR', unit: 'dB' },
};

/**
 * Each metric's own dead-zone threshold, in its own unit. There is no shared
 * default and no shared range: -75 dB of signal-to-noise is not a number, and
 * a threshold typed under one layer must not follow the operator to the other.
 * The service applies the same defaults when a request omits one.
 */
export const THRESHOLDS: Record<CoverageMetric, { default: number; min: number; max: number }> = {
  /* The service reads a threshold it was sent verbatim; these bounds are the
     range over which each analysis means anything. */
  rssi: { default: -75, min: -90, max: -40 },
  snr: { default: 20, min: 5, max: 40 },
};

export const DEFAULT_THRESHOLDS: Record<CoverageMetric, number> = {
  rssi: THRESHOLDS.rssi.default,
  snr: THRESHOLDS.snr.default,
};

export function isCoverageMetric(metric: string): metric is CoverageMetric {
  return (COVERAGE_METRICS as readonly string[]).includes(metric);
}

/**
 * The typed value as a threshold for `metric`, or undefined while it is not
 * one — mid-edit ("-", "") or out of range — so the last good value stands.
 */
export function parseThreshold(metric: CoverageMetric, raw: string): number | undefined {
  const parsed = Number(raw);
  const { min, max } = THRESHOLDS[metric];
  return Number.isInteger(parsed) && parsed >= min && parsed <= max ? parsed : undefined;
}
