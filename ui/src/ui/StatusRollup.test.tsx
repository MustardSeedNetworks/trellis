import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import i18n from '@/i18n';
import { StatusRollup } from './StatusRollup';

describe('StatusRollup', () => {
  it('exposes its state so the theme gate and the page can both read it', () => {
    const { container } = render(<StatusRollup state="warn" headline="Loss above threshold" />);
    expect(container.querySelector('[data-state="warn"]')).not.toBeNull();
  });

  it('prints figures when it has data', () => {
    render(
      <StatusRollup
        state="ok"
        headline="Nothing is wrong right now"
        figures={[{ label: 'Frames', value: '1.2M' }]}
      />,
    );
    expect(screen.getByText('1.2M')).toBeInTheDocument();
  });

  /**
   * The one bug this component exists to prevent. A rollup that renders zeros
   * when its source failed says "nothing is wrong" at the moment nobody can
   * tell, so the unknown state must not be able to show a figure even when a
   * caller passes one.
   */
  it('refuses to print figures it does not have', () => {
    render(
      <StatusRollup
        state="unknown"
        headline="Health data is not arriving"
        figures={[
          { label: 'Frames', value: '0' },
          { label: 'Loss', value: '0%' },
        ]}
      />,
    );
    expect(screen.queryByText('0')).not.toBeInTheDocument();
    expect(screen.queryByText('0%')).not.toBeInTheDocument();
    expect(screen.getAllByText('—')).toHaveLength(2);
    // Labels survive: the reader still learns what is missing.
    expect(screen.getByText('Frames')).toBeInTheDocument();
  });

  /**
   * The kicker sits on the section's wash, which the per-line pill gate cannot
   * see because the two classes live in different fields of STATE_STYLES. A
   * hue's own wash pulls the ground toward it (pillText.test.ts), so washed
   * states must name the -strong token that is measured on that wash.
   */
  it.each([
    ['warn', 'status.warn'],
    ['crit', 'status.crit'],
  ] as const)('%s kicker uses the text token measured on its wash', (state, key) => {
    render(<StatusRollup state={state} headline="Headline" />);
    const wash = screen.getByTestId('status-rollup').className.match(/\bbg-([\w-]+)\/\d+\b/);
    expect(wash, 'washed state lost its wash').not.toBeNull();
    expect(screen.getByText(i18n.t(key)).className).toContain(`text-${wash?.[1]}-strong`);
  });

  it('never reports unknown as ok', () => {
    const { container } = render(
      <StatusRollup state="unknown" headline="Health data is not arriving" />,
    );
    expect(container.querySelector('[data-state="ok"]')).toBeNull();
    expect(screen.getByText(/not arriving/i)).toBeInTheDocument();
  });

  it('shows at most four figures, because more stops being a summary', () => {
    render(
      <StatusRollup
        state="ok"
        headline="Nothing is wrong right now"
        figures={[
          { label: 'A', value: '1' },
          { label: 'B', value: '2' },
          { label: 'C', value: '3' },
          { label: 'D', value: '4' },
          { label: 'E', value: '5' },
        ]}
      />,
    );
    expect(screen.queryByText('E')).not.toBeInTheDocument();
    expect(screen.getByText('D')).toBeInTheDocument();
  });
});
