// Configuracion que cambia segun el entorno y NO va dentro del bundle: la sirve nginx en /config.json
// (ver nginx.web.conf). Asi el mismo build sirve en local y en la nube, y el bundle no lleva
// direcciones de ningun entorno (criterio de D1). Son valores publicos, nunca secretos.

export interface RuntimeConfig {
  /** Base publica del almacenamiento de derivados HLS; el manifiesto es <base>/hls/<id>/master.m3u8. */
  hlsBaseUrl?: string;
}

let config: RuntimeConfig = {};

export async function loadRuntimeConfig(): Promise<void> {
  try {
    const res = await fetch("/config.json", { cache: "no-store" });
    if (res.ok && (res.headers.get("content-type") ?? "").includes("json")) config = await res.json();
  } catch {
    // Sin config.json (p. ej. `vite dev`): los videos mostraran que no hay origen configurado.
  }
}

export const runtimeConfig = (): RuntimeConfig => config;

export function stripTrailingSlashes(value: string): string {
  let end = value.length;
  while (end > 0 && value.charCodeAt(end - 1) === 47) {
    end--;
  }
  return value.slice(0, end);
}

export function hlsManifestUrl(resourceStableId: string): string | null {
  const base = config.hlsBaseUrl ? stripTrailingSlashes(config.hlsBaseUrl) : "";
  return base ? `${base}/hls/${resourceStableId}/master.m3u8` : null;
}
