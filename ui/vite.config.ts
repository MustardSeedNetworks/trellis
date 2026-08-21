import { fileURLToPath, URL } from 'node:url';
import babel from '@rolldown/plugin-babel';
import react, { reactCompilerPreset } from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
import pkg from './package.json' with { type: 'json' };

export default defineConfig({
  // Surfaced in the rail footer; read from package.json so the number in
  // the UI cannot drift from the published artefact.
  define: { __APP_VERSION__: JSON.stringify(pkg.version) },
  plugins: [
    react(),
    // React Compiler. It memoises what it can prove is safe, which is why the
    // existing hand-written memos stay in place here: removing them is a
    // separate change, and some hold an identity stable rather than save work
    // — the compiler does not replace those.
    //
    // plugin-react v6 is oxc-based and has no `babel` option; the compiler runs
    // through @rolldown/plugin-babel with the preset the plugin ships.
    babel({ presets: [reactCompilerPreset()] }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
  },
  build: {
    outDir: '../internal/api/ui',
    // emptyOutDir intentionally omitted: outDir is outside Vite's project
    // root, and Vite's default for an outside-root outDir is
    // emptyOutDir: false. Setting it explicitly would wipe .gitkeep on every
    // build — see the Universal Build Contract in CLAUDE.md.
  },
});
