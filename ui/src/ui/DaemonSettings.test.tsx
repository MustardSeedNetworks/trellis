import { Code, ConnectError } from '@connectrpc/connect';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { DaemonSettings } from './DaemonSettings';

const getConfig = vi.fn();

vi.mock('@/lib/client', () => ({
  daemonClient: { getConfig: (req: unknown) => getConfig(req) },
}));

function renderTab() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <DaemonSettings />
    </QueryClientProvider>,
  );
}

const config = {
  listenAddr: '0.0.0.0:8446',
  dataDir: '/srv/trellis',
  captureMode: 'monitor',
  tlsEnabled: true,
  authRequired: true,
  logFile: '/var/log/trellis.log',
};

beforeEach(() => getConfig.mockReset());

describe('DaemonSettings', () => {
  it('shows the daemon configuration it was sent', async () => {
    getConfig.mockResolvedValue(config);
    renderTab();

    expect(await screen.findByText('0.0.0.0:8446')).toBeInTheDocument();
    expect(screen.getByText('/srv/trellis')).toBeInTheDocument();
    expect(screen.getByText('Monitor mode')).toBeInTheDocument();
    expect(screen.getByText('/var/log/trellis.log')).toBeInTheDocument();
    expect(screen.getAllByText('Yes')).toHaveLength(2);
  });

  it('says standard output when the daemon logs to no file, and No for TLS and sign-in off', async () => {
    getConfig.mockResolvedValue({
      ...config,
      captureMode: 'scan',
      tlsEnabled: false,
      authRequired: false,
      logFile: '',
    });
    renderTab();

    expect(await screen.findByText('Standard output')).toBeInTheDocument();
    expect(screen.getByText('OS scan')).toBeInTheDocument();
    expect(screen.getAllByText('No')).toHaveLength(2);
  });

  it('says so when the daemon does not answer rather than showing nothing', async () => {
    getConfig.mockRejectedValueOnce(new ConnectError('unreachable', Code.Unavailable));
    renderTab();

    expect(await screen.findByText('Daemon settings are not arriving')).toBeInTheDocument();
  });
});
