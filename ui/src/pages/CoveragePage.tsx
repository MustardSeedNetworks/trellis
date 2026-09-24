import { Code, ConnectError } from '@connectrpc/connect';
import { useQuery } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router';
import { CoverageFindings } from '@/components/CoverageFindings';
import { HeatmapLegend } from '@/components/HeatmapLegend';
import { HeatmapSurface } from '@/components/HeatmapSurface';
import { surveyClient } from '@/lib/client';
import { formatCoverageScore, formatSignal } from '@/lib/format';
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
/* RSSI, SNR, dBm, dB and Mbps are glossary terms the gate requires verbatim in
   every locale, so they are not translated and that is not an oversight. The
   throughput layer has no glossary name: "Download" alone read as a file
   action rather than as the layer being plotted (trellis#476), so it carries
   ordinary English copy from the catalogue instead. */
const METRICS = [
  { id: 'rssi', glossary: 'RSSI', unit: 'dBm' },
  { id: 'snr', glossary: 'SNR', unit: 'dB' },
  { id: 'download', glossary: undefined, unit: 'Mbps' },
] as const;

/**
 * Metrics the dead-zone analysis can speak about. Download throughput is not
 * one of them: `GetCoverage` refuses a metric it has no rule for rather than
 * answering about signal strength under another heading, so offering it here
 * would only produce an error where a finding belongs.
 */
const COVERAGE_METRICS: readonly Metric[] = ['rssi', 'snr'];

/**
 * Each metric's own dead-zone threshold, in its own unit. There is no shared
 * default and no shared range: -75 dB of signal-to-noise is not a number, and
 * a threshold typed under one layer must not follow the operator to the other.
 * The service applies the same defaults when a request omits one.
 */
const THRESHOLDS: Record<Metric & ('rssi' | 'snr'), { default: number; min: number; max: number }> =
  {
    /* The service reads a threshold it was sent verbatim; these bounds are the
       range over which each analysis means anything. */
    rssi: { default: -75, min: -90, max: -40 },
    snr: { default: 20, min: 5, max: 40 },
  };

type Metric = (typeof METRICS)[number]['id'];

