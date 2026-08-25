import { useTranslation } from 'react-i18next';
import type { SurveySummary } from '@/gen/trellis/survey/v1/survey_pb';

interface SurveyListProps {
  surveys: SurveySummary[];
  selectedId: string | undefined;
  onSelect: (id: string) => void;
}

export function SurveyList({ surveys, selectedId, onSelect }: SurveyListProps) {
  const { t } = useTranslation('pages');

  if (surveys.length === 0) {
    return <p className="p-4 text-sm text-text-muted">{t('surveys.listEmpty')}</p>;
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
              {t('surveys.summary', {
                status: survey.status,
                floors: t('surveys.floorCount', { count: survey.floorCount }),
                samples: t('surveys.sampleCount', { count: survey.sampleCount }),
              })}
            </div>
          </button>
        </li>
      ))}
    </ul>
  );
}
