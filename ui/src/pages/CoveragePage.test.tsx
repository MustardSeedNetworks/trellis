/**
 * Coverage — the Canvas page.
 *
 * These pin the three things the page exists to get right: the image is
 * described by the scale the service painted it with, a failure says which
 * failure it was instead of rendering an empty floor, and the dead-zone
 * threshold cannot reach the value the service silently reinterprets.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { CoveragePage } from './CoveragePage';

const listSurveys = vi.fn();
const listFloors = vi.fn();
const getHeatmap = vi.fn();
const getCoverage = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    listSurveys: (req: unknown) => listSurveys(req),
    listFloors: (req: unknown) => listFloors(req),
    getHeatmap: (req: unknown) => getHeatmap(req),
    getCoverage: (req: unknown) => getCoverage(req),
  },
}));

function renderPage(initialPath = '/coverage') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={[initialPath]}>{children}</MemoryRouter>
      </QueryClientProvider>
    );
  }
  return render(<CoveragePage />, { wrapper });
}

const survey = {
  id: 'svy-1',
  name: 'Everett HQ',
  status: 'complete',
  floorCount: 1,
  sampleCount: 42,
  hasFloorPlan: true,
};

/** A 1x1 PNG's first bytes are enough: the page only encodes what it is given. */
const heatmapReply = {
  png: new Uint8Array([0x89, 0x50, 0x4e, 0x47]),
  width: 400,
  height: 300,
  min: -91.2,
  max: -41.5,
  sampleCount: 42,
  metric: 'rssi',
  /* 2x2 grid of 200 px cells over the 400x300 image, so a page test that
     renders the surface has something coherent to read from. */
  grid: [-52, -61, -70, -84],
  gridCols: 2,
  gridRows: 2,
  cellSize: 200,
  legend: [
    { value: -100, color: '#808080' },
    { value: -75, color: '#ffcc00' },
    { value: -30, color: '#22aa55' },
  ],
};

beforeEach(() => {
  listSurveys.mockReset();
  listFloors.mockReset();
  getHeatmap.mockReset();
  getCoverage.mockReset();
  listSurveys.mockResolvedValue({ surveys: [survey] });
  listFloors.mockResolvedValue({ floors: [] });
  getHeatmap.mockResolvedValue(heatmapReply);
  getCoverage.mockResolvedValue({ coverageScore: 82.4, deadZoneCount: 2, recommendations: [] });
});

