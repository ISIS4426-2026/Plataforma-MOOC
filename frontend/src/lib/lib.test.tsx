import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Markdown } from "./markdown";
import { disponible } from "./resources";

describe("disponible (un recurso en procesamiento nunca se muestra como disponible)", () => {
  it.each(["pending", "processing", "failed"] as const)("video %s NO esta disponible", (estado) => {
    expect(disponible({ type: "video", processing_status: estado })).toBe(false);
  });
  it("video completed SI", () => expect(disponible({ type: "video", processing_status: "completed" })).toBe(true));
  it("un PDF sin estado NO esta disponible (necesita pasar por el worker)", () => {
    expect(disponible({ type: "pdf", processing_status: undefined })).toBe(false);
  });
  it("texto y cuestionario sin estado SI (no pasan por el worker)", () => {
    expect(disponible({ type: "text", processing_status: undefined })).toBe(true);
    expect(disponible({ type: "quiz", processing_status: undefined })).toBe(true);
  });
  it("un texto cuyo estado es failed NO esta disponible", () => {
    expect(disponible({ type: "text", processing_status: "failed" })).toBe(false);
  });
});

describe("Markdown seguro", () => {
  const html = (t: string) => renderToStaticMarkup(<Markdown texto={t} />);

  it("titulos, listas, negrita y codigo", () => {
    const h = html("# Titulo\n\nUn **fuerte** y `codigo`.\n\n- uno\n- dos");
    expect(h).toContain("<h2>Titulo</h2>");
    expect(h).toContain("<strong>fuerte</strong>");
    expect(h).toContain("<code>codigo</code>");
    expect(h).toContain("<li>uno</li>");
  });
  it("entiende el salto de linea escapado que traen algunos textos sembrados", () => {
    expect(html("# A\n\nparrafo")).toContain("<h2>A</h2>");
  });
  it("no inyecta HTML: un script en el texto sale escapado", () => {
    const h = html('<script>alert(1)</script> <img src=x onerror=alert(2)>');
    expect(h).not.toContain("<script>");
    expect(h).not.toContain("<img");
    expect(h).toContain("&lt;script&gt;");
  });
});
