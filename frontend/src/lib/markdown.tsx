import type { ReactNode } from "react";

// Renderizador minimo y SEGURO de los textos de los recursos: construye elementos de React (nunca
// HTML en bruto), asi que un texto malicioso no puede inyectar scripts. Cubre lo que el contenido
// del curso usa: titulos, parrafos, listas, **negrita** y `codigo`.

function inline(texto: string, clave: string): ReactNode[] {
  const partes: ReactNode[] = [];
  const re = /(\*\*[^*]+\*\*|`[^`]+`)/g;
  let ultimo = 0;
  let n = 0;
  for (let m = re.exec(texto); m; m = re.exec(texto)) {
    if (m.index > ultimo) partes.push(texto.slice(ultimo, m.index));
    const tok = m[0];
    partes.push(
      tok.startsWith("**") ? <strong key={`${clave}-${n++}`}>{tok.slice(2, -2)}</strong> : <code key={`${clave}-${n++}`}>{tok.slice(1, -1)}</code>,
    );
    ultimo = m.index + tok.length;
  }
  if (ultimo < texto.length) partes.push(texto.slice(ultimo));
  return partes;
}

function parseHeading(line: string): { level: 1 | 2 | 3; text: string } | null {
  let hashes = 0;
  while (hashes < line.length && hashes < 4 && line.charCodeAt(hashes) === 35) {
    hashes++;
  }
  if (hashes < 1 || hashes > 3 || hashes >= line.length) {
    return null;
  }
  const next = line[hashes];
  if (next !== " " && next !== "\t") {
    return null;
  }
  return {
    level: hashes as 1 | 2 | 3,
    text: line.slice(hashes).trimStart(),
  };
}

function parseListItem(line: string): string | null {
  const trimmed = line.trimStart();
  if (trimmed.length < 2) {
    return null;
  }
  const marker = trimmed[0];
  const sep = trimmed[1];
  if ((marker !== "-" && marker !== "*") || (sep !== " " && sep !== "\t")) {
    return null;
  }
  return trimmed.slice(1).trimStart();
}

export function Markdown({ texto }: { texto: string }) {
  // Algunos textos sembrados traen el salto de linea escapado ("\\n") en lugar del caracter real.
  const lineas = texto.replace(/\\n/g, "\n").split("\n");
  const bloques: ReactNode[] = [];
  let lista: string[] = [];
  let parrafo: string[] = [];

  const cerrarLista = () => {
    if (lista.length) {
      bloques.push(
        <ul key={`l${bloques.length}`}>
          {lista.map((it, i) => (
            <li key={i}>{inline(it, `li${bloques.length}-${i}`)}</li>
          ))}
        </ul>,
      );
      lista = [];
    }
  };
  const cerrarParrafo = () => {
    if (parrafo.length) {
      bloques.push(<p key={`p${bloques.length}`}>{inline(parrafo.join(" "), `p${bloques.length}`)}</p>);
      parrafo = [];
    }
  };

  for (const crudo of lineas) {
    const l = crudo.trimEnd();
    const h = parseHeading(l);
    const item = h ? null : parseListItem(l);
    if (h) {
      cerrarLista();
      cerrarParrafo();
      const Tag = (`h${h.level + 1}`) as "h2" | "h3" | "h4";
      bloques.push(<Tag key={`h${bloques.length}`}>{inline(h.text, `h${bloques.length}`)}</Tag>);
    } else if (item !== null) {
      cerrarParrafo();
      lista.push(item);
    } else if (l.trim() === "") {
      cerrarLista();
      cerrarParrafo();
    } else {
      cerrarLista();
      parrafo.push(l.trim());
    }
  }
  cerrarLista();
  cerrarParrafo();
  return <div className="prose">{bloques}</div>;
}
