import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import i18n from '@/i18n';
import { CommandPalette } from './CommandPalette';

// cmdk scrolls the active item into view; jsdom implements no scrolling. The
// ResizeObserver it also needs comes from the global test setup.
beforeAll(() => {
  Element.prototype.scrollIntoView = () => {};
});
afterEach(async () => {
  await i18n.changeLanguage('en');
});

function Where() {
  return <p data-testid="where">{useLocation().pathname}</p>;
}

function renderPalette() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <CommandPalette />
      <Routes>
        <Route path="*" element={<Where />} />
      </Routes>
    </MemoryRouter>,
  );
}

function open(key: { metaKey?: boolean; ctrlKey?: boolean }) {
  fireEvent.keyDown(document, { key: 'k', ...key });
}

describe('CommandPalette', () => {
  it('is closed until Ctrl+K, and again on a second press', () => {
    renderPalette();
    expect(screen.queryByTestId('command-palette')).not.toBeInTheDocument();

    open({ ctrlKey: true });
    expect(screen.getByTestId('command-palette')).toBeInTheDocument();

    open({ ctrlKey: true });
    expect(screen.queryByTestId('command-palette')).not.toBeInTheDocument();
  });

  it('opens on Cmd+K too, and ignores a bare k', () => {
    renderPalette();

    fireEvent.keyDown(document, { key: 'k' });
    expect(screen.queryByTestId('command-palette')).not.toBeInTheDocument();

    open({ metaKey: true });
    expect(screen.getByRole('dialog', { name: 'Command palette' })).toBeInTheDocument();
  });

  it('lists every route the rail offers', () => {
    renderPalette();
    open({ ctrlKey: true });

    for (const name of ['Surveys', 'Import', 'Coverage', 'Live', 'Reports']) {
      expect(screen.getByRole('option', { name })).toBeInTheDocument();
    }
  });

  it('navigates to the chosen route and closes', () => {
    renderPalette();
    open({ ctrlKey: true });

    fireEvent.click(screen.getByTestId('palette-reports'));

    expect(screen.getByTestId('where')).toHaveTextContent('/reports');
    expect(screen.queryByTestId('command-palette')).not.toBeInTheDocument();
  });

  it('filters by what is typed and says when nothing matches', () => {
    renderPalette();
    open({ ctrlKey: true });
    const input = screen.getByPlaceholderText('Go to a page or run a command…');

    fireEvent.change(input, { target: { value: 'cover' } });
    expect(screen.getByRole('option', { name: 'Coverage' })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Import' })).not.toBeInTheDocument();

    fireEvent.change(input, { target: { value: 'zzzz' } });
    expect(screen.getByText('No matches')).toBeInTheDocument();
  });

  it('switches the language from the palette', async () => {
    renderPalette();
    open({ ctrlKey: true });

    fireEvent.click(screen.getByTestId('palette-language-es'));

    expect(await screen.findByTestId('where')).toBeInTheDocument();
    expect(i18n.resolvedLanguage).toBe('es');
    expect(screen.queryByTestId('command-palette')).not.toBeInTheDocument();
  });
});
