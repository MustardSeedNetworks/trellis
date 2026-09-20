import { create } from '@bufbuild/protobuf';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { LegendStopSchema } from '@/gen/trellis/survey/v1/survey_pb';
import { HeatmapLegend } from './HeatmapLegend';

/** The scale the service painted with, so the legend describes the picture. */
const meta = {
  title: 'Coverage/HeatmapLegend',
  component: HeatmapLegend,
} satisfies Meta<typeof HeatmapLegend>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * The stops the server sends for RSSI at the default -70 dBm threshold: the
 * coverage ramp, orange below the operator's floor and teal at and above it,
 * with the step between the third and fourth stops (core/survey/coveragescale.go).
 * The four values this story used to carry were an invented traffic light that
 * no build ever painted.
 */
export const Rssi: Story = {
  args: {
    unit: 'dBm',
    stops: [
      create(LegendStopSchema, { value: -100, color: '#7a3c06' }),
      create(LegendStopSchema, { value: -85, color: '#c2680f' }),
      create(LegendStopSchema, { value: -70.01, color: '#f0a860' }),
      create(LegendStopSchema, { value: -70, color: '#10646b' }),
      create(LegendStopSchema, { value: -30, color: '#8fd6d8' }),
    ],
  },
};

export const NoScale: Story = {
  args: { unit: 'dBm', stops: [] },
};
