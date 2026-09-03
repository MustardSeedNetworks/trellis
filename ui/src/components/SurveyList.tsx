import { useTranslation } from 'react-i18next';
import type { SurveySummary } from '@/gen/trellis/survey/v1/survey_pb';
import { surveyStatusLabel } from '@/lib/surveyStatus';

interface SurveyListProps {
  surveys: SurveySummary[];
  selectedId: string | undefined;
  onSelect: (id: string) => void;
}

export function SurveyList({ surveys, selectedId, onSelect }: SurveyListProps) {
  const { t } = useTranslation(['common', 'pages']);

  if (surveys.length === 0) {
    return <p className="p-4 text-sm text-text-muted">{t('pages:surveys.listEmpty')}</p>;
  }

  return (
    <ul className="divide-y divide-hairline">
      {surveys.map((survey) => (
        <li key={survey.id}>
          <button
            type="button"
            onClick={() => onSelect(survey.id)}
            aria-pressed={survey.id === selectedId}
            data-testid="survey-row"
            className={`w-full px-4 py-3 text-left hover:bg-surface-hover ${
              survey.id === selectedId ? 'bg-surface-hover' : ''
            }`}
          >
            <div className="font-medium text-text-primary">{survey.name}</div>
            <div className="text-xs text-text-muted">
              {t('pages:surveys.summary', {
                status: surveyStatusLabel(t, survey.status),
                floors: t('pages:surveys.floorCount', { count: survey.floorCount }),
                samples: t('pages:surveys.sampleCount', { count: survey.sampleCount }),
              })}
            </div>
          </button>
        </li>
      ))}
    </ul>
  );
}
