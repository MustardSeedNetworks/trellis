import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { surveyClient } from '@/lib/client';

/**
 * ThroughputTarget names the iperf3 server a survey's active measurements run
 * against.
 *
 * It belongs to the survey rather than to each measurement, because comparing
 * two positions only means something if both were measured against the same
 * thing. Which is also why it is a field an operator fills in rather than a
 * setting with a default: there is no server that is right by default, and a
 * survey without one is walked passively, which is a perfectly good survey.
 */
export function ThroughputTarget({ surveyId, server }: { surveyId: string; server: string }) {
  const { t } = useTranslation(['common', 'pages']);
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState(server);

  const saveMutation = useMutation({
    mutationFn: (target: string) =>
      surveyClient.setThroughputTarget({ surveyId, server: target, durationSec: 0 }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['surveys'] });
    },
  });

  return (
    <div className="flex flex-wrap items-end gap-3">
      <label className="flex flex-col gap-1 text-sm" htmlFor="throughput-target">
        <span className="kicker">{t('pages:surveys.throughputTarget')}</span>
        <input
          id="throughput-target"
          type="text"
          value={draft}
          placeholder={t('pages:surveys.throughputTargetPlaceholder')}
          onChange={(event) => setDraft(event.target.value)}
          className="rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary"
          data-testid="throughput-target"
        />
      </label>
      <button
        type="button"
        onClick={() => saveMutation.mutate(draft.trim())}
        disabled={saveMutation.isPending || draft.trim() === server}
        data-testid="save-throughput-target"
        className="rounded border border-hairline px-3 py-2 text-sm text-text-primary hover:bg-surface-raised disabled:opacity-50"
      >
        {t('pages:surveys.saveTarget')}
      </button>
      {saveMutation.isError ? (
        <p className="text-sm text-status-error" data-testid="throughput-target-error">
          {String(saveMutation.error)}
        </p>
      ) : null}
    </div>
  );
}
