import type { LucideIcon } from 'lucide-react';
import { FileText, Signal, Upload, Waypoints } from 'lucide-react';
import type { FC } from 'react';
import { useTranslation } from 'react-i18next';
import { CoveragePage } from '@/pages/CoveragePage';
import { ImportPage } from '@/pages/ImportPage';
import { ReportsPage } from '@/pages/ReportsPage';
import { SurveysPage } from '@/pages/SurveysPage';

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
 * whole namespaces; at four pages there is no reason to do that here.
 */
export interface PageConfig {
  path: string;
  /** Kicker above the title naming the product domain. */
  eyebrow?: string;
  title: string;
  description?: string;
  icon: LucideIcon;
  component: FC;
}

export function usePages(): PageConfig[] {
  const { t } = useTranslation(['pages', 'common']);

  return [
    {
      path: '/',
      eyebrow: t('common:nav.capture'),
      title: t('pages:surveys.title'),
      icon: Waypoints,
      component: SurveysPage,
    },
    {
      path: '/import',
      eyebrow: t('common:nav.capture'),
      title: t('pages:import.title'),
      description: t('pages:import.description'),
      icon: Upload,
      component: ImportPage,
    },
    {
      path: '/coverage',
      eyebrow: t('common:nav.analysis'),
      title: t('pages:coverage.title'),
      description: t('pages:coverage.description'),
      icon: Signal,
      component: CoveragePage,
    },
    {
      path: '/reports',
      eyebrow: t('common:nav.deliver'),
      title: t('pages:reports.title'),
      description: t('pages:reports.description'),
      icon: FileText,
      component: ReportsPage,
    },
  ];
}
