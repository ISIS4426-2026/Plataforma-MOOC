import { APP_VERSION } from "../config";
import { useAuth } from "../auth/AuthContext";

/** Aviso global: sesion vencida, sin permiso, servidor caido. */
export function NoticeBanner() {
  const { notice, clearNotice } = useAuth();
  if (!notice) return null;
  return (
    <div role="status" className={`notice ${notice.kind}`}>
      <span>{notice.text}</span>
      <button type="button" className="link" onClick={clearNotice} aria-label="Cerrar aviso">
        ×
      </button>
    </div>
  );
}

/**
 * La version del build es visible en la interfaz (criterio de D1): es el commit que lo
 * produjo, el mismo que queda registrado en la evidencia y en /version.json.
 */
export function Footer() {
  return (
    <footer className="footer" data-testid="app-version">
      Versión <code>{APP_VERSION.commit}</code> · compilada {new Date(APP_VERSION.builtAt).toLocaleString("es-CO")}
    </footer>
  );
}
