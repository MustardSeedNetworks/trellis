import { expect, type Page, test } from '@playwright/test';

/**
 * The measured walk, end to end through the daemon.
 *
 * Create a survey, start it, capture three points, complete it, and plot
 * coverage from what was stored. Every step crosses the daemon/UI seam, and
 * the heatmap at the end is rendered by core/survey from the points the
 * clicks stored — the unit suites on either side cannot see that path.
 *
 * The radio is scripted (cmd/trellisd/scanner_e2e.go, built with -tags e2e):
 * the runners have no adapter. Everything after the scan is the real thing.
 *
 * Names carry a nonce because chromium and webkit share one daemon and one
 * survey store, and the list has no order.
 */

async function createSurvey(page: Page, name: string) {
  await page.goto('/');
  await page.getByTestId('new-survey-name').fill(name);
  await page.getByTestId('create-survey').click();
  const detail = page.getByTestId('survey-detail');
  await expect(detail).toBeVisible();
  await expect(detail).toContainText('Created');
  return detail;
}

test('walks a survey and plots coverage from its stored points', async ({ page }) => {
  const name = `Walk ${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  await createSurvey(page, name);

  await page.getByTestId('survey-start').click();
  const surface = page.getByTestId('capture-surface');
  await expect(surface).toBeVisible();

  // Three distinct positions: a heatmap needs spread to interpolate over, and
  // three coincident points would be a degenerate field.
  const positions = [
    { x: 0.2, y: 0.3 },
    { x: 0.5, y: 0.55 },
    { x: 0.8, y: 0.75 },
  ];
  for (const [index, position] of positions.entries()) {
    const box = await surface.boundingBox();
    if (!box) {
      throw new Error('capture surface has no layout box');
    }
    await surface.click({
      position: { x: box.width * position.x, y: box.height * position.y },
    });
    // Wait for the point to land, not for the click: the next click is
    // ignored while a scan is in flight, by design.
    await expect(page.getByTestId('capture-pin')).toHaveCount(index + 1);
  }
  await expect(page.getByTestId('capture-status')).toContainText('3 networks at');

  await page.getByTestId('survey-complete').click();
  const row = page.getByTestId('survey-row').filter({ hasText: name });
  await expect(row).toContainText('Completed · 1 floor · 3 samples');
  await expect(page.getByTestId('capture-surface')).toHaveCount(0);

  // Follow the link rather than typing /coverage: a bare /coverage analyses
  // whichever survey the list puts first, and the other project's surveys are
  // in the same store.
  await page.getByTestId('plot-coverage').click();
  await expect(page).toHaveURL(/\/coverage\?survey=/);
  await expect(page.getByTestId('heatmap-image')).toBeVisible();
  await expect(page.getByTestId('surface-meta')).toContainText('3 samples');
});

test('deletes a survey only after confirmation', async ({ page }) => {
  const name = `Discard ${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  await createSurvey(page, name);
  const row = page.getByTestId('survey-row').filter({ hasText: name });
  await expect(row).toHaveCount(1);

  await page.getByTestId('survey-delete').click();
  await page.getByTestId('survey-delete-cancel').click();
  await expect(row).toHaveCount(1);

  await page.getByTestId('survey-delete').click();
  await page.getByTestId('survey-delete-confirm').click();
  await expect(row).toHaveCount(0);
  await expect(page.getByTestId('survey-detail')).toHaveCount(0);
});
