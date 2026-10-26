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
    const h = /^(#{1,3})\s+(.*)$/.exec(l);
    if (h) {
      cerrarLista();
      cerrarParrafo();
      const nivel = h[1].length;
      const Tag = (`h${nivel + 1}`) as "h2" | "h3" | "h4";
      bloques.push(<Tag key={`h${bloques.length}`}>{inline(h[2], `h${bloques.length}`)}</Tag>);
    } else if (/^\s*[-*]\s+/.test(l)) {
      cerrarParrafo();
      lista.push(l.replace(/^\s*[-*]\s+/, ""));
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
