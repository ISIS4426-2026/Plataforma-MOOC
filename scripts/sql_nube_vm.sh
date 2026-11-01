#!/usr/bin/env bash
# Aplica un archivo SQL a la base administrada DESDE la VM generadora de carga (G3 y G4).
#
# Es el equivalente de scripts/cargar_cuentas_carga_nube.sh para la arquitectura elastica. Aquel apuntaba a
# `mooc-web-server`, que ya no existe: la API es un grupo de VMs cuyos nombres cambian en cada rotacion. Esta
# VM, en cambio, vive en la misma red que la base (que solo tiene IP privada) mientras dure la campana.
#
# La contrasena nunca pasa por la maquina de quien opera ni se imprime: la VM la lee de Secret Manager con
# su propia cuenta de servicio (que solo puede leer ESE secreto, y solo mientras la VM exista:
# infra/terraform/generator.tf) y la entrega a psql por variable de entorno.
#
#   DB_HOST=<IP privada de la base> bash scripts/sql_nube_vm.sh scripts/seeds/capacity_reset.sql
#
# DB_HOST se obtiene en la maquina de quien opera con `terraform output -raw db_private_ip` y se pasa a
# la sesion de la VM; no se adivina, porque la IP cambia si alguien recrea la base.
#
# Imprime la salida de psql tal cual (formato alineado: lo lee verificar_estado de scripts/h3_nube.sh) y,
# al final, cuantas cuentas de carga hay.

set -euo pipefail

SQL="${1:?Uso: DB_HOST=<ip> bash scripts/sql_nube_vm.sh <archivo.sql>}"
[ -f "${SQL}" ] || { echo "No encuentro ${SQL}: corre este script desde la raiz del proyecto." >&2; exit 1; }
: "${DB_HOST:?Declara DB_HOST, la IP privada de la base (terraform output -raw db_private_ip)}"

PROJECT="${PROJECT_ID:-plataforma-mooc-entrega2}"
SECRET="${DB_SECRET:-db-password}"
META="http://metadata.google.internal/computeMetadata/v1"

for cmd in curl jq docker base64; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "Falta '$cmd' en esta maquina; este script corre en la VM generadora." >&2; exit 1; }
done

TOKEN="$(curl -sf -H 'Metadata-Flavor: Google' "${META}/instance/service-accounts/default/token" | jq -r .access_token)"
[ -n "${TOKEN}" ] && [ "${TOKEN}" != "null" ] || { echo "No pude obtener el token de la cuenta de servicio de la VM." >&2; exit 1; }

# Misma normalizacion que scripts/prepare_web_env.sh: un secreto creado desde Windows puede traer salto de
# linea o espacio al final, y la base no lo tiene.
PW="$(curl -sf -H "Authorization: Bearer ${TOKEN}" \
  "https://secretmanager.googleapis.com/v1/projects/${PROJECT}/secrets/${SECRET}/versions/latest:access" \
  | jq -r .payload.data | base64 -d | tr -d '\r\n ')"
unset TOKEN
[ -n "${PW}" ] || { echo "El secreto ${SECRET} salio vacio: la cuenta de la VM no puede leerlo (infra/terraform/generator.tf)." >&2; exit 1; }

# El valor viaja por la variable de entorno PGPASSWORD (por nombre: no aparece en la linea de comandos).
PGPASSWORD="${PW}" PGSSLMODE=require docker run --rm -i -e PGPASSWORD -e PGSSLMODE postgres:16-alpine \
  psql -h "${DB_HOST}" -U moocuser -d moocdb -v ON_ERROR_STOP=1 \
  -f - -c "SELECT count(*) AS cuentas_de_carga FROM users WHERE email LIKE 'carga.estudiante%@plataforma-mooc.test'" \
  < "${SQL}"
