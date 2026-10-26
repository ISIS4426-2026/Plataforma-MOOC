import "@fontsource-variable/mulish";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider, useAuth } from "./auth/AuthContext";
import { Shell } from "./components/Shell";
import Estado from "./pages/Estado";
import Login from "./pages/Login";
import { loadRuntimeConfig } from "./runtime";
import Catalog from "./student/Catalog";
import Course from "./student/Course";
import Dashboard from "./student/Dashboard";
import MyCourses from "./student/MyCourses";
import { StudentDataProvider } from "./student/StudentData";
import "./styles.css";

/** Solo con sesion; el estudiante ademas carga sus datos compartidos una sola vez. */
function RequireAuth() {
  const { user } = useAuth();
  if (!user) return <Navigate to="/login" replace />;
  return user.role === "estudiante" ? (
    <StudentDataProvider>
      <Shell />
    </StudentDataProvider>
  ) : (
    <Shell />
  );
}

/** Cada rol ve solo sus rutas; las demas lo devuelven a su inicio (la autorizacion real es del servidor). */
function SoloRol({ rol, children }: { rol: "estudiante" | "profesor"; children: React.ReactElement }) {
  const { user } = useAuth();
  if (user?.role === rol) return children;
  return <Navigate to={user?.role === "profesor" ? "/profesor" : "/"} replace />;
}

function Inicio() {
  const { user } = useAuth();
  return user?.role === "profesor" ? <Navigate to="/profesor" replace /> : <Dashboard />;
}

// La configuracion de entorno (/config.json) se lee antes de pintar nada.
void loadRuntimeConfig().then(() =>
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <BrowserRouter>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route element={<RequireAuth />}>
              <Route index element={<Inicio />} />
              <Route path="estado" element={<Estado />} />
              <Route
                path="catalogo"
                element={
                  <SoloRol rol="estudiante">
                    <Catalog />
                  </SoloRol>
                }
              />
              <Route
                path="mis-cursos"
                element={
                  <SoloRol rol="estudiante">
                    <MyCourses />
                  </SoloRol>
                }
              />
              <Route
                path="cursos/:courseId/:resourceId?"
                element={
                  <SoloRol rol="estudiante">
                    <Course />
                  </SoloRol>
                }
              />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </AuthProvider>
      </BrowserRouter>
    </StrictMode>,
  ),
);
