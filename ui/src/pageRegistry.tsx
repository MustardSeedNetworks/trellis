import type { LucideIcon } from 'lucide-react';
import { FileText, Radar, Signal, Upload, Waypoints } from 'lucide-react';
import { type ComponentType, lazy } from 'react';
import { useTranslation } from 'react-i18next';

// Lazy-loaded so each page's code ships in its own chunk instead of one
// entry bundle carrying all four — see App.tsx for the Suspense boundary
// that shows while a chunk is fetched.
const SurveysPage = lazy(() =>
  import('@/pages/SurveysPage').then((m) => ({ default: m.SurveysPage })),
);
const ImportPage = lazy(() =>
  import('@/pages/ImportPage').then((m) => ({ default: m.ImportPage })),
);
const CoveragePage = lazy(() =>
  import('@/pages/CoveragePage').then((m) => ({ default: m.CoveragePage })),
);
const ReportsPage = lazy(() =>
  import('@/pages/ReportsPage').then((m) => ({ default: m.ReportsPage })),
);
const LivePage = lazy(() => import('@/pages/LivePage').then((m) => ({ default: m.LivePage })));

/**
 * Page registry — declarative route table for Trellis.
 *
 * The header a page wears is rendered centrally by App from this table,
 * not by the page itself, matching seed / stem / niac. Page bodies
 * therefore cannot drift from the label the rail shows for the same route.
 *
 * The table is a hook rather than a constant because the labels are
 * translated: resolving them at module load would freeze whichever language
 * happened to be active first. Every key is written out literally rather than
 * built from the entry (`t(page.titleKey)`), so the key checker can see them.
 * The siblings build theirs dynamically and pay for it with broad
 * `dynamic-prefixes.txt` entries that switch the unused-key check off for
 * whole namespaces; at five pages there is no reason to do that here.
 */
export interface PageConfig {
  path: string;
  /** Kicker above the title naming the product domain. */
  eyebrow?: string;
  title: string;
  /** Subtitle under the title. Required: Surveys was the one page without one
   *  and wore a shorter header than the rest of the product (trellis#476), and
   *  a type that permits the gap is what let it happen. */
  description: string;
  help: string;
  icon: LucideIcon;
  component: ComponentType;
}

export function usePages(): PageConfig[] {
  const { t } = useTranslation(['pages', 'common']);

  return [
    {
      path: '/',
      eyebrow: t('common:nav.capture'),
      title: t('pages:surveys.title'),
      description: t('pages:surveys.description'),
      help: t('pages:help.surveys'),
      icon: Waypoints,
      component: SurveysPage,
    },
    {
      path: '/import',
      eyebrow: t('common:nav.capture'),
      title: t('pages:import.title'),
      description: t('pages:import.description'),
      help: t('pages:help.import'),
      icon: Upload,
      component: ImportPage,
    },
    {
      path: '/coverage',
      eyebrow: t('common:nav.analysis'),
      title: t('pages:coverage.title'),
      description: t('pages:coverage.description'),
      help: t('pages:help.coverage'),
      icon: Signal,
      component: CoveragePage,
    },
    {
      path: '/live',
      eyebrow: t('common:nav.analysis'),
      title: t('pages:live.title'),
      description: t('pages:live.description'),
      help: t('pages:live.description'),
      icon: Radar,
      component: LivePage,
    },
    {
      path: '/reports',
      eyebrow: t('common:nav.deliver'),
      title: t('pages:reports.title'),
      description: t('pages:reports.description'),
      help: t('pages:reports.description'),
      icon: FileText,
      component: ReportsPage,
    },
  ];
}
