#!/usr/bin/env bash
# Calibracion del generador de carga (G1, issue #222): una corrida corta, de SOLO LECTURA, a traves del
# balanceador, para comprobar que el generador no limita lo que mide (capacity_contraste_generador.py)
# antes de las corridas de G3 y G4. No usa cuentas, no necesita contrasena y no escribe nada en la plataforma.
#
#   BASE_URL=https://<balanceador> ENTORNO=nube-entrega3 \
#     bash scripts/capacity_calibracion_generador.sh <usuarios> [duracion_s] [etiqueta]
#   ej. BASE_URL=https://136.68.55.254.sslip.io bash scripts/capacity_calibracion_generador.sh 25 180 calibracion25
#
# Variables: JMETER_IMAGE (por omision justb4/jmeter:latest; en la VM del generador, el digest de generator.tf),
#            RAMP_SECONDS (30), THINK_MS (1000), THINK_RANGE_MS (1000), RESULTS_ROOT.
# Salida: <RESULTS_ROOT>/<etiqueta>_<fecha>/ con resultados.jtl, generador.csv, ventana.txt, destino.json y
# resumen.json: la misma forma que el escenario 1, para que capacity_postproceso.sh la procese igual.
set -euo pipefail

USERS="${1:?Uso: capacity_calibracion_generador.sh <usuarios> [duracion_s] [etiqueta]}"
DURACION="${2:-180}"
LABEL="${3:-calibracion${USERS}}"
BASE_URL="${BASE_URL:?Declara el destino: BASE_URL=https://<balanceador>}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PLAN_DIR="${ROOT}/capacity-planning"
RUN="${LABEL}_$(date +%Y%m%d_%H%M%S)"
RESULTS_ROOT="${RESULTS_ROOT:-${ROOT}/docs/entrega3/evidencias/G1/calibracion}"
OUT="${RESULTS_ROOT}/${RUN}"
IMAGEN="${JMETER_IMAGE:-justb4/jmeter:latest}"

PROTO="${BASE_URL%%://*}"; REST="${BASE_URL#*://}"; REST="${REST%%/*}"; HOST="${REST%%:*}"
if [[ "$REST" == *:* ]]; then PORT="${REST##*:}"; else [ "$PROTO" = https ] && PORT=443 || PORT=80; fi
PY=python; command -v python >/dev/null 2>&1 || PY=python3

mkdir -p "${OUT}"
echo "==> ${RUN}: ${USERS} usuarios, ${DURACION} s, contra ${BASE_URL} (solo lectura)"

CONTENEDOR="jmeter-${RUN}"
(
  echo "hora_utc,cpu_pct,memoria" > "${OUT}/generador.csv"
  while sleep 10; do
    linea="$(docker stats --no-stream --format '{{.CPUPerc}},{{.MemUsage}}' "${CONTENEDOR}" 2>/dev/null || true)"
    [ -n "${linea}" ] && echo "$(date -u +%H:%M:%S),${linea}" >> "${OUT}/generador.csv"
  done
) &
MUESTREO=$!
trap 'kill ${MUESTREO} 2>/dev/null || true' EXIT

date -u +%Y-%m-%dT%H:%M:%SZ > "${OUT}/ventana.txt"
MSYS_NO_PATHCONV=1 docker run --rm --name "${CONTENEDOR}" \
  -v "${PLAN_DIR}:/plan:ro" -v "${OUT}:/salida" \
  "${IMAGEN}" \
  -n -t /plan/calibracion_generador.jmx -l /salida/resultados.jtl -j /salida/jmeter.log \
  -Jusers="${USERS}" -Jramp="${RAMP_SECONDS:-30}" -Jduration="${DURACION}" \
  -Jthink_ms="${THINK_MS:-1000}" -Jthink_range_ms="${THINK_RANGE_MS:-1000}" \
  -Jhost="${HOST}" -Jport="${PORT}" -Jprotocol="${PROTO}" -Jorigin="${BASE_URL}" \
  | tail -n 12
date -u +%Y-%m-%dT%H:%M:%SZ >> "${OUT}/ventana.txt"
kill "${MUESTREO}" 2>/dev/null || true

cat > "${OUT}/destino.json" <<JSON
{
  "base_url": "${BASE_URL}",
  "entorno": "${ENTORNO:-sin declarar}",
  "plan": "calibracion_generador.jmx",
  "usuarios": ${USERS},
  "rampa_s": ${RAMP_SECONDS:-30},
  "pausa_ms": "${THINK_MS:-1000}+-${THINK_RANGE_MS:-1000}",
  "sesiones": 1,
  "calentamiento_omitido_s": ${WARMUP_SKIP_SECONDS:-45},
  "commit": "$(git -C "${ROOT}" rev-parse --short HEAD 2>/dev/null || echo desconocido)",
  "generador": "${IMAGEN} en $(hostname)",
  "nucleos_generador": $(nproc 2>/dev/null || echo 0),
  "memoria_generador_gib": $(awk '/MemTotal/ {printf "%.1f", $2/1048576}' /proc/meminfo 2>/dev/null || echo 0),
  "inicio_utc": "$(sed -n 1p "${OUT}/ventana.txt")",
  "fin_utc": "$(sed -n 2p "${OUT}/ventana.txt")"
}
JSON

"$PY" "${SCRIPT_DIR}/capacity_resumen_jtl.py" "${OUT}/resultados.jtl" "${WARMUP_SKIP_SECONDS:-45}" \
  --exigir-destino ${EXIGIR_NUBE:+--exigir-nube} | tee "${OUT}/resumen.txt"
echo "==> ${OUT}"
