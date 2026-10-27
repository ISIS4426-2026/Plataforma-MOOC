#!/usr/bin/env bash
# ==============================================================================
# Plataforma MOOC - Ejecutor Formal de Capacidad: Escenario 2 (Issue H5)
# ==============================================================================
# Ejecuta la suite formal de capacidad para el Escenario 2:
#   - Línea base y 4 niveles crecientes (Nivel 0 al 4)
#   - Instrumentación desacoplada en las 6 etapas del pipeline multimedia
#   - Métricas de cola Asynq (profundidad, antigüedad)
#   - Concurrencia de workers fija en 2 (WORKER_CONCURRENCY=2)
#   - Separación formal de tráfico de control (API) y datos (Storage)
#   - Consumo HLS con el patrón declarado, y descarga a máxima velocidad aparte
#   - Observación de drenaje total de cola y verificación terminal en PostgreSQL
#   - Generación de reportes y evidencias en docs/entrega2/evidencias/H5/
#
# AQUÍ NACIERON LOS CLIPS DE 8 SEGUNDOS (correcciones de A5 y A6)
#
# Este script generaba los tres videos con `ffmpeg ... duration=8`, `=20` y
# `=40`, con esas duraciones escritas a mano, mientras el plan declaraba perfiles
# de 2, 10 y 30 minutos. Nadie comparaba una cosa con la otra, así que la campaña
# midió clips y publicó perfiles. Costó un tercio del criterio del escenario 2.
#
# Ahora la generación se delega en scripts/generar_media_prueba.sh, que genera
# **y verifica** campo por campo contra el perfil declarado, y falla si un
# archivo no es lo que dice. Los tiempos de una corrida con los perfiles reales
# están en la nota técnica 24: se mide en horas, no en minutos.
#
# Uso:
#   ENTORNO=compose-local bash ./scripts/run_capacity_escenario2.sh
#   ENTORNO=nube-entrega3 HLS_BASE=https://storage.googleapis.com/<bucket> \
#     bash ./scripts/run_capacity_escenario2.sh https://<host>
#
# Variables:
#   ENTORNO   etiqueta del entorno medido. Obligatoria: es lo que distingue una
#             corrida en la nube de una contra Docker Compose en la evidencia.
#   HLS_BASE  base de los derivados HLS. Obligatoria contra la nube; en local se
#             usa MinIO.
#   PATRON    cadencia (por omisión, reproducción a 1x) o maxima (sin pausa).
#   MAX_SEGMENTOS  segmentos por estudiante; 0 = todos. Acota el egreso.
#   OUT_DIR   directorio de evidencia. Por omision el de H5, que es de la
#             entrega anterior; A7 lo redirige al suyo.
#   SALTAR_VERIFICACION  cualquier valor salta la verificacion de los perfiles,
#             que necesita ffprobe (Docker en Windows). Solo si ya se
#             verificaron en esta sesion y la prueba esta en la evidencia.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SCRATCH_DIR="${ROOT_DIR}/scratch"
# OUT_DIR permite medir hacia otro directorio sin repetir -out-dir detras de los
# argumentos del guion. A7 lo usa para no escribir sobre la evidencia de H5, que
# es de la entrega anterior.
EVIDENCIAS_DIR="${OUT_DIR:-${ROOT_DIR}/docs/entrega2/evidencias/H5}"

: "${ENTORNO:?Declara el entorno medido, p. ej. ENTORNO=nube-entrega3. Sin esto la evidencia no dice contra qué se midió (A6)}"
PATRON="${PATRON:-cadencia}"
MAX_SEGMENTOS="${MAX_SEGMENTOS:-0}"

mkdir -p "${SCRATCH_DIR}" "${EVIDENCIAS_DIR}" "${ROOT_DIR}/bin"

# 1. Los tres perfiles reales, generados y verificados por el guion de A5.
#    Verificar es la parte que no existía: un archivo que no coincide con su
#    perfil declarado hace fallar esto antes de medir nada.
if [ -n "${SALTAR_VERIFICACION:-}" ]; then
    # La verificación necesita ffprobe, y en Windows eso significa Docker. Si la
    # campaña tiene que correr con Docker cerrado --por memoria, por ejemplo--
    # se salta, pero NO en silencio: el motivo de que exista es que la Entrega 2
    # midió clips de 8 segundos declarados como 2, 10 y 30 minutos.
    echo "==> AVISO: verificación de perfiles SALTADA (SALTAR_VERIFICACION)."
    echo "    Solo es admisible si se verificaron en esta misma sesión y nada ha"
    echo "    escrito en scratch/ desde entonces. La prueba tiene que quedar en la"
    echo "    evidencia; si no está, la campaña no es publicable (A6)."
    for p in corto medio largo; do
        [ -f "${SCRATCH_DIR}/video_${p}.mp4" ] || {
            echo "    FALTA ${SCRATCH_DIR}/video_${p}.mp4; sin los archivos no hay nada que medir." >&2
            exit 1
        }
    done
    ls -l "${SCRATCH_DIR}"/video_*.mp4 | awk '{print "    " $5 " bytes  " $NF}'
else
    echo "==> Verificando los tres perfiles multimedia declarados..."
    bash "${SCRIPT_DIR}/generar_media_prueba.sh" --verificar || {
        echo "    Faltan o no coinciden. Generándolos (el perfil largo tarda unos minutos)..."
        bash "${SCRIPT_DIR}/generar_media_prueba.sh"
    }
fi

# 2. Compilar el binario del ejecutor de capacidad
echo "==> Compilando ejecutable de capacidad Escenario 2..."
CGO_ENABLED=0 go build -o "${ROOT_DIR}/bin/capacity_escenario2" "${ROOT_DIR}/scripts/capacity_escenario2/main.go"

# 3. Determinar modo de ejecución (Docker compose local o Cloud)
BASE_URL="${1:-}"
[ $# -gt 0 ] && shift

if [[ -n "${BASE_URL}" && "${BASE_URL}" == http* ]]; then
    : "${HLS_BASE:?Contra la nube hay que declarar HLS_BASE; adivinarla fue cómo se midieron los derivados de otro entorno}"
    echo "==> Ejecutando pruebas contra endpoint en la nube: ${BASE_URL}"
    "${ROOT_DIR}/bin/capacity_escenario2" \
        -base "${BASE_URL}" \
        -hls-base "${HLS_BASE}" \
        -entorno "${ENTORNO}" \
        -patron-consumo "${PATRON}" \
        -max-segmentos "${MAX_SEGMENTOS}" \
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
            -entorno "${ENTORNO}" \
            -patron-consumo "${PATRON}" \
            -max-segmentos "${MAX_SEGMENTOS}" \
            -redis "redis:6379" \
            -db "postgres://moocuser:moocpassword@postgres:5432/moocdb?sslmode=disable" \
            -out-dir "/workspace/${EVIDENCIAS_DIR#${ROOT_DIR}/}" \
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
