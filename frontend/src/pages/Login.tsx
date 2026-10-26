import { useState, type FormEvent } from "react";
import { Navigate } from "react-router-dom";
import { ApiError } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { Footer, NoticeBanner } from "../components/Chrome";

export default function Login() {
  const { user, login } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [enviando, setEnviando] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (user) return <Navigate to="/" replace />;

  async function enviar(e: FormEvent) {
    e.preventDefault();
    setEnviando(true);
    setError(null);
    try {
      await login(email.trim(), password);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) setError("Correo o contraseña incorrectos.");
      else if (err instanceof ApiError && err.status === 429) setError("Demasiados intentos. Espera un minuto e inténtalo de nuevo.");
      else if (err instanceof ApiError) setError(err.message);
      else setError("Ocurrió un error inesperado.");
    } finally {
      setEnviando(false);
    }
  }

  return (
    <div className="page">
      <main className="card narrow">
        <h1>Plataforma MOOC</h1>
        <p className="muted">Inicia sesión para continuar.</p>
        <NoticeBanner />
        <form onSubmit={enviar} noValidate>
          <label htmlFor="email">Correo</label>
          <input id="email" type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} required />
          <label htmlFor="password">Contraseña</label>
          <input id="password" type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
          <button type="submit" disabled={enviando || !email || !password}>
            {enviando ? "Entrando…" : "Entrar"}
          </button>
        </form>
      </main>
      <Footer />
    </div>
  );
}
