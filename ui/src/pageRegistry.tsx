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
  /** Kicker above the title naming the product domain. Every page has one. */
  kicker: string;
  title: string;
  /** Subtitle under the title. Required: Surveys was the one page without one
   *  and wore a shorter header than the rest of the product (trellis#476), and
   *  a type that permits the gap is what let it happen. */
  description: string;
  help: string;
  component: ComponentType;
}

export function usePages(): PageConfig[] {
  const { t } = useTranslation(['pages', 'common']);

  return [
    {
      path: '/',
      kicker: t('common:nav.capture'),
      title: t('pages:surveys.title'),
      description: t('pages:surveys.description'),
      help: t('pages:help.surveys'),
      component: SurveysPage,
    },
    {
      path: '/import',
      kicker: t('common:nav.capture'),
      title: t('pages:import.title'),
      description: t('pages:import.description'),
      help: t('pages:help.import'),
      component: ImportPage,
    },
    {
      path: '/coverage',
      kicker: t('common:nav.analysis'),
      title: t('pages:coverage.title'),
      description: t('pages:coverage.description'),
      help: t('pages:help.coverage'),
      component: CoveragePage,
    },
    {
      path: '/live',
      kicker: t('common:nav.analysis'),
      title: t('pages:live.title'),
      description: t('pages:live.description'),
      help: t('pages:live.description'),
      component: LivePage,
    },
    {
      path: '/reports',
      kicker: t('common:nav.deliver'),
      title: t('pages:reports.title'),
      description: t('pages:reports.description'),
      help: t('pages:reports.description'),
      component: ReportsPage,
    },
  ];
}
