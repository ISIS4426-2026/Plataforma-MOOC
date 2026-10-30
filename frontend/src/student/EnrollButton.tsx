import { useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { ApiError } from "../api/client";
import { enroll } from "../api/endpoints";
import { newIdempotencyKey } from "../api/client";
import type { Course } from "../api/types";
import { useStudentData } from "./StudentData";

/**
 * Inscribirse. La Idempotency-Key se fija al montar el boton y se reutiliza en cada clic: un doble
 * clic o un reintento tras un corte devuelve la misma inscripcion y no crea otra (la API ademas es
 * idempotente por (estudiante, curso)).
 */
export function EnrollButton({ course, compact = false }: { course: Course; compact?: boolean }) {
  const { state, refresh } = useStudentData();
  const navigate = useNavigate();
  const key = useRef(newIdempotencyKey());
  const [enviando, setEnviando] = useState(false);
  const enCurso = useRef(false);
  const [error, setError] = useState<string | null>(null);

  const inscrito = state.status === "ok" && state.mine.some((m) => m.enrollment.course_stable_id === course.stable_id);

  if (inscrito) {
    return (
      <button type="button" className={`btn ${compact ? "small" : ""}`} onClick={() => navigate(`/cursos/${course.id}`)}>
        Ir al curso
      </button>
    );
  }

  async function inscribir() {
    if (enCurso.current) return; // doble clic: una sola peticion
    enCurso.current = true;
    setEnviando(true);
    setError(null);
    try {
      await enroll(course.id, key.current);
      refresh();
      navigate(`/cursos/${course.id}`);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "No se pudo completar la inscripción.");
    } finally {
      enCurso.current = false;
      setEnviando(false);
    }
  }

  return (
    <span className="enroll">
      <button type="button" className={`btn ${compact ? "small" : ""}`} disabled={enviando} onClick={inscribir}>
        {enviando ? "Inscribiendo…" : "Inscribirme"}
      </button>
      {error && (
        <small role="alert" className="error">
          {error}
        </small>
      )}
    </span>
  );
}
