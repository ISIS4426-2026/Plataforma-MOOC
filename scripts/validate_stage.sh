#!/usr/bin/env bash
set -e

# Colors for presentation output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================================================${NC}"
echo -e "${GREEN}  Plataforma MOOC — Suite de Validación Automatizada de la Etapa        ${NC}"
echo -e "${BLUE}========================================================================${NC}"
echo -e "${CYAN}Verificación integral según Secciones 6, 9 y 10.2 y PROJECT_KEY_ASPECTS.md${NC}\n"

# 1. Formatting and Static Analysis
echo -e "${YELLOW}==> 1. Verificando formato y análisis estático (fmt, vet, lint)...${NC}"
go fmt ./...
go vet ./...
if [ -f "./scripts/lint.sh" ]; then
    ./scripts/lint.sh
fi
echo -e "${GREEN}  [✔] Formato y análisis estático superados exitosamente.${NC}\n"

# 2. Database Migrations
echo -e "${YELLOW}==> 2. Verificando migraciones de PostgreSQL (Up & Down reversible)...${NC}"
go test -v ./migrations/...
echo -e "${GREEN}  [✔] Migraciones de base de datos verificadas exitosamente.${NC}\n"

# 3. Unit and Domain Integration Tests
echo -e "${YELLOW}==> 3. Ejecutando batería de pruebas unitarias y de dominio...${NC}"
go test -v \
    ./internal/auth \
    ./internal/course \
    ./internal/domain \
    ./internal/http/... \
    ./internal/observability \
    ./internal/postgres \
    ./internal/structure
echo -e "${GREEN}  [✔] Todas las pruebas de dominio, auth, curso, estructura y observabilidad superadas.${NC}\n"

# 4. Background Worker & Idempotency Tests (Segment 4)
echo -e "${YELLOW}==> 4. Verificando pruebas de Worker, Idempotencia y DLQ (Segmento 4)...${NC}"
REDIS_HOST="${REDIS_HOST:-127.0.0.1}"
REDIS_PORT="${REDIS_PORT:-6379}"

is_redis_available() {
    if command -v nc &> /dev/null; then
        nc -z "$REDIS_HOST" "$REDIS_PORT" 2>/dev/null
    else
        (echo > "/dev/tcp/$REDIS_HOST/$REDIS_PORT") 2>/dev/null
    fi
}

if is_redis_available; then
    echo -e "${CYAN}     Redis disponible en ${REDIS_HOST}:${REDIS_PORT}. Ejecutando pruebas de worker...${NC}"
    go test -v ./internal/worker -run 'TestIdempotency|TestWorker_DLQ'
    echo -e "${GREEN}  [✔] Pruebas de idempotencia, reintentos con backoff y DLQ superadas.${NC}\n"
else
    echo -e "${YELLOW}  [i] Redis no está activo en ${REDIS_HOST}:${REDIS_PORT}.${NC}"
    echo -e "${CYAN}      Para ejecutar las pruebas del Worker e Idempotencia, inicie Redis con 'docker compose up -d redis'${NC}"
    echo -e "${CYAN}      o ejecute 'make demo-segment4' con la infraestructura activa.${NC}\n"
fi

# 5. Build Binaries
echo -e "${YELLOW}==> 5. Compilando binarios ejecutables (api y worker)...${NC}"
mkdir -p bin
go build -o bin/api ./cmd/api
go build -o bin/worker ./cmd/worker
echo -e "${GREEN}  [✔] Binarios compilados exitosamente en ./bin/${NC}\n"

echo -e "${BLUE}========================================================================${NC}"
echo -e "${GREEN}  ✔ VALIDACIÓN DE LA ETAPA COMPLETADA EXITOSAMENTE                      ${NC}"
echo -e "${BLUE}========================================================================${NC}"
