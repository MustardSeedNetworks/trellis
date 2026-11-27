import { Info, Palette, Server } from 'lucide-react';
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
import { DaemonSettings } from '@/ui/DaemonSettings';
import { SettingsFacts } from '@/ui/SettingsFacts';

const SECTIONS = ['appearance', 'daemon', 'about'] as const;
type SectionId = (typeof SECTIONS)[number];

function isSectionId(value: string): value is SectionId {
  return SECTIONS.some((section) => section === value);
}

function isLanguageCode(value: string): value is LanguageCode {
  return languages.some((language) => language.code === value);
}

function isTheme(value: string): value is Theme {
  return value === 'system' || value === 'light' || value === 'dark';
}

/**
 * Settings: the preferences that live in this browser (theme, language) and
 * what this daemon says about itself: its configuration and its build. Nothing
 * here writes to the daemon.
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
    { id: 'daemon', label: t('settings.daemon'), icon: <Server className="h-4 w-4" /> },
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
        onChange={(id) => {
          if (isSectionId(id)) {
            setSection(id);
          }
        }}
        label={t('settings.tabs')}
      >
        {section === 'appearance' ? <AppearanceSection /> : null}
        {section === 'daemon' ? <DaemonSettings /> : null}
        {section === 'about' ? <AboutSection build={build} /> : null}
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
      <SettingsFacts facts={facts} />
    </div>
  );
}
