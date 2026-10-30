import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, newIdempotencyKey, request, setApiHooks } from "./client";

function respuesta(status: number, body?: unknown, headers: Record<string, string> = {}): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", ...headers },
  });
}

describe("cliente HTTP", () => {
  const fetchMock = vi.fn();
  const hooks = { onUnauthorized: vi.fn(), onForbidden: vi.fn(), onServerError: vi.fn() };

  beforeEach(() => {
    fetchMock.mockReset();
    Object.values(hooks).forEach((h) => h.mockReset());
    vi.stubGlobal("fetch", fetchMock);
    setApiHooks(hooks);
  });
  afterEach(() => vi.unstubAllGlobals());

  it("pide al mismo origen, con cookie y sin token en cabeceras", async () => {
    fetchMock.mockResolvedValue(respuesta(200, { ok: true }, { "X-Request-ID": "r1", "X-Instance-ID": "web-a" }));
    const res = await request<{ ok: boolean }>("/health");
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/health");
    expect(init.credentials).toBe("same-origin");
    expect(init.headers).not.toHaveProperty("Authorization");
    expect(res).toEqual({ data: { ok: true }, requestId: "r1", instanceId: "web-a" });
  });

  it("envia el cuerpo como JSON y la Idempotency-Key cuando se da", async () => {
    fetchMock.mockResolvedValue(respuesta(200, {}));
    await request("/courses/1/enrollments", { method: "POST", body: {}, idempotencyKey: "k-1" });
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(init.method).toBe("POST");
    expect(init.body).toBe("{}");
    expect(init.headers).toMatchObject({ "Content-Type": "application/json", "Idempotency-Key": "k-1" });
  });

  it("401 fuera del login avisa al manejador global", async () => {
    fetchMock.mockResolvedValue(respuesta(401, { code: "unauthorized", message: "Se requiere autenticacion." }));
    await expect(request("/me/enrollments")).rejects.toMatchObject({ status: 401, code: "unauthorized" });
    expect(hooks.onUnauthorized).toHaveBeenCalledTimes(1);
  });

  it("401 del login (credenciales malas) NO se trata como sesion vencida", async () => {
    fetchMock.mockResolvedValue(respuesta(401, { code: "invalid_credentials", message: "Credenciales invalidas." }));
    await expect(request("/auth/login", { method: "POST", body: {}, skipUnauthorizedHook: true })).rejects.toBeInstanceOf(ApiError);
    expect(hooks.onUnauthorized).not.toHaveBeenCalled();
  });

  it("403 avisa al manejador y conserva el codigo de negocio", async () => {
    fetchMock.mockResolvedValue(respuesta(403, { code: "csrf_origin_rejected", message: "El origen de la solicitud no esta permitido." }));
    await expect(request("/auth/logout", { method: "POST" })).rejects.toMatchObject({ status: 403, code: "csrf_origin_rejected" });
    expect(hooks.onForbidden).toHaveBeenCalledTimes(1);
  });

  it("5xx avisa al manejador, aunque el cuerpo no sea JSON", async () => {
    fetchMock.mockResolvedValue(new Response("<html>502</html>", { status: 502 }));
    await expect(request("/courses")).rejects.toMatchObject({ status: 502, code: "http_error" });
    expect(hooks.onServerError).toHaveBeenCalledTimes(1);
  });

  it("sin respuesta (red caida) es un ApiError con status 0", async () => {
    fetchMock.mockRejectedValue(new TypeError("Failed to fetch"));
    await expect(request("/courses")).rejects.toMatchObject({ status: 0, code: "network_error" });
    expect(hooks.onServerError).toHaveBeenCalledTimes(1);
  });

  it("una respuesta 204 sin cuerpo no rompe", async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 204 }));
    const res = await request<undefined>("/auth/logout", { method: "POST" });
    expect(res.data).toBeUndefined();
  });

  it("un 422 de publicacion conserva la lista de problemas campo a campo", async () => {
    fetchMock.mockResolvedValue(
      respuesta(422, {
        code: "validation_failed",
        message: "El curso no cumple los requisitos de publicación.",
        details: [{ field: "structure", message: "El curso debe tener al menos un módulo." }, { field: "x" }],
      }),
    );
    const error = await request("/courses/1/publish", { method: "POST", body: {} }).catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(422);
    // solo se conservan los problemas que traen un mensaje legible
    expect(error.details).toEqual([{ field: "structure", message: "El curso debe tener al menos un módulo." }]);
  });

  it("envia If-Match cuando se da y devuelve el ETag de la respuesta", async () => {
    fetchMock.mockResolvedValue(respuesta(200, { id: "c1" }, { ETag: '"abc"' }));
    const res = await request("/courses/c1", { method: "PUT", body: {}, ifMatch: '"v1"' });
    expect((fetchMock.mock.calls[0][1] as RequestInit).headers).toMatchObject({ "If-Match": '"v1"' });
    expect(res.etag).toBe('"abc"');
  });

  it("genera claves de idempotencia distintas", () => {
    expect(newIdempotencyKey()).not.toBe(newIdempotencyKey());
  });
});
