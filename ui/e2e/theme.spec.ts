import { expect, type Page, test } from '@playwright/test';

/**
 * Dark mode (trellis#472). The palette was fully written and never applied, so
 * the UI audit's light and dark screenshots came back pixel-identical.
 *
 * These read the computed background rather than comparing screenshots: a
 * screenshot diff proves two renders differ, not that the right tokens are
 * live, and this defect was precisely two renders that did not differ.
 *
 * Both schemes are asserted. A dark-only test passes on a build that hardcodes
 * dark — which is what the three sibling products' hooks do — and would have
 * called this row done with the light palette just as broken.
 */

/** The `--color-surface-base` each palette defines, as rgb. */
const LIGHT_SURFACE = 'rgb(251, 250, 245)';
const DARK_SURFACE = 'rgb(6, 13, 19)';

async function surfaceColour(page: Page): Promise<string> {
  return page.evaluate(() =>
    getComputedStyle(document.documentElement).getPropertyValue('--color-surface-base').trim(),
  );
}

async function rootIsDark(page: Page): Promise<boolean> {
  return page.evaluate(() => document.documentElement.classList.contains('dark'));
}

/**
 * The colour actually painted behind the shell.
 *
 * `document.body` is transparent here — the shell's outer flex container is
 * what carries `bg-surface-base` — so reading the body proves nothing. This
 * reads the element that paints, which is what an operator sees.
 */
async function paintedSurface(page: Page): Promise<string> {
  return page.evaluate(() => {
    const shell = document.querySelector('.bg-surface-base');
    if (!shell) {
      throw new Error('no element carries the surface background');
    }
    return getComputedStyle(shell).backgroundColor;
  });
}

test.describe('with a dark desktop', () => {
  test.use({ colorScheme: 'dark' });

  test('renders the dark palette without being asked', async ({ page }) => {
    await page.goto('/');

    expect(await rootIsDark(page)).toBe(true);
    expect(await paintedSurface(page)).toBe(DARK_SURFACE);
  });
});

test.describe('with a light desktop', () => {
  test.use({ colorScheme: 'light' });

  test('renders the light palette without being asked', async ({ page }) => {
    await page.goto('/');

    expect(await rootIsDark(page)).toBe(false);
    expect(await paintedSurface(page)).toBe(LIGHT_SURFACE);
  });

  test('the toggle overrides the OS and survives a reload', async ({ page }) => {
    await page.goto('/');
    expect(await surfaceColour(page)).toBe('#fbfaf5');

    await page.getByTestId('theme-toggle').click();
    expect(await rootIsDark(page)).toBe(true);
    expect(await surfaceColour(page)).toBe('#060d13');

    // The reload is the assertion: an in-memory toggle passes every line above.
    await page.reload();
    expect(await rootIsDark(page)).toBe(true);
    expect(await surfaceColour(page)).toBe('#060d13');

    // And back, so the control is not one-way.
    await page.getByTestId('theme-toggle').click();
    await page.reload();
    expect(await rootIsDark(page)).toBe(false);
    expect(await surfaceColour(page)).toBe('#fbfaf5');
  });
});
