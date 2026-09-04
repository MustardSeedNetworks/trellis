/**
 * The target is what every active measurement on a survey runs against, so what
 * matters is that the operator's server actually reaches the service — a field
 * that accepted input and sent nothing would look identical.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ThroughputTarget } from './ThroughputTarget';

const setThroughputTarget = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    setThroughputTarget: (req: unknown) => setThroughputTarget(req),
  },
}));

function renderTarget(server = '') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  }
  return render(<ThroughputTarget surveyId="svy-9" server={server} />, { wrapper });
}

beforeEach(() => {
  setThroughputTarget.mockReset();
  setThroughputTarget.mockResolvedValue({ survey: { id: 'svy-9', iperfServer: '10.44.30.30' } });
});

describe('ThroughputTarget', () => {
  it('sends the server the operator typed', async () => {
    renderTarget();

    fireEvent.change(screen.getByTestId('throughput-target'), {
      target: { value: '  10.44.30.30  ' },
    });
    fireEvent.click(screen.getByTestId('save-throughput-target'));

    await waitFor(() => expect(setThroughputTarget).toHaveBeenCalled());
    // Trimmed: a pasted address carries whitespace, and iperf3 would fail on it
    // with a message about the host rather than about the space.
    expect(setThroughputTarget.mock.calls[0]?.[0]).toMatchObject({
      surveyId: 'svy-9',
      server: '10.44.30.30',
    });
  });

  it("starts from the survey's existing target and will not resave it unchanged", () => {
    renderTarget('10.44.30.30');

    expect(screen.getByTestId('throughput-target')).toHaveValue('10.44.30.30');
    expect(screen.getByTestId('save-throughput-target')).toBeDisabled();
  });

  it('clears the target when the field is emptied', async () => {
    renderTarget('10.44.30.30');

    fireEvent.change(screen.getByTestId('throughput-target'), { target: { value: '' } });
    fireEvent.click(screen.getByTestId('save-throughput-target'));

    // An empty server is how a survey goes back to passive-only, so it has to
    // be sendable rather than treated as "nothing to do".
    await waitFor(() => expect(setThroughputTarget).toHaveBeenCalled());
    expect(setThroughputTarget.mock.calls[0]?.[0]).toMatchObject({ server: '' });
  });

  it('says a save failed rather than leaving the old value on screen', async () => {
    setThroughputTarget.mockRejectedValue(new Error('survey not found'));
    renderTarget();

    fireEvent.change(screen.getByTestId('throughput-target'), { target: { value: '10.0.0.1' } });
    fireEvent.click(screen.getByTestId('save-throughput-target'));

    expect(await screen.findByTestId('throughput-target-error')).toHaveTextContent(
      'survey not found',
    );
  });
});
