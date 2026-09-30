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

  it.each([
    /* The stops CoverageScale sends: the ramp steps at the threshold with two
       stops a hundredth apart, so printed raw the key read "-60.01 dBm" beside
       "-60 dBm" (#602). These are the labels the PDF key prints for the same
       scales (core/survey report_heatmap_key_test.go). */
    {
      name: 'rssi at the threshold',
      values: [-100, -80, -60.01, -60, -30],
      unit: 'dBm',
      want: ['-100 dBm', '-80 dBm', '< -60 dBm', '-60 dBm', '-30 dBm'],
    },
    {
      name: 'snr at the threshold',
      values: [0, 12.5, 24.99, 25, 50],
      unit: 'dB',
      want: ['0 dB', '12.5 dB', '< 25 dB', '25 dB', '50 dB'],
    },
    {
      /* -66.75 rounds away from zero, as the PDF does; Math.round alone
         would print -66.7 and the two keys would disagree. */
      name: 'rssi threshold clamped inside the range',
      values: [-100, -66.75, -33.51, -33.5, -30],
      unit: 'dBm',
      want: ['-100 dBm', '-66.8 dBm', '< -33.5 dBm', '-33.5 dBm', '-30 dBm'],
    },
  ])('labels the two sides of the threshold step: $name', ({ values, unit, want }) => {
    render(<HeatmapLegend stops={values.map((value) => stop(value, '#808080'))} unit={unit} />);
    const labels = screen.getAllByRole('listitem').map((item) => item.textContent);
    expect(labels).toEqual(want);
  });

  it('says the scale is missing rather than drawing a bar that implies a range', () => {
    render(<HeatmapLegend stops={[stop(-70, '#808080')]} unit="dBm" />);
    expect(screen.getByTestId('legend-unavailable')).toBeInTheDocument();
    expect(screen.queryByTestId('legend-gradient')).not.toBeInTheDocument();
  });
});
