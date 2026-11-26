import { RadioTower } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { ScannedNetwork } from '@/gen/trellis/survey/v1/survey_pb';
import { bandLabel, formatSignal } from '@/lib/format';
import { useTableLabels } from '@/lib/useTableLabels';
import { CategoryChip } from '@/shared/CategoryChip';
import { DataTable, type DataTableColumn } from '@/shared/DataTable';
import { EmptyState } from '@/shared/EmptyState';
import { StatusPill } from '@/shared/StatusPill';

/**
 * NeighbourTable — every BSS one sweep heard, strongest first.
 *
 * A component rather than markup inside the Live page so the airspaces worth
 * looking at — a hidden network, a DFS channel, an AP that reports no BSS Load
 * element, the row the host is joined to — are reachable as stories and go
 * through the accessibility gate. The server does the ordering, so the table
 * opens in that order and only re-sorts when the operator asks; one that sorted
 * on its own would disagree with what a stored survey point aggregated to.
 */
export function NeighbourTable({ networks }: { networks: ScannedNetwork[] }) {
  const { t } = useTranslation(['common', 'pages']);
  const labels = useTableLabels<ScannedNetwork>();
  const notReported = t('pages:live.notReported');

  const columns: DataTableColumn<ScannedNetwork>[] = [
    {
      key: 'network',
      header: t('pages:live.columns.network'),
      sortValue: (network) => network.ssid || t('common:labels.hiddenNetwork'),
      /* A hidden network is named as hidden rather than left blank: an empty
         cell reads as a rendering fault, and "this AP is not broadcasting its
         SSID" is a real observation about the airspace. */
      cell: (network) => (
        <span
          className="inline-flex items-center gap-2 text-text-primary"
          data-testid="neighbour-row"
          data-associated={network.associated ? 'true' : undefined}
        >
          {network.ssid || t('common:labels.hiddenNetwork')}
          {network.associated ? (
            <StatusPill tone="info" label={t('pages:live.connectedBadge')} />
          ) : null}
        </span>
      ),
    },
    {
      key: 'bssid',
      header: t('pages:live.columns.bssid'),
      cell: (network) => <span className="figure text-xs">{network.bssid}</span>,
    },
    {
      key: 'channel',
      header: t('pages:live.columns.channel'),
      sortValue: (network) => network.channel,
      cell: (network) => (
        <span className="inline-flex items-center gap-2">
          {t('pages:live.channelCell', {
            channel: network.channel,
            band: bandLabel(network.frequencyMhz),
            width: network.channelWidthMhz,
          })}
          {network.isDfs ? <CategoryChip label={t('pages:live.dfs')} /> : null}
        </span>
      ),
    },
    {
      key: 'signal',
      header: t('pages:live.columns.signal'),
      sortValue: (network) => network.signalDbm,
      cell: (network) => (
        <span className="text-text-primary">{formatSignal(network.signalDbm, 'dBm')}</span>
      ),
    },
    {
      key: 'snr',
      header: t('pages:live.columns.snr'),
      sortValue: (network) => network.snrDb ?? Number.NEGATIVE_INFINITY,
      cell: (network) =>
        network.snrDb === undefined ? notReported : formatSignal(network.snrDb, 'dB'),
    },
    {
      key: 'security',
      header: t('pages:live.columns.security'),
      sortValue: (network) => network.security,
      cell: (network) => <CategoryChip label={network.security} />,
    },
    {
      key: 'utilization',
      header: t('pages:live.columns.utilization'),
      sortValue: (network) => network.channelUtilizationPercent ?? -1,
      cell: (network) =>
        network.channelUtilizationPercent === undefined
          ? notReported
          : `${network.channelUtilizationPercent}%`,
    },
  ];

  return (
    <section aria-labelledby="neighbours-heading" className="flex min-w-0 flex-col gap-2">
      <h2 id="neighbours-heading" className="kicker">
        {t('pages:live.neighbours', { count: networks.length })}
      </h2>
      <DataTable
        data-testid="neighbour-table"
        rows={networks}
        columns={columns}
        getRowKey={(network) => network.bssid}
        caption={t('pages:live.neighbourCaption')}
        labels={labels}
        empty={
          <div className="panel">
            <EmptyState
              icon={<RadioTower className="h-6 w-6" />}
              title={t('pages:live.nothingHeardTitle')}
              hint={t('pages:live.nothingHeard')}
            />
          </div>
        }
      />
    </section>
  );
}
