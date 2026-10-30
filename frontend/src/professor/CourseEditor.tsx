import { CheckCircle2, ChevronDown, ChevronRight, Loader2, Plus, Trash2, TriangleAlert } from "lucide-react";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import { ApiError, newIdempotencyKey, type FieldProblem } from "../api/client";
import { loadCourseTree, type CourseTree } from "../api/endpoints";
import type { Course, Resource, ResourceType } from "../api/types";
import { Badge, EmptyState, ErrorState, PageTitle, Spinner } from "../components/ui";
import { useAsync } from "../hooks/useAsync";
import { TIPOS_CON_ARCHIVO, disponible, sinArchivo } from "../lib/resources";
import { createModule, createResource, createUnit, deleteModule, deleteResource, deleteUnit, getCourseWithEtag, publishCourse, unpublishCourse, updateCourse, type NewResource } from "./api";
import { ETIQUETA_ESTADO, TONO_ESTADO, useProfessorData } from "./ProfessorData";
import { QuizEditor } from "./QuizEditor";
import { UploadControl } from "./UploadControl";

const TIPOS: Array<{ valor: ResourceType; etiqueta: string }> = [
  { valor: "text", etiqueta: "Texto" },
  { valor: "video", etiqueta: "Video" },
  { valor: "pdf", etiqueta: "PDF" },
  { valor: "presentation", etiqueta: "Presentación" },
  { valor: "image", etiqueta: "Imagen" },
  { valor: "audio", etiqueta: "Audio" },
  { valor: "downloadable", etiqueta: "Archivo descargable" },
  { valor: "external_link", etiqueta: "Enlace externo" },
  { valor: "quiz", etiqueta: "Cuestionario" },
];
const ETIQUETA_TIPO = Object.fromEntries(TIPOS.map((t) => [t.valor, t.etiqueta])) as Record<ResourceType, string>;
const CON_ARCHIVO: ResourceType[] = TIPOS_CON_ARCHIVO;

function SinPermiso({ titulo }: { titulo: string }) {
  return (
    <div className="panel">
      <EmptyState title={titulo}>
        Solo puedes ver y editar los cursos que creaste tú.{" "}
        <Link to="/profesor/cursos" className="btn small">
          Ir a mis cursos
        </Link>
      </EmptyState>
    </div>
  );
}

export default function CourseEditor() {
  const { courseId = "" } = useParams();
  const { state: mios } = useProfessorData();
  // Los datos basicos de un curso PUBLICADO son publicos (son los del catalogo), asi que pedirlos no
  // basta para saber si es mio. La lista del profesor (GET /me/courses) si lo dice: si el curso no esta
  // ahi, no se muestra el editor. Es solo cortesia de la interfaz: la autoridad es el servidor, que
  // devuelve 403 a cualquier modificacion de un curso ajeno (comprobado, ver evidencia D4).
  if (mios.status === "loading") return <Spinner label="Cargando…" />;
  if (mios.status === "ok" && !mios.courses.some((c) => c.id === courseId)) {
    return <SinPermiso titulo="No tienes permiso para editar este curso" />;
  }
  return <EditorDeCurso courseId={courseId} />;
}

function EditorDeCurso({ courseId }: { courseId: string }) {
  const curso = useAsync(() => getCourseWithEtag(courseId), [courseId]);
  const arbol = useAsync(() => loadCourseTree(courseId), [courseId]);

  // Mientras algun archivo se procesa, la lista se actualiza sola: el profesor ve el paso a disponible.
  const procesando = arbol.status === "ok" && arbol.data.modules.some((m) => m.units.some((u) => u.resources.some((r) => r.processing_status === "pending" || r.processing_status === "processing")));
  const recargar = arbol.status === "ok" || arbol.status === "error" ? arbol.reload : undefined;
  useEffect(() => {
    if (!procesando || !recargar) return;
    const t = window.setInterval(recargar, 4000);
    return () => window.clearInterval(t);
  }, [procesando, recargar]);

  if (curso.status === "loading") return <Spinner label="Cargando el curso…" />;
  if (curso.status === "error") {
    const e = curso.error;
    if (e instanceof ApiError && (e.status === 403 || e.status === 404)) {
      return <SinPermiso titulo={e.status === 403 ? "No tienes permiso para ver este curso" : "Este curso no existe"} />;
    }
    return <ErrorState error={e} onRetry={curso.reload} />;
  }

  return <Editor course={curso.data.course} etag={curso.data.etag} arbol={arbol} onCourseChanged={curso.reload} />;
}

