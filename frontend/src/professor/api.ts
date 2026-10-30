import { newIdempotencyKey, request } from "../api/client";
import type { Course, Page, Resource, ResourceType } from "../api/types";

const data = <T>(p: Promise<{ data: T }>): Promise<T> => p.then((r) => r.data);

// ---- Cursos propios ------------------------------------------------------------------

/** Cursos del profesor en cualquier estado, borradores incluidos (el catalogo publico no los lista). */
export function listMyCourses(params: { cursor?: string; limit?: number; search?: string } = {}): Promise<Page<Course>> {
  const q = new URLSearchParams();
  if (params.search) q.set("search", params.search);
  if (params.cursor) q.set("cursor", params.cursor);
  q.set("limit", String(params.limit ?? 50));
  return data(request<Page<Course>>(`/me/courses?${q}`));
}

export async function allMyCourses(): Promise<Course[]> {
  const all: Course[] = [];
  let cursor: string | undefined;
  do {
    const page = await listMyCourses({ cursor, limit: 100 });
    all.push(...page.items);
    cursor = page.pagination.has_more ? (page.pagination.next_cursor ?? undefined) : undefined;
  } while (cursor);
  return all;
}

export const createCourse = (title: string, description: string, key = newIdempotencyKey()): Promise<Course> =>
  data(request<Course>("/courses", { method: "POST", body: { title, description }, idempotencyKey: key }));

/** Devuelve el curso y su ETag: es lo que se manda como If-Match al editar. */
export async function getCourseWithEtag(id: string): Promise<{ course: Course; etag?: string }> {
  const r = await request<Course>(`/courses/${id}`);
  return { course: r.data, etag: r.etag };
}

export const updateCourse = (id: string, title: string, description: string, etag?: string): Promise<{ course: Course; etag?: string }> =>
  request<Course>(`/courses/${id}`, { method: "PUT", body: { title, description }, ifMatch: etag }).then((r) => ({ course: r.data, etag: r.etag }));

export const publishCourse = (id: string): Promise<Course> => data(request<Course>(`/courses/${id}/publish`, { method: "POST", body: {} }));
export const unpublishCourse = (id: string): Promise<Course> => data(request<Course>(`/courses/${id}/unpublish`, { method: "POST", body: {} }));

// ---- Estructura ----------------------------------------------------------------------

export const createModule = (courseId: string, title: string, key = newIdempotencyKey()) =>
  data(request<{ id: string }>(`/courses/${courseId}/modules`, { method: "POST", body: { title }, idempotencyKey: key }));
export const deleteModule = (id: string) => request(`/modules/${id}`, { method: "DELETE" });

export const createUnit = (moduleId: string, title: string, key = newIdempotencyKey()) =>
  data(request<{ id: string }>(`/modules/${moduleId}/units`, { method: "POST", body: { title }, idempotencyKey: key }));
export const deleteUnit = (id: string) => request(`/units/${id}`, { method: "DELETE" });

export interface NewResource {
  title: string;
  type: ResourceType;
  is_mandatory: boolean;
  allow_download: boolean;
  content_text?: string;
}

export const createResource = (unitId: string, body: NewResource, key = newIdempotencyKey()) =>
  data(request<Resource>(`/units/${unitId}/resources`, { method: "POST", body, idempotencyKey: key }));
export const deleteResource = (id: string) => request(`/resources/${id}`, { method: "DELETE" });

// ---- Carga de archivos (URL prefirmada: el archivo NO pasa por la API) ----------------

export interface PresignedUpload {
  upload_url: string;
  object_key: string;
  method: string;
  content_type: string;
  expires_at: string;
}

export const presignUpload = (resourceId: string, file: File): Promise<PresignedUpload> =>
  data(
    request<PresignedUpload>("/media/presigned-url", {
      method: "POST",
      body: { resource_id: resourceId, filename: file.name, mime_type: MIME_BY_EXT[extensionOf(file.name)] ?? file.type, file_size_bytes: file.size },
    }),
  );

/**
 * Transferencia directa al almacenamiento de objetos. Se usa XMLHttpRequest y no fetch porque solo
 * XHR informa el progreso de la subida. El Content-Type debe ser exactamente el firmado.
 */
