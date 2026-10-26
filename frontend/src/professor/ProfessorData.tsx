import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import type { Course } from "../api/types";
import { allMyCourses } from "./api";

type State = { status: "loading" } | { status: "ok"; courses: Course[] } | { status: "error"; error: Error };

interface Value {
  state: State;
  refresh: () => void;
}

const Ctx = createContext<Value | null>(null);

/** «Mis cursos» del profesor, cargados una vez para el inicio, la lista y el panel derecho. */
export function ProfessorDataProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>({ status: "loading" });
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    allMyCourses()
      .then((courses) => !cancelled && setState({ status: "ok", courses }))
      .catch((error) => !cancelled && setState({ status: "error", error }));
    return () => {
      cancelled = true;
    };
  }, [tick]);

  const refresh = useCallback(() => setTick((t) => t + 1), []);
  const value = useMemo(() => ({ state, refresh }), [state, refresh]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useProfessorData(): Value {
  const v = useContext(Ctx);
  if (!v) throw new Error("useProfessorData debe usarse dentro de <ProfessorDataProvider>");
  return v;
}

export const ETIQUETA_ESTADO: Record<Course["status"], string> = {
  draft: "Borrador",
  published: "Publicado",
  unpublished: "Despublicado",
};

export const TONO_ESTADO: Record<Course["status"], "warn" | "ok" | "info"> = {
  draft: "warn",
  published: "ok",
  unpublished: "info",
};
