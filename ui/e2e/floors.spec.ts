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

/**
 * TR-1's refusal, and the route it was written to have and could not build
 * (#337, #342). A plan of different dimensions over a floor that already holds
 * measurements is refused — every stored point is a pixel on the plan it was
 * walked against — and the answer is that the plan usually belongs to a
 * different storey. So the panel offers to put it on one.
 *
 * Round trip, not mocks: the refusal comes from the daemon, and the new floor
 * and its plan have to come back from it too.
 */
test('puts a plan that would strand the measurements on a floor of its own', async ({ page }) => {
  await createSurvey(page, uniqueName('Strand'));

  await page.getByTestId('floor-plan-input').setInputFiles('e2e/fixtures/ninth-floor.png');
  await expect(page.getByTestId('floor-plan-status')).toContainText('no scale yet');

  // A measurement on this plan is what makes the replacement dangerous.
  await page.getByTestId('survey-start').click();
  const surface = page.getByTestId('capture-surface');
  await expect(surface).toBeEnabled();
  const box = await surface.boundingBox();
  if (!box) {
    throw new Error('capture surface has no layout box');
  }
  await surface.click({ position: { x: box.width * 0.4, y: box.height * 0.4 } });
  await expect(page.getByTestId('capture-pin')).toHaveCount(1);

  // A 1024x768 plan over a floor walked against an 800x600 one.
  await page.getByTestId('floor-plan-input').setInputFiles('e2e/fixtures/tenth-floor.png');
  await expect(page.getByTestId('floor-plan-status')).toContainText('strand the measurements');
  await expect(page.getByTestId('floor-plan-stranded-hint')).toContainText('add it as a new floor');

  await page.getByTestId('strand-new-floor-name').fill('Tenth');
  await page.getByTestId('strand-new-floor').click();

  const rows = page.getByTestId('floor-rail').getByRole('listitem');
  await expect(rows).toHaveCount(2);
  await expect(rows.nth(1)).toContainText('Tenth');
  // The refusal is gone and the offer with it: the plan found a home, and the
  // walked floor still has the plan and the point it was measured against.
  await expect(page.getByTestId('strand-new-floor')).toHaveCount(0);
  await expect(page.getByTestId('capture-pin')).toHaveCount(1);
  await expect(rows.nth(0)).toContainText('1 sample');
});