type ArbolEstado = ReturnType<typeof useAsync<CourseTree>>;

function Editor({ course, etag, arbol, onCourseChanged }: { course: Course; etag?: string; arbol: ArbolEstado; onCourseChanged: () => void }) {
  const { refresh } = useProfessorData();
  const editable = course.status !== "published";

  return (
    <>
      <PageTitle
        title={course.title}
        subtitle={`Versión ${course.version}`}
        action={<Badge tone={TONO_ESTADO[course.status]}>{ETIQUETA_ESTADO[course.status]}</Badge>}
      />
      {!editable && (
        <p className="notice info" role="status">
          Este curso está publicado y es inmutable: despublícalo para editar su contenido.
        </p>
      )}
      <Datos course={course} etag={etag} editable={editable} onSaved={() => { refresh(); onCourseChanged(); }} />
      <Estructura courseId={course.id} editable={editable} arbol={arbol} />
      <Publicacion course={course} onChanged={() => { refresh(); onCourseChanged(); arbol.reload(); }} />
    </>
  );
}

// ---- Titulo y descripcion --------------------------------------------------------------

function Datos({ course, etag, editable, onSaved }: { course: Course; etag?: string; editable: boolean; onSaved: () => void }) {
  const [titulo, setTitulo] = useState(course.title);
  const [descripcion, setDescripcion] = useState(course.description ?? "");
  const [msg, setMsg] = useState<{ tipo: "ok" | "error"; texto: string } | null>(null);
  const [enviando, setEnviando] = useState(false);
  const ultimoEtag = useRef(etag);
  ultimoEtag.current = etag;

  async function guardar(e: FormEvent) {
    e.preventDefault();
    setEnviando(true);
    setMsg(null);
    try {
      const r = await updateCourse(course.id, titulo.trim(), descripcion.trim(), ultimoEtag.current);
      ultimoEtag.current = r.etag;
      setMsg({ tipo: "ok", texto: "Cambios guardados." });
      onSaved();
    } catch (err) {
      setMsg({
        tipo: "error",
        texto: err instanceof ApiError && err.status === 412 ? "El curso cambió desde que lo abriste. Recarga la página y vuelve a intentarlo." : (err as Error).message,
      });
    } finally {
      setEnviando(false);
    }
  }

  return (
    <form className="panel pad form" onSubmit={guardar}>
      <h2>Datos del curso</h2>
      <label htmlFor="c-titulo">Título</label>
      <input id="c-titulo" value={titulo} onChange={(e) => setTitulo(e.target.value)} disabled={!editable} required />
      <label htmlFor="c-desc">Descripción</label>
      <textarea id="c-desc" rows={3} value={descripcion} onChange={(e) => setDescripcion(e.target.value)} disabled={!editable} required />
      {msg && (
        <p role={msg.tipo === "error" ? "alert" : "status"} className={msg.tipo === "error" ? "error" : "ok-text"}>
          {msg.texto}
        </p>
      )}
      {editable && (
        <div className="actions">
          <button type="submit" className="btn small" disabled={enviando || !titulo.trim() || !descripcion.trim()}>
            {enviando ? "Guardando…" : "Guardar cambios"}
          </button>
        </div>
      )}
    </form>
  );
}

// ---- Estructura: modulos, unidades y recursos -------------------------------------------

