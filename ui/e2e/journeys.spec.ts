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
