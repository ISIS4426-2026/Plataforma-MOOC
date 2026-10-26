import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { ApiError, request, setApiHooks } from "../api/client";

export type Role = "estudiante" | "profesor" | "administrador";

export interface SessionUser {
  id: string;
  email: string;
  full_name: string;
  role: Role;
  status: string;
}

interface LoginResponse {
  // El cuerpo tambien trae "token" y "expires_at": se ignoran a proposito (ver api/client.ts).
  user: SessionUser;
}

/** Aviso global (sesion vencida, sin permiso, servidor caido) que cualquier vista puede mostrar. */
export interface Notice {
  kind: "info" | "warning" | "error";
  text: string;
}

interface AuthState {
  user: SessionUser | null;
  notice: Notice | null;
  clearNotice: () => void;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const STORAGE_KEY = "mooc.user";

// La API no tiene un GET /me: solo el login entrega el usuario. Para no perderlo al recargar
// se guarda ESTA copia (sin token: no es secreto) en sessionStorage, que muere con la pestana.
// Es solo una ayuda de interfaz: la autoridad es siempre la cookie y el servidor, que revalida
// rol y sesion en cada peticion. Cuando exista un endpoint de sesion actual, se reemplaza.
function leerUsuarioGuardado(): SessionUser | null {
  try {
    const crudo = sessionStorage.getItem(STORAGE_KEY);
    return crudo ? (JSON.parse(crudo) as SessionUser) : null;
  } catch {
    return null;
  }
}

function guardarUsuario(user: SessionUser | null): void {
  try {
    if (user) sessionStorage.setItem(STORAGE_KEY, JSON.stringify(user));
    else sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // Sin almacenamiento disponible (modo privado estricto): la sesion sigue, solo no sobrevive a una recarga.
  }
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<SessionUser | null>(leerUsuarioGuardado);
  const [notice, setNotice] = useState<Notice | null>(null);

  const cerrarLocal = useCallback(() => {
    guardarUsuario(null);
    setUser(null);
  }, []);

  useEffect(() => {
    setApiHooks({
      onUnauthorized: () => {
        cerrarLocal();
        setNotice({ kind: "warning", text: "Tu sesión venció. Inicia sesión de nuevo." });
      },
      onForbidden: (e) =>
        setNotice({
          kind: "error",
          text:
            e.code === "csrf_origin_rejected"
              ? "El servidor rechazó el origen de esta página (configuración de seguridad)."
              : "No tienes permiso para hacer esto.",
        }),
      onServerError: (e) =>
        setNotice({
          kind: "error",
          text: e.status === 0 ? "No hay conexión con el servidor." : "El servidor tuvo un problema. Intenta de nuevo.",
        }),
    });
  }, [cerrarLocal]);

  const login = useCallback(async (email: string, password: string) => {
    const { data } = await request<LoginResponse>("/auth/login", {
      method: "POST",
      body: { email, password },
      skipUnauthorizedHook: true,
    });
    guardarUsuario(data.user);
    setUser(data.user);
    setNotice(null);
  }, []);

  const logout = useCallback(async () => {
    try {
      await request("/auth/logout", { method: "POST" });
    } catch (e) {
      // Si la sesion ya no existe en el servidor (401) o no hay red, igual se limpia lo local.
      if (!(e instanceof ApiError)) throw e;
    } finally {
      cerrarLocal();
    }
  }, [cerrarLocal]);

  const value = useMemo<AuthState>(
    () => ({ user, notice, clearNotice: () => setNotice(null), login, logout }),
    [user, notice, login, logout],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth debe usarse dentro de <AuthProvider>");
  return ctx;
}
