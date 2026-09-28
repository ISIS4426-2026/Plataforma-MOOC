#!/usr/bin/env bash
# Paso previo del escenario 1 (issue #132, H2): inicia sesion con las cuentas de
# carga y guarda su token en un archivo LOCAL que despues lee JMeter.
#
# Por que es un paso aparte y no parte del recorrido medido:
#   el login esta limitado a 10 intentos por minuto por IP y todo el generador de
#   carga sale de una sola IP. Si el login entrara al recorrido, lo que se
#   mediria seria ese limitador (429), no la capacidad de la plataforma. Las
#   sesiones duran 24 h, asi que se inicia sesion una vez, con ritmo, y el
#   recorrido medido usa el token. La rafaga de login se mide como variante
#   separada (capacity-planning/escenario1.md, seccion 4).
#
# Uso:
#   BASE_URL=https://34.24.52.111.sslip.io CAPACITY_PASSWORD=<contrasena de prueba> \
#     bash scripts/capacity_login_tokens.sh [cantidad] [primera]
#     cantidad  cuantas cuentas (por defecto 25)
#     primera   indice de la primera cuenta (por defecto 1; hay 200)
#
# Salida: capacity-planning/datos/tokens.csv  (indice,email,token)
#   Esta en .gitignore: son credenciales de sesion, NUNCA se suben al repositorio.
#   Tampoco se imprimen: el script solo cuenta.
#
# La contrasena es la de prueba de la semilla (encabezado de
# scripts/seeds/capacity_data.sql) y se pasa por CAPACITY_PASSWORD: no queda
# escrita en este archivo.

set -uo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
COUNT="${1:-25}"
FIRST="${2:-1}"
PASSWORD="${CAPACITY_PASSWORD:?Falta CAPACITY_PASSWORD: la contrasena de prueba de las cuentas de carga (esta en el encabezado de scripts/seeds/capacity_data.sql)}"
# 7 s entre intentos = ~8,5 por minuto, por debajo del limite de 10/min.
PAUSE="${LOGIN_PAUSE_SECONDS:-7}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)/capacity-planning/datos"
OUT="${OUT_DIR}/tokens.csv"
mkdir -p "${OUT_DIR}"

if command -v curl.exe >/dev/null 2>&1; then CURL=curl.exe; else CURL=curl; fi
PY=python; command -v python >/dev/null 2>&1 || PY=python3

echo "==> Iniciando sesion de ${COUNT} cuentas de carga en ${BASE_URL} (una cada ${PAUSE}s)"
echo "indice,email,token" > "${OUT}"

ok=0; fail=0
for ((n = 0; n < COUNT; n++)); do
  i=$((FIRST + n))
  email="$(printf 'carga.estudiante%04d@plataforma-mooc.test' "$i")"
  attempts=0
  while :; do
    attempts=$((attempts + 1))
    body="$("$CURL" -s --max-time 30 -o - -w '\n%{http_code}' -X POST \
      -H 'Content-Type: application/json' -H "Origin: ${BASE_URL}" \
      -d "{\"email\":\"${email}\",\"password\":\"${PASSWORD}\"}" \
      "${BASE_URL}/api/v1/auth/login")"
    status="$(printf '%s' "$body" | tail -n1 | tr -d '\r')"
    json="$(printf '%s' "$body" | sed '$d')"
    if [ "$status" = "200" ]; then
      token="$(printf '%s' "$json" | "$PY" -c 'import json,sys; print(json.load(sys.stdin)["token"])' | tr -d '\r')"
      echo "${i},${email},${token}" >> "${OUT}"
      ok=$((ok + 1))
      break
    elif [ "$status" = "429" ] && [ "$attempts" -lt 5 ]; then
      echo "   429 en ${email}: esperando 30 s y reintentando"
      sleep 30
    else
      echo "   FALLO ${email}: HTTP ${status}"
      fail=$((fail + 1))
      break
    fi
  done
  sleep "${PAUSE}"
done

echo "==> Listo: ${ok} sesiones guardadas, ${fail} fallos. Archivo: ${OUT}"
[ "$fail" -eq 0 ]
