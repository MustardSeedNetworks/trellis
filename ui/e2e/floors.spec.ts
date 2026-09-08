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

/**
 * The other half of #342: a floor can be renamed, and deleted along with the
 * measurements walked on it. The delete is two clicks for the same reason the
 * survey delete is — the readings go with the floor.
 */
test('renames a floor, then deletes it with its measurements', async ({ page }) => {
  const name = uniqueName('FloorEdit');
  await createSurvey(page, name);

  await page.getByTestId('floor-name-input').fill('Mezzanine');
  await page.getByTestId('floor-level-input').fill('2');
  await page.getByTestId('create-floor').click();

  const rows = page.getByTestId('floor-rail').getByRole('listitem');
  await expect(rows).toHaveCount(2);

  // Walk the new floor so the delete has something to take with it.
  await rows.nth(1).getByRole('button', { name: 'Walk this floor' }).click();
  await expect(rows.nth(1)).toContainText('Walking');
  await page.getByTestId('survey-start').click();
  const surface = page.getByTestId('capture-surface');
  await expect(surface).toBeEnabled();
  const box = await surface.boundingBox();
  if (!box) {
    throw new Error('capture surface has no layout box');
  }
  await surface.click({ position: { x: box.width * 0.4, y: box.height * 0.4 } });
  await expect(page.getByTestId('capture-pin')).toHaveCount(1);
  await expect(rows.nth(1)).toContainText('1 sample');

  const mezzanineRow = page.getByTestId('floor-rail').getByRole('listitem').nth(1);
  const floorId = (await mezzanineRow.getAttribute('data-testid'))?.replace('floor-row-', '');
  if (!floorId) {
    throw new Error('the mezzanine row carries no floor id');
  }

  await page.getByTestId(`rename-floor-${floorId}`).click();
  await page.getByTestId(`floor-rename-input-${floorId}`).fill('Upper mezzanine');
  await page.getByTestId(`floor-relevel-input-${floorId}`).fill('3');
  await page.getByTestId(`floor-rename-save-${floorId}`).click();
  await expect(mezzanineRow).toContainText('Upper mezzanine');
  // A rename is metadata: the reading walked on the floor is still there.
  await expect(mezzanineRow).toContainText('1 sample');

  // One click arms, the second deletes.
  await page.getByTestId(`delete-floor-${floorId}`).click();
  await expect(page.getByTestId(`delete-floor-confirm-${floorId}`)).toBeVisible();
  await page.getByTestId(`delete-floor-confirm-${floorId}`).click();

  await expect(rows).toHaveCount(1);
  // And the walk came back to the survivor, which now holds every measurement
  // the survey has — none, because the only reading was on the deleted floor.
  await expect(rows.nth(0)).toContainText('Walking');
  await expect(rows.nth(0)).toContainText('0 samples');
  // The last floor offers no delete: the handler refuses it.
  await expect(page.getByTestId(/^delete-floor-/)).toHaveCount(0);
});
