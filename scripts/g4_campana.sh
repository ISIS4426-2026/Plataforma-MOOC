#!/usr/bin/env bash
# G4 (#224): la campana del escenario 2 contra la arquitectura elastica, EN LA VM GENERADORA, con nohup.
#
#   BASE_URL=<lb> DB_HOST=<ip de la base> QUEUE_IP=<ip de la cola> \
#     nohup bash scripts/g4_campana.sh > ~/g4-logs/campana.log 2>&1 &
#
# Antes: el binario linux (bin/capacity_escenario2), los tres videos de perfil en scratch/ (generar_media_prueba.sh)
# y la contrasena de la base en /run/mooc-db-pw (RUNBOOK de G3, seccion 1). El destino y el entorno se declaran:
# sin ellos la campana se niega a medir (A6). Cada nivel mide por etapa y deja el drenaje de la cola al final.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
: "${BASE_URL:?declara el balanceador: BASE_URL=https://<ip>.sslip.io}"
: "${DB_HOST:?declara la IP privada de la base (terraform output -raw db_private_ip)}"
: "${QUEUE_IP:?declara la IP privada de la cola (terraform output -raw queue_private_ip)}"
HLS_BASE="${HLS_BASE:-https://storage.googleapis.com/plataforma-mooc-entrega2-hls}"
SALIDA="${SALIDA:-docs/entrega3/evidencias/G4/campana}"
HASTA="${HASTA_NIVEL:-3}"

PW_FILE="${PGPASSWORD_FILE:-/run/mooc-db-pw}"
[ -s "${PW_FILE}" ] || { echo "No hay ${PW_FILE}: deja la contrasena de la base (RUNBOOK de G3, seccion 1)." >&2; exit 1; }
DB_CONN="postgres://moocuser:$(tr -d '[:space:]' < "${PW_FILE}")@${DB_HOST}:5432/moocdb?sslmode=require"
export DB_CONN

docker image inspect mooc-ttff >/dev/null 2>&1 || docker build -q -t mooc-ttff scripts/capacity_escenario2/ttff
mkdir -p "${SALIDA}"
echo "[$(date +%H:%M:%S)] inicio de la campana del escenario 2 hasta el nivel ${HASTA} contra ${BASE_URL}"
./bin/capacity_escenario2 \
  -base "${BASE_URL}" -hls-base "${HLS_BASE}" -entorno nube-entrega3 \
  -redis "${QUEUE_IP}:6379" -out-dir "${SALIDA}" \
  -worker-concurrency 2 -max-segmentos 10 -patron-consumo cadencia \
  -hasta-nivel "${HASTA}" -ttff-muestras 6
echo "[$(date +%H:%M:%S)] fin (codigo $?)"
