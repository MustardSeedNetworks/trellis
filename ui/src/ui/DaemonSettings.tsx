import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { daemonClient } from '@/lib/client';
import { Alert } from '@/shared/Alert';
import { Skeleton } from '@/shared/Skeleton';
import { SettingsFacts } from '@/ui/SettingsFacts';

/**
 * What the running daemon was started with, read from its own API. Read only
 * and non-secret: the daemon does not send the credential or key material, so
 * there is nothing here to hide.
 */
export function DaemonSettings() {
  const { t } = useTranslation('common');
  const query = useQuery({
    queryKey: ['daemon-config'],
    queryFn: () => daemonClient.getConfig({}),
  });

  const captureModeLabel = (mode: string) => {
    switch (mode) {
      case 'scan':
        return t('settings.captureModes.scan');
      case 'monitor':
        return t('settings.captureModes.monitor');
      case 'unavailable':
        return t('settings.captureModes.unavailable');
      default:
        return mode;
    }
  };
  const yesNo = (on: boolean) => (on ? t('settings.yes') : t('settings.no'));
  const config = query.data;

  return (
    <div className="flex flex-col gap-4" data-testid="settings-daemon">
      <p className="text-sm text-text-muted">{t('settings.daemonHint')}</p>
      {query.isError ? (
        <Alert tone="error" title={t('settings.daemonUnavailable')}>
          {String(query.error)}
        </Alert>
      ) : config === undefined ? (
        <Skeleton lines={6} />
      ) : (
        <SettingsFacts
          facts={[
            { label: t('settings.listenAddr'), value: config.listenAddr },
            { label: t('settings.dataDir'), value: config.dataDir },
            {
              label: t('settings.captureMode'),
              value: captureModeLabel(config.captureMode),
            },
            { label: t('settings.tls'), value: yesNo(config.tlsEnabled) },
            { label: t('settings.authRequired'), value: yesNo(config.authRequired) },
            { label: t('settings.logFile'), value: config.logFile || t('settings.logStdout') },
          ]}
        />
      )}
    </div>
  );
}
