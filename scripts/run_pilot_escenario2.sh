#!/usr/bin/env bash
# ==============================================================================
# Plataforma MOOC - Ejecutor del Piloto Corto Escenario 2 (H4)
# ==============================================================================
# Ejecuta un ciclo multimedia controlado instrumentando cada etapa por separado:
#   1. Autorización y emisión de URL prefirmada (API)
#   2. Transferencia directa al almacenamiento (Data plane)
#   3. Confirmación de carga completa (API 202 Accepted)
#   4. Espera en cola (Redis Asynq pending latency)
#   5. Duración de transcodificación (FFmpeg worker)
#   6. Tiempo total hasta available (HLS master & derivatives)
#   7. Consumo: Cadencia real vs "lo más rápido posible"
#   8. Medición con reproductor real (TTFF y stalls)
#
# Uso:
#   bash ./scripts/run_pilot_escenario2.sh
#   bash ./scripts/run_pilot_escenario2.sh -base https://34.24.52.111.sslip.io
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SCRATCH_DIR="${ROOT_DIR}/scratch"
EVIDENCIAS_DIR="${ROOT_DIR}/docs/entrega2/evidencias/H4"
VIDEO_SAMPLE="${SCRATCH_DIR}/pilot_sample_video.mp4"
REPORT_FILE="${EVIDENCIAS_DIR}/piloto_etapas_instrumentadas.txt"

mkdir -p "${SCRATCH_DIR}" "${EVIDENCIAS_DIR}"

# 1. Asegurar video de prueba sintético (8 segundos, 1280x720)
if [ ! -f "${VIDEO_SAMPLE}" ]; then
    echo "==> Generando video sintético de prueba (8s, 1280x720, 25fps) con ffmpeg..."
    if command -v ffmpeg >/dev/null 2>&1; then
        ffmpeg -hide_banner -loglevel error -y \
            -f lavfi -i "testsrc=size=1280x720:rate=25:duration=8" \
            -f lavfi -i "sine=frequency=440:duration=8" \
            -c:v libx264 -preset veryfast -pix_fmt yuv420p -c:a aac -shortest "${VIDEO_SAMPLE}"
    else
        echo "ERROR: ffmpeg no está instalado en el host para sintetizar el video de prueba."
        exit 1
    fi
fi

# 2. Compilar el binario del piloto
echo "==> Compilando ejecutable del piloto..."
CGO_ENABLED=0 go build -o "${ROOT_DIR}/bin/pilot_escenario2" "${ROOT_DIR}/scripts/pilot_escenario2/main.go"

# 3. Determinar modo de ejecución (Docker compose local o Cloud)
BASE_URL="${1:-}"

if [[ -n "${BASE_URL}" && "${BASE_URL}" == http* ]]; then
    echo "==> Ejecutando piloto contra endpoint explícito: ${BASE_URL}"
    "${ROOT_DIR}/bin/pilot_escenario2" \
        -base "${BASE_URL}" \
        -video "${VIDEO_SAMPLE}" \
        -out "${REPORT_FILE}" \
        "$@"
else
    echo "==> Ejecutando piloto contra el entorno Docker local (red plataforma-mooc_default)..."
    docker run --rm \
        --network plataforma-mooc_default \
        -v "${ROOT_DIR}:/workspace" \
        -w /workspace \
        alpine:latest \
        /workspace/bin/pilot_escenario2 \
            -base "http://api:8080" \
            -video "/workspace/scratch/pilot_sample_video.mp4" \
            -out "/workspace/docs/entrega2/evidencias/H4/piloto_etapas_instrumentadas.txt" \
            "$@"
fi

echo ""
echo "==> Piloto finalizado. Resumen de evidencia:"
cat "${REPORT_FILE}"
