import { fileURLToPath, URL } from 'node:url';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react()],
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
