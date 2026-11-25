import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { SurveySummary } from '@/gen/trellis/survey/v1/survey_pb';
import { surveyClient } from '@/lib/client';
import { Button } from '@/shared/Button';
import { ConfirmDialog } from '@/shared/ConfirmDialog';
import { useToast } from '@/ui/toastContext';

interface SurveyLifecycleProps {
  survey: SurveySummary;
  /** The selection is stale once the survey is gone; the page clears it. */
  onDeleted: () => void;
}

/**
 * SurveyLifecycle moves a survey between the states the service defines.
 *
 * Only the transitions the current state allows are offered. The service
 * refuses the others with a precondition error, but a button that exists and
 * then fails reads as a defect; a button that is absent reads as the state.
 *
 * Delete is the filled destructive button and goes through a ConfirmDialog,
 * which takes focus on Cancel and closes on Escape, so it is as reachable from
 * the keyboard as the buttons beside it.
 */
export function SurveyLifecycle({ survey, onDeleted }: SurveyLifecycleProps) {
  const { t } = useTranslation(['common', 'pages']);
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['surveys'] });

  const transitionFailed = (error: unknown) =>
    addToast(t('pages:surveys.transitionFailed', { error: String(error) }), 'error');

  const startMutation = useMutation({
    mutationFn: () => surveyClient.startSurvey({ id: survey.id }),
    onSuccess: refresh,
    onError: transitionFailed,
  });
  const pauseMutation = useMutation({
    mutationFn: () => surveyClient.pauseSurvey({ id: survey.id }),
    onSuccess: refresh,
    onError: transitionFailed,
  });
  const completeMutation = useMutation({
    mutationFn: () => surveyClient.completeSurvey({ id: survey.id }),
    onSuccess: refresh,
    onError: transitionFailed,
  });
  const deleteMutation = useMutation({
    mutationFn: () => surveyClient.deleteSurvey({ id: survey.id }),
    onSuccess: async () => {
      await refresh();
      onDeleted();
    },
    onError: (error) => {
      setConfirmingDelete(false);
      addToast(t('pages:surveys.deleteFailed', { error: String(error) }), 'error');
    },
  });

  const busy =
    startMutation.isPending ||
    pauseMutation.isPending ||
    completeMutation.isPending ||
    deleteMutation.isPending;

  const canStart = survey.status === 'created' || survey.status === 'paused';
  const canPause = survey.status === 'in_progress';
  const canComplete = survey.status === 'in_progress' || survey.status === 'paused';

  return (
    <div className="flex flex-wrap items-center gap-3">
      {canStart ? (
        <Button
          variant="primary"
          onClick={() => startMutation.mutate()}
          disabled={busy}
          data-testid="survey-start"
        >
          {survey.status === 'paused' ? t('pages:surveys.resume') : t('pages:surveys.start')}
        </Button>
      ) : null}
      {canPause ? (
        <Button onClick={() => pauseMutation.mutate()} disabled={busy} data-testid="survey-pause">
          {t('pages:surveys.pause')}
        </Button>
      ) : null}
      {canComplete ? (
        <Button
          onClick={() => completeMutation.mutate()}
          disabled={busy}
          data-testid="survey-complete"
        >
          {t('pages:surveys.complete')}
        </Button>
      ) : null}

      <Button
        variant="destructive"
        onClick={() => setConfirmingDelete(true)}
        disabled={busy}
        className="sm:ml-auto"
        data-testid="survey-delete"
      >
        {t('pages:surveys.delete')}
      </Button>

      <ConfirmDialog
        open={confirmingDelete}
        title={t('pages:surveys.delete')}
        message={t('pages:surveys.deletePrompt')}
        confirmLabel={t('pages:surveys.confirmDelete', { name: survey.name })}
        cancelLabel={t('pages:surveys.keepIt')}
        closeLabel={t('common:accessibility.close')}
        onConfirm={() => deleteMutation.mutate()}
        onCancel={() => setConfirmingDelete(false)}
        confirming={deleteMutation.isPending}
      />
    </div>
  );
}
