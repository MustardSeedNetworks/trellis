/**
 * The rail and its drawer variant (trellis#473).
 *
 * The rail is drawn from lg up; below it MobileNav opens the same contents as
 * a drawer. What matters here is what the variants must not share: the
 * operator's own collapse choice is remembered, and the drawer — always
 * expanded, with no collapse control — never records one.
 */
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it } from 'vitest';
import { Sidebar } from './Sidebar';

const STORAGE_KEY = 'trellis-sidebar-collapsed';

function renderSidebar(variant?: 'rail' | 'drawer') {
  return render(
    <MemoryRouter>
      <Sidebar version="1.2.3" variant={variant} />
    </MemoryRouter>,
  );
}

beforeEach(() => localStorage.clear());

describe('Sidebar rail', () => {
  it('names every collapsed link without relying on a native title tooltip', async () => {
    const user = userEvent.setup();
    localStorage.setItem(STORAGE_KEY, 'true');
    renderSidebar();
    for (const name of ['Surveys', 'Import', 'Coverage', 'Live', 'Reports']) {
      const link = screen.getByRole('link', { name });
      expect(link).not.toHaveAttribute('title');
      await user.hover(link);
      expect(screen.getByRole('tooltip')).toHaveTextContent(name);
      await user.unhover(link);
    }
  });

  it('is hidden below lg and expanded on a desktop that has never been collapsed', () => {
    renderSidebar();

    const rail = screen.getByTestId('sidebar');
    expect(rail.className).toContain('hidden');
    expect(rail.className).toContain('lg:flex');
    expect(rail.className).toContain('w-56');
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

describe('Sidebar drawer', () => {
  it('is expanded and offers no collapse control', () => {
    renderSidebar('drawer');

    const drawer = screen.getByTestId('sidebar-mobile');
    expect(drawer.className).toContain('w-72');
    expect(drawer.className).not.toContain('hidden');
    expect(screen.queryByTestId('sidebar-collapse')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Reports' })).toBeInTheDocument();
  });

  it('ignores the collapse the operator chose on the desktop rail', () => {
    localStorage.setItem(STORAGE_KEY, 'true');

    renderSidebar('drawer');

    expect(screen.getByTestId('sidebar-mobile').className).toContain('w-72');
    expect(localStorage.getItem(STORAGE_KEY)).toBe('true');
  });
});
