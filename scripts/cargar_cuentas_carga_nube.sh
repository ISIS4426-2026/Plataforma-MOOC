#!/usr/bin/env bash
# Aplica un archivo SQL de las cuentas de carga sobre la base administrada de la
# nube (issue #132, H2). Por defecto carga las 200 cuentas
# (scripts/seeds/capacity_data.sql); tambien sirve para reiniciarlas entre
# corridas (capacity_reset.sql) y para verificar el estado tras una corrida
# (capacity_verificar_estado.sql), pasando el archivo como argumento.
#
# La base solo tiene IP privada, asi que la carga se hace DESDE la VM web, por el
# tunel IAP (el mismo acceso que ya usa el equipo). Todo se lanza desde la
# maquina de quien ejecuta, sin abrir una sesion interactiva:
#   1. copia el archivo SQL a la VM,
#   2. la VM lee la contrasena de la base desde Secret Manager con su propia
#      cuenta de servicio (la contrasena no pasa por tu maquina ni se imprime),
#   3. aplica el SQL con un cliente psql en un contenedor y cuenta las cuentas.
#
# Es idempotente: si las cuentas ya existen, las actualiza sin duplicar.
# Solo AGREGA usuarios de carga (ids c9...); no toca ningun otro dato.
#
# Uso (desde la raiz del proyecto, con `gcloud auth login` hecho):
#   bash scripts/cargar_cuentas_carga_nube.sh [archivo.sql]

set -euo pipefail

PROJECT="${PROJECT_ID:-plataforma-mooc-entrega2}"
ZONE="${ZONE:-us-east1-b}"
VM="${VM_NAME:-mooc-web-server}"
DB_IP="${DB_IP:-10.171.240.3}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}/.."
SQL="${1:-scripts/seeds/capacity_data.sql}"
[ -f "${SQL}" ] || { echo "No encuentro ${SQL}: corre este script desde el proyecto."; exit 1; }

# Nota: rutas relativas y sin "/" inicial a proposito. Git Bash convierte los
# argumentos que empiezan por "/" y eso rompe a gcloud en Windows.
OPCIONES=(--zone="${ZONE}" --project="${PROJECT}" --tunnel-through-iap --strict-host-key-checking=no --quiet)

echo "==> 1/2 Copiando ${SQL} a ${VM}"
gcloud compute scp "${OPCIONES[@]}" "${SQL}" "${VM}:capacity_sql.sql"

echo "==> 2/2 Aplicando el SQL desde la VM contra ${DB_IP}"
gcloud compute ssh "${VM}" "${OPCIONES[@]}" --command='
set -e
# Misma normalizacion que scripts/prepare_web_env.sh: un secreto creado desde
# Windows puede traer salto de linea o espacio al final, y la base no lo tiene.
PW="$(gcloud secrets versions access latest --secret=db-password | python3 -c "import sys; sys.stdout.write(sys.stdin.read().strip())")"
sudo docker run --rm -i -e PGPASSWORD="$PW" -e PGSSLMODE=require postgres:16-alpine \
  psql -h '"${DB_IP}"' -U moocuser -d moocdb -v ON_ERROR_STOP=1 \
  -f - -c "SELECT count(*) AS cuentas_de_carga FROM users WHERE email LIKE '"'"'carga.estudiante%@plataforma-mooc.test'"'"'" \
  < ~/capacity_sql.sql
rm -f ~/capacity_sql.sql
'

echo "==> Listo: ${SQL} aplicado."
