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
  },
});
