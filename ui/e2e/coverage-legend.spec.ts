import { expect, type Page, test } from '@playwright/test';
import { createSurvey, uniqueName, walkThreePoints } from './helpers';

/**
 * The legend has to be on screen with the map it explains (trellis#476).
 *
 * The audit found Coverage scrolling 1254 px inside a 636 px container at
 * 1440x900: the surface grew to whatever the floor plan needed and pushed the
 * colour key below the fold, so the one thing that says what the colours mean
 * was never visible beside them.
 *
 * Asserted with a bounding box rather than toBeVisible(): the legend exists
 * and can be scrolled to on the broken build, which was never the complaint.
 */
const WIDE = { width: 1440, height: 900 };
/** Below xl the findings panel stacks under the map instead of beside it. */
const NARROW = { width: 1024, height: 768 };
/** The phone width the fleet keeps full parity at (owner 2026-09-15). */
const PHONE = { width: 390, height: 844 };

/** A floor-plan survey, plotted and left on Coverage. */
async function plotAPlannedFloor(page: Page, label: string) {
  await createSurvey(page, uniqueName(label));
  /* A floor plan, because that is the shape the defect needs: the surface is
     laid out from the plan's aspect ratio, and the 800x600 fixture is the
     taller of the two the suite has. Without a plan the blank 800x500 canvas
     leaves the legend at y=840 of 900 — on screen by 5 px, which would make
     the assertion below pass on the broken build. */
  await page.getByTestId('floor-plan-input').setInputFiles('e2e/fixtures/ninth-floor.png');
  await expect(page.getByTestId('floor-plan-status')).toContainText('no scale yet');
  await walkThreePoints(page);
  await page.getByTestId('survey-complete').click();
  await page.getByTestId('plot-coverage').click();
  await expect(page.getByTestId('heatmap-image')).toBeVisible();
}

test.describe('where the findings sit beside the map', () => {
  test.use({ viewport: WIDE });

  test('shows the legend with the map at 1440x900 without scrolling', async ({ page }) => {
    await plotAPlannedFloor(page, 'Legend');

    const legend = page.getByTestId('heatmap-legend');
    const box = await legend.boundingBox();
    if (!box) {
      throw new Error('legend has no layout box');
    }
    expect(box.y, 'legend starts above the viewport').toBeGreaterThanOrEqual(0);
    expect(box.y + box.height, 'legend ends below the fold').toBeLessThanOrEqual(WIDE.height);

    // And the map is still on screen with it: a legend made visible by
    // shrinking the surface to nothing would satisfy the box assertion alone.
    const imageBox = await page.getByTestId('heatmap-image').boundingBox();
    if (!imageBox) {
      throw new Error('heatmap image has no layout box');
    }
    expect(imageBox.height, 'map collapsed to make room').toBeGreaterThan(100);
  });
});

/**
 * The bound above must not travel below the breakpoint that earns it. Bounding
 * a stacked layout does not put the legend on screen — it moves the clipping
 * onto whatever is under it. Measured at 1024x768 while the bound started at
 * md: the findings panel ended 118 px past the fold with nothing to scroll.
 */
test.describe('where the findings stack under the map', () => {
  test.use({ viewport: NARROW });

  test('keeps the page scrollable so the findings stay reachable', async ({ page }) => {
    await plotAPlannedFloor(page, 'Stacked');

    const findings = page.getByTestId('coverage-findings');
    await findings.scrollIntoViewIfNeeded();
    const box = await findings.boundingBox();
    if (!box) {
      throw new Error('findings panel has no layout box');
    }
    expect(box.y, 'findings start above the viewport').toBeGreaterThanOrEqual(0);
    expect(box.y + box.height, 'findings cannot be scrolled into view').toBeLessThanOrEqual(
      NARROW.height,
    );
  });
});

/**
 * The key stays on screen where the layout scrolls (UI-TRL-9, trellis#484).
 *
 * The bound above only holds at xl. Below it the page scrolls, and a tall
 * floor plan carried the legend past the fold again — so the one thing that
 * says what the colours mean was gone exactly while an operator was reading
 * them. The ramp now steps at their own threshold, which is a boundary they
 * cannot guess from the picture, so the key matters more than it did.
 *
 * Asserted after scrolling the map, not on arrival: a legend that is only on
 * screen before you touch the page is the defect.
 */
for (const viewport of [NARROW, PHONE]) {
  test.describe(`the key at ${viewport.width}x${viewport.height}`, () => {
    test.use({ viewport });

    test('stays with the map while the map is scrolled', async ({ page }) => {
      await plotAPlannedFloor(page, 'Sticky');

      const image = page.getByTestId('heatmap-image');
      await image.scrollIntoViewIfNeeded();
      await page.mouse.wheel(0, 400);

      const legend = page.getByTestId('heatmap-legend');
      const box = await legend.boundingBox();
      if (!box) {
        throw new Error('legend has no layout box');
      }
      expect(box.y, 'legend scrolled off the top').toBeGreaterThanOrEqual(0);
      expect(box.y + box.height, 'legend scrolled past the fold').toBeLessThanOrEqual(
        viewport.height,
      );

      // And it is the map's key, not a floating bar over nothing: the surface
      // has to still be on screen with it.
      const imageBox = await image.boundingBox();
      if (!imageBox) {
        throw new Error('heatmap image has no layout box');
      }
      expect(imageBox.y, 'map is entirely below the fold').toBeLessThan(viewport.height);
      expect(imageBox.y + imageBox.height, 'map is entirely above the viewport').toBeGreaterThan(0);
    });
  });
}
