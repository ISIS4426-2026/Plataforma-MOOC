import type { Resource } from "../api/types";

/**
 * El enunciado habla de un recurso «available»; en el codigo ese estado se llama `completed`
 * (nota 8 de NOTAS_TECNICAS). Un recurso que no esta listo NUNCA se muestra como disponible ni se
 * puede abrir. Los tipos que no pasan por el worker (texto, cuestionario, enlace) no llevan
 * estado de procesamiento y estan disponibles desde que se crean.
 */
export function disponible(r: Pick<Resource, "type" | "processing_status">): boolean {
  if (r.processing_status) return r.processing_status === "completed";
  return r.type === "text" || r.type === "quiz" || r.type === "external_link";
}
