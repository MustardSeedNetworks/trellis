import { expect, test } from '@playwright/test';
import axe from 'axe-core';
import enPages from '../../internal/i18n/locales/en/pages.json' with { type: 'json' };
import esPages from '../../internal/i18n/locales/es/pages.json' with { type: 'json' };

const routes = [
  { path: '/', key: 'surveys' },
  { path: '/import', key: 'import' },
  { path: '/coverage', key: 'coverage' },
  { path: '/live', key: 'live' },
  { path: '/reports', key: 'reports' },
] as const;

for (const language of ['en', 'es'] as const) {
  test.describe(`${language} page help`, () => {
    test.use({
      viewport: { width: 390, height: 844 },
      contextOptions: { reducedMotion: 'reduce' },
    });

    test.beforeEach(async ({ page }) => {
      await page.addInitScript((locale) => localStorage.setItem('language', locale), language);
    });

    for (const route of routes) {
      test(`${route.path} has named controls, page help and keyboard navigation`, async ({
        page,
        browserName,
      }) => {
        const copy = language === 'en' ? enPages : esPages;
        await page.goto(route.path);
        expect(
          await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches),
        ).toBe(true);
        await expect(page.getByTestId('page-header-title')).toHaveText(copy[route.key].title);
        await expect(page).toHaveTitle(`${copy[route.key].title} | Trellis`);
        const tabKey =
          browserName === 'webkit' && process.platform === 'darwin' ? 'Alt+Tab' : 'Tab';
        await page.keyboard.press(tabKey);
        await expect(page.getByTestId('skip-to-content')).toBeFocused();
        await page.keyboard.press('Enter');
        await expect(page.locator('#main-content')).toBeFocused();

        const links = page.getByTestId('sidebar').locator('nav a');
        expect(
          await links.evaluateAll((items) => items.map((item) => item.getAttribute('href'))),
        ).toEqual(routes.map((item) => item.path));
        await page.addScriptTag({ content: axe.source });
        const names = await page.evaluate(async () => {
          const engine = (window as unknown as { axe: typeof axe }).axe;
          return engine.run(document, {
            runOnly: ['button-name', 'link-name', 'aria-command-name'],
          });
        });
        expect(names.violations).toEqual([]);
        expect(names.incomplete).toEqual([]);
        expect(names.passes.length).toBeGreaterThan(0);

        const opener = page.getByTestId('page-help-open');
        await opener.click();
        const drawer = page.getByTestId('page-help-drawer');
        await expect(drawer).toBeVisible();
        await expect(drawer).toHaveAccessibleName(
          `${language === 'en' ? 'Help' : 'Ayuda'}: ${copy[route.key].title}`,
        );
        const expectedCopy =
          route.key === 'live' || route.key === 'reports'
            ? copy[route.key].description
            : copy.help[route.key];
        await expect(page.getByTestId('page-help-copy')).toHaveText(expectedCopy);
        await expect(page.getByTestId('page-help-close')).toBeFocused();
        await page.keyboard.press('Shift+Tab');
        await expect(page.getByTestId('page-help-close')).toBeFocused();
        const box = await drawer.boundingBox();
        expect(box).not.toBeNull();
        expect(box?.x).toBeGreaterThanOrEqual(0);
        expect((box?.x ?? 0) + (box?.width ?? 0)).toBeLessThanOrEqual(390);
        await page.keyboard.press('Escape');
        await expect(drawer).not.toBeVisible();
        await expect(opener).toBeFocused();
        await opener.click();
        await page.getByTestId('page-help-close').click();
        await expect(drawer).not.toBeVisible();
        await expect(opener).toBeFocused();
        await expect(page.getByTestId('sidebar')).toHaveCSS('transition-property', 'none');
      });
    }
  });
}

test('collapsed rail tooltips support hover, keyboard focus and Escape', async ({ page }) => {
  await page.goto('/');
  await page.getByTestId('sidebar-collapse').click();
  const link = page.getByTestId('nav-reports');
  await link.hover();
  const tooltip = page.getByRole('tooltip', { name: 'Reports' });
  await expect(tooltip).toBeVisible();
  await tooltip.hover();
  await expect(tooltip).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(tooltip).not.toBeVisible();
  await page.mouse.move(700, 700);
  await link.focus();
  await expect(tooltip).toBeVisible();
  await expect(link).toHaveAttribute('aria-describedby', (await tooltip.getAttribute('id')) ?? '');
  await page.keyboard.press('Escape');
  await expect(tooltip).not.toBeVisible();
});

test('browser history changes the page without reopening old help', async ({ page }) => {
  await page.goto('/');
  await page.getByTestId('nav-import').click();
  await page.getByTestId('page-help-open').click();
  await expect(page.getByTestId('page-help-drawer')).toBeVisible();
  await page.goBack();
  await expect(page).toHaveTitle('Surveys | Trellis');
  await expect(page.getByTestId('page-help-drawer')).not.toBeVisible();
  await page.goForward();
  await expect(page).toHaveTitle('Import | Trellis');
  await expect(page.getByTestId('page-help-drawer')).not.toBeVisible();
  await page.getByTestId('page-help-open').click();
  await expect(page.getByTestId('page-help-drawer')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByTestId('page-help-open')).toBeFocused();
});
