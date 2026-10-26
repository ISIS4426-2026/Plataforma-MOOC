import { Check, CircleDashed, FileText, HelpCircle, Image as ImageIcon, Link2, Loader2, Music, PlayCircle, TriangleAlert } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Link, Navigate, useNavigate, useParams } from "react-router-dom";
import { ApiError } from "../api/client";
import { courseProgress, downloadUrl, getCourse, heartbeat, loadCourseTree } from "../api/endpoints";
import type { Progress, Resource } from "../api/types";
import { Badge, EmptyState, ErrorState, PageTitle, ProgressBar, Spinner } from "../components/ui";
import { useAsync } from "../hooks/useAsync";
import { Markdown } from "../lib/markdown";
import { disponible } from "../lib/resources";
import { EnrollButton } from "./EnrollButton";
import { QuizView } from "./QuizView";
import { useStudentData } from "./StudentData";
import { VideoPlayer } from "./VideoPlayer";

function IconoRecurso({ r }: { r: Resource }) {
  if (!disponible(r)) return r.processing_status === "failed" ? <TriangleAlert size={16} /> : <Loader2 size={16} className="spin" />;
  switch (r.type) {
    case "video":
      return <PlayCircle size={16} />;
    case "quiz":
      return <HelpCircle size={16} />;
    case "image":
      return <ImageIcon size={16} />;
    case "audio":
      return <Music size={16} />;
    case "external_link":
    case "iframe":
      return <Link2 size={16} />;
    default:
      return <FileText size={16} />;
  }
}

export default function Course() {
  const { courseId = "", resourceId } = useParams();
  const { state: mis } = useStudentData();
  const curso = useAsync(() => getCourse(courseId), [courseId]);

  if (curso.status === "loading" || mis.status === "loading") return <Spinner label="Cargando el curso…" />;
  if (curso.status === "error") {
    return curso.error instanceof ApiError && curso.error.status === 404 ? (
      <div className="panel">
        <EmptyState title="Este curso no existe o no está publicado">
          <Link to="/catalogo" className="btn small">
            Volver al catálogo
          </Link>
        </EmptyState>
      </div>
    ) : (
      <ErrorState error={curso.error} onRetry={curso.reload} />
    );
  }
  if (mis.status === "error") return <ErrorState error={mis.error} />;

  const c = curso.data;
  const inscrito = mis.mine.some((m) => m.enrollment.course_stable_id === c.stable_id);

  if (!inscrito) {
    return (
      <>
        <PageTitle title={c.title} subtitle={`Versión ${c.version}`} action={<EnrollButton course={c} />} />
        <div className="panel pad">
          <p>{c.description || "Este curso no tiene descripción."}</p>
          <p className="muted">Inscríbete para ver sus módulos, unidades y recursos.</p>
        </div>
      </>
    );
  }
  return <Classroom courseId={c.id} title={c.title} description={c.description} resourceId={resourceId} />;
}

function Classroom({ courseId, title, description, resourceId }: { courseId: string; title: string; description?: string; resourceId?: string }) {
  const navigate = useNavigate();
  const { refresh } = useStudentData();
  const arbol = useAsync(() => loadCourseTree(courseId), [courseId]);
  const [progreso, setProgreso] = useState<Progress | null>(null);

  useEffect(() => {
    let cancelado = false;
    courseProgress(courseId)
      .then((p) => !cancelado && setProgreso(p))
      .catch(() => undefined);
    return () => {
      cancelado = true;
    };
  }, [courseId]);

  const alAvanzar = useCallback(
    (p: Progress) => {
      setProgreso(p);
      refresh();
    },
    [refresh],
  );

  if (arbol.status === "loading") return <Spinner label="Cargando los módulos…" />;
  if (arbol.status === "error") return <ErrorState error={arbol.error} onRetry={arbol.reload} />;

  const recursos = arbol.data.modules.flatMap((m) => m.units.flatMap((u) => u.resources));
  if (recursos.length === 0) {
    return (
      <>
        <PageTitle title={title} subtitle={description} />
        <div className="panel">
          <EmptyState title="Este curso todavía no tiene contenido" />
        </div>
      </>
    );
  }

  const completados = new Set(progreso?.completed_resource_stable_ids ?? []);
  const actual = recursos.find((r) => r.id === resourceId);
  if (resourceId && !actual) return <Navigate to={`/cursos/${courseId}`} replace />;
  const seleccionado = actual ?? recursos.find((r) => disponible(r) && !completados.has(r.stable_id)) ?? recursos[0]!;

  return (
    <>
      <PageTitle
        title={title}
        subtitle={description}
        action={
          progreso?.is_approved ? (
            <Badge tone="ok">Curso aprobado</Badge>
          ) : undefined
        }
      />
      <div className="panel pad progress-head">
        <ProgressBar value={progreso?.percentage ?? 0} label="Progreso del curso" />
        <small className="muted">
          {progreso ? `${progreso.completed_count} de ${progreso.total_count} recursos obligatorios · ${Math.round(progreso.percentage)} %` : "Calculando progreso…"}
        </small>
      </div>

      <div className="classroom">
        <nav className="panel tree" aria-label="Contenido del curso">
          {arbol.data.modules.map((m) => (
            <section key={m.id}>
              <h3>{m.title}</h3>
              {m.units.map((u) => (
                <div key={u.id}>
                  <h4>{u.title}</h4>
                  <ul>
                    {u.resources.map((r) => {
                      const hecho = completados.has(r.stable_id);
                      return (
                        <li key={r.id}>
                          <button
                            type="button"
                            className={`res${r.id === seleccionado.id ? " active" : ""}`}
                            aria-current={r.id === seleccionado.id ? "true" : undefined}
                            onClick={() => navigate(`/cursos/${courseId}/${r.id}`)}
                          >
                            <IconoRecurso r={r} />
                            <span>{r.title}</span>
                            {!disponible(r) ? <small>{r.processing_status === "failed" ? "falló" : "procesando"}</small> : hecho ? <Check size={14} aria-label="Completado" /> : <CircleDashed size={14} aria-label="Pendiente" />}
                          </button>
                        </li>
                      );
                    })}
                  </ul>
                </div>
              ))}
            </section>
          ))}
        </nav>

        <article className="panel pad viewer" aria-live="polite">
          <h2>{seleccionado.title}</h2>
          <ResourceViewer key={seleccionado.id} courseId={courseId} resource={seleccionado} completado={completados.has(seleccionado.stable_id)} onProgress={alAvanzar} />
        </article>
      </div>
    </>
  );
}

