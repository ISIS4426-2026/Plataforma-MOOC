#!/usr/bin/env bash
# G3 (#223): la matriz de corridas de UN operando (capacity-planning/entrega3_g3_g4.md, §2.3). Se lanza EN LA VM
# GENERADORA con nohup: una campana dura horas y no debe depender de que la sesion SSH siga abierta.
#
#   OPERANDO=R|E DB_HOST=<ip> BASE_URL=<lb> JMETER_IMAGE=<digest> ENTORNO=nube-entrega3 \
#     nohup bash scripts/g3_campana.sh > ~/g3-logs/campana_<operando>.log 2>&1 &
#
# Antes: tokens vigentes (scripts/capacity_tokens_semilla.sh), la contrasena de la base en /run/mooc-db-pw y el
# autoescalador de la API en el estado del operando (R: min = max = 1; E: 1 a 3). Eso se hace desde fuera de la
# VM, que no tiene permisos de Compute. Si una serie activa un criterio de PARADA (escenario1.md §8.3) la campana
# se detiene: no se reintenta un nivel sin cambiar algo y documentarlo.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
: "${OPERANDO:?OPERANDO=R o OPERANDO=E}"
export MODO=vm REUSAR_TOKENS=1
LOG_SERIE="$(mktemp)"
log() { echo "[$(date +%H:%M:%S)] $*"; }

serie() { # $1 = titulo ; el resto = comando
  local titulo="$1"; shift
  log "=== ${OPERANDO}: ${titulo}"
  "$@" 2>&1 | tee "${LOG_SERIE}"
  if grep -q "PARADA" "${LOG_SERIE}"; then
    log "PARADA en ${titulo}: la campana se detiene aqui."; exit 3
  fi
}
A()  { bash scripts/h3_nube.sh repetir "$@"; }
B()  { RAMPA_FIJA=15 MARGEN_CALENTAMIENTO=10 bash scripts/h3_nube.sh repetir "$@"; }
C()  { RAMPA_FIJA=10 MARGEN_CALENTAMIENTO=5  bash scripts/h3_nube.sh repetir "$@"; }
S()  { PAUSA_ENTRE_CORRIDAS=10 RAMPA_FIJA=15 MARGEN_CALENTAMIENTO=10 bash scripts/h3_nube.sh repetir "$@"; }

log "inicio del operando ${OPERANDO} (commit $(git rev-parse --short HEAD))"
case "${OPERANDO}" in
  R)
    serie "A100 x3" A 100 4000 2000 3 A100
    serie "A200 x3" A 200 4000 2000 3 A200
    serie "B100 x3" B 100 1000 1000 3 B100
    serie "B200 x3" B 200 1000 1000 3 B200
    serie "C200 x3" C 200 250 500 3 C200
    ;;
  E)
    serie "escalera (1,10,25,50,100,200 + B100 y B200)" bash scripts/h3_nube.sh escalera
    serie "A100 x2 mas" A 100 4000 2000 2 A100
    serie "A200 x2 mas" A 200 4000 2000 2 A200
    serie "B100 x2 mas" B 100 1000 1000 2 B100
    serie "B200 x2 mas" B 200 1000 1000 2 B200
    serie "C200 x3" C 200 250 500 3 C200
    serie "S200 sostenida x12" S 200 1000 1000 12 S200
    ;;
  *) echo "OPERANDO debe ser R o E" >&2; exit 1 ;;
esac
log "fin del operando ${OPERANDO}"
