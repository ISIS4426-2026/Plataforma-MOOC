import { useEffect, useRef, useState } from "react";
import { ApiError, newIdempotencyKey } from "../api/client";
import type { Resource } from "../api/types";
import { ProgressBar } from "../components/ui";
import { EXTENSIONS_BY_TYPE, MAX_UPLOAD_BYTES, confirmUpload, extensionOf, presignUpload, putToStorage } from "./api";

type Fase =
  | { fase: "libre" }
  | { fase: "pidiendo" }
  | { fase: "subiendo"; fraccion: number }
  | { fase: "confirmando" }
  | { fase: "encolado" }
  | { fase: "error"; mensaje: string };

/**
 * Carga de un archivo en tres pasos, con el estado visible en cada uno:
 *   1. se pide a la API una URL firmada,
 *   2. el navegador sube el archivo DIRECTO al almacenamiento de objetos (no pasa por la API),
 *   3. se confirma a la API, que comprueba el objeto y encola el procesamiento.
 * El resultado final (procesando -> disponible) lo muestra la fila del recurso, que se actualiza sola.
 */
export function UploadControl({ resource, onQueued }: { resource: Resource; onQueued: () => void }) {
  const [estado, setEstado] = useState<Fase>({ fase: "libre" });
  const [archivo, setArchivo] = useState<File | null>(null);
  const confirmKey = useRef(newIdempotencyKey());
  const enCurso = useRef(false);
  const abortar = useRef<AbortController | null>(null);
  const extensiones = EXTENSIONS_BY_TYPE[resource.type] ?? [];

  // Si se sale de la pantalla a mitad de subida, se cancela la transferencia.
  useEffect(() => () => abortar.current?.abort(), []);

  function elegir(f: File | null) {
    setArchivo(f);
    setEstado({ fase: "libre" });
    confirmKey.current = newIdempotencyKey();
    if (!f) return;
    if (!extensiones.includes(extensionOf(f.name))) {
      setEstado({ fase: "error", mensaje: `Este recurso acepta archivos ${extensiones.join(", ")}.` });
    } else if (f.size > MAX_UPLOAD_BYTES) {
      setEstado({ fase: "error", mensaje: "El archivo supera el máximo de 1 GiB." });
    } else if (f.size === 0) {
      setEstado({ fase: "error", mensaje: "El archivo está vacío." });
    }
  }

  async function subir() {
    if (!archivo || enCurso.current) return;
    enCurso.current = true;
    abortar.current = new AbortController();
    try {
      setEstado({ fase: "pidiendo" });
      const ticket = await presignUpload(resource.id, archivo);
      setEstado({ fase: "subiendo", fraccion: 0 });
      await putToStorage(ticket, archivo, (fraccion) => setEstado({ fase: "subiendo", fraccion }), abortar.current.signal);
      setEstado({ fase: "confirmando" });
      await confirmUpload(resource.id, ticket.object_key, confirmKey.current);
      setEstado({ fase: "encolado" });
      setArchivo(null);
      onQueued();
    } catch (e) {
      const mensaje =
        e instanceof ApiError && e.status === 400 ? "La API rechazó el archivo (tipo o tamaño no válidos)." : e instanceof ApiError && e.status === 409 ? "El curso está publicado: despublícalo para cambiar sus archivos." : (e as Error).message;
      setEstado({ fase: "error", mensaje });
    } finally {
      enCurso.current = false;
    }
  }

  const ocupado = estado.fase === "pidiendo" || estado.fase === "subiendo" || estado.fase === "confirmando";
  const listo = archivo !== null && estado.fase !== "error";

  return (
    <div className="upload">
      <input
        type="file"
        accept={extensiones.join(",")}
        aria-label={`Archivo para «${resource.title}»`}
        disabled={ocupado}
        onChange={(e) => elegir(e.target.files?.[0] ?? null)}
      />
      <button type="button" className="btn small" disabled={!listo || ocupado} onClick={subir}>
        {estado.fase === "error" && archivo ? "Reintentar subida" : "Subir archivo"}
      </button>
      {estado.fase === "pidiendo" && <small role="status">Pidiendo permiso…</small>}
      {estado.fase === "subiendo" && (
        <div role="status" className="upload-progress">
          <ProgressBar value={estado.fraccion * 100} label="Progreso de la subida" />
          <small>Subiendo… {Math.round(estado.fraccion * 100)} %</small>
        </div>
      )}
      {estado.fase === "confirmando" && <small role="status">Confirmando…</small>}
      {estado.fase === "encolado" && <small role="status">Archivo recibido: se está procesando.</small>}
      {estado.fase === "error" && (
        <small role="alert" className="error">
          {estado.mensaje}
        </small>
      )}
    </div>
  );
}
