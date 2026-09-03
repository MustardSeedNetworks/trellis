import { useMutation, useQueryClient } from '@tanstack/react-query';
import { type FormEvent, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { surveyClient } from '@/lib/client';

interface SurveyCreateFormProps {
  /** Called with the new survey's ID so the page can select it. */
  onCreated: (id: string) => void;
}

/**
 * SurveyCreateForm opens an empty survey to walk.
 *
 * The service creates a survey with one floor and no plan, which is all a
 * walk needs to start; the plan and its scale arrive with floor import. The
 * interface is optional because two of the three capture backends resolve the
 * adapter themselves and only Linux reads the name.
 */
export function SurveyCreateForm({ onCreated }: SurveyCreateFormProps) {
  const { t } = useTranslation(['common', 'pages']);
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const [iface, setIface] = useState('');

  const createMutation = useMutation({
    mutationFn: (input: { name: string; interface: string }) => surveyClient.createSurvey(input),
    onSuccess: async (reply) => {
      await queryClient.invalidateQueries({ queryKey: ['surveys'] });
      setName('');
      setIface('');
      if (reply.survey) {
        onCreated(reply.survey.id);
      }
    },
  });

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = name.trim();
    if (trimmed === '' || createMutation.isPending) {
      return;
    }
    createMutation.mutate({ name: trimmed, interface: iface.trim() });
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-3 border-b border-hairline p-4">
      <span className="kicker">{t('pages:surveys.newSurvey')}</span>

      <label className="flex flex-col gap-1 text-sm" htmlFor="new-survey-name">
        <span className="text-text-secondary">{t('pages:surveys.newSurveyName')}</span>
        <input
          id="new-survey-name"
          type="text"
          value={name}
          onChange={(event) => setName(event.target.value)}
          className="rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary"
          data-testid="new-survey-name"
        />
      </label>

      <label className="flex flex-col gap-1 text-sm" htmlFor="new-survey-interface">
        <span className="text-text-secondary">{t('pages:surveys.interface')}</span>
        <input
          id="new-survey-interface"
          type="text"
          value={iface}
          onChange={(event) => setIface(event.target.value)}
          aria-describedby="new-survey-interface-hint"
          className="figure rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary"
          data-testid="new-survey-interface"
        />
        <span id="new-survey-interface-hint" className="text-xs text-text-muted">
          {t('pages:surveys.interfaceHint')}
        </span>
      </label>

      <button
        type="submit"
        disabled={name.trim() === '' || createMutation.isPending}
        className="w-fit rounded bg-brand-primary px-3 py-2 text-sm font-medium text-on-brand hover:bg-brand-accent disabled:opacity-50"
        data-testid="create-survey"
      >
        {createMutation.isPending ? t('pages:surveys.creating') : t('pages:surveys.create')}
      </button>

      {createMutation.isError ? (
        <p className="text-sm text-status-error" data-testid="create-survey-error">
          {t('pages:surveys.createFailed', { error: String(createMutation.error) })}
        </p>
      ) : null}
    </form>
  );
}
