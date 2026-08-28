import { defineConfig, devices } from '@playwright/test';

/**
 * Trellis E2E.
 *
 * The suite drives the UI *as trellisd serves it* — the daemon embeds the built
 * assets from internal/api/ui, so `npm run build` must have run first. Testing
 * against `vite dev` instead would prove the components render but not that the
 * daemon serves them, and the daemon/UI handshake is the thing most worth
 * covering: trellis's known heatmap defects all lived in the read path between
 * the two, and passed unit tests that asserted counts rather than values.
 *
 * Port 18446, not the daemon's default 8446: a developer running trellisd
 * locally must not have their instance silently reused as the test subject,
 * and the #69-class port-fallback walk means an occupied 8446 would otherwise
 * relocate the daemon somewhere the tests are not looking.
 */
const PORT = 18446;
const baseURL = `http://127.0.0.1:${PORT}`;

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  // retries 1, matching seed/stem/niac. One retry absorbs a transient; the
  // flake budget in CI then fails the job for having needed it, so a
  // retry-pass cannot quietly become the normal state.
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: [
    ['html', { outputFolder: 'playwright-report' }],
    ['list'],
    // The flake budget reads this. Never pass --reporter on the CLI: it
    // REPLACES this array, which is how seed, stem and niac each ended up
    // producing no JSON report at all.
    ['json', { outputFile: 'playwright-report/results.json' }],
  ],
  use: {
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],
  webServer: {
    // TRELLIS_DATA_DIR is per-run so a survey written by one run cannot leak
    // into the next and make an assertion pass for the wrong reason.
    // `go build` then run, not `go run`. go run does not stamp VCS metadata, so
    // /__version reports commit "unknown" and the build-contract assertion in
    // handshake.spec.ts fails for a reason that has nothing to do with the
    // product. Building first is also closer to what ships.
    command:
      'cd .. && go build -o "$TMPDIR/trellisd-e2e" ./cmd/trellisd && ' +
      'TRELLIS_ADDR=127.0.0.1:18446 TRELLIS_DATA_DIR="$(mktemp -d)" "$TMPDIR/trellisd-e2e"',
    url: `${baseURL}/__version`,
    reuseExistingServer: false,
    timeout: 180_000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
