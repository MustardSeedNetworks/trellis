import { expect, test } from '@playwright/test';
import { createSurvey, uniqueName, walkThreePoints } from './helpers';

/**
 * The three journeys that are not the walk: bring a capture in, tune the
 * analysis, take a deliverable out. Each crosses the daemon/UI seam where the
 * product's defects have lived, and each asserts a value the service computed
 * rather than that a control rendered.
 */

test('imports an AirMapper archive as a survey with its floor plan', async ({ page }) => {
  const name = uniqueName('Imported');
  await page.goto('/import');

  // The input is hidden behind a button; setInputFiles drives it directly.
  await page.getByTestId('amp-file-input').setInputFiles('e2e/fixtures/plan-only.amp');
  // The filename is proposed as the name; the operator renames before committing.
  await expect(page.locator('#survey-name')).toHaveValue('plan-only');
  await page.locator('#survey-name').fill(name);
  await page.getByRole('button', { name: 'Import survey' }).click();

  const rollup = page.getByTestId('status-rollup');
  await expect(rollup).toContainText(`Imported ${name}`);
  await expect(rollup).toHaveAttribute('data-state', 'ok');
  // Plan-only fixture: zero samples, one floor recovered from the PNG. The
  // figures are asserted by value; a rollup that printed the wrong survey's
  // numbers would still be "a rollup that rendered".
  await expect(rollup.locator('dd')).toHaveText(['0', '1']);

  await page.goto('/');
  const row = page.getByTestId('survey-row').filter({ hasText: name });
  await expect(row).toContainText('1 floor · 0 samples');
  await row.click();
  await expect(page.getByTestId('survey-detail')).toContainText('Present');
});

test('changing the dead-zone threshold changes the verdict', async ({ page }) => {
  await createSurvey(page, uniqueName('Threshold'));
  await walkThreePoints(page);
  await page.getByTestId('plot-coverage').click();

  const findings = page.getByTestId('coverage-findings');
  await expect(page.getByTestId('heatmap-image')).toBeVisible();
  // The scripted radio's strongest AP cycles -48, -54, -60, -66 dBm, all above
  // the default -75 dBm floor.
  await expect(findings).toContainText('No dead zones below -75 dBm');
  await expect(findings).toHaveAttribute('data-state', 'ok');

  // Raise the floor above every measured point: every point is now a gap.
  await page.locator('#coverage-threshold').fill('-40');
  await expect(findings).toContainText('below -40 dBm');
  await expect(findings).toHaveAttribute('data-state', 'warn');
});

test('generates a PDF report for a walked survey', async ({ page }) => {
  const name = uniqueName('Report');
  await createSurvey(page, name);
  await walkThreePoints(page);
  await page.getByTestId('survey-complete').click();

  await page.goto('/reports');
  await page.locator('#report-survey').selectOption({ label: name });
  await page.locator('#report-company').fill('Mustard Seed Networks');

  const downloadPromise = page.waitForEvent('download');
  await page.getByTestId('generate-report').click();
  const download = await downloadPromise;

  expect(download.suggestedFilename()).toMatch(/^report-.*-survey-report\.pdf$/);
  const stream = await download.createReadStream();
  const head = await new Promise<Buffer>((resolve, reject) => {
    const chunks: Buffer[] = [];
    stream.on('data', (chunk: Buffer) => chunks.push(chunk));
    stream.on('end', () => resolve(Buffer.concat(chunks)));
    stream.on('error', reject);
  });
  // A real PDF from the generator, not an empty or HTML error body.
  expect(head.subarray(0, 5).toString()).toBe('%PDF-');
  expect(head.length).toBeGreaterThan(1024);
});

test('reads a measured value off the heatmap and zooms it', async ({ page }) => {
  await createSurvey(page, uniqueName('Readout'));
  await walkThreePoints(page);
  await page.getByTestId('plot-coverage').click();

  const image = page.getByTestId('heatmap-image');
  await expect(image).toBeVisible();

  const readout = page.getByTestId('heatmap-readout');
  await expect(readout).toContainText('Point at the surface');

  // Hover the middle of the surface. The value comes from the grid the daemon
  // painted with, so this asserts a real dBm reading rather than that a
  // tooltip appeared: the scripted radio's APs run -48 to -66 dBm.
  const box = await image.boundingBox();
  if (!box) {
    throw new Error('heatmap image has no layout box');
  }
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await expect(readout).toHaveText(/-\d+\.\d dBm at \d+, \d+/);

  const viewport = page.getByTestId('heatmap-viewport');
  await expect(viewport).toHaveAttribute('data-zoom', '1');
  await expect(page.getByTestId('zoom-reset')).toBeDisabled();

  // Keyboard, not a click: the controls exist so zoom is reachable without a
  // mouse, and a button that only responds to a pointer would still pass a
  // click-driven check.
  await page.getByTestId('zoom-in').focus();
  await page.keyboard.press('Enter');
  await page.keyboard.press('Enter');
  await expect(viewport).toHaveAttribute('data-zoom', '1.5');
  await expect(page.getByTestId('zoom-level')).toHaveText('150%');

  await page.getByTestId('zoom-reset').click();
  await expect(viewport).toHaveAttribute('data-zoom', '1');
});

