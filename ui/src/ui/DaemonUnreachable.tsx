import { WifiOff } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { iconSizes } from '../constants/sizes';

/**
 * The daemon cannot be reached, and the UI is already retrying. Full screen
 * before the shell has anything to show; a strip above the page once it has.
 */
export function DaemonUnreachable({ fullScreen = false }: { fullScreen?: boolean }) {
  const { t } = useTranslation('common');

  if (fullScreen) {
    return (
      <div
        role="status"
        data-testid="daemon-unreachable"
        className="flex h-screen items-center justify-center bg-surface-base px-4 text-text-primary"
      >
        <div className="panel w-full max-w-md p-6 text-center">
          <WifiOff className={`${iconSizes['3xl']} mx-auto text-status-warning`} aria-hidden />
          <h1 className="mt-4 text-lg font-semibold">{t('connection.unreachableTitle')}</h1>
          <p className="mt-2 text-sm text-text-secondary">{t('connection.unreachableBody')}</p>
        </div>
      </div>
    );
  }
  return (
    <div
      role="status"
      data-testid="daemon-unreachable-banner"
      className="flex items-center gap-2 border-b border-hairline bg-status-warning/10 px-6 py-2 text-sm text-status-warning-strong"
    >
      <WifiOff className={`${iconSizes.md} shrink-0`} aria-hidden />
      <span className="font-medium">{t('connection.unreachableTitle')}</span>
      <span>{t('connection.unreachableBody')}</span>
    </div>
  );
}
