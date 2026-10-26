// Comprueba que el bundle compilado no lleve secretos ni direcciones internas (criterio de D1)
// y deja el resultado en dist/bundle-check.txt. Falla (codigo 1) si encuentra algo.
//
// Uso:  npm run build && npm run check:bundle
//
// Un bundle de navegador lo lee cualquiera: lo que entra aqui es publico. La busqueda cubre
// lo que de verdad se ha colado en proyectos parecidos: claves y tokens de proveedores, llaves
// privadas, y direcciones privadas o de desarrollo (localhost, el servicio "api" de Compose,
// IPs RFC 1918) que revelarian la topologia interna.
import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join, relative } from "node:path";

const DIST = process.argv[2] ?? "dist";

const REGLAS = [
  ["llave privada", /-----BEGIN [A-Z ]*PRIVATE KEY-----/],
  ["clave de Google API", /AIza[0-9A-Za-z_-]{35}/],
  ["clave de acceso AWS", /AKIA[0-9A-Z]{16}/],
  ["token de GitHub", /gh[pousr]_[A-Za-z0-9]{30,}/],
  ["token de Slack", /xox[baprs]-[A-Za-z0-9-]{10,}/],
  ["JWT", /eyJ[A-Za-z0-9_-]{15,}\.[A-Za-z0-9_-]{15,}\.[A-Za-z0-9_-]{10,}/],
  ["cadena de conexion con credenciales", /\b[a-z][a-z0-9+.-]*:\/\/[^\s"'/:@]+:[^\s"'/@]+@/i],
  ["IP privada 10.x", /\b10\.\d{1,3}\.\d{1,3}\.\d{1,3}\b/],
  ["IP privada 172.16-31.x", /\b172\.(1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}\b/],
  ["IP privada 192.168.x", /\b192\.168\.\d{1,3}\.\d{1,3}\b/],
  // "http://localhost" a secas (sin puerto) es la base de relleno que react-router usa para
  // interpretar URLs relativas; no es una direccion nuestra. Lo que delata una URL de
  // desarrollo es el puerto o una IP de loopback.
  ["localhost con puerto o loopback", /\blocalhost:\d{2,5}\b|\b127\.0\.0\.1\b|\b0\.0\.0\.0\b/],
  ["servicio interno de Compose", /\b(api|postgres|redis|minio|worker):\d{2,5}\b|host\.docker\.internal/],
  // DNS interno de Compute Engine (<vm>.<zona>.c.<proyecto>.internal) y el servidor de metadatos.
  // No se busca ".internal" suelto: es tambien el nombre de una propiedad de react-router.
  ["dominio interno de GCP", /\.c\.[a-z0-9-]+\.internal\b|metadata\.google\.internal/],
];

function archivos(dir) {
  return readdirSync(dir).flatMap((n) => {
    const p = join(dir, n);
    return statSync(p).isDirectory() ? archivos(p) : [p];
  });
}

const hallazgos = [];
let revisados = 0;
for (const f of archivos(DIST)) {
  if (!/\.(js|mjs|css|html|json|map|txt)$/.test(f) || f.endsWith("bundle-check.txt")) continue;
  revisados++;
  const texto = readFileSync(f, "utf8");
  for (const [nombre, re] of REGLAS) {
    const m = texto.match(re);
    if (m) hallazgos.push(`${nombre}: ${relative(DIST, f)} -> ${m[0].slice(0, 60)}`);
  }
}

const lineas = [
  `# Revision del bundle (${DIST})`,
  `# Archivos revisados: ${revisados}. Reglas: ${REGLAS.length}.`,
  hallazgos.length ? "# RESULTADO: HALLAZGOS" : "# RESULTADO: sin hallazgos",
  ...hallazgos,
];
writeFileSync(join(DIST, "bundle-check.txt"), lineas.join("\n") + "\n");
console.log(lineas.join("\n"));
process.exit(hallazgos.length ? 1 : 0);
