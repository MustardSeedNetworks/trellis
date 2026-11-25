import { expect, type Page, test } from '@playwright/test';

/**
 * Desktop clarity and density (UI-TRL-13, trellis#515).
 *
 * The desktop counterpart to `phone.spec.ts`. That file establishes the shape
 * of the assertion for this shell and the reason for it: trellis is `h-screen`
 * with `overflow-hidden` on `<main>`, so an over-wide child is CLIPPED rather
 * than scrolled and `document.documentElement.scrollWidth` reads exactly the
 * viewport width on the defect. Every measurement here is therefore PER
 * ELEMENT, never on the document — the same finding the fleet recorded as
 * UI-FLEET-2 finding 1, where trellis's own UI-TRL-2 acceptance was vacuous
 * for this reason.
 *
 * Two properties, both ratchets:
 *
 *   1. Content fits its container at 1280 and 1440 px.
 *   2. The page header's own footprint stays inside the budget the density
 *      pass left it, so a later change that loosens the page fails instead of
 *      drifting.
 */

const DESKTOP_WIDTHS = [1280, 1440] as const;
const VIEWPORT_HEIGHT = 900;

/** Every route the rail offers, with what must be on screen before measuring. */
const ROUTES: readonly { path: string; ready?: string }[] = [
  { path: '/' },
  { path: '/import' },
  { path: '/coverage' },
  { path: '/live', ready: 'neighbour-row' },
  { path: '/reports' },
];

/**
 * Ceiling measured at 1440×900. The UI-TRL-13 density pass took every route's
 * page header from 83 px to 76 px, the fleet's kicker + 24 px title + one-line
 * subtitle. The shared `PageHeader` (UI-TRL-19) adds the breadcrumb row the
 * owner asked for on every page (msn-owner-decisions §25), which is one 20 px
 * line and 4 px beneath it: 76 + 24 + 4 = 104 px, and that is what every route
 * measures. A ratchet at the shared header's own footprint — tightening is
 * welcome, loosening needs a reason.
 */
const MAX_PAGE_HEADER_FOOTPRINT_PX = 104;

interface Overflow {
  selector: string;
  scrollWidth: number;
  clientWidth: number;
}

/**
 * Elements whose content is wider than the box that holds it. Rounding in
 * fractional layout widths costs a pixel or two, so only a real overflow
 * counts.
 */
async function findHorizontalOverflow(page: Page): Promise<Overflow[]> {
  return page.evaluate(() => {
    const TOLERANCE_PX = 2;
    // A visually-hidden element is clipped to a 1 px box on purpose, so its
    // text always "overflows" — that is the technique, not a defect.
    const VISUALLY_HIDDEN_PX = 2;
    const found: { selector: string; scrollWidth: number; clientWidth: number }[] = [];

    const selectorFor = (el: Element): string => {
      const testId = el.getAttribute('data-testid');
      if (testId) return `[data-testid="${testId}"]`;
      const cls = el.className;
      const classes = typeof cls === 'string' ? cls.split(/\s+/).slice(0, 3).join('.') : '';
      return classes ? `${el.tagName.toLowerCase()}.${classes}` : el.tagName.toLowerCase();
    };

    for (const el of Array.from(document.body.querySelectorAll('*'))) {
      // The fleet attribute from UI-FLEET-2 — a region that scrolls sideways
      // by design says so in the markup.
      if (el.closest('[data-phone-width-exempt]')) continue;

      // A <select> clips a long option label inside its own control by design;
      // webkit reports that as overflow on the select and its ancestors while
      // chromium does not. phone.spec.ts excludes it for the same reason.
      if (el.closest('select')) continue;

      const rect = el.getBoundingClientRect();
      if (rect.width === 0 || rect.height === 0) continue;
      if (rect.width <= VISUALLY_HIDDEN_PX || rect.height <= VISUALLY_HIDDEN_PX) continue;

      // Truncation is the prescribed fit for a cell too long for its column,
      // and a truncated cell overflows its box by construction.
      const style = getComputedStyle(el);
      if (style.textOverflow === 'ellipsis') continue;

      // An element that clips nothing cannot hide a defect of its own: what
      // spills out of it reaches the clipping ancestor, which is measured. The
      // shared help button is the case in point: its hit area is an absolutely
      // positioned ::after wider than the 28 px button, so its own scrollWidth
      // reads 36 and nothing is wrong with it.
      if (style.overflowX === 'visible') continue;

      if (el.scrollWidth > el.clientWidth + TOLERANCE_PX) {
        found.push({
          selector: selectorFor(el),
          scrollWidth: el.scrollWidth,
          clientWidth: el.clientWidth,
        });
      }
    }
    return found;
  });
}

async function gotoRoute(page: Page, route: (typeof ROUTES)[number]): Promise<void> {
  await page.goto(route.path);
  await expect(page.getByTestId('page-header-title')).toBeVisible();
  if (route.ready) {
    await expect(page.getByTestId(route.ready).first()).toBeVisible();
  }
}

test.describe('desktop clarity and density', () => {
  for (const width of DESKTOP_WIDTHS) {
    test(`content fits its container at ${width}px on every route`, async ({ page }) => {
      await page.setViewportSize({ width, height: VIEWPORT_HEIGHT });

      const offenders: string[] = [];
      for (const route of ROUTES) {
        await gotoRoute(page, route);

        for (const overflow of await findHorizontalOverflow(page)) {
          offenders.push(
            `${route.path} → ${overflow.selector} (content ${overflow.scrollWidth}px in ${overflow.clientWidth}px)`,
          );
        }
      }

      expect(
        offenders,
        `no element may scroll sideways at ${width}px without data-phone-width-exempt`,
      ).toEqual([]);
    });
  }

  test('page chrome stays within its density budget at 1440px', async ({ page }, testInfo) => {
    await page.setViewportSize({ width: 1440, height: VIEWPORT_HEIGHT });

    const measured: Record<string, number> = {};
    const tooTall: string[] = [];

    for (const route of ROUTES) {
      await gotoRoute(page, route);

      const height = await page
        .getByTestId('page-header')
        .evaluate((el: HTMLElement) => Math.round(el.getBoundingClientRect().height));

      measured[route.path] = height;
      expect(height, `${route.path} should render a page header`).toBeGreaterThan(0);
      if (height > MAX_PAGE_HEADER_FOOTPRINT_PX) {
        tooTall.push(`${route.path} → page header ${height}px`);
      }
    }

    await testInfo.attach('density-1440.json', {
      body: JSON.stringify(measured, null, 2),
      contentType: 'application/json',
    });

    expect(
      tooTall,
      `page header must stay within ${MAX_PAGE_HEADER_FOOTPRINT_PX}px (UI-TRL-13 ratchet)`,
    ).toEqual([]);
  });
});
