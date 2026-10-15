import { Code, ConnectError } from '@connectrpc/connect';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { type FormEvent, type ReactNode, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { HeatmapLegend } from '@/components/HeatmapLegend';
import { HeatmapSurface } from '@/components/HeatmapSurface';
import { apLabel, PlacementLayer } from '@/components/PlacementLayer';
import type {
  APCalibration,
  APPlacement,
  GetHeatmapResponse,
} from '@/gen/trellis/survey/v1/survey_pb';
import { surveyClient } from '@/lib/client';
import { bytesToDataUrl } from '@/lib/format';
import { BANDS, type Band, type PlanPoint, placedOnBand, TX_POWER } from '@/lib/placement';

interface PredictionViewProps {
  surveyId: string;
  floorId: string;
  band: Band;
  heatmap: GetHeatmapResponse | undefined;
  heatmapError: unknown;
  heatmapLoading: boolean;
}

/**
 * Coverage's predicted layer: place APs on the floor and see the signal they
 * would give.
 *
 * The prediction is drawn by the service from the floor's placements, so this
 * view only edits placements and re-reads the map. Markers sit on the
 * predicted map where there is one, and on the bare plan where the service
 * refuses to predict — a floor with nothing placed has to be clickable, or
 * the first AP could never go down.
 */
export function PredictionView({
  surveyId,
  floorId,
  band,
  heatmap,
  heatmapError,
  heatmapLoading,
}: PredictionViewProps) {
  const { t } = useTranslation(['common', 'pages']);
  const queryClient = useQueryClient();
  const [selectedId, setSelectedId] = useState<string>();

  const planQuery = useQuery({
    queryKey: ['floor-plan', surveyId, floorId],
    queryFn: () => surveyClient.getFloorPlanImage({ surveyId, floorId }),
  });
  const placementsKey = ['placements', surveyId, floorId];
  const placementsQuery = useQuery({
    queryKey: placementsKey,
    queryFn: () => surveyClient.listAPPlacements({ surveyId, floorId }),
  });
  const placements = placementsQuery.data?.placements ?? [];

  /* Every edit changes the map, so each one re-reads both. */
  async function refresh() {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: placementsKey }),
      queryClient.invalidateQueries({ queryKey: ['heatmap', surveyId, 'predicted_rssi'] }),
    ]);
  }

  const addMutation = useMutation({
    /* A hand-placed AP is on the band being looked at: placed with no band it
       would radiate on all three, which is not what clicking this map meant. */
    mutationFn: (point: PlanPoint) =>
      surveyClient.createAPPlacement({
        surveyId,
        floorId,
        placement: { x: point.x, y: point.y, label: '', band },
      }),
    onSuccess: async (reply) => {
      setSelectedId(reply.placement?.id);
      await refresh();
    },
  });
  const editMutation = useMutation({
    mutationFn: ({ ap, edit }: { ap: APPlacement; edit: Partial<PlacementEdit> }) =>
      surveyClient.updateAPPlacement({
        surveyId,
        floorId,
        placementId: ap.id,
        placement: { ...editOf(ap), ...edit },
      }),
    onSuccess: refresh,
  });
  const deleteMutation = useMutation({
    mutationFn: (id: string) =>
      surveyClient.deleteAPPlacement({ surveyId, floorId, placementId: id }),
    onSuccess: async () => {
      setSelectedId(undefined);
      await refresh();
    },
  });
  const failed = addMutation.error ?? editMutation.error ?? deleteMutation.error;

  const plan = planQuery.data;
  const layer =
    plan && plan.width > 0 ? (
      <PlacementLayer
        placements={placements}
        width={plan.width}
        height={plan.height}
        selectedId={selectedId}
        onSelect={setSelectedId}
        onAdd={(point) => addMutation.mutate(point)}
        onMove={(id, point) => {
          const ap = placements.find((candidate) => candidate.id === id);
          if (ap) {
            editMutation.mutate({ ap, edit: point });
          }
        }}
      />
    ) : null;

  const selectedIndex = placements.findIndex((ap) => ap.id === selectedId);
  const selected = placements[selectedIndex];
  const calibrations = heatmapError ? [] : (heatmap?.calibrations ?? []);

  return (
    <>
      <section className="panel flex flex-col gap-4 p-5 xl:min-h-0 xl:self-stretch">
        {renderPredictionSurface()}
        {heatmap && !heatmapError ? (
          <div className="sticky bottom-0 -mx-5 -mb-5 mt-auto bg-surface-raised px-5 pb-5 pt-3">
            <HeatmapLegend stops={heatmap.legend} unit="dBm" />
          </div>
        ) : null}
      </section>

      <aside className="panel flex flex-col gap-4 p-5" data-testid="placement-panel">
        <div>
          <p className="kicker text-text-muted">{t('pages:coverage.plannedTitle')}</p>
          <p className="mt-2 text-sm text-text-secondary">{t('pages:coverage.plannedHint')}</p>
        </div>
        {failed ? (
          <p role="alert" className="text-sm text-status-error" data-testid="placement-error">
            {t('pages:coverage.placementFailed', { error: ConnectError.from(failed).rawMessage })}
          </p>
        ) : null}
        {placements.length > 0 ? (
          <ul className="flex flex-col gap-1" data-testid="placement-list">
            {placements.map((ap, index) => (
              <li key={ap.id}>
                <button
                  type="button"
                  onClick={() => setSelectedId(ap.id)}
                  aria-pressed={ap.id === selectedId}
                  className={`flex w-full flex-wrap justify-between gap-2 rounded-[9px] px-3 py-2 text-left text-sm ${
                    ap.id === selectedId
                      ? 'bg-surface-hover text-text-primary'
                      : 'text-text-secondary hover:bg-surface-hover'
                  }`}
                >
                  <span className="font-bold">{apLabel(ap, index, t)}</span>
                  <span className="figure text-xs">{describeRadio(ap, t)}</span>
                </button>
              </li>
            ))}
          </ul>
        ) : null}
        {selected ? (
          <PlacementEditor
            key={selected.id}
            ap={selected}
            name={apLabel(selected, selectedIndex, t)}
            calibration={calibrations.find((entry) => entry.placementId === selected.id)}
            pending={editMutation.isPending || deleteMutation.isPending}
            onApply={(edit) => editMutation.mutate({ ap: selected, edit })}
            onDelete={() => deleteMutation.mutate(selected.id)}
          />
        ) : null}
      </aside>
    </>
  );

  function renderPredictionSurface() {
    if (planQuery.isLoading || placementsQuery.isLoading) {
      return <SurfaceMessage text={t('pages:coverage.loadingPlan')} />;
    }
    if (planQuery.error || placementsQuery.error) {
      return (
        <SurfaceMessage
          error
          text={t('pages:coverage.surveyServiceSilent', {
            error: String(planQuery.error ?? placementsQuery.error),
          })}
        />
      );
    }
    if (!plan || plan.width === 0) {
      return <SurfaceMessage text={t('pages:coverage.noPlanToPlace')} />;
    }
    /* Refused rather than drawn empty: nothing placed on this band, or a plan
       with no scale to turn pixels into metres. Either way the plan stays on
       screen so the operator can fix it from here. */
    if (heatmapError != null && ConnectError.from(heatmapError).code === Code.FailedPrecondition) {
      const text = placedOnBand(placements, band)
        ? t('pages:coverage.predictionRefused', {
            reason: ConnectError.from(heatmapError).rawMessage,
          })
        : t('pages:coverage.noPlacements', { band });
      return (
        <>
          <SurfaceMessage text={text} />
          <PlanWithLayer imageUrl={bytesToDataUrl(plan.image, 'image/png')} layer={layer} />
        </>
      );
    }
    if (heatmapError) {
      return (
        <SurfaceMessage
          error
          text={t('pages:coverage.heatmapFailed', { error: String(heatmapError) })}
        />
      );
    }
    if (heatmapLoading || !heatmap) {
      return <SurfaceMessage text={t('pages:coverage.renderingHeatmap')} />;
    }
    return (
      <HeatmapSurface
        png={heatmap.png}
        width={heatmap.width}
        height={heatmap.height}
        grid={heatmap.grid}
        gridCols={heatmap.gridCols}
        gridRows={heatmap.gridRows}
        cellSize={heatmap.cellSize}
        unit="dBm"
        metric={heatmap.metric}
        overlay={layer}
      />
    );
  }
}

