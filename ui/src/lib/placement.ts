/**
 * Placing an AP on a floor plan, in the plan's own pixels.
 *
 * The service refuses a placement that is not a pixel of the plan
 * (`x < width`, `y < height`), so a marker dragged past the edge stops on the
 * last pixel instead of becoming a request the service will reject.
 */

/** The bands a prediction is drawn for, spelled as a placement states them. */
export const BANDS = ['2.4 GHz', '5 GHz', '6 GHz'] as const;

export type Band = (typeof BANDS)[number];

export function isBand(value: string): value is Band {
  return (BANDS as readonly string[]).includes(value);
}

/** The transmit power range the service accepts, in dBm. */
export const TX_POWER = { min: -10, max: 36, assumed: 10 } as const;

export interface PlanPoint {
  x: number;
  y: number;
}

/** The nearest pixel of a width-by-height plan to `point`. */
export function clampToPlan(point: PlanPoint, width: number, height: number): PlanPoint {
  return {
    x: Math.min(Math.max(Math.round(point.x), 0), width - 1),
    y: Math.min(Math.max(Math.round(point.y), 0), height - 1),
  };
}

/**
 * The plan pixel under a pointer. The plan is drawn at whatever size the panel
 * and the zoom give it, so the client position is scaled back through the
 * rendered box before it means a pixel.
 */
export function pointerToPlan(
  client: PlanPoint,
  box: { left: number; top: number; width: number; height: number },
  width: number,
  height: number,
): PlanPoint | undefined {
  if (box.width === 0 || box.height === 0) {
    return undefined;
  }
  return clampToPlan(
    {
      x: ((client.x - box.left) / box.width) * width,
      y: ((client.y - box.top) / box.height) * height,
    },
    width,
    height,
  );
}

/**
 * Whether a prediction on `band` has anything to draw: a placement that states
 * no band radiates on every band, as the service draws it.
 */
export function placedOnBand(placements: readonly { band: string }[], band: Band): boolean {
  return placements.some((placement) => placement.band === '' || placement.band === band);
}
