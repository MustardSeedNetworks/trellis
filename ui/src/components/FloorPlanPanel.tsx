import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { type ChangeEvent, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { surveyClient } from '@/lib/client';

/**
 * FloorPlanPanel — the plan a survey's points are drawn on, and what one of its
 * pixels is worth.
 *
 * Until now a plan could only arrive inside an AirMapper archive, so a survey
 * walked with this product had nothing to draw its points on: the capture
 * surface was a blank canvas and every position was a pixel in a space with no
 * relationship to the building.
 *
 * Calibration is two points and a real distance because that is the only method
 * that needs nothing but the operator and a tape measure. A plan exported at an
 * arbitrary resolution has no scale of its own, and the analysis reports a dead
 * zone's radius in metres — a figure that is meaningless until somebody says
 * what one line on the plan is.
 */
export function FloorPlanPanel({
  surveyId,
  floorId,
  hasPlan,
  scaleM,
}: {
  surveyId: string;
  floorId: string;
  hasPlan: boolean;
  scaleM: number;
}) {
  const { t } = useTranslation(['common', 'pages']);
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [metres, setMetres] = useState('10');

  const planQuery = useQuery({
    queryKey: ['floor-plan', surveyId, floorId],
    queryFn: () => surveyClient.getFloorPlanImage({ surveyId, floorId }),
    enabled: hasPlan,
  });

  const uploadMutation = useMutation({
    mutationFn: async (file: File) => {
      const buffer = await file.arrayBuffer();
      return surveyClient.setFloorPlan({ surveyId, floorId, image: new Uint8Array(buffer) });
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['floors', surveyId] }),
        queryClient.invalidateQueries({ queryKey: ['floor-plan', surveyId, floorId] }),
        queryClient.invalidateQueries({ queryKey: ['surveys'] }),
      ]);
    },
  });

  const calibrateMutation = useMutation({
    mutationFn: (line: { metres: number }) =>
      surveyClient.calibrateFloorPlan({
        surveyId,
        floorId,
        // The reference line runs across the plan's own width, which is the
        // longest measurable thing on it and the one an operator can most
        // easily pace or read off a drawing's own dimension line.
        x1: 0,
        y1: 0,
        x2: planQuery.data?.width ?? 0,
        y2: 0,
        metres: line.metres,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['floors', surveyId] });
    },
  });

  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const chosen = event.target.files?.[0];
    if (chosen) {
      uploadMutation.mutate(chosen);
    }
  }

  const width = planQuery.data?.width ?? 0;
  const error = uploadMutation.error ?? calibrateMutation.error;

  return (
    <section className="flex flex-col gap-3" aria-labelledby="floor-plan-title">
      <h3 id="floor-plan-title" className="kicker">
        {t('pages:surveys.floorPlanTitle')}
      </h3>

      <input
        ref={fileInputRef}
        type="file"
        accept="image/png,image/jpeg"
        onChange={handleFileChange}
        className="hidden"
        data-testid="floor-plan-input"
      />
      <div className="flex flex-wrap items-end gap-3">
        <button
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={uploadMutation.isPending}
          data-testid="upload-floor-plan"
          className="rounded border border-hairline px-3 py-2 text-sm text-text-primary hover:bg-surface-raised disabled:opacity-50"
        >
          {hasPlan ? t('pages:surveys.replacePlan') : t('pages:surveys.uploadPlan')}
        </button>

        {/* Offered only once there is a plan: the two points a calibration is
            expressed in are points on it. */}
        {hasPlan ? (
          <>
            <label className="flex flex-col gap-1 text-sm" htmlFor="plan-width-metres">
              <span className="kicker">{t('pages:surveys.planWidthMetres')}</span>
              <input
                id="plan-width-metres"
                type="number"
                min={0.1}
                step={0.1}
                value={metres}
                onChange={(event) => setMetres(event.target.value)}
                className="figure w-28 rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary"
                data-testid="plan-width-metres"
              />
            </label>
            <button
              type="button"
              onClick={() => calibrateMutation.mutate({ metres: Number(metres) })}
              disabled={calibrateMutation.isPending || !(Number(metres) > 0) || width === 0}
              data-testid="calibrate-floor-plan"
              className="rounded border border-hairline px-3 py-2 text-sm text-text-primary hover:bg-surface-raised disabled:opacity-50"
            >
              {t('pages:surveys.calibrate')}
            </button>
          </>
        ) : null}
      </div>

      <p
        className={`text-sm ${error ? 'text-status-error' : 'text-text-secondary'}`}
        data-testid="floor-plan-status"
      >
        {error
          ? String(error)
          : !hasPlan
            ? t('pages:surveys.noPlan')
            : scaleM > 0
              ? // The width the plan is across is the figure a person can check
                // against a building; the metres-per-pixel alone is not. It is
                // withheld until the plan's dimensions have arrived rather than
                // printed as 0.0 m in the meantime.
                width > 0
                ? t('pages:surveys.calibrated', {
                    scale: scaleM.toFixed(3),
                    across: (scaleM * width).toFixed(1),
                  })
                : t('pages:surveys.calibratedScaleOnly', { scale: scaleM.toFixed(3) })
              : t('pages:surveys.uncalibrated')}
      </p>
    </section>
  );
}
