/**
 * A path that matches no route used to render the same friendly "not built
 * yet" panel every unbuilt rail item did, so a typo in the address bar looked
 * like a feature on its way. Every rail entry now has a page, which makes an
 * unmatched path a mistake rather than a promise.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it, vi } from 'vitest';
import { App } from './App';

vi.mock('@/lib/client', () => ({
  surveyClient: {
    listSurveys: vi.fn().mockResolvedValue({ surveys: [] }),
    getCaptureCapability: vi.fn().mockResolvedValue({ available: true, reason: '', remedy: '' }),
  },
}));

function renderAt(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('App routing', () => {
  it('says a mistyped address is not a page, with a way back', async () => {
    renderAt('/interferance');

    const panel = await screen.findByTestId('not-found');
    expect(panel).toHaveTextContent(/not found/i);
    // The rail also links to Surveys, so the way back is looked for inside the
    // panel — the point is that the page itself offers one.
    expect(within(panel).getByRole('link', { name: /surveys/i })).toHaveAttribute('href', '/');
  });
});
