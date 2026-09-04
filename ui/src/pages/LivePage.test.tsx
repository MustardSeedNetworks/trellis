/**
 * The page's job is a verdict on the link, so the tests are about which reading
 * it reaches from a given airspace — a table that renders every row and calls
 * every one of them healthy would look identical and be worthless.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LivePage } from './LivePage';

const scan = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    scan: (req: unknown) => scan(req),
  },
}));

/** One BSS, with the fields the page reads and sane values for the rest. */
function network(over: Record<string, unknown> = {}) {
  return {
    ssid: 'Trellis Lab',
    bssid: '02:00:00:00:00:01',
    signalDbm: -48,
    channel: 36,
    frequencyMhz: 5180,
    security: 'WPA3',
    channelWidthMhz: 80,
    noiseFloorDbm: -95,
    snrDb: 47,
    htMode: 'VHT80',
    isDfs: false,
    associated: false,
    channelUtilizationPercent: undefined,
    ...over,
  };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={['/live']}>{children}</MemoryRouter>
      </QueryClientProvider>
    );
  }
  return render(<LivePage />, { wrapper });
}

beforeEach(() => {
  scan.mockReset();
  scan.mockResolvedValue({ networks: [network()] });
});

describe('LivePage', () => {
  it('reports the connection rather than the airspace when there is one', async () => {
    scan.mockResolvedValue({
      networks: [
        network({ associated: true, snrDb: 42, channelUtilizationPercent: 12 }),
        network({ bssid: '02:00:00:00:00:02', ssid: 'Guest', signalDbm: -70, snrDb: 25 }),
      ],
    });
    renderPage();

    expect(await screen.findByText(/Connected to Trellis Lab with 42 dB/)).toBeInTheDocument();
    // The reading is the connection's, not the strongest neighbour's or an
    // average — a page that summed the airspace would still print a number.
    const rollup = screen.getByTestId('status-rollup');
    expect(rollup).toHaveTextContent('42.0 dB');
    expect(rollup).toHaveTextContent('12%');
  });

  it('calls a strong signal on a noisy channel weak', async () => {
    // Signal alone reads as excellent here. SNR is what says otherwise, and a
    // page judging on dBm would call this healthy.
    scan.mockResolvedValue({
      networks: [network({ associated: true, signalDbm: -35, snrDb: 11 })],
    });
    renderPage();

    expect(await screen.findByText(/only 11 dB of margin/)).toBeInTheDocument();
  });

  it('flags a congested channel on an otherwise strong link', async () => {
    scan.mockResolvedValue({
      networks: [network({ associated: true, snrDb: 45, channelUtilizationPercent: 78 })],
    });
    renderPage();

    expect(await screen.findByText(/channel 78% busy/)).toBeInTheDocument();
  });

  it('withholds a verdict when the host is not associated', async () => {
    renderPage();

    expect(await screen.findByText('Not joined to a network')).toBeInTheDocument();
    // StatusRollup prints em dashes for figures in the unknown state; the point
    // is that nothing here claims a link that does not exist.
    expect(screen.queryByText(/Connected to/)).not.toBeInTheDocument();
  });

  it('distinguishes an unreported channel utilization from an idle channel', async () => {
    scan.mockResolvedValue({
      networks: [
        network({ channelUtilizationPercent: 0 }),
        network({ bssid: '02:00:00:00:00:02', channelUtilizationPercent: undefined }),
      ],
    });
    renderPage();

    const rows = await screen.findAllByTestId('neighbour-row');
    expect(rows[0]).toHaveTextContent('0%');
    expect(rows[1]).toHaveTextContent('Not reported');
  });

  it('names a hidden network instead of leaving the cell blank', async () => {
    scan.mockResolvedValue({ networks: [network({ ssid: '' })] });
    renderPage();

    expect(await screen.findByText('Hidden network')).toBeInTheDocument();
  });

  it('surfaces a scan failure instead of an empty airspace', async () => {
    scan.mockRejectedValue(new Error('no Wi-Fi capture backend configured'));
    renderPage();

    expect(await screen.findByText('The scan failed')).toBeInTheDocument();
    expect(screen.getByText(/no Wi-Fi capture backend configured/)).toBeInTheDocument();
  });

  it('stops taking the radio when scanning is paused', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      renderPage();
      await waitFor(() => expect(scan).toHaveBeenCalledTimes(1));

      fireEvent.click(screen.getByTestId('toggle-polling'));
      await vi.advanceTimersByTimeAsync(30_000);

      // The one radio is also what a walk captures with; a paused page must
      // actually stop asking for it, not just relabel its button.
      expect(scan).toHaveBeenCalledTimes(1);
      expect(screen.getByTestId('toggle-polling')).toHaveTextContent('Resume scanning');
    } finally {
      vi.useRealTimers();
    }
  });
});
