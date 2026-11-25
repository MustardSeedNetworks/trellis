import { useTranslation } from 'react-i18next';
import { Verdict, type VerdictFigure } from '@/components/Verdict';
import type { VerdictState } from '@/lib/verdictState';

/**
 * CoverageFindings — the verdict column of the Canvas archetype.
 *
 * Canvas pages do not open with a verdict band the way Live does. The
 * archetype puts the verdict beside the surface rather than above it, because
 * the reading is *about a region of the image*: "two gaps on this floor" only
 * means something next to the floor it is about. The honesty rules still
 * apply and are enforced by `Verdict`: an unreadable analysis prints em dashes
 * and says why, never zeros.
 */
interface CoverageFindingsProps {
  /**
   * What the findings are about. The panel reads as a verdict on whatever map
   * is on screen, and it is always the RSSI analysis whichever layer is drawn,
   * so it says which one it means rather than letting the map imply it.
   */
  title: string;
  state: VerdictState;
  headline: string;
  body?: string;
  figures: VerdictFigure[];
  /** Service-generated recommendations. Empty is a legitimate answer. */
  recommendations: string[];
}

export function CoverageFindings({
  title,
  state,
  headline,
  body,
  figures,
  recommendations,
}: CoverageFindingsProps) {
  const { t } = useTranslation('common');

  return (
    <Verdict
      data-testid="coverage-findings"
      kicker={title}
      state={state}
      headline={headline}
      {...(body === undefined ? {} : { body })}
      figures={figures}
    >
      {recommendations.length > 0 ? (
        <div className="flex flex-col gap-2">
          <span className="kicker">{t('labels.recommendations')}</span>
          <ul className="flex list-inside list-disc flex-col gap-1 text-sm text-text-secondary">
            {recommendations.map((recommendation) => (
              <li key={recommendation}>{recommendation}</li>
            ))}
          </ul>
        </div>
      ) : null}
    </Verdict>
  );
}
