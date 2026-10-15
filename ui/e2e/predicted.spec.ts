import { expect, type Locator, type Page, test } from '@playwright/test';
import { createSurvey, uniqueName, uploadScaledPlan } from './helpers';

/**
 * Planning a floor nobody has walked (UI-TRL-18): put an AP down on a scaled
 * plan, look at the predicted map, move the AP and see the map follow, and
 * take it away again. Every step goes through trellisd, so what is asserted is
 * what the daemon stored and drew, not what the page remembers.
 */

/** Opens Coverage on the predicted layer for the survey the page has selected. */
async function openPrediction(page: Page) {
  await page.getByTestId('coverage-metric-predicted_rssi').click();
  await expect(page.getByTestId('placement-panel')).toBeVisible();
}

/** The predicted map as drawn, so a redraw can be told apart from the old one. */
function mapSource(page: Page) {
  return page.getByTestId('heatmap-image').getAttribute('src');
}

async function centreOf(locator: Locator) {
  const box = await locator.boundingBox();
  if (box === null) {
    throw new Error('not laid out');
  }
  return { x: box.x + box.width / 2, y: box.y + box.height / 2, box };
}

test('places, moves and deletes an AP and the predicted map follows', async ({ page }) => {
  await createSurvey(page, uniqueName('Predict'));
  await uploadScaledPlan(page);
  await page.getByTestId('plot-coverage').click();
  await openPrediction(page);

  // Nothing placed yet: refused, not drawn empty, and the plan is there to click.
  await expect(page.getByTestId('surface-message')).toHaveText(
    'No AP placed on 5 GHz. Click the plan to place one.',
  );
  const plan = page.getByTestId('placement-surface');
  const { box } = await centreOf(plan);
  await plan.click({ position: { x: box.width * 0.25, y: box.height * 0.3 } });

  const marker = page.getByTestId('ap-marker');
  await expect(marker).toHaveCount(1);
  await expect(page.getByTestId('heatmap-image')).toBeVisible();
  await expect(page.getByTestId('placement-calibration')).toContainText(
    'Drawn with the default model',
  );
  const before = await mapSource(page);
  const placedAt = Number(await marker.getAttribute('data-x'));
  expect(placedAt).toBeGreaterThan(150);
  expect(placedAt).toBeLessThan(250);

  // Drag it most of the way across: the stored position and the map both move.
  // Hovered first: it scrolls the marker into view and waits until nothing
  // (the sticky legend, the zoom bar) covers it, which raw mouse calls do not.
  await marker.hover();
  const from = await centreOf(marker);
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  await page.mouse.move(from.x + box.width * 0.5, from.y, { steps: 8 });
  await page.mouse.up();
  await expect.poll(async () => Number(await marker.getAttribute('data-x'))).toBeGreaterThan(550);
  await expect.poll(() => mapSource(page)).not.toBe(before);
  const movedTo = Number(await marker.getAttribute('data-x'));

  // A reload keeps it: the placement is the daemon's, not the page's.
  await page.reload();
  await openPrediction(page);
  await expect(marker).toHaveCount(1);
  await expect(marker).toHaveAttribute('data-x', String(movedTo));
  await expect(page.getByTestId('heatmap-image')).toBeVisible();

  // Deleting the only AP leaves nothing to predict from, and the page says so.
  await marker.click();
  await page.getByTestId('placement-delete').click();
  await expect(marker).toHaveCount(0);
  await expect(page.getByTestId('surface-message')).toHaveText(
    'No AP placed on 5 GHz. Click the plan to place one.',
  );
  await expect(page.getByTestId('heatmap-image')).toHaveCount(0);
});

test.describe('at phone width', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('the predicted layer and its AP panel fit the screen', async ({ page }) => {
    await createSurvey(page, uniqueName('Predict phone'));
    await uploadScaledPlan(page);
    await page.getByTestId('plot-coverage').click();
    await openPrediction(page);

    const plan = page.getByTestId('placement-surface');
    const { box } = await centreOf(plan);
    await plan.click({ position: { x: box.width * 0.5, y: box.height * 0.5 } });
    await expect(page.getByTestId('heatmap-image')).toBeVisible();
    await page.getByTestId('ap-marker').click();
    await expect(page.getByTestId('placement-editor')).toBeVisible();

    for (const testId of ['heatmap-viewport', 'placement-panel', 'placement-delete']) {
      const control = await page.getByTestId(testId).boundingBox();
      expect(control, `${testId} is not laid out`).not.toBeNull();
      expect(
        (control?.x ?? 0) + (control?.width ?? 0),
        `${testId} runs off screen`,
      ).toBeLessThanOrEqual(390);
    }
  });
});
