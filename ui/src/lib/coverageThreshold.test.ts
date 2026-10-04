import { describe, expect, it } from 'vitest';
import { type CoverageMetric, isCoverageMetric, parseThreshold } from './coverageThreshold';

describe('parseThreshold', () => {
  it.each<[CoverageMetric, string, number | undefined]>([
    ['rssi', '-75', -75],
    ['rssi', '-90', -90],
    ['rssi', '-40', -40],
    ['rssi', '-91', undefined],
    ['rssi', '-39', undefined],
    ['rssi', '-75.5', undefined],
    ['rssi', '-', undefined],
    ['rssi', '', undefined],
    ['snr', '25', 25],
    ['snr', '5', 5],
    ['snr', '40', 40],
    ['snr', '4', undefined],
    /* A dBm figure is not a margin: the bounds are per metric. */
    ['snr', '-75', undefined],
  ])('%s %j → %s', (metric, raw, want) => {
    expect(parseThreshold(metric, raw)).toBe(want);
  });
});

describe('isCoverageMetric', () => {
  it.each([
    ['rssi', true],
    ['snr', true],
    ['download', false],
    ['', false],
  ])('%j → %s', (metric, want) => {
    expect(isCoverageMetric(metric)).toBe(want);
  });
});
