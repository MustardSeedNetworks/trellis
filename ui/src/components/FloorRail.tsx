import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { Floor } from '@/gen/trellis/survey/v1/survey_pb';
import { surveyClient } from '@/lib/client';

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
 * Deleting one is two clicks, like deleting a survey: a floor holds the
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

  return (
    <section className="flex flex-col gap-3" aria-labelledby="floor-rail-title">
      <h3 id="floor-rail-title" className="kicker">
        {t('pages:surveys.floorsTitle')}
      </h3>

      <ul className="flex flex-col gap-2" data-testid="floor-rail">
        {floors.map((floor) => (
          <li
            key={floor.id}
            data-testid={`floor-row-${floor.id}`}
            className="flex flex-wrap items-center gap-3 rounded border border-hairline px-3 py-2"
          >
            <span className="text-sm text-text-primary">{floor.name}</span>
            <span className="text-sm text-text-secondary">
              {t('pages:surveys.floorLevel', { level: floor.level })}
            </span>
            <span className="text-sm text-text-secondary">
              {t('pages:surveys.floorSamples', { count: floor.sampleCount })}
            </span>
            {editing === floor.id ? (
              <>
                <label className="sr-only" htmlFor={`floor-rename-${floor.id}`}>
                  {t('pages:surveys.floorName')}
                </label>
                <input
                  id={`floor-rename-${floor.id}`}
                  value={editName}
                  onChange={(event) => setEditName(event.target.value)}
                  data-testid={`floor-rename-input-${floor.id}`}
                  className="rounded border border-hairline bg-surface-raised px-2 py-1 text-sm text-text-primary"
                />
                <label className="sr-only" htmlFor={`floor-relevel-${floor.id}`}>
                  {t('pages:surveys.floorLevelField')}
                </label>
                <input
                  id={`floor-relevel-${floor.id}`}
                  type="number"
                  value={editLevel}
                  onChange={(event) => setEditLevel(event.target.value)}
                  data-testid={`floor-relevel-input-${floor.id}`}
                  className="w-20 rounded border border-hairline bg-surface-raised px-2 py-1 text-sm text-text-primary"
                />
                <button
                  type="button"
                  onClick={() => setEditing(null)}
                  className="rounded border border-hairline px-3 py-1 text-sm text-text-primary hover:bg-surface-raised"
                  data-testid={`floor-rename-cancel-${floor.id}`}
                >
                  {t('pages:surveys.cancelRename')}
                </button>
                <button
                  type="button"
                  onClick={() =>
                    editName.trim() !== '' &&
                    renameMutation.mutate({
                      floorId: floor.id,
                      name: editName.trim(),
                      level: Number.parseInt(editLevel, 10) || 0,
                    })
                  }
                  disabled={renameMutation.isPending}
                  data-testid={`floor-rename-save-${floor.id}`}
                  className="rounded bg-brand-primary px-3 py-1 text-sm font-medium text-on-brand hover:bg-brand-accent disabled:opacity-50"
                >
                  {t('pages:surveys.saveRename')}
                </button>
              </>
            ) : (
              <button
                type="button"
                onClick={() => {
                  setEditing(floor.id);
                  setEditName(floor.name);
                  setEditLevel(String(floor.level));
                }}
                data-testid={`rename-floor-${floor.id}`}
                className="rounded border border-hairline px-3 py-1 text-sm text-text-primary hover:bg-surface-raised"
              >
                {t('pages:surveys.renameFloor')}
              </button>
            )}

            {deletable ? (
              confirmingDelete === floor.id ? (
                <>
                  <span className="text-sm text-text-secondary">
                    {t('pages:surveys.deleteFloorPrompt', { count: floor.sampleCount })}
                  </span>
                  <button
                    type="button"
                    onClick={() => setConfirmingDelete(null)}
                    data-testid={`delete-floor-cancel-${floor.id}`}
                    className="rounded border border-hairline px-3 py-1 text-sm text-text-primary hover:bg-surface-raised"
                  >
                    {t('pages:surveys.keepIt')}
                  </button>
                  <button
                    type="button"
                    onClick={() => deleteMutation.mutate(floor.id)}
                    disabled={deleteMutation.isPending}
                    data-testid={`delete-floor-confirm-${floor.id}`}
                    className="rounded border border-status-error px-3 py-1 text-sm text-status-error hover:bg-surface-raised disabled:opacity-50"
                  >
                    {t('pages:surveys.confirmDeleteFloor', { name: floor.name })}
                  </button>
                </>
              ) : (
                <button
                  type="button"
                  onClick={() => setConfirmingDelete(floor.id)}
                  data-testid={`delete-floor-${floor.id}`}
                  className="rounded border border-status-error px-3 py-1 text-sm text-status-error hover:bg-surface-raised"
                >
                  {t('pages:surveys.deleteFloor')}
                </button>
              )
            ) : null}

            {floor.isActive ? (
              <span className="kicker" data-testid={`floor-walking-${floor.id}`}>
                {t('pages:surveys.floorWalking')}
              </span>
            ) : (
              <button
                type="button"
                onClick={() => activateMutation.mutate(floor.id)}
                disabled={activateMutation.isPending}
                data-testid={`walk-floor-${floor.id}`}
                className="rounded border border-hairline px-3 py-1 text-sm text-text-primary hover:bg-surface-raised disabled:opacity-50"
              >
                {t('pages:surveys.walkThisFloor')}
              </button>
            )}
          </li>
        ))}
      </ul>

      <div className="flex flex-wrap items-end gap-3">
        <label className="flex flex-col gap-1 text-sm text-text-secondary">
          {t('pages:surveys.floorName')}
          <input
            value={name}
            onChange={(event) => setName(event.target.value)}
            data-testid="floor-name-input"
            className="rounded border border-hairline bg-surface-raised px-3 py-2 text-sm text-text-primary"
          />
        </label>
        <label className="flex flex-col gap-1 text-sm text-text-secondary">
          {t('pages:surveys.floorLevelField')}
          <input
            type="number"
            value={level}
            onChange={(event) => setLevel(event.target.value)}
            data-testid="floor-level-input"
            className="w-24 rounded border border-hairline bg-surface-raised px-3 py-2 text-sm text-text-primary"
          />
        </label>
        <button
          type="button"
          // An unnamed floor is refused by the handler too; stopping here makes
          // the refusal immediate instead of a round trip that says the same.
          onClick={() =>
            trimmed !== '' &&
            createMutation.mutate({ name: trimmed, level: Number.parseInt(level, 10) || 0 })
          }
          disabled={createMutation.isPending}
          data-testid="create-floor"
          className="rounded bg-brand-primary px-3 py-2 text-sm font-medium text-on-brand hover:bg-brand-accent disabled:opacity-50"
        >
          {t('pages:surveys.addFloor')}
        </button>
      </div>

      {error ? (
        <p className="text-sm text-status-error" data-testid="floor-rail-error">
          {String(error)}
        </p>
      ) : null}
    </section>
  );
}
