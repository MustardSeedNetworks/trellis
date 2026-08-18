import { useMutation, useQueryClient } from '@tanstack/react-query';
import { type ChangeEvent, useRef, useState } from 'react';
import { surveyClient } from '@/lib/client';

export function ImportSurvey() {
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [error, setError] = useState<string | undefined>();

  const importMutation = useMutation({
    mutationFn: async ({ name, ampData }: { name: string; ampData: Uint8Array }) =>
      surveyClient.importAirMapper({ name, ampData }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['surveys'] });
    },
    onError: (mutationError: unknown) => {
      setError(mutationError instanceof Error ? mutationError.message : 'Import failed');
    },
  });

  async function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }

    const name = window.prompt('Survey name', file.name.replace(/\.amp$/i, ''));
    if (!name) {
      event.target.value = '';
      return;
    }

    setError(undefined);
    const buffer = await file.arrayBuffer();
    importMutation.mutate({ name, ampData: new Uint8Array(buffer) });
    event.target.value = '';
  }

  return (
    <div className="border-t border-hairline p-4">
      <input
        ref={fileInputRef}
        type="file"
        accept=".amp"
        onChange={handleFileChange}
        className="hidden"
        data-testid="amp-file-input"
      />
      <button
        type="button"
        onClick={() => fileInputRef.current?.click()}
        disabled={importMutation.isPending}
        className="w-full rounded bg-brand-primary px-3 py-2 text-sm font-medium text-on-brand hover:bg-brand-accent disabled:opacity-50"
      >
        {importMutation.isPending ? 'Importing…' : 'Import AirMapper (.amp)'}
      </button>
      {error && <p className="mt-2 text-xs text-status-error">{error}</p>}
    </div>
  );
}
