import { useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { surveyClient } from '@/lib/client';
import { bytesToDataUrl, formatCoverageScore, reportFilename } from '@/lib/format';

type Metric = 'rssi' | 'snr';

interface SurveyDetailProps {
  surveyId: string;
  surveyName: string;
}

export function SurveyDetail({ surveyId, surveyName }: SurveyDetailProps) {
  const [metric, setMetric] = useState<Metric>('rssi');

  const reportMutation = useMutation({
    mutationFn: async () => {
      const reply = await surveyClient.generateReport({ surveyId });
      // Reuse the tested bytes→data-URL path and trigger a browser download
      // via a transient anchor; no library needed for a one-shot PDF save.
      const link = document.createElement('a');
      link.href = bytesToDataUrl(reply.pdf, 'application/pdf');
      link.download = reportFilename(surveyName);
      link.click();
    },
  });

  const heatmapQuery = useQuery({
    queryKey: ['heatmap', surveyId, metric],
    queryFn: () => surveyClient.getHeatmap({ surveyId, metric }),
  });

  const coverageQuery = useQuery({
    queryKey: ['coverage', surveyId],
    queryFn: () => surveyClient.getCoverage({ surveyId }),
  });

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="mb-4 flex items-center gap-2">
        <span className="text-sm font-medium text-text-secondary">Metric:</span>
        {(['rssi', 'snr'] as const).map((m) => (
          <button
            key={m}
            type="button"
            onClick={() => setMetric(m)}
            className={`rounded px-2 py-1 text-xs uppercase ${
              metric === m
                ? 'bg-brand-primary text-on-brand'
                : 'bg-surface-sunken text-text-secondary'
            }`}
          >
            {m}
          </button>
        ))}
        <button
          type="button"
          onClick={() => reportMutation.mutate()}
          disabled={reportMutation.isPending}
          className="ml-auto rounded bg-brand-primary px-3 py-1 text-xs text-on-brand disabled:opacity-50"
        >
          {reportMutation.isPending ? 'Generating…' : 'Download PDF report'}
        </button>
      </div>
      {reportMutation.isError && (
        <p className="mb-4 text-sm text-status-error">
          Failed to generate report: {String(reportMutation.error)}
        </p>
      )}

      <section className="mb-6">
        <h2 className="mb-2 text-sm font-semibold text-text-secondary">Heatmap</h2>
        {heatmapQuery.isLoading && <p className="text-sm text-text-muted">Loading heatmap…</p>}
        {heatmapQuery.isError && (
          <p className="text-sm text-status-error">
            Failed to load heatmap: {String(heatmapQuery.error)}
          </p>
        )}
        {heatmapQuery.data && (
          <div>
            <img
              src={bytesToDataUrl(heatmapQuery.data.png, 'image/png')}
              alt={`${metric.toUpperCase()} heatmap`}
              width={heatmapQuery.data.width}
              height={heatmapQuery.data.height}
              className="max-w-full rounded border border-hairline"
            />
            <p className="mt-1 text-xs text-text-muted">
              {heatmapQuery.data.sampleCount} samples · range {heatmapQuery.data.min.toFixed(1)} to{' '}
              {heatmapQuery.data.max.toFixed(1)}
            </p>
          </div>
        )}
      </section>

      <section>
        <h2 className="mb-2 text-sm font-semibold text-text-secondary">Coverage</h2>
        {coverageQuery.isLoading && <p className="text-sm text-text-muted">Loading coverage…</p>}
        {coverageQuery.isError && (
          <p className="text-sm text-status-error">
            Failed to load coverage: {String(coverageQuery.error)}
          </p>
        )}
        {coverageQuery.data && (
          <div className="space-y-2">
            <div className="flex gap-6 text-sm text-text-secondary">
              <span>Coverage score: {formatCoverageScore(coverageQuery.data.coverageScore)}</span>
              <span>Dead zones: {coverageQuery.data.deadZoneCount}</span>
            </div>
            {coverageQuery.data.recommendations.length > 0 && (
              <ul className="list-inside list-disc text-sm text-text-secondary">
                {coverageQuery.data.recommendations.map((recommendation) => (
                  <li key={recommendation}>{recommendation}</li>
                ))}
              </ul>
            )}
          </div>
        )}
      </section>
    </div>
  );
}
