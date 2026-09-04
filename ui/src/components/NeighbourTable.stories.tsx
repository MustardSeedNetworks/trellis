import { create } from '@bufbuild/protobuf';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { ScannedNetworkSchema } from '@/gen/trellis/survey/v1/survey_pb';
import { NeighbourTable } from './NeighbourTable';

/**
 * The airspaces worth looking at, rather than one representative one: the row
 * the host is joined to, a hidden SSID, a DFS channel, an AP that advertises no
 * BSS Load element beside one that reports an idle channel — each is a cell
 * that renders differently and could regress on its own.
 */
const networks = [
  create(ScannedNetworkSchema, {
    ssid: 'Trellis Lab',
    bssid: '02:00:00:00:00:01',
    signalDbm: -44,
    channel: 36,
    frequencyMhz: 5180,
    security: 'WPA3',
    channelWidthMhz: 80,
    noiseFloorDbm: -95,
    snrDb: 51,
    htMode: 'VHT80',
    associated: true,
    channelUtilizationPercent: 18,
  }),
  create(ScannedNetworkSchema, {
    ssid: 'Trellis Guest',
    bssid: '02:00:00:00:00:02',
    signalDbm: -61,
    channel: 6,
    frequencyMhz: 2437,
    security: 'WPA2',
    channelWidthMhz: 20,
    noiseFloorDbm: -95,
    snrDb: 34,
    htMode: 'HT20',
    // Idle, and reported as such — distinct from the row below, which sent no
    // element at all.
    channelUtilizationPercent: 0,
  }),
  create(ScannedNetworkSchema, {
    ssid: '',
    bssid: '02:00:00:00:00:03',
    signalDbm: -79,
    channel: 100,
    frequencyMhz: 5500,
    security: 'WPA2',
    channelWidthMhz: 40,
    noiseFloorDbm: -95,
    snrDb: 16,
    htMode: 'HT40',
    isDfs: true,
  }),
];

const meta = {
  title: 'Live/NeighbourTable',
  component: NeighbourTable,
  args: { networks },
  decorators: [
    (Story) => (
      <section className="panel max-w-4xl p-5">
        <Story />
      </section>
    ),
  ],
} satisfies Meta<typeof NeighbourTable>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Airspace: Story = {};

/**
 * A sweep that heard nothing says so. An empty table with headings reads as a
 * component that failed to render its rows.
 */
export const NothingHeard: Story = {
  args: { networks: [] },
};
