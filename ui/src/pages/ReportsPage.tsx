import { useMutation, useQuery } from '@tanstack/react-query';
import { FileText } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useSearchParams } from 'react-router';
import { surveyClient } from '@/lib/client';
import {
  COVERAGE_METRIC_COPY,
  COVERAGE_METRICS,
  type CoverageMetric,
  DEFAULT_THRESHOLDS,
  isCoverageMetric,
  parseThreshold,
  THRESHOLDS,
} from '@/lib/coverageThreshold';
import { bytesToDataUrl, reportFilename } from '@/lib/format';
import { Button, ButtonLink } from '@/shared/Button';
import { Card } from '@/shared/Card';
import { Checkbox } from '@/shared/Checkbox';
import { EmptyState } from '@/shared/EmptyState';
import { FormField } from '@/shared/FormField';
import { Input } from '@/shared/Input';
import { SegmentedControl } from '@/shared/SegmentedControl';
import { Select } from '@/shared/Select';
import { useToast } from '@/ui/toastContext';

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
 * the survey opens the report for it. Its findings are judged by the same
 * metric and threshold control too, or the PDF would disagree with the dead
 * zones the operator just analysed (trellis#509).
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
  const { addToast } = useToast();
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
  const [metric, setMetric] = useState<CoverageMetric>('rssi');
  const [thresholds, setThresholds] = useState(DEFAULT_THRESHOLDS);
  const bounds = THRESHOLDS[metric];

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
        options: {
          ...sections,
          companyName: companyName.trim(),
          metric,
          threshold: thresholds[metric],
        },
      });
      // Reuse the tested bytes→data-URL path and trigger a browser download
      // via a transient anchor; no library needed for a one-shot PDF save.
      const link = document.createElement('a');
      link.href = bytesToDataUrl(reply.pdf, 'application/pdf');
      link.download = reportFilename(selected?.name ?? 'survey');
      link.click();
    },
    onError: (error) =>
      addToast(t('pages:reports.generateFailed', { error: String(error) }), 'error'),
  });

  if (surveys.length === 0 && !surveysQuery.isLoading) {
    return (
      <Card>
        <EmptyState
          icon={<FileText className="h-6 w-6" />}
          title={t('pages:reports.noSurveysTitle')}
          hint={t('pages:reports.noSurveys')}
          action={
            <ButtonLink renderLink={(link) => <Link to="/import" {...link} />}>
              {t('pages:reports.importSurvey')}
            </ButtonLink>
          }
        />
      </Card>
    );
  }

  return (
    <Card className="flex max-w-2xl flex-col gap-6">
      <FormField label={t('common:labels.survey')}>
        {(control) => (
          <Select
            {...control}
            value={surveyId ?? ''}
            disabled={surveys.length === 0}
            onValueChange={(value) => setSearchParams({ survey: value })}
            options={surveys.map((survey) => ({ value: survey.id, label: survey.name }))}
          />
        )}
      </FormField>

      <fieldset className="flex flex-col gap-3 border-t border-hairline pt-6">
        <legend className="kicker">{t('common:labels.sections')}</legend>
        {SECTION_KEYS.map((sectionKey) => (
          <Checkbox
            key={sectionKey}
            checked={sections[sectionKey]}
            onChange={(checked) =>
              setSections((current) => ({ ...current, [sectionKey]: checked }))
            }
            label={sectionCopy[sectionKey].label}
            description={sectionCopy[sectionKey].hint}
          />
        ))}
      </fieldset>

      <fieldset className="flex flex-col gap-3 border-t border-hairline pt-6">
        <legend className="kicker">{t('pages:reports.analysis')}</legend>
        <p className="text-xs text-text-muted">{t('pages:reports.analysisHint')}</p>
        <div className="flex flex-wrap items-end gap-x-6 gap-y-3">
          <FormField label={t('common:labels.metric')} className="w-auto">
            {() => (
              <SegmentedControl
                label={t('common:labels.metric')}
                value={metric}
                onChange={(value) => {
                  if (isCoverageMetric(value)) {
                    setMetric(value);
                  }
                }}
                options={COVERAGE_METRICS.map((option) => ({
                  value: option,
                  label: COVERAGE_METRIC_COPY[option].glossary,
                }))}
              />
            )}
          </FormField>
          <FormField label={t('common:labels.deadZoneThreshold')} className="sm:w-40">
            {(control) => (
              <div className="flex items-center gap-2">
                <Input
                  {...control}
                  type="number"
                  value={thresholds[metric]}
                  min={bounds.min}
                  max={bounds.max}
                  step={1}
                  onChange={(event) => {
                    const parsed = parseThreshold(metric, event.target.value);
                    if (parsed !== undefined) {
                      setThresholds((current) => ({ ...current, [metric]: parsed }));
                    }
                  }}
                  className="figure"
                  data-testid="report-threshold"
                />
                <span className="text-sm text-text-secondary" data-testid="report-threshold-unit">
                  {COVERAGE_METRIC_COPY[metric].unit}
                </span>
              </div>
            )}
          </FormField>
        </div>
      </fieldset>

      <FormField label={t('common:labels.companyName')}>
        {(control) => (
          <Input
            {...control}
            value={companyName}
            onChange={(event) => setCompanyName(event.target.value)}
            placeholder={t('pages:reports.companyPlaceholder')}
          />
        )}
      </FormField>

      <div className="border-t border-hairline pt-6">
        <Button
          variant="primary"
          onClick={() => generate.mutate()}
          loading={generate.isPending}
          disabled={surveyId === undefined}
          data-testid="generate-report"
        >
          {generate.isPending ? t('common:buttons.generating') : t('pages:reports.generate')}
        </Button>
      </div>
    </Card>
  );
}
