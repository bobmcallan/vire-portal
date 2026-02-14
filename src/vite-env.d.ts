/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL: string;
  readonly VITE_DOMAIN: string;
  readonly VITE_APP_VERSION: string;
  readonly VITE_APP_BUILD: string;
  readonly VITE_APP_COMMIT: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
