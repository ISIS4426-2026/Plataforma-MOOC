import { useCallback, useEffect, useRef, useState, type DependencyList } from "react";
import { ApiError } from "../api/client";

export type AsyncState<T> =
  | { status: "loading" }
  | { status: "ok"; data: T }
  | { status: "error"; error: ApiError | Error };

/**
 * Carga de datos con los tres estados que toda vista debe mostrar (cargando, ok, error).
 * `reload` vuelve a pedir sin parpadeo: conserva los datos anteriores hasta que llegan los nuevos.
 */
export function useAsync<T>(load: () => Promise<T>, deps: DependencyList): AsyncState<T> & { reload: () => void } {
  const [state, setState] = useState<AsyncState<T>>({ status: "loading" });
  const [tick, setTick] = useState(0);
  const first = useRef(true);

  useEffect(() => {
    let cancelled = false;
    // En un cambio de dependencias se vuelve a «cargando»; en un reload se mantienen los datos.
    if (first.current) first.current = false;
    load()
      .then((data) => !cancelled && setState({ status: "ok", data }))
      .catch((error) => !cancelled && setState({ status: "error", error }));
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, tick]);

  // Al cambiar las dependencias (no en un reload) se descartan los datos del recurso anterior.
  const depsKey = JSON.stringify(deps);
  const lastDeps = useRef(depsKey);
  useEffect(() => {
    if (lastDeps.current !== depsKey) {
      lastDeps.current = depsKey;
      setState({ status: "loading" });
    }
  }, [depsKey]);

  const reload = useCallback(() => setTick((t) => t + 1), []);
  return { ...state, reload } as AsyncState<T> & { reload: () => void };
}
