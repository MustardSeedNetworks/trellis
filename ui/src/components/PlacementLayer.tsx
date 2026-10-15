import type { TFunction } from 'i18next';
import { type KeyboardEvent, type MouseEvent, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { clampToPlan, type PlanPoint, pointerToPlan } from '@/lib/placement';

/** Arrow keys move a marker by this many plan pixels; with Shift, five times it. */
const KEY_STEP = 5;
const KEY_STEP_FAST = 25;

export interface PlacedAP {
  id: string;
  x: number;
  y: number;
  label: string;
  band: string;
}

interface PlacementLayerProps {
  placements: readonly PlacedAP[];
  /** The plan's own pixel size, which placements are positioned in. */
  width: number;
  height: number;
  selectedId: string | undefined;
  onSelect: (id: string) => void;
  onAdd: (point: PlanPoint) => void;
  onMove: (id: string, point: PlanPoint) => void;
}

/**
 * The APs placed on a floor, drawn over its plan or its predicted map.
 *
 * Clicking the plan places an AP there; a marker is dragged to move it. Both
 * are in the plan's pixels whatever size the plan is drawn at, which is what
 * the service stores. From the keyboard, Enter on the plan places an AP at its
 * centre and the arrow keys move the focused marker: a plan only a pointer
 * could edit would leave the prediction out of reach of anyone without one.
 */
export function PlacementLayer({
  placements,
  width,
  height,
  selectedId,
  onSelect,
  onAdd,
  onMove,
}: PlacementLayerProps) {
  const { t } = useTranslation(['common', 'pages']);
  const layerRef = useRef<HTMLDivElement>(null);
  /* Where a marker is while it is being dragged. It is only sent on release:
     one request per drag, not one per pointer event. */
  const [drag, setDrag] = useState<{ id: string; point: PlanPoint; moved: boolean }>();

  function planPointAt(clientX: number, clientY: number) {
    const box = layerRef.current?.getBoundingClientRect();
    return box ? pointerToPlan({ x: clientX, y: clientY }, box, width, height) : undefined;
  }

  function handlePlanClick(event: MouseEvent<HTMLButtonElement>) {
    const point =
      event.detail === 0
        ? { x: Math.floor(width / 2), y: Math.floor(height / 2) }
        : planPointAt(event.clientX, event.clientY);
    if (point) {
      onAdd(point);
    }
  }

  /* The drag is followed on the window, not the marker: the pointer leaves a
     32 px marker at once, and a marker that only heard its own events stood
     still under a WebKit drag. */
  function handlePointerDown(ap: PlacedAP) {
    let current = { point: { x: ap.x, y: ap.y }, moved: false };
    setDrag({ id: ap.id, ...current });

    function move(event: globalThis.PointerEvent) {
      const point = planPointAt(event.clientX, event.clientY);
      if (point) {
        current = { point, moved: true };
        setDrag({ id: ap.id, ...current });
      }
    }
    function finish(commit: boolean) {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', release);
      window.removeEventListener('pointercancel', cancel);
      setDrag(undefined);
      if (commit && current.moved) {
        onMove(ap.id, current.point);
      }
      onSelect(ap.id);
    }
    const release = () => finish(true);
    const cancel = () => finish(false);
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', release);
    window.addEventListener('pointercancel', cancel);
  }

  function handleMarkerKey(event: KeyboardEvent<HTMLButtonElement>, ap: PlacedAP) {
    const step = event.shiftKey ? KEY_STEP_FAST : KEY_STEP;
    const moves: Record<string, PlanPoint> = {
      ArrowUp: { x: 0, y: -step },
      ArrowDown: { x: 0, y: step },
      ArrowLeft: { x: -step, y: 0 },
      ArrowRight: { x: step, y: 0 },
    };
    const move = moves[event.key];
    if (move) {
      event.preventDefault();
      onMove(ap.id, clampToPlan({ x: ap.x + move.x, y: ap.y + move.y }, width, height));
    }
  }

  return (
    <div ref={layerRef} className="absolute inset-0 overflow-hidden" data-testid="placement-layer">
      <button
        type="button"
        onClick={handlePlanClick}
        aria-label={t('pages:coverage.placeOnPlan')}
        className="absolute inset-0 cursor-crosshair"
        data-testid="placement-surface"
      />
      {placements.map((ap, index) => {
        const at = drag?.id === ap.id ? drag.point : ap;
        const selected = ap.id === selectedId;
        return (
          <button
            key={ap.id}
            type="button"
            onPointerDown={() => handlePointerDown(ap)}
            onKeyDown={(event) => handleMarkerKey(event, ap)}
            onFocus={() => onSelect(ap.id)}
            aria-pressed={selected}
            aria-label={t('pages:coverage.apMarker', { label: apLabel(ap, index, t) })}
            style={{ left: `${(at.x / width) * 100}%`, top: `${(at.y / height) * 100}%` }}
            className={`figure absolute flex h-8 w-8 -translate-x-1/2 -translate-y-1/2 cursor-grab touch-none items-center justify-center rounded-full border-2 text-xs font-bold ${
              selected
                ? 'border-text-primary bg-brand-primary text-on-brand'
                : 'border-surface-raised bg-brand-primary text-on-brand'
            }`}
            data-testid="ap-marker"
            data-x={at.x}
            data-y={at.y}
          >
            {index + 1}
          </button>
        );
      })}
    </div>
  );
}

/** The name an AP goes by: its own label, or its number on this floor. */
export function apLabel(
  ap: { label: string },
  index: number,
  t: TFunction<['common', 'pages']>,
): string {
  return ap.label !== '' ? ap.label : t('pages:coverage.apNumbered', { number: index + 1 });
}
