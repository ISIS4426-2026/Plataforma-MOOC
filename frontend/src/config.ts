// Unica fuente de configuracion del frontend. Todo lo que hay aqui termina dentro
// del bundle, que cualquiera puede leer: por eso son valores publicos (la version,
// el origen de la API) y NUNCA claves, tokens ni direcciones internas.

function stripTrailingSlashes(value: string): string {
  let end = value.length;
  while (end > 0 && value.charCodeAt(end - 1) === 47) {
    end--;
  }
  return value.slice(0, end);
}

/** Vacio = la API vive en el mismo origen que la pagina (balanceador o nginx). */
export const API_BASE_URL: string = stripTrailingSlashes(import.meta.env.VITE_API_BASE_URL ?? "");

export const API_PREFIX = "/api/v1";

export const APP_VERSION = {
  commit: __APP_COMMIT__,
  builtAt: __APP_BUILT_AT__,
};
