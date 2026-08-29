/**
 * SurveysPage, SurveyList and SurveyDetail were each at 0% coverage while the
 * package reported 71.97% overall. They are the only route into everything
 * else the product does with a survey.
 *
 * The state this page exists to distinguish is the one worth pinning: a failed
 * load and an empty list both render zero surveys, and the page is written to
 * say which. A test that only counted rows could not tell them apart either.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SurveysPage } from './SurveysPage';

const listSurveys = vi.fn();
const generateReport = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    listSurveys: (req: unknown) => listSurveys(req),
    generateReport: (req: unknown) => generateReport(req),
  },
}));

const everett = {
  id: 'svy-1',
  name: 'Everett HQ',
  status: 'completed',
  floorCount: 2,
  sampleCount: 87,
  hasFloorPlan: true,
};

const vegas = {
  id: 'svy-2',
  name: 'MGM Temp Location',
  status: 'importing',
  floorCount: 1,
  sampleCount: 5,
  hasFloorPlan: false,
};

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={['/surveys']}>{children}</MemoryRouter>
      </QueryClientProvider>
    );
  }
  return render(<SurveysPage />, { wrapper });
}

beforeEach(() => {
  listSurveys.mockReset();
  generateReport.mockReset();
});

describe('SurveysPage', () => {
  it('names each survey and summarises it with real counts', async () => {
    listSurveys.mockResolvedValue({ surveys: [everett, vegas] });
    renderPage();

    expect(await screen.findByText('Everett HQ')).toBeInTheDocument();
    expect(screen.getByText('MGM Temp Location')).toBeInTheDocument();
    expect(screen.getByText('2 surveys available')).toBeInTheDocument();

    // Pluralisation is per-value, so both branches are asserted: one floor and
    // one sample must not read "1 floors".
    expect(screen.getByText('completed · 2 floors · 87 samples')).toBeInTheDocument();
    expect(screen.getByText('importing · 1 floor · 5 samples')).toBeInTheDocument();
  });

  it('distinguishes an empty list from a failed load', async () => {
    listSurveys.mockResolvedValue({ surveys: [] });
    const { unmount } = renderPage();

    expect(await screen.findByText('No surveys captured yet')).toBeInTheDocument();
    expect(
      screen.getByText('Import a survey to build a heatmap and coverage analysis from it.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('Survey data is not arriving')).not.toBeInTheDocument();
    unmount();

    listSurveys.mockRejectedValue(new Error('survey service unreachable'));
    renderPage();

    expect(await screen.findByText('Survey data is not arriving')).toBeInTheDocument();
    // The failure has to name itself, not render as an empty list.
    expect(screen.getByText(/survey service unreachable/)).toBeInTheDocument();
    expect(screen.queryByText('No surveys captured yet')).not.toBeInTheDocument();
  });

  it('shows the selection prompt until a survey is chosen, then its facts', async () => {
    listSurveys.mockResolvedValue({ surveys: [everett] });
    renderPage();

    // Wait for the list, not the prompt: the prompt renders before the query
    // resolves, so clicking on it alone races the survey button into existence.
    await screen.findByText('Everett HQ');
    expect(
      screen.getByText('Select a survey to see what it stores and what it can produce'),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Everett HQ/ }));

    // The detail pane's facts come from the selected survey, so they are
    // asserted by value -- a pane that rendered the wrong survey's numbers
    // would still be "a pane that rendered".
    expect(screen.getByText('87')).toBeInTheDocument();
    expect(screen.getByText('completed')).toBeInTheDocument();
    expect(screen.getByText('Present')).toBeInTheDocument();
    expect(
      screen.queryByText('Select a survey to see what it stores and what it can produce'),
    ).not.toBeInTheDocument();
  });

  it('renders the facts of the survey that was clicked, not the first one', async () => {
    listSurveys.mockResolvedValue({ surveys: [everett, vegas] });
    renderPage();

    await screen.findByText('MGM Temp Location');
    fireEvent.click(screen.getByRole('button', { name: /MGM Temp Location/ }));

    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('importing')).toBeInTheDocument();
    expect(screen.getByText('None')).toBeInTheDocument();
    expect(screen.queryByText('87')).not.toBeInTheDocument();
  });

  it('offers a coverage link for the selected survey', async () => {
    listSurveys.mockResolvedValue({ surveys: [everett] });
    renderPage();

    await screen.findByText('Everett HQ');
    fireEvent.click(screen.getByRole('button', { name: /Everett HQ/ }));

    expect(screen.getByRole('link', { name: 'Plot coverage' })).toHaveAttribute(
      'href',
      '/coverage',
    );
  });
});

describe('SurveysPage report download', () => {
  it('requests the report for the selected survey and hands the browser the PDF', async () => {
    listSurveys.mockResolvedValue({ surveys: [everett, vegas] });
    generateReport.mockResolvedValue({ pdf: new Uint8Array([0x25, 0x50, 0x44, 0x46]) });

    // The download is a transient anchor, so the effect is only observable by
    // intercepting the click. Asserting the request alone would pass for a
    // page that fetched the PDF and dropped it.
    const clicks: Array<{ href: string; download: string }> = [];
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
      this: HTMLAnchorElement,
    ) {
      clicks.push({ href: this.href, download: this.download });
    });

    try {
      renderPage();
      await screen.findByText('MGM Temp Location');
      fireEvent.click(screen.getByRole('button', { name: /MGM Temp Location/ }));
      fireEvent.click(screen.getByRole('button', { name: 'Download PDF report' }));

      await waitFor(() => expect(clicks).toHaveLength(1));
      expect(generateReport).toHaveBeenCalledTimes(1);
      expect(generateReport).toHaveBeenCalledWith({ surveyId: 'svy-2' });
      expect(clicks[0]?.href).toMatch(/^data:application\/pdf/);
      expect(clicks[0]?.download).toMatch(/mgm-temp-location/i);
    } finally {
      clickSpy.mockRestore();
    }
  });

  it('reports a failed report and downloads nothing', async () => {
    listSurveys.mockResolvedValue({ surveys: [everett] });
    generateReport.mockRejectedValue(new Error('renderer out of memory'));

    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    try {
      renderPage();
      await screen.findByText('Everett HQ');
      fireEvent.click(screen.getByRole('button', { name: /Everett HQ/ }));
      fireEvent.click(screen.getByRole('button', { name: 'Download PDF report' }));

      expect(await screen.findByText(/renderer out of memory/)).toBeInTheDocument();
      expect(clickSpy).not.toHaveBeenCalled();
    } finally {
      clickSpy.mockRestore();
    }
  });
});

describe('SurveyList empty state', () => {
  it('says the list is empty rather than rendering nothing', async () => {
    listSurveys.mockResolvedValue({ surveys: [] });
    renderPage();

    expect(
      await screen.findByText('No surveys yet. Import an AirMapper file to get started.'),
    ).toBeInTheDocument();
  });
});
