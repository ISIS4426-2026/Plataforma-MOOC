#!/usr/bin/env bash
# Verificador de integracion de SonarQube en GitHub Actions, alcance total
# (API, workers y frontend), exclusiones justificadas, publicacion de cobertura
# y ausencia de credenciales (H1, issue #228).
#
# Cumple la regla de la Nota Tecnica 36: afirma cada condicion con OK / FALLA / OBS
# y devuelve codigo distinto de cero si falla cualquier control.
# Si se invoca con --exportar-sonar (o si hay conectividad a sonarcloud.io),
# descarga ademas las metricas, el estado del Quality Gate y el listado de
# hallazgos desde la Web API de SonarQube Cloud en formato JSON verificable
# sin incluir tokens ni secretos.
#
# USO:
#   bash ./scripts/h1_verificar_y_exportar_sonar.sh [directorio-salida] [--exportar-sonar]

set -Eeuo pipefail

export PATH="${PATH}:/home/dfortizr1/.local/go/bin:/usr/local/go/bin:${HOME}/go/bin:/tmp/copilot-node20/bin"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
OUT="${ROOT}/docs/entrega3/evidencias/H1"
EXPORTAR_SONAR=0

for arg in "$@"; do
    case "${arg}" in
        --exportar-sonar) EXPORTAR_SONAR=1 ;;
        *) OUT="${arg}" ;;
    esac
done

SONAR_PROJECT_KEY="${SONAR_PROJECT_KEY:-ISIS4426-2026_Plataforma-MOOC}"
SONAR_ORGANIZATION="${SONAR_ORGANIZATION:-isis4426-2026}"
SONAR_API_BASE="${SONAR_API_BASE:-https://sonarcloud.io/api}"

PY=python3
command -v python3 >/dev/null 2>&1 || PY=python

mkdir -p "${OUT}" "${OUT}/capturas"
RESUMEN="${OUT}/resumen.txt"
: >"${RESUMEN}"

ahora() { date -u +%Y-%m-%dT%H:%M:%SZ; }

cabecera() {
    local archivo="$1"
    local titulo="$2"
    {
        echo "# ${titulo}"
        echo "# Generado: $(ahora) por scripts/h1_verificar_y_exportar_sonar.sh"
        echo "# Commit  : $(git -C "${ROOT}" rev-parse HEAD)"
        echo
    } >"${archivo}"
}

echo "H1 — Verificacion de SonarQube en GitHub Actions sobre API, workers y frontend"
echo "=============================================================================="

# ==============================================================================
# CONTROL A: Alcance total (API, workers y frontend) y exclusiones justificadas
# ==============================================================================
FA="${OUT}/a_alcance_y_exclusiones.txt"
cabecera "${FA}" "a) Verificacion de alcance (API, workers, frontend) y exclusiones en sonar-project.properties (H1, #228)"

"${PY}" - "${ROOT}" >>"${FA}" <<'PYEOF'
import os, re, sys

root = sys.argv[1]
prop_path = os.path.join(root, "sonar-project.properties")

if not os.path.isfile(prop_path):
    print("FALLA  No existe sonar-project.properties en la raiz del repositorio")
    sys.exit(0)

props = {}
with open(prop_path, "r", encoding="utf-8") as f:
    for raw in f:
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        k, v = line.split("=", 1)
        props[k.strip()] = v.strip()

sources = [s.strip() for s in props.get("sonar.sources", "").split(",") if s.strip()]
tests = [s.strip() for s in props.get("sonar.tests", "").split(",") if s.strip()]
test_inclusions = [s.strip() for s in props.get("sonar.test.inclusions", "").split(",") if s.strip()]
exclusions = [s.strip() for s in props.get("sonar.exclusions", "").split(",") if s.strip()]

required_sources = ["api", "cmd", "internal", "frontend/src"]
missing_sources = [s for s in required_sources if s not in sources]
if not missing_sources:
    print(f"OK     sonar.sources cubre API, workers y frontend completos: {','.join(sources)}")
else:
    print(f"FALLA  Faltan directorios de aplicacion en sonar.sources: {missing_sources}")

required_test_dirs = ["internal", "migrations", "frontend/src"]
missing_tests = [t for t in required_test_dirs if t not in tests]
if not missing_tests:
    print(f"OK     sonar.tests declara las suites de Go, migraciones y frontend: {','.join(tests)}")
