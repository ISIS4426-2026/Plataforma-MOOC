import { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { listCourses } from "../api/endpoints";
import type { Course } from "../api/types";
import { useAsync } from "../hooks/useAsync";
import { EmptyState, ErrorState, PageTitle, Spinner } from "../components/ui";
import { EnrollButton } from "./EnrollButton";

export default function Catalog() {
  const [params] = useSearchParams();
  const q = params.get("q")?.trim() ?? "";
  const primera = useAsync(() => listCourses({ search: q || undefined, limit: 20 }), [q]);

  // «Cargar mas»: las paginas siguientes se acumulan aparte de la primera, que se recarga al buscar.
  const [extra, setExtra] = useState<{ q: string; items: Course[]; cursor: string | null } | null>(null);
  const [cargando, setCargando] = useState(false);
  const [errorMas, setErrorMas] = useState<string | null>(null);

  if (primera.status === "loading") return <Spinner label="Buscando cursos…" />;
  if (primera.status === "error") return <ErrorState error={primera.error} onRetry={primera.reload} />;

  const propias = extra && extra.q === q ? extra : null;
  const cursos = [...primera.data.items, ...(propias?.items ?? [])];
  const cursor = propias ? propias.cursor : primera.data.pagination.has_more ? primera.data.pagination.next_cursor : null;

  async function masCursos() {
    if (!cursor) return;
    setCargando(true);
    setErrorMas(null);
    try {
      const pagina = await listCourses({ search: q || undefined, cursor, limit: 20 });
      setExtra({
        q,
        items: [...(propias?.items ?? []), ...pagina.items],
        cursor: pagina.pagination.has_more ? pagina.pagination.next_cursor : null,
      });
    } catch (e) {
      setErrorMas((e as Error).message);
    } finally {
      setCargando(false);
    }
  }

  return (
    <>
      <PageTitle
        title="Catálogo de cursos"
        subtitle={q ? `Resultados para «${q}»` : "Solo se muestran los cursos publicados."}
      />
      <div className="panel table-panel">
        {cursos.length === 0 ? (
          <EmptyState title={q ? "No hay cursos que coincidan con tu búsqueda" : "Todavía no hay cursos publicados"}>
            {q && (
              <Link to="/catalogo" className="btn ghost small">
                Quitar la búsqueda
              </Link>
            )}
          </EmptyState>
        ) : (
          <table>
            <thead>
              <tr>
                <th scope="col">Curso</th>
                <th scope="col">Descripción</th>
                <th scope="col">Versión</th>
                <th scope="col" />
              </tr>
            </thead>
            <tbody>
              {cursos.map((c) => (
                <tr key={c.id}>
                  <th scope="row">
                    <Link to={`/cursos/${c.id}`}>{c.title}</Link>
                  </th>
                  <td className="clip">{c.description}</td>
                  <td>v{c.version}</td>
                  <td className="right">
                    <EnrollButton course={c} compact />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      {cursor && (
        <div className="center">
          <button type="button" className="btn ghost" onClick={masCursos} disabled={cargando}>
            {cargando ? "Cargando…" : "Cargar más cursos"}
          </button>
          {errorMas && (
            <p role="alert" className="error">
              {errorMas}
            </p>
          )}
        </div>
      )}
    </>
  );
}
