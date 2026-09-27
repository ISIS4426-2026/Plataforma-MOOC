#!/usr/bin/env bash
# Corre las colecciones de Postman/newman contra el entorno real en la nube
# (issue #128, G2). Un comando, reproducible, con reporte exportable por
# colección.
#
# Uso:
#   bash ./scripts/test_postman_cloud.sh
#
# Requiere Docker (usa la imagen oficial postman/newman:alpine, igual que los
# objetivos test-postman-* del Makefile) y salida a internet hacia el origen
# publico declarado en docs/postman/mooc_cloud.postman_environment.json.
#
# La coleccion de identidad (collection_api) NO se incluye en la corrida
# principal: su primer paso (POST /auth/register) depende de que F1 (SMTP en
# la nube) este desplegado, y F1 todavia no esta mergeado a main. Correrla
# ahora produciria un 500 real, no un fallo de la coleccion — documentado en
# docs/entrega2/evidencias/G2/README.md. Se puede correr sola para
# diagnostico con --incluir-identidad.
#
# PAUSA ENTRE COLECCIONES. El limitador de login (RATE_LIMIT_LOGIN_ATTEMPTS)
# esta acotado por IP, no por cuenta (internal/http/middleware/ratelimit.go).
# Seis colecciones corridas sin pausa comparten un mismo cupo de 10
# intentos/minuto desde la IP de quien ejecuta el runner, y una coleccion que
# hace varios logins (Progreso, Autoria) agota el cupo de la siguiente antes
# de que empiece. Es el limitador funcionando como se diseno, no un fallo de
# la coleccion ni del servidor -- la pausa evita que una corrida completa se
# tropiece con su propia carga.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
POSTMAN_DIR="${PROJECT_ROOT}/docs/postman"
REPORTS_DIR="${PROJECT_ROOT}/docs/entrega2/evidencias/G2/reportes"
ENV_FILE="mooc_cloud.postman_environment.json"

mkdir -p "${REPORTS_DIR}"

COLLECTIONS=(
    collection_admin
    collection_authoring
    collection_enrollments
    collection_progress
    collection_quizzes
    collection_media
)

if [[ "${1:-}" == "--incluir-identidad" ]]; then
    COLLECTIONS+=(collection_api)
fi

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}==> Corriendo ${#COLLECTIONS[@]} colecciones contra la nube (${ENV_FILE})${NC}"

PAUSA_SEGUNDOS="${PAUSA_SEGUNDOS:-65}"

# El reporter JSON de newman vuelca cada peticion y respuesta completas,
# cabeceras incluidas -- eso es el password de login en el cuerpo y el token
# Bearer real de una sesion contra produccion en la cabecera Authorization de
# todo lo que viene despues. No se exporta. El reporter CLI (lo que se ve en
# pantalla) es la evidencia: nombres de peticion, metodo, status y las
# aserciones, sin cuerpos ni cabeceras -- el mismo formato que ya usan las
# demas evidencias de newman del repositorio (p. ej. docs/entrega2/evidencias/A5).
run_collection() {
    local name="$1"
    MSYS_NO_PATHCONV=1 docker run --rm \
        -v "${POSTMAN_DIR}:/etc/newman" \
        postman/newman:alpine run "/etc/newman/${name}.postman_collection.json" \
        -e "/etc/newman/${ENV_FILE}" \
        --reporters cli \
        | tee "${REPORTS_DIR}/${name}.txt"
    return "${PIPESTATUS[0]}"
}

FAILED=()
first=1
for name in "${COLLECTIONS[@]}"; do
    if [[ ${first} -eq 0 ]]; then
        echo -e "\n${YELLOW}... esperando ${PAUSA_SEGUNDOS}s para no agotar el cupo de login por IP ...${NC}"
        sleep "${PAUSA_SEGUNDOS}"
    fi
    first=0

    echo -e "\n${YELLOW}--- ${name} ---${NC}"
    if run_collection "${name}"; then
        echo -e "${GREEN}[OK] ${name}${NC}"
    else
        echo -e "${RED}[FALLO] ${name}, reintentando una vez tras la pausa...${NC}"
        sleep "${PAUSA_SEGUNDOS}"
        if run_collection "${name}"; then
            echo -e "${GREEN}[OK] ${name} (en el reintento)${NC}"
        else
            echo -e "${RED}[FALLO] ${name}${NC}"
            FAILED+=("${name}")
        fi
    fi
done

echo -e "\n${YELLOW}==================== Resumen ====================${NC}"
if [[ ${#FAILED[@]} -eq 0 ]]; then
    echo -e "${GREEN}Las ${#COLLECTIONS[@]} colecciones pasaron contra la nube, 0 fallos.${NC}"
    echo "Reportes en: ${REPORTS_DIR}"
    exit 0
else
    echo -e "${RED}Colecciones con fallos: ${FAILED[*]}${NC}"
    echo "Reportes en: ${REPORTS_DIR}"
    exit 1
fi
