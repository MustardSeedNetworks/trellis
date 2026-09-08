import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { type ChangeEvent, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { PlanCalibrator } from '@/components/PlanCalibrator';
import { surveyClient } from '@/lib/client';
import { bytesToDataUrl } from '@/lib/format';

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
  nextLevel,
}: {
  surveyId: string;
  floorId: string;
  hasPlan: boolean;
  scaleM: number;
  // The storey the new floor gets when a refused plan is put on one of its
  // own: one above the highest the survey already has.
  nextLevel: number;
}) {
  const { t } = useTranslation(['common', 'pages']);
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [newFloorName, setNewFloorName] = useState('');

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
    mutationFn: (line: {
      from: { x: number; y: number };
      to: { x: number; y: number };
      metres: number;
    }) =>
      surveyClient.calibrateFloorPlan({
        surveyId,
        floorId,
        x1: line.from.x,
        y1: line.from.y,
        x2: line.to.x,
        y2: line.to.y,
        metres: line.metres,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['floors', surveyId] });
    },
  });

  // The plan a floor refused, put on a floor of its own. The bytes are the
  // ones already uploaded — react-query keeps a failed mutation's variables —
  // so the operator does not choose the same file twice.
  const newFloorMutation = useMutation({
    mutationFn: async (name: string) => {
      const image = uploadMutation.variables;
      if (image === undefined) {
        throw new Error('no plan to move');
      }
      const created = await surveyClient.createFloor({ surveyId, name, level: nextLevel });
      // Not `?? ''`: an empty floor_id means the ACTIVE floor, which is the one
      // that just refused this plan, so a fallback would quietly re-run the
      // failure and report it as the new floor's.
      if (created.floor === undefined) {
        throw new Error('the new floor came back without an id');
      }
      const buffer = await image.arrayBuffer();
      return surveyClient.setFloorPlan({
        surveyId,
        floorId: created.floor.id,
        image: new Uint8Array(buffer),
      });
    },
    // The upload is reset only once the plan is on the new floor. Resetting it
    // on failure too would clear the refusal that draws this form, taking the
    // bytes with it — the operator would have to choose the same file again to
    // get the offer back, which is what this route exists to avoid.
    onSuccess: () => {
      setNewFloorName('');
      uploadMutation.reset();
    },
    onSettled: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['floors', surveyId] }),
        queryClient.invalidateQueries({ queryKey: ['surveys'] }),
      ]);
    },
  });

  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const chosen = event.target.files?.[0];
    if (chosen) {
      uploadMutation.mutate(chosen);
    }
  }

  const width = planQuery.data?.width ?? 0;
  // The plan the calibrator draws on. It is the same image the capture surface
  // shows, encoded the same way: two points mean nothing without the picture
  // they were marked on.
  const planImage =
    planQuery.data && planQuery.data.image.length > 0
      ? {
          width: planQuery.data.width,
          height: planQuery.data.height,
          imageUrl: bytesToDataUrl(planQuery.data.image, 'image/png'),
        }
      : undefined;
  // The new-floor route's failure comes first: the strand refusal is still
  // standing by design while the form is offered, so leaving it ahead would
  // report the old refusal instead of what just went wrong.
  const error = newFloorMutation.error ?? uploadMutation.error ?? calibrateMutation.error;
  // The one refusal an operator can do something about, and the one worth
  // explaining rather than only reporting: a plan of different dimensions over
  // a floor that already holds measurements is refused, because every stored
  // point is a pixel coordinate on the plan it was walked against.
  const stranded =
    uploadMutation.error !== null && /strand the measurements/i.test(String(uploadMutation.error));

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
      </div>

      {/* Offered only once there is a plan: the two points a calibration is
          expressed in are points on it. */}
      {planImage ? (
        <PlanCalibrator
          plan={planImage}
          pending={calibrateMutation.isPending}
          onApply={(line) => calibrateMutation.mutate(line)}
        />
      ) : null}

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

      {stranded ? (
        <>
          <p className="text-sm text-text-secondary" data-testid="floor-plan-stranded-hint">
            {t('pages:surveys.planWouldStrandHint')}
          </p>
          <div className="flex flex-wrap items-end gap-3">
            <label className="flex flex-col gap-1 text-sm text-text-secondary">
              {t('pages:surveys.newFloorName')}
              <input
                value={newFloorName}
                onChange={(event) => setNewFloorName(event.target.value)}
                data-testid="strand-new-floor-name"
                className="rounded border border-hairline bg-surface-raised px-3 py-2 text-sm text-text-primary"
              />
            </label>
            <button
              type="button"
              onClick={() =>
                newFloorName.trim() !== '' && newFloorMutation.mutate(newFloorName.trim())
              }
              disabled={newFloorMutation.isPending}
              data-testid="strand-new-floor"
              className="rounded border border-hairline px-3 py-2 text-sm text-text-primary hover:bg-surface-raised disabled:opacity-50"
            >
              {t('pages:surveys.planAsNewFloor', { level: nextLevel })}
            </button>
          </div>
        </>
      ) : null}
    </section>
  );
}
