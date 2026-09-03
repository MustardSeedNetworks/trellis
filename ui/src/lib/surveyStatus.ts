import type { TFunction } from 'i18next';

/**
 * surveyStatusLabel names a survey's lifecycle state in the operator's
 * language. The wire carries core/survey's identifiers; before the walk
 * existed only "completed" and the importer's states ever reached the screen,
 * so they were shown raw. A state the service adds later still shows raw
 * rather than vanishing.
 */
export function surveyStatusLabel(t: TFunction<['common', 'pages']>, status: string): string {
  switch (status) {
    case 'created':
      return t('pages:surveys.status.created');
    case 'in_progress':
      return t('pages:surveys.status.inProgress');
    case 'paused':
      return t('pages:surveys.status.paused');
    case 'completed':
      return t('pages:surveys.status.completed');
    default:
      return status;
  }
}
