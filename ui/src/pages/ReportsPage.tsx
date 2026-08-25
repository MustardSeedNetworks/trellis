import { useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router';
import { surveyClient } from '@/lib/client';
import { bytesToDataUrl, reportFilename } from '@/lib/format';

/**
 * Reports — Form + wizard, in its single-screen form.
 *
 * The capability is not new: GenerateReport has always existed, reachable
 * from a survey's detail pane as one button with no choices. What was new is
 * that the report engine reads five options the API hardcoded, so every
 * operator got the same document — always heatmaps and recommendations,
 * never the raw-data appendix, and never their own name on the cover.
 *
 * The cover-page name is the reason this page is worth having rather than a
 * bigger button: it is the difference between a deliverable a consultant
 * hands to a client and a printout.
 *
 * The report is still *about* one survey, so the survey is chosen here the
 * same way Coverage chooses one, and for the same reason — a link that names
 * the survey opens the report for it.
 */
const SECTION_KEYS = [
  'includeExecutiveSummary',
  'includeRecommendations',
  'includeHeatmaps',
  'includeRawData',
] as const;

type SectionKey = (typeof SECTION_KEYS)[number];

/** Matches core/survey's DefaultReportOptions. */
const DEFAULT_SECTIONS: Record<SectionKey, boolean> = {
  includeExecutiveSummary: true,
  includeRecommendations: true,
  includeHeatmaps: true,
  includeRawData: false,
};

export function ReportsPage() {
  const { t } = useTranslation(['common', 'pages']);
  const [searchParams, setSearchParams] = useSearchParams();

  /* Resolved here rather than in a module constant: a constant would freeze
     whichever language loaded first. Written out literally rather than as
     t(section.labelKey), so the key checker can see every key — the dynamic
     form would need a dynamic-prefixes entry that switches the unused-key
     check off for this whole group. */
  const sectionCopy: Record<SectionKey, { label: string; hint: string }> = {
    includeExecutiveSummary: {
      label: t('pages:reports.sectionExecutiveSummary'),
      hint: t('pages:reports.sectionExecutiveSummaryHint'),
    },
    includeRecommendations: {
      label: t('pages:reports.sectionRecommendations'),
      hint: t('pages:reports.sectionRecommendationsHint'),
    },
    includeHeatmaps: {
      label: t('pages:reports.sectionHeatmaps'),
      hint: t('pages:reports.sectionHeatmapsHint'),
    },
    includeRawData: {
      label: t('pages:reports.sectionRawData'),
      hint: t('pages:reports.sectionRawDataHint'),
    },
  };
  const [sections, setSections] = useState<Record<SectionKey, boolean>>(DEFAULT_SECTIONS);
  const [companyName, setCompanyName] = useState('');

  const surveysQuery = useQuery({
    queryKey: ['surveys'],
    queryFn: () => surveyClient.listSurveys({}),
  });

  const surveys = surveysQuery.data?.surveys ?? [];
  const requestedId = searchParams.get('survey') ?? undefined;
  const surveyId = requestedId ?? surveys[0]?.id;
  const selected = surveys.find((survey) => survey.id === surveyId);

  const generate = useMutation({
    mutationFn: async () => {
      if (surveyId === undefined) {
        throw new Error('no survey selected');
      }
      const reply = await surveyClient.generateReport({
        surveyId,
        options: { ...sections, companyName: companyName.trim() },
      });
      // Reuse the tested bytes→data-URL path and trigger a browser download
      // via a transient anchor; no library needed for a one-shot PDF save.
      const link = document.createElement('a');
      link.href = bytesToDataUrl(reply.pdf, 'application/pdf');
      link.download = reportFilename(selected?.name ?? 'survey');
      link.click();
    },
  });

  return (
    <div className="flex flex-1 flex-col gap-6 overflow-y-auto p-6">
      <div className="panel flex max-w-2xl flex-col gap-6 p-6">
        <label className="flex flex-col gap-2 text-sm" htmlFor="report-survey">
          <span className="kicker">{t('common:labels.survey')}</span>
          <select
            id="report-survey"
            value={surveyId ?? ''}
            disabled={surveys.length === 0}
            onChange={(event) => setSearchParams({ survey: event.target.value })}
            className="w-fit rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary disabled:opacity-50"
          >
            {surveys.map((survey) => (
              <option key={survey.id} value={survey.id}>
                {survey.name}
              </option>
            ))}
          </select>
        </label>

        <fieldset className="flex flex-col gap-3 border-t border-hairline pt-6">
          <legend className="kicker">{t('common:labels.sections')}</legend>
          {SECTION_KEYS.map((sectionKey) => (
            <label key={sectionKey} className="flex items-start gap-3 text-sm">
              <input
                type="checkbox"
                checked={sections[sectionKey]}
                onChange={(event) =>
                  setSections((current) => ({ ...current, [sectionKey]: event.target.checked }))
                }
                className="mt-1"
              />
              <span className="flex flex-col">
                <span className="text-text-primary">{sectionCopy[sectionKey].label}</span>
                <span className="text-xs text-text-muted">{sectionCopy[sectionKey].hint}</span>
              </span>
            </label>
          ))}
        </fieldset>

        <label className="flex flex-col gap-2 text-sm" htmlFor="report-company">
          <span className="kicker">{t('common:labels.companyName')}</span>
          <input
            id="report-company"
            type="text"
            value={companyName}
            onChange={(event) => setCompanyName(event.target.value)}
            placeholder={t('pages:reports.companyPlaceholder')}
            className="rounded border border-hairline bg-surface-base px-3 py-2 text-sm text-text-primary"
          />
        </label>

        <div className="flex items-center gap-3 border-t border-hairline pt-6">
          <button
            type="button"
            onClick={() => generate.mutate()}
            disabled={surveyId === undefined || generate.isPending}
            className="rounded bg-brand-primary px-3 py-2 text-sm font-medium text-on-brand hover:bg-brand-accent disabled:opacity-50"
            data-testid="generate-report"
          >
            {generate.isPending ? t('common:buttons.generating') : t('pages:reports.generate')}
          </button>
          {surveys.length === 0 && !surveysQuery.isLoading ? (
            <span className="text-sm text-text-muted">{t('pages:reports.noSurveys')}</span>
          ) : null}
        </div>

        {generate.isError ? (
          <p className="text-sm text-status-error" data-testid="report-error">
            {t('pages:reports.generateFailed', { error: String(generate.error) })}
          </p>
        ) : null}
      </div>
    </div>
  );
}
