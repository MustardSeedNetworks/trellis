import type { Meta, StoryObj } from '@storybook/react-vite';
import { CoverageFindings } from './CoverageFindings';

const meta = {
  title: 'Coverage/CoverageFindings',
  component: CoverageFindings,
  args: {
    figures: [
      { label: 'Coverage score', value: '82%' },
      { label: 'Dead zones', value: '2' },
      { label: 'Samples', value: '87' },
    ],
    recommendations: [],
  },
} satisfies Meta<typeof CoverageFindings>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Clear: Story = {
  args: { state: 'ok', headline: 'No dead zones below -75 dBm' },
};

export const DeadZones: Story = {
  args: {
    state: 'warn',
    headline: '2 dead zones below -75 dBm',
    body: 'Each is a measured region where signal fell under the threshold.',
    recommendations: [
      'Add an access point near the north-east corridor.',
      'Raise the transmit power on AP-3 by 3 dB.',
    ],
  },
};

export const Analysing: Story = {
  args: { state: 'unknown', headline: 'Analysing coverage', figures: [] },
};
