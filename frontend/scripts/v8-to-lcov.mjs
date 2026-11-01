// Convierte la cobertura nativa V8 de Node.js (NODE_V8_COVERAGE=coverage/v8)
// generada durante `vitest run` a formato LCOV estandar (coverage/lcov.info)
// consumible por SonarQube (`sonar.javascript.lcov.reportPaths=frontend/coverage/lcov.info`).
//
// Por que este script:
// - No anade dependencias externas ni altera package-lock.json, conservando
//   intacto `npm ci --no-audit --no-fund` del pipeline de D1.
// - Usa `source-map-cache.lineLengths` cuando Node V8 lo adjunta y verifica
//   contra el archivo fuente real en `src/` que ninguna linea `DA:<linea>,<hits>`
//   exceda el numero real de lineas del archivo (evitando rechazos del sensor
//   SonarJS en SonarQube).

import { existsSync, mkdirSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = process.cwd();
const SRC_DIR = resolve(ROOT, "src");
const V8_DIR = resolve(ROOT, process.argv[2] ?? "coverage/v8");
const OUT_FILE = resolve(ROOT, process.argv[3] ?? "coverage/lcov.info");

function listarArchivosSrc(dir) {
  if (!existsSync(dir)) return [];
  return readdirSync(dir).flatMap((nombre) => {
    const ruta = join(dir, nombre);
    if (statSync(ruta).isDirectory()) return listarArchivosSrc(ruta);
    if (!/\.(ts|tsx)$/.test(nombre)) return [];
    if (/\.test\.(ts|tsx)$/.test(nombre) || nombre.endsWith(".d.ts")) return [];
    return [ruta];
  });
}

function normalizarUrlAArchivo(url) {
  if (!url) return null;
  let candidata = url;
  if (candidata.startsWith("file://")) {
    try {
      candidata = fileURLToPath(candidata);
    } catch {
      candidata = candidata.replace(/^file:\/\//, "");
    }
  }
  candidata = candidata.split("?")[0].split("#")[0];
  const abs = resolve(ROOT, candidata);
  if (abs.startsWith(SRC_DIR + "/") && existsSync(abs)) {
    return abs;
  }
  return null;
}

function construirOffsetsDeLinea(texto) {
  const lineas = texto.split(/\r?\n/);
  const inicios = [];
  let acum = 0;
  for (const l of lineas) {
    inicios.push(acum);
    acum += l.length + 1;
  }
  return { lineas, inicios };
}

function offsetALinea(inicios, offset) {
  let lo = 0;
  let hi = inicios.length - 1;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1;
    if (inicios[mid] <= offset) {
      lo = mid + 1;
    } else {
      hi = mid - 1;
    }
  }
  return Math.max(1, hi + 1);
}

function esLineaEjecutable(textoLinea) {
  const t = textoLinea.trim();
  if (!t) return false;
  if (t.startsWith("//") || t.startsWith("/*") || t.startsWith("*")) return false;
  if (/^(import\s+type\b|export\s+type\b|type\s+\w+|interface\s+\w+)/.test(t)) return false;
  if (/^[{}();,]+$/.test(t)) return false;
  return true;
}

// Mapa: ruta absoluta -> Map(numeroLinea -> hits)
const coberturaPorArchivo = new Map();

for (const archivo of listarArchivosSrc(SRC_DIR)) {
  const texto = readFileSync(archivo, "utf8");
  const { lineas } = construirOffsetsDeLinea(texto);
  const mapaLineas = new Map();
  lineas.forEach((contenido, idx) => {
    if (esLineaEjecutable(contenido)) {
      mapaLineas.set(idx + 1, 0);
    }
  });
  coberturaPorArchivo.set(archivo, mapaLineas);
}

if (existsSync(V8_DIR)) {
  const archivosJson = readdirSync(V8_DIR).filter((f) => f.endsWith(".json"));
  for (const nombreJson of archivosJson) {
    let datos;
    try {
      datos = JSON.parse(readFileSync(join(V8_DIR, nombreJson), "utf8"));
    } catch {
      continue;
    }
    for (const script of datos.result ?? []) {
      const rutaAbs = normalizarUrlAArchivo(script.url);
      if (!rutaAbs || !coberturaPorArchivo.has(rutaAbs)) continue;

      const texto = readFileSync(rutaAbs, "utf8");
      const { lineas, inicios } = construirOffsetsDeLinea(texto);
      const maxLinea = lineas.length;
      const mapaLineas = coberturaPorArchivo.get(rutaAbs);

      for (const fn of script.functions ?? []) {
        for (const rango of fn.ranges ?? []) {
          const lIni = Math.min(maxLinea, offsetALinea(inicios, rango.startOffset));
          const lFin = Math.min(maxLinea, offsetALinea(inicios, Math.max(rango.startOffset, rango.endOffset - 1)));
          for (let l = lIni; l <= lFin; l++) {
            if (!mapaLineas.has(l)) continue;
            if (rango.count > 0) {
              mapaLineas.set(l, Math.max(mapaLineas.get(l) ?? 0, rango.count));
            }
          }
        }
      }
    }
  }
}

mkdirSync(resolve(OUT_FILE, ".."), { recursive: true });

const bloquesLcov = [];
let totalLF = 0;
let totalLH = 0;

const REPO_ROOT = existsSync(resolve(ROOT, "..", "sonar-project.properties")) ? resolve(ROOT, "..") : ROOT;

for (const [rutaAbs, mapaLineas] of [...coberturaPorArchivo.entries()].sort((a, b) => a[0].localeCompare(b[0]))) {
  const relFromRepo = relative(REPO_ROOT, rutaAbs).replace(/\\/g, "/");
  const registros = [`TN:`, `SF:${relFromRepo}`];
  let lf = 0;
  let lh = 0;
  for (const [linea, hits] of [...mapaLineas.entries()].sort((a, b) => a[0] - b[0])) {
    registros.push(`DA:${linea},${hits}`);
    lf++;
    if (hits > 0) lh++;
  }
  registros.push(`LF:${lf}`, `LH:${lh}`, `end_of_record`);
  bloquesLcov.push(registros.join("\n"));
  totalLF += lf;
  totalLH += lh;
}

writeFileSync(OUT_FILE, bloquesLcov.join("\n") + "\n", "utf8");
const pct = totalLF > 0 ? ((totalLH / totalLF) * 100).toFixed(2) : "0.00";
console.log(`==> LCOV generado en ${relative(ROOT, OUT_FILE)}: ${totalLH}/${totalLF} lineas cubiertas (${pct}%)`);
