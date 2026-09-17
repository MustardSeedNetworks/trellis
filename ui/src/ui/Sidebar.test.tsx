/**
 * The rail's responsive behaviour (trellis#473).
 *
 * The interesting part is not that a phone collapses the rail — the E2E proves
 * that against a real layout — but that the forced collapse must not be
 * MISTAKEN for the operator's own choice. `collapsed` was persisted on every
 * change, so a width-driven collapse would have written "true" and left the
 * rail collapsed on the desktop the survey is written up on.
 */
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { setViewportNarrow } from '@/test/matchMedia';
import { Sidebar } from './Sidebar';

const STORAGE_KEY = 'trellis-sidebar-collapsed';

function renderSidebar() {
  return render(
    <MemoryRouter>
      <Sidebar version="1.2.3" />
    </MemoryRouter>,
  );
}

beforeEach(() => localStorage.clear());
afterEach(() => setViewportNarrow(false));

describe('Sidebar at phone width', () => {
  it('names every collapsed link without relying on a native title tooltip', async () => {
    const user = userEvent.setup();
    setViewportNarrow(true);
    renderSidebar();
    for (const name of ['Surveys', 'Import', 'Coverage', 'Live', 'Reports']) {
      const link = screen.getByRole('link', { name });
      expect(link).not.toHaveAttribute('title');
      await user.hover(link);
      expect(screen.getByRole('tooltip')).toHaveTextContent(name);
      await user.unhover(link);
    }
  });

  it('collapses without recording a preference the operator never set', () => {
    setViewportNarrow(true);

    renderSidebar();

    expect(screen.getByTestId('sidebar').className).toContain('w-16');
    expect(localStorage.getItem(STORAGE_KEY)).toBe('false');
  });

  it('does not offer the collapse control, which would undo the layout', () => {
    setViewportNarrow(true);

    renderSidebar();

    expect(screen.queryByTestId('sidebar-collapse')).not.toBeInTheDocument();
  });

  it('keeps an expanded rail on a desktop that has never been collapsed', () => {
    renderSidebar();

    expect(screen.getByTestId('sidebar').className).toContain('w-56');
    expect(screen.getByTestId('sidebar-collapse')).toBeInTheDocument();
  });

  it('remembers the operator’s own collapse across a mount', async () => {
    const user = userEvent.setup();
    const { unmount } = renderSidebar();

    await user.click(screen.getByTestId('sidebar-collapse'));
    expect(localStorage.getItem(STORAGE_KEY)).toBe('true');

    unmount();
    renderSidebar();
    expect(screen.getByTestId('sidebar').className).toContain('w-16');
  });
});
