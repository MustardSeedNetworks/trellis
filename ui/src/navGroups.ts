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
 * That only holds while the item is a page someone intends to build. Reports
 * and Exports were listed and are not: a report is produced from the survey it
 * describes, so it belongs to the survey rather than to a destination of its
 * own, and exporting a survey is not something the product does. Listing them
 * promised two pages that were never coming.
 */

import type { LucideIcon } from 'lucide-react';
import { Activity, Layers, Radio, Signal, Upload, Waypoints } from 'lucide-react';

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

export const navGroups: TrellisNavGroup[] = [
  {
    label: 'Capture',
    items: [
      { label: 'Surveys', path: '/', icon: Waypoints },
      { label: 'Import', path: '/import', icon: Upload },
      { label: 'Floors', path: '/floors', icon: Layers },
    ],
  },
  {
    label: 'Analysis',
    items: [
      { label: 'Coverage', path: '/coverage', icon: Signal },
      { label: 'Interference', path: '/interference', icon: Radio },
      { label: 'Capacity', path: '/capacity', icon: Activity },
    ],
  },
];
