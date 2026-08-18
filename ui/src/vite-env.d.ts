/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_TRELLIS_API?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

/** Injected by vite.config.ts from package.json. */
declare const __APP_VERSION__: string;
