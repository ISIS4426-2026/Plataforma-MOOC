#!/usr/bin/env bash
# Genera los tokens de las cuentas de carga (G3) usando la contrasena de PRUEBA de la semilla, leida del
# encabezado de scripts/seeds/capacity_data.sql. No es una contrasena real: es el valor de prueba que ya esta
# escrito en el repositorio y en los entornos de Postman. No se imprime ni se guarda en otro lado.
#
#   BASE_URL=https://<balanceador> bash scripts/capacity_tokens_semilla.sh [cantidad]
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
CANTIDAD="${1:-200}"
CAPACITY_PASSWORD="$(grep -m1 'synthetic_data.sql:' scripts/seeds/capacity_data.sql | sed 's/.*: "\(.*\)" ->.*/\1/')"
[ -n "${CAPACITY_PASSWORD}" ] || { echo "No pude leer la contrasena de prueba del encabezado de capacity_data.sql." >&2; exit 1; }
export CAPACITY_PASSWORD
exec bash scripts/capacity_login_tokens.sh "${CANTIDAD}" 1
