/**
 * The marker layer over a plan. What it must get right is the coordinate
 * space: a click or a drag is wherever the plan is drawn, and what reaches the
 * service is a pixel of the plan itself, never one past its edge.
 */
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { PlacementLayer } from './PlacementLayer';

const placements = [
  { id: 'ap-1', x: 200, y: 150, label: '', band: '5 GHz' },
  { id: 'ap-2', x: 600, y: 450, label: 'Lobby', band: '' },
];

/** An 800x600 plan drawn at half size, 10 px in from the viewport corner.
 *  jsdom lays nothing out, so the layer is given the box a browser would. */
function renderLayer() {
  const handlers = { onSelect: vi.fn(), onAdd: vi.fn(), onMove: vi.fn() };
  render(
    <PlacementLayer
      placements={placements}
      width={800}
      height={600}
      selectedId="ap-1"
      {...handlers}
    />,
  );
  vi.spyOn(screen.getByTestId('placement-layer'), 'getBoundingClientRect').mockReturnValue({
    left: 10,
    top: 10,
    width: 400,
    height: 300,
    right: 410,
    bottom: 310,
    x: 10,
    y: 10,
    toJSON: () => ({}),
  });
  return handlers;
}

describe('PlacementLayer', () => {
  it('positions each marker as a share of the plan, and names it', () => {
    renderLayer();
    const [first, second] = screen.getAllByTestId('ap-marker');
    expect(first).toHaveStyle({ left: '25%', top: '25%' });
    expect(first).toHaveAttribute('aria-pressed', 'true');
    expect(second).toHaveAccessibleName('Lobby: drag to move, or use the arrow keys');
    expect(first).toHaveAccessibleName('AP 1: drag to move, or use the arrow keys');
  });

  it('places an AP at the plan pixel that was clicked', () => {
    const { onAdd } = renderLayer();
    fireEvent.click(screen.getByTestId('placement-surface'), {
      clientX: 110,
      clientY: 60,
      detail: 1,
    });
    expect(onAdd).toHaveBeenCalledWith({ x: 200, y: 100 });
  });

  it('places an AP at the centre from the keyboard', () => {
    const { onAdd } = renderLayer();
    fireEvent.click(screen.getByTestId('placement-surface'), { detail: 0 });
    expect(onAdd).toHaveBeenCalledWith({ x: 400, y: 300 });
  });

  it('sends one move per drag, stopped on the last pixel of the plan', () => {
    const { onMove } = renderLayer();
    const marker = screen.getAllByTestId('ap-marker')[0] as HTMLElement;
    fireEvent.pointerDown(marker, { pointerId: 1, clientX: 110, clientY: 85 });
    fireEvent.pointerMove(marker, { pointerId: 1, clientX: 300, clientY: 200 });
    fireEvent.pointerMove(marker, { pointerId: 1, clientX: 900, clientY: 900 });
    expect(marker).toHaveAttribute('data-x', '799');
    expect(onMove).not.toHaveBeenCalled();

    fireEvent.pointerUp(marker, { pointerId: 1 });
    expect(onMove).toHaveBeenCalledTimes(1);
    expect(onMove).toHaveBeenCalledWith('ap-1', { x: 799, y: 599 });
  });

  it('selects a marker that was pressed without moving', () => {
    const { onMove, onSelect } = renderLayer();
    const marker = screen.getAllByTestId('ap-marker')[1] as HTMLElement;
    fireEvent.pointerDown(marker, { pointerId: 1 });
    fireEvent.pointerUp(marker, { pointerId: 1 });
    expect(onMove).not.toHaveBeenCalled();
    expect(onSelect).toHaveBeenCalledWith('ap-2');
  });

  it('nudges a marker with the arrow keys, further with Shift', () => {
    const { onMove } = renderLayer();
    const [first, second] = screen.getAllByTestId('ap-marker') as HTMLElement[];
    fireEvent.keyDown(first as HTMLElement, { key: 'ArrowRight' });
    expect(onMove).toHaveBeenLastCalledWith('ap-1', { x: 205, y: 150 });
    fireEvent.keyDown(second as HTMLElement, { key: 'ArrowDown', shiftKey: true });
    expect(onMove).toHaveBeenLastCalledWith('ap-2', { x: 600, y: 475 });
  });
});
