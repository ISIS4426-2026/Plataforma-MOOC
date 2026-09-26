#!/usr/bin/env bash
#
# Crea el bucket que guarda el estado de Terraform (issue #115).
#
# Es el unico recurso que Terraform no puede crear por si mismo: necesita un
# sitio donde guardar el estado antes de tener estado. Se ejecuta UNA VEZ, por
# una sola persona del equipo; los demas no tienen que hacer nada.
#
# El script es idempotente: si el bucket ya existe, lo comprueba y sale sin
# tocarlo. Volver a ejecutarlo no rompe nada.
#
#   ./scripts/bootstrap-tfstate.sh
#
set -euo pipefail

PROJECT_ID="${PROJECT_ID:-plataforma-mooc-entrega2}"
REGION="${REGION:-us-east1}"
BUCKET="${BUCKET:-plataforma-mooc-entrega2-tfstate}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${BLUE}==>${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
fail()  { echo -e "${RED}[x]${NC} $*" >&2; exit 1; }

command -v gcloud >/dev/null 2>&1 || fail "gcloud no esta instalado. Ver infra/terraform/README.md"

info "Proyecto: ${PROJECT_ID} | Region: ${REGION} | Bucket: gs://${BUCKET}"

if gcloud storage buckets describe "gs://${BUCKET}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
    ok "El bucket ya existe. No hay nada que hacer."
else
    info "Creando el bucket del estado..."

    # --uniform-bucket-level-access: sin ACL por objeto. El acceso se decide
    #   solo con IAM, que es una superficie menos donde equivocarse.
    # --public-access-prevention: el estado contiene la contrasena de la base
    #   en claro. Que nunca pueda hacerse publico, ni por accidente.
    gcloud storage buckets create "gs://${BUCKET}" \
        --project "${PROJECT_ID}" \
        --location "${REGION}" \
        --uniform-bucket-level-access \
        --public-access-prevention

    ok "Bucket creado."
fi

# El versionado es la red de seguridad del estado: si un apply lo deja
# inconsistente o alguien lo sobrescribe, la version anterior sigue ahi y se
# puede restaurar. Se aplica siempre, tambien sobre un bucket preexistente al
# que se le hubiera olvidado.
info "Asegurando el versionado de objetos..."
gcloud storage buckets update "gs://${BUCKET}" --versioning --project "${PROJECT_ID}"
ok "Versionado activo."

echo
ok "Listo. El backend de Terraform ya tiene donde guardar el estado."
echo
echo "   Siguiente paso, desde infra/terraform/:"
echo "     terraform init"
echo
warn "Este bucket contiene la contrasena de la base en claro dentro del estado."
warn "No lo hagas publico, no descargues el estado al repositorio, y da acceso"
warn "solo a los integrantes del equipo."
