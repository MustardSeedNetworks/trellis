import type { Meta, StoryObj } from '@storybook/react-vite';
import { HeatmapSurface } from './HeatmapSurface';

/**
 * A 1x1 PNG stands in for the rendered surface: these stories are about the
 * controls and the readout around it, and axe needs the image element to exist
 * rather than to contain a heatmap.
 */
const png = new Uint8Array([
  0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
  0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
  0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
  0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
  0x42, 0x60, 0x82,
]);

const meta = {
  title: 'Coverage/HeatmapSurface',
  component: HeatmapSurface,
  args: {
    png,
    width: 400,
    height: 300,
    grid: [-40, -50, -60, -70, -45, -55, -65, -75, -50, -60, -70, -80],
    gridCols: 4,
    gridRows: 3,
    cellSize: 100,
    unit: 'dBm',
    metric: 'rssi',
  },
} satisfies Meta<typeof HeatmapSurface>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Rssi: Story = {};

export const Snr: Story = {
  args: { unit: 'dB', metric: 'snr', grid: [40, 30, 20, 10, 35, 25, 15, 5, 30, 20, 10, 2] },
};

/** No grid: the controls still render and the readout stays a prompt. */
export const NoGrid: Story = {
  args: { grid: [], gridCols: 0, gridRows: 0, cellSize: 0 },
};
