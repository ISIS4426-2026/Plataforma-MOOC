import { useRef, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { ApiError, newIdempotencyKey } from "../api/client";
import { PageTitle } from "../components/ui";
import { createCourse } from "./api";
import { useProfessorData } from "./ProfessorData";

export default function NewCourse() {
  const navigate = useNavigate();
  const { refresh } = useProfessorData();
  const [titulo, setTitulo] = useState("");
  const [descripcion, setDescripcion] = useState("");
  const [enviando, setEnviando] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // La clave se fija por formulario: un doble clic o un reintento no crea dos cursos.
  const clave = useRef(newIdempotencyKey());
  const enCurso = useRef(false);

  async function crear(e: FormEvent) {
    e.preventDefault();
    if (enCurso.current) return;
    enCurso.current = true;
    setEnviando(true);
    setError(null);
    try {
      const curso = await createCourse(titulo.trim(), descripcion.trim(), clave.current);
      refresh();
      navigate(`/profesor/cursos/${curso.id}`);
    } catch (err) {
      setError(err instanceof ApiError && err.status === 400 ? "Revisa el título y la descripción: no pueden estar vacíos." : (err as Error).message);
      clave.current = newIdempotencyKey(); // un fallo real permite volver a intentar con otra clave
    } finally {
      enCurso.current = false;
      setEnviando(false);
    }
  }

  return (
    <>
      <PageTitle title="Crear curso" subtitle="Empiezas con un borrador: después agregas módulos, unidades y recursos, y lo publicas." />
      <form className="panel pad form" onSubmit={crear}>
        <label htmlFor="titulo">Título</label>
        <input id="titulo" value={titulo} maxLength={160} onChange={(e) => setTitulo(e.target.value)} required />
        <label htmlFor="descripcion">Descripción</label>
        <textarea id="descripcion" rows={4} value={descripcion} maxLength={2000} onChange={(e) => setDescripcion(e.target.value)} required />
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        <div className="actions">
          <button type="submit" className="btn" disabled={enviando || !titulo.trim() || !descripcion.trim()}>
            {enviando ? "Creando…" : "Crear borrador"}
          </button>
          <button type="button" className="btn ghost" onClick={() => navigate("/profesor/cursos")}>
            Cancelar
          </button>
        </div>
      </form>
    </>
  );
}
