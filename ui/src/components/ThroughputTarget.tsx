import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { surveyClient } from '@/lib/client';
import { Alert } from '@/shared/Alert';
import { Button } from '@/shared/Button';
import { FormField } from '@/shared/FormField';
import { Input } from '@/shared/Input';

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
      <FormField label={t('pages:surveys.throughputTarget')} className="sm:w-72">
        {(control) => (
          <Input
            {...control}
            value={draft}
            placeholder={t('pages:surveys.throughputTargetPlaceholder')}
            onChange={(event) => setDraft(event.target.value)}
            aria-describedby={saveMutation.isError ? 'throughput-target-error' : undefined}
            data-testid="throughput-target"
          />
        )}
      </FormField>
      <Button
        onClick={() => saveMutation.mutate(draft.trim())}
        loading={saveMutation.isPending}
        disabled={draft.trim() === server}
        data-testid="save-throughput-target"
      >
        {t('pages:surveys.saveTarget')}
      </Button>
      {saveMutation.isError ? (
        <div id="throughput-target-error" className="w-full" data-testid="throughput-target-error">
          <Alert tone="error">{String(saveMutation.error)}</Alert>
        </div>
      ) : null}
    </div>
  );
}
