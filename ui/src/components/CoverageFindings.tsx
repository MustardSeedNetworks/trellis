import type { RollupState } from '@/ui/StatusRollup';

/**
 * CoverageFindings — the verdict column of the Canvas archetype.
 *
 * Canvas pages do not open with a StatusRollup band the way Surveys and Import
 * do. The archetype puts the verdict beside the surface rather than above it,
 * because the reading is *about a region of the image* — "two gaps on this
 * floor" only means something next to the floor it is about. The honesty rules
 * the band exists to enforce still apply here and are enforced the same way:
 * an unreadable analysis prints em dashes and says why, never zeros.
 */
interface CoverageFindingsProps {
  state: RollupState;
  headline: string;
  body?: string;
  figures: { label: string; value: string }[];
  /** Service-generated recommendations. Empty is a legitimate answer. */
  recommendations: string[];
}

const STATE_STYLES: Record<RollupState, { edge: string; kicker: string }> = {
  ok: { edge: 'bg-status-success', kicker: 'text-status-success' },
  warn: { edge: 'bg-status-warning', kicker: 'text-status-warning' },
  crit: { edge: 'bg-status-error', kicker: 'text-status-error' },
  unknown: { edge: 'bg-text-muted', kicker: 'text-text-muted' },
};

const STATE_LABELS: Record<RollupState, string> = {
  ok: 'All clear',
  warn: 'Degraded',
  crit: 'Critical',
  unknown: 'No data',
};

export function CoverageFindings({
  state,
  headline,
  body,
  figures,
  recommendations,
}: CoverageFindingsProps) {
  const styles = STATE_STYLES[state];

  return (
    <section
      data-state={state}
      data-testid="coverage-findings"
      aria-live="polite"
      className="panel relative flex flex-col gap-4 overflow-hidden p-5"
    >
      <span aria-hidden="true" className={`absolute inset-y-0 left-0 w-[3px] ${styles.edge}`} />

      <div>
        <p className={`kicker ${styles.kicker}`}>{STATE_LABELS[state]}</p>
        <h2 className="mt-2 text-base font-extrabold tracking-[-0.02em] text-text-primary">
          {headline}
        </h2>
        {body ? <p className="mt-2 text-sm text-text-secondary">{body}</p> : null}
      </div>

      {figures.length > 0 ? (
        <dl className="flex flex-col gap-3 border-t border-hairline pt-4">
          {figures.map((figure) => (
            <div key={figure.label} className="flex items-baseline gap-3">
              <dt className="flex-1 text-sm text-text-secondary">{figure.label}</dt>
              {/* Unknown refuses to print figures for the same reason the
                  rollup does: a number here reads as a measurement. */}
              <dd className="figure text-sm font-bold text-text-primary">
                {state === 'unknown' ? '—' : figure.value}
              </dd>
            </div>
          ))}
        </dl>
      ) : null}

      {recommendations.length > 0 ? (
        <div className="flex flex-col gap-2 border-t border-hairline pt-4">
          <span className="kicker">Recommendations</span>
          <ul className="flex list-inside list-disc flex-col gap-1 text-sm text-text-secondary">
            {recommendations.map((recommendation) => (
              <li key={recommendation}>{recommendation}</li>
            ))}
          </ul>
        </div>
      ) : null}
    </section>
  );
}
