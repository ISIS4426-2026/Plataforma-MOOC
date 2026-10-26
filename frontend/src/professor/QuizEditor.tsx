import { useRef, useState, type FormEvent } from "react";
import { ApiError, newIdempotencyKey } from "../api/client";
import type { Resource } from "../api/types";
import { ErrorState, Spinner } from "../components/ui";
import { useAsync } from "../hooks/useAsync";
import { addQuestion, createQuiz, getAuthorQuiz } from "./api";

/**
 * Cuestionario basico del profesor. El autor ve la clave de respuestas (es suya); el estudiante nunca
 * la recibe. Crear el cuestionario y agregarle preguntas son dos pasos de la API.
 */
export function QuizEditor({ resource, editable }: { resource: Resource; editable: boolean }) {
  const quiz = useAsync(async () => {
    try {
      return await getAuthorQuiz(resource.id);
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) return null; // todavia no tiene cuestionario
      throw e;
    }
  }, [resource.id]);

  if (quiz.status === "loading") return <Spinner label="Cargando el cuestionario…" />;
  if (quiz.status === "error") return <ErrorState error={quiz.error} onRetry={quiz.reload} />;

  if (quiz.data === null) {
    return editable ? <CrearQuiz resourceId={resource.id} onCreated={quiz.reload} /> : <p className="muted">Este recurso no tiene cuestionario.</p>;
  }

  const q = quiz.data;
  const preguntas = (q.questions_with_key ?? []).slice().sort((a, b) => a.position - b.position);
  return (
    <div className="quiz-editor">
      <p className="muted">
        {q.title} · nota mínima {q.passing_score} · {q.max_attempts} intentos · {preguntas.length} pregunta(s)
      </p>
      {preguntas.length === 0 && <p className="muted">Aún no tiene preguntas. Un cuestionario sin preguntas no sirve para evaluar.</p>}
      <ol>
        {preguntas.map((p) => (
          <li key={p.id}>
            <strong>{p.prompt}</strong>
            <ul>
              {p.options
                .slice()
                .sort((a, b) => a.position - b.position)
                .map((o) => (
                  <li key={o.id} className={o.is_correct ? "correct" : ""}>
                    {o.text} {o.is_correct && <span className="badge ok">correcta</span>}
                  </li>
                ))}
            </ul>
          </li>
        ))}
      </ol>
      {editable && <NuevaPregunta quizId={q.id} onAdded={quiz.reload} />}
    </div>
  );
}

function CrearQuiz({ resourceId, onCreated }: { resourceId: string; onCreated: () => void }) {
  const [titulo, setTitulo] = useState("");
  const [nota, setNota] = useState(70);
  const [intentos, setIntentos] = useState(3);
  const [error, setError] = useState<string | null>(null);
  const [enviando, setEnviando] = useState(false);
  const clave = useRef(newIdempotencyKey());

  async function crear(e: FormEvent) {
    e.preventDefault();
    setEnviando(true);
    setError(null);
    try {
      await createQuiz(resourceId, { title: titulo.trim(), passing_score: nota, max_attempts: intentos }, clave.current);
      onCreated();
    } catch (err) {
      setError((err as Error).message);
      clave.current = newIdempotencyKey();
    } finally {
      setEnviando(false);
    }
  }

  return (
    <form className="form inline-form" onSubmit={crear}>
      <label>
        Título del cuestionario
        <input value={titulo} onChange={(e) => setTitulo(e.target.value)} required />
      </label>
      <label>
        Nota mínima para aprobar (0–100)
        <input type="number" min={0} max={100} value={nota} onChange={(e) => setNota(Number(e.target.value))} required />
      </label>
      <label>
        Intentos permitidos
        <input type="number" min={1} max={10} value={intentos} onChange={(e) => setIntentos(Number(e.target.value))} required />
      </label>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <button type="submit" className="btn small" disabled={enviando || !titulo.trim()}>
        {enviando ? "Creando…" : "Crear cuestionario"}
      </button>
    </form>
  );
}

function NuevaPregunta({ quizId, onAdded }: { quizId: string; onAdded: () => void }) {
  const [enunciado, setEnunciado] = useState("");
  const [opciones, setOpciones] = useState(["", ""]);
  const [correcta, setCorrecta] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [enviando, setEnviando] = useState(false);
  const clave = useRef(newIdempotencyKey());

  const llenas = opciones.map((o) => o.trim()).filter(Boolean).length;
  const valido = enunciado.trim() !== "" && llenas >= 2 && correcta !== null && opciones[correcta]?.trim() !== "" && llenas === opciones.length;

  async function agregar(e: FormEvent) {
    e.preventDefault();
    setEnviando(true);
    setError(null);
    try {
      await addQuestion(quizId, { prompt: enunciado.trim(), options: opciones.map((t, i) => ({ text: t.trim(), is_correct: i === correcta })) }, clave.current);
      setEnunciado("");
      setOpciones(["", ""]);
      setCorrecta(null);
      clave.current = newIdempotencyKey();
      onAdded();
    } catch (err) {
      setError(err instanceof ApiError && err.status === 409 ? "El curso está publicado: despublícalo para cambiar el cuestionario." : (err as Error).message);
    } finally {
      setEnviando(false);
    }
  }

  return (
    <form className="form inline-form" onSubmit={agregar}>
      <h4>Agregar pregunta</h4>
      <label>
        Enunciado
        <input value={enunciado} onChange={(e) => setEnunciado(e.target.value)} />
      </label>
      <fieldset>
        <legend>Opciones (marca la correcta)</legend>
        {opciones.map((o, i) => (
          <div key={i} className="option-row">
            <input type="radio" name="correcta" checked={correcta === i} onChange={() => setCorrecta(i)} aria-label={`La opción ${i + 1} es la correcta`} />
            <input value={o} onChange={(e) => setOpciones(opciones.map((x, j) => (j === i ? e.target.value : x)))} aria-label={`Texto de la opción ${i + 1}`} />
          </div>
        ))}
        {opciones.length < 5 && (
          <button type="button" className="btn ghost small" onClick={() => setOpciones([...opciones, ""])}>
            Agregar otra opción
          </button>
        )}
      </fieldset>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <button type="submit" className="btn small" disabled={enviando || !valido}>
        {enviando ? "Guardando…" : "Agregar pregunta"}
      </button>
      {!valido && <small className="muted"> Escribe el enunciado, al menos dos opciones con texto y marca exactamente una correcta.</small>}
    </form>
  );
}