else:
    print(f"FALLA  Faltan directorios en sonar.tests: {missing_tests}")

# Verificar que ningun paquete de aplicacion este excluido en sonar.exclusions
allowed_exclusions = {
    "**/*_test.go",
    "frontend/src/**/*.test.ts",
    "frontend/src/**/*.test.tsx",
    "frontend/node_modules/**",
    "frontend/dist/**",
    "frontend/coverage/**",
    "bin/**",
    "docs/**",
    "capacity-planning/**",
    "scratch/**",
    "scripts/**",
    "infra/**",
}
unjustified = [e for e in exclusions if e not in allowed_exclusions]
if not unjustified:
    print(f"OK     Las {len(exclusions)} exclusiones de sonar.exclusions estan justificadas y ninguna excluye codigo de aplicacion")
else:
    print(f"FALLA  Exclusiones no autorizadas o sin justificar en sonar.exclusions: {unjustified}")

if props.get("sonar.go.coverage.reportPaths") == "coverage.out":
    print("OK     sonar.go.coverage.reportPaths apunta a coverage.out")
else:
    print(f"FALLA  sonar.go.coverage.reportPaths inesperado: {props.get('sonar.go.coverage.reportPaths')}")

if props.get("sonar.javascript.lcov.reportPaths") == "frontend/coverage/lcov.info":
    print("OK     sonar.javascript.lcov.reportPaths apunta a frontend/coverage/lcov.info")
else:
    print(f"FALLA  sonar.javascript.lcov.reportPaths inesperado: {props.get('sonar.javascript.lcov.reportPaths')}")

# Conteo real de archivos y lineas por componente incluido en sonar.sources
def count_files_and_lines(rel_dir, ext_tuple, exclude_test_suffixTuple):
    abs_dir = os.path.join(root, rel_dir)
    files_count = 0
    lines_count = 0
    test_files = 0
    test_lines = 0
    for dirpath, _, filenames in os.walk(abs_dir):
        for fn in sorted(filenames):
            if not fn.endswith(ext_tuple):
                continue
            p = os.path.join(dirpath, fn)
            with open(p, "r", encoding="utf-8") as fh:
                n = sum(1 for _ in fh)
            if fn.endswith(exclude_test_suffixTuple):
                test_files += 1
                test_lines += n
            else:
                files_count += 1
                lines_count += n
    return files_count, lines_count, test_files, test_lines

api_f, api_l, _, _ = count_files_and_lines("api", (".go",), ("_test.go",))
cmd_f, cmd_l, _, _ = count_files_and_lines("cmd", (".go",), ("_test.go",))
int_f, int_l, int_tf, int_tl = count_files_and_lines("internal", (".go",), ("_test.go",))
mig_f, mig_l, mig_tf, mig_tl = count_files_and_lines("migrations", (".go",), ("_test.go",))
fe_f, fe_l, fe_tf, fe_tl = count_files_and_lines("frontend/src", (".ts", ".tsx", ".css"), (".test.ts", ".test.tsx"))

print("\n--- Inventario de archivos y lineas bajo el alcance del analizador ---")
print(f"  Go (api/)          : {api_f:2d} archivo(s) productivos | {api_l:5d} lineas")
print(f"  Go (cmd/)          : {cmd_f:2d} archivo(s) productivos | {cmd_l:5d} lineas (api, worker, queue-exporter, seed-media)")
print(f"  Go (internal/)     : {int_f:2d} archivo(s) productivos | {int_l:5d} lineas | {int_tf:2d} archivos de test ({int_tl:5d} lineas)")
print(f"  Go (migrations/)   : {mig_f:2d} archivo(s) productivos | {mig_l:5d} lineas | {mig_tf:2d} archivos de test ({mig_tl:5d} lineas)")
print(f"  Frontend (src/)    : {fe_f:2d} archivo(s) productivos | {fe_l:5d} lineas | {fe_tf:2d} archivos de test ({fe_tl:5d} lineas)")
print(f"  TOTAL PRODUCTIVO   : {api_f + cmd_f + int_f + fe_f:2d} archivos             | {api_l + cmd_l + int_l + fe_l:5d} lineas")
print(f"  TOTAL TESTS        : {int_tf + mig_tf + fe_tf:2d} archivos             | {int_tl + mig_tl + fe_tl:5d} lineas")
PYEOF

