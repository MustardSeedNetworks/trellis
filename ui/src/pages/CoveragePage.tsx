import { useQuery } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router';
import { CoverageFindings } from '@/components/CoverageFindings';
import { HeatmapLegend } from '@/components/HeatmapLegend';
import { surveyClient } from '@/lib/client';
import { bytesToDataUrl, formatCoverageScore, formatSignal } from '@/lib/format';
import type { RollupState } from '@/ui/StatusRollup';

/**
 * Coverage — Canvas.
 *
 * Shape: toolbar → data surface + legend → findings panel. The capability is
 * not new; the heatmap and the dead-zone analysis were already rendered inside
 * a survey's detail pane, where the image had no scale beside it and the
 * threshold that decides what counts as a dead zone was not adjustable. This
 * page moves both here rather than copying them: the same reading in two
 * places is the drift the family rules exist to prevent.
 *
 * Only rssi and snr appear as metrics because those are the only two the
 * service renders. `core/survey` also defines density and interference scales,
 * but no RPC reaches them, so offering them here would promise a picture the
 * product cannot draw.
 */
/* Not translated, and not an oversight: RSSI, SNR, dBm and dB are glossary
   terms the gate requires verbatim in every locale. */
const METRICS = [
  { id: 'rssi', label: 'RSSI', unit: 'dBm' },
  { id: 'snr', label: 'SNR', unit: 'dB' },
] as const;

type Metric = (typeof METRICS)[number]['id'];

/** Matches the service default in internal/api. */
const DEFAULT_THRESHOLD_DBM = -75;
/* The service reads threshold_dbm == 0 as "unset" and substitutes its default,
   so an operator who types 0 would silently be shown the -75 dBm answer. The
   control cannot reach 0: these bounds are the range a dead-zone threshold is
   meaningful over anyway. */
const MIN_THRESHOLD_DBM = -90;
const MAX_THRESHOLD_DBM = -40;