function ResourceViewer({ courseId, resource, completado, onProgress }: { courseId: string; resource: Resource; completado: boolean; onProgress: (p: Progress) => void }) {
  const inicio = useRef(Date.now());
  const [marcando, setMarcando] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const registrar = useCallback(
    async (segundos: number, terminado: boolean) => {
      try {
        onProgress(await heartbeat(resource.id, Math.min(3600, Math.max(0, Math.round(segundos))), terminado));
      } catch (e) {
        if (terminado) setError((e as Error).message);
      }
    },
    [resource.id, onProgress],
  );

  async function marcar() {
    setMarcando(true);
    setError(null);
    await registrar((Date.now() - inicio.current) / 1000, true);
    setMarcando(false);
  }

  if (!disponible(resource)) {
    return resource.processing_status === "failed" ? (
      <p role="alert" className="error">
        El procesamiento de este archivo falló. Avisa a tu profesor para que lo suba de nuevo.
      </p>
    ) : (
      <div className="state" role="status">
        <Loader2 className="spin" size={22} aria-hidden="true" />
        <strong>Este recurso se está procesando</strong>
        <span className="muted">Todavía no está disponible. Vuelve en unos minutos.</span>
      </div>
    );
  }

  const boton = (
    <div className="actions">
      {completado ? (
        <Badge tone="ok">
          <Check size={12} aria-hidden="true" /> Completado
        </Badge>
      ) : (
        <button type="button" className="btn" disabled={marcando} onClick={marcar}>
          {marcando ? "Guardando…" : "Marcar como completado"}
        </button>
      )}
      {error && (
        <small role="alert" className="error">
          {error}
        </small>
      )}
    </div>
  );

  switch (resource.type) {
    case "text":
      return (
        <>
          <Markdown texto={resource.content_text ?? ""} />
          {boton}
        </>
      );
    case "video":
      return (
        <>
          <VideoPlayer resourceStableId={resource.stable_id} onBeat={(s, t) => void registrar(s, t)} />
          <p className="muted">El progreso se registra al terminar el video.</p>
        </>
      );
    case "quiz":
      return <QuizView resourceId={resource.id} onGraded={() => void courseProgress(courseId).then(onProgress).catch(() => undefined)} />;
    case "external_link":
      return (
        <>
          {resource.content_text?.startsWith("https://") ? (
            <a href={resource.content_text} target="_blank" rel="noopener noreferrer" className="btn">
              Abrir enlace
            </a>
          ) : (
            <p className="muted">Este recurso no tiene un enlace válido.</p>
          )}
          {boton}
        </>
      );
    default:
      return (
        <>
          <Archivo resource={resource} />
          {boton}
        </>
      );
  }
}

/** Documentos, imagenes y audio: salen del almacenamiento de objetos por una URL firmada de corta vida. */
function Archivo({ resource }: { resource: Resource }) {
  const url = useAsync(() => downloadUrl(resource.id), [resource.id]);
  if (url.status === "loading") return <Spinner label="Preparando el archivo…" />;
  if (url.status === "error") {
    return url.error instanceof ApiError && url.error.status === 403 ? (
      <p role="alert" className="error">
        Este archivo no se puede descargar.
      </p>
    ) : (
      <ErrorState error={url.error} onRetry={url.reload} />
    );
  }
  const href = url.data.download_url;
  if (resource.type === "image") return <img src={href} alt={resource.title} className="media" />;
  if (resource.type === "audio") return <audio controls src={href} aria-label={resource.title} />;
  return (
    <a href={href} target="_blank" rel="noopener noreferrer" className="btn">
      Abrir / descargar archivo
    </a>
  );
}
