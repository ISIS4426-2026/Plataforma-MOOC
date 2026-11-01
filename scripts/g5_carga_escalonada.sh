#!/usr/bin/env bash
# Carga escalonada del escenario 3A: ciclo de escalamiento automatico de la API (G5, issue #225).
#
# SE CORRE EN LA VM DEL GENERADOR (mooc-load-generator, G1), no en el portatil. Un portatil perdio el 20 %
# de las solicitudes en la calibracion de G1 (nota 49) y aqui la tasa es justamente lo que se controla.
#
# QUE HACE
#
# Cinco etapas seguidas, cada una con una TASA fija de peticiones por segundo (no un numero de usuarios):
#
#   etapa  por omision        para que
#   e1     10 pet/s, 5 min    linea base: una instancia, muy por debajo del umbral
#   e2     40 pet/s, 2 min    subida, todavia por debajo de 80 pet/s por instancia
#   e3     70 pet/s, 2 min    subida, cerca del umbral
#   e4    130 pet/s, 15 min   por encima de 80 pet/s con una instancia: el autoescalador tiene que pedir 2.
#                             Con dos, cada una queda en ~65 pet/s, por debajo del umbral: el grupo se
#                             estabiliza en 2 en vez de oscilar
#   e5     10 pet/s, 20 min   vuelve la carga baja; se observa la reduccion. scale_in_control (C2) deja
#                             retirar una instancia cada 600 s, asi que de 2 a 1 tarda al menos 10 min
#
# El umbral sale de C2: load_balancing_utilization.target = 0.8 sobre max_rate_per_instance = 100 (C3),
# es decir, se anade una instancia al pasar de 80 pet/s POR INSTANCIA. Si alguien cambia esos valores,
# cambia aqui RATE_ALTA.
#
# Para llegar a 3 instancias haria falta mas de 160 pet/s: RATE_ALTA=200. No es la opcion por omision
# porque el criterio pide "al menos una" instancia adicional, y C2 extrapola que a ~240 pet/s la base
# (1 vCPU) se acerca a su techo: eso es una medicion distinta, con otro coste.
#
# USO (en la VM del generador, desde la raiz del repositorio)
#
#   BASE_URL=https://<balanceador> ENTORNO=nube-entrega3 \
#   JMETER_IMAGE=<digest de generator.tf> \
#     nohup bash scripts/g5_carga_escalonada.sh > g5_carga.log 2>&1 &
#
#   La corrida dura ~45 min con los valores por omision: nohup para que no muera si se cae el IAP.
#
# Variables: COMMIT (el del paquete; en la VM no hay .git)
#   pet/s y segundos: RATE_BASE (10), RATE_E2 (40), RATE_E3 (70), RATE_ALTA (130),
#   DUR_E1 (300), DUR_E2 (120), DUR_E3 (120), DUR_ALTA (900), DUR_BAJA (1200), RESULTS_ROOT, ETIQUETA
#
# SALIDA: <RESULTS_ROOT>/<etiqueta>_<fecha>/ con resultados.jtl, jmeter.log, generador.csv, ventana.txt,
# etapas.tsv (inicio y fin planificados de cada etapa, en UTC), destino.json y resumen.txt. La misma forma
# que el escenario 1, para que capacity_postproceso.sh y capacity_metricas_gcp.py la procesen igual.
set -euo pipefail

BASE_URL="${BASE_URL:?Declara el destino: BASE_URL=https://<balanceador> (terraform output -raw lb_url)}"
ENTORNO="${ENTORNO:?Declara el entorno: ENTORNO=nube-entrega3}"

RATE_BASE="${RATE_BASE:-10}"
RATE_E2="${RATE_E2:-40}"
RATE_E3="${RATE_E3:-70}"
RATE_ALTA="${RATE_ALTA:-130}"
DUR_E1="${DUR_E1:-300}"
DUR_E2="${DUR_E2:-120}"
DUR_E3="${DUR_E3:-120}"
DUR_ALTA="${DUR_ALTA:-900}"
DUR_BAJA="${DUR_BAJA:-1200}"
RAMPA_S="${RAMPA_S:-10}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PLAN_DIR="${ROOT}/capacity-planning"
ETIQUETA="${ETIQUETA:-escenario3a}"
RUN="${ETIQUETA}_$(date -u +%Y%m%d_%H%M%S)"
RESULTS_ROOT="${RESULTS_ROOT:-${ROOT}/docs/entrega3/evidencias/G5/carga}"
OUT="${RESULTS_ROOT}/${RUN}"
IMAGEN="${JMETER_IMAGE:-justb4/jmeter:latest}"
PY=python3; command -v python3 >/dev/null 2>&1 || PY=python
# En la VM los guiones llegan en un paquete sin .git (ver el runbook), asi que el commit se pasa a mano.
COMMIT="${COMMIT:-$(git -C "${ROOT}" rev-parse --short HEAD 2>/dev/null || echo desconocido)}"

