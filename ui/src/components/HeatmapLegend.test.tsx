import { create } from '@bufbuild/protobuf';
import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { LegendStopSchema } from '@/gen/trellis/survey/v1/survey_pb';
import { HeatmapLegend } from './HeatmapLegend';

const stop = (value: number, color: string) => create(LegendStopSchema, { value, color });

describe('HeatmapLegend', () => {
  it('places each stop by its value, not by its index', () => {
    /* The RSSI scale is not evenly spaced — its stops sit at -100, -85, -75,
       -67, -60 and -30 — so a gradient that spread them evenly would label
       colours the image does not use at those signal levels. */
    const { container } = render(
      <HeatmapLegend
        stops={[stop(-100, '#808080'), stop(-75, '#ffcc00'), stop(-50, '#22aa55')]}
        unit="dBm"
      />,
    );
    const gradient = container.querySelector('[data-testid="legend-gradient"]');
    /* jsdom normalises the hex the server sent into rgb() when it parses
       the style, so the colours are asserted in that form. */
    expect(gradient?.getAttribute('style')).toContain('rgb(128, 128, 128) 0.0%');
    expect(gradient?.getAttribute('style')).toContain('rgb(255, 204, 0) 50.0%');
    expect(gradient?.getAttribute('style')).toContain('rgb(34, 170, 85) 100.0%');
  });

  it('labels every stop with its value, so the scale is never colour alone', () => {
    render(<HeatmapLegend stops={[stop(-90, '#808080'), stop(-40, '#22aa55')]} unit="dBm" />);
    expect(screen.getByText('-90 dBm')).toBeInTheDocument();
    expect(screen.getByText('-40 dBm')).toBeInTheDocument();
  });

  it('says the scale is missing rather than drawing a bar that implies a range', () => {
    render(<HeatmapLegend stops={[stop(-70, '#808080')]} unit="dBm" />);
    expect(screen.getByTestId('legend-unavailable')).toBeInTheDocument();
    expect(screen.queryByTestId('legend-gradient')).not.toBeInTheDocument();
  });
});
