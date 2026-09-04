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

test('walks continuously and stops when told', async ({ page }) => {
  await createSurvey(page, uniqueName('Continuous'));
  await page.getByTestId('survey-start').click();
  await expect(page.getByTestId('capture-surface')).toBeEnabled();

  await page.getByTestId('toggle-continuous').click();
  await expect(page.getByTestId('toggle-continuous')).toHaveText('Stop walking');

  // Points accrue with nobody clicking — the whole difference from stop-and-go.
  // Three, not one: one could be the start's own sweep.
  await expect(page.getByTestId('capture-pin')).toHaveCount(3, { timeout: 30000 });
  await expect(page.getByTestId('capture-status')).toContainText('Walking at');

  // Moving the pin moves the walk rather than taking a one-shot point there.
  const surface = page.getByTestId('capture-surface');
  const box = await surface.boundingBox();
  if (!box) {
    throw new Error('capture surface has no layout box');
  }
  await surface.click({ position: { x: box.width * 0.8, y: box.height * 0.8 } });
  // Asserted as a region, not a pixel: the click maps through the rendered box,
  // which is a fraction of a pixel off the 800x500 surface space at any window
  // size. Four fifths across an 800x500 surface is near (640, 400), and the
  // walk starts at its centre — so anything past three quarters proves the
  // capture moved with the operator rather than staying where it began.
  await expect
    .poll(async () => {
      const text = (await page.getByTestId('capture-status').textContent()) ?? '';
      const [x, y] = /\((\d+), (\d+)\)/.exec(text)?.slice(1).map(Number) ?? [0, 0];
      return x > 600 && y > 375;
    })
    .toBe(true);

  // Marking a second position places every reading taken on the way there
  // along the segment. They are drawn as placed rather than as marks, because
  // their positions were worked out and not recorded.
  const placed = page.locator('[data-testid="capture-pin"][data-interpolated="true"]');
  await expect(placed).toHaveCount(3, { timeout: 30000 });
  const xs = await placed.evaluateAll((nodes) =>
    nodes.map((node) => Number(node.querySelector('circle')?.getAttribute('cx') ?? '0')),
  );
  // Along the walk, in the order they were taken: a placement that ignored the
  // timestamps would still produce three points somewhere.
  expect(xs).toEqual([...xs].sort((a, b) => a - b));
  expect(Math.max(...xs)).toBeLessThanOrEqual(641);

  const walked = await page.getByTestId('capture-pin').count();
  await page.getByTestId('toggle-continuous').click();
  await expect(page.getByTestId('toggle-continuous')).toHaveText('Start walking');

  // Stopped means stopped: the daemon must not still be holding the radio for a
  // walk nobody is on. Completing the survey also releases it for the rest of
  // the suite, which shares one daemon and one radio.
  await page.waitForTimeout(6000);
  expect(await page.getByTestId('capture-pin').count()).toBe(walked);
  await page.getByTestId('survey-complete').click();
});
