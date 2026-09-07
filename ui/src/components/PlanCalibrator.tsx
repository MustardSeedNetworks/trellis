import { type KeyboardEvent, type MouseEvent, useState } from 'react';
import { useTranslation } from 'react-i18next';

interface Point {
  x: number;
  y: number;
}

interface PlanCalibratorProps {
  plan: { width: number; height: number; imageUrl: string };
  /** Applies the line the operator marked. Pixels are the plan's own. */
  onApply: (line: { from: Point; to: Point; metres: number }) => void;
  pending: boolean;
}

/** Arrow keys move the cursor by this much; with Shift, five times it. */
const KEY_STEP = 5;
const KEY_STEP_FAST = 25;

/**
 * PlanCalibrator — mark two points on the plan and say how far apart they are.
 *
 * A plan carries no scale of its own: exported at some arbitrary resolution, it
 * has no relationship to the building until somebody says what one line on it
 * is. The first attempt at this asked for the width of the whole plan and
 * measured edge to edge, which is wrong twice over — a plan is usually cropped
 * or padded, so its outer edge is not a wall anybody can pace, and the operator
 * is asked for a number they cannot check. What they *can* check is a thing
 * they can measure: a corridor, a doorway, a dimension line printed on the
 * drawing itself.
 *
 * So the operator marks its two ends and types the distance between them. It is
 * also what the RPC has always taken — two points and a length — which the
 * previous UI supplied as `(0,0)` to `(width,0)`.
 *
 * The whole interaction is reachable from the keyboard, not only the mouse:
 * arrow keys move a cursor and Enter places a point. A calibration only a
 * pointer can perform would leave the scale — and every distance derived from
 * it — out of reach of an operator who cannot use one.
 */
export function PlanCalibrator({ plan, onApply, pending }: PlanCalibratorProps) {
  const { t } = useTranslation(['common', 'pages']);
  const [from, setFrom] = useState<Point | undefined>();
  const [to, setTo] = useState<Point | undefined>();
  const [cursor, setCursor] = useState<Point>({
    x: Math.round(plan.width / 2),
    y: Math.round(plan.height / 2),
  });
  const [focused, setFocused] = useState(false);
  const [metres, setMetres] = useState('10');

  function place(point: Point) {
    const clamped = clampTo(point, plan.width, plan.height);
    // The third click starts a new line rather than adding to the old one:
    // marking a corridor and then deciding a doorway is easier to measure is
    // the ordinary way this goes.
    if (from === undefined || to !== undefined) {
      setFrom(clamped);
      setTo(undefined);
      return;
    }
    setTo(clamped);
  }

  function handleClick(event: MouseEvent<HTMLButtonElement>) {
    // Keyboard activation reaches here as a click with no pointer; the cursor
    // is where the arrow keys left it.
    if (event.detail === 0) {
      place(cursor);
      return;
    }
    const rect = event.currentTarget.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) {
      return;
    }
    const point = {
      x: Math.round(((event.clientX - rect.left) / rect.width) * plan.width),
      y: Math.round(((event.clientY - rect.top) / rect.height) * plan.height),
    };
    setCursor(clampTo(point, plan.width, plan.height));
    place(point);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    const step = event.shiftKey ? KEY_STEP_FAST : KEY_STEP;
    const moves: Record<string, Point> = {
      ArrowUp: { x: 0, y: -step },
      ArrowDown: { x: 0, y: step },
      ArrowLeft: { x: -step, y: 0 },
      ArrowRight: { x: step, y: 0 },
    };
    const move = moves[event.key];
    if (move) {
      event.preventDefault();
      setCursor((current) =>
        clampTo({ x: current.x + move.x, y: current.y + move.y }, plan.width, plan.height),
      );
    }
  }

  const pixels = from && to ? Math.hypot(to.x - from.x, to.y - from.y) : 0;
  const distance = Number(metres);
  const ready = pixels > 0 && distance > 0 && !pending;

  return (
    <div className="flex flex-col gap-3" data-testid="plan-calibrator">
      <button
        type="button"
        onClick={handleClick}
        onKeyDown={handleKeyDown}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        data-testid="calibration-surface"
        aria-label={t('pages:surveys.calibrationSurface')}
        className="block w-full rounded border border-hairline bg-surface-base p-0"
      >
        <svg
          viewBox={`0 0 ${plan.width} ${plan.height}`}
          aria-hidden="true"
          className="block w-full"
        >
          <image href={plan.imageUrl} x={0} y={0} width={plan.width} height={plan.height} />
          {from && to ? (
            <line
              x1={from.x}
              y1={from.y}
              x2={to.x}
              y2={to.y}
              className="stroke-brand-primary"
              strokeWidth={2}
              data-testid="calibration-line"
            />
          ) : null}
          {[from, to].map((point, index) =>
            point ? (
              <circle
                // Two marks with fixed roles; there is no list to reorder.
                key={index === 0 ? 'from' : 'to'}
                cx={point.x}
                cy={point.y}
                r={5}
                className="fill-brand-primary"
                data-testid="calibration-mark"
              />
            ) : null,
          )}
          {focused ? (
            <g className="stroke-brand-accent" strokeWidth={1.5} data-testid="calibration-cursor">
              <line x1={cursor.x - 10} y1={cursor.y} x2={cursor.x + 10} y2={cursor.y} />
              <line x1={cursor.x} y1={cursor.y - 10} x2={cursor.x} y2={cursor.y + 10} />
            </g>
          ) : null}
        </svg>
      </button>

      <p className="text-sm text-text-secondary" data-testid="calibration-prompt">
        {from === undefined
          ? t('pages:surveys.calibrationMarkFirst')
          : to === undefined
            ? t('pages:surveys.calibrationMarkSecond')
            : t('pages:surveys.calibrationLine', { pixels: Math.round(pixels) })}
      </p>

      <div className="flex flex-wrap items-end gap-3">
        <label className="flex flex-col gap-1 text-sm" htmlFor="calibration-metres">
          <span className="kicker">{t('pages:surveys.calibrationDistance')}</span>
          <input
            id="calibration-metres"
            type="number"
            min={0.1}
            step={0.1}
            value={metres}
            onChange={(event) => setMetres(event.target.value)}
            className="figure w-28 rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary"
            data-testid="calibration-metres"
          />
        </label>
        <button
          type="button"
          disabled={!ready}
          onClick={() => {
            if (from && to) {
              onApply({ from, to, metres: distance });
            }
          }}
          data-testid="calibrate-floor-plan"
          className="rounded border border-hairline px-3 py-2 text-sm text-text-primary hover:bg-surface-raised disabled:opacity-50"
        >
          {t('pages:surveys.calibrate')}
        </button>
      </div>
    </div>
  );
}

function clampTo(point: Point, width: number, height: number): Point {
  return {
    x: Math.min(Math.max(point.x, 0), width),
    y: Math.min(Math.max(point.y, 0), height),
  };
}