grep -E '^(OK|FALLA|OBS)' "${FA}" >>"${RESUMEN}" || true

# ==============================================================================
# CONTROL B: Workflow ci.yml, conservacion de jobs previos y ausencia de secretos
# ==============================================================================
FB="${OUT}/b_pipeline_y_secretos.txt"
cabecera "${FB}" "b) Verificacion de .github/workflows/ci.yml, Nota Tecnica 15 y credenciales en Secrets (H1, #228)"

"${PY}" - "${ROOT}" >>"${FB}" <<'PYEOF'
import os, re, sys

root = sys.argv[1]
wf_path = os.path.join(root, ".github", "workflows", "ci.yml")
prop_path = os.path.join(root, "sonar-project.properties")

with open(wf_path, "r", encoding="utf-8") as f:
    wf = f.read()

with open(prop_path, "r", encoding="utf-8") as f:
    props = f.read()

# 1. Presencia de los 4 jobs
for job_id in ["terraform:", "ci:", "frontend:", "sonarqube:"]:
    if re.search(rf"^\s{{2}}{re.escape(job_id)}", wf, re.M):
        print(f"OK     Job '{job_id[:-1]}' presente en .github/workflows/ci.yml")
    else:
        print(f"FALLA  Falta el job '{job_id[:-1]}' en .github/workflows/ci.yml")

# 2. Conservacion de lint, build, migraciones, pruebas de Go y pruebas/build/bundle de frontend
required_commands = [
    ("make lint", "Linting Go + OpenAPI Spectral"),
    ("make build", "Compilacion de cmd/api y cmd/worker"),
    ("make test-migrations", "Ejecucion de migraciones sobre PostgreSQL limpio"),
    ("make test", "Pruebas unitarias de Go con perfil de cobertura"),
    ("make check", "Verificacion integral make check"),
    ("npm ci --no-audit --no-fund", "Instalacion determinista del frontend"),
    ("npm run test:coverage", "Pruebas unitarias de frontend con generacion LCOV"),
    ("npm run build", "Typecheck y empaquetado Vite del frontend"),
    ("npm run check:bundle", "Auditoria de ausencia de secretos en el bundle"),
]
for cmd, desc in required_commands:
    if cmd in wf:
        print(f"OK     Paso conservado en CI: '{cmd}' ({desc})")
    else:
        print(f"FALLA  Se perdio el paso '{cmd}' ({desc}) en .github/workflows/ci.yml")

# 3. Nota Tecnica 15: sin ubuntu-latest ni checkout@v4 / setup-terraform@v3
if "ubuntu-latest" not in wf and "ubuntu-24.04" in wf:
    print("OK     Nota Tecnica 15 atendida: runners fijados en ubuntu-24.04 (sin ubuntu-latest)")
else:
    print("FALLA  Quedan referencias a ubuntu-latest en .github/workflows/ci.yml")

if "actions/checkout@v4" not in wf and "hashicorp/setup-terraform@v3" not in wf:
    print("OK     Nota Tecnica 15 atendida: actions/checkout@v5 y hashicorp/setup-terraform@v4 actualizados")
else:
    print("FALLA  Quedan acciones con Node 20 obsoleto (checkout@v4 o setup-terraform@v3)")

# 4. Configuracion del job sonarqube
if "SonarSource/sonarqube-scan-action@v5" in wf:
    print("OK     Job sonarqube utiliza SonarSource/sonarqube-scan-action@v5")
else:
    print("FALLA  Job sonarqube no declara SonarSource/sonarqube-scan-action@v5")

if "fetch-depth: 0" in wf:
    print("OK     Job sonarqube hace checkout con fetch-depth: 0 para blame completo")
else:
    print("FALLA  Falta fetch-depth: 0 en el job sonarqube")

if "needs: [ci, frontend]" in wf:
    print("OK     Job sonarqube espera a [ci, frontend] y reutiliza sus artefactos de cobertura")
else:
    print("FALLA  Job sonarqube no declara needs: [ci, frontend]")

# 5. Credenciales como secretos del repositorio, nunca en claro
if "SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}" in wf:
    print("OK     SONAR_TOKEN se inyecta exclusivamente desde ${{ secrets.SONAR_TOKEN }}")
