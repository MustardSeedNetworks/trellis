import type { Meta, StoryObj } from '@storybook/react-vite';
import { StatusRollup } from './StatusRollup';

/**
 * The one band that answers "is anything wrong", in each of its four states.
 * `unknown` is the one worth looking at: it must print em dashes for figures
 * it was given, never the numbers.
 */
const meta = {
  title: 'Shell/StatusRollup',
  component: StatusRollup,
  args: {
    figures: [
      { label: 'Surveys', value: '4' },
      { label: 'Samples', value: '312' },
    ],
  },
} satisfies Meta<typeof StatusRollup>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Ok: Story = {
  args: { state: 'ok', headline: '4 surveys available' },
};

export const Warn: Story = {
  args: {
    state: 'warn',
    headline: '2 dead zones below -75 dBm',
    body: 'Each is a measured region where signal fell under the threshold.',
  },
};

export const Crit: Story = {
  args: {
    state: 'crit',
    headline: 'Import failed',
    body: 'floorplan.amp, 2.1 MB. invalid zip archive: zip: not a valid zip file',
  },
};

export const Unknown: Story = {
  args: {
    state: 'unknown',
    headline: 'Survey data is not arriving',
    body: 'The survey service did not answer: connection refused',
  },
};
