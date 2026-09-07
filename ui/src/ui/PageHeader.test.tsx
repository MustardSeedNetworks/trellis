/**
 * The shell's header strip carried no test: every optional slot — breadcrumbs,
 * the help entry point, the secondary readout — was a branch nothing had ever
 * taken. The two that matter are the ones a page can get wrong by omission:
 * the (?) button must not appear on a page with no help content behind it, and
 * a breadcrumb without an href must not look like a link.
 */
import { render, screen, within } from '@testing-library/react';
import { Radar } from 'lucide-react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it, vi } from 'vitest';
import { PageHeader } from './PageHeader';

function renderHeader(props: Parameters<typeof PageHeader>[0]) {
  return render(
    <MemoryRouter>
      <PageHeader {...props} />
    </MemoryRouter>,
  );
}

describe('PageHeader', () => {
  it('shows no help entry point when the page has no help behind it', () => {
    renderHeader({ title: 'Coverage' });

    expect(screen.queryByRole('button', { name: /open help/i })).not.toBeInTheDocument();
    expect(screen.queryByTestId('page-header-eyebrow')).not.toBeInTheDocument();
    expect(screen.queryByTestId('page-header-secondary')).not.toBeInTheDocument();
  });

  it('offers help on the page it was given help for', async () => {
    const onHelp = vi.fn();
    const { container } = renderHeader({
      title: 'Live',
      eyebrow: 'Analysis',
      description: 'Neighbour APs in range',
      icon: Radar,
      secondary: 'Last scan 41s',
      actions: <button type="button">Pause</button>,
      onHelp,
    });

    screen.getByRole('button', { name: 'Open help for Live' }).click();
    expect(onHelp).toHaveBeenCalledOnce();
    expect(screen.getByTestId('page-header-secondary')).toHaveTextContent('Last scan 41s');
    expect(screen.getByRole('button', { name: 'Pause' })).toBeInTheDocument();
    expect(screen.getByText('Neighbour APs in range')).toBeInTheDocument();
    // The icon is decorative; it is the one prop with no accessible name, so
    // it is checked as the drawing it is.
    expect(container.querySelector('svg.h-8')).not.toBeNull();
  });

  it('links the breadcrumbs that have somewhere to go and not the one that does not', () => {
    renderHeader({
      title: 'Everett HQ',
      breadcrumbs: [{ label: 'Surveys', href: '/' }, { label: 'Everett HQ' }],
    });

    const trail = screen.getByRole('navigation', { name: 'Breadcrumb' });
    expect(within(trail).getByRole('link', { name: 'Surveys' })).toHaveAttribute('href', '/');
    expect(within(trail).queryByRole('link', { name: 'Everett HQ' })).not.toBeInTheDocument();
    expect(within(trail).getByText('Everett HQ')).toBeInTheDocument();
  });
});