function Estructura({ courseId, editable, arbol }: { courseId: string; editable: boolean; arbol: ArbolEstado }) {
  return (
    <section className="panel pad">
      <h2>Contenido</h2>
      {arbol.status === "loading" && <Spinner label="Cargando la estructura…" />}
      {arbol.status === "error" && <ErrorState error={arbol.error} onRetry={arbol.reload} />}
      {arbol.status === "ok" && (
        <>
          {arbol.data.modules.length === 0 && <p className="muted">El curso todavía no tiene módulos. Agrega el primero.</p>}
          {arbol.data.modules.map((m) => (
            <div key={m.id} className="modulo">
              <div className="row-head">
                <h3>{m.title}</h3>
                {editable && <BotonBorrar etiqueta={`el módulo «${m.title}»`} onConfirm={() => deleteModule(m.id)} onDone={arbol.reload} />}
              </div>
              {m.units.map((u) => (
                <div key={u.id} className="unidad">
                  <div className="row-head">
                    <h4>{u.title}</h4>
                    {editable && <BotonBorrar etiqueta={`la unidad «${u.title}»`} onConfirm={() => deleteUnit(u.id)} onDone={arbol.reload} />}
                  </div>
                  <ul className="recursos">
                    {u.resources.map((r) => (
                      <FilaRecurso key={r.id} recurso={r} editable={editable} onChanged={arbol.reload} />
                    ))}
                  </ul>
                  {u.resources.length === 0 && <p className="muted">Esta unidad todavía no tiene recursos.</p>}
                  {editable && <NuevoRecurso unitId={u.id} onCreated={arbol.reload} />}
                </div>
              ))}
              {editable && <AgregarTitulo etiqueta="Nueva unidad" onAdd={(t, k) => createUnit(m.id, t, k)} onDone={arbol.reload} />}
            </div>
          ))}
          {editable && <AgregarTitulo etiqueta="Nuevo módulo" onAdd={(t, k) => createModule(courseId, t, k)} onDone={arbol.reload} />}
        </>
      )}
    </section>
  );
}

function AgregarTitulo({ etiqueta, onAdd, onDone }: { etiqueta: string; onAdd: (titulo: string, key: string) => Promise<unknown>; onDone: () => void }) {
  const [titulo, setTitulo] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [enviando, setEnviando] = useState(false);
  const clave = useRef(newIdempotencyKey());
  const enCurso = useRef(false);

  async function enviar(e: FormEvent) {
    e.preventDefault();
    if (enCurso.current) return;
    enCurso.current = true;
    setEnviando(true);
    setError(null);
    try {
      await onAdd(titulo.trim(), clave.current);
      setTitulo("");
      clave.current = newIdempotencyKey();
      onDone();
    } catch (err) {
      setError((err as Error).message);
      clave.current = newIdempotencyKey();
    } finally {
      enCurso.current = false;
      setEnviando(false);
    }
  }

  return (
    <form className="add-title" onSubmit={enviar}>
      <input value={titulo} onChange={(e) => setTitulo(e.target.value)} placeholder={etiqueta} aria-label={etiqueta} />
      <button type="submit" className="btn ghost small" disabled={enviando || !titulo.trim()}>
        <Plus size={14} aria-hidden="true" /> {enviando ? "Agregando…" : "Agregar"}
      </button>
      {error && (
        <small role="alert" className="error">
          {error}
        </small>
      )}
    </form>
  );
}

function BotonBorrar({ etiqueta, onConfirm, onDone }: { etiqueta: string; onConfirm: () => Promise<unknown>; onDone: () => void }) {
  const [error, setError] = useState<string | null>(null);
  return (
    <span>
      <button
        type="button"
        className="btn ghost small"
        aria-label={`Eliminar ${etiqueta}`}
        onClick={async () => {
          if (!window.confirm(`¿Eliminar ${etiqueta}? Esta acción no se puede deshacer.`)) return;
          try {
            await onConfirm();
            onDone();
          } catch (e) {
            setError((e as Error).message);
          }
        }}
      >
        <Trash2 size={14} aria-hidden="true" />
      </button>
      {error && (
        <small role="alert" className="error">
          {error}
        </small>
      )}
    </span>
  );
}

