import { Info, Palette } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { BuildVersion } from '@/hooks/useBuildVersion';
import { type Theme, useTheme } from '@/hooks/useTheme';
import { type LanguageCode, languages } from '@/i18n';
import { FormField } from '@/shared/FormField';
import { RightDrawer } from '@/shared/RightDrawer';
import { SegmentedControl } from '@/shared/SegmentedControl';
import { Select } from '@/shared/Select';
import { SettingsTabs } from '@/shared/SettingsTabs';

type SectionId = 'appearance' | 'about';

function isLanguageCode(value: string): value is LanguageCode {
  return languages.some((language) => language.code === value);
}

function isTheme(value: string): value is Theme {
  return value === 'system' || value === 'light' || value === 'dark';
}

/**
 * Settings: the preferences that live in this browser (theme, language) and
 * what this daemon says about itself. Nothing here writes to the daemon, and
 * the daemon's own configuration is not readable over its API yet, so it is
 * not shown rather than shown stale.
 */
export function SettingsDrawer({
  open,
  onClose,
  build,
}: {
  open: boolean;
  onClose: () => void;
  build: BuildVersion;
}) {
  const { t } = useTranslation('common');
  const [section, setSection] = useState<SectionId>('appearance');

  const tabs = [
    { id: 'appearance', label: t('settings.appearance'), icon: <Palette className="h-4 w-4" /> },
    { id: 'about', label: t('settings.about'), icon: <Info className="h-4 w-4" /> },
  ];

  return (
    <RightDrawer
      open={open}
      onClose={onClose}
      title={t('settings.title')}
      closeLabel={t('settings.close')}
      data-testid="settings-drawer"
    >
      <SettingsTabs
        tabs={tabs}
        value={section}
        onChange={(id) => setSection(id === 'about' ? 'about' : 'appearance')}
        label={t('settings.tabs')}
      >
        {section === 'appearance' ? <AppearanceSection /> : <AboutSection build={build} />}
      </SettingsTabs>
    </RightDrawer>
  );
}

function AppearanceSection() {
  const { t, i18n } = useTranslation('common');
  const { theme, setTheme } = useTheme();

  return (
    <div className="flex flex-col gap-5" data-testid="settings-appearance">
      <FormField label={t('settings.theme')} hint={t('settings.themeHint')}>
        {() => (
          <SegmentedControl
            label={t('settings.theme')}
            value={theme}
            onChange={(value) => {
              if (isTheme(value)) {
                setTheme(value);
              }
            }}
            options={[
              { value: 'system', label: t('settings.themeSystem') },
              { value: 'light', label: t('settings.themeLight') },
              { value: 'dark', label: t('settings.themeDark') },
            ]}
          />
        )}
      </FormField>

      <FormField label={t('labels.language')} hint={t('settings.languageHint')}>
        {(control) => (
          <Select
            {...control}
            value={i18n.resolvedLanguage ?? 'en'}
            onValueChange={(value) => {
              if (isLanguageCode(value)) {
                void i18n.changeLanguage(value);
              }
            }}
            options={languages.map((language) => ({
              value: language.code,
              label: language.nativeLabel,
            }))}
            data-testid="language-switcher"
          />
        )}
      </FormField>
    </div>
  );
}

function AboutSection({ build }: { build: BuildVersion }) {
  const { t } = useTranslation('common');
  const { version, commit, buildTime, uiBuildHash } = build;
  const facts = [
    { label: t('labels.version'), value: version },
    { label: t('settings.commit'), value: commit },
    { label: t('settings.built'), value: buildTime },
    { label: t('settings.uiBuild'), value: uiBuildHash },
  ];

  return (
    <div className="flex flex-col gap-4" data-testid="settings-about">
      <p className="text-sm text-text-muted">{t('settings.aboutHint')}</p>
      <dl className="flex flex-col divide-y divide-hairline">
        {facts.map((fact) => (
          <div key={fact.label} className="flex items-baseline justify-between gap-4 py-2">
            <dt className="kicker">{fact.label}</dt>
            <dd className="figure min-w-0 break-all text-right text-sm text-text-primary">
              {fact.value === '' ? '—' : fact.value}
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
