import { useMutation } from '@tanstack/react-query';
import { Link } from 'react-router';
import type { SurveySummary } from '@/gen/trellis/survey/v1/survey_pb';
import { surveyClient } from '@/lib/client';
import { bytesToDataUrl, reportFilename } from '@/lib/format';

/**
 * SurveyDetail — what is stored about one survey, and what can be produced
 * from it.
 *
 * The heatmap and the dead-zone analysis used to render here. They moved to
 * the Coverage page, which is the Canvas archetype and gives the image the
 * toolbar, scale and findings panel it needs; a copy left behind would be the
 * same reading in two places with only one of them adjustable. The report
 * stays: it is produced *about this survey*, so it belongs to the survey.
 */
interface SurveyDetailProps {
  survey: SurveySummary;
}

export function SurveyDetail({ survey }: SurveyDetailProps) {
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
    { label: 'Status', value: survey.status },
    { label: 'Floors', value: String(survey.floorCount) },
    { label: 'Samples', value: String(survey.sampleCount) },
    { label: 'Floor plan', value: survey.hasFloorPlan ? 'Present' : 'None' },
  ];

  return (
    <div className="panel flex-1 overflow-y-auto p-6">
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

      <div className="mt-6 flex flex-wrap items-center gap-3 border-t border-hairline pt-6">
        <Link
          to="/coverage"
          className="rounded border border-hairline px-3 py-2 text-sm text-text-primary hover:bg-surface-hover"
        >
          Plot coverage
        </Link>
        <button
          type="button"
          onClick={() => reportMutation.mutate()}
          disabled={reportMutation.isPending}
          className="rounded bg-brand-primary px-3 py-2 text-sm font-medium text-on-brand hover:bg-brand-accent disabled:opacity-50"
        >
          {reportMutation.isPending ? 'Generating…' : 'Download PDF report'}
        </button>
      </div>

      {reportMutation.isError && (
        <p className="mt-3 text-sm text-status-error">
          Failed to generate report: {String(reportMutation.error)}
        </p>
      )}
    </div>
  );
}
