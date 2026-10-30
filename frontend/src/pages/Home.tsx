import { useEffect, useState } from "react";
import { ApiError, request } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { Footer, NoticeBanner } from "../components/Chrome";

interface Health {
  status: string;
  timestamp: string;
  services: Record<string, string>;
}

type Sonda =
  | { estado: "cargando" }
  | { estado: "ok"; salud: Health; instanceId?: string; requestId?: string }
  | { estado: "error"; mensaje: string };

// /health es publico: no prueba la sesion. /me/enrollments exige sesion (cookie) y existe para
// cualquier rol, asi que sirve de sonda autenticada hasta que D2 traiga las vistas reales.
type SondaSesion =
  | { estado: "cargando" }
  | { estado: "ok"; inscripciones: number; instanceId?: string }
  | { estado: "error"; mensaje: string };

const ETIQUETA_ROL = { estudiante: "Estudiante", profesor: "Profesor", administrador: "Administrador" } as const;

/**
 * Pantalla provisional de D1: demuestra sesion, conexion con la API y version del build.
 * Las vistas reales de estudiante (D2) y profesor (D3) la reemplazan por la navegacion por rol.
 */
export default function Home() {
  const { user, logout } = useAuth();
  const [sonda, setSonda] = useState<Sonda>({ estado: "cargando" });
  const [sesion, setSesion] = useState<SondaSesion>({ estado: "cargando" });
  const [saliendo, setSaliendo] = useState(false);

  useEffect(() => {
    const control = new AbortController();
    request<{ items: unknown[] }>("/me/enrollments", { signal: control.signal })
      .then(({ data, instanceId }) => setSesion({ estado: "ok", inscripciones: data.items.length, instanceId }))
      .catch((e) => {
        if (control.signal.aborted) return;
        // Un 401 ya lo trata el manejador global (cierra la sesion local y avisa); aqui solo se
        // deja constancia en la sonda.
        setSesion({ estado: "error", mensaje: e instanceof ApiError ? e.message : "Error inesperado." });
      });
    request<Health>("/health", { signal: control.signal })
      .then(({ data, instanceId, requestId }) => setSonda({ estado: "ok", salud: data, instanceId, requestId }))
      .catch((e) => {
        if (control.signal.aborted) return;
        setSonda({ estado: "error", mensaje: e instanceof ApiError ? e.message : "Error inesperado." });
      });
    return () => control.abort();
  }, []);

  if (!user) return null;

  return (
    <div className="page">
      <header className="topbar">
        <strong>Plataforma MOOC</strong>
        <span>
          {user.full_name} · {ETIQUETA_ROL[user.role] ?? user.role}
        </span>
        <button
          type="button"
          disabled={saliendo}
          onClick={async () => {
            setSaliendo(true);
            await logout();
          }}
        >
          Cerrar sesión
        </button>
      </header>
      <main className="card">
        <NoticeBanner />
        <h1>Hola, {user.full_name}</h1>
        <p className="muted">Sesión iniciada. Las vistas de estudiante y profesor llegan en D2 y D3.</p>

        <h2>Sesión en el servidor</h2>
        {sesion.estado === "cargando" && <p>Comprobando…</p>}
        {sesion.estado === "error" && (
          <p role="alert" className="error">
            No se pudo comprobar la sesión: {sesion.mensaje}
          </p>
        )}
        {sesion.estado === "ok" && (
          <p data-testid="sesion-ok">
            Sesión válida: el servidor reconoce tu cookie ({sesion.inscripciones}{" "}
            {sesion.inscripciones === 1 ? "inscripción" : "inscripciones"}).
          </p>
        )}

        <h2>Conexión con la API</h2>
        {sonda.estado === "cargando" && <p>Comprobando…</p>}
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
            <dd>{sonda.instanceId ?? "no informada"}</dd>
            <dt>ID de la petición</dt>
            <dd>{sonda.requestId ?? "—"}</dd>
          </dl>
        )}
      </main>
      <Footer />
    </div>
  );
}
