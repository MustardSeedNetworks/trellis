/**
 * The measured surface.
 *
 * The reason this component exists is that the value under the cursor cannot
 * be read from the picture: heat is composited at partial alpha over the floor
 * plan, so the pixel colour is not the colour the scale chose. These pin that
 * the readout comes from the grid, that it is the cell the cursor is actually
 * over, and that positions with no measurement say nothing rather than zero.
 */
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { HeatmapSurface } from './HeatmapSurface';

/** A 400x300 image over a 4x3 grid of 100 px cells. */
const grid = [-40, -50, -60, -70, -45, -55, -65, -75, -50, -60, -70, -80];

function renderSurface(overrides: Partial<Parameters<typeof HeatmapSurface>[0]> = {}) {
  return render(
    <HeatmapSurface
      png={new Uint8Array([0x89, 0x50, 0x4e, 0x47])}
      width={400}
      height={300}
      grid={grid}
      gridCols={4}
      gridRows={3}
      cellSize={100}
      unit="dBm"
      metric="rssi"
      {...overrides}
    />,
  );
}

/**
 * jsdom lays nothing out, so every getBoundingClientRect is 0x0 and a pointer
 * position could not be mapped. This gives the image the box the browser would.
 */
function layOutImage(left = 0, top = 0, width = 400, height = 300) {
  const image = screen.getByTestId('heatmap-image');
  image.getBoundingClientRect = () =>
    ({
      left,
      top,
      width,
      height,
      right: left + width,
      bottom: top + height,
      x: left,
      y: top,
    }) as DOMRect;
  return image;
}

describe('HeatmapSurface', () => {
  it('reads the value of the cell under the pointer, not of the image', () => {
    renderSurface();
    const image = layOutImage();

    // (250, 150) is column 2, row 1 — the cell holding -65.
    fireEvent.mouseMove(image, { clientX: 250, clientY: 150 });

    expect(screen.getByTestId('heatmap-readout')).toHaveTextContent('-65.0 dBm at 250, 150');
  });

  it('follows the pointer across cells rather than reporting one value', () => {
    renderSurface();
    const image = layOutImage();

    fireEvent.mouseMove(image, { clientX: 10, clientY: 10 });
    expect(screen.getByTestId('heatmap-readout')).toHaveTextContent('-40.0 dBm');

    fireEvent.mouseMove(image, { clientX: 390, clientY: 290 });
    expect(screen.getByTestId('heatmap-readout')).toHaveTextContent('-80.0 dBm');
  });

  it('maps the pointer through the rendered size, not the image size', () => {
    renderSurface();
    // Half-size in the panel: a client position is half the image position, so
    // reading the raw offset would report the wrong cell.
    const image = layOutImage(0, 0, 200, 150);

    fireEvent.mouseMove(image, { clientX: 125, clientY: 75 });

    // 125 of 200 across a 400 px image is x=250, which is column 2, row 1.
    expect(screen.getByTestId('heatmap-readout')).toHaveTextContent('-65.0 dBm at 250, 150');
  });

  it('says nothing where there is no measurement instead of zero', () => {
    // A grid narrower than the image: the right edge is past the last cell.
    renderSurface({ gridCols: 2, gridRows: 3, grid: [-40, -50, -45, -55, -50, -60] });
    const image = layOutImage();

    fireEvent.mouseMove(image, { clientX: 390, clientY: 150 });

    const readout = screen.getByTestId('heatmap-readout');
    expect(readout).not.toHaveTextContent('0.0 dBm');
    expect(readout).toHaveTextContent('Point at the surface');
  });

  it('drops the reading when the pointer leaves', () => {
    renderSurface();
    const image = layOutImage();

    fireEvent.mouseMove(image, { clientX: 250, clientY: 150 });
    expect(screen.getByTestId('heatmap-readout')).toHaveTextContent('-65.0 dBm');

    fireEvent.mouseLeave(image);
    expect(screen.getByTestId('heatmap-readout')).toHaveTextContent('Point at the surface');
  });

  it('zooms from the buttons and returns to fit', () => {
    renderSurface();
    const viewport = screen.getByTestId('heatmap-viewport');

    expect(viewport).toHaveAttribute('data-zoom', '1');
    expect(screen.getByTestId('zoom-out')).toBeDisabled();
    expect(screen.getByTestId('zoom-reset')).toBeDisabled();

    fireEvent.click(screen.getByTestId('zoom-in'));
    fireEvent.click(screen.getByTestId('zoom-in'));
    expect(viewport).toHaveAttribute('data-zoom', '1.5');
    expect(screen.getByTestId('zoom-level')).toHaveTextContent('150%');
    expect(screen.getByTestId('heatmap-image')).toHaveStyle({ width: '150%' });

    fireEvent.click(screen.getByTestId('zoom-reset'));
    expect(viewport).toHaveAttribute('data-zoom', '1');
  });

  it('stops at the zoom bounds', () => {
    renderSurface();
    const zoomIn = screen.getByTestId('zoom-in');

    for (let i = 0; i < 20; i += 1) {
      fireEvent.click(zoomIn);
    }

    expect(screen.getByTestId('heatmap-viewport')).toHaveAttribute('data-zoom', '4');
    expect(zoomIn).toBeDisabled();
  });

  it('names the zoom controls for a screen reader', () => {
    renderSurface();

    expect(screen.getByRole('button', { name: 'Zoom in' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Zoom out' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reset zoom to fit' })).toBeInTheDocument();
    // The readout is announced, not only drawn.
    expect(screen.getByTestId('heatmap-readout')).toHaveAttribute('aria-live', 'polite');
  });

  it('reports nothing rather than crashing when the service sent no grid', () => {
    renderSurface({ grid: [], gridCols: 0, gridRows: 0, cellSize: 0 });
    const image = layOutImage();

    fireEvent.mouseMove(image, { clientX: 250, clientY: 150 });

    expect(screen.getByTestId('heatmap-readout')).toHaveTextContent('Point at the surface');
  });
});
