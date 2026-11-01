#!/usr/bin/env bash
# Lo que se corre DESPUES de una campana (G1, issue #222): reune las metricas de Cloud Monitoring de cada
# corrida y comprueba que el generador no limito la medicion. Una corrida que sale LIMITADO no se publica.
#
#   bash scripts/capacity_postproceso.sh <directorio_de_resultados>
#
# Necesita `gcloud auth login` (solo lectura de Monitoring y Logging). Es idempotente: no repite una corrida
# que ya tiene metricas_gcp.json.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
RESULTADOS="${1:?uso: bash scripts/capacity_postproceso.sh <directorio_de_resultados>}"
PY=python; command -v python >/dev/null 2>&1 || PY=python3

echo "==> 1/2 metricas de Cloud Monitoring por corrida"
"$PY" scripts/capacity_metricas_gcp.py --todas "$RESULTADOS" || exit 1

echo "==> 2/2 el generador no limita (recursos, latencia contrastada, solicitudes contadas)"
"$PY" scripts/capacity_contraste_generador.py --todas "$RESULTADOS"
estado=$?
if [ "$estado" -ne 0 ]; then
  echo "ATENCION: alguna corrida quedo LIMITADA por el generador: no se publica (criterios de A6/G1)." >&2
fi
exit "$estado"
