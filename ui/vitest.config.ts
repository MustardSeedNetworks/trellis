import { fileURLToPath, URL } from 'node:url';
import babel from '@rolldown/plugin-babel';
import react, { reactCompilerPreset } from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

// Node's own experimental webstorage global is read once per test file while
// the jsdom environment is set up, and each read prints
// "localStorage is not available because --localstorage-file was not provided".
// Nothing here uses it -- `localStorage` in a test resolves to jsdom's
// MemoryStorage on a proper http://localhost origin -- so the feature is turned
// off rather than the message silenced. Workers do not inherit the parent's
// CLI flags and worker_threads ignores execArgv for process-level options, so
// NODE_OPTIONS is the only channel that reaches them; setting it here rather
// than in the npm script keeps it working on Windows shells too.
process.env.NODE_OPTIONS = [process.env.NODE_OPTIONS, '--no-experimental-webstorage']
  .filter(Boolean)
  .join(' ');

export default defineConfig({
  plugins: [
    react(),
    // The React Compiler, matching vite.config.ts. Without it the suite
    // exercises un-compiled components while the shipped bundle is compiled —
    // so a memo the compiler subsumes looks required here, and a compiler
    // regression could never fail a test.
    babel({ presets: [reactCompilerPreset()] }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      // Must mirror vite.config.ts: the suite initialises i18next, which
      // imports the locale catalogs through this alias.
      '@locales': fileURLToPath(new URL('../internal/i18n/locales', import.meta.url)),
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
    globals: false,
    // Scoped to src/. Vitest's default include is **/*.{test,spec}.* , which
    // also matches the Playwright specs under e2e/ -- Vitest then collects them
    // and Playwright rejects the call with "Playwright Test did not expect
    // test() to be called here". Same shape as seed's config.
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html', 'lcov'],
      exclude: ['node_modules/', 'src/test-setup.ts', '**/*.d.ts', '**/*.config.*', 'dist/', 'src/gen/'],
      // This project had no coverage gate at all: no `test:coverage` script, no
      // thresholds, and CI running plain `npm run test`. Coverage passed
      // because nothing was being asked -- while SurveyDetail.tsx,
      // SurveyList.tsx and SurveysPage.tsx sat at 0%, which is every route into
      // what the product does with a survey.
      //
      // These track the fleet standard (stem's configured 88/80/92/88) rather
      // than a floor drawn under today's debt -- measured 2026-08-29 at
      // branches 80.24, functions 92.3, lines 98.13.
      //
      // Statements is 87, not 88, and the reason is worth stating rather than
      // rounding away: excluding src/gen (generated protobuf, not authored
      // code) removes 30 trivially-covered statements from the denominator and
      // the real figure lands at 87.81. Keeping the exclusion and admitting 87
      // is honest; padding the denominator with generated code to reach a
      // rounder number would not be. Two authored statements close the gap.
      //
      // Lines sits at 88 with the rest because a 98 floor would fail on
      // ordinary run-to-run drift; raise it once a second measurement
      // confirms the margin.
      //
      // Ratchet up as coverage rises. Never lower these to make a change pass.
      thresholds: {
        lines: 88,
        branches: 80,
        functions: 92,
        statements: 87,
      },
    },
  },
});
