/**
 * The surface is the product's central action: a click becomes a stored
 * measurement. What matters is that the click reaches the service as integer
 * coordinates in the survey's space, that the reply becomes a pin the operator
 * can see, and that a failure names itself instead of leaving a gap.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CaptureSurface, SURFACE_HEIGHT, SURFACE_WIDTH } from './CaptureSurface';

const capturePoint = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    capturePoint: (req: unknown) => capturePoint(req),
  },
}));

const scan = {
  networks: [
    { ssid: 'Near', bssid: 'aa:bb:cc:00:00:02', signalDbm: -41 },
    { ssid: 'Faint', bssid: 'aa:bb:cc:00:00:01', signalDbm: -85 },
  ],
};

function renderSurface() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  }
  return render(<CaptureSurface surveyId="svy-9" surveyName="Everett HQ" />, { wrapper });
}

// jsdom lays nothing out, so the surface reports a zero box and the click math
// has nothing to scale by. Give it the surface's own size at half scale so the
// mapping through the viewBox is exercised rather than an identity.
//
// Pointer clicks carry detail: 1, as a browser's do; the default of 0 is what a
// keyboard activation produces, and the component reads it that way.
const halfScale = {
  left: 0,
  top: 0,
  width: SURFACE_WIDTH / 2,
  height: SURFACE_HEIGHT / 2,
} as DOMRect;

beforeEach(() => {
  capturePoint.mockReset();
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(halfScale);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('CaptureSurface', () => {
  it('maps a click through the viewBox and sends integer coordinates', async () => {
    capturePoint.mockResolvedValue(scan);
    renderSurface();

    fireEvent.click(screen.getByTestId('capture-surface'), {
      clientX: 100.4,
      clientY: 60.2,
      detail: 1,
    });

    await waitFor(() => expect(capturePoint).toHaveBeenCalledTimes(1));
    // Half scale: 100.4 px on screen is 200.8 in survey space, rounded to 201.
    expect(capturePoint).toHaveBeenCalledWith({ surveyId: 'svy-9', x: 201, y: 120 });
  });

  it('draws the reply as a pin labelled with the strongest signal', async () => {
    capturePoint.mockResolvedValue(scan);
    renderSurface();

    expect(screen.getByText('No points yet. The first click starts the map.')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('capture-surface'), {
      clientX: 200,
      clientY: 100,
      detail: 1,
    });

    expect(await screen.findAllByTestId('capture-pin')).toHaveLength(1);
    expect(screen.getByText('2 networks at (400, 200), strongest -41.0 dBm')).toBeInTheDocument();
    expect(screen.getByText('1 point captured this session')).toBeInTheDocument();
  });

  it('ignores clicks while a scan is in flight rather than queueing them', async () => {
    let finish: (value: typeof scan) => void = () => {};
    capturePoint.mockImplementation(
      () =>
        new Promise<typeof scan>((resolve) => {
          finish = resolve;
        }),
    );
    renderSurface();

    const surface = screen.getByTestId('capture-surface');
    fireEvent.click(surface, { clientX: 10, clientY: 10, detail: 1 });
    await screen.findByText('Scanning the airspace…');
    fireEvent.click(surface, { clientX: 50, clientY: 50, detail: 1 });

    finish(scan);
    expect(await screen.findAllByTestId('capture-pin')).toHaveLength(1);
    expect(capturePoint).toHaveBeenCalledTimes(1);
  });

  it('names a failed capture with the service message', async () => {
    capturePoint.mockRejectedValue(
      new Error(
        '[failed_precondition] capture: OS permission required to read network identifiers',
      ),
    );
    renderSurface();

    fireEvent.click(screen.getByTestId('capture-surface'), { clientX: 10, clientY: 10, detail: 1 });

    expect(await screen.findByText(/OS permission required/)).toBeInTheDocument();
    expect(screen.queryAllByTestId('capture-pin')).toHaveLength(0);
  });

  it('captures from the keyboard at a cursor the arrow keys move', async () => {
    capturePoint.mockResolvedValue(scan);
    renderSurface();

    const surface = screen.getByTestId('capture-surface');
    fireEvent.focus(surface);
    expect(screen.getByTestId('capture-cursor')).toBeInTheDocument();

    // Centre is (400, 250); two fast steps right and one slow step up.
    fireEvent.keyDown(surface, { key: 'ArrowRight', shiftKey: true });
    fireEvent.keyDown(surface, { key: 'ArrowRight', shiftKey: true });
    fireEvent.keyDown(surface, { key: 'ArrowUp' });
    // Enter on a button is a click with no pointer (detail 0); jsdom does not
    // synthesise it from the key, so it is dispatched as the browser would.
    fireEvent.click(surface);

    await waitFor(() => expect(capturePoint).toHaveBeenCalledTimes(1));
    expect(capturePoint).toHaveBeenCalledWith({ surveyId: 'svy-9', x: 500, y: 240 });
  });
});
