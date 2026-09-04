import { useMutation, useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { CaptureSurface } from '@/components/CaptureSurface';
import { FloorPlanPanel } from '@/components/FloorPlanPanel';
import { SurveyLifecycle } from '@/components/SurveyLifecycle';
import { ThroughputTarget } from '@/components/ThroughputTarget';
import type { SurveySummary } from '@/gen/trellis/survey/v1/survey_pb';
import { surveyClient } from '@/lib/client';
import { bytesToDataUrl, reportFilename } from '@/lib/format';
import { surveyStatusLabel } from '@/lib/surveyStatus';

/**
 * SurveyDetail — what is stored about one survey, how to walk it, and what
 * can be produced from it.
 *
 * The heatmap and the dead-zone analysis used to render here. They moved to
 * the Coverage page, which is the Canvas archetype and gives the image the
 * toolbar, scale and findings panel it needs; a copy left behind would be the
 * same reading in two places with only one of them adjustable. The report
 * stays: it is produced *about this survey*, so it belongs to the survey. The
 * walk lives here too, because a point is captured *into this survey*.
 */
interface SurveyDetailProps {
  survey: SurveySummary;
  onDeleted: () => void;
}

export function SurveyDetail({ survey, onDeleted }: SurveyDetailProps) {
  const { t } = useTranslation(['common', 'pages']);

  // The floors are read here rather than passed down: the plan's dimensions and
  // scale live on the floor, and both the plan panel and the capture surface
  // need them.
  const floorsQuery = useQuery({
    queryKey: ['floors', survey.id],
    queryFn: () => surveyClient.listFloors({ surveyId: survey.id }),
  });
  const activeFloor =
    floorsQuery.data?.floors.find((floor) => floor.isActive) ?? floorsQuery.data?.floors[0];

  const planQuery = useQuery({
    queryKey: ['floor-plan', survey.id, activeFloor?.id ?? ''],
    queryFn: () =>
      surveyClient.getFloorPlanImage({ surveyId: survey.id, floorId: activeFloor?.id ?? '' }),
    enabled: activeFloor?.hasFloorPlan === true,
  });
  const planImage =
    planQuery.data && planQuery.data.image.length > 0
      ? {
          width: planQuery.data.width,
          height: planQuery.data.height,
          // The plan is bytes on the wire; the surface draws it as an image, so
          // it needs a URL. The same encoder the report PDF download uses.
          imageUrl: bytesToDataUrl(planQuery.data.image, 'image/png'),
        }
      : undefined;
  const reportMutation = useMutation({
    mutationFn: async () => {
      const reply = await surveyClient.generateReport({ surveyId: survey.id });
      // Reuse the tested bytes→data-URL path and trigger a browser download
      // via a transient anchor; no library needed for a one-shot PDF save.
      const link = document.createElement('a');
      link.href = bytesToDataUrl(reply.pdf, 'application/pdf');
      link.download = reportFilename(survey.name);
      link.click();
    },
  });

  const facts = [
    { label: t('common:labels.status'), value: surveyStatusLabel(t, survey.status) },
    { label: t('common:labels.floors'), value: String(survey.floorCount) },
    { label: t('common:labels.samples'), value: String(survey.sampleCount) },
    {
      label: t('common:labels.floorPlan'),
      value: survey.hasFloorPlan ? t('common:values.present') : t('common:values.none'),
    },
  ];

  return (
    <div className="panel flex-1 overflow-y-auto p-6" data-testid="survey-detail">
      <dl className="flex flex-wrap gap-x-10 gap-y-4">
        {facts.map((fact) => (
          <div key={fact.label}>
            <dd className="figure text-lg font-bold leading-none text-text-primary">
              {fact.value}
            </dd>
            <dt className="kicker mt-2">{fact.label}</dt>
          </div>
        ))}
      </dl>

      <div className="mt-6 border-t border-hairline pt-6">
        <SurveyLifecycle survey={survey} onDeleted={onDeleted} />
      </div>

      <div className="mt-6 border-t border-hairline pt-6">
        {activeFloor ? (
          <FloorPlanPanel
            surveyId={survey.id}
            floorId={activeFloor.id}
            hasPlan={activeFloor.hasFloorPlan}
            scaleM={activeFloor.scaleM}
          />
        ) : null}
      </div>

      <div className="mt-6 border-t border-hairline pt-6">
        <ThroughputTarget surveyId={survey.id} server={survey.iperfServer} />
      </div>

      <div className="mt-6 border-t border-hairline pt-6">
        <CaptureSurface
          surveyId={survey.id}
          surveyName={survey.name}
          walking={survey.status === 'in_progress'}
          capture={survey.capture}
          throughputTarget={survey.iperfServer}
          plan={planImage}
        />
      </div>

      <div className="mt-6 flex flex-wrap items-center gap-3 border-t border-hairline pt-6">
        <Link
          to={{ pathname: '/coverage', search: `?survey=${encodeURIComponent(survey.id)}` }}
          className="rounded border border-hairline px-3 py-2 text-sm text-text-primary hover:bg-surface-hover"
          data-testid="plot-coverage"
        >
          {t('pages:surveys.plotCoverage')}
        </Link>
        <button
          type="button"
          onClick={() => reportMutation.mutate()}
          disabled={reportMutation.isPending}
          className="rounded bg-brand-primary px-3 py-2 text-sm font-medium text-on-brand hover:bg-brand-accent disabled:opacity-50"
        >
          {reportMutation.isPending
            ? t('common:buttons.generating')
            : t('pages:surveys.downloadReport')}
        </button>
      </div>

      {reportMutation.isError && (
        <p className="mt-3 text-sm text-status-error">
          {t('pages:surveys.reportFailed', { error: String(reportMutation.error) })}
        </p>
      )}
    </div>
  );
}
