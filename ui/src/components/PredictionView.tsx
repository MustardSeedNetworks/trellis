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
import { useTableLabels } from '@/lib/useTableLabels';
import { Alert } from '@/shared/Alert';
import { Button } from '@/shared/Button';
import { Card } from '@/shared/Card';
import { ConfirmDialog } from '@/shared/ConfirmDialog';
import { DataTable, type DataTableColumn } from '@/shared/DataTable';
import { FormField } from '@/shared/FormField';
import { Input } from '@/shared/Input';
import { Select } from '@/shared/Select';

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

  const tableLabels = useTableLabels<APPlacement>();
  const placementColumns: DataTableColumn<APPlacement>[] = [
    {
      key: 'name',
      header: t('pages:coverage.placementName'),
      cell: (ap) => (
        <span className="font-semibold text-text-primary">
          {apLabel(ap, placements.indexOf(ap), t)}
        </span>
      ),
    },
    {
      key: 'radio',
      header: t('pages:coverage.placementRadio'),
      cell: (ap) => <span className="figure text-xs">{describeRadio(ap, t)}</span>,
    },
  ];
  const selectedIndex = placements.findIndex((ap) => ap.id === selectedId);
  const selected = placements[selectedIndex];
  const calibrations = heatmapError ? [] : (heatmap?.calibrations ?? []);

  return (
    <>
      <Card className="flex min-w-0 flex-col gap-4">
        {renderPredictionSurface()}
        {heatmap && !heatmapError ? (
          <div className="sticky bottom-0 -mx-4 -mb-4 bg-surface-raised px-4 pb-4 pt-3 sm:-mx-5 sm:-mb-5 sm:px-5 sm:pb-5">
            <HeatmapLegend stops={heatmap.legend} unit="dBm" />
          </div>
        ) : null}
      </Card>

      <Card className="flex flex-col gap-4" data-testid="placement-panel">
        <div>
          <p className="kicker text-text-muted">{t('pages:coverage.plannedTitle')}</p>
          <p className="mt-2 text-sm text-text-secondary">{t('pages:coverage.plannedHint')}</p>
        </div>
        {failed ? (
          <div data-testid="placement-error">
            <Alert tone="error">
              {t('pages:coverage.placementFailed', { error: ConnectError.from(failed).rawMessage })}
            </Alert>
          </div>
        ) : null}
        {placements.length > 0 ? (
          <DataTable
            data-testid="placement-list"
            rows={placements}
            columns={placementColumns}
            getRowKey={(ap) => ap.id}
            caption={t('pages:coverage.plannedTitle')}
            labels={tableLabels}
            empty={null}
            onRowClick={(ap) => setSelectedId(ap.id)}
            {...(selectedId === undefined ? {} : { selectedRowKey: selectedId })}
          />
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
      </Card>
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
    <div data-testid="surface-message">
      {error ? (
        <Alert tone="error">{text}</Alert>
      ) : (
        <p className="text-sm text-text-muted">{text}</p>
      )}
    </div>
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

  const [confirming, setConfirming] = useState(false);

  return (
    <form
      onSubmit={submit}
      className="flex flex-col gap-3 border-t border-hairline pt-4"
      data-testid="placement-editor"
    >
      <h3 className="text-base font-semibold text-text-primary">{name}</h3>
      <FormField label={t('pages:coverage.band')}>
        {(control) => (
          <Select
            {...control}
            value={band}
            onValueChange={setBand}
            options={[
              { value: '', label: t('pages:coverage.everyBand') },
              ...BANDS.map((option) => ({ value: option, label: option })),
            ]}
            data-testid="placement-band"
          />
        )}
      </FormField>
      <FormField label={t('pages:coverage.txPower')}>
        {(control) => (
          <Input
            {...control}
            type="number"
            min={TX_POWER.min}
            max={TX_POWER.max}
            step={1}
            value={power}
            placeholder={t('pages:coverage.txPowerAssumed', { power: TX_POWER.assumed })}
            onChange={(event) => setPower(event.target.value)}
            aria-invalid={!powerValid}
            className="figure"
            data-testid="placement-power"
          />
        )}
      </FormField>
      <p className="text-sm text-text-secondary" data-testid="placement-calibration">
        {describeCalibration(calibration, t)}
      </p>
      <div className="flex flex-wrap gap-2">
        <Button
          type="submit"
          variant="primary"
          disabled={pending || !powerValid}
          data-testid="placement-apply"
        >
          {t('pages:coverage.applyPlacement')}
        </Button>
        <Button
          variant="destructive"
          onClick={() => setConfirming(true)}
          disabled={pending}
          data-testid="placement-delete"
        >
          {t('pages:coverage.deletePlacement')}
        </Button>
      </div>
      <ConfirmDialog
        open={confirming}
        title={t('pages:coverage.deletePlacement')}
        message={t('pages:coverage.deletePlacementPrompt', { name })}
        confirmLabel={t('pages:coverage.deletePlacement')}
        cancelLabel={t('pages:coverage.keepPlacement')}
        closeLabel={t('common:accessibility.close')}
        onConfirm={() => {
          setConfirming(false);
          onDelete();
        }}
        onCancel={() => setConfirming(false)}
      />
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
