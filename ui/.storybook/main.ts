import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { StorybookConfig } from '@storybook/react-vite';
import type { UserConfig } from 'vite';

const currentDir: string = dirname(fileURLToPath(import.meta.url));

const config: StorybookConfig = {
  stories: ['../src/**/*.stories.@(ts|tsx)'],
  addons: ['@storybook/addon-vitest', '@storybook/addon-a11y'],
  framework: '@storybook/react-vite',
  viteFinal: (viteConfig: UserConfig): UserConfig => ({
    // Override rather than merge, as stem does: vite.config.ts carries the
    // React Compiler and the embed outDir, neither of which the story runner
    // wants. What the stories need is the two aliases.
    ...viteConfig,
    resolve: {
      ...viteConfig.resolve,
      alias: {
        '@': resolve(currentDir, '../src'),
        '@locales': resolve(currentDir, '../../internal/i18n/locales'),
      },
    },
    optimizeDeps: {
      ...viteConfig.optimizeDeps,
      include: [...(viteConfig.optimizeDeps?.include ?? []), 'aria-query'],
    },
  }),
};
export default config;