function EstadoArchivo({ r }: { r: Resource }) {
  if (!CON_ARCHIVO.includes(r.type)) return null;
  if (sinArchivo(r)) return <Badge tone="info">Sin archivo</Badge>;
  if (disponible(r)) return <Badge tone="ok"><CheckCircle2 size={12} aria-hidden="true" /> Disponible</Badge>;
  if (r.processing_status === "failed") return <Badge tone="bad"><TriangleAlert size={12} aria-hidden="true" /> Falló el procesamiento</Badge>;
  if (r.processing_status === "pending" || r.processing_status === "processing") {
    return <Badge tone="warn"><Loader2 size={12} className="spin" aria-hidden="true" /> {r.processing_status === "pending" ? "En cola" : "Procesando"}</Badge>;
  }
  return <Badge tone="info">Sin archivo</Badge>;
}

function FilaRecurso({ recurso: r, editable, onChanged }: { recurso: Resource; editable: boolean; onChanged: () => void }) {
  const [abierto, setAbierto] = useState(false);
  const tieneArchivo = CON_ARCHIVO.includes(r.type);
  const esQuiz = r.type === "quiz";

  return (
    <li className="recurso">
      <div className="row-head">
        <span>
          <strong>{r.title}</strong> <small className="muted">· {ETIQUETA_TIPO[r.type] ?? r.type}{r.is_mandatory ? " · obligatorio" : ""}</small>
        </span>
        <span className="row-actions">
          <EstadoArchivo r={r} />
          {(tieneArchivo || esQuiz) && (
            <button type="button" className="btn ghost small" aria-expanded={abierto} onClick={() => setAbierto(!abierto)}>
              {abierto ? <ChevronDown size={14} aria-hidden="true" /> : <ChevronRight size={14} aria-hidden="true" />} {esQuiz ? "Cuestionario" : "Archivo"}
            </button>
          )}
          {editable && <BotonBorrar etiqueta={`el recurso «${r.title}»`} onConfirm={() => deleteResource(r.id)} onDone={onChanged} />}
        </span>
      </div>
      {abierto && tieneArchivo && (editable ? <UploadControl resource={r} onQueued={onChanged} /> : <p className="muted">El curso está publicado: no se pueden cambiar sus archivos.</p>)}
      {abierto && esQuiz && <QuizEditor resource={r} editable={editable} />}
    </li>
  );
}

function NuevoRecurso({ unitId, onCreated }: { unitId: string; onCreated: () => void }) {
  const [abierto, setAbierto] = useState(false);
  const [titulo, setTitulo] = useState("");
  const [tipo, setTipo] = useState<ResourceType>("text");
  const [obligatorio, setObligatorio] = useState(true);
  const [descargable, setDescargable] = useState(false);
  const [texto, setTexto] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [enviando, setEnviando] = useState(false);
  const clave = useRef(newIdempotencyKey());
  const enCurso = useRef(false);

  async function crear(e: FormEvent) {
    e.preventDefault();
    if (enCurso.current) return;
    enCurso.current = true;
    setEnviando(true);
    setError(null);
    const body: NewResource = {
      title: titulo.trim(),
      type: tipo,
      is_mandatory: obligatorio,
      allow_download: descargable,
      content_text: tipo === "text" || tipo === "external_link" ? texto : undefined,
    };
    try {
      await createResource(unitId, body, clave.current);
      setTitulo("");
      setTexto("");
      clave.current = newIdempotencyKey();
      setAbierto(false);
      onCreated();
    } catch (err) {
      setError(err instanceof ApiError && err.status === 400 ? "Revisa los datos del recurso." : (err as Error).message);
      clave.current = newIdempotencyKey();
    } finally {
      enCurso.current = false;
      setEnviando(false);
    }
  }

  if (!abierto) {
    return (
      <button type="button" className="btn ghost small" onClick={() => setAbierto(true)}>
        <Plus size={14} aria-hidden="true" /> Agregar recurso
      </button>
    );
  }
  return (
    <form className="form inline-form" onSubmit={crear}>
      <label>
        Título del recurso
        <input value={titulo} onChange={(e) => setTitulo(e.target.value)} required />
      </label>
      <label>
        Tipo
        <select value={tipo} onChange={(e) => setTipo(e.target.value as ResourceType)}>
          {TIPOS.map((t) => (
            <option key={t.valor} value={t.valor}>
              {t.etiqueta}
            </option>
          ))}
        </select>
      </label>
      {(tipo === "text" || tipo === "external_link") && (
        <label>
          {tipo === "text" ? "Contenido (admite # títulos, - listas y **negrita**)" : "Enlace (https://…)"}
          <textarea rows={tipo === "text" ? 5 : 1} value={texto} onChange={(e) => setTexto(e.target.value)} required />
        </label>
      )}
      <label className="check">
        <input type="checkbox" checked={obligatorio} onChange={(e) => setObligatorio(e.target.checked)} /> Obligatorio para aprobar el curso
      </label>
      {CON_ARCHIVO.includes(tipo) && (
        <label className="check">
          <input type="checkbox" checked={descargable} onChange={(e) => setDescargable(e.target.checked)} /> El estudiante puede descargarlo
        </label>
      )}
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <div className="actions">
        <button type="submit" className="btn small" disabled={enviando || !titulo.trim()}>
          {enviando ? "Creando…" : "Crear recurso"}
        </button>
        <button type="button" className="btn ghost small" onClick={() => setAbierto(false)}>
          Cancelar
        </button>
      </div>
      {CON_ARCHIVO.includes(tipo) && <small className="muted">Después de crearlo, abre «Archivo» en su fila para subir el archivo.</small>}
    </form>
  );
}