export function CoveragePage() {
  const { t } = useTranslation(['common', 'pages']);
  /* The survey lives in the URL so "Plot coverage" on a survey opens that
     survey's coverage, and so a floor worth showing someone is a link. */
  const [searchParams, setSearchParams] = useSearchParams();
  const [metric, setMetric] = useState<Metric>('rssi');
  const [threshold, setThreshold] = useState(DEFAULT_THRESHOLD_DBM);

  const surveysQuery = useQuery({
    queryKey: ['surveys'],
    queryFn: () => surveyClient.listSurveys({}),
  });

  const surveys = surveysQuery.data?.surveys ?? [];
  const requestedId = searchParams.get('survey') ?? undefined;
  /* The first survey is a starting point, not a selection anyone made; it is
     only used while the URL names none. A named survey is honoured even when
     it is not in the list — the request then fails as not-found and says so,
     rather than quietly analysing a different floor. */
  const surveyId = requestedId ?? surveys[0]?.id;
  const listed = surveys.some((survey) => survey.id === surveyId);

  const heatmapQuery = useQuery({
    queryKey: ['heatmap', surveyId, metric],
    queryFn: () => surveyClient.getHeatmap({ surveyId: surveyId ?? '', metric }),
    enabled: surveyId !== undefined,
  });

  const coverageQuery = useQuery({
    queryKey: ['coverage', surveyId, threshold],
    queryFn: () => surveyClient.getCoverage({ surveyId: surveyId ?? '', thresholdDbm: threshold }),
    enabled: surveyId !== undefined,
  });

  const heatmap = heatmapQuery.data;
  const unit = METRICS.find((m) => m.id === metric)?.unit ?? '';
  const findings = describeCoverage(
    {
      threshold,
      hasSurvey: surveyId !== undefined,
      loading: coverageQuery.isLoading,
      error: coverageQuery.error,
      coverage: coverageQuery.data,
      sampleCount: heatmap?.sampleCount,
    },
    t,
  );

  return (
    <div className="flex flex-1 flex-col gap-6 overflow-y-auto p-6">
      <div className="panel flex flex-wrap items-center gap-4 p-4">
        <label className="flex items-center gap-2 text-sm" htmlFor="coverage-survey">
          <span className="kicker">{t('common:labels.survey')}</span>
          <select
            id="coverage-survey"
            value={surveyId ?? ''}
            disabled={surveys.length === 0}
            onChange={(event) => setSearchParams({ survey: event.target.value })}
            className="rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary disabled:opacity-50"
          >
            {surveyId !== undefined && !listed ? (
              <option value={surveyId}>{t('pages:coverage.notInList', { id: surveyId })}</option>
            ) : null}
            {surveys.map((survey) => (
              <option key={survey.id} value={survey.id}>
                {survey.name}
              </option>
            ))}
          </select>
        </label>

        <div className="flex items-center gap-2">
          <span className="kicker">{t('common:labels.metric')}</span>
          <div className="flex gap-1">
            {METRICS.map((option) => (
              <button
                key={option.id}
                type="button"
                onClick={() => setMetric(option.id)}
                aria-pressed={metric === option.id}
                className={`rounded-[9px] px-3 py-2 text-sm font-bold ${
                  metric === option.id
                    ? 'bg-brand-primary text-on-brand'
                    : 'text-text-secondary hover:bg-surface-hover'
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>

        <label className="flex items-center gap-2 text-sm" htmlFor="coverage-threshold">
          <span className="kicker">{t('pages:coverage.deadZoneThreshold')}</span>
          <input
            id="coverage-threshold"
            type="number"
            value={threshold}
            min={MIN_THRESHOLD_DBM}
            max={MAX_THRESHOLD_DBM}
            step={1}
            onChange={(event) => {
              const parsed = Number(event.target.value);
              if (
                Number.isInteger(parsed) &&
                parsed >= MIN_THRESHOLD_DBM &&
                parsed <= MAX_THRESHOLD_DBM
              ) {
                setThreshold(parsed);
              }
            }}
            className="figure w-24 rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary"
          />
          <span className="text-text-secondary">dBm</span>
        </label>

        {heatmap ? (
          <span className="figure ml-auto text-xs text-text-muted" data-testid="surface-meta">
            {t('pages:coverage.surfaceMeta', {
              metric: heatmap.metric,
              count: heatmap.sampleCount,
              min: formatSignal(heatmap.min, unit),
              max: formatSignal(heatmap.max, unit),
            })}
          </span>
        ) : null}
      </div>

      <div className="grid flex-1 grid-cols-1 items-start gap-6 xl:grid-cols-[1fr_320px]">
        <section className="panel flex flex-col gap-4 p-5">
          {renderSurface(
            {
              hasSurveys: surveys.length > 0,
              surveysLoading: surveysQuery.isLoading,
              surveysError: surveysQuery.error,
              heatmapLoading: heatmapQuery.isLoading,
              heatmapError: heatmapQuery.error,
              heatmap,
              metric,
            },
            t,
          )}
          {heatmap ? <HeatmapLegend stops={heatmap.legend} unit={unit} /> : null}
        </section>

        <CoverageFindings
          state={findings.state}
          headline={findings.headline}
          body={findings.body}
          figures={findings.figures}
          recommendations={findings.recommendations}
        />
      </div>
    </div>
  );
}

interface SurfaceState {
  hasSurveys: boolean;
  surveysLoading: boolean;
  surveysError: unknown;
  heatmapLoading: boolean;
  heatmapError: unknown;
  heatmap: { png: Uint8Array; width: number; height: number } | undefined;
  metric: Metric;
}

/**
 * The surface says which of its several failures happened. A blank card would
 * read as "this floor has no coverage" in every one of them.
 */
function renderSurface(
  {
    hasSurveys,
    surveysLoading,
    surveysError,
    heatmapLoading,
    heatmapError,
    heatmap,
    metric,
  }: SurfaceState,
  t: TFunction<['common', 'pages']>,
) {
  if (surveysError) {
    return (
      <p className="text-sm text-status-error" data-testid="surface-message">
        {t('pages:coverage.surveyServiceSilent', { error: String(surveysError) })}
      </p>
    );
  }
  if (surveysLoading) {
    return (
      <p className="text-sm text-text-muted" data-testid="surface-message">
        {t('pages:coverage.loadingSurveys')}
      </p>
    );
  }
  if (!hasSurveys) {
    return (
      <p className="text-sm text-text-muted" data-testid="surface-message">
        {t('pages:coverage.noSurveys')}
      </p>
    );
  }
  if (heatmapError) {
    /* A survey with no floor plan is the common case here, and the service
       says so in its message — worth quoting rather than replacing. */
    return (
      <p className="text-sm text-status-error" data-testid="surface-message">
        {t('pages:coverage.heatmapFailed', { error: String(heatmapError) })}
      </p>
    );
  }
  if (heatmapLoading || !heatmap) {
    return (
      <p className="text-sm text-text-muted" data-testid="surface-message">
        {t('pages:coverage.renderingHeatmap')}
      </p>
    );
  }
  return (
    <img
      src={bytesToDataUrl(heatmap.png, 'image/png')}
      alt={t('pages:coverage.heatmapAlt', { metric: metric.toUpperCase() })}
      width={heatmap.width}
      height={heatmap.height}
      className="max-w-full rounded-[12px] border border-hairline"
      data-testid="heatmap-image"
    />
  );
}

interface CoverageState {
  threshold: number;
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
function describeCoverage(
  { threshold, hasSurvey, loading, error, coverage, sampleCount }: CoverageState,
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
      headline: t('pages:coverage.noDeadZones', { threshold }),
      figures,
      recommendations: coverage.recommendations,
    };
  }
  return {
    state: 'warn',
    headline: t('pages:coverage.deadZones', { count: coverage.deadZoneCount, threshold }),
    body: t('pages:coverage.deadZoneBody'),
    figures,
    recommendations: coverage.recommendations,
  };
}
