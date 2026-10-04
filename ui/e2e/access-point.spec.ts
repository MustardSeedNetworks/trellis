import { expect, test } from '@playwright/test';
import { createSurvey, uniqueName, walkThreePoints } from './helpers';

/**
 * Where one access point serves (UI-TRL-17, trellis#702). The daemon draws a
 * single AP's map when the request names its BSSID (T-C24); this walks the
 * operator's route to it and asserts the request the real daemon was sent,
 * then that it drew from that AP's readings rather than the strongest AP's.
 *
 * The scripted radio hears three BSSs at every point: 02:…:01 fading from
 * -48 dBm, 02:…:02 steady at -62 dBm, and an unnamed 02:…:03 at -79 dBm.
 */
const STEADY_AP = '02:00:00:00:00:02';

test('draws the map from the one access point chosen', async ({ page }) => {
  await createSurvey(page, uniqueName('One AP'));
  await walkThreePoints(page);
  await page.getByTestId('survey-complete').click();
  await page.getByTestId('plot-coverage').click();
  await expect(page.getByTestId('heatmap-image')).toBeVisible();

  const select = page.getByTestId('coverage-access-point');
  await expect(select.getByRole('option')).toHaveText([
    'All access points',
    /^Trellis Lab · channel 36 · 02:00:00:00:00:01$/,
    /^Trellis Lab · channel 6 · 02:00:00:00:00:02$/,
    /^Hidden network · channel 100 · 02:00:00:00:00:03$/,
  ]);

  // Reached from the keyboard: one Shift+Tab back from the threshold field.
  await page.getByTestId('coverage-threshold').focus();
  await page.keyboard.press('Shift+Tab');
  await expect(select).toBeFocused();
  await expect(page.getByRole('combobox', { name: 'Access point' })).toBeFocused();

  const heatmapRequest = page.waitForRequest(
    (request) =>
      request.url().endsWith('/trellis.survey.v1.SurveyService/GetHeatmap') &&
      request.postDataJSON()?.bssid === STEADY_AP,
  );
  await select.selectOption(STEADY_AP);
  expect((await heatmapRequest).postDataJSON()).toMatchObject({ metric: 'rssi' });

  // A steady -62 dBm everywhere is a flat field; the strongest-AP map it
  // replaced ranged up to -48. Reading the meta proves the daemon answered
  // about this AP, which a request that merely carried the field would not.
  await expect(page.getByTestId('surface-meta')).toContainText('-62.0 dBm to -62.0 dBm');
  await expect(page.getByTestId('heatmap-image')).toBeVisible();
  await expect(select).toHaveValue(STEADY_AP);
});
