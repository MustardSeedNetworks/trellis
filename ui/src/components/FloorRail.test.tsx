/**
 * A survey opened to be walked has one floor, and a building has more. What
 * these pin is that the rail can add the others and move the walk onto one:
 * a floor that cannot be switched to cannot be measured.
 */
import { create } from '@bufbuild/protobuf';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { FloorSchema } from '@/gen/trellis/survey/v1/survey_pb';
import { FloorRail } from './FloorRail';

const createFloor = vi.fn();
const setActiveFloor = vi.fn();
const updateFloor = vi.fn();
const deleteFloor = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    createFloor: (req: unknown) => createFloor(req),
    setActiveFloor: (req: unknown) => setActiveFloor(req),
    updateFloor: (req: unknown) => updateFloor(req),
    deleteFloor: (req: unknown) => deleteFloor(req),
  },
}));

const ground = create(FloorSchema, {
  id: 'flr-1',
  name: 'Floor 1',
  level: 0,
  sampleCount: 12,
  hasFloorPlan: true,
  isActive: true,
});

const floors = [
  ground,
  create(FloorSchema, { id: 'flr-2', name: 'Basement', level: -1 }),
];

function renderRail(rows = floors) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  }
  return render(<FloorRail surveyId="svy-9" floors={rows} />, { wrapper });
}

beforeEach(() => {
  createFloor.mockReset();
  setActiveFloor.mockReset();
  updateFloor.mockReset();
  deleteFloor.mockReset();
  updateFloor.mockResolvedValue({ floor: { id: 'flr-2', name: 'Lower ground', level: 0 } });
  deleteFloor.mockResolvedValue({ floors: [ground] });
  createFloor.mockResolvedValue({
    floor: { id: 'flr-3', name: 'Mezzanine', level: 2, sampleCount: 0, isActive: false },
  });
  setActiveFloor.mockResolvedValue({ floor: { id: 'flr-2', isActive: true } });
});

describe('FloorRail', () => {
  it('lists every floor and marks the one being walked', () => {
    renderRail();

    expect(screen.getByTestId('floor-row-flr-1')).toHaveTextContent('Floor 1');
    expect(screen.getByTestId('floor-row-flr-2')).toHaveTextContent('Basement');
    // The active floor offers no "walk this floor" button — it is the walk.
    expect(screen.queryByTestId('walk-floor-flr-1')).not.toBeInTheDocument();
    expect(screen.getByTestId('walk-floor-flr-2')).toBeInTheDocument();
  });

  it('creates a floor with the name and level given', async () => {
    renderRail();

    fireEvent.change(screen.getByTestId('floor-name-input'), { target: { value: 'Mezzanine' } });
    fireEvent.change(screen.getByTestId('floor-level-input'), { target: { value: '2' } });
    fireEvent.click(screen.getByTestId('create-floor'));

    await waitFor(() => expect(createFloor).toHaveBeenCalled());
    expect(createFloor.mock.calls[0]?.[0]).toMatchObject({
      surveyId: 'svy-9',
      name: 'Mezzanine',
      level: 2,
    });
  });

  it('refuses to submit an unnamed floor without calling the service', () => {
    renderRail();

    fireEvent.click(screen.getByTestId('create-floor'));

    // The handler rejects it too; not sending it keeps the refusal immediate
    // rather than a round trip that says the same thing.
    expect(createFloor).not.toHaveBeenCalled();
  });

  it('moves the walk onto another floor', async () => {
    renderRail();

    fireEvent.click(screen.getByTestId('walk-floor-flr-2'));

    await waitFor(() => expect(setActiveFloor).toHaveBeenCalled());
    expect(setActiveFloor.mock.calls[0]?.[0]).toMatchObject({
      surveyId: 'svy-9',
      floorId: 'flr-2',
    });
  });

  it('renames a floor with the level it is given', async () => {
    renderRail();

    fireEvent.click(screen.getByTestId('rename-floor-flr-2'));
    fireEvent.change(screen.getByTestId('floor-rename-input-flr-2'), {
      target: { value: 'Lower ground' },
    });
    fireEvent.change(screen.getByTestId('floor-relevel-input-flr-2'), { target: { value: '0' } });
    fireEvent.click(screen.getByTestId('floor-rename-save-flr-2'));

    await waitFor(() => expect(updateFloor).toHaveBeenCalled());
    // Both fields travel on every call: a ground floor is level 0, which the
    // wire cannot tell from an absent field.
    expect(updateFloor.mock.calls[0]?.[0]).toMatchObject({
      surveyId: 'svy-9',
      floorId: 'flr-2',
      name: 'Lower ground',
      level: 0,
    });
  });

  it('does not delete a floor until the confirmation is clicked', async () => {
    renderRail();

    fireEvent.click(screen.getByTestId('delete-floor-flr-2'));
    expect(deleteFloor).not.toHaveBeenCalled();

    // A floor holds measurements, so the destructive click is the second one —
    // the same two-step the survey delete uses.
    fireEvent.click(screen.getByTestId('delete-floor-confirm-flr-2'));
    await waitFor(() => expect(deleteFloor).toHaveBeenCalled());
    expect(deleteFloor.mock.calls[0]?.[0]).toMatchObject({
      surveyId: 'svy-9',
      floorId: 'flr-2',
    });
  });

  it('confirms one floor at a time', () => {
    renderRail();

    fireEvent.click(screen.getByTestId('delete-floor-flr-2'));

    // Arming the basement must not arm the ground floor as well: a shared
    // boolean would offer to delete every row at once.
    expect(screen.queryByTestId('delete-floor-confirm-flr-1')).not.toBeInTheDocument();
    expect(screen.getByTestId('delete-floor-confirm-flr-2')).toBeInTheDocument();
  });

  it('offers no delete on the last floor', () => {
    renderRail([ground]);

    // The handler refuses it; a button that can only fail is worse than none.
    expect(screen.queryByTestId('delete-floor-flr-1')).not.toBeInTheDocument();
  });
});
