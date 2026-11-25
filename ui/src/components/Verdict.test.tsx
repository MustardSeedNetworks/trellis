import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Verdict } from './Verdict';

describe('Verdict', () => {
  it('exposes its state so the theme gate and the page can both read it', () => {
    render(<Verdict state="warn" headline="Loss above threshold" />);
    expect(screen.getByTestId('verdict')).toHaveAttribute('data-state', 'warn');
  });

  it('prints figures when it has data', () => {
    render(
      <Verdict state="ok" headline="Connected" figures={[{ label: 'Frames', value: '1.2M' }]} />,
    );
    expect(screen.getByText('1.2M')).toBeInTheDocument();
  });

  // The bug this component exists to prevent: a verdict that renders zeros when
  // its source failed says "nothing is wrong" at the moment nobody can tell.
  it('refuses to print figures it does not have', () => {
    render(
      <Verdict
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
    expect(screen.getByText('Frames')).toBeInTheDocument();
  });

  it('states its state in words, never in colour alone', () => {
    render(<Verdict state="crit" headline="Import failed" />);
    expect(screen.getByText('Critical')).toBeInTheDocument();
  });

  it('shows at most four figures, because more stops being a summary', () => {
    render(
      <Verdict
        state="ok"
        headline="Connected"
        figures={['A', 'B', 'C', 'D', 'E'].map((label, index) => ({
          label,
          value: String(index),
        }))}
      />,
    );
    expect(screen.queryByText('E')).not.toBeInTheDocument();
    expect(screen.getByText('D')).toBeInTheDocument();
  });
});
