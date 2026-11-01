import { PlusCircle } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import { Badge, EmptyState, ErrorState, PageTitle, Spinner } from "../components/ui";
import type { Course } from "../api/types";
import { ETIQUETA_ESTADO, TONO_ESTADO, useProfessorData } from "./ProfessorData";

type Filtro = "published" | "draft" | "unpublished" | "todos";
const FILTROS: { id: Filtro; etiqueta: string }[] = [
  { id: "published", etiqueta: "Publicados" },
  { id: "draft", etiqueta: "Borradores" },
  { id: "unpublished", etiqueta: "Despublicados" },
  { id: "todos", etiqueta: "Todos" },
];

export default function ProfessorCourses() {
  const { state, refresh } = useProfessorData();
  // Por omision se ven los publicados: son los que usan los estudiantes. Los borradores y los
  // despublicados (pruebas, versiones retiradas) quedan a un clic, no en medio de la lista.
  const [filtro, setFiltro] = useState<Filtro | null>(null);
  const todos: Course[] = state.status === "ok" ? state.courses : [];
  const activo: Filtro = filtro ?? (todos.some((c) => c.status === "published") ? "published" : "todos");
  const visibles = activo === "todos" ? todos : todos.filter((c) => c.status === activo);
  const cuantos = (f: Filtro) => (f === "todos" ? todos.length : todos.filter((c) => c.status === f).length);

  return (
    <>
      <PageTitle
        title="Mis cursos"
        subtitle="Los cursos que has creado, con su estado. Los borradores solo los ves tú."
        action={
          <Link to="/profesor/cursos/nuevo" className="btn">
            <PlusCircle size={16} aria-hidden="true" /> Crear curso
          </Link>
        }
      />
      {state.status === "loading" && <Spinner label="Cargando tus cursos…" />}
      {state.status === "error" && <ErrorState error={state.error} onRetry={refresh} />}
      {state.status === "ok" && todos.length > 0 && (
        <div className="filters" role="tablist" aria-label="Filtrar por estado">
          {FILTROS.map((f) => (
            <button key={f.id} type="button" role="tab" aria-selected={activo === f.id} className={`btn small ${activo === f.id ? "" : "ghost"}`} onClick={() => setFiltro(f.id)}>
              {f.etiqueta} ({cuantos(f.id)})
            </button>
          ))}
        </div>
      )}
      {state.status === "ok" && (
        <div className="panel table-panel">
          {state.courses.length === 0 ? (
            <EmptyState title="Todavía no has creado ningún curso">Pulsa «Crear curso» para empezar.</EmptyState>
          ) : (
            <table>
              <thead>
                <tr>
                  <th scope="col">Curso</th>
                  <th scope="col">Estado</th>
                  <th scope="col">Versión</th>
                  <th scope="col" />
                </tr>
              </thead>
              <tbody>
                {visibles.map((c) => (
                  <tr key={c.id}>
                    <th scope="row">
                      <Link to={`/profesor/cursos/${c.id}`}>{c.title}</Link>
                    </th>
                    <td>
                      <Badge tone={TONO_ESTADO[c.status]}>{ETIQUETA_ESTADO[c.status]}</Badge>
                    </td>
                    <td>v{c.version}</td>
                    <td className="right">
                      <Link to={`/profesor/cursos/${c.id}`} className="btn small">
                        Abrir
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </>
  );
}
