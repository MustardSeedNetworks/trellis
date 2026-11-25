import { useMutation, useQueryClient } from '@tanstack/react-query';
import { type ChangeEvent, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { surveyClient } from '@/lib/client';
import { Alert } from '@/shared/Alert';
import { Button } from '@/shared/Button';
import { Card } from '@/shared/Card';
import { FormField } from '@/shared/FormField';
import { Input } from '@/shared/Input';

/**
 * Import — AirMapper and AirMagnet ingest.
 *
 * The capability shipped before the page did, as a button in the Surveys rail
 * that collected the survey name with window.prompt. A prompt is the wrong
 * shape for this: it demands the name before the operator has seen anything
 * about the file, cancelling it discards the chosen file, and it cannot be
 * driven by a test. The page keeps the file and the name on screen together
 * until the operator commits.
 *
 * A failed import is an `Alert` above the form, and a finished one says what
 * the archive yielded — an import that fails silently and leaves a calm empty
 * form reads as "nothing happened".
 *
 * The two formats share the page because they are the same act to the
 * operator: choose the file another tool wrote, name the survey, import. The
 * file's extension picks the service call, so nothing asks the operator which
 * vendor wrote a file they are looking at in a file dialog.
 */
export function ImportPage() {
  const { t } = useTranslation(['common', 'pages']);
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | undefined>();
  const [name, setName] = useState('');

  const importMutation = useMutation({
    mutationFn: async ({ surveyName, data }: { surveyName: string; data: Uint8Array }) =>
      isAirMagnet(file)
        ? surveyClient.importAirMagnet({ name: surveyName, svdData: data })
        : surveyClient.importAirMapper({ name: surveyName, ampData: data }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['surveys'] });
    },
  });

  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const chosen = event.target.files?.[0];
    if (!chosen) {
      return;
    }
    importMutation.reset();
    setFile(chosen);
    // Proposed, not imposed — the operator can rename before committing, which
    // is the whole reason this is a field and not a prompt.
    setName(chosen.name.replace(/\.(amp|svd)$/i, ''));
  }

  async function handleImport() {
    if (!file || name.trim() === '') {
      return;
    }
    const buffer = await file.arrayBuffer();
    importMutation.mutate({ surveyName: name.trim(), data: new Uint8Array(buffer) });
  }

  const chosen = file ? `${file.name}, ${formatBytes(file.size)}` : undefined;
  const imported = importMutation.data?.survey;
  const nameMissing = file !== undefined && name.trim() === '';

  return (
    <div className="flex flex-col gap-4">
      {importMutation.error ? (
        <Alert tone="error" title={t('pages:import.failed')}>
          {importMutation.error instanceof Error
            ? importMutation.error.message
            : String(importMutation.error)}
        </Alert>
      ) : null}

      {imported ? (
        <div data-testid="import-result">
          <Alert tone="success" title={t('pages:import.imported', { name: imported.name })}>
            {t('pages:import.importedBody', { id: imported.id })}{' '}
            {t('pages:import.importedFigures', {
              samples: imported.sampleCount,
              floors: imported.floorCount,
            })}
          </Alert>
        </div>
      ) : null}

      <Card>
        <input
          ref={fileInputRef}
          type="file"
          accept=".amp,.svd"
          onChange={handleFileChange}
          className="hidden"
          data-testid="amp-file-input"
        />

        <div className="flex flex-col gap-5">
          <FormField
            label={t('pages:import.archive')}
            hint={chosen ?? t('pages:import.noFileChosenBody')}
          >
            {() => (
              <div>
                <Button onClick={() => fileInputRef.current?.click()}>
                  {file ? t('pages:import.chooseDifferentFile') : t('pages:import.chooseFile')}
                </Button>
              </div>
            )}
          </FormField>

          {isAirMagnet(file) ? <Alert tone="info">{t('pages:import.noPlanInExport')}</Alert> : null}

          <FormField
            label={t('pages:import.surveyName')}
            {...(nameMissing ? { error: t('pages:import.needsName') } : {})}
          >
            {(control) => (
              <Input
                {...control}
                value={name}
                disabled={!file}
                onChange={(event) => setName(event.target.value)}
              />
            )}
          </FormField>

          <div>
            <Button
              variant="primary"
              onClick={() => void handleImport()}
              loading={importMutation.isPending}
              disabled={!file || name.trim() === ''}
            >
              {importMutation.isPending ? t('pages:import.importing') : t('pages:import.submit')}
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
}

/**
 * formatBytes states an archive's size in the units a person reads. Captures
 * run to several megabytes, and the size is the other half of "is this the file
 * I meant" — a 6 MB store survey and a 200 KB test capture are told apart by it.
 */
function formatBytes(bytes: number): string {
  const mb = bytes / 1024 / 1024;
  if (mb >= 1) {
    return `${mb.toFixed(1)} MB`;
  }
  return `${Math.max(1, Math.round(bytes / 1024))} KB`;
}

/**
 * isAirMagnet reads the vendor off the extension. It is what the operator
 * already chose in the file dialog, so asking them again would be asking twice.
 */
function isAirMagnet(file: File | undefined): boolean {
  return file !== undefined && /\.svd$/i.test(file.name);
}
