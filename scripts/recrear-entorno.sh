#!/usr/bin/env bash
# Reconstruye la base de datos de la entrega, de cero a utilizable (issue #119, C2).
#
# POR QUE EXISTE
#
# El enunciado exige eliminar la instancia administrada al cerrar la entrega, y
# poder recrear el entorno para una sustentacion sincrona. Eso convierte la
# reconstruccion en un procedimiento que tiene que funcionar a la primera y que
# **cualquier integrante** debe poder ejecutar, no en una lista de comandos que
# alguien recuerda a medias.
#
# LAS TRES FASES, Y POR QUE NO SE PUEDEN JUNTAR
#
#   1. Aprovisionar   `terraform apply`. Se ejecuta desde una maquina cualquiera
#                     con las credenciales personales, y **desde main** (sexta
#                     regla de infra/terraform/README.md).
#
#   2. Migrar         Crea el esquema. Solo se puede desde DENTRO de la VPC: la
#                     instancia no tiene IP publica, asi que desde un portatil
#                     no hay a donde conectarse.
#
#   3. Sembrar        Carga los datos sinteticos. Misma restriccion que la 2.
#
# La fase 1 corre fuera y las fases 2 y 3 dentro, y esa frontera es la razon de
# que este script tenga dos caras:
#
#   bash ./scripts/recrear-entorno.sh              # orquesta las tres desde tu maquina
#   bash ./scripts/recrear-entorno.sh --en-la-vpc  # solo 2 y 3; se ejecuta EN la VM
#
# La primera forma crea una VM minima y temporal para ejecutar la segunda, y la
# borra al acabar. Es la misma tecnica que uso C1 para migrar antes de que
# existiera la VM de D2, y sigue siendo necesaria: el host que migra no tiene por
# que ser el que sirve la aplicacion.
#
# QUE NO HACE
#
# No despliega la aplicacion. Eso es D2 y E1, con Docker Compose sobre las VMs.
# Este script deja la base lista para que aquello arranque.

set -uo pipefail

PROJECT_ID="${PROJECT_ID:-plataforma-mooc-entrega2}"
ZONA="${ZONA:-us-east1-b}"
VM_TEMPORAL="mooc-reconstructor-temporal"
SECRETO_BD="db-password"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RAIZ="$(cd "${SCRIPT_DIR}/.." && pwd)"

# --- Cronometro ------------------------------------------------------------
# El issue pide el tiempo de recreacion de extremo a extremo, asi que se mide
# desde dentro en lugar de cronometrarlo a mano por fuera.
declare -a FASES=()
T_INICIO=$(date +%s)

fase() {
    local nombre="$1"
    FASE_ACTUAL="${nombre}"
    FASE_T0=$(date +%s)
    echo
    echo "==================================================================="
    echo " ${nombre}"
    echo " $(date -u '+%H:%M:%S UTC')"
    echo "==================================================================="
}

fin_fase() {
    local dur=$(($(date +%s) - FASE_T0))
    FASES+=("$(printf '%-42s %4d s' "${FASE_ACTUAL}" "${dur}")")
    echo "--- ${FASE_ACTUAL}: ${dur} s ---"
}

resumen() {
    local total=$(($(date +%s) - T_INICIO))
    echo
    echo "==================================================================="
    echo " Tiempos"
    echo "==================================================================="
    local f
    for f in "${FASES[@]}"; do echo "  ${f}"; done
    echo "  ----------------------------------------------------"
    printf '  %-42s %4d s  (%d min)\n' "TOTAL" "${total}" "$((total / 60))"
}

morir() {
    echo "ERROR: $*" >&2
    exit 1
}

