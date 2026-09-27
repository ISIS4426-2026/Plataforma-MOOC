#!/usr/bin/env bash
# Corre el smoke test de JMeter contra el entorno real en la nube (issue #131,
# H1). Un comando, reproducible, generador corriendo fuera de las dos VMs de
# la aplicacion (en la maquina de quien lo ejecuta), tal como exige el
# enunciado y confirmo el docente en el hilo del issue.
#
# Uso:
#   bash ./scripts/run_jmeter_smoke.sh
#
# Requiere Docker. No requiere instalar Java ni JMeter: usa la imagen
# justb4/jmeter, igual que los objetivos de Postman usan postman/newman.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
H1_DIR="${PROJECT_ROOT}/docs/entrega2/evidencias/H1"
RESULTS_DIR="${H1_DIR}/resultados"

mkdir -p "${RESULTS_DIR}"

STAMP="$(date +%Y%m%d_%H%M%S)"
RESULT_FILE="resultados/smoke_test_${STAMP}.jtl"

echo "==> Corriendo smoke_test.jmx contra la nube real (JMeter en Docker)"
MSYS_NO_PATHCONV=1 docker run --rm \
    -v "${H1_DIR}:/jmeter" \
    justb4/jmeter:latest \
    -n -t /jmeter/smoke_test.jmx -l "/jmeter/${RESULT_FILE}"

echo "==> Resultados en: ${RESULTS_DIR}/$(basename "${RESULT_FILE}")"
