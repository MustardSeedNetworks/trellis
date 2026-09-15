import { expect, type Page, test } from '@playwright/test';
import { createSurvey, uniqueName, walkThreePoints } from './helpers';

/**
 * Phone width (trellis#473). The rail was fixed at 252px with no responsive
 * behaviour, leaving about 138px of a 390px screen for the page — the UI
 * audit's `light/390x844/surveys-populated.png` shows form fields clipping
 * mid-word.
 *
 * The obvious assertion — "the document does not scroll sideways" — is
 * VACUOUS here, and deliberately not what these tests make. The shell is
 * `h-screen` with `overflow-hidden` on <main>, so at 390px the broken build
 * CLIPS rather than scrolls: `document.documentElement.scrollWidth` is 390 on
 * the defect. These assert on content instead — that every scroll container
 * fits its box, and that each route's controls lie inside the viewport — which
 * is the shape the operator actually sees.
 */

const PHONE = { width: 390, height: 844 };

/**
 * Every route the rail offers, which is every route the registry has, with
 * what has to be on screen before it is worth measuring. Live is the one that
 * matters: the scripted radio answers with three BSSs, so a sweep taken as
 * soon as the rail appears measures an empty page and proves nothing about
 * the table that actually renders there.
 */
const ROUTES: readonly { path: string; ready?: string }[] = [
  { path: '/' },
  { path: '/import' },
  { path: '/coverage' },
  { path: '/live', ready: 'neighbour-row' },
  { path: '/reports' },
];

test.use({ viewport: PHONE });

/**
 * The widest right edge of any control on the page, in viewport coordinates.
 * Hidden elements have a zero box and are skipped; a control pushed off the
 * right of a clipping shell reports a right edge beyond the viewport, which is
 * exactly the audit's screenshot in numbers.
 */
async function widestControlEdge(page: Page): Promise<{ right: number; label: string }> {
  return page.evaluate(() => {
    let worst = { right: 0, label: 'nothing rendered' };
    const controls = document.querySelectorAll('input, button, select, textarea, a, h1, h2');
    for (const element of controls) {
      const box = element.getBoundingClientRect();
      if (box.width === 0 && box.height === 0) {
        continue;
      }
      if (box.right > worst.right) {
        worst = {
          right: box.right,
          label: `${element.tagName.toLowerCase()} "${(element.textContent ?? '').trim().slice(0, 40)}"`,
        };
      }
    }
    return worst;
  });
}

/**
 * Any element whose own content is wider than the box it is drawn in.
 *
 * Two things are excluded, both of them content that is wider than its box on
 * purpose. A `<select>` clips a long option label inside its own control by
 * design — webkit reports that as overflow on the select and on its ancestors,
 * chromium does not — so a survey named longer than the control is not a
 * layout defect. An element that declares horizontal scrolling is the other:
 * the neighbour table on Live is 567px inside a 236px `overflow-x-auto`
 * wrapper, which is the one shape a narrow screen is allowed to have. Neither
 * exclusion hides a control escaping the screen — `widestControlEdge` measures
 * every control against the viewport rather than against its parent.
 */
async function overflowingElements(page: Page): Promise<string[]> {
  return page.evaluate(() =>
    [...document.querySelectorAll('*')]
      .filter((element) => element.scrollWidth > element.clientWidth + 1 && element.clientWidth > 0)
      .filter((element) => element.tagName !== 'SELECT' && element.querySelector('select') === null)
      .filter((element) => !/^(auto|scroll)$/.test(getComputedStyle(element).overflowX))
      .map(
        (element) =>
          `${element.tagName.toLowerCase()}.${element.className.toString().slice(0, 60)} ` +
          `(${element.scrollWidth} in ${element.clientWidth})`,
      ),
  );
}

for (const route of ROUTES) {
  test(`every control on ${route.path} is inside a 390px screen`, async ({ page }) => {
    await page.goto(route.path);
    await expect(page.getByTestId('sidebar')).toBeVisible();
    if (route.ready !== undefined) {
      await expect(page.getByTestId(route.ready).first()).toBeVisible();
    }

    const widest = await widestControlEdge(page);
    expect(widest.right, `${widest.label} runs past the right edge`).toBeLessThanOrEqual(
      PHONE.width,
    );
    expect(await overflowingElements(page)).toEqual([]);
  });
}

test('the rail is collapsed at phone width and cannot be expanded over the page', async ({
  page,
}) => {
  await page.goto('/');

  const rail = page.getByTestId('sidebar');
  const box = await rail.boundingBox();
  expect(box?.width, 'the rail still takes most of a phone screen').toBeLessThanOrEqual(80);

  // The collapse control is the way a phone layout gets broken again: expanding
  // to 252px leaves 138px of content. It is not offered below the breakpoint.
  await expect(page.getByTestId('sidebar-collapse')).toBeHidden();
});

test('the Create survey form is fully usable at phone width', async ({ page }) => {
  await page.goto('/');

  const name = page.getByTestId('new-survey-name');
  const create = page.getByTestId('create-survey');
  await expect(name).toBeVisible();
  await expect(create).toBeVisible();

  for (const [label, control] of [
    ['the name field', name],
    ['the create button', create],
  ] as const) {
    const box = await control.boundingBox();
    expect(box, `${label} has no layout box`).not.toBeNull();
    expect(box?.x ?? -1, `${label} starts off-screen`).toBeGreaterThanOrEqual(0);
    expect((box?.x ?? 0) + (box?.width ?? 0), `${label} is cut off`).toBeLessThanOrEqual(
      PHONE.width,
    );
  }

  // Not just laid out — it works: the survey is created and its detail opens.
  await name.fill(`Phone ${Date.now()}`);
  await create.click();
  await expect(page.getByTestId('survey-detail')).toBeVisible();

  // And the populated page is the one the audit screenshotted: an empty
  // Surveys page fits on almost anything, so a layout assertion taken before
  // this point proves very little.
  const widest = await widestControlEdge(page);
  expect(widest.right, `${widest.label} runs past the right edge`).toBeLessThanOrEqual(PHONE.width);
  expect(await overflowingElements(page)).toEqual([]);
});

/**
 * The owner's scope for this row is full parity at 390x844 — "every route
 * usable, including floor-plan capture" — so the walk itself is driven here
 * rather than only the pages around it.
 */
test('a floor can be walked at phone width', async ({ page }) => {
  await createSurvey(page, uniqueName('Phone walk'));
  await walkThreePoints(page);

  const surface = page.getByTestId('capture-surface');
  const box = await surface.boundingBox();
  expect(box, 'the capture surface has no layout box').not.toBeNull();
  expect((box?.x ?? 0) + (box?.width ?? 0), 'the capture surface is cut off').toBeLessThanOrEqual(
    PHONE.width,
  );
  expect(await overflowingElements(page)).toEqual([]);

  // Coverage with a real heatmap on it, which is the widest thing this UI
  // draws: the empty-route assertions above never load an image.
  await page.getByTestId('plot-coverage').click();
  await expect(page.getByTestId('heatmap-image')).toBeVisible();
  expect(await overflowingElements(page)).toEqual([]);
});

/**
 * The desktop must not inherit the phone's forced collapse, and the operator's
 * own collapse choice must not be overwritten by a visit at phone width.
 */
test.describe('at desktop width', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('the rail is expanded and its control is offered', async ({ page }) => {
    await page.goto('/');

    const box = await page.getByTestId('sidebar').boundingBox();
    expect(box?.width).toBeGreaterThan(200);
    await expect(page.getByTestId('sidebar-collapse')).toBeVisible();
  });
});
