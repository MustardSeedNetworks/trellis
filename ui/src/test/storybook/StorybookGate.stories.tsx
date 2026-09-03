import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, fn, userEvent, within } from 'storybook/test';

/**
 * The gate's own gate.
 *
 * A story harness that has never been shown to fail is not worth trusting.
 * These two stories render normally in CI and pass; the contract script
 * re-runs them with VITE_STORYBOOK_INJECT_DEFECT set and requires the suite to
 * go red, once for a broken interaction and once for an accessibility
 * violation. No local a11y override: depending on the global setting means
 * turning the gate back down fails this script.
 */
type Defect = 'accessibility' | 'interaction';

interface SharedControlsProps {
  defect?: Defect;
  onSave: () => void;
}

function SharedControls({ defect, onSave }: SharedControlsProps) {
  return (
    <form className="flex flex-col gap-3" onSubmit={(event) => event.preventDefault()}>
      <label className="flex flex-col gap-1 text-sm" htmlFor="gate-target">
        <span className="text-text-secondary">Target address</span>
        <input
          id="gate-target"
          type="text"
          defaultValue="192.0.2.1"
          className="rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary"
        />
      </label>
      {/* Disabled swallows the click the play function asserts on. */}
      <button
        type="button"
        disabled={defect === 'interaction'}
        onClick={onSave}
        className="w-fit rounded bg-brand-primary px-3 py-2 text-sm font-medium text-on-brand"
      >
        {/* aria-hidden on the only label leaves the button with no discernible text. */}
        <span aria-hidden={defect === 'accessibility'}>Save target</span>
      </button>
    </form>
  );
}

const injectedDefect = import.meta.env.VITE_STORYBOOK_INJECT_DEFECT as Defect | undefined;

const meta = {
  title: 'Test/Storybook gate',
  component: SharedControls,
} satisfies Meta<typeof SharedControls>;

export default meta;
type Story = StoryObj<typeof meta>;

export const SharedComponentInteraction: Story = {
  args: {
    defect: injectedDefect === 'interaction' ? injectedDefect : undefined,
    onSave: fn(),
  },
  play: async ({ args, canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole('button', { name: 'Save target' }));
    await expect(args.onSave).toHaveBeenCalledOnce();
  },
};

export const SharedComponentAccessibility: Story = {
  args: {
    defect: injectedDefect === 'accessibility' ? injectedDefect : undefined,
    onSave: fn(),
  },
};
