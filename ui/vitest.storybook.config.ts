import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { storybookTest } from '@storybook/addon-vitest/vitest-plugin';
import { playwright } from '@vitest/browser-playwright';
import { defineConfig } from 'vitest/config';

const currentDir = dirname(fileURLToPath(import.meta.url));

/**
 * Story files that do not yet pass the interaction/a11y run, excluded by path
 * so every other story is gated by default. A deny-list, on seed's evidence:
 * under a tag allow-list one of 88 story files carried the tag and the job
 * proved the harness worked while covering nothing. Shrink this; never grow it.
 */
const NOT_YET_PASSING: string[] = [];

export default defineConfig({
  /* aria-query is CommonJS and reached only through addon-a11y's runtime;
     left un-prebundled the browser runner dies on a missing named export
     before any story mounts. react-query is reached only from inside stories,
     so Vite discovers it mid-run and reloads the browser — which hung stem's
     CI runner silently until the job timed out (stem#955). Pre-bundling both
     leaves nothing to discover. */
  optimizeDeps: {
    include: ['aria-query', '@tanstack/react-query', 'react-router'],
  },
  server: {
    // One-shot run; a watcher invalidating the module graph mid-suite is the
    // signature of the flake stem saw, so there is none.
    watch: null,
  },
  test: {
    projects: [
      {
        extends: true,
        plugins: [storybookTest({ configDir: resolve(currentDir, '.storybook') })],
        test: {
          name: 'storybook',
          exclude: NOT_YET_PASSING,
          // Serial: browser mode starts a Chromium per worker and the
          // contention on a CI runner read as story files failing to import.
          fileParallelism: false,
          browser: {
            enabled: true,
            headless: true,
            provider: playwright({}),
            instances: [{ browser: 'chromium' }],
          },
        },
      },
    ],
  },
});
