import { expect, test } from '@playwright/test';
import { createSurvey, uniqueName } from './helpers';

/**
 * A building has more than one storey, and until #342 the product could only
 * browse floors an import had created. This walks the operator's own route:
 * open a survey, add the second floor, move the walk onto it, and measure it.
 *
 * The assertion that matters is the last one — the point lands on the floor
 * that was switched to. A rail that lists a floor it cannot collect onto would
 * pass every step before it.
 */
test('adds a floor, walks it, and lists both floors on Coverage', async ({ page }) => {
  const name = uniqueName('Floors');
  await createSurvey(page, name);

  // The survey opens with one floor, being walked.
  await expect(page.getByTestId('floor-rail').getByRole('listitem')).toHaveCount(1);

  await page.getByTestId('floor-name-input').fill('Mezzanine');
  await page.getByTestId('floor-level-input').fill('2');
  await page.getByTestId('create-floor').click();

  const rows = page.getByTestId('floor-rail').getByRole('listitem');
  await expect(rows).toHaveCount(2);
  await expect(rows.nth(1)).toContainText('Mezzanine');
  // Adding a floor does not move the walk onto it.
  await expect(rows.nth(1)).toContainText('Walk this floor');

  const mezzanine = rows.nth(1);
  await mezzanine.getByRole('button', { name: 'Walk this floor' }).click();
  await expect(mezzanine).toContainText('Walking');

  await page.getByTestId('survey-start').click();
  const surface = page.getByTestId('capture-surface');
  await expect(surface).toBeEnabled();
  const box = await surface.boundingBox();
  if (!box) {
    throw new Error('capture surface has no layout box');
  }
  await surface.click({ position: { x: box.width * 0.4, y: box.height * 0.4 } });
  await expect(page.getByTestId('capture-pin')).toHaveCount(1);

  // The point is on the mezzanine, not on the floor the survey opened with.
  await expect(page.getByTestId('capture-count')).toHaveText('1 point on this floor');
  await expect(rows.nth(1)).toContainText('1 sample');
  await expect(rows.nth(0)).toContainText('0 samples');

  await page.getByTestId('plot-coverage').click();
  await expect(page.getByTestId('coverage-floor')).toContainText('Mezzanine');
});
