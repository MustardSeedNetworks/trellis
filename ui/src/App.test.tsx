/**
 * A path that matches no route used to render the same friendly "not built
 * yet" panel every unbuilt rail item did, so a typo in the address bar looked
 * like a feature on its way. Every rail entry now has a page, which makes an
 * unmatched path a mistake rather than a promise.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ToastProvider } from '@/ui/Toast';
import { App } from './App';

vi.mock('@/lib/client', () => ({
  daemonClient: {
    getConfig: vi.fn().mockResolvedValue({
      listenAddr: '127.0.0.1:8446',
      dataDir: '/data',
      captureMode: 'scan',
      tlsEnabled: false,
      authRequired: false,
      logFile: '',
    }),
  },
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
    expect(within(panel).getByRole('link', { name: /surveys/i })).toBeInTheDocument();
  });

  it('gives the 404 the same header as every page, neutral and with a trail', async () => {
    renderAt('/interferance');

    const panel = await screen.findByTestId('not-found');
    expect(within(panel).getByTestId('page-header-eyebrow')).toHaveTextContent('Trellis');
    expect(within(panel).getByTestId('page-header-title')).toHaveTextContent('Page not found');
    expect(within(panel).getByTestId('breadcrumbs')).toHaveTextContent('Trellis');
    expect(document.title).toBe('Page not found \u00b7 Trellis');
  });
});

describe('App shell', () => {
  it('resolves a trailing-slash route to its page title', async () => {
    renderAt('/import/');
    expect(await screen.findByTestId('page-header-help-button')).toHaveAccessibleName(
      'Open help for Import',
    );
    expect(document.title).toBe('Import \u00b7 Trellis');
  });

  it('offers page help, sets the document title, and places skip navigation first', async () => {
    const { container } = renderAt('/');
    expect(await screen.findByTestId('page-header-help-button')).toHaveAccessibleName(
      'Open help for Surveys',
    );
    expect(document.title).toBe('Surveys \u00b7 Trellis');
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
    // Every page carries a trail that ends on itself, as plain text.
    const trail = screen.getByTestId('breadcrumbs');
    expect(within(trail).getByRole('link', { name: 'Trellis' })).toHaveAttribute('href', '/');
    expect(within(trail).getByText('Surveys')).toHaveAttribute('aria-current', 'page');
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

    expect(await within(screen.getByTestId('sidebar')).findByText('v9.8.7')).toBeInTheDocument();
    expect(screen.queryByTestId('daemon-unreachable-banner')).not.toBeInTheDocument();
  });

  it('says so above the page when the daemon stops answering', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network')));
    renderAt('/');

    expect(await screen.findByTestId('daemon-unreachable-banner')).toHaveTextContent(
      "Can't reach the Trellis daemon",
    );
  });

  it('draws the phone top bar and opens Settings from the rail', async () => {
    renderAt('/');

    expect(await screen.findByTestId('mobile-menu-toggle')).toBeInTheDocument();
    await userEvent.click(within(screen.getByTestId('sidebar')).getByTestId('rail-settings'));

    const drawer = screen.getByTestId('settings-drawer');
    expect(within(drawer).getByRole('tab', { name: 'Appearance' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(within(drawer).getByTestId('language-switcher')).toBeInTheDocument();
    expect(within(drawer).getByRole('radio', { name: 'System' })).toBeChecked();

    await userEvent.click(within(drawer).getByRole('tab', { name: 'Daemon' }));
    expect(
      await within(screen.getByTestId('settings-daemon')).findByText('127.0.0.1:8446'),
    ).toBeInTheDocument();

    await userEvent.click(within(drawer).getByRole('tab', { name: 'About' }));
    expect(within(screen.getByTestId('settings-about')).getByText('Version')).toBeInTheDocument();
  });

  it('opens page help in the shared right drawer and closes it with Escape', async () => {
    renderAt('/');

    await userEvent.click(await screen.findByTestId('page-header-help-button'));
    const drawer = screen.getByTestId('page-help-drawer');
    expect(drawer).toHaveAccessibleName('Help: Surveys');
    expect(within(drawer).getByTestId('page-help-copy')).not.toBeEmptyDOMElement();

    await userEvent.keyboard('{Escape}');
    expect(screen.queryByTestId('page-help-drawer')).not.toBeInTheDocument();
  });
});