type PlacementEdit = { x: number; y: number; label: string; band: string; txPowerDbm?: number };

/** A placement as the edit the service takes, so a change to one field keeps the rest. */
function editOf(ap: APPlacement): PlacementEdit {
  return { x: ap.x, y: ap.y, label: ap.label, band: ap.band, txPowerDbm: ap.txPowerDbm };
}

function describeRadio(ap: APPlacement, t: TFunction<['common', 'pages']>) {
  const band = ap.band === '' ? t('pages:coverage.everyBand') : ap.band;
  return ap.txPowerDbm === undefined
    ? t('pages:coverage.radioAssumed', { band, power: TX_POWER.assumed })
    : t('pages:coverage.radioStated', { band, power: ap.txPowerDbm });
}

function SurfaceMessage({ text, error = false }: { text: string; error?: boolean }) {
  return (
    <p
      className={`text-sm ${error ? 'text-status-error' : 'text-text-muted'}`}
      data-testid="surface-message"
    >
      {text}
    </p>
  );
}

function PlanWithLayer({ imageUrl, layer }: { imageUrl: string; layer: ReactNode }) {
  const { t } = useTranslation(['common', 'pages']);
  return (
    <div className="max-h-[70vh] overflow-auto rounded-[12px] border border-hairline">
      <div className="relative">
        <img
          src={imageUrl}
          alt={t('pages:coverage.planAlt')}
          className="block w-full"
          data-testid="placement-plan"
        />
        {layer}
      </div>
    </div>
  );
}

