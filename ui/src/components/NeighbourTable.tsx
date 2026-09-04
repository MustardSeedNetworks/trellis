import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import type { ScannedNetwork } from '@/gen/trellis/survey/v1/survey_pb';
import { bandLabel, formatSignal } from '@/lib/format';

/**
 * NeighbourTable — every BSS one sweep heard, strongest first.
 *
 * A component rather than markup inside the Live page so the airspaces worth
 * looking at — a hidden network, a DFS channel, an AP that reports no BSS Load
 * element, the row the host is joined to — are reachable as stories and go
 * through the accessibility gate. The server does the ordering; a table that
 * re-sorted here would disagree with what a stored survey point aggregated to.
 */
export function NeighbourTable({ networks }: { networks: ScannedNetwork[] }) {
  const { t } = useTranslation(['common', 'pages']);

  return (
    <section className="panel flex flex-col gap-4 p-5" aria-labelledby="neighbours-heading">
      <h2 id="neighbours-heading" className="kicker">
        {t('pages:live.neighbours', { count: networks.length })}
      </h2>

      {networks.length === 0 ? (
        <p className="text-sm text-text-secondary">{t('pages:live.nothingHeard')}</p>
      ) : (
        // The table can outgrow a narrow window on its own without the page
        // scrolling sideways with it.
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm" data-testid="neighbour-table">
            <thead className="text-text-secondary">
              <tr>
                <th scope="col" className="py-2 pr-4 font-normal">
                  {t('pages:live.columns.network')}
                </th>
                <th scope="col" className="py-2 pr-4 font-normal">
                  {t('pages:live.columns.bssid')}
                </th>
                <th scope="col" className="py-2 pr-4 font-normal">
                  {t('pages:live.columns.channel')}
                </th>
                <th scope="col" className="py-2 pr-4 font-normal">
                  {t('pages:live.columns.signal')}
                </th>
                <th scope="col" className="py-2 pr-4 font-normal">
                  {t('pages:live.columns.snr')}
                </th>
                <th scope="col" className="py-2 pr-4 font-normal">
                  {t('pages:live.columns.security')}
                </th>
                <th scope="col" className="py-2 font-normal">
                  {t('pages:live.columns.utilization')}
                </th>
              </tr>
            </thead>
            <tbody>
              {networks.map((network) => (
                <NeighbourRow key={network.bssid} network={network} t={t} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

/**
 * One observed BSS.
 *
 * A hidden network is named as hidden rather than left blank: an empty cell
 * reads as a rendering fault, and "this AP is not broadcasting its SSID" is a
 * real observation about the airspace.
 */
function NeighbourRow({
  network,
  t,
}: {
  network: ScannedNetwork;
  t: TFunction<['common', 'pages']>;
}) {
  return (
    <tr
      data-testid="neighbour-row"
      data-associated={network.associated ? 'true' : undefined}
      className="border-hairline border-t"
    >
      <td className="py-2 pr-4 text-text-primary">
        {network.ssid || t('pages:live.hiddenNetwork')}
        {network.associated ? (
          <span className="ml-2 rounded bg-surface-raised px-2 py-0.5 text-text-secondary text-xs">
            {t('pages:live.connectedBadge')}
          </span>
        ) : null}
      </td>
      <td className="py-2 pr-4 font-mono text-text-secondary text-xs">{network.bssid}</td>
      <td className="py-2 pr-4 text-text-secondary">
        {t('pages:live.channelCell', {
          channel: network.channel,
          band: bandLabel(network.frequencyMhz),
          width: network.channelWidthMhz,
        })}
        {network.isDfs ? ` ${t('pages:live.dfs')}` : ''}
      </td>
      <td className="py-2 pr-4 text-text-primary">{formatSignal(network.signalDbm, 'dBm')}</td>
      <td className="py-2 pr-4 text-text-secondary">{formatSignal(network.snrDb, 'dB')}</td>
      <td className="py-2 pr-4 text-text-secondary">{network.security}</td>
      <td className="py-2 text-text-secondary">
        {network.channelUtilizationPercent === undefined
          ? t('pages:live.notReported')
          : `${network.channelUtilizationPercent}%`}
      </td>
    </tr>
  );
}
