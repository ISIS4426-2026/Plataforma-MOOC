import { Award } from "lucide-react";
import { Link } from "react-router-dom";
import { Badge, EmptyState, ErrorState, PageTitle, ProgressBar, Spinner, gradiente } from "../components/ui";
import { useStudentData } from "./StudentData";

export default function MyCourses() {
  const { state, refresh } = useStudentData();

  return (
    <>
      <PageTitle title="Mis cursos" subtitle="Los cursos en los que estás inscrito y tu avance en cada uno." />
      {state.status === "loading" && <Spinner label="Cargando tus cursos…" />}
      {state.status === "error" && <ErrorState error={state.error} onRetry={refresh} />}
      {state.status === "ok" &&
        (state.mine.length === 0 ? (
          <div className="panel">
            <EmptyState title="Aún no estás inscrito en ningún curso">
              <Link to="/catalogo" className="btn small">
                Ir al catálogo
              </Link>
            </EmptyState>
          </div>
        ) : (
          <div className="cards wide">
            {state.mine.map((m, i) => (
              <Link key={m.enrollment.id} to={m.course ? `/cursos/${m.course.id}` : "#"} className={`course-card ${gradiente(i)}`} aria-disabled={!m.course}>
                <strong>{m.course?.title ?? "Curso no disponible"}</strong>
                <span>{m.course ? m.course.description : "Este curso ya no está publicado."}</span>
                <ProgressBar value={m.progress?.percentage ?? 0} label={`Progreso de ${m.course?.title ?? "curso"}`} />
                <small>
                  {m.progress ? `${m.progress.completed_count} de ${m.progress.total_count} recursos · ${Math.round(m.progress.percentage)} %` : "Sin datos de progreso"}
                </small>
                {m.progress?.is_approved && (
                  <Badge tone="ok">
                    <Award size={12} aria-hidden="true" /> Aprobado
                  </Badge>
                )}
              </Link>
            ))}
          </div>
        ))}
    </>
  );
}
