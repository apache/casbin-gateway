/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Absolute base for API calls; empty means same-origin (see Setting.tsx). */
  readonly VITE_SERVER_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
