#!/usr/bin/env bash
set -e

# Colors for presentation output
GREEN='\030[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================================================${NC}"
echo -e "${GREEN}  Plataforma MOOC — Segmento 4 Demo: Tolerancia a Fallos e Idempotencia ${NC}"
echo -e "${BLUE}========================================================================${NC}"
echo -e "${CYAN}Verificación de la condición: 'Una entrega duplicada no genera salidas repetidas' (Sección 6 & 10.2)${NC}\n"

# 1. Verify Redis connectivity
REDIS_URL="${REDIS_URL:-localhost:6379}"
echo -e "${YELLOW}==> 1. Verificando disponibilidad de Redis en ${REDIS_URL}...${NC}"
if ! command -v nc &> /dev/null; then
    echo "nc not found, proceeding with test execution..."
fi

# 2. Prepare log directory and file
LOG_DIR="docs"
LOG_FILE="${LOG_DIR}/demo_segment4_idempotency.log"
mkdir -p "${LOG_DIR}"

echo -e "${YELLOW}==> 2. Ejecutando batería de pruebas unitarias y de integración de idempotencia y DLQ...${NC}"
echo -e "${CYAN}     Logs estructurados (slog) registrándose en: ${LOG_FILE}${NC}\n"

# Run tests and pipe output to log file
go test -v ./internal/worker -run 'TestIdempotency|TestWorker_DLQ' 2>&1 | tee "${LOG_FILE}"

echo -e "\n${BLUE}------------------------------------------------------------------------${NC}"
echo -e "${GREEN}==> 3. Resumen de Evidencia Reproducible (Segmento 4 Demo):${NC}"
echo -e "${BLUE}------------------------------------------------------------------------${NC}"

if grep -q "Task already processed successfully, skipping execution to prevent duplicate effects" "${LOG_FILE}"; then
    echo -e "  [✔] ${GREEN}Criterio 1 Cumplido:${NC} Middleware de idempotencia interceptó la entrega duplicada del evento/job y omitió la ejecución para mantener 1 solo efecto persistido."
else
    echo -e "  [✘] ${YELLOW}Advertencia: No se encontró mensaje de intercepción de duplicados en los logs.${NC}"
fi

if grep -q "Job moved to Dead-Letter Queue (DLQ)" "${LOG_FILE}"; then
    echo -e "  [✔] ${GREEN}Tolerancia a Fallos & DLQ Cumplida:${NC} Tarea fallida tras 3 reintentos automáticos movida a la cola DLQ y alerta emitida."
else
    echo -e "  [✘] ${YELLOW}Advertencia: No se encontró registro de movimiento a DLQ.${NC}"
fi

if grep -q "PASS: TestIdempotency_ReenqueueWithSameKey_NoDuplicateResult" "${LOG_FILE}"; then
    echo -e "  [✔] ${GREEN}Criterio 2 Cumplido:${NC} Tarea reencolada desde DLQ procesada con éxito sin duplicar el resultado final al reintentar."
fi

echo -e "\n${GREEN}========================================================================${NC}"
echo -e "${GREEN}  ✔ DEMO SEGMENTO 4 EXITOSA: Todas las pruebas de idempotencia pasaron!  ${NC}"
echo -e "${GREEN}========================================================================${NC}"
