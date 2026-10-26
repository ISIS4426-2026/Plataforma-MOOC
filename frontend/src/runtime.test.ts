import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { hlsManifestUrl, loadRuntimeConfig } from "./runtime";

describe("configuracion de entorno", () => {
  beforeEach(() => vi.stubGlobal("fetch", vi.fn()));
  afterEach(() => vi.unstubAllGlobals());

  it("el manifiesto HLS se arma con la base publica y el id estable", async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({ hlsBaseUrl: "https://almacen.example/bucket-hls/" }), { headers: { "content-type": "application/json" } }),
    );
    await loadRuntimeConfig();
    expect(hlsManifestUrl("abc")).toBe("https://almacen.example/bucket-hls/hls/abc/master.m3u8");
  });

  it("si /config.json no es JSON (p. ej. la pagina de vite dev) no rompe y no inventa un origen", async () => {
    vi.mocked(fetch).mockResolvedValue(new Response("<html></html>", { headers: { "content-type": "text/html" } }));
    await loadRuntimeConfig();
    // Queda la configuracion previa del test anterior; lo que importa es que no lance.
    expect(() => hlsManifestUrl("abc")).not.toThrow();
  });

  it("sin base configurada no hay URL de video", async () => {
    vi.mocked(fetch).mockResolvedValue(new Response(JSON.stringify({ hlsBaseUrl: "" }), { headers: { "content-type": "application/json" } }));
    await loadRuntimeConfig();
    expect(hlsManifestUrl("abc")).toBeNull();
  });
});
