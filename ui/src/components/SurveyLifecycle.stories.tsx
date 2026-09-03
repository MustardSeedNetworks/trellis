import { create } from '@bufbuild/protobuf';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { fn } from 'storybook/test';
import { SurveySummarySchema } from '@/gen/trellis/survey/v1/survey_pb';
import { SurveyLifecycle } from './SurveyLifecycle';

/**
 * One story per lifecycle state, because the component's contract is which
 * buttons exist in each. Nothing here calls the service until a click.
 */
function survey(status: string) {
  return create(SurveySummarySchema, { id: 'svy-1', name: 'Everett HQ', status, floorCount: 1 });
}

const meta = {
  title: 'Surveys/SurveyLifecycle',
  component: SurveyLifecycle,
  args: { onDeleted: fn() },
} satisfies Meta<typeof SurveyLifecycle>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Created: Story = { args: { survey: survey('created') } };
export const Walking: Story = { args: { survey: survey('in_progress') } };
export const Paused: Story = { args: { survey: survey('paused') } };
export const Completed: Story = { args: { survey: survey('completed') } };
