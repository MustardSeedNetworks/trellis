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
const createFloor = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    setFloorPlan: (req: unknown) => setFloorPlan(req),
    calibrateFloorPlan: (req: unknown) => calibrateFloorPlan(req),
    getFloorPlanImage: (req: unknown) => getFloorPlanImage(req),
    createFloor: (req: unknown) => createFloor(req),
  },
}));

function renderPanel(hasPlan = false, scaleM = 0, nextLevel = 1) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  }
  return render(
    <FloorPlanPanel
      surveyId="svy-9"
      floorId="flr-1"
      hasPlan={hasPlan}
      scaleM={scaleM}
      nextLevel={nextLevel}
    />,
    { wrapper },
  );
}

beforeEach(() => {
  setFloorPlan.mockReset();
  calibrateFloorPlan.mockReset();
  getFloorPlanImage.mockReset();
  createFloor.mockReset();
  createFloor.mockResolvedValue({ floor: { id: 'flr-9', name: 'Ninth', level: 1 } });
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

  it('calibrates from two points marked on the plan', async () => {
    renderPanel(true);

    await waitFor(() => expect(getFloorPlanImage).toHaveBeenCalled());
    const surface = await screen.findByTestId('calibration-surface');
    vi.spyOn(surface, 'getBoundingClientRect').mockReturnValue({
      left: 0,
      top: 0,
      width: 800,
      height: 600,
      right: 800,
      bottom: 600,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    });

    fireEvent.click(surface, { clientX: 100, clientY: 100, detail: 1 });
    fireEvent.click(surface, { clientX: 500, clientY: 100, detail: 1 });
    fireEvent.change(screen.getByTestId('calibration-metres'), { target: { value: '20' } });
    fireEvent.click(screen.getByTestId('calibrate-floor-plan'));

    await waitFor(() => expect(calibrateFloorPlan).toHaveBeenCalled());
    // The two points the operator marked, not the plan's outer edge. The
    // previous UI sent (0,0)→(width,0) whatever was on the drawing.
    expect(calibrateFloorPlan.mock.calls[0]?.[0]).toMatchObject({
      x1: 100,
      y1: 100,
      x2: 500,
      y2: 100,
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
    /* The refusal has to be ANNOUNCED, not merely coloured. The live region is
       asserted on the element itself and unconditionally — a role that appears
       with the error is announced unreliably, because a screen reader has to
       be watching the region before the text changes. */
    expect(status).toHaveAttribute('aria-live', 'polite');
    // An ordinary refusal gets no hint: there is nothing to explain beyond
    // "that was not an image".
    expect(screen.queryByTestId('floor-plan-stranded-hint')).toBeNull();
  });

  // The one refusal an operator can act on. Reporting it alone reads as the
  // product being broken; the point is that a same-sized plan works and a
  // differently-sized one would move every measured point.
  it('explains a plan refused because it would strand the measurements', async () => {
    setFloorPlan.mockRejectedValue(
      new Error(
        'replacing the floor plan would strand the measurements taken on it: ' +
          '3 measurements on this floor were taken against a 800x600 plan, not 1024x768',
      ),
    );
    renderPanel();

    const file = new File([new Uint8Array([1])], 'plan.png', { type: 'image/png' });
    fireEvent.change(screen.getByTestId('floor-plan-input'), { target: { files: [file] } });

    const status = await screen.findByTestId('floor-plan-status');
    // The server says how much is at stake; three points is a different
    // decision from three hundred.
    expect(status).toHaveTextContent('3 measurements');
    const hint = await screen.findByTestId('floor-plan-stranded-hint');
    expect(hint).toHaveTextContent(/same dimensions/i);
    expect(hint).toHaveTextContent(/delete the measurements/i);
  });

  // The route TR-1 was written to have and could not build: the plan a floor
  // refused is a plan of a different storey often enough that offering the new
  // floor is the fix, not an apology.
  it('puts the refused plan on a new floor without asking for the file again', async () => {
    setFloorPlan.mockRejectedValueOnce(
      new Error('replacing the floor plan would strand the measurements taken on it: 3 …'),
    );
    renderPanel();

    const file = new File([new Uint8Array([1, 2, 3])], 'ninth.png', { type: 'image/png' });
    fireEvent.change(screen.getByTestId('floor-plan-input'), { target: { files: [file] } });
    await screen.findByTestId('floor-plan-stranded-hint');

    fireEvent.change(screen.getByTestId('strand-new-floor-name'), { target: { value: 'Ninth' } });
    fireEvent.click(screen.getByTestId('strand-new-floor'));

    await waitFor(() => expect(createFloor).toHaveBeenCalled());
    expect(createFloor.mock.calls[0]?.[0]).toMatchObject({
      surveyId: 'svy-9',
      name: 'Ninth',
      level: 1,
    });
    // The bytes the operator already chose go straight onto the new floor.
    await waitFor(() => expect(setFloorPlan).toHaveBeenCalledTimes(2));
    expect(setFloorPlan.mock.calls[1]?.[0]).toMatchObject({ floorId: 'flr-9' });
    expect(setFloorPlan.mock.calls[1]?.[0].image).toBeInstanceOf(Uint8Array);
  });

  it('offers the new-floor route only on the refusal that has one', async () => {
    renderPanel();

    const file = new File([new Uint8Array([1])], 'notes.txt', { type: 'text/plain' });
    fireEvent.change(screen.getByTestId('floor-plan-input'), { target: { files: [file] } });

    await screen.findByTestId('floor-plan-status');
    expect(screen.queryByTestId('strand-new-floor')).toBeNull();
  });

  it('keeps the offer and the bytes when the new floor cannot be made', async () => {
    setFloorPlan.mockRejectedValueOnce(
      new Error('replacing the floor plan would strand the measurements taken on it: 3 …'),
    );
    createFloor.mockRejectedValueOnce(new Error('[unavailable] the daemon went away'));
    renderPanel();

    const file = new File([new Uint8Array([1, 2, 3])], 'ninth.png', { type: 'image/png' });
    fireEvent.change(screen.getByTestId('floor-plan-input'), { target: { files: [file] } });
    await screen.findByTestId('floor-plan-stranded-hint');

    fireEvent.change(screen.getByTestId('strand-new-floor-name'), { target: { value: 'Ninth' } });
    fireEvent.click(screen.getByTestId('strand-new-floor'));

    // What just went wrong, not the refusal that is still standing behind it.
    await waitFor(() =>
      expect(screen.getByTestId('floor-plan-status')).toHaveTextContent('the daemon went away'),
    );
    // And the offer survives with the file already chosen: making the operator
    // re-pick it is the thing this route exists to avoid.
    expect(screen.getByTestId('strand-new-floor')).toBeInTheDocument();

    createFloor.mockResolvedValueOnce({ floor: { id: 'flr-9', name: 'Ninth', level: 1 } });
    fireEvent.click(screen.getByTestId('strand-new-floor'));
    await waitFor(() => expect(setFloorPlan).toHaveBeenCalledTimes(2));
    expect(setFloorPlan.mock.calls[1]?.[0]).toMatchObject({ floorId: 'flr-9' });
  });
});
