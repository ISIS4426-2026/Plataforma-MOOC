#!/usr/bin/env bash
set -e

# ============================================================================
# Plataforma MOOC — Ejecutor y Generador de Evidencia E2E: Identidad y Autoría
# ============================================================================
# Criterios de Aceptación Cumplidos:
#   1. Ejecución sobre el sistema real desplegado en Docker Compose (sin mocks).
#   2. Reporte detallado pass/fail de cada caso de las colecciones de Identidad y Autoría.
#   3. Evidencia guardada (respuestas JSON, emails Mailpit, logs estructurados)
#      lista para los segmentos 1 y 2 de la demo (Sección 10.2).
# ============================================================================

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${PROJECT_ROOT}"

EVIDENCIA_DIR="${PROJECT_ROOT}/docs/e2e/evidencia"
SEG1_DIR="${EVIDENCIA_DIR}/segmento1_identidad_admin"
SEG2_DIR="${EVIDENCIA_DIR}/segmento2_autoria_publicacion"

mkdir -p "${SEG1_DIR}" "${SEG2_DIR}"

echo -e "${BLUE}========================================================================${NC}"
echo -e "${GREEN}${BOLD}  Plataforma MOOC — Pruebas E2E: Identidad y Autoría (#27)             ${NC}"
echo -e "${BLUE}========================================================================${NC}"
echo -e "${CYAN}Verificación Integral sobre Contenedores Docker Compose (Cero Mocks)${NC}\n"

# 1. Verificar estado de la infraestructura en Docker Compose
echo -e "${YELLOW}==> 1. Verificando estado de la infraestructura Docker Compose...${NC}"

if ! command -v docker &> /dev/null; then
    echo -e "${RED}Error: Docker no está instalado o no está en el PATH.${NC}" >&2
    exit 1
fi

REQUIRED_SERVICES=("api" "postgres" "redis" "mailpit" "minio" "worker")
for svc in "${REQUIRED_SERVICES[@]}"; do
    if ! docker compose ps "$svc" 2>/dev/null | grep -qi "up\|healthy\|running"; then
        echo -e "${RED}Error: El servicio '${svc}' de Docker Compose no está activo.${NC}" >&2
        echo -e "${YELLOW}Inicie la infraestructura completa con: make docker-up${NC}" >&2
        exit 1
    fi
    echo -e "  [✔] Servicio Docker '${svc}': ${GREEN}Activo y Saludable${NC}"
done

echo -e "\n${YELLOW}==> 1.1 Restaurando estado determinístico de la base de datos (Seed)...${NC}"
"${PROJECT_ROOT}/scripts/seed.sh" --reset > /dev/null
echo -e "  [✔] Base de datos restaurada con usuarios y cursos sintéticos determinísticos."

flush_redis_ratelimits() {
    docker compose exec -T redis redis-cli EVAL "for _,k in ipairs(redis.call('keys','ratelimit:*')) do redis.call('del',k) end" 0 >/dev/null 2>&1 || true
}

echo -e "\n${YELLOW}==> 2. Ejecutando Colección E2E de Identidad (Postman/Newman en red Compose)...${NC}"
flush_redis_ratelimits

LOG_START_SEG1=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

docker run --rm --network plataforma-mooc_default \
    -v "${PROJECT_ROOT}/docs/postman":/etc/newman \
    -v "${EVIDENCIA_DIR}":/etc/evidence \
    postman/newman:alpine \
    run /etc/newman/collection_api.postman_collection.json \
    -e /etc/newman/mooc_docker.postman_environment.json \
    -r cli,json \
    --reporter-json-export /etc/evidence/test_identity_raw.json

echo -e "${GREEN}  [✔] Colección de Identidad superada exitosamente.${NC}"

echo -e "\n${YELLOW}==> 3. Ejecutando Colección E2E de Administración (Segmento 1 - Postman/Newman)...${NC}"
flush_redis_ratelimits