export function putToStorage(upload: PresignedUpload, file: File, onProgress: (fraction: number) => void, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open(upload.method || "PUT", upload.upload_url);
    xhr.setRequestHeader("Content-Type", upload.content_type);
    xhr.upload.onprogress = (e) => e.lengthComputable && onProgress(e.loaded / e.total);
    xhr.onload = () => (xhr.status >= 200 && xhr.status < 300 ? resolve() : reject(new Error(`El almacenamiento rechazó el archivo (HTTP ${xhr.status}).`)));
    xhr.onerror = () => reject(new Error("Se cortó la conexión con el almacenamiento durante la subida."));
    xhr.onabort = () => reject(new Error("La subida se canceló."));
    signal?.addEventListener("abort", () => xhr.abort());
    xhr.send(file);
  });
}

/** Confirma la carga: la API comprueba que el objeto exista y encola el procesamiento (202). */
export const confirmUpload = (resourceId: string, objectKey: string, key = newIdempotencyKey()) =>
  request(`/media/uploads/${resourceId}/complete`, { method: "POST", body: { object_key: objectKey }, idempotencyKey: key });

/**
 * Espejo de la lista de extensiones del servidor (internal/storage/keys.go). La API exige que el tipo
 * MIME declarado sea EXACTAMENTE el canonico de la extension (va firmado en la URL), asi que la
 * interfaz lo deriva de la extension y no de lo que declare el navegador.
 */
export const MIME_BY_EXT: Record<string, string> = {
  ".mp4": "video/mp4",
  ".mov": "video/quicktime",
  ".mkv": "video/x-matroska",
  ".webm": "video/webm",
  ".mp3": "audio/mpeg",
  ".m4a": "audio/mp4",
  ".wav": "audio/wav",
  ".ogg": "audio/ogg",
  ".pdf": "application/pdf",
  ".ppt": "application/vnd.ms-powerpoint",
  ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
  ".doc": "application/msword",
  ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".png": "image/png",
  ".webp": "image/webp",
};

/** Extensiones que ofrece la interfaz para cada clase de recurso (el servidor valida el area). */
export const EXTENSIONS_BY_TYPE: Partial<Record<ResourceType, string[]>> = {
  video: [".mp4", ".mov", ".mkv", ".webm"],
  audio: [".mp3", ".m4a", ".wav", ".ogg"],
  pdf: [".pdf"],
  presentation: [".ppt", ".pptx"],
  downloadable: [".pdf", ".ppt", ".pptx", ".doc", ".docx"],
  image: [".jpg", ".jpeg", ".png", ".webp"],
};

export const extensionOf = (filename: string): string => {
  const i = filename.lastIndexOf(".");
  return i < 0 ? "" : filename.slice(i).toLowerCase();
};

/** Tamano maximo que la API autoriza (internal/media/service.go: 1 GiB). */
export const MAX_UPLOAD_BYTES = 1024 * 1024 * 1024;

// ---- Quiz (vista del autor: trae la clave) --------------------------------------------

export interface AuthorQuiz {
  id: string;
  resource_id: string;
  title: string;
  passing_score: number;
  max_attempts: number;
  questions_with_key?: Array<{
    id: string;
    prompt: string;
    position: number;
    options: Array<{ id: string; text: string; position: number; is_correct: boolean }>;
  }>;
}

export const getAuthorQuiz = (resourceId: string): Promise<AuthorQuiz> => data(request<AuthorQuiz>(`/resources/${resourceId}/quiz`));

export const createQuiz = (resourceId: string, body: { title: string; passing_score: number; max_attempts: number }, key = newIdempotencyKey()) =>
  data(request<AuthorQuiz>(`/resources/${resourceId}/quiz`, { method: "POST", body, idempotencyKey: key }));

export const addQuestion = (quizId: string, body: { prompt: string; options: Array<{ text: string; is_correct: boolean }> }, key = newIdempotencyKey()) =>
  data(request(`/quizzes/${quizId}/questions`, { method: "POST", body, idempotencyKey: key }));
