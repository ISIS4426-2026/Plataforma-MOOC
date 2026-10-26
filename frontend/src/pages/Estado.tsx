import { useEffect, useState } from "react";
import { ApiError, request } from "../api/client";
import { PageTitle, Spinner } from "../components/ui";

interface Health {
  status: string;
  timestamp: string;
  services: Record<string, string>;
}

type Sonda =
  | { estado: "cargando" }
  | { estado: "ok"; salud: Health; instanceId?: string; requestId?: string }
  | { estado: "error"; mensaje: string };

/**
 * Estado del sistema: que replica de la API contesta. Es lo que se usa para demostrar el reparto
 * del balanceador entre instancias (X-Instance-ID) sin abrir una consola.
 */
export default function Estado() {
  const [sonda, setSonda] = useState<Sonda>({ estado: "cargando" });
  const [vuelta, setVuelta] = useState(0);

  useEffect(() => {
    const control = new AbortController();
    setSonda({ estado: "cargando" });
    request<Health>("/health", { signal: control.signal })
      .then(({ data, instanceId, requestId }) => setSonda({ estado: "ok", salud: data, instanceId, requestId }))
      .catch((e) => {
        if (!control.signal.aborted) setSonda({ estado: "error", mensaje: e instanceof ApiError ? e.message : "Error inesperado." });
      });
    return () => control.abort();
  }, [vuelta]);

  return (
    <>
      <PageTitle title="Estado del sistema" subtitle="Comprobación de la conexión con la API." />
      <div className="panel pad">
        {sonda.estado === "cargando" && <Spinner label="Comprobando…" />}
        {sonda.estado === "error" && (
          <p role="alert" className="error">
            No se pudo comprobar la API: {sonda.mensaje}
          </p>
        )}
        {sonda.estado === "ok" && (
          <dl className="datos">
            <dt>Estado</dt>
            <dd>{sonda.salud.status}</dd>
            <dt>Servicios</dt>
            <dd>{Object.entries(sonda.salud.services).map(([k, v]) => `${k}: ${v}`).join(" · ") || "—"}</dd>
            <dt>Réplica que respondió</dt>
            <dd data-testid="instancia">{sonda.instanceId ?? "no informada"}</dd>
            <dt>ID de la petición</dt>
            <dd>{sonda.requestId ?? "—"}</dd>
          </dl>
        )}
        <button type="button" className="btn ghost" onClick={() => setVuelta((v) => v + 1)}>
          Volver a comprobar
        </button>
      </div>
    </>
  );
}
