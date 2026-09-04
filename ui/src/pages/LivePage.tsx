import { useQuery } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { NeighbourTable } from '@/components/NeighbourTable';
import type { ScannedNetwork } from '@/gen/trellis/survey/v1/survey_pb';
import { surveyClient } from '@/lib/client';
import { bandLabel, formatSignal } from '@/lib/format';
import { type RollupState, StatusRollup } from '@/ui/StatusRollup';

/**
 * Live — what this host's radio can hear right now.
 *
 * Trellis's other four pages read a survey: a walk that was taken, stored, and
 * is being looked at afterwards. This one reads the airspace with nothing
 * recorded, which is the shape Wi-Fi *analysis* has (2026-09-03 decision: Seed
 * does analysis, Trellis does analysis and survey, survey primary). It is
 * secondary to Surveys in the rail for that reason.
 *
 * It polls rather than streams. There is one radio, a scan blocks in its driver
 * for seconds, and the driver serves its cache to anything that asks again
 * inside that window — so a stream would repeat itself at whatever rate a
 * client chose. Polling also makes the cost visible: the same adapter is what a
 * walk captures with, and `pollMs` is how often this page takes it. Hence the
 * pause control, which is not a preference — leaving this page open during a
 * walk puts every survey point behind a live poll.
 */
const pollMs = 5000;

/** SNR below this reads as a connection worth looking at rather than a healthy one. */
const weakSnrDb = 20;

/** Channel utilisation at or above this is congestion, not traffic. */
const busyChannelPercent = 60;

export function LivePage() {
  const { t } = useTranslation(['common', 'pages']);
  const [polling, setPolling] = useState(true);

  const scanQuery = useQuery({
    queryKey: ['scan'],
    queryFn: () => surveyClient.scan({}),
    refetchInterval: polling ? pollMs : false,
    // A scan is a measurement, not a cache: showing the last one as current
    // while a fresh one is in flight would make a stale reading look live.
    staleTime: 0,
    retry: false,
  });

  const networks = scanQuery.data?.networks ?? [];
  const connected = networks.find((network) => network.associated);
  const rollup = describeAirspace(
    {
      loading: scanQuery.isPending,
      error: scanQuery.error,
      networks,
      connected,
    },
    t,
  );

  return (
    <div className="flex flex-1 flex-col gap-6 overflow-y-auto p-6">
      <StatusRollup
        state={rollup.state}
        headline={rollup.headline}
        body={rollup.body}
        figures={rollup.figures}
        actions={
          <button
            type="button"
            onClick={() => setPolling((on) => !on)}
            data-testid="toggle-polling"
            className="rounded border border-hairline px-3 py-2 text-sm text-text-primary hover:bg-surface-raised"
          >
            {polling ? t('pages:live.pause') : t('pages:live.resume')}
          </button>
        }
      />

      <NeighbourTable networks={networks} />
    </div>
  );
}

interface AirspaceState {
  loading: boolean;
  error: unknown;
  networks: ScannedNetwork[];
  connected: ScannedNetwork | undefined;
}

interface AirspaceRollup {
  state: RollupState;
  headline: string;
  body?: string;
  figures?: { label: string; value: string }[];
}

/**
 * describeAirspace turns a scan into the sentence the page leads with.
 *
 * The reading is about the connection where there is one, because that is what
 * an operator standing in a bad spot is asking about. With no association there
 * is no connection to judge, so the rollup reports the airspace and stays
 * `unknown` — StatusRollup withholds figures in that state, which is right: a
 * count of neighbours is not a verdict on anything.
 */
function describeAirspace(
  { loading, error, networks, connected }: AirspaceState,
  t: TFunction<['common', 'pages']>,
): AirspaceRollup {
  if (error) {
    return {
      state: 'crit',
      headline: t('pages:live.scanFailed'),
      body: error instanceof Error ? error.message : String(error),
    };
  }
  if (loading) {
    return { state: 'unknown', headline: t('pages:live.scanning') };
  }
  if (!connected) {
    return {
      state: 'unknown',
      headline: t('pages:live.notAssociated'),
      body: t('pages:live.notAssociatedBody', { count: networks.length }),
    };
  }

  const figures = [
    { label: t('pages:live.columns.signal'), value: formatSignal(connected.signalDbm, 'dBm') },
    { label: t('pages:live.columns.snr'), value: formatSignal(connected.snrDb, 'dB') },
    {
      label: t('pages:live.columns.channel'),
      value: t('pages:live.channelCell', {
        channel: connected.channel,
        band: bandLabel(connected.frequencyMhz),
        width: connected.channelWidthMhz,
      }),
    },
    {
      label: t('pages:live.columns.utilization'),
      value:
        connected.channelUtilizationPercent === undefined
          ? t('pages:live.notReported')
          : `${connected.channelUtilizationPercent}%`,
    },
  ];
  const ssid = connected.ssid || t('pages:live.hiddenNetwork');

  // SNR is what decides this, not signal: a strong signal on a noisy channel
  // performs worse than a weaker one in quiet air, and the derived margin is
  // the number that says which of those an operator is standing in.
  if (connected.snrDb < weakSnrDb) {
    return {
      state: 'warn',
      headline: t('pages:live.weakLink', { ssid, snr: connected.snrDb }),
      body: t('pages:live.weakLinkBody'),
      figures,
    };
  }
  if (
    connected.channelUtilizationPercent !== undefined &&
    connected.channelUtilizationPercent >= busyChannelPercent
  ) {
    return {
      state: 'warn',
      headline: t('pages:live.busyChannel', {
        ssid,
        percent: connected.channelUtilizationPercent,
      }),
      body: t('pages:live.busyChannelBody'),
      figures,
    };
  }
  return {
    state: 'ok',
    headline: t('pages:live.healthy', { ssid, snr: connected.snrDb }),
    body: t('pages:live.healthyBody', { count: networks.length }),
    figures,
  };
}
