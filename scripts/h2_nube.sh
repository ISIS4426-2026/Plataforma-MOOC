#!/usr/bin/env bash
# Atajo para el piloto del escenario 1 contra la nube (issue #132, H2).
# Existe para escribir comandos cortos: es solo una envoltura de
# capacity_login_tokens.sh y run_escenario1.sh con el entorno de la nube.
#
#   bash scripts/h2_nube.sh tokens   # inicia sesion con 3 cuentas (~25 s)
#   bash scripts/h2_nube.sh reiniciar # deja las cuentas de carga sin inscripcion ni intentos
#   bash scripts/h2_nube.sh piloto   # piloto de 3 usuarios (requiere Docker abierto)
#   bash scripts/h2_nube.sh verificar # comprueba el estado en la base tras el piloto
#
# La contrasena de las cuentas de carga se pide por teclado y no se guarda.

set -euo pipefail

export BASE_URL="${BASE_URL:-https://34.24.52.111.sslip.io}"
cd "$(dirname "${BASH_SOURCE[0]}")/.."

case "${1:-}" in
  tokens)
    if [ -z "${CAPACITY_PASSWORD:-}" ]; then
      read -rsp "Contrasena de prueba de las cuentas de carga (no se ve al escribir): " CAPACITY_PASSWORD
      echo
      export CAPACITY_PASSWORD
    fi
    bash scripts/capacity_login_tokens.sh 3 1
    ;;
  reiniciar)
    bash scripts/cargar_cuentas_carga_nube.sh scripts/seeds/capacity_reset.sql
    ;;
  verificar)
    bash scripts/cargar_cuentas_carga_nube.sh scripts/seeds/capacity_verificar_estado.sql
    ;;
  piloto)
    RAMP_SECONDS=3 THINK_MS=800 THINK_RANGE_MS=800 WARMUP_SKIP_SECONDS=0 \
      bash scripts/run_escenario1.sh 3 nube-piloto
    ;;
  *)
    echo "Uso: bash scripts/h2_nube.sh tokens|reiniciar|piloto|verificar"
    exit 1
    ;;
esac
