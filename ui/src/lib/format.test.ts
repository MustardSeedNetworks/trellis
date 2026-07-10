import { describe, expect, it } from 'vitest';
import { bytesToDataUrl, formatCoverageScore, formatSignal, reportFilename } from '@/lib/format';

describe('bytesToDataUrl', () => {
  it('encodes bytes as a base64 data URL with the given mime type', () => {
    const bytes = new Uint8Array([0x89, 0x50, 0x4e, 0x47]);
    expect(bytesToDataUrl(bytes, 'image/png')).toBe('data:image/png;base64,iVBORw==');
  });

  it('produces a decodable data URL', () => {
    const original = new Uint8Array([1, 2, 3, 255, 0, 128]);
    const url = bytesToDataUrl(original, 'application/octet-stream');
    const base64 = url.slice(url.indexOf(',') + 1);
    const decoded = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0));
    expect(Array.from(decoded)).toEqual(Array.from(original));
  });

  it('returns an empty-payload data URL for empty bytes', () => {
    expect(bytesToDataUrl(new Uint8Array(), 'image/png')).toBe('data:image/png;base64,');
  });
});

describe('formatSignal', () => {
  it('formats to one decimal place with a unit suffix', () => {
    expect(formatSignal(-67.34, 'dBm')).toBe('-67.3 dBm');
    expect(formatSignal(24, 'dB')).toBe('24.0 dB');
  });
});

describe('formatCoverageScore', () => {
  it('formats a 0-1 score as a rounded percentage', () => {
    expect(formatCoverageScore(0.823)).toBe('82%');
    expect(formatCoverageScore(1)).toBe('100%');
    expect(formatCoverageScore(0)).toBe('0%');
  });
});

describe('reportFilename', () => {
  it('slugifies a survey name into a pdf filename', () => {
    expect(reportFilename('Everett HQ')).toBe('everett-hq-survey-report.pdf');
  });

  it('collapses punctuation and trims dashes', () => {
    expect(reportFilename('  Floor 3 — West!! ')).toBe('floor-3-west-survey-report.pdf');
  });

  it('falls back to a generic name when nothing usable remains', () => {
    expect(reportFilename('—')).toBe('survey-survey-report.pdf');
  });
});