# --- comprobaciones previas: que fallen aqui y no a los 20 minutos -----------------------------------
for v in RATE_BASE RATE_E2 RATE_E3 RATE_ALTA DUR_E1 DUR_E2 DUR_E3 DUR_ALTA DUR_BAJA RAMPA_S; do
    [[ "${!v}" =~ ^[0-9]+$ ]] && [ "${!v}" -gt 0 ] || { echo "ERROR: ${v}=${!v} tiene que ser un entero positivo" >&2; exit 2; }
done
# Con menos de 80 pet/s el grupo no tiene por que crecer; la corrida gastaria 45 min para nada.
[ "${RATE_ALTA}" -gt 80 ] || { echo "ERROR: RATE_ALTA=${RATE_ALTA} no supera el umbral de 80 pet/s por instancia (C2)" >&2; exit 2; }
[ "${IMAGEN}" != "justb4/jmeter:latest" ] || echo "AVISO: JMETER_IMAGE sin fijar; en la VM del generador va el digest de generator.tf (nota 49)" >&2
command -v docker >/dev/null || { echo "ERROR: falta docker (esto se corre en la VM del generador)" >&2; exit 2; }
[ -f "${PLAN_DIR}/escenario3a_api.jmx" ] || { echo "ERROR: no encuentro capacity-planning/escenario3a_api.jmx" >&2; exit 2; }

# El destino responde antes de empezar: si las reglas de reenvio no existen (lb_enabled=false), la URL
# no contesta y JMeter registraria 45 minutos de errores de conexion.
codigo="$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "${BASE_URL}/api/v1/ready" || true)"
[ "${codigo}" = "200" ] || { echo "ERROR: ${BASE_URL}/api/v1/ready responde '${codigo}'. Encendiste con -var lb_enabled=true?" >&2; exit 2; }

PROTO="${BASE_URL%%://*}"; REST="${BASE_URL#*://}"; REST="${REST%%/*}"; HOST="${REST%%:*}"
if [[ "$REST" == *:* ]]; then PORT="${REST##*:}"; else [ "$PROTO" = https ] && PORT=443 || PORT=80; fi

# --- plan de etapas -----------------------------------------------------------------------------------
# Hilos: el techo de concurrencia, no la tasa. Con latencias de 10-120 ms (A7) una peticion ocupa un hilo
# poco tiempo; la mitad de la tasa mas 10 sobra incluso si la base se frena. Si los hilos no alcanzaran,
# la tasa real quedaria por debajo de la pedida, y g5_linea_de_tiempo.py lo detecta y lo dice.
declare -A RATE=([e1]="${RATE_BASE}" [e2]="${RATE_E2}" [e3]="${RATE_E3}" [e4]="${RATE_ALTA}" [e5]="${RATE_BASE}")
declare -A DUR=([e1]="${DUR_E1}" [e2]="${DUR_E2}" [e3]="${DUR_E3}" [e4]="${DUR_ALTA}" [e5]="${DUR_BAJA}")
declare -A NOMBRE=([e1]="base" [e2]="subida-40" [e3]="subida-70" [e4]="alta-sostenida" [e5]="bajada")

mkdir -p "${OUT}"
ARGS=()
INICIO=0
for e in e1 e2 e3 e4 e5; do
    ARGS+=("-Jhilos_${e}=$(( RATE[$e] / 2 + 10 ))" "-Jdur_${e}=${DUR[$e]}" "-Jinicio_${e}=${INICIO}" "-Jppm_${e}=$(( RATE[$e] * 60 ))")
    INICIO=$(( INICIO + DUR[$e] ))
done
TOTAL="${INICIO}"

echo "==> ${RUN}: escenario 3A contra ${BASE_URL}, ${TOTAL} s en total"
for e in e1 e2 e3 e4 e5; do printf '    %s %-15s %4s pet/s  %5s s\n' "$e" "${NOMBRE[$e]}" "${RATE[$e]}" "${DUR[$e]}"; done

