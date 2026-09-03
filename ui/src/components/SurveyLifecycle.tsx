import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { SurveySummary } from '@/gen/trellis/survey/v1/survey_pb';
import { surveyClient } from '@/lib/client';

interface SurveyLifecycleProps {
  survey: SurveySummary;
  /** The selection is stale once the survey is gone; the page clears it. */
  onDeleted: () => void;
}

const BUTTON =
  'rounded border border-hairline px-3 py-2 text-sm text-text-primary hover:bg-surface-hover disabled:opacity-50';
const PRIMARY =
  'rounded bg-brand-primary px-3 py-2 text-sm font-medium text-on-brand hover:bg-brand-accent disabled:opacity-50';
const DANGER =
  'rounded border border-status-error px-3 py-2 text-sm text-status-error hover:bg-surface-hover disabled:opacity-50';

/**
 * SurveyLifecycle moves a survey between the states the service defines.
 *
 * Only the transitions the current state allows are offered. The service
 * refuses the others with a precondition error, but a button that exists and
 * then fails reads as a defect; a button that is absent reads as the state.
 *
 * Delete is two steps on the same buttons rather than a modal: both steps are
 * ordinary focusable buttons, so the confirmation is reachable from the
 * keyboard — a modal that only a mouse can dismiss has shipped in this family
 * before.
 */
export function SurveyLifecycle({ survey, onDeleted }: SurveyLifecycleProps) {
  const { t } = useTranslation(['common', 'pages']);
  const queryClient = useQueryClient();
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['surveys'] });

  const startMutation = useMutation({
    mutationFn: () => surveyClient.startSurvey({ id: survey.id }),
    onSuccess: refresh,
  });
  const pauseMutation = useMutation({
    mutationFn: () => surveyClient.pauseSurvey({ id: survey.id }),
    onSuccess: refresh,
  });
  const completeMutation = useMutation({
    mutationFn: () => surveyClient.completeSurvey({ id: survey.id }),
    onSuccess: refresh,
  });
  const deleteMutation = useMutation({
    mutationFn: () => surveyClient.deleteSurvey({ id: survey.id }),
    onSuccess: async () => {
      await refresh();
      onDeleted();
    },
  });

  const busy =
    startMutation.isPending ||
    pauseMutation.isPending ||
    completeMutation.isPending ||
    deleteMutation.isPending;

  const transitionError =
    startMutation.error ?? pauseMutation.error ?? completeMutation.error ?? undefined;

  const canStart = survey.status === 'created' || survey.status === 'paused';
  const canPause = survey.status === 'in_progress';
  const canComplete = survey.status === 'in_progress' || survey.status === 'paused';

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-3">
        {canStart ? (
          <button
            type="button"
            onClick={() => startMutation.mutate()}
            disabled={busy}
            className={PRIMARY}
            data-testid="survey-start"
          >
            {survey.status === 'paused' ? t('pages:surveys.resume') : t('pages:surveys.start')}
          </button>
        ) : null}
        {canPause ? (
          <button
            type="button"
            onClick={() => pauseMutation.mutate()}
            disabled={busy}
            className={BUTTON}
            data-testid="survey-pause"
          >
            {t('pages:surveys.pause')}
          </button>
        ) : null}
        {canComplete ? (
          <button
            type="button"
            onClick={() => completeMutation.mutate()}
            disabled={busy}
            className={BUTTON}
            data-testid="survey-complete"
          >
            {t('pages:surveys.complete')}
          </button>
        ) : null}

        <span className="ml-auto flex flex-wrap items-center gap-3">
          {confirmingDelete ? (
            <>
              <span className="text-sm text-text-secondary">{t('pages:surveys.deletePrompt')}</span>
              <button
                type="button"
                onClick={() => setConfirmingDelete(false)}
                disabled={busy}
                className={BUTTON}
                data-testid="survey-delete-cancel"
              >
                {t('pages:surveys.keepIt')}
              </button>
              <button
                type="button"
                onClick={() => deleteMutation.mutate()}
                disabled={busy}
                className={DANGER}
                data-testid="survey-delete-confirm"
              >
                {t('pages:surveys.confirmDelete', { name: survey.name })}
              </button>
            </>
          ) : (
            <button
              type="button"
              onClick={() => setConfirmingDelete(true)}
              disabled={busy}
              className={DANGER}
              data-testid="survey-delete"
            >
              {t('pages:surveys.delete')}
            </button>
          )}
        </span>
      </div>

      {transitionError ? (
        <p className="text-sm text-status-error" data-testid="lifecycle-error">
          {t('pages:surveys.transitionFailed', { error: String(transitionError) })}
        </p>
      ) : null}
      {deleteMutation.isError ? (
        <p className="text-sm text-status-error" data-testid="lifecycle-error">
          {t('pages:surveys.deleteFailed', { error: String(deleteMutation.error) })}
        </p>
      ) : null}
    </div>
  );
}
