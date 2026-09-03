import { useCallback, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { bytesToDataUrl, formatSignal } from '@/lib/format';

/** Zoom bounds and step. 1 is fit-to-panel; 4 is where a 10 px cell is a
 *  readable block on a laptop panel. */
const MIN_ZOOM = 1;
const MAX_ZOOM = 4;
const ZOOM_STEP = 0.25;

export interface HeatmapSurfaceProps {
  png: Uint8Array;
  width: number;
  height: number;
  /** Row-major, one value per cellSize-by-cellSize block. */
  grid: number[];
  gridCols: number;
  gridRows: number;
  cellSize: number;
  /** "dBm" for rssi, "dB" for snr. */
  unit: string;
  metric: string;
}

/**
 * The measured surface: the rendered heatmap, the value under the pointer, and
 * zoom.
 *
 * The readout comes from the grid the service painted with, not from the
 * pixel under the cursor. Heat is composited at partial alpha over the floor
 * plan, so the colour on screen is not the colour the scale chose and reading
 * a value back from it would be confidently wrong.
 */
export function HeatmapSurface({
  png,
  width,
  height,
  grid,
  gridCols,
  gridRows,
  cellSize,
  unit,
  metric,
}: HeatmapSurfaceProps) {
  const { t } = useTranslation(['common', 'pages']);
  const imageRef = useRef<HTMLImageElement>(null);
  const [zoom, setZoom] = useState(MIN_ZOOM);
  const [reading, setReading] = useState<{ value: number; x: number; y: number } | null>(null);

  const readAt = useCallback(
    (clientX: number, clientY: number) => {
      const image = imageRef.current;
      if (!image || gridCols === 0 || gridRows === 0 || cellSize <= 0) {
        return;
      }

      /* The image is laid out at whatever width the panel allows and then
         scaled by the zoom transform, so a client position has to come back
         through both before it means an image pixel. Deriving the factor from
         the rendered box covers the two together. */
      const box = image.getBoundingClientRect();
      if (box.width === 0 || box.height === 0) {
        return;
      }
      const x = Math.floor(((clientX - box.left) / box.width) * width);
      const y = Math.floor(((clientY - box.top) / box.height) * height);

      const col = Math.floor(x / cellSize);
      const row = Math.floor(y / cellSize);
      /* Off the grid is not zero. The pointer can sit inside the image and
         past the last whole cell, and printing 0 dBm there would be a
         measurement nobody took. */
      if (col < 0 || col >= gridCols || row < 0 || row >= gridRows) {
        setReading(null);
        return;
      }

      const value = grid[row * gridCols + col];
      if (value === undefined) {
        setReading(null);
        return;
      }
      setReading({ value, x, y });
    },
    [cellSize, grid, gridCols, gridRows, height, width],
  );

  function changeZoom(delta: number) {
    setZoom((current) => {
      const next = Number((current + delta).toFixed(2));
      return Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, next));
    });
  }

  return (
    /* The controls sit inside the scrolling region rather than above it. A
       region that scrolls must be reachable from a keyboard, and the honest
       way to satisfy that is real focusable content inside it — tabIndex on
       the container itself is a non-interactive element pretending to be one.
       With the buttons in here, Tab reaches them and the arrow keys then pan
       the region they are in. Sticky keeps them in view while it scrolls. */
    <section
      className="max-h-[70vh] overflow-auto rounded-[12px] border border-hairline"
      aria-label={t('pages:coverage.surfaceRegion')}
      data-testid="heatmap-viewport"
      data-zoom={zoom}
    >
      <div className="sticky top-0 z-10 flex flex-wrap items-center gap-2 border-b border-hairline bg-surface-raised px-3 py-2">
        <span className="kicker">{t('pages:coverage.zoom')}</span>
        <div className="flex gap-1">
          <button
            type="button"
            onClick={() => changeZoom(-ZOOM_STEP)}
            disabled={zoom <= MIN_ZOOM}
            aria-label={t('pages:coverage.zoomOut')}
            className="rounded-[9px] px-3 py-2 text-sm font-bold text-text-secondary hover:bg-surface-hover disabled:opacity-50"
            data-testid="zoom-out"
          >
            −
          </button>
          <button
            type="button"
            onClick={() => changeZoom(ZOOM_STEP)}
            disabled={zoom >= MAX_ZOOM}
            aria-label={t('pages:coverage.zoomIn')}
            className="rounded-[9px] px-3 py-2 text-sm font-bold text-text-secondary hover:bg-surface-hover disabled:opacity-50"
            data-testid="zoom-in"
          >
            +
          </button>
          <button
            type="button"
            onClick={() => setZoom(MIN_ZOOM)}
            disabled={zoom === MIN_ZOOM}
            aria-label={t('pages:coverage.zoomReset')}
            className="rounded-[9px] px-3 py-2 text-sm text-text-secondary hover:bg-surface-hover disabled:opacity-50"
            data-testid="zoom-reset"
          >
            {t('pages:coverage.zoomFit')}
          </button>
        </div>
        <span className="figure text-xs text-text-muted" data-testid="zoom-level">
          {`${Math.round(zoom * 100)}%`}
        </span>

        {/* Announced rather than only drawn: a value that exists solely as a
            tooltip is invisible to a screen reader and to a keyboard user. */}
        <output
          aria-live="polite"
          className="figure ml-auto text-xs text-text-secondary"
          data-testid="heatmap-readout"
        >
          {reading
            ? t('pages:coverage.readingAt', {
                value: formatSignal(reading.value, unit),
                x: reading.x,
                y: reading.y,
              })
            : t('pages:coverage.readingPrompt')}
        </output>
      </div>

      <img
        ref={imageRef}
        src={bytesToDataUrl(png, 'image/png')}
        alt={t('pages:coverage.heatmapAlt', { metric: metric.toUpperCase() })}
        width={width}
        height={height}
        onMouseMove={(event) => readAt(event.clientX, event.clientY)}
        onMouseLeave={() => setReading(null)}
        style={{ width: `${zoom * 100}%` }}
        className="max-w-none"
        data-testid="heatmap-image"
      />
    </section>
  );
}
