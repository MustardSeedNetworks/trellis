/**
 * The service refuses a transition its state does not allow. The component's
 * job is to never offer one, so the assertions are about which buttons exist
 * in each state as much as about what a click sends.
 */
import { create } from '@bufbuild/protobuf';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SurveySummarySchema } from '@/gen/trellis/survey/v1/survey_pb';
import { SurveyLifecycle } from './SurveyLifecycle';

const startSurvey = vi.fn();
const pauseSurvey = vi.fn();
const completeSurvey = vi.fn();
const deleteSurvey = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    startSurvey: (req: unknown) => startSurvey(req),
    pauseSurvey: (req: unknown) => pauseSurvey(req),
    completeSurvey: (req: unknown) => completeSurvey(req),
    deleteSurvey: (req: unknown) => deleteSurvey(req),
  },
}));

function survey(status: string) {
  return create(SurveySummarySchema, { id: 'svy-1', name: 'Everett HQ', status, floorCount: 1 });
}

function renderLifecycle(status: string, onDeleted = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  }
  render(<SurveyLifecycle survey={survey(status)} onDeleted={onDeleted} />, { wrapper });
  return onDeleted;
}

beforeEach(() => {
  for (const fn of [startSurvey, pauseSurvey, completeSurvey, deleteSurvey]) {
    fn.mockReset();
    fn.mockResolvedValue({});
  }
});

describe('SurveyLifecycle', () => {
  it('offers only Start on a created survey', () => {
    renderLifecycle('created');
    expect(screen.getByTestId('survey-start')).toHaveTextContent('Start walk');
    expect(screen.queryByTestId('survey-pause')).not.toBeInTheDocument();
    expect(screen.queryByTestId('survey-complete')).not.toBeInTheDocument();
  });

  it('offers Pause and Complete while walking, and neither Start', () => {
    renderLifecycle('in_progress');
    expect(screen.queryByTestId('survey-start')).not.toBeInTheDocument();
    expect(screen.getByTestId('survey-pause')).toBeInTheDocument();
    expect(screen.getByTestId('survey-complete')).toBeInTheDocument();
  });

  it('offers Resume and Complete when paused', () => {
    renderLifecycle('paused');
    expect(screen.getByTestId('survey-start')).toHaveTextContent('Resume walk');
    expect(screen.queryByTestId('survey-pause')).not.toBeInTheDocument();
    expect(screen.getByTestId('survey-complete')).toBeInTheDocument();
  });

  it('offers no transition on a completed survey, only delete', () => {
    renderLifecycle('completed');
    expect(screen.queryByTestId('survey-start')).not.toBeInTheDocument();
    expect(screen.queryByTestId('survey-pause')).not.toBeInTheDocument();
    expect(screen.queryByTestId('survey-complete')).not.toBeInTheDocument();
    expect(screen.getByTestId('survey-delete')).toBeInTheDocument();
  });

  it('sends each transition for this survey', async () => {
    renderLifecycle('in_progress');
    fireEvent.click(screen.getByTestId('survey-pause'));
    await waitFor(() => expect(pauseSurvey).toHaveBeenCalledWith({ id: 'svy-1' }));
    fireEvent.click(screen.getByTestId('survey-complete'));
    await waitFor(() => expect(completeSurvey).toHaveBeenCalledWith({ id: 'svy-1' }));
  });

  it('deletes only after the confirmation step, and can be talked out of it', async () => {
    const onDeleted = renderLifecycle('completed');

    fireEvent.click(screen.getByTestId('survey-delete'));
    expect(deleteSurvey).not.toHaveBeenCalled();
    fireEvent.click(screen.getByTestId('survey-delete-cancel'));
    expect(screen.queryByTestId('survey-delete-confirm')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('survey-delete'));
    fireEvent.click(screen.getByTestId('survey-delete-confirm'));
    await waitFor(() => expect(deleteSurvey).toHaveBeenCalledWith({ id: 'svy-1' }));
    await waitFor(() => expect(onDeleted).toHaveBeenCalledTimes(1));
  });

  it('shows a refused transition with the service message', async () => {
    startSurvey.mockRejectedValue(new Error('[failed_precondition] survey already in progress'));
    renderLifecycle('created');

    fireEvent.click(screen.getByTestId('survey-start'));

    expect(await screen.findByTestId('lifecycle-error')).toHaveTextContent(
      'survey already in progress',
    );
  });
});
