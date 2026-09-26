#!/usr/bin/env bash
set -e

# ============================================================================
# Plataforma MOOC — Gestor de Datos Sintéticos y Semillas (Seed)
# ============================================================================
# Uso:
#   ./scripts/seed.sh [opción]
#
# Opciones:
#   --load     (Por defecto) Carga el dataset sintético determinístico.
#   --clean    Limpia todas las tablas de la base de datos y la caché de Redis.
#   --reset    Ejecuta --clean y luego --load para restaurar el estado inicial.
#   --status   Muestra un resumen del estado actual de datos en la base de datos.
#   --help     Muestra esta ayuda.
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
SEEDS_DIR="${SCRIPT_DIR}/seeds"

SEED_SQL="${SEEDS_DIR}/synthetic_data.sql"
CLEAN_SQL="${SEEDS_DIR}/clean_data.sql"

# Detectar comando de ejecución (Docker Compose vs psql directo)
run_psql() {
    local sql_file="$1"
    if [ ! -f "$sql_file" ]; then
        echo -e "${RED}Error: Archivo SQL no encontrado: ${sql_file}${NC}" >&2
        exit 1
    fi

    if [ -n "$DATABASE_URL" ] && command -v psql &> /dev/null; then
        psql "$DATABASE_URL" -v ON_ERROR_STOP=1 < "$sql_file"
    else
        # Intentar vía Docker Compose
        if docker compose ps postgres 2>/dev/null | grep -q "Up\|healthy\|running"; then
            docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-moocuser}" -d "${POSTGRES_DB:-moocdb}" < "$sql_file"
        elif docker ps --format '{{.Names}}' | grep -q "postgres"; then
            local container_name
            container_name=$(docker ps --format '{{.Names}}' | grep "postgres" | head -n1)
            docker exec -i "$container_name" psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-moocuser}" -d "${POSTGRES_DB:-moocdb}" < "$sql_file"
        else
            echo -e "${RED}Error: No se encontró PostgreSQL activo en Docker Compose ni DATABASE_URL configurado.${NC}" >&2
            echo -e "${YELLOW}Inicie la infraestructura con: make docker-up (o docker compose up -d)${NC}" >&2
            exit 1
        fi
    fi
}

run_psql_query() {
    local query="$1"
    if [ -n "$DATABASE_URL" ] && command -v psql &> /dev/null; then
        psql "$DATABASE_URL" -t -A -c "$query"
    else
        docker compose exec -T postgres psql -U "${POSTGRES_USER:-moocuser}" -d "${POSTGRES_DB:-moocdb}" -t -A -c "$query"
    fi
}

flush_redis() {
    if docker compose ps redis 2>/dev/null | grep -q "Up\|healthy\|running"; then
        docker compose exec -T redis redis-cli FLUSHDB >/dev/null 2>&1 || true
    elif command -v redis-cli &> /dev/null; then
        redis-cli -u "${REDIS_URL:-redis://127.0.0.1:6379}" FLUSHDB >/dev/null 2>&1 || true
    fi
}

show_header() {
    echo -e "${BLUE}========================================================================${NC}"
    echo -e "${GREEN}${BOLD}  Plataforma MOOC — Gestión de Datos Sintéticos Determinísticos         ${NC}"
    echo -e "${BLUE}========================================================================${NC}"
}

cmd_load() {
    show_header
    echo -e "${YELLOW}==> Cargando seed de datos sintéticos (${SEED_SQL})...${NC}"
    run_psql "$SEED_SQL"
    echo -e "${GREEN}[✔] Datos sintéticos cargados exitosamente de forma determinística.${NC}\n"
    cmd_status
}

cmd_clean() {
    show_header
    echo -e "${YELLOW}==> Limpiando tablas de datos sintéticos (${CLEAN_SQL})...${NC}"
    run_psql "$CLEAN_SQL"
    flush_redis
    echo -e "${GREEN}[✔] Tablas de PostgreSQL truncadas y sesiones en Redis limpiadas exitosamente.${NC}\n"
}

