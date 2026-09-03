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
