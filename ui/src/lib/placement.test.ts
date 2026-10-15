import { describe, expect, it } from 'vitest';
import { clampToPlan, isBand, placedOnBand, pointerToPlan } from './placement';

describe('clampToPlan', () => {
  it.each([
    { name: 'inside the plan', point: { x: 120.4, y: 80.6 }, want: { x: 120, y: 81 } },
    { name: 'past the right and bottom edge', point: { x: 900, y: 700 }, want: { x: 799, y: 599 } },
    { name: 'past the left and top edge', point: { x: -40, y: -1 }, want: { x: 0, y: 0 } },
    { name: 'exactly on the far edge', point: { x: 800, y: 600 }, want: { x: 799, y: 599 } },
  ])('keeps a point $name on a pixel of an 800x600 plan', ({ point, want }) => {
    expect(clampToPlan(point, 800, 600)).toEqual(want);
  });
});

describe('pointerToPlan', () => {
  // An 800x600 plan drawn at half size, 10 px in from the viewport's corner.
  const box = { left: 10, top: 10, width: 400, height: 300 };

  it.each([
    { name: 'the centre', client: { x: 210, y: 160 }, want: { x: 400, y: 300 } },
    { name: 'the top-left corner', client: { x: 10, y: 10 }, want: { x: 0, y: 0 } },
    { name: 'a drag past the right edge', client: { x: 900, y: 160 }, want: { x: 799, y: 300 } },
    { name: 'a drag above the plan', client: { x: 210, y: -50 }, want: { x: 400, y: 0 } },
  ])('maps $name back through the rendered size', ({ client, want }) => {
    expect(pointerToPlan(client, box, 800, 600)).toEqual(want);
  });

  it('has no pixel for a plan that is not laid out', () => {
    expect(pointerToPlan({ x: 5, y: 5 }, { left: 0, top: 0, width: 0, height: 0 }, 800, 600)).toBe(
      undefined,
    );
  });
});

describe('placedOnBand', () => {
  it.each([
    { name: 'nothing placed', placements: [], want: false },
    { name: 'only another band', placements: [{ band: '2.4 GHz' }], want: false },
    { name: 'the band itself', placements: [{ band: '2.4 GHz' }, { band: '5 GHz' }], want: true },
    { name: 'an AP that states no band', placements: [{ band: '' }], want: true },
  ])('is $want with $name', ({ placements, want }) => {
    expect(placedOnBand(placements, '5 GHz')).toBe(want);
  });
});

describe('isBand', () => {
  it('accepts the three bands and nothing else', () => {
    expect(['2.4 GHz', '5 GHz', '6 GHz'].every(isBand)).toBe(true);
    expect(isBand('5GHz')).toBe(false);
    expect(isBand('')).toBe(false);
  });
});
