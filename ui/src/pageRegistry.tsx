import type { LucideIcon } from 'lucide-react';
import { Upload, Waypoints } from 'lucide-react';
import type { FC } from 'react';
import { ImportPage } from '@/pages/ImportPage';
import { SurveysPage } from '@/pages/SurveysPage';

/**
 * Page registry — declarative route table for Trellis.
 *
 * The header a page wears is rendered centrally by App from this table,
 * not by the page itself, matching seed / stem / niac. Page bodies
 * therefore cannot drift from the label the rail shows for the same route.
 *
 * Deviation from the siblings: their entries key into a `pages` locale
 * namespace, because those products ship i18n. Trellis has none, so the
 * copy is literal here. It is still single-source — the page no longer
 * carries a second copy of it — and this is the file that grows a t()
 * call if trellis ever gains locales.
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

export const pages: PageConfig[] = [
  {
    path: '/',
    eyebrow: 'Capture',
    title: 'Surveys',
    icon: Waypoints,
    component: SurveysPage,
  },
  {
    path: '/import',
    eyebrow: 'Capture',
    title: 'Import',
    description: 'Bring an AirMapper .amp archive in as a survey.',
    icon: Upload,
    component: ImportPage,
  },
];
