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

export const Rssi: Story = {
  args: {
    unit: 'dBm',
    stops: [
      create(LegendStopSchema, { value: -90, color: '#b93a3a' }),
      create(LegendStopSchema, { value: -75, color: '#8a6208' }),
      create(LegendStopSchema, { value: -60, color: '#2f7d4f' }),
      create(LegendStopSchema, { value: -40, color: '#0f6f68' }),
    ],
  },
};

export const NoScale: Story = {
  args: { unit: 'dBm', stops: [] },
};
