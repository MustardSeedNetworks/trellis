import { useMutation, useQueryClient } from '@tanstack/react-query';
import { type ChangeEvent, useRef, useState } from 'react';
import { surveyClient } from '@/lib/client';
import { type RollupState, StatusRollup } from '@/ui/StatusRollup';

/**
 * Import — AirMapper ingest.
 *
 * The capability shipped before the page did, as a button in the Surveys rail
 * that collected the survey name with window.prompt. A prompt is the wrong
 * shape for this: it demands the name before the operator has seen anything
 * about the file, cancelling it discards the chosen file, and it cannot be
 * driven by a test. The page keeps the file and the name on screen together
 * until the operator commits.
 *
 * The rollup leads for the same reason it does on Surveys — an import that
 * fails silently and leaves a calm empty form reads as "nothing happened".
 */
export function ImportPage() {
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | undefined>();
  const [name, setName] = useState('');

  const importMutation = useMutation({
    mutationFn: async ({ surveyName, ampData }: { surveyName: string; ampData: Uint8Array }) =>
      surveyClient.importAirMapper({ name: surveyName, ampData }),
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
    setName(chosen.name.replace(/\.amp$/i, ''));
  }

  async function handleImport() {
    if (!file || name.trim() === '') {
      return;
    }
    const buffer = await file.arrayBuffer();
    importMutation.mutate({ surveyName: name.trim(), ampData: new Uint8Array(buffer) });
  }

  const rollup = describeImport({
    file,
    name,
    pending: importMutation.isPending,
    error: importMutation.error,
    imported: importMutation.data?.survey,
  });

  return (
    <div className="flex flex-1 flex-col gap-6 overflow-y-auto p-6">
      <StatusRollup
        state={rollup.state}
        headline={rollup.headline}
        body={rollup.body}
        figures={rollup.figures}
      />

      <div className="panel flex max-w-xl flex-col gap-4 p-6">
        <input
          ref={fileInputRef}
          type="file"
          accept=".amp"
          onChange={handleFileChange}
          className="hidden"
          data-testid="amp-file-input"
        />

        <div className="flex flex-col gap-2">
          <span className="kicker">AirMapper archive</span>
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            className="w-fit rounded border border-hairline px-3 py-2 text-sm text-text-primary hover:bg-surface-raised"
          >
            {file ? 'Choose a different file' : 'Choose .amp file'}
          </button>
        </div>

        <label className="flex flex-col gap-2 text-sm" htmlFor="survey-name">
          <span className="kicker">Survey name</span>
          <input
            id="survey-name"
            type="text"
            value={name}
            disabled={!file}
            onChange={(event) => setName(event.target.value)}
            className="rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary disabled:opacity-50"
          />
        </label>

        <button
          type="button"
          onClick={() => void handleImport()}
          disabled={!file || name.trim() === '' || importMutation.isPending}
          className="w-fit rounded bg-brand-primary px-3 py-2 text-sm font-medium text-on-brand hover:bg-brand-accent disabled:opacity-50"
        >
          {importMutation.isPending ? 'Importing…' : 'Import survey'}
        </button>
      </div>
    </div>
  );
}

interface ImportState {
  file: File | undefined;
  name: string;
  pending: boolean;
  error: unknown;
  /** The stored survey the service created, once it has. */
  imported: { id: string; name: string; sampleCount: number; floorCount: number } | undefined;
}

interface ImportRollup {
  state: RollupState;
  headline: string;
  body?: string;
  /** Only a completed import has anything measured to show. */
  figures?: { label: string; value: string }[];
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
 * describeImport turns the form's state into the rollup's reading.
 *
 * Nothing chosen is `unknown` rather than `ok`: the page has no result to
 * report, and a calm green rollup over an untouched form would claim one.
 */
function describeImport({ file, name, pending, error, imported }: ImportState): ImportRollup {
  // The chosen file is named in the body rather than carried as a figure.
  // StatusRollup withholds figures in the unknown state on purpose — a rollup
  // that prints values when it has no reading claims one — and "ready to
  // import" is genuinely unknown, since nothing has been measured yet. The
  // filename is not a measurement, so it belongs in the sentence, where it is
  // readable at exactly the moment the user is deciding whether to commit.
  const chosen = file ? `${file.name}, ${formatBytes(file.size)}.` : '';

  if (error) {
    return {
      state: 'crit',
      headline: 'Import failed',
      body: [chosen, error instanceof Error ? error.message : String(error)]
        .filter(Boolean)
        .join(' '),
    };
  }
  if (pending) {
    return { state: 'unknown', headline: 'Importing the archive', body: chosen };
  }
  if (imported) {
    // What the archive actually yielded is the useful readout — an import
    // that stored zero samples "succeeded" and is still worth seeing.
    return {
      state: 'ok',
      headline: `Imported ${imported.name}`,
      body: `Stored as ${imported.id}. It is now listed under Surveys.`,
      figures: [
        { label: 'Samples', value: String(imported.sampleCount) },
        { label: 'Floors', value: String(imported.floorCount) },
      ],
    };
  }
  if (!file) {
    return {
      state: 'unknown',
      headline: 'No survey file chosen',
      body: 'Choose an AirMapper .amp archive to import it as a survey.',
    };
  }
  if (name.trim() === '') {
    return {
      state: 'warn',
      headline: 'The survey needs a name',
      body: `${chosen} Give the survey a name to import it.`,
    };
  }
  return { state: 'unknown', headline: 'Ready to import', body: chosen };
}
