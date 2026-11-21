/**
 * The phone shell: a top bar in place of the rail, and a drawer behind it.
 * jsdom draws no CSS, so these assert what the drawer is made of and when it
 * is open — the visual width is the e2e's and the screenshots' to prove.
 */
import { fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useNavigate } from 'react-router';
import { describe, expect, it } from 'vitest';
import { MobileNav } from './MobileNav';

function GoBack() {
  const navigate = useNavigate();
  return (
    <button type="button" data-testid="history-back" onClick={() => void navigate(-1)}>
      back
    </button>
  );
}

function renderNav() {
  return render(
    <MemoryRouter initialEntries={['/', '/coverage']} initialIndex={1}>
      <MobileNav version="1.2.3" />
      <GoBack />
      <Routes>
        <Route path="*" element={<div data-testid="page" />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('MobileNav', () => {
  it('draws a top bar that names the product and keeps the drawer closed', () => {
    renderNav();

    expect(screen.getByTestId('mobile-top-bar')).toHaveTextContent('Trellis');
    expect(screen.getByTestId('mobile-menu-toggle')).toHaveAttribute('aria-expanded', 'false');
    expect(screen.getByTestId('mobile-menu-toggle')).toHaveAccessibleName('Open menu');
    // Nothing is mounted in a closed drawer: no second copy of the controls.
    expect(screen.queryByTestId('sidebar-mobile')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Reports' })).not.toBeInTheDocument();
  });

  it('opens a drawer holding the rail and closes it from the toggle', () => {
    renderNav();

    fireEvent.click(screen.getByTestId('mobile-menu-toggle'));
    const drawer = screen.getByTestId('sidebar-mobile');
    expect(screen.getByTestId('mobile-menu-toggle')).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByTestId('mobile-menu-toggle')).toHaveAccessibleName('Close menu');
    for (const name of ['Surveys', 'Import', 'Coverage', 'Live', 'Reports']) {
      expect(within(drawer).getByRole('link', { name })).toBeInTheDocument();
    }

    fireEvent.click(screen.getByTestId('mobile-menu-toggle'));
    expect(screen.queryByTestId('sidebar-mobile')).not.toBeInTheDocument();
  });

  it('closes from the scrim and from Escape', () => {
    renderNav();

    fireEvent.click(screen.getByTestId('mobile-menu-toggle'));
    fireEvent.click(screen.getByTestId('mobile-scrim'));
    expect(screen.queryByTestId('sidebar-mobile')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('mobile-menu-toggle'));
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByTestId('sidebar-mobile')).not.toBeInTheDocument();
  });

  it('closes when a destination is chosen', () => {
    renderNav();

    fireEvent.click(screen.getByTestId('mobile-menu-toggle'));
    fireEvent.click(
      within(screen.getByTestId('sidebar-mobile')).getByRole('link', { name: 'Live' }),
    );

    expect(screen.queryByTestId('sidebar-mobile')).not.toBeInTheDocument();
  });

  it('closes when the page already open is chosen again, where the path does not change', () => {
    renderNav();

    fireEvent.click(screen.getByTestId('mobile-menu-toggle'));
    fireEvent.click(
      within(screen.getByTestId('sidebar-mobile')).getByRole('link', { name: 'Coverage' }),
    );

    expect(screen.queryByTestId('sidebar-mobile')).not.toBeInTheDocument();
  });

  it('closes on a navigation it did not start, such as browser back', () => {
    renderNav();

    fireEvent.click(screen.getByTestId('mobile-menu-toggle'));
    expect(screen.getByTestId('sidebar-mobile')).toBeInTheDocument();

    fireEvent.click(screen.getByTestId('history-back'));
    expect(screen.queryByTestId('sidebar-mobile')).not.toBeInTheDocument();
  });
});