// ---- Publicacion ------------------------------------------------------------------------

function Publicacion({ course, onChanged }: { course: Course; onChanged: () => void }) {
  const [enviando, setEnviando] = useState(false);
  const [resultado, setResultado] = useState<{ ok: true; texto: string } | { ok: false; titulo: string; problemas: FieldProblem[] } | null>(null);

  async function publicar() {
    setEnviando(true);
    setResultado(null);
    try {
      await publishCourse(course.id);
      setResultado({ ok: true, texto: "Curso publicado. Ya aparece en el catálogo de los estudiantes." });
      onChanged();
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) {
        setResultado({ ok: false, titulo: "El curso todavía no cumple los requisitos de publicación:", problemas: err.details ?? [{ field: "", message: err.message }] });
      } else if (err instanceof ApiError && err.status === 409) {
        setResultado({ ok: false, titulo: "No se puede publicar en el estado actual del curso.", problemas: [] });
        onChanged();
      } else {
        setResultado({ ok: false, titulo: (err as Error).message, problemas: [] });
      }
    } finally {
      setEnviando(false);
    }
  }

  async function despublicar() {
    setEnviando(true);
    setResultado(null);
    try {
      await unpublishCourse(course.id);
      setResultado({ ok: true, texto: "Curso despublicado: salió del catálogo y vuelve a ser editable." });
      onChanged();
    } catch (err) {
      setResultado({ ok: false, titulo: (err as Error).message, problemas: [] });
    } finally {
      setEnviando(false);
    }
  }

  return (
    <section className="panel pad">
      <h2>Publicación</h2>
      <p className="muted">
        Para publicar, el curso necesita título y descripción, y al menos un módulo con una unidad con un recurso visible y disponible (un archivo en
        procesamiento no cuenta), con algún recurso obligatorio.
      </p>
      <div className="actions">
        {course.status === "published" ? (
          <button type="button" className="btn ghost" disabled={enviando} onClick={despublicar}>
            {enviando ? "Despublicando…" : "Despublicar para editar"}
          </button>
        ) : (
          <button type="button" className="btn" disabled={enviando} onClick={publicar}>
            {enviando ? "Publicando…" : "Publicar curso"}
          </button>
        )}
      </div>
      {resultado?.ok && (
        <p role="status" className="ok-text">
          {resultado.texto}
        </p>
      )}
      {resultado && !resultado.ok && (
        <div role="alert" className="error">
          <p>{resultado.titulo}</p>
          {resultado.problemas.length > 0 && (
            <ul>
              {resultado.problemas.map((p, i) => (
                <li key={i}>{p.message}</li>
              ))}
            </ul>
          )}
        </div>
      )}
    </section>
  );
}