describe('CoveragePage', () => {
  it('draws the legend from the scale the service painted the image with', async () => {
    const { container } = renderPage();

    expect(await screen.findByTestId('heatmap-image')).toBeInTheDocument();
    const gradient = container.querySelector('[data-testid="legend-gradient"]');
    /* -75 sits halfway between -100 and -30 by value but is the middle of
       three stops by index; the gradient has to place it by value. */
    expect(gradient?.getAttribute('style')).toContain('rgb(255, 204, 0) 35.7%');
    expect(screen.getByText('-100 dBm')).toBeInTheDocument();
  });

  it('reports the coverage verdict as a sentence about this floor', async () => {
    renderPage();

    expect(await screen.findByText('2 dead zones below -75 dBm')).toBeInTheDocument();
    /* The service sends a percentage; rendering 82.4 as 8240% was the bug. */
    expect(screen.getByText('82%')).toBeInTheDocument();
    expect(screen.getByTestId('coverage-findings')).toHaveAttribute('data-state', 'warn');
  });

  it('says which failure happened rather than showing an empty floor', async () => {
    getHeatmap.mockRejectedValue(new Error('floor plan required'));

    renderPage();

    await waitFor(() =>
      expect(screen.getByTestId('surface-message')).toHaveTextContent('floor plan required'),
    );
    expect(screen.queryByTestId('heatmap-image')).not.toBeInTheDocument();
  });

  it('refuses to print coverage figures it does not have', async () => {
    getCoverage.mockRejectedValue(new Error('service unavailable'));

    renderPage();

    const findings = await screen.findByTestId('coverage-findings');
    await waitFor(() => expect(findings).toHaveTextContent('Coverage analysis is not arriving'));
    expect(findings).toHaveAttribute('data-state', 'unknown');
    expect(findings).not.toHaveTextContent('0%');
  });

  it('names the empty case as empty, not as a floor with no signal', async () => {
    listSurveys.mockResolvedValue({ surveys: [] });

    renderPage();

    await waitFor(() =>
      expect(screen.getByTestId('surface-message')).toHaveTextContent('No surveys captured'),
    );
    expect(getHeatmap).not.toHaveBeenCalled();
  });

  it('analyses the survey the URL names, so a link opens the floor it promised', async () => {
    listSurveys.mockResolvedValue({ surveys: [survey, { ...survey, id: 'svy-2', name: 'Depot' }] });

    renderPage('/coverage?survey=svy-2');

    await waitFor(() =>
      expect(getHeatmap).toHaveBeenCalledWith({ surveyId: 'svy-2', metric: 'rssi', floorId: '' }),
    );
  });

  /**
   * The service reads threshold_dbm == 0 as "unset" and substitutes -75, so a
   * zero would be answered with a different threshold than the one on screen.
   */
  it('never sends a threshold the service would reinterpret', async () => {
    renderPage();

    await waitFor(() => expect(getCoverage).toHaveBeenCalled());
    fireEvent.change(screen.getByLabelText(/dead zone below/i), { target: { value: '0' } });

    for (const call of getCoverage.mock.calls) {
      expect(call[0].thresholdDbm).not.toBe(0);
    }
    expect(screen.getByLabelText(/dead zone below/i)).toHaveValue(-75);
  });

  /**
   * Floors — the picker exists so a multi-floor survey can be read one storey
   * at a time. The service answers about the active floor when a request names
   * none, so an unnamed floor must stay unnamed rather than being filled in.
   */
  describe('floors', () => {
    const twoFloors = { ...survey, floorCount: 2 };
    const floors = [
      {
        id: 'flr-basement',
        name: 'Basement',
        level: -1,
        sampleCount: 3,
        hasFloorPlan: false,
        isActive: false,
      },
      {
        id: 'flr-ground',
        name: 'Floor 1',
        level: 0,
        sampleCount: 2,
        hasFloorPlan: true,
        isActive: true,
      },
    ];

    it('does not ask for floors a single-floor survey cannot have', async () => {
      renderPage();

      await waitFor(() => expect(getHeatmap).toHaveBeenCalled());
      expect(listFloors).not.toHaveBeenCalled();
      expect(screen.queryByTestId('coverage-floor')).not.toBeInTheDocument();
      // Empty, not the active floor's id: the page does not know it.
      expect(getHeatmap).toHaveBeenCalledWith({ surveyId: 'svy-1', metric: 'rssi', floorId: '' });
    });

    it('analyses the floor the picker names, heatmap and coverage together', async () => {
      listSurveys.mockResolvedValue({ surveys: [twoFloors] });
      listFloors.mockResolvedValue({ floors });

      renderPage('/coverage?survey=svy-1');

      const picker = await screen.findByTestId('coverage-floor');
      // The active floor is the selection until someone chooses otherwise.
      expect(picker).toHaveValue('');

      fireEvent.change(picker, { target: { value: 'flr-basement' } });

      await waitFor(() =>
        expect(getHeatmap).toHaveBeenCalledWith({
          surveyId: 'svy-1',
          metric: 'rssi',
          floorId: 'flr-basement',
        }),
      );
      // Coverage follows the same floor: a score for one storey beside a
      // picture of another is the defect this guards.
      await waitFor(() =>
        expect(getCoverage).toHaveBeenCalledWith({
          surveyId: 'svy-1',
          thresholdDbm: -75,
          floorId: 'flr-basement',
        }),
      );
    });

    it('falls back to the active floor when the URL names one this survey lacks', async () => {
      listSurveys.mockResolvedValue({ surveys: [twoFloors] });
      listFloors.mockResolvedValue({ floors });

      renderPage('/coverage?survey=svy-1&floor=flr-of-another-survey');

      await waitFor(() => expect(screen.getByTestId('heatmap-image')).toBeInTheDocument());
      for (const call of getHeatmap.mock.calls) {
        expect(call[0].floorId).not.toBe('flr-of-another-survey');
      }
    });
  });
});
