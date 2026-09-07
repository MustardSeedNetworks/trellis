import type { Timestamp } from '@bufbuild/protobuf/wkt';

/**
 * How old the reading on screen is.
 *
 * The Live page polls, and pausing it stops the polling without changing
 * anything the page draws — so a scan from ten minutes ago reads exactly like
 * the airspace right now. That is the one thing a live view must never do, and
 * the daemon has always answered with the moment it swept (`scanned_at`); the
 * page simply never read it.
 *
 * Staleness is measured in poll intervals rather than in seconds so it means
 * the same thing if the interval changes: past two of them, a reading that
 * should have been replaced twice over has not been.
 */
export interface ScanAge {
  /** Whole seconds since the sweep. Never negative — a clock skew reads as 0. */
  seconds: number;
  /** The wall-clock time of the sweep, in the viewer's own locale. */
  takenAt: string;
  /** Older than two poll intervals. */
  stale: boolean;
}

const STALE_INTERVALS = 2;

export function scanAge(
  scannedAt: Timestamp | undefined,
  now: number,
  pollMs: number,
): ScanAge | undefined {
  if (!scannedAt) {
    // A daemon that did not say when it swept cannot be quoted on it. Showing
    // an age of zero would be an invention, and the page shows nothing instead.
    return undefined;
  }
  const takenMs = Number(scannedAt.seconds) * 1000 + Math.floor(scannedAt.nanos / 1_000_000);
  const elapsed = Math.max(0, now - takenMs);
  return {
    seconds: Math.floor(elapsed / 1000),
    takenAt: new Date(takenMs).toLocaleTimeString(),
    stale: elapsed > pollMs * STALE_INTERVALS,
  };
}
