/**
 * Reports exists because the engine reads five options the API hardcoded, so
 * the thing worth pinning is that the operator's choices actually reach the
 * request — a page full of controls that always sends the defaults would look
 * identical and be worthless.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ReportsPage } from './ReportsPage';

const listSurveys = vi.fn();
const generateReport = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    listSurveys: (req: unknown) => listSurveys(req),
    generateReport: (req: unknown) => generateReport(req),
  },
}));

const survey = {
  id: 'svy-1',
  name: 'Everett HQ',
  status: 'completed',
  floorCount: 1,
  sampleCount: 42,
  hasFloorPlan: true,
};

/** The button stays disabled until the survey list arrives, so every test
    waits for it rather than clicking into a page that cannot act yet. */
async function readyButton(): Promise<HTMLElement> {
  const button = await screen.findByTestId('generate-report');
  await waitFor(() => expect(button).not.toBeDisabled());
  return button;
}

function renderPage(initialPath = '/reports') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={[initialPath]}>{children}</MemoryRouter>
      </QueryClientProvider>
    );
  }
  return render(<ReportsPage />, { wrapper });
}

beforeEach(() => {
  listSurveys.mockReset();
  generateReport.mockReset();
  listSurveys.mockResolvedValue({ surveys: [survey] });
  generateReport.mockResolvedValue({ pdf: new Uint8Array([0x25, 0x50, 0x44, 0x46]) });
});

describe('ReportsPage', () => {
  it('sends the engine defaults when nothing is changed', async () => {
    renderPage();
    fireEvent.click(await readyButton());

    await waitFor(() => expect(generateReport).toHaveBeenCalled());
    expect(generateReport.mock.calls[0]?.[0]).toMatchObject({
      surveyId: 'svy-1',
      options: {
        includeExecutiveSummary: true,
        includeRecommendations: true,
        includeHeatmaps: true,
        includeRawData: false,
      },
    });
  });

  it('sends a section the operator turned off as off', async () => {
    renderPage();
    const button = await readyButton();

    fireEvent.click(screen.getByLabelText(/Floor heatmaps/));
    fireEvent.click(button);

    await waitFor(() => expect(generateReport).toHaveBeenCalled());
    expect(generateReport.mock.calls[0]?.[0].options.includeHeatmaps).toBe(false);
  });

  it('sends the company name that goes on the cover', async () => {
    renderPage();
    const button = await readyButton();

    fireEvent.change(screen.getByLabelText(/Company name/), {
      target: { value: '  Mustard Seed Networks  ' },
    });
    fireEvent.click(button);

    await waitFor(() => expect(generateReport).toHaveBeenCalled());
    /* Trimmed: a cover page centred on trailing spaces looks like a bug. */
    expect(generateReport.mock.calls[0]?.[0].options.companyName).toBe('Mustard Seed Networks');
  });

  it('reports a failed generation instead of silently doing nothing', async () => {
    generateReport.mockRejectedValue(new Error('no floor plan'));
    renderPage();
    fireEvent.click(await readyButton());

    await waitFor(() =>
      expect(screen.getByTestId('report-error')).toHaveTextContent('no floor plan'),
    );
  });

  it('reports on the survey the URL names', async () => {
    listSurveys.mockResolvedValue({ surveys: [survey, { ...survey, id: 'svy-2', name: 'Depot' }] });
    renderPage('/reports?survey=svy-2');
    fireEvent.click(await readyButton());

    await waitFor(() => expect(generateReport).toHaveBeenCalled());
    expect(generateReport.mock.calls[0]?.[0].surveyId).toBe('svy-2');
  });
});
