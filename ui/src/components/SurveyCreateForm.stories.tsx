import type { Meta, StoryObj } from '@storybook/react-vite';
import { fn } from 'storybook/test';
import { SurveyCreateForm } from './SurveyCreateForm';

const meta = {
  title: 'Surveys/SurveyCreateForm',
  component: SurveyCreateForm,
  args: { onCreated: fn() },
  decorators: [
    (Story) => (
      <aside className="panel w-72">
        <Story />
      </aside>
    ),
  ],
} satisfies Meta<typeof SurveyCreateForm>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Empty: Story = {};
