import type { TFunction } from 'i18next';
import { surveyClient } from '@/lib/client';
import { COVERAGE_METRIC_COPY } from '@/lib/coverageThreshold';
import type { Band } from '@/lib/placement';

/* The layers Coverage draws, and how each one is asked for.

   Mbps is a glossary term like RSSI and SNR. The throughput layer has no
   glossary name: "Download" alone read as a file action rather than as the
   layer being plotted (trellis#476), so it carries ordinary English copy from
   the catalogue instead. */
export const METRICS = [
  { id: 'rssi', ...COVERAGE_METRIC_COPY.rssi },
  { id: 'snr', ...COVERAGE_METRIC_COPY.snr },
  { id: 'download', glossary: undefined, unit: 'Mbps' },
  { id: 'predicted_rssi', glossary: undefined, unit: 'dBm' },
] as const;

export type Metric = (typeof METRICS)[number]['id'];

/** A layer's name: its glossary term, or plain English where it has none. */
export function metricName(metric: Metric, t: TFunction<['common', 'pages']>): string {
  const glossary = METRICS.find((m) => m.id === metric)?.glossary;
  if (glossary !== undefined) {
    return glossary;
  }
  return metric === 'predicted_rssi'
    ? t('pages:coverage.metricPredicted')
    : t('pages:coverage.metricDownload');
}

/** The request field for one AP; absent, not empty, means every AP. */
export function apField(bssid: string) {
  return bssid === '' ? {} : { bssid };
}

/**
 * One heatmap request, shared by the map on screen and the heard-AP list. The
 * band is only part of a predicted request: the measured layers ignore it, and
 * keying them on it would re-fetch the same map when the band changes.
 */
export function heatmapOptions(
  surveyId: string | undefined,
  metric: Metric,
  floorId: string,
  bssid: string,
  band: Band,
) {
  const forBand = metric === 'predicted_rssi' ? { band } : {};
  return {
    queryKey: ['heatmap', surveyId, metric, floorId, bssid, forBand],
    queryFn: () =>
      surveyClient.getHeatmap({
        surveyId: surveyId ?? '',
        metric,
        floorId,
        ...forBand,
        ...apField(bssid),
      }),
    enabled: surveyId !== undefined,
  };
}
