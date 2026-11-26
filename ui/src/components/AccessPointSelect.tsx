import { useTranslation } from 'react-i18next';
import type { HeardAccessPoint } from '@/gen/trellis/survey/v1/survey_pb';
import { FormField } from '@/shared/FormField';
import { Select } from '@/shared/Select';

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
    <FormField label={t('common:labels.accessPoint')} className="sm:w-64">
      {(control) => (
        <Select
          {...control}
          value={value}
          onValueChange={onChange}
          options={[
            { value: '', label: t('pages:coverage.allAccessPoints') },
            ...accessPoints.map((ap) => ({
              value: ap.bssid,
              label: t('pages:coverage.accessPointOption', {
                ssid: ap.ssid || t('common:labels.hiddenNetwork'),
                channel: ap.channel,
                bssid: ap.bssid,
              }),
            })),
          ]}
          data-testid="coverage-access-point"
        />
      )}
    </FormField>
  );
}
