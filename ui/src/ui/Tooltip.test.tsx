import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Tooltip } from './Tooltip';

function renderTooltip(enabled = true) {
  return render(
    <Tooltip content="Survey help" enabled={enabled}>
      {(props) => (
        <button {...props} type="button">
          Help
        </button>
      )}
    </Tooltip>,
  );
}

afterEach(() => vi.restoreAllMocks());

describe('Tooltip', () => {
  it('repositions when translated content grows near the viewport edge', async () => {
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (
      this: HTMLElement,
    ) {
      if (this.getAttribute('role') === 'tooltip') {
        return new DOMRect(0, 0, this.textContent === 'Short' ? 100 : 300, 40);
      }
      return new DOMRect(window.innerWidth - 50, window.innerHeight - 20, 40, 20);
    });
    const hint = (content: string) => (
      <Tooltip content={content}>
        {(props) => (
          <button {...props} type="button">
            Help
          </button>
        )}
      </Tooltip>
    );
    const { rerender } = render(hint('Short'));
    await userEvent.setup().hover(screen.getByRole('button'));
    expect(screen.getByRole('tooltip')).toHaveStyle({
      left: `${window.innerWidth - 108}px`,
      top: `${window.innerHeight - 60}px`,
    });
    rerender(hint('A longer translated label'));
    expect(screen.getByRole('tooltip')).toHaveStyle({ left: `${window.innerWidth - 308}px` });
  });

  it('describes its trigger on hover and stays readable over the tooltip', async () => {
    const user = userEvent.setup();
    renderTooltip();
    const trigger = screen.getByRole('button');
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
    await user.hover(trigger);
    const tooltip = screen.getByRole('tooltip');
    expect(trigger).toHaveAttribute('aria-describedby', tooltip.id);
    expect(tooltip).toHaveTextContent('Survey help');
    // user-event's mouseout omits relatedTarget; the browser supplies the tooltip.
    fireEvent.mouseOut(trigger, { relatedTarget: tooltip });
    expect(tooltip).toBeInTheDocument();
    fireEvent.mouseOut(tooltip, { relatedTarget: document.body });
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('opens on keyboard focus, dismisses on Escape, and reopens on a new focus', async () => {
    const user = userEvent.setup();
    renderTooltip();
    await user.tab();
    expect(screen.getByRole('tooltip')).toBeInTheDocument();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
    expect(screen.getByRole('button')).not.toHaveAttribute('aria-describedby');
    await user.tab();
    await user.tab({ shift: true });
    expect(screen.getByRole('tooltip')).toBeInTheDocument();
    await user.tab();
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('dismisses a hovered tooltip with Escape even without trigger focus', async () => {
    const user = userEvent.setup();
    renderTooltip();
    await user.hover(screen.getByRole('button'));
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it.each(['scroll', 'resize'])('dismisses when %s moves the trigger', async (event) => {
    const user = userEvent.setup();
    renderTooltip();
    await user.hover(screen.getByRole('button'));
    fireEvent(window, new Event(event));
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('does not describe a disabled tooltip on hover or focus', async () => {
    const user = userEvent.setup();
    renderTooltip(false);
    await user.hover(screen.getByRole('button'));
    await user.tab();
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
    expect(screen.getByRole('button')).not.toHaveAttribute('aria-describedby');
  });
});