cmd_reset() {
    show_header
    echo -e "${YELLOW}==> 1. Limpiando datos existentes...${NC}"
    run_psql "$CLEAN_SQL"
    flush_redis
    echo -e "${GREEN}[✔] Limpieza completada.${NC}\n"
    echo -e "${YELLOW}==> 2. Cargando datos sintéticos determinísticos...${NC}"
    run_psql "$SEED_SQL"
    echo -e "${GREEN}[✔] Estado inicial restaurado exitosamente.${NC}\n"
    cmd_status
}

cmd_status() {
    echo -e "${CYAN}${BOLD}Resumen de Entidades en Base de Datos:${NC}"
    echo -e "${BLUE}------------------------------------------------------------------------${NC}"
    
    local total_users admin_users prof_users student_users
    total_users=$(run_psql_query "SELECT COUNT(*) FROM users;")
    admin_users=$(run_psql_query "SELECT COUNT(*) FROM users WHERE role='administrador';")
    prof_users=$(run_psql_query "SELECT COUNT(*) FROM users WHERE role='profesor';")
    student_users=$(run_psql_query "SELECT COUNT(*) FROM users WHERE role='estudiante';")

    echo -e "  ${BOLD}Usuarios:${NC} Total: ${total_users} (Admins: ${admin_users} | Profesores: ${prof_users} | Estudiantes: ${student_users})"

    local total_courses published_courses draft_courses unpub_courses
    total_courses=$(run_psql_query "SELECT COUNT(*) FROM courses;")
    published_courses=$(run_psql_query "SELECT COUNT(*) FROM courses WHERE status='published';")
    draft_courses=$(run_psql_query "SELECT COUNT(*) FROM courses WHERE status='draft';")
    unpub_courses=$(run_psql_query "SELECT COUNT(*) FROM courses WHERE status='unpublished';")

    echo -e "  ${BOLD}Cursos:${NC}   Total: ${total_courses} (Publicados: ${published_courses} | Borradores: ${draft_courses} | Despublicados: ${unpub_courses})"

    local modules units resources quizzes badges audit_logs enrollments progress events
    modules=$(run_psql_query "SELECT COUNT(*) FROM modules;")
    units=$(run_psql_query "SELECT COUNT(*) FROM units;")
    resources=$(run_psql_query "SELECT COUNT(*) FROM resources;")
    quizzes=$(run_psql_query "SELECT COUNT(*) FROM quizzes;")
    badges=$(run_psql_query "SELECT COUNT(*) FROM badges;")
    audit_logs=$(run_psql_query "SELECT COUNT(*) FROM audit_logs;")
    enrollments=$(run_psql_query "SELECT COUNT(*) FROM enrollments WHERE status = 'active';")
    progress=$(run_psql_query "SELECT COUNT(*) FROM student_progress;")
    events=$(run_psql_query "SELECT COUNT(*) FROM progress_events;")

    echo -e "  ${BOLD}Jerarquía:${NC} Módulos: ${modules} | Unidades: ${units} | Recursos: ${resources}"
    echo -e "  ${BOLD}Evaluación:${NC} Quizzes: ${quizzes} | Insignias Emitidas: ${badges}"
    echo -e "  ${BOLD}Avance:${NC}    Inscripciones activas: ${enrollments} | Estudiantes con progreso: ${progress} | Latidos: ${events}"
    echo -e "  ${BOLD}Auditoría:${NC}  Registros inmutables: ${audit_logs}"
    echo -e "${BLUE}------------------------------------------------------------------------${NC}"
}

# Enrutamiento de argumentos CLI
ACTION="${1:---load}"

case "$ACTION" in
    --load|load)
        cmd_load
        ;;
    --clean|clean)
        cmd_clean
        ;;
    --reset|reset)
        cmd_reset
        ;;
    --status|status)
        show_header
        cmd_status
        ;;
    --help|help|-h)
        echo "Uso: ./scripts/seed.sh [--load | --clean | --reset | --status | --help]"
        echo ""
        echo "  --load    Carga el seed determinístico de datos sintéticos."
        echo "  --clean   Limpia las tablas de la BD relacional y la caché Redis."
        echo "  --reset   Limpia y vuelve a cargar los datos determinísticos."
        echo "  --status  Muestra conteos de usuarios, cursos, jerarquía y auditoría."
        ;;
    *)
        echo -e "${RED}Opción desconocida: ${ACTION}${NC}"
        echo "Ejecute ./scripts/seed.sh --help para consultar las opciones disponibles."
        exit 1
        ;;
esac
