import { describe, expect, it } from 'vitest';
import {
  bandLabel,
  bytesToDataUrl,
  formatCoverageScore,
  formatSignal,
  reportFilename,
} from '@/lib/format';

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
  /* The service sends a percentage (core/survey documents CoverageScore as
     0-100), so a score of 82 is 82% and not 8200%. */
  it('rounds an already-percentage score', () => {
    expect(formatCoverageScore(82.3)).toBe('82%');
    expect(formatCoverageScore(100)).toBe('100%');
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

describe('bandLabel', () => {
  it('reads the band from the frequency, not the channel', () => {
    expect(bandLabel(2412)).toBe('2.4 GHz');
    expect(bandLabel(5180)).toBe('5 GHz');
    // Channel 1 in both 2.4 and 6 GHz. Naming the band off the channel number
    // would file this AP under 2.4 GHz.
    expect(bandLabel(5955)).toBe('6 GHz');
  });

  it('reports a frequency in no allocation rather than guessing', () => {
    expect(bandLabel(0)).toBe('—');
    expect(bandLabel(900)).toBe('—');
  });

  it('puts the 5/6 GHz boundary at 5900 MHz', () => {
    expect(bandLabel(5895)).toBe('5 GHz');
    expect(bandLabel(5900)).toBe('6 GHz');
  });
});