export function CoveragePage() {
  const { t } = useTranslation(['common', 'pages']);
  /* The survey lives in the URL so "Plot coverage" on a survey opens that
     survey's coverage, and so a floor worth showing someone is a link. */
  const [searchParams, setSearchParams] = useSearchParams();
  const [metric, setMetric] = useState<Metric>('rssi');
  /* One threshold per metric, not one control that changes unit under the
     operator: carrying -75 into a dB box would ask for a margin no radio has,
     and the number an operator settled on for one layer is still the one they
     want when they come back to it. */
  const [thresholds, setThresholds] = useState<Record<string, number>>({
    rssi: THRESHOLDS.rssi.default,
    snr: THRESHOLDS.snr.default,
  });

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

  /* Floors are only fetched for a survey that has more than one. A
     single-floor survey is the common case and its one floor is the active
     one, which is what the service reads when a request names none. */
  const floorCount = surveys.find((survey) => survey.id === surveyId)?.floorCount ?? 0;
  const floorsQuery = useQuery({
    queryKey: ['floors', surveyId],
    queryFn: () => surveyClient.listFloors({ surveyId: surveyId ?? '' }),
    enabled: surveyId !== undefined && floorCount > 1,
  });

  const floors = floorsQuery.data?.floors ?? [];
  /* The floor is in the URL for the same reason the survey is: a floor worth
     showing someone is a link. Empty means the active floor, which is how a
     link to a survey without a floor keeps working. */
  const requestedFloorId = searchParams.get('floor') ?? '';
  /* A floor named in the URL that this survey does not have would be answered
     with NotFound. Falling back to the active floor is right here — the stale
     part of the link is the floor, and the survey it names is still the one
     being analysed. */
  const floorId =
    requestedFloorId !== '' && floors.some((floor) => floor.id === requestedFloorId)
      ? requestedFloorId
      : '';

  const heatmapQuery = useQuery({
    queryKey: ['heatmap', surveyId, metric, floorId],
    queryFn: () => surveyClient.getHeatmap({ surveyId: surveyId ?? '', metric, floorId }),
    enabled: surveyId !== undefined,
  });

  const analysable = COVERAGE_METRICS.includes(metric);
  const bounds = THRESHOLDS[metric as 'rssi' | 'snr'] ?? THRESHOLDS.rssi;
  const threshold = thresholds[metric] ?? bounds.default;

  const coverageQuery = useQuery({
    queryKey: ['coverage', surveyId, metric, threshold, floorId],
    queryFn: () =>
      surveyClient.getCoverage({ surveyId: surveyId ?? '', metric, threshold, floorId }),
    enabled: surveyId !== undefined && analysable,
  });

  const heatmap = heatmapQuery.data;
  const metricOption = METRICS.find((m) => m.id === metric);
  const unit = metricOption?.unit ?? '';
  const metricLabel = metricOption?.glossary ?? t('pages:coverage.metricDownload');
  const findings = describeCoverage(
    {
      metric,
      metricLabel,
      threshold,
      unit,
      hasSurvey: surveyId !== undefined,
      loading: coverageQuery.isLoading,
      error: coverageQuery.error,
      coverage: coverageQuery.data,
      sampleCount: heatmap?.sampleCount,
    },
    t,
  );

  return (
    /* Bounded where the findings sit BESIDE the map, scrolling everywhere else.
       Bounded is what keeps the colour key on screen with the map it explains:
       the surface grew to whatever the floor plan needed and pushed the legend
       past the fold (trellis#476), and inside a bound the surface is the one
       thing that gives way. The breakpoint is xl rather than md because below
       it the findings panel stacks UNDER the map — bounding there only moves
       the clipping from the legend onto the findings, which was measured at
       1024x768: their panel ended 118 px past the fold with nothing to
       scroll. */
    <div className="flex flex-1 flex-col gap-6 overflow-y-auto p-6 xl:min-h-0 xl:overflow-hidden">
      {/* A row of controls on a laptop; a stack on a phone. Wrapping alone was
          not enough: a <select> keeps a minimum width from its longest option,
          so a survey with a long name pushed its own label past the panel. Each
          control is a full-width block below md, where that width is definite. */}
      <div className="panel flex flex-col items-stretch gap-4 p-4 md:flex-row md:flex-wrap md:items-center">
        <label
          className="flex min-w-0 flex-wrap items-center gap-2 text-sm"
          htmlFor="coverage-survey"
        >
          <span className="kicker">{t('common:labels.survey')}</span>
          <select
            id="coverage-survey"
            value={surveyId ?? ''}
            disabled={surveys.length === 0}
            onChange={(event) => setSearchParams({ survey: event.target.value })}
            className="w-full min-w-0 rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary disabled:opacity-50 md:w-auto md:max-w-full"
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

        {floors.length > 1 ? (
          <label
            className="flex min-w-0 flex-wrap items-center gap-2 text-sm"
            htmlFor="coverage-floor"
          >
            <span className="kicker">{t('common:labels.floor')}</span>
            <select
              id="coverage-floor"
              value={floorId}
              onChange={(event) =>
                setSearchParams(
                  event.target.value === ''
                    ? { survey: surveyId ?? '' }
                    : { survey: surveyId ?? '', floor: event.target.value },
                )
              }
              className="w-full min-w-0 rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary md:w-auto md:max-w-full"
              data-testid="coverage-floor"
            >
              {/* The active floor by name rather than a blank row: "which floor
                  is this" must be answerable without opening the list. */}
              {floors.map((floor) => (
                <option key={floor.id} value={floor.isActive ? '' : floor.id}>
                  {floor.name}
                </option>
              ))}
            </select>
          </label>
        ) : null}

        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <span className="kicker">{t('common:labels.metric')}</span>
          <div className="flex flex-wrap gap-1">
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
                {option.glossary ?? t('pages:coverage.metricDownload')}
              </button>
            ))}
          </div>
        </div>

        {/* Only where it means something. The threshold and the findings below
            speak about a layer the analysis has a rule for; over a throughput
            layer they would answer a question nobody asked, and a control that
            appears to do nothing is worse than one that is not there. */}
        {analysable ? (
          <label
            className="flex min-w-0 flex-wrap items-center gap-2 text-sm"
            htmlFor="coverage-threshold"
          >
            <span className="kicker">{t('pages:coverage.deadZoneThreshold')}</span>
            <input
              id="coverage-threshold"
              type="number"
              value={threshold}
              min={bounds.min}
              max={bounds.max}
              step={1}
              onChange={(event) => {
                const parsed = Number(event.target.value);
                if (Number.isInteger(parsed) && parsed >= bounds.min && parsed <= bounds.max) {
                  setThresholds((current) => ({ ...current, [metric]: parsed }));
                }
              }}
              className="figure w-24 rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary"
              data-testid="coverage-threshold"
            />
            <span className="text-text-secondary" data-testid="coverage-threshold-unit">
              {unit}
            </span>
          </label>
        ) : null}

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

      <div className="grid flex-1 grid-cols-1 items-start gap-6 xl:min-h-0 xl:grid-cols-[1fr_320px]">
        <section className="panel flex flex-col gap-4 p-5 xl:min-h-0 xl:self-stretch">
          {renderSurface(
            {
              hasSurveys: surveys.length > 0,
              surveysLoading: surveysQuery.isLoading,
              surveysError: surveysQuery.error,
              heatmapLoading: heatmapQuery.isLoading,
              heatmapError: heatmapQuery.error,
              heatmap,
              metric,
              metricLabel,
              unit,
            },
            t,
          )}
          {/* Pinned to the foot of the surface panel. Bounding the layout keeps
              the key beside the map at xl (trellis#476); below that breakpoint
              the page scrolls and a tall floor plan still carried the key past
              the fold, so the one thing that says what the colours mean was
              off screen exactly while an operator was reading them (#484).
              Sticky rather than fixed: it belongs to this panel, and on a short
              plan it simply sits where it always did. */}
          {heatmap ? (
            <div className="sticky bottom-0 -mx-5 -mb-5 mt-auto bg-surface-raised px-5 pb-5 pt-3">
              <HeatmapLegend stops={heatmap.legend} unit={unit} />
            </div>
          ) : null}
        </section>

        {analysable ? (
          <CoverageFindings
            title={
              metric === 'snr'
                ? t('pages:coverage.findingsTitleSnr')
                : t('pages:coverage.findingsTitleRssi')
            }
            state={findings.state}
            headline={findings.headline}
            body={findings.body}
            figures={findings.figures}
            recommendations={findings.recommendations}
          />
        ) : null}
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
  heatmap:
    | {
        png: Uint8Array;
        width: number;
        height: number;
        grid: number[];
        gridCols: number;
        gridRows: number;
        cellSize: number;
      }
    | undefined;
  metric: Metric;
  metricLabel: string;
  unit: string;
}

