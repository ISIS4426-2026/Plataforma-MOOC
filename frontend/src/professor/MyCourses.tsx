import { PlusCircle } from "lucide-react";
import { Link } from "react-router-dom";
import { Badge, EmptyState, ErrorState, PageTitle, Spinner } from "../components/ui";
import { ETIQUETA_ESTADO, TONO_ESTADO, useProfessorData } from "./ProfessorData";

export default function ProfessorCourses() {
  const { state, refresh } = useProfessorData();

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
                {state.courses.map((c) => (
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