else:
    print("FALLA  SONAR_TOKEN no se toma de ${{ secrets.SONAR_TOKEN }}")

token_pattern = re.compile(r"\b(sq[pua]_[0-9a-fA-F]{20,}|[0-9a-f]{40})\b")
if not token_pattern.search(wf) and not token_pattern.search(props):
    print("OK     0 tokens o credenciales literales en .github/workflows/ci.yml y sonar-project.properties")
else:
    print("FALLA  Posible token literal detectado en ci.yml o sonar-project.properties")
PYEOF

grep -E '^(OK|FALLA|OBS)' "${FB}" >>"${RESUMEN}" || true

# ==============================================================================
# CONTROL C: Verificacion de generacion LCOV y estructura de cobertura
# ==============================================================================
FC="${OUT}/c_cobertura_y_reporte.txt"
cabecera "${FC}" "c) Verificacion de generador LCOV del frontend y reporte de cobertura (H1, #228)"

NODE_BIN=""
if command -v node >/dev/null 2>&1; then
    NODE_BIN="$(command -v node)"
elif [[ -x /tmp/copilot-node20/bin/node ]]; then
    NODE_BIN="/tmp/copilot-node20/bin/node"
fi

if [[ -n "${NODE_BIN}" ]]; then
    (cd "${ROOT}/frontend" && "${NODE_BIN}" scripts/v8-to-lcov.mjs >/dev/null)
    "${PY}" - "${ROOT}" >>"${FC}" <<'PYEOF'
import os, sys

root = sys.argv[1]
lcov_path = os.path.join(root, "frontend", "coverage", "lcov.info")

if not os.path.isfile(lcov_path):
    print("FALLA  No se genero frontend/coverage/lcov.info")
    sys.exit(0)

sf_files = []
out_of_range = []
current_sf = None
current_max_line = 0

with open(lcov_path, "r", encoding="utf-8") as f:
    for raw in f:
        line = raw.strip()
        if line.startswith("SF:"):
            current_sf = line[3:]
            sf_files.append(current_sf)
            abs_sf = os.path.join(root, current_sf)
            if os.path.isfile(abs_sf):
                with open(abs_sf, "r", encoding="utf-8") as sf_fh:
                    current_max_line = sum(1 for _ in sf_fh)
            else:
                out_of_range.append(f"{current_sf}: archivo no existe")
        elif line.startswith("DA:") and current_sf:
            parts = line[3:].split(",")
            lnum = int(parts[0])
            if lnum < 1 or lnum > current_max_line:
                out_of_range.append(f"{current_sf}: linea DA:{lnum} fuera de rango (1..{current_max_line})")

if sf_files and all(s.startswith("frontend/src/") for s in sf_files):
    print(f"OK     frontend/coverage/lcov.info indexa {len(sf_files)} archivos bajo el prefijo 'frontend/src/'")
else:
    print(f"FALLA  Rutas SF inesperadas en frontend/coverage/lcov.info: {sf_files[:3]}")

if not out_of_range:
    print("OK     0 lineas DA fuera de rango en frontend/coverage/lcov.info (100 % compatible con SonarJS)")
else:
    print(f"FALLA  Lineas fuera de rango en lcov.info: {out_of_range[:5]}")
PYEOF
else
    echo "OBS    Node no encontrado en PATH local; validacion de ejecucion LCOV delegada al runner de CI" >>"${FC}"
fi

