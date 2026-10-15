/**
 * The predicted layer. These pin what an operator relies on: a floor with
 * nothing placed is still a plan they can click, an AP goes down on the band
 * they are looking at, and the panel says whether the walk calibrated it.
 */
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import type { ComponentProps } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { GetHeatmapResponseSchema } from '@/gen/trellis/survey/v1/survey_pb';
import { PredictionView } from './PredictionView';

const getFloorPlanImage = vi.fn();
const listAPPlacements = vi.fn();
const createAPPlacement = vi.fn();
const updateAPPlacement = vi.fn();
const deleteAPPlacement = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    getFloorPlanImage: (req: unknown) => getFloorPlanImage(req),
    listAPPlacements: (req: unknown) => listAPPlacements(req),
    createAPPlacement: (req: unknown) => createAPPlacement(req),
    updateAPPlacement: (req: unknown) => updateAPPlacement(req),
    deleteAPPlacement: (req: unknown) => deleteAPPlacement(req),
  },
}));

const png = new Uint8Array([0x89, 0x50, 0x4e, 0x47]);
const placed = { id: 'p-1', x: 100, y: 80, label: '', bssid: '', band: '5 GHz', imported: false };

function renderView(props: Partial<ComponentProps<typeof PredictionView>> = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PredictionView
        surveyId="svy-1"
        floorId=""
        band="5 GHz"
        heatmap={undefined}
        heatmapError={null}
        heatmapLoading={false}
        {...props}
      />
    </QueryClientProvider>,
  );
}

const noPlacements = new ConnectError(
  'floor has no AP placed on this band: 5 GHz',
  Code.FailedPrecondition,
);

const prediction = {
  png,
  width: 400,
  height: 300,
  metric: 'predicted_rssi',
  grid: [-50],
  gridCols: 1,
  gridRows: 1,
  cellSize: 400,
  legend: [],
  calibrations: [],
};

beforeEach(() => {
  vi.clearAllMocks();
  getFloorPlanImage.mockResolvedValue({ image: png, width: 400, height: 300 });
  listAPPlacements.mockResolvedValue({ placements: [] });
  createAPPlacement.mockResolvedValue({ placement: { ...placed } });
  updateAPPlacement.mockResolvedValue({ placement: { ...placed } });
});

describe('PredictionView', () => {
  it('keeps the plan clickable when nothing is placed, and places on the band shown', async () => {
    renderView({ heatmapError: noPlacements });

    await waitFor(() =>
      expect(screen.getByTestId('surface-message')).toHaveTextContent(
        'No AP placed on 5 GHz. Click the plan to place one.',
      ),
    );
    expect(screen.getByTestId('placement-plan')).toBeInTheDocument();

    fireEvent.click(screen.getByTestId('placement-surface'), { detail: 0 });
    await waitFor(() =>
      expect(createAPPlacement).toHaveBeenCalledWith({
        surveyId: 'svy-1',
        floorId: '',
        placement: { x: 200, y: 150, label: '', band: '5 GHz' },
      }),
    );
  });

  it('quotes the refusal when APs are placed but the floor cannot be predicted', async () => {
    listAPPlacements.mockResolvedValue({ placements: [placed] });
    renderView({
      heatmapError: new ConnectError(
        'floor plan has no scale (metres per pixel); set one before predicting',
        Code.FailedPrecondition,
      ),
    });

    await waitFor(() =>
      expect(screen.getByTestId('surface-message')).toHaveTextContent(
        'Nothing to predict yet: floor plan has no scale',
      ),
    );
  });

  it('asks the plan to be uploaded first on a floor without one', async () => {
    getFloorPlanImage.mockResolvedValue({ image: new Uint8Array(), width: 0, height: 0 });
    renderView({ heatmapError: noPlacements });

    await waitFor(() =>
      expect(screen.getByTestId('surface-message')).toHaveTextContent('This floor has no plan.'),
    );
    expect(screen.queryByTestId('placement-surface')).not.toBeInTheDocument();
  });

  it.each([
    {
      name: 'a fit to the walk',
      calibration: {
        calibrated: true,
        clamped: false,
        pairs: 12,
        meanAbsErrorDb: 3.14,
        exponent: 2.9,
      },
      want: "Fitted to this floor's walk: 12 readings, 3.1 dB mean error, path-loss exponent 2.90",
    },
    {
      name: 'a fit held at its bound',
      calibration: { calibrated: true, clamped: true, pairs: 1, meanAbsErrorDb: 4, exponent: 4 },
      want: "at the exponent's limit of 4.00: 1 reading, 4.0 dB mean error",
    },
    {
      name: 'the defaults',
      calibration: {
        calibrated: false,
        clamped: false,
        pairs: 0,
        exponent: 3,
        fallback: 'no BSSID',
      },
      want: 'Drawn with the default model: no BSSID',
    },
  ])('says how the selected AP was drawn: $name', async ({ calibration, want }) => {
    listAPPlacements.mockResolvedValue({ placements: [placed] });
    renderView({
      heatmap: create(GetHeatmapResponseSchema, {
        ...prediction,
        calibrations: [{ placementId: 'p-1', label: '', bssid: '', fallback: '', ...calibration }],
      }),
    });

    fireEvent.click(within(await screen.findByTestId('placement-list')).getByRole('button'));
    expect(screen.getByTestId('placement-calibration')).toHaveTextContent(want);
  });

  it('sends an emptied power as unstated, and refuses one no radio has', async () => {
    listAPPlacements.mockResolvedValue({ placements: [{ ...placed, txPowerDbm: 17 }] });
    renderView({ heatmapError: noPlacements });

    fireEvent.click(within(await screen.findByTestId('placement-list')).getByRole('button'));
    const power = screen.getByTestId('placement-power');
    expect(power).toHaveValue(17);

    fireEvent.change(power, { target: { value: '40' } });
    expect(screen.getByTestId('placement-apply')).toBeDisabled();

    fireEvent.change(power, { target: { value: '' } });
    fireEvent.click(screen.getByTestId('placement-apply'));
    await waitFor(() => expect(updateAPPlacement).toHaveBeenCalled());
    expect(updateAPPlacement.mock.calls[0]?.[0]).toMatchObject({
      placementId: 'p-1',
      placement: { x: 100, y: 80, band: '5 GHz', txPowerDbm: undefined },
    });
  });
});
