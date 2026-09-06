/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_DEV_SESSION_TOKEN?: string;
  readonly VITE_GATEWAY_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
