import type { SurveySummary } from '@/gen/trellis/survey/v1/survey_pb';

interface SurveyListProps {
  surveys: SurveySummary[];
  selectedId: string | undefined;
  onSelect: (id: string) => void;
}

export function SurveyList({ surveys, selectedId, onSelect }: SurveyListProps) {
  if (surveys.length === 0) {
    return (
      <p className="p-4 text-sm text-text-muted">
        No surveys yet. Import an AirMapper file to get started.
      </p>
    );
  }

  return (
    <ul className="divide-y divide-hairline">
      {surveys.map((survey) => (
        <li key={survey.id}>
          <button
            type="button"
            onClick={() => onSelect(survey.id)}
            className={`w-full px-4 py-3 text-left hover:bg-surface-hover ${
              survey.id === selectedId ? 'bg-surface-hover' : ''
            }`}
          >
            <div className="font-medium text-text-primary">{survey.name}</div>
            <div className="text-xs text-text-muted">
              {survey.status} · {survey.floorCount} floor{survey.floorCount === 1 ? '' : 's'} ·{' '}
              {survey.sampleCount} samples
            </div>
          </button>
        </li>
      ))}
    </ul>
  );
}