# ===========================================================================
# CARA B: lo que se ejecuta DENTRO de la VPC
# ===========================================================================
if [[ "${1:-}" == "--en-la-vpc" ]]; then
    command -v psql >/dev/null 2>&1 || morir "falta psql (apt-get install postgresql-client)"
    [[ -n "${IP_BASE:-}" ]] || morir "falta IP_BASE (la IP privada de la instancia)"

    echo "Host: $(hostname) ($(hostname -I | tr -d ' '))"

    # La contrasena la lee esta maquina con su cuenta de servicio adjunta. Es el
    # mismo camino que usa el despliegue real, y significa que no hay que
    # pasarla por argumento ni por el historial del shell.
    fase "1/2  Leer la contrasena de Secret Manager"
    CONTRASENA="$(gcloud secrets versions access latest --secret="${SECRETO_BD}" 2>/dev/null)"
    [[ -n "${CONTRASENA}" ]] || morir "no se pudo leer ${SECRETO_BD}. Falta secretmanager.secretAccessor para la cuenta de esta VM."
    echo "Leida (${#CONTRASENA} caracteres). No se imprime."
    export DATABASE_URL="postgres://moocuser:${CONTRASENA}@${IP_BASE}:5432/moocdb?sslmode=require"
    unset CONTRASENA
    fin_fase

    fase "2/2  Migrar y sembrar"
    bash "${SCRIPT_DIR}/migrate.sh" || morir "las migraciones fallaron"
    echo
    # seed.sh usa DATABASE_URL cuando esta definida, en lugar de Docker Compose.
    # Llamarlo en vez de traer aqui su SQL es deliberado: cuando G1 (issue #127)
    # amplie la semilla para la nube, este script se beneficia sin cambiar.
    bash "${SCRIPT_DIR}/seed.sh" --load || morir "la siembra fallo"
    fin_fase

    resumen
    exit 0
fi

# ===========================================================================
# CARA A: la orquestacion, desde la maquina de quien reconstruye
# ===========================================================================
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    sed -n '2,45p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    exit 0
fi

command -v gcloud >/dev/null 2>&1 || morir "falta gcloud"
command -v terraform >/dev/null 2>&1 || morir "falta terraform"

SOLO_BASE=0
[[ "${1:-}" == "--solo-base" ]] && SOLO_BASE=1

echo "==================================================================="
echo " Reconstruccion del entorno - issue #119 (C2)"
echo " Proyecto: ${PROJECT_ID}   Zona: ${ZONA}"
echo " Inicio:   $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
echo "==================================================================="

# --- Fase 1: aprovisionar --------------------------------------------------
if [[ ${SOLO_BASE} -eq 0 ]]; then
    fase "1/4  Aprovisionar con Terraform"

    # La contrasena no se inventa: se lee del gestor, que es la unica fuente.
    if [[ -z "${TF_VAR_db_password:-}" ]]; then
        echo "Leyendo la contrasena de Secret Manager para TF_VAR_db_password..."
        TF_VAR_db_password="$(gcloud secrets versions access latest \
            --secret="${SECRETO_BD}" --project="${PROJECT_ID}" 2>/dev/null)"
        [[ -n "${TF_VAR_db_password}" ]] || morir "no se pudo leer ${SECRETO_BD}"
        export TF_VAR_db_password
    fi

    terraform -chdir="${RAIZ}/infra/terraform" init -input=false >/dev/null \
        || morir "terraform init fallo"

    # Se muestra el plan antes de aplicar. Un `to destroy` inesperado aqui
    # significa que este arbol no esta en main o que alguien aplico desde una
    # rama (nota 16 de docs/entrega2/NOTAS_TECNICAS.md).
    terraform -chdir="${RAIZ}/infra/terraform" plan -input=false \
        || morir "terraform plan fallo"

    echo
    read -r -p "Aplicar estos cambios? (escribe si) " respuesta
    [[ "${respuesta}" == "si" ]] || morir "cancelado por el usuario"

    terraform -chdir="${RAIZ}/infra/terraform" apply -input=false -auto-approve \
        || morir "terraform apply fallo"
    fin_fase
fi

IP_BASE="$(terraform -chdir="${RAIZ}/infra/terraform" output -raw db_private_ip 2>/dev/null)"
[[ -n "${IP_BASE}" ]] || morir "no se pudo obtener db_private_ip de las salidas de Terraform"
SA_WEB="$(terraform -chdir="${RAIZ}/infra/terraform" output -raw web_server_service_account 2>/dev/null)"
echo "IP privada de la base: ${IP_BASE}"

# --- Fase 2: la VM temporal ------------------------------------------------
fase "2/4  Crear el host temporal dentro de la VPC"

