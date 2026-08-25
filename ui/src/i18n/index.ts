/**
 * i18n configuration.
 *
 * Trellis shipped without i18n and was retrofitted at 22 files, deliberately
 * early: seed's retrofit cost 407 fallback removals, and the price only rises
 * with the string count. The gate blocks from day one here — no `--ratchet`,
 * no baseline file — so this repo never accumulates the debt the others paid
 * down.
 *
 * Locales live beside the Go package that embeds them, so the backend and the
 * bundle read the same files rather than two copies that drift.
 */
import enCommon from '@locales/en/common.json';
import enPages from '@locales/en/pages.json';
import esCommon from '@locales/es/common.json';
import esPages from '@locales/es/pages.json';
import i18n from 'i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { initReactI18next } from 'react-i18next';

export const languages = [
  { code: 'en', label: 'English', nativeLabel: 'English' },
  { code: 'es', label: 'Spanish', nativeLabel: 'Español' },
] as const;

export type LanguageCode = (typeof languages)[number]['code'];

export const namespaces = ['common', 'pages'] as const;

export type Namespace = (typeof namespaces)[number];

export const defaultNs: Namespace = 'common';

const resources = {
  en: { common: enCommon, pages: enPages },
  es: { common: esCommon, pages: esPages },
};

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: resources as Parameters<typeof i18n.init>[0]['resources'],
    fallbackLng: 'en',
    defaultNS: defaultNs,
    ns: namespaces,
    detection: {
      order: ['localStorage', 'navigator', 'htmlTag'],
      caches: ['localStorage'],
      lookupLocalStorage: 'language',
    },
    interpolation: {
      // React escapes values already.
      escapeValue: false,
    },
    debug: import.meta.env.DEV,
  });

export default i18n;
