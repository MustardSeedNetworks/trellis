/**
 * Typed translation keys.
 *
 * Without this declaration merging, `t()` takes any string: a key that does
 * not exist typechecks, ships, and renders as the raw key. seed, stem and
 * niac-go all declare it; trellis was the one product where a typo in a
 * translation key was invisible until someone read the screen.
 *
 * The namespace shapes come from the EN locale files, so adding a key to
 * `en/*.json` is what makes it available to `t()` — there is nothing to
 * regenerate.
 */

import type enCommon from '@locales/en/common.json';
import type enPages from '@locales/en/pages.json';

export interface Translations {
  common: typeof enCommon;
  pages: typeof enPages;
}

declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'common';
    resources: Translations;
  }
}