interface PlacementEditorProps {
  ap: APPlacement;
  name: string;
  calibration: APCalibration | undefined;
  pending: boolean;
  onApply: (edit: Partial<PlacementEdit>) => void;
  onDelete: () => void;
}

/** The selected AP's radio, and how the prediction used it. */
function PlacementEditor({
  ap,
  name,
  calibration,
  pending,
  onApply,
  onDelete,
}: PlacementEditorProps) {
  const { t } = useTranslation(['common', 'pages']);
  const [band, setBand] = useState(ap.band);
  const [power, setPower] = useState(ap.txPowerDbm === undefined ? '' : String(ap.txPowerDbm));
  /* Empty is a real answer — "not stated", drawn at the assumed power — and
     distinct from 0 dBm, so it is sent as absent rather than as zero. */
  const parsedPower = power.trim() === '' ? undefined : Number(power);
  const powerValid =
    parsedPower === undefined ||
    (Number.isFinite(parsedPower) && parsedPower >= TX_POWER.min && parsedPower <= TX_POWER.max);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (powerValid) {
      onApply({ band, txPowerDbm: parsedPower });
    }
  }

  return (
    <form
      onSubmit={submit}
      className="flex flex-col gap-3 border-t border-hairline pt-4"
      data-testid="placement-editor"
    >
      <h2 className="text-base font-extrabold tracking-[-0.02em] text-text-primary">{name}</h2>
      <label className="flex flex-col gap-1 text-sm" htmlFor="placement-band">
        <span className="kicker">{t('pages:coverage.band')}</span>
        <select
          id="placement-band"
          value={band}
          onChange={(event) => setBand(event.target.value)}
          className="rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary"
          data-testid="placement-band"
        >
          <option value="">{t('pages:coverage.everyBand')}</option>
          {BANDS.map((option) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </select>
      </label>
      <label className="flex flex-col gap-1 text-sm" htmlFor="placement-power">
        <span className="kicker">{t('pages:coverage.txPower')}</span>
        <input
          id="placement-power"
          type="number"
          min={TX_POWER.min}
          max={TX_POWER.max}
          step={1}
          value={power}
          placeholder={t('pages:coverage.txPowerAssumed', { power: TX_POWER.assumed })}
          onChange={(event) => setPower(event.target.value)}
          aria-invalid={!powerValid}
          className="figure w-full rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary"
          data-testid="placement-power"
        />
      </label>
      <p className="text-sm text-text-secondary" data-testid="placement-calibration">
        {describeCalibration(calibration, t)}
      </p>
      <div className="flex flex-wrap gap-2">
        <button
          type="submit"
          disabled={pending || !powerValid}
          className="rounded-[9px] bg-brand-primary px-3 py-2 text-sm font-bold text-on-brand disabled:opacity-50"
          data-testid="placement-apply"
        >
          {t('pages:coverage.applyPlacement')}
        </button>
        <button
          type="button"
          onClick={onDelete}
          disabled={pending}
          className="rounded-[9px] border border-hairline px-3 py-2 text-sm text-text-primary hover:bg-surface-hover disabled:opacity-50"
          data-testid="placement-delete"
        >
          {t('pages:coverage.deletePlacement')}
        </button>
      </div>
    </form>
  );
}

/**
 * Whether the AP was fitted to this floor's walk. Absent means it is not drawn
 * on the band being looked at, so there is nothing to say about its model.
 */
function describeCalibration(
  calibration: APCalibration | undefined,
  t: TFunction<['common', 'pages']>,
) {
  if (!calibration) {
    return t('pages:coverage.calibrationNone');
  }
  if (!calibration.calibrated) {
    return t('pages:coverage.calibrationDefaults', { reason: calibration.fallback });
  }
  /* A clamped fit is still the floor's own fit, drawn at the physical bound
     its exponent ran into (owner decision 2026-10-15); it says so, since its
     exponent is the bound rather than what the walk measured. */
  const figures = {
    count: calibration.pairs,
    error: calibration.meanAbsErrorDb?.toFixed(1) ?? '—',
    exponent: calibration.exponent.toFixed(2),
  };
  return calibration.clamped
    ? t('pages:coverage.calibrationClamped', figures)
    : t('pages:coverage.calibrationFitted', figures);
}
