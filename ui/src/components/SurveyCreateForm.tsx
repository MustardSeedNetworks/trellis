import { useMutation, useQueryClient } from '@tanstack/react-query';
import { type FormEvent, useId, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { surveyClient } from '@/lib/client';
import { Alert } from '@/shared/Alert';
import { Button } from '@/shared/Button';
import { FormField } from '@/shared/FormField';
import { Input } from '@/shared/Input';

interface SurveyCreateFormProps {
  /** Called with the new survey's ID so the page can select it. */
  onCreated: (id: string) => void;
  /**
   * Whether Create is the page's one primary button. It is until a survey is
   * open: that survey's Start is then the action an operator is after.
   */
  primary?: boolean;
}

/**
 * SurveyCreateForm opens an empty survey to walk.
 *
 * The service creates a survey with one floor and no plan, which is all a
 * walk needs to start; the plan and its scale arrive with floor import. The
 * interface is optional because two of the three capture backends resolve the
 * adapter themselves and only Linux reads the name.
 */
export function SurveyCreateForm({ onCreated, primary = true }: SurveyCreateFormProps) {
  const { t } = useTranslation(['common', 'pages']);
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const [iface, setIface] = useState('');
  const errorId = useId();

  const createMutation = useMutation({
    mutationFn: (input: { name: string; interface: string }) => surveyClient.createSurvey(input),
    onSuccess: async (reply) => {
      // A list still loading was requested before this survey existed, and
      // invalidate alone would join it rather than refetch while the query has
      // no data yet (trellis#739). Cancel it so the refetch sees the survey.
      await queryClient.cancelQueries({ queryKey: ['surveys'] });
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

  // The failure belongs to the whole form, so both fields point at it: the hint
  // still reads first (what the field is for), the error second (what went wrong).
  const describedBy = (own: string | undefined) =>
    [own, createMutation.isError ? errorId : undefined].filter(Boolean).join(' ') || undefined;

  return (
    <form onSubmit={handleSubmit} className="flex flex-wrap items-start gap-4">
      <FormField label={t('pages:surveys.newSurveyName')}>
        {(control) => (
          <Input
            {...control}
            aria-describedby={describedBy(control['aria-describedby'])}
            value={name}
            onChange={(event) => setName(event.target.value)}
            data-testid="new-survey-name"
          />
        )}
      </FormField>

      <FormField label={t('pages:surveys.interface')} hint={t('pages:surveys.interfaceHint')}>
        {(control) => (
          <Input
            {...control}
            aria-describedby={describedBy(control['aria-describedby'])}
            value={iface}
            onChange={(event) => setIface(event.target.value)}
            className="figure"
            data-testid="new-survey-interface"
          />
        )}
      </FormField>

      <Button
        type="submit"
        variant={primary ? 'primary' : 'secondary'}
        loading={createMutation.isPending}
        disabled={name.trim() === ''}
        className="sm:mt-[1.625rem]"
        data-testid="create-survey"
      >
        {createMutation.isPending ? t('pages:surveys.creating') : t('pages:surveys.create')}
      </Button>

      {createMutation.isError ? (
        <div id={errorId} className="w-full" data-testid="create-survey-error">
          <Alert tone="error">
            {t('pages:surveys.createFailed', { error: String(createMutation.error) })}
          </Alert>
        </div>
      ) : null}
    </form>
  );
}
