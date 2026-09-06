/**
 * Import — the AirMapper and AirMagnet ingest page.
 *
 * The capability existed before the page did: a button in the Surveys rail
 * that asked for the survey name with window.prompt. These tests pin what the
 * page has to do that the button could not — name the file before committing
 * to it, refuse an empty name without throwing away the chosen file, and say
 * what happened afterwards.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ImportPage } from './ImportPage';

const importAirMapper = vi.fn();
const importAirMagnet = vi.fn();

vi.mock('@/lib/client', () => ({
  surveyClient: {
    importAirMapper: (req: unknown) => importAirMapper(req),
    importAirMagnet: (req: unknown) => importAirMagnet(req),
  },
}));

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

function renderPage() {
  return render(<ImportPage />, { wrapper });
}

/**
 * The rollup is the shared StatusRollup, which carries no test id — it is the
 * page's live region, so the test finds it the way a screen reader would
 * rather than by adding a hook to a component four products share.
 */
function rollup(): HTMLElement {
  const el = document.querySelector<HTMLElement>('section[aria-live="polite"][data-state]');
  if (el === null) {
    throw new Error('status rollup not rendered');
  }
  return el;
}

/** A survey file the browser will hand to the input. */
function ampFile(name = 'office-3f.amp') {
  return new File([new Uint8Array([1, 2, 3])], name, { type: 'application/octet-stream' });
}

/** choose drives the hidden file input the way the browser would. */
function choose(name?: string) {
  const input = screen.getByTestId('amp-file-input');
  fireEvent.change(input, { target: { files: [ampFile(name)] } });
}

/** setName replaces the survey-name field's contents. */
function setName(value: string) {
  fireEvent.change(screen.getByLabelText(/survey name/i), { target: { value } });
}

function clickImport() {
  fireEvent.click(screen.getByRole('button', { name: /import survey/i }));
}

describe('ImportPage', () => {
  beforeEach(() => {
    importAirMagnet.mockReset();
    importAirMagnet.mockResolvedValue({
      survey: { id: 'svy-2', name: 'lab', status: 'ready', sampleCount: 96, floorCount: 1 },
    });
    importAirMapper.mockReset();
    importAirMapper.mockResolvedValue({
      survey: { id: 'svy-1', name: 'office-3f', status: 'ready', sampleCount: 412, floorCount: 1 },
    });
  });

  it('says nothing has been chosen before a file is picked', () => {
    renderPage();

    expect(rollup()).toHaveTextContent(/no survey file chosen/i);
    expect(screen.getByRole('button', { name: /import survey/i })).toBeDisabled();
  });

  // window.prompt could not do this: the name was demanded before the file was
  // even read, and cancelling threw the chosen file away.
  it('proposes the file name as the survey name, without the extension', async () => {
    renderPage();

    choose('office-3f.amp');

    expect(await screen.findByLabelText(/survey name/i)).toHaveValue('office-3f');
  });

  it('refuses an empty name but keeps the chosen file', async () => {
    renderPage();

    choose();
    await screen.findByLabelText(/survey name/i);
    setName('');

    expect(screen.getByRole('button', { name: /import survey/i })).toBeDisabled();
    expect(rollup()).toHaveTextContent(/office-3f\.amp/);
    expect(importAirMapper).not.toHaveBeenCalled();
  });

  it('sends the file bytes and the chosen name', async () => {
    renderPage();

    choose();
    await screen.findByLabelText(/survey name/i);
    setName('Floor 3 walkthrough');
    clickImport();

    await waitFor(() => expect(importAirMapper).toHaveBeenCalledTimes(1));
    const [req] = importAirMapper.mock.calls[0] as [{ name: string; ampData: Uint8Array }];
    expect(req.name).toBe('Floor 3 walkthrough');
    expect(Array.from(req.ampData)).toEqual([1, 2, 3]);
  });

  it('reports what the archive yielded when the import succeeds', async () => {
    renderPage();

    choose();
    await screen.findByLabelText(/survey name/i);
    clickImport();

    await waitFor(() => expect(rollup()).toHaveTextContent(/svy-1/));
    // An import that stored nothing still "succeeds"; the counts are what say
    // whether it was worth anything.
    expect(rollup()).toHaveTextContent('412');
  });

  // The two formats are one act to the operator, so the extension — which they
  // already chose in the file dialog — picks the call rather than a second
  // control asking which vendor wrote the file in front of them.
  it('sends an .svd to the AirMagnet import, not the AirMapper one', async () => {
    renderPage();

    choose('vofi-walk.svd');
    await screen.findByLabelText(/survey name/i);
    clickImport();

    await waitFor(() => expect(importAirMagnet).toHaveBeenCalledTimes(1));
    const [req] = importAirMagnet.mock.calls[0] as [{ name: string; svdData: Uint8Array }];
    expect(req.name).toBe('vofi-walk');
    expect(Array.from(req.svdData)).toEqual([1, 2, 3]);
    expect(importAirMapper).not.toHaveBeenCalled();
  });

  // An .svd holds measurements and no plan, and a survey that arrives with
  // nothing behind its points looks broken unless the page said so first.
  it('warns that an AirMagnet export brings no floor plan, before importing', async () => {
    renderPage();

    choose('vofi-walk.svd');

    await waitFor(() => expect(rollup()).toHaveTextContent(/carries no floor plan/i));
  });

  // A failed import that renders as a calm empty form reads as "nothing
  // happened", which is the one thing the rollup exists to prevent.
  it('says the import failed, with the reason the service gave', async () => {
    importAirMapper.mockRejectedValue(new Error('archive is not an AirMapper export'));
    renderPage();

    choose();
    await screen.findByLabelText(/survey name/i);
    clickImport();

    await waitFor(() => expect(rollup()).toHaveTextContent(/archive is not an AirMapper export/i));
    expect(rollup()).toHaveAttribute('data-state', 'crit');
  });
});
