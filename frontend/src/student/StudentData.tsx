import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { courseIndex, courseProgress, myEnrollments } from "../api/endpoints";
import type { Course, Enrollment, Progress } from "../api/types";

export interface MyCourse {
  enrollment: Enrollment;
  /** Null si el curso ya no esta publicado (no aparece en el catalogo). */
  course: Course | null;
  progress: Progress | null;
}

type State =
  | { status: "loading" }
  | { status: "ok"; mine: MyCourse[]; catalog: Course[] }
  | { status: "error"; error: Error };

interface StudentDataValue {
  state: State;
  refresh: () => void;
}

const Ctx = createContext<StudentDataValue | null>(null);

/**
 * «Mis cursos» une tres fuentes: inscripciones (solo traen course_stable_id), el catalogo (da el
 * titulo y el id con que se pide el progreso) y el progreso de cada curso. Se carga una sola vez
 * para el panel derecho, el inicio y «mis cursos», y se refresca tras inscribirse o avanzar.
 */
export function StudentDataProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>({ status: "loading" });
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [enrollments, catalog] = await Promise.all([myEnrollments(), courseIndex(tick > 0)]);
        const activos = enrollments.filter((e) => e.status === "active");
        const mine = await Promise.all(
          activos.map(async (enrollment): Promise<MyCourse> => {
            const course = catalog.find((c) => c.stable_id === enrollment.course_stable_id) ?? null;
            let progress: Progress | null = null;
            if (course) {
              try {
                progress = await courseProgress(course.id);
              } catch {
                progress = null; // el curso se muestra igual; el progreso queda «sin dato»
              }
            }
            return { enrollment, course, progress };
          }),
        );
        if (!cancelled) setState({ status: "ok", mine, catalog });
      } catch (error) {
        if (!cancelled) setState({ status: "error", error: error as Error });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [tick]);

  const refresh = useCallback(() => setTick((t) => t + 1), []);
  const value = useMemo(() => ({ state, refresh }), [state, refresh]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useStudentData(): StudentDataValue {
  const v = useContext(Ctx);
  if (!v) throw new Error("useStudentData debe usarse dentro de <StudentDataProvider>");
  return v;
}
