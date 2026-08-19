/**
 * Encodes raw bytes as a base64 data URL so a Connect `bytes` field (e.g. a
 * rendered PNG heatmap) can be dropped straight into an <img src>.
 */
export function bytesToDataUrl(bytes: Uint8Array, mimeType: string): string {
  let binary = '';
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return `data:${mimeType};base64,${btoa(binary)}`;
}

/**
 * Formats a signal metric (dBm or dB) to one decimal place, e.g. "-67.3 dBm".
 */
export function formatSignal(value: number, unit: string): string {
  return `${value.toFixed(1)} ${unit}`;
}

/**
 * Formats a coverage score as a whole-number percentage, e.g. "82%".
 *
 * The score arrives already expressed as a percentage — `core/survey` documents
 * `CoverageScore` as 0-100 and computes it as a percentage of samples above the
 * threshold. This used to multiply by 100 again, rendering 82% as "8200%".
 */
export function formatCoverageScore(score: number): string {
  return `${Math.round(score)}%`;
}

/**
 * Builds a safe download filename for a survey's PDF report from its name,
 * e.g. "Everett HQ" → "everett-hq-survey-report.pdf". Falls back to a generic
 * name when the survey name has no usable characters.
 */
export function reportFilename(surveyName: string): string {
  const slug = surveyName
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return `${slug || 'survey'}-survey-report.pdf`;
}
