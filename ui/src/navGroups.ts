/**
 * Trellis navigation, grouped as the rail renders it.
 *
 * The route list comes from the product's own scope — survey capture, the
 * analyses run over a survey, and what leaves the tool — rather than from a
 * copy of another product's groups. Sibling products keep their own file for
 * the same reason; the shell is shared, the map of a product is not.
 *
 * Pages arrive over the following steps. A group whose pages do not exist yet
 * still belongs here: the rail is how the shape of the product is agreed, and
 * an item that routes nowhere is easier to see than one that was never listed.
 *
 * That only holds while the item is a page someone intends to build. Exports
 * was listed and is not: exporting a survey is not something the product does,
 * and listing it promised a page that was never coming.
 *
 * Reports came back. It was taken off the rail on the reasoning that a report
 * belongs to the survey it describes — true of *generating* one, which is
 * still a button in the survey's detail. What that reasoning missed is that
 * the engine reads five options the API hardcoded, including the company name
 * printed on the cover. Those choices need somewhere to live, and one button
 * has no room for them.
 */

import type { LucideIcon } from 'lucide-react';
import { Activity, FileText, Layers, Radar, Radio, Signal, Upload, Waypoints } from 'lucide-react';
import { useTranslation } from 'react-i18next';

export interface TrellisNavItem {
  label: string;
  path: string;
  icon: LucideIcon;
}

export interface TrellisNavGroup {
  /** Rendered with .kicker — the third label tier the family shares. */
  label: string;
  items: TrellisNavItem[];
}

/**
 * A hook rather than a constant because the labels are translated; resolving
 * them at module load would freeze the language chosen on first render. Keys
 * are literal so the key checker can see them.
 */
export function useNavGroups(): TrellisNavGroup[] {
  const { t } = useTranslation('common');

  return [
    {
      label: t('nav.capture'),
      items: [
        { label: t('nav.surveys'), path: '/', icon: Waypoints },
        { label: t('nav.import'), path: '/import', icon: Upload },
        { label: t('nav.floors'), path: '/floors', icon: Layers },
      ],
    },
    {
      label: t('nav.analysis'),
      items: [
        { label: t('nav.coverage'), path: '/coverage', icon: Signal },
        { label: t('nav.live'), path: '/live', icon: Radar },
        { label: t('nav.interference'), path: '/interference', icon: Radio },
        { label: t('nav.capacity'), path: '/capacity', icon: Activity },
      ],
    },
    {
      label: t('nav.deliver'),
      items: [{ label: t('nav.reports'), path: '/reports', icon: FileText }],
    },
  ];
}
