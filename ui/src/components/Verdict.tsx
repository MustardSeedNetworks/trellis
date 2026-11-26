import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import type { VerdictState } from '@/lib/verdictState';
import { Card } from '@/shared/Card';
import { cx } from '@/shared/cx';
import { StatusPill, type StatusTone } from '@/shared/StatusPill';

export interface VerdictFigure {
  label: string;
  /** Printed in the figure face. Ignored while the verdict is unknown. */
  value: string;
}

interface VerdictProps {
  state: VerdictState;
  /** A sentence that names the finding, not a category. */
  headline: string;
  /** One paragraph of what to do about it. */
  body?: string;
  /** Up to four. More than four stops being a summary. */
  figures?: VerdictFigure[];
  actions?: ReactNode;
  /** What the reading is about, above the state, where the card could be mistaken for another. */
  kicker?: string;
  /** More about the reading, under the figures (recommendations). */
  children?: ReactNode;
  /** Lays the figures beside the headline rather than under it. */
  wide?: boolean;
  className?: string;
  'data-testid'?: string;
}

const TONES: Record<VerdictState, StatusTone> = {
  ok: 'success',
  warn: 'warning',
  crit: 'error',
  unknown: 'unknown',
};

/**
 * Verdict: a reading the product has taken, said once, in a flat `Card`.
 *
 * It exists for the pages that can be wrong (Live, Coverage findings). The
 * `unknown` state refuses to print figures at all: a card that shows zeros when
 * its source failed says "nothing is wrong" at the moment nobody can tell, so
 * that is enforced here rather than left to each caller. It is never the
 * answer to "there is no data yet": that is an `EmptyState`.
 */
export function Verdict({
  state,
  headline,
  body,
  figures = [],
  actions,
  wide = false,
  kicker,
  children,
  className,
  'data-testid': testId = 'verdict',
}: VerdictProps) {
  const { t } = useTranslation('common');

  /* Written out literally rather than as t(LABEL_KEYS[state]): a key built from
     a variable is invisible to the key checker. */
  const labels: Record<VerdictState, string> = {
    ok: t('status.ok'),
    warn: t('status.warn'),
    crit: t('status.crit'),
    unknown: t('status.unknown'),
  };
  const shown = figures.slice(0, 4);

  return (
    <Card className={className}>
      <div data-state={state} data-testid={testId} aria-live="polite">
        <div
          className={cx(
            'flex flex-col gap-4',
            wide && 'lg:flex-row lg:items-start lg:justify-between',
          )}
        >
          <div className="min-w-0">
            {kicker ? <p className="kicker mb-2 text-text-muted">{kicker}</p> : null}
            <StatusPill tone={TONES[state]} label={labels[state]} />
            <h2 className="mt-2 text-lg font-bold leading-snug text-text-primary">{headline}</h2>
            {body ? <p className="mt-1 max-w-prose text-sm text-text-secondary">{body}</p> : null}
            {actions ? <div className="mt-3 flex flex-wrap gap-2">{actions}</div> : null}
          </div>
          {shown.length > 0 ? (
            <dl
              className={cx(
                wide
                  ? 'flex shrink-0 flex-wrap gap-x-8 gap-y-3'
                  : 'flex flex-col gap-3 border-t border-hairline pt-4',
              )}
            >
              {shown.map((figure) => (
                <div key={figure.label} className={cx(!wide && 'flex items-baseline gap-3')}>
                  <dd
                    className={cx(
                      'figure font-bold text-text-primary',
                      wide ? 'text-xl leading-none' : 'order-2 text-sm',
                    )}
                  >
                    {state === 'unknown' ? '—' : figure.value}
                  </dd>
                  <dt
                    className={cx(
                      wide ? 'kicker mt-2' : 'order-1 flex-1 text-sm text-text-secondary',
                    )}
                  >
                    {figure.label}
                  </dt>
                </div>
              ))}
            </dl>
          ) : null}
        </div>
        {children ? <div className="mt-4 border-t border-hairline pt-4">{children}</div> : null}
      </div>
    </Card>
  );
}
