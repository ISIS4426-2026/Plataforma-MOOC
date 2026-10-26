/** Primer nombre para saludar: ignora tratamientos como «Prof.» o «Dra.». */
export function primerNombre(nombreCompleto: string): string {
  const sinTitulo = nombreCompleto.replace(/^\s*(prof(esor|esora)?|dr|dra|mg|ing|lic)\.?\s+/i, "");
  return sinTitulo.split(/\s+/)[0] ?? "";
}

export const plural = (n: number, singular: string, plural: string): string => `${n} ${n === 1 ? singular : plural}`;
