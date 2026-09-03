import { expect, test } from '@playwright/test';
import { createSurvey, uniqueName, walkThreePoints } from './helpers';

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
 */

test('walks a survey and plots coverage from its stored points', async ({ page }) => {
  const name = uniqueName('Walk');
  await createSurvey(page, name);
  await walkThreePoints(page);
  await expect(page.getByTestId('capture-status')).toContainText('3 networks at');

  // The pins are the stored points, so they survive leaving and coming back.
  // A reload is the harshest version of that: nothing client-side survives it.
  await page.reload();
  const row = page.getByTestId('survey-row').filter({ hasText: name });
  await row.click();
  await expect(page.getByTestId('capture-pin')).toHaveCount(3);
  await expect(page.getByTestId('capture-count')).toHaveText('3 points on this floor');

  await page.getByTestId('survey-complete').click();
  await expect(row).toContainText('Completed · 1 floor · 3 samples');
  // Completed: the walk is still drawn, but no longer accepts a point.
  await expect(page.getByTestId('capture-pin')).toHaveCount(3);
  await expect(page.getByTestId('capture-surface')).toBeDisabled();

  // Follow the link rather than typing /coverage: a bare /coverage analyses
  // whichever survey the list puts first, and the other project's surveys are
  // in the same store.
  await page.getByTestId('plot-coverage').click();
  await expect(page).toHaveURL(/\/coverage\?survey=/);
  await expect(page.getByTestId('heatmap-image')).toBeVisible();
  await expect(page.getByTestId('surface-meta')).toContainText('3 samples');
});

test('deletes a survey only after confirmation', async ({ page }) => {
  const name = uniqueName('Discard');
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
