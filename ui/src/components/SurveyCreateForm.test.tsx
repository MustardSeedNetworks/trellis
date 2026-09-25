/**
 * A survey that could not be opened has to say why to whoever is on the field,
 * not only to whoever is looking at the paragraph under it.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SurveyCreateForm } from './SurveyCreateForm';

const createSurvey = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    createSurvey: (req: unknown) => createSurvey(req),
  },
}));

function renderForm() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  }
  return render(<SurveyCreateForm onCreated={vi.fn()} />, { wrapper });
}

beforeEach(() => {
  createSurvey.mockReset();
});

describe('SurveyCreateForm', () => {
  it('links a failed create to both fields and keeps the interface hint', async () => {
    createSurvey.mockRejectedValue(new Error('interface wlan9 not found'));
    renderForm();

    const name = screen.getByTestId('new-survey-name');
    const iface = screen.getByTestId('new-survey-interface');
    expect(name).not.toHaveAttribute('aria-describedby');
    const hint = iface.getAttribute('aria-describedby') ?? '';
    const hintText = document.getElementById(hint)?.textContent ?? '';
    expect(hintText).not.toBe('');

    fireEvent.change(name, { target: { value: 'Store 12' } });
    fireEvent.change(iface, { target: { value: 'wlan9' } });
    fireEvent.click(screen.getByTestId('create-survey'));

    await screen.findByTestId('create-survey-error');
    expect(name).toHaveAccessibleDescription(/interface wlan9 not found/);
    // The hint still reads first: it is what the field is for, the error is
    // what went wrong with it this time.
    expect(iface).toHaveAccessibleDescription(
      expect.stringMatching(new RegExp(`^${hintText}.*interface wlan9 not found`)),
    );
  });
});
