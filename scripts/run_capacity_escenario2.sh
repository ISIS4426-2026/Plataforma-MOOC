#!/usr/bin/env bash
# ==============================================================================
# Plataforma MOOC - Ejecutor Formal de Capacidad: Escenario 2 (Issue H5)
# ==============================================================================
# Ejecuta la suite formal de capacidad para el Escenario 2:
#   - Línea base y 4 niveles crecientes (Nivel 0 al 4)
#   - Instrumentación desacoplada en las 6 etapas del pipeline multimedia
#   - Métricas de cola Asynq (profundidad, antigüedad, tasa de servicio)
#   - Concurrencia de workers fija en 2 (WORKER_CONCURRENCY=2)
#   - Separación formal de tráfico de control (API) y datos (Storage)
#   - Streaming HLS con cadencia real (6.0s pacing) vs ráfaga greedy
#   - Sonda QoE de reproductor real (TTFF y stalls)
#   - Observación de drenaje total de cola y verificación terminal en PostgreSQL
#   - Generación de reportes y evidencias en docs/entrega2/evidencias/H5/
#
# Uso:
#   bash ./scripts/run_capacity_escenario2.sh
#   bash ./scripts/run_capacity_escenario2.sh https://34.24.52.111.sslip.io
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SCRATCH_DIR="${ROOT_DIR}/scratch"
EVIDENCIAS_DIR="${ROOT_DIR}/docs/entrega2/evidencias/H5"

mkdir -p "${SCRATCH_DIR}" "${EVIDENCIAS_DIR}" "${ROOT_DIR}/bin"

# 1. Asegurar la presencia de los tres videos de prueba representativos
echo "==> Verificando videos sintéticos de prueba en ${SCRATCH_DIR}..."

if [ ! -f "${SCRATCH_DIR}/video_corto.mp4" ]; then
    echo "    Generando video_corto.mp4 (8s, 1280x720, 25fps)..."
    ffmpeg -hide_banner -loglevel error -y \
        -f lavfi -i "testsrc=size=1280x720:rate=25:duration=8" \
        -f lavfi -i "sine=frequency=440:duration=8" \
        -c:v libx264 -preset veryfast -pix_fmt yuv420p -c:a aac -shortest "${SCRATCH_DIR}/video_corto.mp4"
fi

if [ ! -f "${SCRATCH_DIR}/video_medio.mp4" ]; then
    echo "    Generando video_medio.mp4 (20s, 1280x720, 25fps)..."
    ffmpeg -hide_banner -loglevel error -y \
        -f lavfi -i "testsrc=size=1280x720:rate=25:duration=20" \
        -f lavfi -i "sine=frequency=440:duration=20" \
        -c:v libx264 -preset veryfast -pix_fmt yuv420p -c:a aac -shortest "${SCRATCH_DIR}/video_medio.mp4"
fi

if [ ! -f "${SCRATCH_DIR}/video_largo.mp4" ]; then
    echo "    Generando video_largo.mp4 (40s, 1280x720, 25fps)..."
    ffmpeg -hide_banner -loglevel error -y \
        -f lavfi -i "testsrc=size=1280x720:rate=25:duration=40" \
        -f lavfi -i "sine=frequency=440:duration=40" \
        -c:v libx264 -preset veryfast -pix_fmt yuv420p -c:a aac -shortest "${SCRATCH_DIR}/video_largo.mp4"
fi

echo "    Videos disponibles:"
ls -lh "${SCRATCH_DIR}"/video_*.mp4

# 2. Compilar el binario del ejecutor de capacidad
echo "==> Compilando ejecutable de capacidad Escenario 2..."
CGO_ENABLED=0 go build -o "${ROOT_DIR}/bin/capacity_escenario2" "${ROOT_DIR}/scripts/capacity_escenario2/main.go"

# 3. Determinar modo de ejecución (Docker compose local o Cloud)
BASE_URL="${1:-}"

if [[ -n "${BASE_URL}" && "${BASE_URL}" == http* ]]; then
    echo "==> Ejecutando pruebas contra endpoint en la nube: ${BASE_URL}"
    "${ROOT_DIR}/bin/capacity_escenario2" \
        -base "${BASE_URL}" \
        -out-dir "${EVIDENCIAS_DIR}" \
        -video-corto "${SCRATCH_DIR}/video_corto.mp4" \
        -video-medio "${SCRATCH_DIR}/video_medio.mp4" \
        -video-largo "${SCRATCH_DIR}/video_largo.mp4" \
        "$@"
else
    echo "==> Ejecutando pruebas contra el entorno Docker local (red plataforma-mooc_default)..."
    docker run --rm \
        --network plataforma-mooc_default \
        -v "${ROOT_DIR}:/workspace" \
        -w /workspace \
        alpine:latest \
        /workspace/bin/capacity_escenario2 \
            -base "http://api:8080" \
            -redis "redis:6379" \
            -db "postgres://moocuser:moocpassword@postgres:5432/moocdb?sslmode=disable" \
            -out-dir "/workspace/docs/entrega2/evidencias/H5" \
            -video-corto "/workspace/scratch/video_corto.mp4" \
            -video-medio "/workspace/scratch/video_medio.mp4" \
            -video-largo "/workspace/scratch/video_largo.mp4" \
            "$@"
fi

echo ""
echo "================================================================================"
echo " Ejecución de Escenario 2 completada exitosamente."
echo " Evidencias generadas en: ${EVIDENCIAS_DIR}"
echo "================================================================================"
ls -lh "${EVIDENCIAS_DIR}"
