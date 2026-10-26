import { Award, BookOpen, GraduationCap, Home, LibraryBig, LogOut, PlusCircle, Search } from "lucide-react";
import { useState, type FormEvent, type ReactNode } from "react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useAuth, type Role } from "../auth/AuthContext";
import { APP_VERSION } from "../config";
import { useStudentData } from "../student/StudentData";
import { NoticeBanner } from "./Chrome";
import { Avatar, ProgressBar } from "./ui";

interface NavItem {
  to: string;
  label: string;
  icon: ReactNode;
  end?: boolean;
}

const NAV: Record<Role, NavItem[]> = {
  estudiante: [
    { to: "/", label: "Inicio", icon: <Home size={18} />, end: true },
    { to: "/catalogo", label: "Catálogo", icon: <LibraryBig size={18} /> },
    { to: "/mis-cursos", label: "Mis cursos", icon: <GraduationCap size={18} /> },
  ],
  profesor: [
    { to: "/profesor", label: "Inicio", icon: <Home size={18} />, end: true },
    { to: "/profesor/cursos", label: "Mis cursos", icon: <BookOpen size={18} /> },
    { to: "/profesor/cursos/nuevo", label: "Crear curso", icon: <PlusCircle size={18} /> },
  ],
  administrador: [{ to: "/", label: "Inicio", icon: <Home size={18} />, end: true }],
};

const ETIQUETA_ROL: Record<Role, string> = { estudiante: "Estudiante", profesor: "Profesor", administrador: "Administrador" };

export function Shell() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const [busqueda, setBusqueda] = useState("");
  const [saliendo, setSaliendo] = useState(false);
  if (!user) return null;

  const hoy = new Date().toLocaleDateString("es-CO", { weekday: "long", day: "numeric", month: "long", year: "numeric" });

  function buscar(e: FormEvent) {
    e.preventDefault();
    navigate(`/catalogo${busqueda.trim() ? `?q=${encodeURIComponent(busqueda.trim())}` : ""}`);
  }

  return (
    <div className="app">
      <aside className="panel sidebar" aria-label="Navegación principal">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true">
            <BookOpen size={20} />
          </span>
          <span>Plataforma MOOC</span>
        </div>
        <nav>
          {NAV[user.role].map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end} className={({ isActive }) => `nav-link${isActive ? " active" : ""}`}>
              {item.icon}
              <span>{item.label}</span>
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-foot">
          <button
            type="button"
            className="btn ghost block"
            disabled={saliendo}
            onClick={async () => {
              setSaliendo(true);
              await logout();
            }}
          >
            <LogOut size={16} aria-hidden="true" /> Cerrar sesión
          </button>
          <NavLink to="/estado" className="muted"><small>Estado del sistema</small></NavLink>
          <small className="muted" data-testid="app-version">
            Versión <code>{APP_VERSION.commit}</code>
          </small>
        </div>
      </aside>

      <div className="main">
        <header className="topbar-row">
          {user.role === "estudiante" ? (
            <form className="search" role="search" onSubmit={buscar}>
              <Search size={18} aria-hidden="true" />
              <input
                type="search"
                placeholder="Buscar cursos en el catálogo"
                aria-label="Buscar cursos en el catálogo"
                value={busqueda}
                onChange={(e) => setBusqueda(e.target.value)}
              />
            </form>
          ) : (
            <div />
          )}
          <span className="today">{hoy}</span>
        </header>
        <NoticeBanner />
        <Outlet />
      </div>

      <aside className="panel rightpanel" aria-label="Perfil">
        <div className="profile">
          <Avatar name={user.full_name} size={84} />
          <strong>{user.full_name}</strong>
          <span className="muted">{ETIQUETA_ROL[user.role]}</span>
          <small className="muted">{user.email}</small>
        </div>
        {user.role === "estudiante" && <StudentSummary />}
      </aside>
    </div>
  );
}

function StudentSummary() {
  const { state } = useStudentData();
  if (state.status === "loading") return <p className="muted center">Cargando tu progreso…</p>;
  if (state.status === "error") return <p className="muted center">No se pudo cargar tu progreso.</p>;

  const { mine } = state;
  const completados = mine.filter((m) => m.progress?.is_approved).length;
  const insignias = mine.filter((m) => m.progress?.badge && !m.progress.badge.revoked).length;
  const siguientes = mine.filter((m) => m.course && (m.progress?.percentage ?? 0) < 100).slice(0, 3);

  return (
    <>
      <div className="stats">
        <div>
          <strong>{mine.length}</strong>
          <span className="muted">Inscritos</span>
        </div>
        <div>
          <strong>{completados}</strong>
          <span className="muted">Aprobados</span>
        </div>
        <div>
          <strong>{insignias}</strong>
          <span className="muted">
            <Award size={12} aria-hidden="true" /> Insignias
          </span>
        </div>
      </div>
      <h2 className="side-title">Para continuar</h2>
      {siguientes.length === 0 ? (
        <p className="muted">No tienes cursos pendientes. Explora el catálogo.</p>
      ) : (
        <ul className="next-list">
          {siguientes.map((m) => (
            <li key={m.enrollment.id}>
              <NavLink to={`/cursos/${m.course!.id}`}>{m.course!.title}</NavLink>
              <ProgressBar value={m.progress?.percentage ?? 0} label={`Progreso de ${m.course!.title}`} />
              <small className="muted">{Math.round(m.progress?.percentage ?? 0)} % completado</small>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
