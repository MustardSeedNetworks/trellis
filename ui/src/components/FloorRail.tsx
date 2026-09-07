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
 */
export function FloorRail({ surveyId, floors }: { surveyId: string; floors: Floor[] }) {
  const { t } = useTranslation(['common', 'pages']);
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const [level, setLevel] = useState('');

  // The floor list, and the survey summary whose floor count is drawn from it.
  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: ['floors', surveyId] }),
      queryClient.invalidateQueries({ queryKey: ['surveys'] }),
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

  const trimmed = name.trim();
  const error = createMutation.error ?? activateMutation.error;

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