/**
 * FailedPrecondition on a layer is the service saying the walk never measured
 * that metric — SNR from a radio that reports no noise floor (#600). It is a
 * fact about the survey, told apart by its code rather than its message
 * (#607), and never a failure to render or analyse.
 */
function isUnmeasured(error: unknown): boolean {
  return error != null && ConnectError.from(error).code === Code.FailedPrecondition;
}

/** What an unmeasured layer says, on the surface and in the findings alike. */
function unmeasuredCopy(metric: Metric, metricLabel: string, t: TFunction<['common', 'pages']>) {
  return {
    headline: t('pages:coverage.layerUnmeasured', { metric: metricLabel }),
    body: metric === 'snr' ? t('pages:coverage.snrUnmeasuredBody') : undefined,
  };
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
    metricLabel,
    unit,
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
  if (isUnmeasured(heatmapError)) {
    const copy = unmeasuredCopy(metric, metricLabel, t);
    return (
      <div className="flex flex-col gap-1 text-sm text-text-muted" data-testid="surface-message">
        <p className="font-medium text-text-primary">{copy.headline}</p>
        {copy.body ? <p>{copy.body}</p> : null}
      </div>
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
    <HeatmapSurface
      png={heatmap.png}
      width={heatmap.width}
      height={heatmap.height}
      grid={heatmap.grid}
      gridCols={heatmap.gridCols}
      gridRows={heatmap.gridRows}
      cellSize={heatmap.cellSize}
      unit={unit}
      metric={metric}
    />
  );
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
function describeCoverage(
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
