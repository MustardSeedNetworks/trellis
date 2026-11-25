import { Waypoints } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { SurveySummary } from '@/gen/trellis/survey/v1/survey_pb';
import { surveyStatusLabel, surveyStatusTone } from '@/lib/surveyStatus';
import { useTableLabels } from '@/lib/useTableLabels';
import { DataTable, type DataTableColumn } from '@/shared/DataTable';
import { EmptyState } from '@/shared/EmptyState';
import { StatusPill } from '@/shared/StatusPill';

interface SurveyListProps {
  surveys: SurveySummary[];
  loading?: boolean;
  selectedId: string | undefined;
  onSelect: (id: string) => void;
}

/**
 * SurveyList — every survey on this host, one row each. A row opens the
 * survey's detail beneath the table; there is no second pane to keep on screen
 * beside it, so the table gets the page's full width at any size.
 */
export function SurveyList({ surveys, loading = false, selectedId, onSelect }: SurveyListProps) {
  const { t } = useTranslation(['common', 'pages']);
  const labels = useTableLabels<SurveySummary>();

  const columns: DataTableColumn<SurveySummary>[] = [
    {
      key: 'name',
      header: t('common:labels.name'),
      sortValue: (survey) => survey.name,
      cell: (survey) => (
        <span className="font-medium text-text-primary" data-testid="survey-row">
          {survey.name}
        </span>
      ),
    },
    {
      key: 'status',
      header: t('common:labels.status'),
      sortValue: (survey) => surveyStatusLabel(t, survey.status),
      cell: (survey) => (
        <StatusPill
          tone={surveyStatusTone(survey.status)}
          label={surveyStatusLabel(t, survey.status)}
        />
      ),
    },
    {
      key: 'floors',
      header: t('common:labels.floors'),
      align: 'right',
      sortValue: (survey) => survey.floorCount,
      cell: (survey) => <span className="figure">{survey.floorCount}</span>,
    },
    {
      key: 'samples',
      header: t('common:labels.samples'),
      align: 'right',
      sortValue: (survey) => survey.sampleCount,
      cell: (survey) => <span className="figure">{survey.sampleCount}</span>,
    },
  ];

  return (
    <DataTable
      data-testid="survey-list"
      rows={surveys}
      columns={columns}
      getRowKey={(survey) => survey.id}
      caption={t('pages:surveys.listCaption')}
      labels={labels}
      loading={loading}
      pageSize={10}
      onRowClick={(survey) => onSelect(survey.id)}
      {...(selectedId === undefined ? {} : { selectedRowKey: selectedId })}
      empty={
        <div className="panel">
          <EmptyState
            icon={<Waypoints className="h-6 w-6" />}
            title={t('pages:surveys.noneCaptured')}
            hint={t('pages:surveys.emptyBody')}
          />
        </div>
      }
    />
  );
}
