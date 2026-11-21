import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import i18n from '@/i18n';
import { LanguageSwitcher } from './LanguageSwitcher';

afterEach(async () => {
  await i18n.changeLanguage('en');
});

describe('LanguageSwitcher', () => {
  it('offers English and Español with the current language selected', () => {
    render(<LanguageSwitcher />);

    const select = screen.getByRole('combobox', { name: 'Language' });
    expect(select).toHaveValue('en');
    expect([...select.querySelectorAll('option')].map((option) => option.textContent)).toEqual([
      'English',
      'Español',
    ]);
  });

  it('switches the interface language and remembers the choice', async () => {
    render(<LanguageSwitcher />);

    fireEvent.change(screen.getByTestId('language-switcher'), { target: { value: 'es' } });

    await waitFor(() => expect(i18n.resolvedLanguage).toBe('es'));
    // The label itself is translated, which is what shows the switch took.
    expect(await screen.findByRole('combobox', { name: 'Idioma' })).toHaveValue('es');
    expect(localStorage.getItem('language')).toBe('es');
  });

  it('shows two-letter codes when the rail is collapsed', () => {
    render(<LanguageSwitcher compact />);

    const select = screen.getByTestId('language-switcher');
    expect([...select.querySelectorAll('option')].map((option) => option.textContent)).toEqual([
      'EN',
      'ES',
    ]);
  });
});
