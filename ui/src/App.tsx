import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { ImportSurvey } from '@/components/ImportSurvey';
import { SurveyDetail } from '@/components/SurveyDetail';
import { SurveyList } from '@/components/SurveyList';
import { surveyClient } from '@/lib/client';

export function App() {
  const [selectedId, setSelectedId] = useState<string | undefined>();

  const surveysQuery = useQuery({
    queryKey: ['surveys'],
    queryFn: () => surveyClient.listSurveys({}),
  });

  const surveys = surveysQuery.data?.surveys ?? [];
  const selectedSurvey = surveys.find((s) => s.id === selectedId);

  return (
    <div className="flex h-screen flex-col bg-slate-50 text-slate-900">
      <header className="border-b border-slate-200 bg-white px-6 py-4">
        <h1 className="text-lg font-semibold">Trellis</h1>
        <p className="text-xs text-slate-500">Wi-Fi survey heatmaps and coverage analysis</p>
      </header>

      <div className="flex flex-1 overflow-hidden">
        <aside className="flex w-72 flex-col border-r border-slate-200 bg-white">
          <div className="flex-1 overflow-y-auto">
            {surveysQuery.isLoading && (
              <p className="p-4 text-sm text-slate-500">Loading surveys…</p>
            )}
            {surveysQuery.isError && (
              <p className="p-4 text-sm text-red-600">
                Failed to load surveys: {String(surveysQuery.error)}
              </p>
            )}
            {surveysQuery.data && (
              <SurveyList surveys={surveys} selectedId={selectedId} onSelect={setSelectedId} />
            )}
          </div>
          <ImportSurvey />
        </aside>

        {selectedId ? (
          <SurveyDetail surveyId={selectedId} surveyName={selectedSurvey?.name ?? 'survey'} />
        ) : (
          <div className="flex flex-1 items-center justify-center text-sm text-slate-500">
            Select a survey to view its heatmap and coverage
          </div>
        )}
      </div>
    </div>
  );
}
