import { ArrowRight } from "lucide-react";
import { Link } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { primerNombre } from "../lib/names";
import { EmptyState, ErrorState, ProgressBar, Spinner, gradiente } from "../components/ui";
import { EnrollButton } from "./EnrollButton";
import { useStudentData } from "./StudentData";

export default function Dashboard() {
  const { user } = useAuth();
  const { state, refresh } = useStudentData();
  const nombre = primerNombre(user?.full_name ?? "");

  return (
    <>
      <section className="panel hero">
        <div>
          <h1>¡Bienvenido de nuevo, {nombre}!</h1>
          <p className="muted">Retoma tus cursos donde los dejaste o explora el catálogo para empezar uno nuevo.</p>
          <Link to="/catalogo" className="btn">
            Explorar catálogo
          </Link>
        </div>
        <svg className="hero-art" viewBox="0 0 220 150" aria-hidden="true">
          <rect x="30" y="92" width="150" height="26" rx="6" fill="#5a67ad" />
          <rect x="22" y="64" width="150" height="26" rx="6" fill="#f2a7b6" />
          <rect x="40" y="36" width="150" height="26" rx="6" fill="#aab4e6" />
          <rect x="52" y="44" width="52" height="5" rx="2.5" fill="#fff" opacity=".8" />
          <rect x="34" y="72" width="40" height="5" rx="2.5" fill="#fff" opacity=".8" />
          <rect x="42" y="100" width="60" height="5" rx="2.5" fill="#fff" opacity=".7" />
        </svg>
      </section>

      <section>
        <div className="section-head">
          <h2>Mis cursos</h2>
          <Link to="/mis-cursos" className="btn ghost small">
            Ver todos <ArrowRight size={14} aria-hidden="true" />
          </Link>
        </div>
        {state.status === "loading" && <Spinner label="Cargando tus cursos…" />}
        {state.status === "error" && <ErrorState error={state.error} onRetry={refresh} />}
        {state.status === "ok" &&
          (state.mine.length === 0 ? (
            <div className="panel">
              <EmptyState title="Aún no estás inscrito en ningún curso">Busca uno en el catálogo y pulsa «Inscribirme».</EmptyState>
            </div>
          ) : (
            <div className="cards">
              {state.mine.slice(0, 3).map((m, i) => (
                <Link key={m.enrollment.id} to={m.course ? `/cursos/${m.course.id}` : "/mis-cursos"} className={`course-card ${gradiente(i)}`}>
                  <strong>{m.course?.title ?? "Curso no disponible"}</strong>
                  <span>
                    {m.progress ? `${m.progress.completed_count} de ${m.progress.total_count} recursos` : "Sin datos de progreso"}
                  </span>
                  <ProgressBar value={m.progress?.percentage ?? 0} label={`Progreso de ${m.course?.title ?? "curso"}`} />
                  <small>{Math.round(m.progress?.percentage ?? 0)} % completado</small>
                </Link>
              ))}
            </div>
          ))}
      </section>

      <section>
        <div className="section-head">
          <h2>Cursos del catálogo</h2>
          <Link to="/catalogo" className="btn ghost small">
            Ver todos <ArrowRight size={14} aria-hidden="true" />
          </Link>
        </div>
        {state.status === "ok" && (
          <div className="panel table-panel">
            {state.catalog.length === 0 ? (
              <EmptyState title="Todavía no hay cursos publicados" />
            ) : (
              <table>
                <thead>
                  <tr>
                    <th scope="col">Curso</th>
                    <th scope="col">Descripción</th>
                    <th scope="col" />
                  </tr>
                </thead>
                <tbody>
                  {state.catalog.slice(0, 5).map((c) => (
                    <tr key={c.id}>
                      <th scope="row">
                        <Link to={`/cursos/${c.id}`}>{c.title}</Link>
                      </th>
                      <td className="clip">{c.description}</td>
                      <td className="right">
                        <EnrollButton course={c} compact />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}
      </section>
    </>
  );
}
