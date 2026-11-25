/**
 * How a reading came out. `unknown` is "could not say" (not arrived, not
 * measured), never a quiet `ok`: a verdict on no data is not a good one.
 */
export type VerdictState = 'ok' | 'warn' | 'crit' | 'unknown';
