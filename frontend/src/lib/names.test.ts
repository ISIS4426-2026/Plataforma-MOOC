import { describe, expect, it } from "vitest";
import { plural, primerNombre } from "./names";

describe("saludo y plurales", () => {
  it.each([
    ["Prof. Carlos Mendoza", "Carlos"],
    ["Dra. Laura Gómez", "Laura"],
    ["Estudiante de Carga 0001", "Estudiante"],
    ["Ana", "Ana"],
  ])("%s -> %s", (entrada, esperado) => expect(primerNombre(entrada)).toBe(esperado));

  it("pluraliza", () => {
    expect(plural(1, "pregunta", "preguntas")).toBe("1 pregunta");
    expect(plural(2, "pregunta", "preguntas")).toBe("2 preguntas");
    expect(plural(0, "pregunta", "preguntas")).toBe("0 preguntas");
  });
});
