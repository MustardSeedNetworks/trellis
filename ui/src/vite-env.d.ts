/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_TRELLIS_API?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