# Sin IP externa: la salida a internet la da el Cloud NAT de B3 y la entrada,
# el tunel de IAP. Lleva adjunta la cuenta de la API para que la lectura del
# secreto ocurra por el mismo camino que en el despliegue real.
gcloud compute instances create "${VM_TEMPORAL}" \
    --project="${PROJECT_ID}" --zone="${ZONA}" \
    --machine-type=e2-micro --subnet=mooc-subnet --no-address \
    --tags=allow-iap-ssh \
    --service-account="${SA_WEB}" \
    --scopes=https://www.googleapis.com/auth/cloud-platform \
    --image-family=debian-12 --image-project=debian-cloud \
    --labels=proyecto=plataforma-mooc,entrega=2,gestion=manual-temporal \
    || morir "no se pudo crear la VM temporal"

# Que la VM se borre pase lo que pase. Sin esto, un fallo a mitad deja una
# maquina facturando que nadie recuerda.
limpiar() {
    echo
    echo "==> Borrando el host temporal..."
    gcloud compute instances delete "${VM_TEMPORAL}" \
        --project="${PROJECT_ID}" --zone="${ZONA}" --quiet 2>/dev/null \
        && echo "    borrado." || echo "    OJO: no se pudo borrar. Hazlo a mano."
}
trap limpiar EXIT

echo "Esperando a que acepte SSH..."
for intento in $(seq 1 20); do
    if gcloud compute ssh "${VM_TEMPORAL}" --project="${PROJECT_ID}" --zone="${ZONA}" \
        --tunnel-through-iap --quiet --command='true' >/dev/null 2>&1; then
        echo "    lista (intento ${intento})"
        break
    fi
    [[ ${intento} -eq 20 ]] && morir "la VM no acepto SSH"
done
fin_fase

# --- Fase 3: preparar el host y copiar lo necesario ------------------------
fase "3/4  Copiar migraciones y semilla al host"

gcloud compute ssh "${VM_TEMPORAL}" --project="${PROJECT_ID}" --zone="${ZONA}" \
    --tunnel-through-iap --quiet \
    --command='sudo apt-get update -qq >/dev/null 2>&1 && sudo apt-get install -y -qq postgresql-client >/dev/null 2>&1 && mkdir -p /tmp/mooc/scripts && echo listo' \
    || morir "no se pudo preparar la VM"

# Solo lo necesario. Clonar el repositorio obligaria a autenticarse en la VM.
#
# La estructura importa: seed.sh resuelve sus rutas desde su propia ubicacion y
# espera encontrar seeds/ al lado, asi que la jerarquia scripts/seeds tiene que
# reproducirse tal cual.
copiar() {
    gcloud compute scp --tunnel-through-iap --zone="${ZONA}" \
        --project="${PROJECT_ID}" --quiet "$@" >/dev/null
}

copiar --recurse "${RAIZ}/migrations" "${VM_TEMPORAL}:/tmp/mooc/" \
    || morir "fallo al copiar las migraciones"
copiar --recurse "${RAIZ}/scripts/seeds" "${VM_TEMPORAL}:/tmp/mooc/scripts/" \
    || morir "fallo al copiar los datos de la semilla"
copiar "${RAIZ}/scripts/migrate.sh" "${RAIZ}/scripts/seed.sh" "${BASH_SOURCE[0]}" \
    "${VM_TEMPORAL}:/tmp/mooc/scripts/" \
    || morir "fallo al copiar los scripts"
fin_fase

# --- Fase 4: migrar y sembrar, dentro de la VPC ---------------------------
fase "4/4  Migrar y sembrar (ejecutandose dentro de la VPC)"
gcloud compute ssh "${VM_TEMPORAL}" --project="${PROJECT_ID}" --zone="${ZONA}" \
    --tunnel-through-iap --quiet \
    --command="IP_BASE=${IP_BASE} bash /tmp/mooc/scripts/recrear-entorno.sh --en-la-vpc" \
    || morir "la preparacion de la base fallo dentro de la VPC"
fin_fase

resumen

echo
echo "==================================================================="
echo " La base esta lista. Lo que falta para tener la plataforma en pie"
echo " es desplegar la aplicacion sobre las VMs (issues #123 y #125)."
echo "==================================================================="