# --- salud del generador: CPU y memoria del contenedor cada 10 s (G1, nota 49) ------------------------
CONTENEDOR="jmeter-${RUN}"
(
    echo "hora_utc,cpu_pct,memoria" > "${OUT}/generador.csv"
    while sleep 10; do
        linea="$(docker stats --no-stream --format '{{.CPUPerc}},{{.MemUsage}}' "${CONTENEDOR}" 2>/dev/null || true)"
        [ -n "${linea}" ] && echo "$(date -u +%H:%M:%S),${linea}" >> "${OUT}/generador.csv"
    done
) &
MUESTREO=$!
trap 'kill ${MUESTREO} 2>/dev/null || true' EXIT

T_INICIO="$(date -u +%s)"
date -u -d "@${T_INICIO}" +%Y-%m-%dT%H:%M:%SZ > "${OUT}/ventana.txt"

# Las horas planificadas de cada etapa, ANTES de arrancar: el observador de la otra terminal y el
# analisis posterior las necesitan para separar «cuando subio la carga» de «cuando decidio el servicio».
{
    printf 'etapa\tnombre\tpet_s\tinicio_utc\tfin_utc\n'
    acum="${T_INICIO}"
    for e in e1 e2 e3 e4 e5; do
        fin=$(( acum + DUR[$e] ))
        printf '%s\t%s\t%s\t%s\t%s\n' "$e" "${NOMBRE[$e]}" "${RATE[$e]}" \
            "$(date -u -d "@${acum}" +%Y-%m-%dT%H:%M:%SZ)" "$(date -u -d "@${fin}" +%Y-%m-%dT%H:%M:%SZ)"
        acum="${fin}"
    done
} > "${OUT}/etapas.tsv"
column -t -s $'\t' "${OUT}/etapas.tsv" 2>/dev/null || cat "${OUT}/etapas.tsv"
echo ">>> La etapa alta (e4) empieza a las $(awk -F'\t' '$1=="e4"{print $4}' "${OUT}/etapas.tsv"). Ten el observador corriendo antes. <<<"

docker run --rm --name "${CONTENEDOR}" \
    -v "${PLAN_DIR}:/plan:ro" -v "${OUT}:/salida" \
    "${IMAGEN}" \
    -n -t /plan/escenario3a_api.jmx -l /salida/resultados.jtl -j /salida/jmeter.log \
    -Jhost="${HOST}" -Jport="${PORT}" -Jprotocol="${PROTO}" -Jorigin="${BASE_URL}" -Jrampa_s="${RAMPA_S}" \
    "${ARGS[@]}" \
    | tail -n 15

date -u +%Y-%m-%dT%H:%M:%SZ >> "${OUT}/ventana.txt"
kill "${MUESTREO}" 2>/dev/null || true

cat > "${OUT}/destino.json" <<JSON
{
  "base_url": "${BASE_URL}",
  "entorno": "${ENTORNO}",
  "plan": "escenario3a_api.jmx",
  "etapas_pet_s": {"e1": ${RATE[e1]}, "e2": ${RATE[e2]}, "e3": ${RATE[e3]}, "e4": ${RATE[e4]}, "e5": ${RATE[e5]}},
  "etapas_s": {"e1": ${DUR[e1]}, "e2": ${DUR[e2]}, "e3": ${DUR[e3]}, "e4": ${DUR[e4]}, "e5": ${DUR[e5]}},
  "calentamiento_omitido_s": 0,
  "commit": "${COMMIT}",
  "generador": "${IMAGEN} en $(hostname)",
  "nucleos_generador": $(nproc 2>/dev/null || echo 0),
  "memoria_generador_gib": $(awk '/MemTotal/ {printf "%.1f", $2/1048576}' /proc/meminfo 2>/dev/null || echo 0),
  "inicio_utc": "$(sed -n 1p "${OUT}/ventana.txt")",
  "fin_utc": "$(sed -n 2p "${OUT}/ventana.txt")"
}
JSON

# El resumen comun (A6) sobre toda la corrida; el desglose por etapa y la linea de tiempo los hace
# g5_linea_de_tiempo.py en el portatil, donde estan tambien la observacion y la auditoria.
"$PY" "${SCRIPT_DIR}/capacity_resumen_jtl.py" "${OUT}/resultados.jtl" 0 --exigir-destino --exigir-nube \
    | tee "${OUT}/resumen.txt" || echo "AVISO: capacity_resumen_jtl.py salio con error; revisa resumen.txt" >&2

echo "==> ${OUT}"
echo "    Traelo al portatil con:"
echo "    gcloud compute scp --recurse --zone=us-east1-c --tunnel-through-iap \\"
echo "      mooc-load-generator:$(realpath "${OUT}") docs/entrega3/evidencias/G5/carga/"
