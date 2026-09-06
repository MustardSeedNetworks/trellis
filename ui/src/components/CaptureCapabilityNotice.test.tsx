/**
 * The notice is the difference between "this machine cannot use Trellis" and
 * "this machine cannot start a walk". The daemon knew which one was true all
 * along and only said it in a log nobody reads on a packaged install.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { CaptureCapabilityNotice } from './CaptureCapabilityNotice';

const getCaptureCapability = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    getCaptureCapability: (req: unknown) => getCaptureCapability(req),
  },
}));

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe('CaptureCapabilityNotice', () => {
  beforeEach(() => {
    getCaptureCapability.mockReset();
  });

  it('says nothing when the host has a radio', async () => {
    getCaptureCapability.mockResolvedValue({ available: true, reason: '', remedy: '' });

    render(<CaptureCapabilityNotice />, { wrapper });

    await waitFor(() => expect(getCaptureCapability).toHaveBeenCalled());
    expect(screen.queryByTestId('capture-unavailable')).toBeNull();
  });

  it('gives the reason, the remedy, and what still works without a radio', async () => {
    getCaptureCapability.mockResolvedValue({
      available: false,
      reason: 'capture: no Wi-Fi interface',
      remedy: 'attach a supported adapter',
    });

    render(<CaptureCapabilityNotice />, { wrapper });

    const notice = await screen.findByTestId('capture-unavailable');
    expect(notice).toHaveTextContent(/cannot measure/i);
    expect(notice).toHaveTextContent('capture: no Wi-Fi interface');
    expect(notice).toHaveTextContent('attach a supported adapter');
    // The half that keeps this from reading as "Trellis is broken here".
    expect(notice).toHaveTextContent(/import, analyse and report/i);
  });

  it('omits the remedy line when there is nothing the operator can do', async () => {
    getCaptureCapability.mockResolvedValue({
      available: false,
      reason: 'capture: not supported on this platform',
      remedy: '',
    });

    const notice = await (async () => {
      render(<CaptureCapabilityNotice />, { wrapper });
      return screen.findByTestId('capture-unavailable');
    })();
    expect(notice).toHaveTextContent('not supported on this platform');
    expect(notice.textContent).not.toContain('undefined');
  });
});
