import { ConnectError } from '@connectrpc/connect';
import { useQuery } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { Signal } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useSearchParams } from 'react-router';
import { AccessPointSelect } from '@/components/AccessPointSelect';
import { CoverageFindings } from '@/components/CoverageFindings';
import { HeatmapLegend } from '@/components/HeatmapLegend';
import { HeatmapSurface } from '@/components/HeatmapSurface';
import { PredictionView } from '@/components/PredictionView';
import { surveyClient } from '@/lib/client';
import {
  apField,
  heatmapOptions,
  isPredictedLayer,
  METRICS,
  type Metric,
  metricName,
} from '@/lib/coverageLayers';
import {
  DEFAULT_THRESHOLDS,
  isCoverageMetric,
  parseThreshold,
  THRESHOLDS,
} from '@/lib/coverageThreshold';
import { describeCoverage, isUnmeasured, unmeasuredCopy } from '@/lib/coverageVerdict';
import { formatSignal } from '@/lib/format';
import { BANDS, type Band, isBand } from '@/lib/placement';
import { Alert } from '@/shared/Alert';
import { ButtonLink } from '@/shared/Button';
import { Card } from '@/shared/Card';
import { EmptyState } from '@/shared/EmptyState';
import { FormField } from '@/shared/FormField';
import { Input } from '@/shared/Input';
import { Select } from '@/shared/Select';
import { Skeleton } from '@/shared/Skeleton';

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
 * The measured layers are the ones the service renders from a walk: rssi, snr
 * and download throughput. `core/survey` also defines density and
 * interference scales, but no RPC reaches them, so offering them here would
 * promise a picture the product cannot draw. The predicted layer is not
 * measured at all: it is drawn from the APs placed on the floor, which is why
 * it brings its own surface and side panel (PredictionView).
 */
