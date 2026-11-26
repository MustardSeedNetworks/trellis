import { expect, type Page } from '@playwright/test';

/**
 * Names carry a nonce because chromium and webkit share one daemon and one
 * survey store, and the list has no order.
 */
export function uniqueName(prefix: string): string {
  return `${prefix} ${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

/** Creates a survey from the Surveys page and leaves it selected. */
export async function createSurvey(page: Page, name: string) {
  await page.goto('/');
  await page.getByTestId('new-survey-name').fill(name);
  await page.getByTestId('create-survey').click();
  const detail = page.getByTestId('survey-detail');
  await expect(detail).toBeVisible();
  await expect(detail).toContainText('Created');
  return detail;
}

/**
 * Starts the selected survey and captures three spread points through the
 * scripted radio. Three distinct positions: a heatmap needs spread to
 * interpolate over, and three coincident points would be a degenerate field.
 */
export async function walkThreePoints(page: Page) {
  await page.getByTestId('survey-start').click();
  const surface = page.getByTestId('capture-surface');
  await expect(surface).toBeEnabled();

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
}

/**
 * Uploads the 800x600 fixture plan onto the selected survey's floor and scales
 * it: two marks 400 px apart called 10 metres, 0.025 m per pixel.
 */
export async function uploadScaledPlan(page: Page) {
  await page.getByTestId('floor-plan-input').setInputFiles('e2e/fixtures/ninth-floor.png');
  await expect(page.getByTestId('floor-plan-status')).toContainText('no scale yet');

  const calibration = page.getByTestId('calibration-surface');
  await expect(calibration).toBeVisible();
  const box = await calibration.boundingBox();
  if (box === null) {
    throw new Error('the calibration surface is not laid out');
  }
  await calibration.click({ position: { x: box.width * 0.25, y: box.height / 2 } });
  await calibration.click({ position: { x: box.width * 0.75, y: box.height / 2 } });
  await page.getByTestId('calibration-metres').fill('10');
  await page.getByTestId('calibrate-floor-plan').click();
  await expect(page.getByTestId('floor-plan-status')).toContainText('0.025 m per pixel');
}

/**
 * The survey's row in the list, searched from the first page forward. The list
 * is a paged table over a shared store and the server does not promise an
 * order, so a survey made a moment ago is not necessarily on the first page,
 * and one whose state just changed can move to another. Call this again after
 * anything that changes the survey rather than reusing an earlier locator.
 */
export async function surveyRow(page: Page, name: string) {
  const row = page.getByTestId('survey-row').filter({ hasText: name });
  const previous = page.getByRole('button', { name: 'Previous page' });
  const next = page.getByRole('button', { name: 'Next page' });
  // The table shows skeleton rows until the list arrives; paging before then
  // finds no pager and gives up on the first page.
  await page.getByTestId('survey-row').first().waitFor();
  while ((await previous.count()) > 0 && (await previous.isEnabled())) {
    await previous.click();
  }
  while ((await row.count()) === 0 && (await next.count()) > 0 && (await next.isEnabled())) {
    await next.click();
  }
  return row;
}
