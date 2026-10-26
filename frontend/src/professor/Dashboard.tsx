import { ArrowRight, PlusCircle } from "lucide-react";
import { Link } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Badge, EmptyState, ErrorState, Spinner, gradiente } from "../components/ui";
import { ETIQUETA_ESTADO, TONO_ESTADO, useProfessorData } from "./ProfessorData";

export default function ProfessorDashboard() {
  const { user } = useAuth();
  const { state, refresh } = useProfessorData();
  const nombre = user?.full_name.split(" ")[0] ?? "";

  return (
    <>
      <section className="panel hero">
        <div>
          <h1>¡Hola, {nombre}!</h1>
          <p className="muted">Crea un curso, súbele contenido y publícalo cuando esté listo para que los estudiantes lo encuentren.</p>
          <Link to="/profesor/cursos/nuevo" className="btn">
            <PlusCircle size={16} aria-hidden="true" /> Crear curso
          </Link>
        </div>
        <svg className="hero-art" viewBox="0 0 220 150" aria-hidden="true">
          <rect x="30" y="92" width="150" height="26" rx="6" fill="#5a67ad" />
          <rect x="22" y="64" width="150" height="26" rx="6" fill="#f2a7b6" />
          <rect x="40" y="36" width="150" height="26" rx="6" fill="#aab4e6" />
        </svg>
      </section>

      <section>
        <div className="section-head">
          <h2>Mis cursos</h2>
          <Link to="/profesor/cursos" className="btn ghost small">
            Ver todos <ArrowRight size={14} aria-hidden="true" />
          </Link>
        </div>
        {state.status === "loading" && <Spinner label="Cargando tus cursos…" />}
        {state.status === "error" && <ErrorState error={state.error} onRetry={refresh} />}
        {state.status === "ok" &&
          (state.courses.length === 0 ? (
            <div className="panel">
              <EmptyState title="Todavía no has creado ningún curso">
                <Link to="/profesor/cursos/nuevo" className="btn small">
                  Crear mi primer curso
                </Link>
              </EmptyState>
            </div>
          ) : (
            <div className="cards">
              {state.courses.slice(0, 4).map((c, i) => (
                <Link key={c.id} to={`/profesor/cursos/${c.id}`} className={`course-card ${gradiente(i)}`}>
                  <strong>{c.title}</strong>
                  <span>{c.description || "Sin descripción"}</span>
                  <small>
                    <Badge tone={TONO_ESTADO[c.status]}>{ETIQUETA_ESTADO[c.status]}</Badge>
                  </small>
                </Link>
              ))}
            </div>
          ))}
      </section>
    </>
  );
}