export function CoveragePage() {
  const { t } = useTranslation(['common', 'pages']);
  /* The survey lives in the URL so "Plot coverage" on a survey opens that
     survey's coverage, and so a floor worth showing someone is a link. */
  const [searchParams, setSearchParams] = useSearchParams();
  const [metric, setMetric] = useState<Metric>('rssi');
  /* A prediction is for one band; 5 GHz is where most clients are served. */
  const [band, setBand] = useState<Band>('5 GHz');
  const predicted = metric === 'predicted_rssi';
  /* The delta is drawn like a measured map, but it is for one band too. */
  const banded = isPredictedLayer(metric);
  /* One threshold per metric, not one control that changes unit under the
     operator: carrying -75 into a dB box would ask for a margin no radio has,
     and the number an operator settled on for one layer is still the one they
     want when they come back to it. */
  const [thresholds, setThresholds] = useState(DEFAULT_THRESHOLDS);

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

  /* Download throughput has no dead-zone rule, so the analysis is not asked
     about it: offering it would only produce an error where a finding belongs. */
  const coverageMetric = isCoverageMetric(metric) ? metric : undefined;
  const analysable = coverageMetric !== undefined;
  const bounds = THRESHOLDS[coverageMetric ?? 'rssi'];
  const threshold = thresholds[coverageMetric ?? 'rssi'];

  /* A BSSID belongs to the floor that heard it, so the choice is kept with
     its floor: elsewhere it would be refused as not heard. Only rssi and snr
     have a per-AP reading, so the throughput layer is always the whole floor. */
  const floorKey = `${surveyId}/${floorId}`;
  const [apChoice, setApChoice] = useState({ floorKey: '', bssid: '' });
  const bssid = analysable && apChoice.floorKey === floorKey ? apChoice.bssid : '';

  const heatmapQuery = useQuery(heatmapOptions(surveyId, metric, floorId, bssid, band));
  /* Every reply lists the floor's heard APs. They are read from the
     whole-floor RSSI map the page opens on, not the map on screen: an AP with
     no SNR reading fails to draw, and the way back must not go with it. */
  const heardQuery = useQuery(heatmapOptions(surveyId, 'rssi', floorId, '', band));

  const coverageQuery = useQuery({
    queryKey: ['coverage', surveyId, metric, threshold, floorId, bssid],
    queryFn: () =>
      surveyClient.getCoverage({
        surveyId: surveyId ?? '',
        metric,
        threshold,
        floorId,
        ...apField(bssid),
      }),
    enabled: surveyId !== undefined && analysable,
  });

  const accessPoints = heardQuery.data?.accessPoints ?? [];
  /* A refetch that fails keeps the last good reply as data; the error is what
     is true now. Deleting the last placed AP is exactly that refetch. */
  const heatmap = heatmapQuery.error ? undefined : heatmapQuery.data;
  const unit = METRICS.find((m) => m.id === metric)?.unit ?? '';
  const metricLabel = metricName(metric, t);
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

  if (surveysQuery.isError) {
    return (
      <Alert tone="error" title={t('pages:surveys.notArriving')}>
        {t('pages:coverage.surveyServiceSilent', { error: String(surveysQuery.error) })}
      </Alert>
    );
  }
  if (surveysQuery.isLoading) {
    return (
      <Card>
        <div className="flex flex-col gap-3" data-testid="surface-message">
          <Skeleton className="h-10 w-1/2" />
          <Skeleton className="h-40" />
          <p className="sr-only">{t('pages:coverage.loadingSurveys')}</p>
        </div>
      </Card>
    );
  }
  /* No survey is not a layer with nothing drawn on it: there is nothing to
     plot, so there is no toolbar, no empty canvas and no findings, only what
     to do next. */
  if (surveys.length === 0) {
    return (
      <Card>
        <EmptyState
          icon={<Signal className="h-6 w-6" />}
          title={t('pages:coverage.noSurveysTitle')}
          hint={t('pages:coverage.noSurveys')}
          action={
            <ButtonLink variant="primary" renderLink={(link) => <Link to="/import" {...link} />}>
              {t('pages:coverage.importSurvey')}
            </ButtonLink>
          }
          className="py-10"
        />
      </Card>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {/* A row of controls on a laptop; a stack on a phone. Each field is a
          full-width block below sm, where that width is definite: a <select>
          keeps a minimum width from its longest option, so a survey with a
          long name pushed its own label past the panel. */}
      <Card className="flex flex-wrap items-end gap-x-4 gap-y-3" data-testid="coverage-toolbar">
        <FormField label={t('common:labels.survey')} className="sm:w-56">
          {(control) => (
            <Select
              {...control}
              value={surveyId ?? ''}
              onValueChange={(value) => setSearchParams({ survey: value })}
              options={[
                ...(surveyId !== undefined && !listed
                  ? [{ value: surveyId, label: t('pages:coverage.notInList', { id: surveyId }) }]
                  : []),
                ...surveys.map((survey) => ({ value: survey.id, label: survey.name })),
              ]}
              data-testid="coverage-survey"
            />
          )}
        </FormField>

        {floors.length > 1 ? (
          <FormField label={t('common:labels.floor')} className="sm:w-44">
            {(control) => (
              <Select
                {...control}
                value={floorId}
                onValueChange={(value) =>
                  setSearchParams(
                    value === ''
                      ? { survey: surveyId ?? '' }
                      : { survey: surveyId ?? '', floor: value },
                  )
                }
                /* The active floor by name rather than a blank row: "which
                   floor is this" must be answerable without opening the list. */
                options={floors.map((floor) => ({
                  value: floor.isActive ? '' : floor.id,
                  label: floor.name,
                }))}
                data-testid="coverage-floor"
              />
            )}
          </FormField>
        ) : null}

        <FormField label={t('common:labels.metric')} className="sm:w-48">
          {(control) => (
            <Select
              {...control}
              value={metric}
              onValueChange={(value) => {
                const chosen = METRICS.find((option) => option.id === value);
                if (chosen) {
                  setMetric(chosen.id);
                }
              }}
              options={METRICS.map((option) => ({
                value: option.id,
                label: metricName(option.id, t),
              }))}
              data-testid="coverage-metric"
            />
          )}
        </FormField>

        {banded ? (
          <FormField label={t('pages:coverage.band')} className="sm:w-32">
            {(control) => (
              <Select
                {...control}
                value={band}
                onValueChange={(value) => {
                  if (isBand(value)) {
                    setBand(value);
                  }
                }}
                options={BANDS.map((option) => ({ value: option, label: option }))}
                data-testid="coverage-band"
              />
            )}
          </FormField>
        ) : null}

        {analysable && accessPoints.length > 0 ? (
          <AccessPointSelect
            accessPoints={accessPoints}
            value={bssid}
            onChange={(chosen) => setApChoice({ floorKey, bssid: chosen })}
          />
        ) : null}

        {/* Only where it means something. The threshold and the findings below
            speak about a layer the analysis has a rule for; over a throughput
            layer they would answer a question nobody asked, and a control that
            appears to do nothing is worse than one that is not there. */}
        {coverageMetric !== undefined ? (
          <FormField label={t('common:labels.deadZoneThreshold')} className="sm:w-40">
            {(control) => (
              <div className="flex items-center gap-2">
                <Input
                  {...control}
                  type="number"
                  value={threshold}
                  min={bounds.min}
                  max={bounds.max}
                  step={1}
                  onChange={(event) => {
                    const parsed = parseThreshold(coverageMetric, event.target.value);
                    if (parsed !== undefined) {
                      setThresholds((current) => ({ ...current, [coverageMetric]: parsed }));
                    }
                  }}
                  className="figure"
                  data-testid="coverage-threshold"
                />
                <span className="text-sm text-text-secondary" data-testid="coverage-threshold-unit">
                  {unit}
                </span>
              </div>
            )}
          </FormField>
        ) : null}

        {heatmap && !predicted ? (
          <span className="figure text-xs text-text-muted sm:ml-auto" data-testid="surface-meta">
            {t('pages:coverage.surfaceMeta', {
              metric: heatmap.metric,
              count: heatmap.sampleCount,
              min: formatSignal(heatmap.min, unit),
              max: formatSignal(heatmap.max, unit),
            })}
          </span>
        ) : null}
      </Card>

      <div className="grid grid-cols-1 items-start gap-4 xl:grid-cols-[1fr_320px]">
        {predicted && surveyId !== undefined ? (
          <PredictionView
            surveyId={surveyId}
            floorId={floorId}
            band={band}
            heatmap={heatmap}
            heatmapError={heatmapQuery.error}
            heatmapLoading={heatmapQuery.isLoading}
          />
        ) : (
          <>
            <Card className="flex min-w-0 flex-col gap-4">
              {renderSurface(
                {
                  heatmapLoading: heatmapQuery.isLoading,
                  heatmapError: heatmapQuery.error,
                  heatmap,
                  metric,
                  metricLabel,
                  unit,
                },
                t,
              )}
              {/* Pinned to the foot of the surface card, so the one thing that
                  says what the colours mean is on screen while an operator is
                  reading them (#484). Sticky rather than fixed: it belongs to
                  this card, and on a short plan it simply sits where it always
                  did. */}
              {heatmap ? (
                <div className="sticky bottom-0 -mx-4 -mb-4 bg-surface-raised px-4 pb-4 pt-3 sm:-mx-5 sm:-mb-5 sm:px-5 sm:pb-5">
                  <HeatmapLegend stops={heatmap.legend} unit={unit} />
                </div>
              ) : null}
            </Card>

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
          </>
        )}
      </div>
    </div>
  );
}

interface SurfaceState {
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
 * The surface says which of its several failures happened. A blank card would
 * read as "this floor has no coverage" in every one of them.
 */
function renderSurface(
  { heatmapLoading, heatmapError, heatmap, metric, metricLabel, unit }: SurfaceState,
  t: TFunction<['common', 'pages']>,
) {
  if (isUnmeasured(heatmapError)) {
    const copy = unmeasuredCopy(metric, metricLabel, t);
    /* A delta can be missing for several reasons — no scale, no BSSID placed,
       a walk that never heard one — and only the service knows which. */
    const body =
      metric === 'predicted_delta' ? ConnectError.from(heatmapError).rawMessage : copy.body;
    return (
      <div data-testid="surface-message">
        <EmptyState
          icon={<Signal className="h-6 w-6" />}
          title={copy.headline}
          {...(body ? { hint: body } : {})}
        />
      </div>
    );
  }
  if (heatmapError) {
    /* A survey with no floor plan is the common case here, and the service
       says so in its message — worth quoting rather than replacing. */
    return (
      <div data-testid="surface-message">
        <Alert tone="error">
          {t('pages:coverage.heatmapFailed', { error: String(heatmapError) })}
        </Alert>
      </div>
    );
  }
  if (heatmapLoading || !heatmap) {
    return (
      <div className="flex flex-col gap-3" data-testid="surface-message">
        <Skeleton className="h-48" />
        <p className="text-sm text-text-muted">{t('pages:coverage.renderingHeatmap')}</p>
      </div>
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
