#!/usr/bin/env bash
# Corre el escenario 1 (issue #132, H2): actividad academica concurrente, con
# JMeter en Docker, desde la maquina de quien lo ejecuta (fuera de las VMs).
#
# Antes: tener el archivo de tokens (scripts/capacity_login_tokens.sh) y las
# cuentas de carga en estado limpio (scripts/seeds/capacity_reset.sql).
#
# Uso:
#   BASE_URL=https://34.24.52.111.sslip.io bash scripts/run_escenario1.sh <usuarios> [etiqueta]
#   ej. piloto:   bash scripts/run_escenario1.sh 3 piloto
#       nivel 25: BASE_URL=... bash scripts/run_escenario1.sh 25 nivel25-r1
#
# Variables opcionales (todas tienen el valor del plan; cambiarlas cambia la mezcla):
#   RAMP_SECONDS (30)   THINK_MS (4000)   THINK_RANGE_MS (2000)   SESSIONS (3)
#   WARMUP_SKIP_SECONDS (45) segundos iniciales que el resumen omite
#
# Variante separada, rafaga de login (mide el limitador, no la capacidad):
#   VARIANT=login CAPACITY_PASSWORD=<contrasena de prueba> bash scripts/run_escenario1.sh 20 login
#
# Salida: docs/entrega2/evidencias/H2/resultados/<etiqueta>_<fecha>/
#   resultados.jtl  (muestras originales, una fila por peticion)  y  resumen.txt

set -euo pipefail

USERS="${1:?Uso: run_escenario1.sh <usuarios> [etiqueta]}"
LABEL="${2:-corrida}"
BASE_URL="${BASE_URL:-http://host.docker.internal:8080}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
H2_DIR="${ROOT}/docs/entrega2/evidencias/H2"
DATA_DIR="${ROOT}/capacity-planning/datos"
STAMP="$(date +%Y%m%d_%H%M%S)"
RUN="${LABEL}_${STAMP}"
OUT="${H2_DIR}/resultados/${RUN}"

[ -f "${DATA_DIR}/tokens.csv" ] || { echo "Falta ${DATA_DIR}/tokens.csv: corre scripts/capacity_login_tokens.sh"; exit 1; }
lines=$(($(wc -l < "${DATA_DIR}/tokens.csv") - 1))
[ "$lines" -ge "$USERS" ] || { echo "tokens.csv tiene ${lines} cuentas y pides ${USERS} usuarios."; exit 1; }

# Separar protocolo, host y puerto de BASE_URL.
PROTO="${BASE_URL%%://*}"
REST="${BASE_URL#*://}"; REST="${REST%%/*}"
HOST="${REST%%:*}"
if [[ "$REST" == *:* ]]; then PORT="${REST##*:}"; else [ "$PROTO" = https ] && PORT=443 || PORT=80; fi

PLAN="escenario1.jmx"
EXTRA_ARGS=()
if [ "${VARIANT:-}" = "login" ]; then
  PLAN="escenario1_login_rafaga.jmx"
  : "${CAPACITY_PASSWORD:?La rafaga de login necesita CAPACITY_PASSWORD (la contrasena de prueba de la semilla)}"
  # La contrasena viaja en un archivo de propiedades dentro de capacity-planning/datos
  # (ignorado por git), no como -J: JMeter imprime los argumentos en su log.
  umask 077
  printf 'password=%s\n' "${CAPACITY_PASSWORD}" > "${DATA_DIR}/login.properties"
  EXTRA_ARGS=(-q /jmeter/capacity/login.properties)
  RAMP_SECONDS="${RAMP_SECONDS:-1}"
  WARMUP_SKIP_SECONDS=0
fi

mkdir -p "${OUT}"
echo "==> ${RUN} (${PLAN}): ${USERS} usuarios contra ${BASE_URL}"
MSYS_NO_PATHCONV=1 docker run --rm \
  -v "${H2_DIR}:/jmeter" -v "${DATA_DIR}:/jmeter/capacity:ro" \
  justb4/jmeter:latest \
  -n -t "/jmeter/${PLAN}" -l "/jmeter/resultados/${RUN}/resultados.jtl" -j "/jmeter/resultados/${RUN}/jmeter.log" \
  -Jusers="${USERS}" -Jramp="${RAMP_SECONDS:-30}" -Jsessions="${SESSIONS:-3}" \
  -Jthink_ms="${THINK_MS:-4000}" -Jthink_range_ms="${THINK_RANGE_MS:-2000}" \
  -Jhost="${HOST}" -Jport="${PORT}" -Jprotocol="${PROTO}" -Jorigin="${BASE_URL}" \
  -JrunId="${RUN}" -Jtokens=/jmeter/capacity/tokens.csv "${EXTRA_ARGS[@]}" \
  | tail -n 12

PY=python; command -v python >/dev/null 2>&1 || PY=python3
"$PY" "${SCRIPT_DIR}/capacity_resumen_jtl.py" "${OUT}/resultados.jtl" "${WARMUP_SKIP_SECONDS:-45}" | tee "${OUT}/resumen.txt"
echo "==> Resultados originales: ${OUT}/resultados.jtl"