test('reads the live airspace and stops taking the radio when paused', async ({ page }) => {
  await page.goto('/live');

  const rollup = page.getByTestId('status-rollup');
  // The scripted radio is joined to Trellis Lab, whose SNR fades 47 → 29 dB
  // across the cycle and stays above the 20 dB the page calls weak.
  await expect(rollup).toContainText('Connected to Trellis Lab');
  await expect(rollup).toHaveAttribute('data-state', 'ok');

  const rows = page.getByTestId('neighbour-row');
  await expect(rows).toHaveCount(3);

  // Identified by the association, not by position: the scripted radio fades
  // its strongest AP from -48 to -66 dBm, so the joined BSS is the top row for
  // three of every four scans and the second row on the fourth. An assertion on
  // rows.first() passes or fails on which scan the page happened to be showing.
  const associated = page.locator('[data-testid="neighbour-row"][data-associated="true"]');
  await expect(associated).toHaveCount(1);
  await expect(associated).toContainText('Trellis Lab');
  await expect(associated).toContainText('18%');

  // Strongest first, read off the rendered values rather than assumed from the
  // fixture — the ordering is the server's and this is what checks it.
  const dbm = await rows.evaluateAll((tr) =>
    tr.map((row) => Number.parseFloat(row.querySelectorAll('td')[3]?.textContent ?? '')),
  );
  expect(dbm).toEqual([...dbm].sort((a, b) => b - a));

  // The weakest BSS broadcasts no SSID and sits on a DFS channel. It is last
  // under every fade step, since it never moves.
  await expect(rows.last()).toContainText('Hidden network');
  await expect(rows.last()).toContainText('(DFS)');
  // An AP that sent no BSS Load element must not read as an idle channel.
  await expect(rows.last()).toContainText('Not reported');

  // Pausing has to stop the polling, not just relabel the button: the same
  // adapter is what a walk captures with. The joined AP is the one that fades,
  // so its reading is what would move if a poll got through.
  const signal = associated.locator('td').nth(3);
  const held = await signal.textContent();
  await page.getByTestId('toggle-polling').click();
  await expect(page.getByTestId('toggle-polling')).toHaveText('Resume scanning');
  await page.waitForTimeout(6000);
  await expect(signal).toHaveText(held ?? '');
});

test('measures throughput at a point and maps it as its own layer', async ({ page }) => {
  const name = uniqueName('Throughput');
  await createSurvey(page, name);

  // No target, no button: the remedy is to name a server, which a click cannot
  // discover.
  await expect(page.getByTestId('measure-throughput')).toHaveCount(0);
  await page.getByTestId('throughput-target').fill('10.44.10.9');
  await page.getByTestId('save-throughput-target').click();
  await expect(page.getByTestId('measure-throughput')).toBeVisible();

  await page.getByTestId('survey-start').click();
  const surface = page.getByTestId('capture-surface');
  await expect(surface).toBeEnabled();

  // A passive point beside an active one, so the layers have to tell them
  // apart rather than rendering everything they hold.
  const box = await surface.boundingBox();
  if (!box) {
    throw new Error('capture surface has no layout box');
  }
  await surface.click({ position: { x: box.width * 0.25, y: box.height * 0.3 } });
  await expect(page.getByTestId('capture-pin')).toHaveCount(1);

  await page.getByTestId('measure-throughput').click();
  await expect(page.getByTestId('capture-status')).toContainText('221.4 Mbps down, 88.2 Mbps up');
  await expect(page.getByTestId('capture-pin')).toHaveCount(2);

  await page.getByTestId('survey-complete').click();
  await page.getByTestId('plot-coverage').click();
  await expect(page.getByTestId('heatmap-image')).toBeVisible();

  // The download layer holds the one measured point, not both: a passive scan
  // measured no throughput, and rendering its signal on this layer would put
  // dBm on a map of Mbps.
  await page.getByRole('button', { name: 'Download' }).click();
  await expect(page.getByTestId('surface-meta')).toContainText('download');
  await expect(page.getByTestId('surface-meta')).toContainText('1 sample');

  // The dead-zone threshold and its findings speak about dBm. Over a
  // throughput layer they would answer a question nobody asked.
  await expect(page.locator('#coverage-threshold')).toHaveCount(0);
  await expect(page.getByTestId('coverage-findings')).toHaveCount(0);
});
