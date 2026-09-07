import { describe, expect, it } from 'vitest';
import { scanAge } from './scanAge';

const at = (seconds: number, nanos = 0) => ({
  $typeName: 'google.protobuf.Timestamp' as const,
  seconds: BigInt(seconds),
  nanos,
});

describe('scanAge', () => {
  it('says nothing when the daemon did not say when it swept', () => {
    expect(scanAge(undefined, 1_000_000, 5000)).toBeUndefined();
  });

  it('counts whole seconds since the sweep', () => {
    const age = scanAge(at(1000), 1_007_400, 5000);
    expect(age?.seconds).toBe(7);
  });

  it('is fresh inside two poll intervals and stale past them', () => {
    expect(scanAge(at(1000), 1_010_000, 5000)?.stale).toBe(false);
    expect(scanAge(at(1000), 1_010_001, 5000)?.stale).toBe(true);
  });

  // A daemon whose clock runs ahead of the browser's would otherwise report a
  // reading taken in the future, which reads as a bug in the product rather
  // than as the clock skew it is.
  it('reads a sweep from the future as brand new rather than as negative', () => {
    const age = scanAge(at(1000), 995_000, 5000);
    expect(age?.seconds).toBe(0);
    expect(age?.stale).toBe(false);
  });
});
