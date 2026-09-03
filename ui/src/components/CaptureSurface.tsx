import { useMutation, useQueryClient } from '@tanstack/react-query';
import { type KeyboardEvent, type MouseEvent, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { surveyClient } from '@/lib/client';
import { formatSignal } from '@/lib/format';

interface CaptureSurfaceProps {
  surveyId: string;
  surveyName: string;
}

/**
 * The surface is drawn in the survey's own coordinate space, which is the
 * floor-plan pixel space imported samples use. There is no plan to draw yet —
 * floor import and scale calibration come later — so the space is this fixed
 * canvas and the heatmap sizes itself to the points captured on it.
 */
export const SURFACE_WIDTH = 800;
export const SURFACE_HEIGHT = 500;

/** Signal at or above this is a comfortable point; below `WEAK_DBM` is a weak one. */
const GOOD_DBM = -60;
const WEAK_DBM = -75;

/** Arrow keys move the keyboard cursor by this much; with Shift, five times it. */
const KEY_STEP = 10;
const KEY_STEP_FAST = 50;

interface Pin {
  /** Sequence within this session; the same spot can be sampled twice. */
  seq: number;
  x: number;
  y: number;
  networkCount: number;
  strongestDbm: number | undefined;
}

interface Point {
  x: number;
  y: number;
}

/**
 * CaptureSurface is the stop-and-go walk: the operator stands at a point,
 * clicks where they are, and the radio samples there. One click is one point,
 * matching the unary RPC — an active scan takes seconds and cannot be
 * cancelled, so further clicks are ignored until it returns rather than queued.
 *
 * The keyboard gets the same walk: the surface takes focus, the arrow keys move
 * a cursor across it and Enter or Space captures there. A surface that only a
 * mouse can drive would leave the product's central action unreachable.
 *
 * Pins live in component state for this session only. The list RPC carries
 * a survey's sample *count* and not its samples, so the count in the survey's
 * facts is the stored truth and these pins are the walk as it was seen from
 * here. The service stores the point; a reload keeps the count and loses the
 * dots, which is a read-path gap for a later change, not a lost measurement.
 */
export function CaptureSurface({ surveyId, surveyName }: CaptureSurfaceProps) {
  const { t } = useTranslation(['common', 'pages']);
  const queryClient = useQueryClient();
  const [pins, setPins] = useState<Pin[]>([]);
  const [cursor, setCursor] = useState<Point>({ x: SURFACE_WIDTH / 2, y: SURFACE_HEIGHT / 2 });
  const [focused, setFocused] = useState(false);

  const captureMutation = useMutation({
    mutationFn: (point: Point) => surveyClient.capturePoint({ surveyId, x: point.x, y: point.y }),
    onSuccess: async (reply, point) => {
      // Strongest first is the service's contract for networks[0].
      const strongest = reply.networks[0];
      setPins((current) => [
        ...current,
        {
          seq: current.length + 1,
          x: point.x,
          y: point.y,
          networkCount: reply.networks.length,
          strongestDbm: strongest?.signalDbm,
        },
      ]);
      await queryClient.invalidateQueries({ queryKey: ['surveys'] });
    },
  });

  function capture(point: Point) {
    if (captureMutation.isPending) {
      return;
    }
    captureMutation.mutate(clamp(point));
  }

  function handleClick(event: MouseEvent<HTMLButtonElement>) {
    // Enter and Space arrive as a click too, with no pointer behind it: the
    // browser reports detail 0 for a keyboard activation. That capture goes at
    // the cursor; a pointer click goes where it landed.
    if (event.detail === 0) {
      capture(cursor);
      return;
    }
    // The surface scales to its container; the service wants integer
    // coordinates in the surface's own space, so map the click back through
    // the viewBox.
    const rect = event.currentTarget.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) {
      return;
    }
    const point = {
      x: Math.round(((event.clientX - rect.left) / rect.width) * SURFACE_WIDTH),
      y: Math.round(((event.clientY - rect.top) / rect.height) * SURFACE_HEIGHT),
    };
    setCursor(clamp(point));
    capture(point);
  }

  // Arrow keys only; Enter and Space are the button's own activation and reach
  // handleClick as a click with no pointer.
  function handleKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    const step = event.shiftKey ? KEY_STEP_FAST : KEY_STEP;
    const moves: Record<string, Point> = {
      ArrowLeft: { x: -step, y: 0 },
      ArrowRight: { x: step, y: 0 },
      ArrowUp: { x: 0, y: -step },
      ArrowDown: { x: 0, y: step },
    };
    const move = moves[event.key];
    if (move) {
      event.preventDefault();
      setCursor((current) => clamp({ x: current.x + move.x, y: current.y + move.y }));
    }
  }

  const last = pins.at(-1);
  const status = captureMutation.isPending
    ? t('pages:surveys.capturing')
    : captureMutation.isError
      ? t('pages:surveys.captureFailed', { error: String(captureMutation.error) })
      : last
        ? t('pages:surveys.captured', {
            count: last.networkCount,
            x: last.x,
            y: last.y,
            signal: signalText(last.strongestDbm),
          })
        : t('pages:surveys.noPoints');

  return (
    <section className="flex flex-col gap-3" aria-labelledby="capture-title">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 id="capture-title" className="kicker">
          {t('pages:surveys.captureTitle')}
        </h3>
        <span className="text-xs text-text-muted">
          {t('pages:surveys.pointsThisSession', { count: pins.length })}
        </span>
      </div>
      <p className="text-sm text-text-secondary">{t('pages:surveys.captureHint')}</p>

      {/* One button, not a clickable picture: the whole surface is a single
          control whose activation samples at a position — where the pointer
          landed, or at the cursor for a keyboard activation. It is not
          disabled while a scan runs: a disabled button drops focus, and the
          click is ignored in capture() instead. */}
      <button
        type="button"
        aria-label={t('pages:surveys.surfaceLabel', { name: surveyName })}
        aria-busy={captureMutation.isPending}
        onClick={handleClick}
        onKeyDown={handleKeyDown}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        className={`block w-full rounded-[12px] border border-hairline bg-surface-sunken p-0 focus:outline-2 focus:outline-brand-primary ${
          captureMutation.isPending ? 'cursor-progress' : 'cursor-crosshair'
        }`}
        data-testid="capture-surface"
      >
        <svg
          viewBox={`0 0 ${SURFACE_WIDTH} ${SURFACE_HEIGHT}`}
          aria-hidden="true"
          className="block w-full"
        >
          {focused ? (
            <g className="stroke-brand-primary" strokeWidth={1.5} data-testid="capture-cursor">
              <line x1={cursor.x - 14} y1={cursor.y} x2={cursor.x + 14} y2={cursor.y} />
              <line x1={cursor.x} y1={cursor.y - 14} x2={cursor.x} y2={cursor.y + 14} />
            </g>
          ) : null}
          {pins.map((pin) => (
            <g key={pin.seq} data-testid="capture-pin">
              <circle
                cx={pin.x}
                cy={pin.y}
                r={9}
                className={`${pinClass(pin.strongestDbm)} stroke-surface-raised`}
                strokeWidth={2}
              />
              <text x={pin.x + 14} y={pin.y + 4} className="figure fill-text-primary text-[13px]">
                {signalText(pin.strongestDbm)}
              </text>
            </g>
          ))}
        </svg>
      </button>

      <p
        className={`text-sm ${captureMutation.isError ? 'text-status-error' : 'text-text-secondary'}`}
        data-testid="capture-status"
      >
        {status}
      </p>
    </section>
  );
}

function clamp(point: Point): Point {
  return {
    x: Math.min(Math.max(point.x, 0), SURFACE_WIDTH),
    y: Math.min(Math.max(point.y, 0), SURFACE_HEIGHT),
  };
}

function signalText(strongestDbm: number | undefined): string {
  return strongestDbm === undefined ? '—' : formatSignal(strongestDbm, 'dBm');
}

function pinClass(strongestDbm: number | undefined): string {
  if (strongestDbm === undefined) {
    return 'fill-text-muted';
  }
  if (strongestDbm >= GOOD_DBM) {
    return 'fill-status-success';
  }
  if (strongestDbm >= WEAK_DBM) {
    return 'fill-status-warning';
  }
  return 'fill-status-error';
}
