import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { Floor } from '@/gen/trellis/survey/v1/survey_pb';
import { surveyClient } from '@/lib/client';
import { useTableLabels } from '@/lib/useTableLabels';
import { Alert } from '@/shared/Alert';
import { Button } from '@/shared/Button';
import { ConfirmDialog } from '@/shared/ConfirmDialog';
import { DataTable, type DataTableColumn } from '@/shared/DataTable';
import { FormField } from '@/shared/FormField';
import { Input } from '@/shared/Input';
import { StatusPill } from '@/shared/StatusPill';

/**
 * FloorRail — the storeys of a survey, and which one the walk is on.
 *
 * A survey opened here starts with one floor and, until now, could only gain
 * others from an import: the domain has had AddFloor and SetActiveFloor since
 * multi-floor surveys landed, and neither was on the wire (#342). So a
 * surveyor could browse an imported building but never lay one out and walk
 * it.
 *
 * Creating a floor deliberately does not switch to it. The floors of a
 * building are entered together and walked one at a time; moving the walk on
 * every add would take it off the floor being measured.
 *
 * Deleting one asks first, like deleting a survey: a floor holds the
 * measurements walked on it and they go with it.
 */
export function FloorRail({ surveyId, floors }: { surveyId: string; floors: Floor[] }) {
  const { t } = useTranslation(['common', 'pages']);
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const [level, setLevel] = useState('');
  // Which row is being renamed, and which is armed for deletion — by floor ID
  // rather than a boolean, so arming one row does not arm every row.
  const [editing, setEditing] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [editLevel, setEditLevel] = useState('');
  const [confirmingDelete, setConfirmingDelete] = useState<string | null>(null);

  // The floor list, the survey summary whose floor count is drawn from it, and
  // everything drawn from the measurements: a deleted floor takes its readings
  // off the capture surface AND off Coverage, which keys its heatmap and its
  // findings separately and would otherwise serve the deleted floor's numbers.
  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: ['floors', surveyId] }),
      queryClient.invalidateQueries({ queryKey: ['surveys'] }),
      queryClient.invalidateQueries({ queryKey: ['samples', surveyId] }),
      queryClient.invalidateQueries({ queryKey: ['heatmap', surveyId] }),
      queryClient.invalidateQueries({ queryKey: ['coverage', surveyId] }),
    ]);

  const createMutation = useMutation({
    mutationFn: (floor: { name: string; level: number }) =>
      surveyClient.createFloor({ surveyId, name: floor.name, level: floor.level }),
    onSuccess: async () => {
      setName('');
      setLevel('');
      await refresh();
    },
  });

  const activateMutation = useMutation({
    mutationFn: (floorId: string) => surveyClient.setActiveFloor({ surveyId, floorId }),
    onSuccess: refresh,
  });

  const renameMutation = useMutation({
    mutationFn: (floor: { floorId: string; name: string; level: number }) =>
      surveyClient.updateFloor({ surveyId, ...floor }),
    // onSettled, not onSuccess: a refused rename still leaves the rail showing
    // whatever the server holds, which is the thing worth drawing.
    onSettled: async () => {
      setEditing(null);
      await refresh();
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (floorId: string) => surveyClient.deleteFloor({ surveyId, floorId }),
    onSettled: async () => {
      setConfirmingDelete(null);
      await refresh();
    },
  });

  const trimmed = name.trim();
  const error =
    createMutation.error ?? activateMutation.error ?? renameMutation.error ?? deleteMutation.error;
  // The last floor cannot be deleted — the handler refuses it, and a button
  // that can only fail is worse than no button.
  const deletable = floors.length > 1;
  // A field points at the error only while the error shown is its own save's.
  const createFailed = error !== null && error === createMutation.error;
  const renameFailed = error !== null && error === renameMutation.error;

  const deleting = floors.find((floor) => floor.id === confirmingDelete);
  const tableLabels = useTableLabels<Floor>();

  // A row being renamed swaps its name and level cells for fields, so the
  // rename happens where the floor is listed rather than in a second form.
  const columns: DataTableColumn<Floor>[] = [
    {
      key: 'name',
      header: t('pages:surveys.floorName'),
      cell: (floor) =>
        editing === floor.id ? (
          <Input
            value={editName}
            onChange={(event) => setEditName(event.target.value)}
            aria-label={t('pages:surveys.floorName')}
            aria-describedby={renameFailed ? 'floor-rail-error' : undefined}
            data-testid={`floor-rename-input-${floor.id}`}
            className="min-h-8 w-40 py-1"
          />
        ) : (
          <span className="inline-flex flex-wrap items-center gap-2">
            <span className="font-medium text-text-primary">{floor.name}</span>
            {floor.isActive ? (
              <StatusPill tone="info" label={t('pages:surveys.floorWalking')} />
            ) : null}
          </span>
        ),
    },
    {
      key: 'level',
      header: t('pages:surveys.floorLevelField'),
      cell: (floor) =>
        editing === floor.id ? (
          <Input
            type="number"
            value={editLevel}
            onChange={(event) => setEditLevel(event.target.value)}
            aria-label={t('pages:surveys.floorLevelField')}
            aria-describedby={renameFailed ? 'floor-rail-error' : undefined}
            data-testid={`floor-relevel-input-${floor.id}`}
            className="min-h-8 w-20 py-1"
          />
        ) : (
          t('pages:surveys.floorLevel', { level: floor.level })
        ),
    },
    {
      key: 'samples',
      header: t('common:labels.samples'),
      cell: (floor) => t('pages:surveys.floorSamples', { count: floor.sampleCount }),
    },
  ];

  return (
    <section className="flex flex-col gap-3" aria-labelledby="floor-rail-title">
      <h3 id="floor-rail-title" className="kicker">
        {t('pages:surveys.floorsTitle')}
      </h3>

      <DataTable
        data-testid="floor-rail"
        rows={floors}
        columns={columns}
        getRowKey={(floor) => floor.id}
        getRowProps={(floor) => ({ 'data-testid': `floor-row-${floor.id}` })}
        caption={t('pages:surveys.floorsTitle')}
        labels={tableLabels}
        empty={null}
        rowActions={(floor) =>
          editing === floor.id ? (
            <>
              <Button
                size="sm"
                onClick={() => setEditing(null)}
                data-testid={`floor-rename-cancel-${floor.id}`}
              >
                {t('pages:surveys.cancelRename')}
              </Button>
              <Button
                size="sm"
                onClick={() =>
                  editName.trim() !== '' &&
                  renameMutation.mutate({
                    floorId: floor.id,
                    name: editName.trim(),
                    level: Number.parseInt(editLevel, 10) || 0,
                  })
                }
                loading={renameMutation.isPending}
                data-testid={`floor-rename-save-${floor.id}`}
              >
                {t('pages:surveys.saveRename')}
              </Button>
            </>
          ) : (
            <>
              <Button
                size="sm"
                onClick={() => {
                  setEditing(floor.id);
                  setEditName(floor.name);
                  setEditLevel(String(floor.level));
                }}
                data-testid={`rename-floor-${floor.id}`}
              >
                {t('pages:surveys.renameFloor')}
              </Button>
              {floor.isActive ? null : (
                <Button
                  size="sm"
                  onClick={() => activateMutation.mutate(floor.id)}
                  disabled={activateMutation.isPending}
                  data-testid={`walk-floor-${floor.id}`}
                >
                  {t('pages:surveys.walkThisFloor')}
                </Button>
              )}
              {deletable ? (
                <Button
                  size="sm"
                  variant="destructive"
                  onClick={() => setConfirmingDelete(floor.id)}
                  data-testid={`delete-floor-${floor.id}`}
                >
                  {t('pages:surveys.deleteFloor')}
                </Button>
              ) : null}
            </>
          )
        }
      />

      <div className="flex flex-wrap items-end gap-3">
        <FormField label={t('pages:surveys.floorName')} className="sm:w-56">
          {(control) => (
            <Input
              {...control}
              value={name}
              onChange={(event) => setName(event.target.value)}
              aria-describedby={createFailed ? 'floor-rail-error' : undefined}
              data-testid="floor-name-input"
            />
          )}
        </FormField>
        <FormField label={t('pages:surveys.floorLevelField')} className="sm:w-28">
          {(control) => (
            <Input
              {...control}
              type="number"
              value={level}
              onChange={(event) => setLevel(event.target.value)}
              aria-describedby={createFailed ? 'floor-rail-error' : undefined}
              data-testid="floor-level-input"
            />
          )}
        </FormField>
        <Button
          // An unnamed floor is refused by the handler too; stopping here makes
          // the refusal immediate instead of a round trip that says the same.
          onClick={() =>
            trimmed !== '' &&
            createMutation.mutate({ name: trimmed, level: Number.parseInt(level, 10) || 0 })
          }
          loading={createMutation.isPending}
          data-testid="create-floor"
        >
          {t('pages:surveys.addFloor')}
        </Button>
      </div>

      {error ? (
        <div id="floor-rail-error" data-testid="floor-rail-error">
          <Alert tone="error">{String(error)}</Alert>
        </div>
      ) : null}

      <ConfirmDialog
        open={deleting !== undefined}
        title={t('pages:surveys.deleteFloor')}
        message={t('pages:surveys.deleteFloorPrompt', { count: deleting?.sampleCount ?? 0 })}
        confirmLabel={t('pages:surveys.confirmDeleteFloor', { name: deleting?.name ?? '' })}
        cancelLabel={t('pages:surveys.keepIt')}
        closeLabel={t('common:accessibility.close')}
        onConfirm={() => deleting && deleteMutation.mutate(deleting.id)}
        onCancel={() => setConfirmingDelete(null)}
        confirming={deleteMutation.isPending}
      />
    </section>
  );
}
