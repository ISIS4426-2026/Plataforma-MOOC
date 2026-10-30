import type { Resource, ResourceType } from "../api/types";

/** Tipos de recurso que llevan un archivo y pasan por el worker. */
export const TIPOS_CON_ARCHIVO: ResourceType[] = ["video", "audio", "pdf", "presentation", "downloadable", "image"];

/**
 * El enunciado habla de un recurso «available»; en el codigo ese estado se llama `completed`
 * (nota 8 de NOTAS_TECNICAS). Un recurso que no esta listo NUNCA se muestra como disponible ni se
 * puede abrir.
 *
 * Para los recursos de archivo no basta `completed`: la base de datos lo pone por defecto, asi que
 * un video recien creado y SIN archivo figura como `completed` (nota 28). Aqui exige ademas que
 * exista el archivo. Los tipos que no pasan por el worker (texto, cuestionario, enlace) no llevan
 * estado de procesamiento y estan disponibles desde que se crean.
 */
export function disponible(r: Pick<Resource, "type" | "processing_status" | "object_key">): boolean {
  if (TIPOS_CON_ARCHIVO.includes(r.type)) return r.processing_status === "completed" && Boolean(r.object_key);
  if (r.processing_status) return r.processing_status === "completed";
  return r.type === "text" || r.type === "quiz" || r.type === "external_link";
}

/** Un recurso de archivo al que todavia no se le subio nada. */
export function sinArchivo(r: Pick<Resource, "type" | "object_key" | "processing_status">): boolean {
  return TIPOS_CON_ARCHIVO.includes(r.type) && !r.object_key && (r.processing_status === "completed" || !r.processing_status);
}
