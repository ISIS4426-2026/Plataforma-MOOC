import { execSync } from "node:child_process";
import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";

// La version del build es el commit que lo produjo. En CI y en la imagen llega
// por VITE_APP_COMMIT (el contexto de Docker no incluye .git); en local se lee
// de git. Si no hay ninguna de las dos, "desconocido" -- nunca un valor inventado.
function commit(env: Record<string, string>): string {
  if (env.VITE_APP_COMMIT) return env.VITE_APP_COMMIT;
  try {
    const sha = execSync("git rev-parse --short=12 HEAD", { stdio: ["ignore", "pipe", "ignore"] })
      .toString()
      .trim();
    // Un build con cambios sin commitear no corresponde a ningun commit: se marca, para que
    // la version visible nunca afirme mas de lo que es.
    const sucio = execSync("git status --porcelain", { stdio: ["ignore", "pipe", "ignore"] }).toString().trim();
    return sucio ? `${sha}-sin-commit` : sha;
  } catch {
    return "desconocido";
  }
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "VITE_");
  return {
    plugins: [
      react(),
      {
        // /version.json: la version del build, consultable sin abrir la interfaz. Es lo que se
        // registra como evidencia de que el frontend desplegado corresponde a un commit.
        name: "version-json",
        generateBundle() {
          this.emitFile({
            type: "asset",
            fileName: "version.json",
            source: JSON.stringify({ commit: commit(env), builtAt: new Date().toISOString() }, null, 2),
          });
        },
      },
    ],
    define: {
      __APP_COMMIT__: JSON.stringify(commit(env)),
      __APP_BUILT_AT__: JSON.stringify(new Date().toISOString()),
    },
    server: {
      port: 5173,
      // En desarrollo el navegador habla con este mismo origen y Vite reenvia
      // /api a la API local: asi la cookie de sesion y la comprobacion de origen
      // se comportan igual que detras del balanceador (mismo origen, sin CORS).
      proxy: {
        "/api": {
          target: env.VITE_DEV_API_TARGET || "http://localhost:8080",
          changeOrigin: false,
        },
      },
    },
    build: { sourcemap: false },
    test: { environment: "node" },
  };
});
