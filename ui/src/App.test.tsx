/**
 * A path that matches no route used to render the same friendly "not built
 * yet" panel every unbuilt rail item did, so a typo in the address bar looked
 * like a feature on its way. Every rail entry now has a page, which makes an
 * unmatched path a mistake rather than a promise.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ToastProvider } from '@/ui/Toast';
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
      <ToastProvider>
        <MemoryRouter initialEntries={[path]}>
          <App />
        </MemoryRouter>
      </ToastProvider>
    </QueryClientProvider>,
  );
}

afterEach(() => vi.unstubAllGlobals());

describe('App routing', () => {
  it('says a mistyped address is not a page, with a way back', async () => {
    renderAt('/interferance');

    const panel = await screen.findByTestId('not-found');
    expect(panel).toHaveTextContent(/not found/i);
    // A bare "Page not found" could be anyone's; this one says whose.
    expect(panel).toHaveTextContent('Trellis');
    // The rail also links to Surveys, so the way back is looked for inside the
    // panel — the point is that the page itself offers one.
    expect(within(panel).getByRole('link', { name: /surveys/i })).toHaveAttribute('href', '/');
  });
});

describe('App shell', () => {
  it('resolves a trailing-slash route to its page title', async () => {
    renderAt('/import/');
    expect(await screen.findByTestId('page-help-open')).toHaveAccessibleName(
      'Open help for Import',
    );
    expect(document.title).toBe('Import | Trellis');
  });

  it('offers page help, sets the document title, and places skip navigation first', async () => {
    const { container } = renderAt('/');
    expect(await screen.findByTestId('page-help-open')).toHaveAccessibleName(
      'Open help for Surveys',
    );
    expect(document.title).toBe('Surveys | Trellis');
    const skip = screen.getByTestId('skip-to-content');
    expect(container.querySelector('a,button,input,select,textarea')).toBe(skip);
    expect(skip).toHaveAttribute('href', '#main-content');
    expect(screen.getByRole('main')).toHaveAttribute('tabindex', '-1');
  });

  it('wears the header the registry gives the route, above the page body', async () => {
    renderAt('/');

    // The header strip is rendered by the shell from the registry entry, not
    // by the page, so a page that never rendered its own title still gets one.
    expect(await screen.findByTestId('page-header-title')).toHaveTextContent(/surveys/i);
    expect(screen.getByTestId('page-header-eyebrow')).toHaveTextContent(/capture/i);
    expect(await screen.findByRole('heading', { name: /surveys/i })).toBeInTheDocument();
  });
});

describe('App daemon status', () => {
  it("shows the running daemon's version in the rail, not the bundle's", async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValue(
          new Response(
            JSON.stringify({ version: '9.8.7', commit: 'c', buildTime: 't', uiBuildHash: 'h' }),
            { status: 200, headers: { 'Content-Type': 'application/json' } },
          ),
        ),
    );
    renderAt('/');

    expect(await within(screen.getByTestId('sidebar')).findByText('9.8.7')).toBeInTheDocument();
    expect(screen.queryByTestId('daemon-unreachable-banner')).not.toBeInTheDocument();
  });

  it('says so above the page when the daemon stops answering', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network')));
    renderAt('/');

    expect(await screen.findByTestId('daemon-unreachable-banner')).toHaveTextContent(
      "Can't reach the Trellis daemon",
    );
  });

  it('draws the phone top bar and the language selector in the shell', async () => {
    renderAt('/');

    expect(await screen.findByTestId('mobile-menu-toggle')).toBeInTheDocument();
    expect(
      within(screen.getByTestId('sidebar')).getByTestId('language-switcher'),
    ).toBeInTheDocument();
  });
});
