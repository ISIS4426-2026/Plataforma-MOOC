import { useEffect, useRef, useState } from "react";
import { ApiError, newIdempotencyKey } from "../api/client";
import { getQuiz, mySubmissions, openAttempt, saveAttempt, submitQuiz } from "../api/endpoints";
import type { QuizAttempt, QuizSubmission } from "../api/types";
import { Badge, ErrorState, Spinner } from "../components/ui";
import { useAsync } from "../hooks/useAsync";

/**
 * Quiz del estudiante. La clave de respuestas nunca llega al navegador (la API no la envia). El
 * envio lleva una Idempotency-Key fija por intento: un doble clic o un reintento devuelve la misma
 * calificacion y no consume otro intento.
 */
export function QuizView({ resourceId, onGraded }: { resourceId: string; onGraded: () => void }) {
  const quiz = useAsync(() => getQuiz(resourceId), [resourceId]);
  const [version, setVersion] = useState(0);
  const previos = useAsync(
    () => (quiz.status === "ok" ? mySubmissions(quiz.data.id) : Promise.resolve([] as QuizSubmission[])),
    [quiz.status === "ok" ? quiz.data.id : "", version],
  );

  const [intento, setIntento] = useState<QuizAttempt | null>(null);
  const [respuestas, setRespuestas] = useState<Record<string, string>>({});
  const [resultado, setResultado] = useState<QuizSubmission | null>(null);
  const [abriendo, setAbriendo] = useState(false);
  const [enviando, setEnviando] = useState(false);
  const [guardado, setGuardado] = useState<"" | "guardando" | "guardado">("");
  const [error, setError] = useState<string | null>(null);
  const clave = useRef<string>("");
  // Cerrojo sincrono: un doble clic en el mismo instante no debe lanzar dos envios (el estado de React
  // aun no se ha actualizado cuando entra el segundo clic).
  const enCurso = useRef(false);
  const temporizador = useRef<number | undefined>(undefined);

  // Al cambiar de quiz se descarta cualquier intento o resultado del anterior.
  useEffect(() => {
    setIntento(null);
    setResultado(null);
    setRespuestas({});
    setError(null);
  }, [resourceId]);

  if (quiz.status === "loading") return <Spinner label="Cargando el cuestionario…" />;
  if (quiz.status === "error") {
    const e = quiz.error;
    return e instanceof ApiError && e.status === 403 ? (
      <p role="alert" className="error">
        Este cuestionario no está disponible para ti. Comprueba que estás inscrito en el curso.
      </p>
    ) : (
      <ErrorState error={e} onRetry={quiz.reload} />
    );
  }

  const q = quiz.data;
  const usados = previos.status === "ok" ? previos.data.length : 0;
  const quedan = Math.max(0, q.max_attempts - usados);

  async function comenzar() {
    setAbriendo(true);
    setError(null);
    try {
      const a = await openAttempt(q.id);
      setIntento(a);
      setRespuestas(Object.fromEntries(a.answers.map((x) => [x.question_id, x.selected_option_id])));
      clave.current = `quiz-${q.id}-intento-${a.attempt}-${newIdempotencyKey()}`;
    } catch (e) {
      setError(e instanceof ApiError && e.status === 409 ? "No te quedan intentos para este cuestionario." : ((e as Error).message ?? "No se pudo iniciar el intento."));
    } finally {
      setAbriendo(false);
    }
  }

  function elegir(preguntaId: string, opcionId: string) {
    const siguiente = { ...respuestas, [preguntaId]: opcionId };
    setRespuestas(siguiente);
    setGuardado("guardando");
    window.clearTimeout(temporizador.current);
    // Guardado parcial: no califica ni consume intento; permite retomar si se cierra el navegador.
    temporizador.current = window.setTimeout(() => {
      saveAttempt(q.id, Object.entries(siguiente).map(([question_id, selected_option_id]) => ({ question_id, selected_option_id })))
        .then(() => setGuardado("guardado"))
        .catch(() => setGuardado(""));
    }, 600);
  }

  async function enviar() {
    if (!intento || enCurso.current) return;
    enCurso.current = true;
    setEnviando(true);
    setError(null);
    try {
      const lista = intento.quiz.questions.map((p) => ({ question_id: p.id, selected_option_id: respuestas[p.id]! }));
      const res = await submitQuiz(q.id, lista, clave.current);
      setResultado(res);
      setError(null);
      setIntento(null);
      setVersion((v) => v + 1);
      onGraded();
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) setError("El intento venció o ya no quedan intentos disponibles.");
      else setError((e as Error).message);
    } finally {
      enCurso.current = false;
      setEnviando(false);
    }
  }

  if (resultado) {
    return (
      <div className="quiz-result" role="status">
        <h3>Resultado del intento {resultado.attempt}</h3>
        <p className="score">{resultado.score} / 100</p>
        <Badge tone={resultado.passed ? "ok" : "bad"}>{resultado.passed ? "Aprobado" : `No aprobado (mínimo ${q.passing_score})`}</Badge>
        {resultado.attempts_remaining !== undefined && <p className="muted">Intentos restantes: {resultado.attempts_remaining}</p>}
        <button type="button" className="btn ghost" onClick={() => setResultado(null)}>
          Volver al cuestionario
        </button>
      </div>
    );
  }

  if (intento) {
    const completo = intento.quiz.questions.every((p) => respuestas[p.id]);
    return (
      <form
        className="quiz"
        onSubmit={(e) => {
          e.preventDefault();
          void enviar();
        }}
      >
        <p className="muted">
          Intento {intento.attempt} de {q.max_attempts}. Tus respuestas se guardan solas.{" "}
          <span aria-live="polite">{guardado === "guardando" ? "Guardando…" : guardado === "guardado" ? "Guardado" : ""}</span>
        </p>
        {intento.expired && (
          <p role="alert" className="error">
            El tiempo de este intento terminó.
          </p>
        )}
        {intento.quiz.questions
          .slice()
          .sort((a, b) => a.position - b.position)
          .map((p, i) => (
            <fieldset key={p.id}>
              <legend>
                {i + 1}. {p.prompt}
              </legend>
              {p.options
                .slice()
                .sort((a, b) => a.position - b.position)
                .map((o) => (
                  <label key={o.id} className="option">
                    <input type="radio" name={p.id} value={o.id} checked={respuestas[p.id] === o.id} onChange={() => elegir(p.id, o.id)} />
                    <span>{o.text}</span>
                  </label>
                ))}
            </fieldset>
          ))}
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        <button type="submit" className="btn" disabled={!completo || enviando || intento.expired}>
          {enviando ? "Enviando…" : "Enviar respuestas"}
        </button>
        {!completo && <small className="muted"> Responde todas las preguntas para enviar.</small>}
      </form>
    );
  }

  return (
    <div className="quiz-intro">
      <h3>{q.title}</h3>
      <p className="muted">
        {q.questions.length} preguntas · nota mínima para aprobar: {q.passing_score} · intentos: {usados} de {q.max_attempts} usados
      </p>
      {previos.status === "ok" && previos.data.length > 0 && (
        <table className="mini">
          <caption>Tus intentos anteriores</caption>
          <thead>
            <tr>
              <th scope="col">Intento</th>
              <th scope="col">Nota</th>
              <th scope="col">Resultado</th>
            </tr>
          </thead>
          <tbody>
            {previos.data
              .slice()
              .sort((a, b) => a.attempt - b.attempt)
              .map((s) => (
                <tr key={s.submission_id}>
                  <td>{s.attempt}</td>
                  <td>{s.score}</td>
                  <td>{s.passed ? "Aprobado" : "No aprobado"}</td>
                </tr>
              ))}
          </tbody>
        </table>
      )}
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {quedan === 0 ? (
        <p role="status">No te quedan intentos en este cuestionario.</p>
      ) : (
        <button type="button" className="btn" disabled={abriendo} onClick={comenzar}>
          {abriendo ? "Abriendo…" : usados === 0 ? "Comenzar cuestionario" : "Intentar de nuevo"}
        </button>
      )}
    </div>
  );
}
