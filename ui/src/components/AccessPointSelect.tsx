import { useTranslation } from 'react-i18next';
import type { HeardAccessPoint } from '@/gen/trellis/survey/v1/survey_pb';

/**
 * AccessPointSelect — which access point the Coverage map is about.
 *
 * Empty is every AP, the floor's coverage; a BSSID is where that one AP
 * serves. APs are told apart the way a surveyor does, by network name and
 * channel, with the BSSID last because two radios often share both.
 */
interface AccessPointSelectProps {
  accessPoints: readonly Pick<HeardAccessPoint, 'bssid' | 'ssid' | 'channel'>[];
  value: string;
  onChange: (bssid: string) => void;
}

export function AccessPointSelect({ accessPoints, value, onChange }: AccessPointSelectProps) {
  const { t } = useTranslation(['common', 'pages']);
  return (
    <label
      className="flex min-w-0 flex-wrap items-center gap-2 text-sm"
      htmlFor="coverage-access-point"
    >
      <span className="kicker">{t('common:labels.accessPoint')}</span>
      <select
        id="coverage-access-point"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="w-full min-w-0 rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary md:w-auto md:max-w-full"
        data-testid="coverage-access-point"
      >
        <option value="">{t('pages:coverage.allAccessPoints')}</option>
        {accessPoints.map((ap) => (
          <option key={ap.bssid} value={ap.bssid}>
            {t('pages:coverage.accessPointOption', {
              ssid: ap.ssid || t('common:labels.hiddenNetwork'),
              channel: ap.channel,
              bssid: ap.bssid,
            })}
          </option>
        ))}
      </select>
    </label>
  );
}
