/**
 * Two points and a distance is what the RPC has always taken. The first UI for
 * it asked for the width of the whole plan and sent (0,0)→(width,0), which is
 * wrong twice: a plan is usually cropped or padded, so its outer edge is not a
 * wall anyone can pace, and the operator is asked for a number they cannot
 * check. These pin the interaction that replaces it — including from the
 * keyboard, because a calibration only a pointer can perform puts the scale,
 * and every distance derived from it, out of reach.
 */
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { PlanCalibrator } from './PlanCalibrator';

const plan = { width: 800, height: 600, imageUrl: 'data:image/png;base64,AAA' };

function renderCalibrator(onApply = vi.fn()) {
  render(<PlanCalibrator plan={plan} onApply={onApply} pending={false} />);
  const surface = screen.getByTestId('calibration-surface');
  // jsdom gives every element a zero-sized box, so the click mapping needs a
  // real one to divide by.
  vi.spyOn(surface, 'getBoundingClientRect').mockReturnValue({
    left: 0,
    top: 0,
    width: 400,
    height: 300,
    right: 400,
    bottom: 300,
    x: 0,
    y: 0,
    toJSON: () => ({}),
  });
  return { surface, onApply };
}

/** A click at CSS pixels (x, y) on a surface drawn at half the plan's size. */
function clickAt(surface: HTMLElement, x: number, y: number) {
  fireEvent.click(surface, { clientX: x, clientY: y, detail: 1 });
}

describe('PlanCalibrator', () => {
  it('asks for the first mark before anything is placed', () => {
    renderCalibrator();

    expect(screen.getByTestId('calibration-prompt')).toHaveTextContent(/mark one end/i);
    expect(screen.getByTestId('calibrate-floor-plan')).toBeDisabled();
  });

  it('sends the two marked points in the plan’s own pixels', () => {
    const { surface, onApply } = renderCalibrator();

    clickAt(surface, 50, 100); // → 100, 200 in plan pixels
    clickAt(surface, 150, 100); // → 300, 200
    fireEvent.change(screen.getByTestId('calibration-metres'), { target: { value: '12.5' } });
    fireEvent.click(screen.getByTestId('calibrate-floor-plan'));

    expect(onApply).toHaveBeenCalledWith({
      from: { x: 100, y: 200 },
      to: { x: 300, y: 200 },
      metres: 12.5,
    });
  });

  it('reports the pixels between the marks, so the operator can sanity-check the line', () => {
    const { surface } = renderCalibrator();

    clickAt(surface, 50, 100);
    clickAt(surface, 50, 250); // 300 px down the plan

    expect(screen.getByTestId('calibration-prompt')).toHaveTextContent('300 px');
    expect(screen.getByTestId('calibration-line')).toBeInTheDocument();
  });

  it('starts a new line on a third mark rather than extending the old one', () => {
    const { surface, onApply } = renderCalibrator();

    clickAt(surface, 50, 100);
    clickAt(surface, 150, 100);
    clickAt(surface, 20, 20); // → 40, 40: a fresh first mark
    expect(screen.getByTestId('calibration-prompt')).toHaveTextContent(/mark the other end/i);
    expect(screen.getByTestId('calibrate-floor-plan')).toBeDisabled();

    clickAt(surface, 20, 60); // → 40, 120
    fireEvent.click(screen.getByTestId('calibrate-floor-plan'));
    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({ from: { x: 40, y: 40 }, to: { x: 40, y: 120 } }),
    );
  });

  it('places both marks from the keyboard alone', () => {
    const { surface, onApply } = renderCalibrator();

    surface.focus();
    // The cursor starts at the middle of the plan: 400, 300.
    fireEvent.keyDown(surface, { key: 'ArrowRight' });
    fireEvent.click(surface, { detail: 0 }); // keyboard activation
    fireEvent.keyDown(surface, { key: 'ArrowRight', shiftKey: true });
    fireEvent.click(surface, { detail: 0 });
    fireEvent.click(screen.getByTestId('calibrate-floor-plan'));

    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({ from: { x: 405, y: 300 }, to: { x: 430, y: 300 } }),
    );
  });

  it('refuses to apply a line with no length, whatever distance is typed', () => {
    const { surface, onApply } = renderCalibrator();

    clickAt(surface, 50, 100);
    clickAt(surface, 50, 100); // the same point twice

    expect(screen.getByTestId('calibrate-floor-plan')).toBeDisabled();
    expect(onApply).not.toHaveBeenCalled();
  });
});
