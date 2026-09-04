/**
 * The plan is what a survey's points are drawn on, and the scale is what turns
 * a pixel distance into a metre. Both reach the service or neither means
 * anything, so those are what these pin.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { FloorPlanPanel } from './FloorPlanPanel';

const setFloorPlan = vi.fn();
const calibrateFloorPlan = vi.fn();
const getFloorPlanImage = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    setFloorPlan: (req: unknown) => setFloorPlan(req),
    calibrateFloorPlan: (req: unknown) => calibrateFloorPlan(req),
    getFloorPlanImage: (req: unknown) => getFloorPlanImage(req),
  },
}));

function renderPanel(hasPlan = false, scaleM = 0) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  }
  return render(
    <FloorPlanPanel surveyId="svy-9" floorId="flr-1" hasPlan={hasPlan} scaleM={scaleM} />,
    { wrapper },
  );
}

beforeEach(() => {
  setFloorPlan.mockReset();
  calibrateFloorPlan.mockReset();
  getFloorPlanImage.mockReset();
  setFloorPlan.mockResolvedValue({ floor: { id: 'flr-1', hasFloorPlan: true } });
  calibrateFloorPlan.mockResolvedValue({ floor: { id: 'flr-1', scaleM: 0.025 } });
  getFloorPlanImage.mockResolvedValue({
    image: new Uint8Array([0x89, 0x50, 0x4e, 0x47]),
    width: 800,
    height: 600,
  });
});

describe('FloorPlanPanel', () => {
  it('sends the chosen file as the floor plan', async () => {
    renderPanel();

    const file = new File([new Uint8Array([1, 2, 3])], 'ninth-floor.png', { type: 'image/png' });
    fireEvent.change(screen.getByTestId('floor-plan-input'), { target: { files: [file] } });

    await waitFor(() => expect(setFloorPlan).toHaveBeenCalled());
    expect(setFloorPlan.mock.calls[0]?.[0]).toMatchObject({
      surveyId: 'svy-9',
      floorId: 'flr-1',
    });
    expect(setFloorPlan.mock.calls[0]?.[0].image).toBeInstanceOf(Uint8Array);
  });

  it('offers no calibration until there is a plan to calibrate', () => {
    renderPanel(false);

    // The two points a calibration is expressed in are points on the plan.
    expect(screen.queryByTestId('calibrate-floor-plan')).not.toBeInTheDocument();
    expect(screen.getByTestId('floor-plan-status')).toHaveTextContent('No plan on this floor');
  });

  it("calibrates across the plan's own width", async () => {
    renderPanel(true);

    await waitFor(() => expect(getFloorPlanImage).toHaveBeenCalled());
    fireEvent.change(screen.getByTestId('plan-width-metres'), { target: { value: '20' } });
    await waitFor(() => expect(screen.getByTestId('calibrate-floor-plan')).not.toBeDisabled());
    fireEvent.click(screen.getByTestId('calibrate-floor-plan'));

    await waitFor(() => expect(calibrateFloorPlan).toHaveBeenCalled());
    // A line from (0,0) to (width,0), which is the longest measurable thing on
    // the plan and the one a drawing usually dimensions.
    expect(calibrateFloorPlan.mock.calls[0]?.[0]).toMatchObject({
      x1: 0,
      y1: 0,
      x2: 800,
      y2: 0,
      metres: 20,
    });
  });

  it('says a plan is uncalibrated rather than implying a scale nobody set', async () => {
    renderPanel(true, 0);

    expect(await screen.findByTestId('floor-plan-status')).toHaveTextContent(
      'The plan has no scale yet',
    );
  });

  it('reports the scale as a distance a person can check', async () => {
    renderPanel(true, 0.025);

    // 0.025 m/px across 800 px is 20 m. A metres-per-pixel figure alone is not
    // something anyone can sanity-check against a building.
    // Withheld until the plan's dimensions arrive: 0.0 m across is a reading
    // nobody measured, and it would be on screen for as long as the fetch takes.
    expect(await screen.findByTestId('floor-plan-status')).toHaveTextContent('0.025 m per pixel.');
    await waitFor(() =>
      expect(screen.getByTestId('floor-plan-status')).toHaveTextContent(
        '0.025 m per pixel — the plan is 20.0 m across',
      ),
    );
  });

  it('names a rejected upload rather than leaving the panel calm', async () => {
    setFloorPlan.mockRejectedValue(new Error('floor plan is not a PNG or JPEG image'));
    renderPanel();

    const file = new File([new Uint8Array([1])], 'notes.txt', { type: 'text/plain' });
    fireEvent.change(screen.getByTestId('floor-plan-input'), { target: { files: [file] } });

    const status = await screen.findByTestId('floor-plan-status');
    expect(status).toHaveTextContent('not a PNG or JPEG image');
    expect(status).toHaveClass('text-status-error');
  });
});
