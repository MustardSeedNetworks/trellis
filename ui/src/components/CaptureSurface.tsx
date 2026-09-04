import { timestampMs } from '@bufbuild/protobuf/wkt';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { type KeyboardEvent, type MouseEvent, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { CaptureStatus, SurveySample } from '@/gen/trellis/survey/v1/survey_pb';
import { surveyClient } from '@/lib/client';
import { formatSignal } from '@/lib/format';

interface CaptureSurfaceProps {
  surveyId: string;
  surveyName: string;
  /** The iperf3 server active measurements run against; empty when none is set. */
  throughputTarget: string;
  /** Only a survey in progress accepts a point; otherwise the surface is a picture. */
  walking: boolean;
  /** The survey's continuous capture, from the summary. Absent when it has never had one. */
  capture?: CaptureStatus;
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

/** How often the stored points are re-read while a continuous capture runs. */
const WALK_POLL_MS = 2000;

/** Arrow keys move the keyboard cursor by this much; with Shift, five times it. */
const KEY_STEP = 10;
const KEY_STEP_FAST = 50;

interface Point {
  x: number;
  y: number;
}

/**
 * CaptureSurface is both survey modes over one surface.
 *
 * **Stop-and-go**: the operator stands at a point, clicks where they are, and
 * the radio samples there. One click is one point, matching the unary RPC — an
 * active scan takes seconds and cannot be cancelled, so further clicks are
 * ignored until it returns rather than queued.
 *
 * **Continuous**: the daemon samples repeatedly at the position the operator
 * last marked, and a click moves that position rather than taking a point. It is
 * the same gesture meaning "I am here now" instead of "sample here" — which is
 * what walking a floor actually is, and why the two modes share a surface
 * instead of being two controls that look alike.
 *
 * The mode is a toggle rather than two buttons because the two are exclusive on
 * one radio, and a surface offering both at once would invite a click that means
 * nothing.
 *
 * The keyboard gets the same walk: the surface takes focus, the arrow keys move
 * a cursor across it and Enter or Space captures there. A surface that only a
 * mouse can drive would leave the product's central action unreachable.
 *
 * The pins are the stored points, read back from the service, not a local
 * record of this session's clicks. A walk is interrupted by glancing at the
 * heatmap, by a reload, by the daemon restarting; the dots have to survive all
 * of those or the operator cannot see where they have already been.
 */
export function CaptureSurface({
  surveyId,
  surveyName,
  walking,
  capture,
  throughputTarget,
}: CaptureSurfaceProps) {
  const { t } = useTranslation(['common', 'pages']);
  const queryClient = useQueryClient();
  const [cursor, setCursor] = useState<Point>({ x: SURFACE_WIDTH / 2, y: SURFACE_HEIGHT / 2 });
  const [focused, setFocused] = useState(false);

  const capturing = capture?.running === true;
  const samplesQuery = useQuery({
    queryKey: ['samples', surveyId],
    queryFn: () => surveyClient.listSamples({ surveyId }),
    // Polled only while the daemon is walking: nothing else adds a point behind
    // this component's back, and pins that only appear on a reload would make a
    // running walk look stalled.
    refetchInterval: capturing ? WALK_POLL_MS : false,
  });
  const pins = samplesQuery.data?.samples ?? [];

  const captureMutation = useMutation({
    mutationFn: (point: Point) => surveyClient.capturePoint({ surveyId, x: point.x, y: point.y }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['samples', surveyId] }),
        queryClient.invalidateQueries({ queryKey: ['surveys'] }),
      ]);
    },
  });

  const walkMutation = useMutation({
    mutationFn: (point: Point) =>
      surveyClient.startContinuousCapture({ surveyId, x: point.x, y: point.y }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['surveys'] });
    },
  });

  // Active measurement: a throughput test at the cursor. Its own control rather
  // than a mode of the surface, because it is a different measurement — the
  // surface's click says where, and this says what to measure there. It runs
  // for seconds in each direction, so it is stop-and-go by nature and has no
  // continuous form.
  const throughputMutation = useMutation({
    mutationFn: (point: Point) =>
      surveyClient.measureThroughput({ surveyId, x: point.x, y: point.y }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['samples', surveyId] }),
        queryClient.invalidateQueries({ queryKey: ['surveys'] }),
      ]);
    },
  });

  const stopMutation = useMutation({
    mutationFn: () => surveyClient.stopContinuousCapture({ surveyId }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['samples', surveyId] }),
        queryClient.invalidateQueries({ queryKey: ['surveys'] }),
      ]);
    },
  });

  // While walking, activating the surface moves the capture; otherwise it takes
  // one point. The pending guard is only the one-shot mode's: moving a walking
  // capture is a position write, not a scan, so it does not have to wait for the
  // sweep in flight.
  function markPosition(point: Point) {
    if (!walking) {
      return;
    }
    if (capturing) {
      walkMutation.mutate(clamp(point));
      return;
    }
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
      markPosition(cursor);
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
    markPosition(point);
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

  const captured = captureMutation.data;
  const capturedAt = captureMutation.variables;
  // A walk that stopped by itself says why. Points that simply cease appearing,
  // with a calm surface above them, is the failure this reading exists for.
  const status = throughputMutation.isError
    ? t('pages:surveys.throughputFailed', { error: String(throughputMutation.error) })
    : throughputMutation.data
      ? t('pages:surveys.measured', {
          download: throughputMutation.data.reading?.downloadMbps.toFixed(1) ?? '0',
          upload: throughputMutation.data.reading?.uploadMbps.toFixed(1) ?? '0',
        })
      : capture && !capture.running && capture.lastError !== ''
        ? t('pages:surveys.walkStopped', { error: capture.lastError })
        : capturing
          ? t('pages:surveys.walkingAt', { x: capture.x, y: capture.y, count: pins.length })
          : captureMutation.isPending
            ? t('pages:surveys.capturing')
            : captureMutation.isError
              ? t('pages:surveys.captureFailed', { error: String(captureMutation.error) })
              : samplesQuery.isError
                ? t('pages:surveys.samplesFailed', { error: String(samplesQuery.error) })
                : captured && capturedAt
                  ? t('pages:surveys.captured', {
                      count: captured.networks.length,
                      x: capturedAt.x,
                      y: capturedAt.y,
                      signal: signalText(captured.networks[0]?.signalDbm),
                    })
                  : pins.length === 0
                    ? t('pages:surveys.noPoints')
                    : '';
  const failed =
    throughputMutation.isError ||
    captureMutation.isError ||
    samplesQuery.isError ||
    walkMutation.isError ||
    stopMutation.isError ||
    (capture !== undefined && !capture.running && capture.lastError !== '');

  return (
    <section className="flex flex-col gap-3" aria-labelledby="capture-title">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 id="capture-title" className="kicker">
          {t('pages:surveys.captureTitle')}
        </h3>
        <div className="flex items-center gap-3">
          <span className="text-xs text-text-muted" data-testid="capture-count">
            {t('pages:surveys.pointsOnFloor', { count: pins.length })}
          </span>
          {/* Only offered while the survey is walking: continuous capture on a
              paused or completed survey has nothing to write into. */}
          {/* Offered only when the survey names a server. A button that always
              fails is worse than an absent one, and the remedy — name a target
              — is not something a click can discover. */}
          {throughputTarget === '' ? null : (
            <button
              type="button"
              disabled={!walking || capturing || throughputMutation.isPending}
              onClick={() => throughputMutation.mutate(clamp(cursor))}
              data-testid="measure-throughput"
              className="rounded border border-hairline px-3 py-1 text-sm text-text-primary hover:bg-surface-raised disabled:opacity-50"
            >
              {throughputMutation.isPending
                ? t('pages:surveys.testing')
                : t('pages:surveys.throughputTest')}
            </button>
          )}
          <button
            type="button"
            disabled={!walking || walkMutation.isPending || stopMutation.isPending}
            onClick={() => (capturing ? stopMutation.mutate() : walkMutation.mutate(clamp(cursor)))}
            data-testid="toggle-continuous"
            className="rounded border border-hairline px-3 py-1 text-sm text-text-primary hover:bg-surface-raised disabled:opacity-50"
          >
            {capturing ? t('pages:surveys.stopWalking') : t('pages:surveys.startWalking')}
          </button>
        </div>
      </div>
      <p className="text-sm text-text-secondary">
        {!walking
          ? t('pages:surveys.captureHintIdle')
          : capturing
            ? t('pages:surveys.captureHintWalking')
            : t('pages:surveys.captureHint')}
      </p>

      {/* One button, not a clickable picture: the whole surface is a single
          control whose activation samples at a position — where the pointer
          landed, or at the cursor for a keyboard activation. It is not
          disabled while a scan runs: a disabled button drops focus, and the
          click is ignored in capture() instead. It is disabled when the survey
          is not walking, because then it is a picture of stored points. */}
      <button
        type="button"
        disabled={!walking}
        aria-label={t('pages:surveys.surfaceLabel', { name: surveyName })}
        aria-busy={captureMutation.isPending}
        onClick={handleClick}
        onKeyDown={handleKeyDown}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        className={`block w-full rounded-[12px] border border-hairline bg-surface-sunken p-0 focus:outline-2 focus:outline-brand-primary ${
          captureMutation.isPending ? 'cursor-progress' : walking ? 'cursor-crosshair' : ''
        }`}
        data-testid="capture-surface"
        data-walking={walking}
      >
        <svg
          viewBox={`0 0 ${SURFACE_WIDTH} ${SURFACE_HEIGHT}`}
          aria-hidden="true"
          className="block w-full"
        >
          {focused && walking ? (
            <g className="stroke-brand-primary" strokeWidth={1.5} data-testid="capture-cursor">
              <line x1={cursor.x - 14} y1={cursor.y} x2={cursor.x + 14} y2={cursor.y} />
              <line x1={cursor.x} y1={cursor.y - 14} x2={cursor.x} y2={cursor.y + 14} />
            </g>
          ) : null}
          {pins.map((pin) => (
            <g
              key={pinKey(pin)}
              data-testid="capture-pin"
              data-interpolated={pin.interpolated ? 'true' : undefined}
            >
              {/* A placed reading is drawn hollow and smaller: its position was
                  worked out from the marks on either side of it, not recorded,
                  and a survey that drew the two alike would show a claim about
                  a position as a record of one. Shape, not only colour — the
                  colour is already carrying the signal. */}
              <circle
                cx={pin.x}
                cy={pin.y}
                r={pin.interpolated ? 5 : 9}
                className={
                  pin.interpolated
                    ? `fill-none ${strokeClass(pin.strongestDbm)}`
                    : `${pinClass(pin.strongestDbm)} stroke-surface-raised`
                }
                strokeWidth={2}
              />
              {/* Only the marks are labelled. A walk stores a reading every few
                  seconds, and a value beside each one is an unreadable page. */}
              {pin.interpolated ? null : (
                <text x={pin.x + 14} y={pin.y + 4} className="figure fill-text-primary text-[13px]">
                  {signalText(pin.strongestDbm)}
                </text>
              )}
            </g>
          ))}
        </svg>
      </button>

      {status ? (
        <p
          className={`text-sm ${failed ? 'text-status-error' : 'text-text-secondary'}`}
          data-testid="capture-status"
        >
          {status}
        </p>
      ) : null}
    </section>
  );
}

/**
 * Capture time plus position identifies a point. Two captures at one spot are
 * two points, and they cannot share a millisecond: a scan takes seconds.
 */
function pinKey(pin: SurveySample): string {
  const at = pin.capturedAt ? timestampMs(pin.capturedAt) : 0;
  return `${at}:${pin.x}:${pin.y}`;
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

/**
 * The outline colour for a placed reading, matching the fill a pinned one gets
 * so the same signal reads the same either way.
 */
function strokeClass(strongestDbm: number | undefined): string {
  if (strongestDbm === undefined) {
    return 'stroke-text-muted';
  }
  if (strongestDbm >= GOOD_DBM) {
    return 'stroke-status-success';
  }
  if (strongestDbm >= WEAK_DBM) {
    return 'stroke-status-warning';
  }
  return 'stroke-status-error';
}
