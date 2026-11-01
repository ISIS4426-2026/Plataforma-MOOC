import { ArrowRight, PlusCircle } from "lucide-react";
import { Link } from "react-router-dom";
import { HeroArt } from "../components/HeroArt";
import { useAuth } from "../auth/AuthContext";
import { primerNombre } from "../lib/names";
import { Badge, EmptyState, ErrorState, Spinner, gradiente } from "../components/ui";
import { ETIQUETA_ESTADO, TONO_ESTADO, useProfessorData } from "./ProfessorData";

export default function ProfessorDashboard() {
  const { user } = useAuth();
  const { state, refresh } = useProfessorData();
  const nombre = primerNombre(user?.full_name ?? "");

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
        <HeroArt />
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
          (state.courses.filter((c) => c.status === "published").length === 0 ? (
            <div className="panel">
              <EmptyState title="Todavía no tienes cursos publicados">
                <Link to="/profesor/cursos/nuevo" className="btn small">
                  Crear mi primer curso
                </Link>
              </EmptyState>
            </div>
          ) : (
            <div className="cards">
              {/* El inicio muestra lo publicado; borradores y despublicados estan en «Mis cursos» y en «Por terminar». */}
              {state.courses.filter((c) => c.status === "published").slice(0, 4).map((c, i) => (
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