docker run --rm --network plataforma-mooc_default \
    -v "${PROJECT_ROOT}/docs/postman":/etc/newman \
    -v "${EVIDENCIA_DIR}":/etc/evidence \
    postman/newman:alpine \
    run /etc/newman/collection_admin.postman_collection.json \
    -e /etc/newman/mooc_docker.postman_environment.json \
    -r cli,json \
    --reporter-json-export /etc/evidence/test_admin_raw.json

echo -e "${GREEN}  [✔] Colección de Administración superada exitosamente.${NC}"

# Capturar logs de API para Segmento 1
docker compose logs --since="${LOG_START_SEG1}" api > "${SEG1_DIR}/api_container_segment1.log" 2>/dev/null || true

echo -e "\n${YELLOW}==> 4. Capturando evidencia de correo transaccional en Mailpit (API real)...${NC}"
MAILPIT_MSG_ID=$(curl -s "http://localhost:8025/api/v1/search?query=Confirma&limit=1" | grep -o '"ID":"[^"]*' | head -n1 | cut -d'"' -f4 || true)
if [ -n "$MAILPIT_MSG_ID" ]; then
    curl -s "http://localhost:8025/api/v1/message/${MAILPIT_MSG_ID}" > "${SEG1_DIR}/02_mailpit_verificacion_email.json"
    cp "${SEG1_DIR}/02_mailpit_verificacion_email.json" "${SEG1_DIR}/mailpit_verification_email.json"
    echo -e "${GREEN}  [✔] Correo de verificación capturado desde Mailpit API (ID: ${MAILPIT_MSG_ID}).${NC}"
fi

MAILPIT_RESET_ID=$(curl -s "http://localhost:8025/api/v1/search?query=Restablece&limit=1" | grep -o '"ID":"[^"]*' | head -n1 | cut -d'"' -f4 || true)
if [ -n "$MAILPIT_RESET_ID" ]; then
    curl -s "http://localhost:8025/api/v1/message/${MAILPIT_RESET_ID}" > "${SEG1_DIR}/mailpit_password_reset_email.json"
    echo -e "${GREEN}  [✔] Correo de restablecimiento capturado desde Mailpit API (ID: ${MAILPIT_RESET_ID}).${NC}"
fi

echo -e "\n${YELLOW}==> 5. Ejecutando Colección E2E de Autoría de Cursos (Segmento 2)...${NC}"
flush_redis_ratelimits

LOG_START_SEG2=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

docker run --rm --network plataforma-mooc_default \
    -v "${PROJECT_ROOT}/docs/postman":/etc/newman \
    -v "${EVIDENCIA_DIR}":/etc/evidence \
    postman/newman:alpine \
    run /etc/newman/collection_authoring.postman_collection.json \
    -e /etc/newman/mooc_docker.postman_environment.json \
    -r cli,json \
    --reporter-json-export /etc/evidence/test_authoring_raw.json

echo -e "${GREEN}  [✔] Colección de Autoría de Cursos superada exitosamente.${NC}"

# Capturar logs de API para Segmento 2
docker compose logs --since="${LOG_START_SEG2}" api > "${SEG2_DIR}/api_container_segment2.log" 2>/dev/null || true

echo -e "\n${YELLOW}==> 6. Extrayendo cargas de respuesta y generando reporte Markdown...${NC}"
go run "${PROJECT_ROOT}/scripts/generate_e2e_report.go" "${EVIDENCIA_DIR}"

echo -e "\n${BLUE}========================================================================${NC}"
echo -e "${GREEN}${BOLD}  ✔ PRUEBAS E2E COMPLETADAS CON ÉXITO: 100% PASS (CERO MOCKS)          ${NC}"
echo -e "${BLUE}========================================================================${NC}"
echo -e "${CYAN}Reporte generado:${NC} docs/e2e/REPORTE_E2E_IDENTIDAD_Y_AUTORIA.md"
echo -e "${CYAN}Evidencias Segmento 1:${NC} docs/e2e/evidencia/segmento1_identidad_admin/"
echo -e "${CYAN}Evidencias Segmento 2:${NC} docs/e2e/evidencia/segmento2_autoria_publicacion/"
echo -e "${BLUE}========================================================================${NC}\n"
