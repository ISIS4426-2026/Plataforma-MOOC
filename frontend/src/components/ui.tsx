import { AlertTriangle, Inbox, Loader2 } from "lucide-react";
import type { ReactNode } from "react";
import { ApiError } from "../api/client";

export function Spinner({ label = "Cargando…" }: { label?: string }) {
  return (
    <div className="state" role="status" aria-live="polite">
      <Loader2 className="spin" size={22} aria-hidden="true" />
      <span>{label}</span>
    </div>
  );
}

export function EmptyState({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="state empty">
      <Inbox size={26} aria-hidden="true" />
      <strong>{title}</strong>
      {children && <span className="muted">{children}</span>}
    </div>
  );
}

export function ErrorState({ error, onRetry }: { error: Error; onRetry?: () => void }) {
  const detalle = error instanceof ApiError && error.requestId ? ` (ID de petición ${error.requestId})` : "";
  return (
    <div className="state error-state" role="alert">
      <AlertTriangle size={24} aria-hidden="true" />
      <strong>No se pudo cargar</strong>
      <span className="muted">
        {error.message}
        {detalle}
      </span>
      {onRetry && (
        <button type="button" className="btn ghost" onClick={onRetry}>
          Reintentar
        </button>
      )}
    </div>
  );
}

export function ProgressBar({ value, label }: { value: number; label?: string }) {
  const pct = Math.max(0, Math.min(100, Math.round(value)));
  return (
    <div className="bar" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100} aria-label={label ?? "Progreso"}>
      <div className="bar-fill" style={{ width: `${pct}%` }} />
    </div>
  );
}

export function Avatar({ name, size = 44 }: { name: string; size?: number }) {
  const iniciales = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]!.toUpperCase())
    .join("");
  return (
    <div className="avatar" style={{ width: size, height: size, fontSize: size * 0.38 }} aria-hidden="true">
      {iniciales || "?"}
    </div>
  );
}

export function Badge({ tone, children }: { tone: "ok" | "warn" | "bad" | "info"; children: ReactNode }) {
  return <span className={`badge ${tone}`}>{children}</span>;
}

export function PageTitle({ title, subtitle, action }: { title: string; subtitle?: string; action?: ReactNode }) {
  return (
    <div className="page-title">
      <div>
        <h1>{title}</h1>
        {subtitle && <p className="muted">{subtitle}</p>}
      </div>
      {action}
    </div>
  );
}

/** Tarjeta de color de «mis cursos», con la paleta de la interfaz. */
export const GRADIENTES = ["g-azul", "g-violeta", "g-rosa", "g-verde"] as const;
export const gradiente = (i: number): string => GRADIENTES[i % GRADIENTES.length]!;