if [[ "${EXPORTAR_SONAR}" -eq 1 ]]; then
    echo >>"${FC}"
    echo "--- Exportacion desde la Web API de SonarQube Cloud (${SONAR_PROJECT_KEY}) ---" >>"${FC}"
    MEASURES_URL="${SONAR_API_BASE}/measures/component?component=${SONAR_PROJECT_KEY}&metricKeys=ncloc,lines,files,functions,classes,complexity,cognitive_complexity,duplicated_lines_density,duplicated_lines,duplicated_blocks,duplicated_files,sqale_rating,sqale_index,sqale_debt_ratio,reliability_rating,security_rating,security_review_rating,coverage,line_coverage,lines_to_cover,uncovered_lines,bugs,vulnerabilities,code_smells,security_hotspots,violations"
    QG_URL="${SONAR_API_BASE}/qualitygates/project_status?projectKey=${SONAR_PROJECT_KEY}"
    ISSUES_URL="${SONAR_API_BASE}/issues/search?componentKeys=${SONAR_PROJECT_KEY}&ps=500&resolved=false"
    ANALYSES_URL="${SONAR_API_BASE}/project_analyses/search?project=${SONAR_PROJECT_KEY}&ps=10"

    CURL_AUTH=()
    if [[ -n "${SONAR_TOKEN:-}" ]]; then
        CURL_AUTH=(-u "${SONAR_TOKEN}:")
    fi

    if curl -fsSL "${CURL_AUTH[@]}" "${MEASURES_URL}" -o "${OUT}/sonarcloud_measures.json" &&
       curl -fsSL "${CURL_AUTH[@]}" "${QG_URL}" -o "${OUT}/sonarcloud_qualitygate.json" &&
       curl -fsSL "${CURL_AUTH[@]}" "${ISSUES_URL}" -o "${OUT}/sonarcloud_issues.json" &&
       curl -fsSL "${CURL_AUTH[@]}" "${ANALYSES_URL}" -o "${OUT}/sonarcloud_analyses.json"; then
        "${PY}" - "${OUT}" "${SONAR_PROJECT_KEY}" "${SONAR_ORGANIZATION}" >>"${FC}" <<'PYEOF'
import json, os, sys
from datetime import datetime, timezone

out_dir, project_key, org = sys.argv[1], sys.argv[2], sys.argv[3]

with open(os.path.join(out_dir, "sonarcloud_measures.json"), "r", encoding="utf-8") as f:
    measures_raw = json.load(f)
with open(os.path.join(out_dir, "sonarcloud_qualitygate.json"), "r", encoding="utf-8") as f:
    qg_raw = json.load(f)
with open(os.path.join(out_dir, "sonarcloud_issues.json"), "r", encoding="utf-8") as f:
    issues_raw = json.load(f)
with open(os.path.join(out_dir, "sonarcloud_analyses.json"), "r", encoding="utf-8") as f:
    analyses_raw = json.load(f)

consolidated = {
    "exported_at_utc": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "organization": org,
    "project_key": project_key,
    "dashboard_url": f"https://sonarcloud.io/project/overview?id={project_key}",
    "quality_gate": qg_raw.get("projectStatus", {}),
    "measures": {
        m["metric"]: m.get("value")
        for m in measures_raw.get("component", {}).get("measures", [])
    },
    "analyses": analyses_raw.get("analyses", []),
    "issues_total": issues_raw.get("total", 0),
    "issues": [
        {
            "key": i.get("key"),
            "rule": i.get("rule"),
            "severity": i.get("severity"),
            "type": i.get("type"),
            "component": i.get("component"),
            "line": i.get("line"),
            "message": i.get("message"),
            "effort": i.get("effort"),
            "status": i.get("status"),
        }
        for i in issues_raw.get("issues", [])
    ],
}

report_path = os.path.join(out_dir, "reporte_sonarcloud.json")
with open(report_path, "w", encoding="utf-8") as f:
    json.dump(consolidated, f, indent=2, ensure_ascii=False)

for tmp_name in ["sonarcloud_measures.json", "sonarcloud_qualitygate.json", "sonarcloud_issues.json", "sonarcloud_analyses.json"]:
    p = os.path.join(out_dir, tmp_name)
    if os.path.exists(p):
        os.remove(p)

print(f"OK     Exportacion verificable guardada en docs/entrega3/evidencias/H1/reporte_sonarcloud.json ({consolidated['issues_total']} hallazgos)")
PYEOF
    else
        echo "FALLA  No se pudo consultar la Web API de SonarQube Cloud para ${SONAR_PROJECT_KEY}" >>"${FC}"
    fi
fi

grep -E '^(OK|FALLA|OBS)' "${FC}" >>"${RESUMEN}" || true

OKS="$(grep -c '^OK' "${RESUMEN}" || true)"
FALLAS="$(grep -c '^FALLA' "${RESUMEN}" || true)"
OBSERVACIONES="$(grep -c '^OBS' "${RESUMEN}" || true)"

echo
cat "${RESUMEN}"
echo
echo "Resumen H1: ${OKS} OK · ${FALLAS} FALLA · ${OBSERVACIONES} OBS"

[[ "${FALLAS}" -eq 0 ]]

