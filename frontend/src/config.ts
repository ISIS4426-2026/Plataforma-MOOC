// Unica fuente de configuracion del frontend. Todo lo que hay aqui termina dentro
// del bundle, que cualquiera puede leer: por eso son valores publicos (la version,
// el origen de la API) y NUNCA claves, tokens ni direcciones internas.

/** Vacio = la API vive en el mismo origen que la pagina (balanceador o nginx). */
export const API_BASE_URL: string = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/+$/, "");

export const API_PREFIX = "/api/v1";

export const APP_VERSION = {
  commit: __APP_COMMIT__,
  builtAt: __APP_BUILT_AT__,
};
