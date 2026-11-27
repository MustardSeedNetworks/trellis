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
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ConnectionState } from '../hooks/useConnectionStatus';
import { Sidebar } from './Sidebar';

const STORAGE_KEY = 'trellis-sidebar-collapsed';

interface Handlers {
  onOpenHelp?: () => void;
  onOpenSettings?: () => void;
  connection?: ConnectionState;
}

function renderSidebar(variant?: 'rail' | 'drawer', handlers: Handlers = {}) {
  return render(
    <MemoryRouter>
      <Sidebar
        version="v1.2.3-4-gabcdef-dirty"
        variant={variant}
        connection={handlers.connection ?? 'connected'}
        onOpenHelp={handlers.onOpenHelp ?? (() => undefined)}
        onOpenSettings={handlers.onOpenSettings ?? (() => undefined)}
      />
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
      expect(await screen.findByRole('tooltip', { name })).toBeInTheDocument();
      await user.unhover(link);
    }
  });

  it('is hidden below lg and expanded on a desktop that has never been collapsed', () => {
    renderSidebar();

    const rail = screen.getByTestId('sidebar');
    expect(rail.className).toContain('hidden');
    expect(rail.className).toContain('lg:flex');
    expect(rail.className).toContain('w-56');
    expect(screen.getByTestId('rail-collapse')).toBeInTheDocument();
  });

  it('remembers the operator’s own collapse across a mount', async () => {
    const user = userEvent.setup();
    const { unmount } = renderSidebar();

    await user.click(screen.getByTestId('rail-collapse'));
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
    expect(screen.queryByTestId('rail-collapse')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Reports' })).toBeInTheDocument();
  });

  it('ignores the collapse the operator chose on the desktop rail', () => {
    localStorage.setItem(STORAGE_KEY, 'true');

    renderSidebar('drawer');

    expect(screen.getByTestId('sidebar-mobile').className).toContain('w-72');
    expect(localStorage.getItem(STORAGE_KEY)).toBe('true');
  });
});

describe('Sidebar footer', () => {
  it('asks the shell to open Settings and the current page help', async () => {
    const user = userEvent.setup();
    const onOpenSettings = vi.fn();
    const onOpenHelp = vi.fn();
    renderSidebar('rail', { onOpenSettings, onOpenHelp });

    await user.click(screen.getByTestId('rail-settings'));
    await user.click(screen.getByTestId('rail-help'));

    expect(onOpenSettings).toHaveBeenCalledOnce();
    expect(onOpenHelp).toHaveBeenCalledOnce();
  });

  it('keeps the language selector and the theme words out of the rail', () => {
    renderSidebar();

    expect(screen.queryByTestId('language-switcher')).not.toBeInTheDocument();
    expect(screen.queryByText(/Theme:/)).not.toBeInTheDocument();
  });

  it('says whether the daemon answers, and shows the version as v<semver>', () => {
    const { unmount } = renderSidebar('rail', { connection: 'disconnected' });
    expect(screen.getByTestId('rail-status')).toHaveTextContent('Disconnected');
    expect(screen.getByTestId('rail-version')).toHaveTextContent('v1.2.3');
    expect(screen.getByTestId('rail-version')).not.toHaveTextContent('dirty');
    unmount();

    renderSidebar('rail', { connection: 'connected' });
    expect(screen.getByTestId('rail-status')).toHaveTextContent('Connected');
  });

  it('takes no verdict until the first probe answers', () => {
    renderSidebar('rail', { connection: 'checking' });
    expect(screen.queryByTestId('rail-status')).not.toBeInTheDocument();
  });

  it('offers no sign-out on a daemon that asked for no session', () => {
    renderSidebar();

    expect(screen.queryByTestId('rail-sign-out')).not.toBeInTheDocument();
  });

  it('flips light and dark from the quick toggle, naming where a click goes', async () => {
    const user = userEvent.setup();
    localStorage.setItem('trellis-theme', 'light');
    renderSidebar();

    const toggle = screen.getByTestId('rail-theme');
    expect(toggle).toHaveAccessibleName('Switch to dark theme');
    await user.click(toggle);

    expect(toggle).toHaveAccessibleName('Switch to light theme');
    expect(localStorage.getItem('trellis-theme')).toBe('dark');
  });

  it('draws the drawer variant with no collapse control', () => {
    renderSidebar('drawer');

    expect(screen.getByTestId('rail-footer')).toBeInTheDocument();
    expect(screen.queryByTestId('rail-collapse')).not.toBeInTheDocument();
  });
});
