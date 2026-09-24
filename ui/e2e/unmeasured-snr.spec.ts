import { expect, test } from '@playwright/test';
import { uniqueName } from './helpers';

/**
 * A survey with no measured noise floor has signal and no SNR (#600) — the
 * normal case for every Linux and Windows walk. Its SNR layer used to read as
 * the service failing: "Coverage analysis is not arriving" in the findings and
 * "The heatmap did not render" on the surface (trellis#607).
 *
 * The survey is imported from an AirMagnet capture with no NoiseDBM column
 * rather than walked: the scripted radio reports a floor for its measured APs,
 * and this is the real import path an operator's floor-less archive takes.
 */
test('an unmeasured SNR layer reads as a fact about the survey', async ({ page }) => {
  const name = uniqueName('No noise floor');
  await page.goto('/import');
  await page.getByTestId('amp-file-input').setInputFiles('e2e/fixtures/no-noise-floor.svd');
  await page.locator('#survey-name').fill(name);
  await page.getByRole('button', { name: 'Import survey' }).click();
  await expect(page.getByTestId('status-rollup')).toContainText(`Imported ${name}`);

  await page.goto('/');
  await page.getByTestId('survey-row').filter({ hasText: name }).click();
  await page.getByTestId('plot-coverage').click();

  // Signal was measured, so the RSSI layer draws as it always did.
  await expect(page.getByTestId('heatmap-image')).toBeVisible();
  const findings = page.getByTestId('coverage-findings');
  await expect(findings).toHaveAttribute('data-state', 'ok');

  await page.getByRole('button', { name: 'SNR' }).click();

  await expect(findings).toContainText('No SNR measured on this floor');
  await expect(findings).toContainText('reports no noise floor');
  await expect(findings).not.toContainText('not arriving');
  await expect(findings).toHaveAttribute('data-state', 'unknown');

  const surface = page.getByTestId('surface-message');
  await expect(surface).toContainText('No SNR measured on this floor');
  await expect(surface).not.toContainText('did not render');
  await expect(page.getByTestId('heatmap-image')).toHaveCount(0);
});
