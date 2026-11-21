import { useTranslation } from 'react-i18next';
import { type LanguageCode, languages } from '@/i18n';

function isLanguageCode(value: string): value is LanguageCode {
  return languages.some((language) => language.code === value);
}

/**
 * English / Español selector.
 *
 * The choice persists because i18next's browser detector caches it in
 * localStorage under `language` (see i18n/index.ts), so there is no second
 * store to keep in step. The value is `resolvedLanguage`, not `language`: a
 * browser that reports `en-US` has no option of that name, and the select would
 * show nothing chosen while the page is plainly in English.
 */
export function LanguageSwitcher({ compact = false }: { compact?: boolean }) {
  const { t, i18n } = useTranslation('common');
  const label = t('accessibility.selectLanguage');

  return (
    <label className="mt-3 flex min-h-11 items-center gap-2 text-xs text-text-muted">
      {!compact ? <span className="shrink-0">{t('labels.language')}</span> : null}
      <select
        value={i18n.resolvedLanguage}
        onChange={(event) => {
          if (isLanguageCode(event.target.value)) {
            void i18n.changeLanguage(event.target.value);
          }
        }}
        aria-label={label}
        data-testid="language-switcher"
        className={`min-h-11 min-w-0 flex-1 rounded-[11px] border border-hairline bg-surface-base px-2 text-xs text-text-primary ${
          compact ? 'text-center' : ''
        }`}
      >
        {languages.map((language) => (
          <option key={language.code} value={language.code}>
            {compact ? language.code.toUpperCase() : language.nativeLabel}
          </option>
        ))}
      </select>
    </label>
  );
}
