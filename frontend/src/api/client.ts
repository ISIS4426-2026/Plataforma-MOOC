import { API_BASE_URL, API_PREFIX } from "../config";

export interface FieldProblem {
  field: string;
  message: string;
}

/** Error de la API o de la red, con lo necesario para mostrarlo y para rastrearlo. */
export class ApiError extends Error {
  constructor(
    /** Codigo HTTP, o 0 si ni siquiera hubo respuesta (red caida, CORS, timeout). */
    readonly status: number,
    /** Codigo de negocio de la API (p. ej. "invalid_credentials"), o "network_error". */
    readonly code: string,
    message: string,
    /** X-Request-ID de la respuesta: es lo que permite buscar la traza en el servidor. */
    readonly requestId?: string,
    /** X-Instance-ID: que replica de la API contesto. */
    readonly instanceId?: string,
    /** Problemas campo a campo (p. ej. por que un curso no se puede publicar: 422 validation_failed). */
    readonly details?: FieldProblem[],
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/**
 * Manejo global. Se registra una sola vez (ver AuthProvider) para que ninguna vista
 * tenga que acordarse de tratar una sesion vencida o un error del servidor.
 */
export interface ApiHooks {
  /** 401 fuera del login: la sesion no existe o vencio. */
  onUnauthorized?: (error: ApiError) => void;
  /** 403: autenticado pero sin permiso (o origen rechazado por CSRF). */
  onForbidden?: (error: ApiError) => void;
  /** 5xx o sin respuesta. */
  onServerError?: (error: ApiError) => void;
}

let hooks: ApiHooks = {};

export function setApiHooks(next: ApiHooks): void {
  hooks = next;
}

export interface RequestOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  /** Obligatorio en las escrituras que la API protege contra repeticion (inscripcion, envio de quiz...). */
  idempotencyKey?: string;
  signal?: AbortSignal;
  /** El login responde 401 cuando las credenciales son malas: no es una sesion vencida. */
  skipUnauthorizedHook?: boolean;
  /** ETag que el cliente leyo; con el la API rechaza (412) una edicion sobre una version vieja. */
  ifMatch?: string;
}

export interface ApiResult<T> {
  data: T;
  requestId?: string;
  instanceId?: string;
  /** ETag de la respuesta, para enviarlo despues como If-Match. */
  etag?: string;
}

export function newIdempotencyKey(): string {
  return crypto.randomUUID();
}

/**
 * Autenticacion: SOLO por la cookie de sesion que pone el servidor (HttpOnly, Secure).
 * El login tambien devuelve el token en el cuerpo, pero este cliente nunca lo lee ni lo
 * guarda: asi un XSS no tiene nada que robar y la comprobacion de origen (CSRF) de la API
 * protege las escrituras. Con la API en el mismo origen la cookie viaja sola.
 */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<ApiResult<T>> {
  const method = options.method ?? "GET";
  const headers: Record<string, string> = { Accept: "application/json" };
  if (options.body !== undefined) headers["Content-Type"] = "application/json";
  if (options.idempotencyKey) headers["Idempotency-Key"] = options.idempotencyKey;
  if (options.ifMatch) headers["If-Match"] = options.ifMatch;

  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${API_PREFIX}${path}`, {
      method,
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      // Mismo origen: same-origin. Si la API estuviera en otro origen haria falta "include"
      // y, ademas, CORS con credenciales y SameSite=None en el servidor (nota 22).
      credentials: API_BASE_URL ? "include" : "same-origin",
      signal: options.signal,
    });
  } catch (cause) {
    if (options.signal?.aborted) throw cause;
    const error = new ApiError(0, "network_error", "No se pudo contactar con el servidor.");
    hooks.onServerError?.(error);
    throw error;
  }

  const requestId = response.headers.get("X-Request-ID") ?? undefined;
  const instanceId = response.headers.get("X-Instance-ID") ?? undefined;

  if (response.ok) {
    const text = await response.text();
    return { data: (text ? JSON.parse(text) : undefined) as T, requestId, instanceId, etag: response.headers.get("ETag") ?? undefined };
  }

  let code = "http_error";
  let message = `La solicitud fallo (HTTP ${response.status}).`;
  let details: FieldProblem[] | undefined;
  try {
    const body = (await response.json()) as { code?: string; message?: string; details?: unknown };
    if (body.code) code = body.code;
    if (body.message) message = body.message;
    if (Array.isArray(body.details)) {
      details = body.details.filter((d): d is FieldProblem => typeof d?.message === "string");
    }
  } catch {
    // Cuerpo vacio o no JSON (p. ej. una pagina de error del balanceador): se deja el mensaje generico.
  }

  const error = new ApiError(response.status, code, message, requestId, instanceId, details);
  if (response.status === 401 && !options.skipUnauthorizedHook) hooks.onUnauthorized?.(error);
  else if (response.status === 403) hooks.onForbidden?.(error);
  else if (response.status >= 500) hooks.onServerError?.(error);
  throw error;
}
