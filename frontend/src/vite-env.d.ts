/// <reference types="vite/client" />

declare const __APP_COMMIT__: string;
declare const __APP_BUILT_AT__: string;

interface ImportMetaEnv {
  /** Origen de la API si NO comparte origen con el frontend. Vacio = mismo origen (lo recomendado). */
  readonly VITE_API_BASE_URL?: string;
  readonly VITE_APP_COMMIT?: string;
}
interface ImportMeta {
  readonly env: ImportMetaEnv;
}
