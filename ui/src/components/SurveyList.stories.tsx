import { create } from '@bufbuild/protobuf';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { fn } from 'storybook/test';
import { SurveySummarySchema } from '@/gen/trellis/survey/v1/survey_pb';
import { SurveyList } from './SurveyList';

const surveys = [
  create(SurveySummarySchema, {
    id: 'svy-1',
    name: 'Everett HQ',
    status: 'completed',
    floorCount: 2,
    sampleCount: 87,
    hasFloorPlan: true,
  }),
  create(SurveySummarySchema, {
    id: 'svy-2',
    name: 'Lab walk',
    status: 'in_progress',
    floorCount: 1,
    sampleCount: 3,
  }),
  create(SurveySummarySchema, {
    id: 'svy-3',
    name: 'Clinic east wing',
    status: 'paused',
    floorCount: 1,
  }),
];

const meta = {
  title: 'Surveys/SurveyList',
  component: SurveyList,
  args: { surveys, selectedId: 'svy-2', onSelect: fn() },
  decorators: [
    (Story) => (
      <aside className="panel w-72">
        <Story />
      </aside>
    ),
  ],
} satisfies Meta<typeof SurveyList>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Populated: Story = {};

export const Empty: Story = {
  args: { surveys: [], selectedId: undefined },
};
