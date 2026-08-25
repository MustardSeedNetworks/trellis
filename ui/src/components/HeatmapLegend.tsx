import { useTranslation } from 'react-i18next';
import type { LegendStop } from '@/gen/trellis/survey/v1/survey_pb';

/**
 * HeatmapLegend — the key to the surface beside it.
 *
 * The stops are the ones the server painted the PNG with: `GetHeatmapResponse`
 * carries the colour scale that rendered the image, so the gradient here and
 * the gradient on screen are the same object rather than two copies that drift
 * the first time a threshold moves. Nothing about the scale is defined in this
 * file — it has no palette, and a scale with different colours or a different
 * number of stops renders correctly with no change here.
 *
 * Every stop is labelled with its value, so the scale is never colour-alone.
 */
interface HeatmapLegendProps {
  stops: LegendStop[];
  /** Unit of the stop values — "dBm" for rssi, "dB" for snr. */
  unit: string;
}

export function HeatmapLegend({ stops, unit }: HeatmapLegendProps) {
  const { t } = useTranslation(['common', 'pages']);
  const lowest = stops.at(0);
  const highest = stops.at(-1);

  /* Two stops is the minimum that makes a gradient. The existence checks are
     what narrow the ends' type — under noUncheckedIndexedAccess an index is
     not a promise that something is there — and they are the same condition
     the length check states, so neither is decoration. */
  if (stops.length < 2 || lowest === undefined || highest === undefined) {
    /* One stop is not a gradient and zero is not a scale. Either means the
       reply did not describe what it drew, which is worth saying rather than
       rendering a bar that implies a range. */
    return (
      <p className="text-xs text-text-muted" data-testid="legend-unavailable">
        {t('pages:coverage.legendUnavailable')}
      </p>
    );
  }

  const first = lowest.value;
  const last = highest.value;
  const span = last - first;

  const position = (value: number) => (span === 0 ? 0 : ((value - first) / span) * 100);
  const gradient = `linear-gradient(90deg, ${stops
    .map((stop) => `${stop.color} ${position(stop.value).toFixed(1)}%`)
    .join(', ')})`;

  return (
    <div className="flex flex-col gap-2" data-testid="heatmap-legend">
      <span className="kicker">{t('common:labels.scale')}</span>
      <div
        aria-hidden="true"
        className="h-2 w-full rounded-full border border-hairline"
        style={{ background: gradient }}
        data-testid="legend-gradient"
      />
      <ul className="flex flex-wrap gap-x-5 gap-y-1">
        {stops.map((stop) => (
          <li key={stop.value} className="flex items-center gap-2">
            <span
              aria-hidden="true"
              className="h-2.5 w-2.5 shrink-0 rounded-[3px] border border-hairline"
              style={{ background: stop.color }}
            />
            <span className="figure text-xs text-text-secondary">
              {stop.value} {unit}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
