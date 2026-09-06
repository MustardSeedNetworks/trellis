import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { surveyClient } from '@/lib/client';

/**
 * CaptureCapabilityNotice — what this host can measure, said before an operator
 * sets out rather than after a walk fails.
 *
 * The daemon has always known: it logs "no Wi-Fi capture backend on this host"
 * at startup and carries on serving a UI that offers to walk a floor. On a
 * packaged install nobody reads that log, so the first sign was a capture that
 * returned an error — after the operator had already picked a spot and stood in
 * it. This reads the capability once and says it in the pages where a walk
 * starts.
 *
 * It is deliberately not an error state. A host with no radio browses surveys,
 * imports other tools' captures, analyses them and produces reports; saying
 * "unavailable" without saying that would read as "this machine cannot use
 * Trellis", which is false.
 */
export function useCaptureCapability() {
  return useQuery({
    queryKey: ['capture-capability'],
    queryFn: () => surveyClient.getCaptureCapability({}),
    // A radio does not appear and disappear while a page is open, and the
    // daemon revises this once after its readiness scan; a refetch on mount is
    // enough to pick that up without polling for a fact that does not move.
    staleTime: 60_000,
  });
}

export function CaptureCapabilityNotice() {
  const { t } = useTranslation(['pages']);
  const { data } = useCaptureCapability();

  if (!data || data.available) {
    return null;
  }

  return (
    <section
      className="rounded border border-hairline bg-surface-raised p-4 text-sm text-text-primary"
      aria-live="polite"
      data-testid="capture-unavailable"
    >
      <p className="font-medium">{t('pages:capture.unavailable')}</p>
      <p className="mt-1 text-text-muted">{data.reason}</p>
      {data.remedy ? <p className="mt-1 text-text-muted">{data.remedy}</p> : null}
      <p className="mt-2 text-text-muted">{t('pages:capture.stillUsable')}</p>
    </section>
  );
}
