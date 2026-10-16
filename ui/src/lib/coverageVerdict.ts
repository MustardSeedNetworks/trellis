import { Code, ConnectError } from '@connectrpc/connect';
import type { TFunction } from 'i18next';
import type { Metric } from '@/lib/coverageLayers';
import { formatCoverageScore } from '@/lib/format';
import type { RollupState } from '@/ui/StatusRollup';

/**
 * FailedPrecondition on a layer is the service saying the walk never measured
 * that metric — SNR from a radio that reports no noise floor (#600). It is a
 * fact about the survey, told apart by its code rather than its message
 * (#607), and never a failure to render or analyse.
 */
export function isUnmeasured(error: unknown): boolean {
  return error != null && ConnectError.from(error).code === Code.FailedPrecondition;
}

/** What an unmeasured layer says, on the surface and in the findings alike. */
export function unmeasuredCopy(
  metric: Metric,
  metricLabel: string,
  t: TFunction<['common', 'pages']>,
) {
  return {
    headline: t('pages:coverage.layerUnmeasured', { metric: metricLabel }),
    body: metric === 'snr' ? t('pages:coverage.snrUnmeasuredBody') : undefined,
  };
}

interface CoverageState {
  metric: Metric;
  metricLabel: string;
  threshold: number;
  unit: string;
  hasSurvey: boolean;
  loading: boolean;
  error: unknown;
  coverage: { coverageScore: number; deadZoneCount: number; recommendations: string[] } | undefined;
  sampleCount: number | undefined;
}

interface CoverageVerdict {
  state: RollupState;
  headline: string;
  body?: string;
  figures: { label: string; value: string }[];
  recommendations: string[];
}

/** Turns the analysis into the sentence the findings panel leads with. */
export function describeCoverage(
  {
    metric,
    metricLabel,
    threshold,
    unit,
    hasSurvey,
    loading,
    error,
    coverage,
    sampleCount,
  }: CoverageState,
  t: TFunction<['common', 'pages']>,
): CoverageVerdict {
  if (!hasSurvey) {
    return {
      state: 'unknown',
      headline: t('pages:coverage.noSurveySelected'),
      body: t('pages:coverage.noSurveySelectedBody'),
      figures: [],
      recommendations: [],
    };
  }
  if (isUnmeasured(error)) {
    return {
      state: 'unknown',
      ...unmeasuredCopy(metric, metricLabel, t),
      figures: [],
      recommendations: [],
    };
  }
  if (error) {
    return {
      state: 'unknown',
      headline: t('pages:coverage.notArriving'),
      body: t('pages:coverage.surveyServiceSilent', { error: String(error) }),
      figures: [],
      recommendations: [],
    };
  }
  if (loading || !coverage) {
    return {
      state: 'unknown',
      headline: t('pages:coverage.analysing'),
      figures: [],
      recommendations: [],
    };
  }

  const figures = [
    { label: t('common:labels.coverageScore'), value: formatCoverageScore(coverage.coverageScore) },
    { label: t('common:labels.deadZones'), value: String(coverage.deadZoneCount) },
    {
      label: t('common:labels.samples'),
      value: sampleCount === undefined ? '—' : String(sampleCount),
    },
  ];

  if (coverage.deadZoneCount === 0) {
    return {
      state: 'ok',
      headline: t('pages:coverage.noDeadZones', { threshold, unit }),
      figures,
      recommendations: coverage.recommendations,
    };
  }
  return {
    state: 'warn',
    headline: t('pages:coverage.deadZones', { count: coverage.deadZoneCount, threshold, unit }),
    body:
      metric === 'snr' ? t('pages:coverage.deadZoneBodySnr') : t('pages:coverage.deadZoneBodyRssi'),
    figures,
    recommendations: coverage.recommendations,
  };
}
