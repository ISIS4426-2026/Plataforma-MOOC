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
    // Solo archivos versionados: un PDF suelto en la carpeta no cambia el codigo compilado.
    const sucio = execSync("git status --porcelain --untracked-files=no", { stdio: ["ignore", "pipe", "ignore"] }).toString().trim();
    return sucio ? `${sha}-sin-commit` : sha;
  } catch {
    return "desconocido";
  }
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "VITE_");
  const devApiTarget = env.VITE_DEV_API_TARGET || "http://localhost:8080";
  const devApiRemoto = devApiTarget.startsWith("https://");
  return {
    plugins: [
      // Solo en desarrollo: lo que en la nube sirve nginx en /config.json (ver nginx.web.conf).
      {
        name: "dev-runtime-config",
        configureServer(server) {
          server.middlewares.use("/config.json", (_req, res) => {
            res.setHeader("Content-Type", "application/json");
            res.end(JSON.stringify({ hlsBaseUrl: env.VITE_DEV_HLS_BASE_URL || "" }));
          });
        },
      },
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
      //
      // Contra el entorno desplegado (VITE_DEV_API_TARGET=https://<balanceador>) el proxy manda
      // `Origin: <balanceador>`: es el unico origen que acepta CSRF_ALLOWED_ORIGINS en la nube, y asi
      // se prueba la interfaz contra la API real sin tocar el despliegue. Solo para desarrollo.
      proxy: {
        "/api": {
          target: devApiTarget,
          changeOrigin: devApiRemoto,
          secure: true,
          headers: devApiRemoto ? { Origin: new URL(devApiTarget).origin } : undefined,
        },
      },
    },
    // hls.js pesa ~570 kB y se carga aparte, solo al abrir un video: el aviso de tamano no aplica.
    build: { sourcemap: false, chunkSizeWarningLimit: 700 },
    test: { environment: "node", setupFiles: ["./scripts/vitest-v8-setup.mjs"] },
  };
});
