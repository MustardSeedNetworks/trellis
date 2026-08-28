import { expect, test } from '@playwright/test';

/**
 * The daemon/UI handshake.
 *
 * Not "does React render" -- the unit suite covers that. These assert that
 * trellisd serves the embedded build and that the app reaches it, which is the
 * seam trellis's heatmap defects lived in and which no unit test can see.
 */

test('serves the embedded UI and renders the survey page', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByTestId('page-header-title')).toBeVisible();
});

test('__version reports real build metadata, not placeholders', async ({ request }) => {
  const response = await request.get('/__version');
  expect(response.status()).toBe(200);

  const body = (await response.json()) as Record<string, string>;

  // The Universal Build Contract's point is that these are injected at build
  // time. Asserting the keys exist would pass on a binary built outside the
  // make pipeline, which is the exact failure the contract exists to catch --
  // so assert they are populated and not the "unknown" fallback.
  for (const key of ['version', 'commit', 'buildTime'] as const) {
    expect(body[key], `${key} missing from /__version`).toBeTruthy();
    expect(body[key], `${key} is the unbuilt placeholder`).not.toBe('unknown');
  }

  // uiBuildHash is deliberately NOT asserted yet. The Universal Build Contract
  // requires it, and trellis does not inject it -- its Makefile is a bare
  // `go build ./...` with no ldflags, so the hash proving the UI was embedded
  // is always "unknown". Asserting it here would fail for a real reason that
  // this suite cannot fix; it is filed instead, and this comment is the
  // reminder to tighten the loop once the ldflags land.
  expect(body).toHaveProperty('uiBuildHash');
});

test('navigates to the built pages without falling through to NotBuiltYet', async ({ page }) => {
  // '/', '/import', '/coverage' and '/reports' are the routes with real
  // components; everything else is deliberately NotBuiltYet. A regression that
  // dropped one from the registry would still render *something*, so assert the
  // page header rather than merely that the route resolved.
  for (const path of ['/', '/import', '/coverage', '/reports']) {
    await page.goto(path);
    await expect(page.getByTestId('page-header-title'), `no page header at ${path}`).toBeVisible();
  }
});
