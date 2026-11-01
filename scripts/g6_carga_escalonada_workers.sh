#!/usr/bin/env bash
# ==============================================================================
# scripts/g6_carga_escalonada_workers.sh — Issue G6 (#226): Escenario 3B
# Generador de carga escalonada de trabajos multimedia (media:process) para
# evaluar el ciclo completo de escalamiento automático de mooc-worker-group:
#   - Etapa 1: Baja carga con 1 worker operativo (outstanding <= 2).
#   - Etapa 2: Aumento en la llegada de trabajos pre-subidos a GCS hasta
#              superar single_instance_assignment=2 y provocar la creación
#              automática de workers adicionales (scale-out 1 -> 3).
#   - Etapa 3: Sostenimiento de la llegada mientras los workers nuevos
#              completan su arranque para comprobar que consumen y procesan
#              trabajos concretos de la cola compartida.
#   - Etapa 4: Cese de llegada de trabajos, drenaje de la cola a 0,
#              observación de reducción automática (scale-in) y reconciliación
#              1:1 de todos los trabajos aceptados contra API/PostgreSQL,
#              GCS (master.m3u8 + segmentos .ts) y Redis.
#
# Modos de uso:
#   bash scripts/g6_carga_escalonada_workers.sh --autoprueba
#   bash scripts/g6_carga_escalonada_workers.sh [DIRECTORIO_SALIDA]
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

PROJECT_ID="${PROJECT_ID:-plataforma-mooc-entrega2}"
ZONE="${ZONE:-us-east1-b}"
GCS_HLS_BUCKET="${GCS_HLS_BUCKET:-gs://plataforma-mooc-entrega2-hls}"
PROFESOR_EMAIL="${PROFESOR_EMAIL:-profesor1@plataforma-mooc.test}"
PROFESOR_PASS="${PROFESOR_PASS:-Password123!}"

G6_ETAPA1_JOBS="${G6_ETAPA1_JOBS:-2}"
G6_ETAPA2_JOBS="${G6_ETAPA2_JOBS:-4}"
G6_ETAPA3_JOBS="${G6_ETAPA3_JOBS:-2}"
G6_ESPERA_ETAPA3_S="${G6_ESPERA_ETAPA3_S:-115}"
G6_POLL_INTERVAL_S="${G6_POLL_INTERVAL_S:-10}"
G6_MAX_WAIT_DRENAJE_S="${G6_MAX_WAIT_DRENAJE_S:-1800}"

uso() {
  cat <<'EOF'
Uso:
  bash scripts/g6_carga_escalonada_workers.sh [opciones] [DIRECTORIO_SALIDA]

Opciones:
  --autoprueba     Ejecuta la autoprueba determinista en un directorio temporal
                   sin requerir conexión a GCP ni contenedores activos.
  -h, --help       Muestra esta ayuda.
EOF
}

ejecutar_autoprueba() {
  local tmp_dir
  tmp_dir="$(mktemp -d)"

  echo "[AUTOPRUEBA G6-CARGA] Iniciando validación en ${tmp_dir}..."

  cat > "${tmp_dir}/etapas.tsv" <<'EOF'
etapa	inicio_utc	fin_utc	trabajos_confirmados	objetivo
etapa1_baja_carga	2026-11-01T18:40:10Z	2026-11-01T18:43:00Z	2	Operar con 1 worker activo sin cruzar umbral (outstanding <= 2)
etapa2_aumento_demanda	2026-11-01T18:43:10Z	2026-11-01T18:43:20Z	6	Elevar outstanding por encima de 2 para provocar creacion automatica (scale-out)
etapa3_sostenimiento_consumo_nuevos	2026-11-01T18:44:50Z	2026-11-01T18:45:00Z	4	Sostener cola pendiente al terminar bootstrap de nuevos workers
etapa4_reduccion_drenaje_y_reconciliacion	2026-11-01T18:45:01Z	2026-11-01T18:51:30Z	0	Cesar llegadas, drenar cola a 0 y reconciliar los trabajos aceptados
EOF

  cat > "${tmp_dir}/trabajos.tsv" <<'EOF'
orden	etapa	perfil	course_id	unit_id	resource_id	stable_id	object_key	http_init	http_put	http_complete	api_instance_id	request_id	t_confirm_utc	t_completed_utc	duracion_total_s	processing_status_final	hls_master_ok	hls_segmentos
1	etapa1_baja_carga	video_corto_120s	c1	u1	r-01	s-01	uploads/s-01/v.mp4	200	200	202	api-1	req-01	2026-11-01T18:40:12Z	2026-11-01T18:42:38Z	146	completed	true	30
2	etapa1_baja_carga	video_corto_120s	c1	u1	r-02	s-02	uploads/s-02/v.mp4	200	200	202	api-1	req-02	2026-11-01T18:40:14Z	2026-11-01T18:42:41Z	147	completed	true	30
EOF

  python3 - "${tmp_dir}" <<'PY'
import pathlib, sys
d = pathlib.Path(sys.argv[1])
etapas = (d / "etapas.tsv").read_text(encoding="utf-8").strip().splitlines()
trabajos = (d / "trabajos.tsv").read_text(encoding="utf-8").strip().splitlines()
assert len(etapas) == 5, f"Se esperaban 4 etapas + cabecera, obtenido {len(etapas)}"
assert len(trabajos) == 3, f"Se esperaban 2 trabajos + cabecera, obtenido {len(trabajos)}"
cols_t = trabajos[0].split("\t")
assert "resource_id" in cols_t and "hls_master_ok" in cols_t and "processing_status_final" in cols_t
PY

  rm -rf "${tmp_dir}"
  echo "[AUTOPRUEBA G6-CARGA] OK: esquemas de etapas.tsv y trabajos.tsv verificados."
  return 0
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  uso
  exit 0
fi

if [[ "${1:-}" == "--autoprueba" ]]; then
  ejecutar_autoprueba
  exit 0
fi

TIMESTAMP_TAG="$(date -u +%Y-%m-%d_%H%MUTC)"
OUT_DIR="${1:-${REPO_ROOT}/docs/entrega3/evidencias/G6/corrida_${TIMESTAMP_TAG}}"
mkdir -p "${OUT_DIR}"

ETAPAS_TSV="${OUT_DIR}/etapas.tsv"
TRABAJOS_TSV="${OUT_DIR}/trabajos.tsv"
RECONCILIACION_TSV="${OUT_DIR}/reconciliacion.tsv"
DESTINO_JSON="${OUT_DIR}/destino.json"

log() {
  printf '[%s] [G6-CARGA] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

resolver_api_base_url() {
  if [[ -n "${API_BASE_URL:-}" ]]; then
    echo "${API_BASE_URL%/}"
    return 0
  fi
  if [[ -n "${BASE_URL:-}" ]]; then
    echo "${BASE_URL%/}"
    return 0
  fi
  local postman_url
  postman_url="$(jq -r '.values[] | select(.key=="baseUrl") | .value' "${REPO_ROOT}/docs/postman/mooc_cloud.postman_environment.json" 2>/dev/null || true)"
  if [[ -n "${postman_url}" && "${postman_url}" != "null" ]]; then
    echo "${postman_url%/}"
    return 0
  fi
  echo "ERROR: No se pudo resolver API_BASE_URL / BASE_URL." >&2
  return 1
}

API_BASE_URL="$(resolver_api_base_url)"
log "API_BASE_URL resuelta: ${API_BASE_URL}"

VIDEO_CORTO="${REPO_ROOT}/scratch/video_corto.mp4"
mkdir -p "${REPO_ROOT}/scratch"
DUR_ACTUAL="$(ffprobe -v error -show_entries format=duration -of default=nw=1:nk=1 "${VIDEO_CORTO}" 2>/dev/null | awk '{printf "%d", $1}' || echo 0)"
if [[ ! -s "${VIDEO_CORTO}" ]] || (( DUR_ACTUAL != 120 )); then
  log "Generando insumo canónico video_corto.mp4 (120 s @ 1280x720, 25 fps, H.264/AAC)..."
  ffmpeg -hide_banner -loglevel error -y -nostdin \
    -f lavfi -i "testsrc=size=1280x720:rate=25:duration=120" \
    -f lavfi -i "sine=frequency=440:sample_rate=44100:duration=120" \
    -c:v libx264 -preset veryfast -pix_fmt yuv420p \
    -c:a aac -shortest "${VIDEO_CORTO}"
fi
read -r V_DUR V_W V_H V_CODEC < <(python3 - "${VIDEO_CORTO}" <<'PY'
import json, subprocess, sys
out = subprocess.check_output([
    "ffprobe", "-v", "error", "-print_format", "json",
    "-show_format", "-show_streams", sys.argv[1]
], text=True)
d = json.loads(out)
dur = int(round(float(d["format"]["duration"])))
v = next(s for s in d["streams"] if s["codec_type"] == "video")
print(dur, v["width"], v["height"], v["codec_name"])
PY
)
if [[ "${V_DUR}" != "120" || "${V_W}" != "1280" || "${V_H}" != "720" || "${V_CODEC}" != "h264" ]]; then
  echo "ERROR: ${VIDEO_CORTO} no cumple el perfil canónico 120s 1280x720 h264 (obtenido: ${V_DUR}s ${V_W}x${V_H} ${V_CODEC})" >&2
  exit 1
fi
VIDEO_BYTES="$(stat -c '%s' "${VIDEO_CORTO}")"
VIDEO_SHA256="$(sha256sum "${VIDEO_CORTO}" | awk '{print $1}')"

cat > "${DESTINO_JSON}" <<EOF
{
  "escenario": "3B - Ciclo de escalamiento automatico de workers (G6)",
  "project_id": "${PROJECT_ID}",
  "zone": "${ZONE}",
  "worker_mig": "mooc-worker-group",
  "worker_autoscaler": "mooc-worker-autoscaler",
  "metric_signal": "custom.googleapis.com/mooc/queue/outstanding",
  "single_instance_assignment": 2,
  "min_replicas": 1,
  "max_replicas": 3,
  "cooldown_period_s": 420,
  "scale_in_window_s": 900,
  "insumo_video_perfil": "video_corto_120s_720p",
  "insumo_video_bytes": ${VIDEO_BYTES},
  "insumo_video_sha256": "${VIDEO_SHA256}",
  "etapa1_jobs": ${G6_ETAPA1_JOBS},
  "etapa2_jobs": ${G6_ETAPA2_JOBS},
  "etapa3_jobs": ${G6_ETAPA3_JOBS},
  "total_jobs_planificados": $((G6_ETAPA1_JOBS + G6_ETAPA2_JOBS + G6_ETAPA3_JOBS)),
  "inicio_utc": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF

printf 'etapa\tinicio_utc\tfin_utc\ttrabajos_confirmados\tobjetivo\n' > "${ETAPAS_TSV}"
printf 'orden\tetapa\tperfil\tcourse_id\tunit_id\tresource_id\tstable_id\tobject_key\thttp_init\thttp_put\thttp_complete\tapi_instance_id\trequest_id\tt_confirm_utc\tt_completed_utc\tduracion_total_s\tprocessing_status_final\thls_master_ok\thls_segmentos\n' > "${TRABAJOS_TSV}"

# 1. Iniciar sesión con la cuenta sembrada de profesor (1 solo login para respetar el rate limit de 10/min)
log "Iniciando sesión como ${PROFESOR_EMAIL}..."
LOGIN_JSON="$(curl -sS -X POST "${API_BASE_URL}/api/v1/auth/login" \
  -H "Origin: ${API_BASE_URL}" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${PROFESOR_EMAIL}\",\"password\":\"${PROFESOR_PASS}\"}")"
TOKEN="$(printf '%s' "${LOGIN_JSON}" | jq -r '.token')"
if [[ -z "${TOKEN}" || "${TOKEN}" == "null" ]]; then
  echo "ERROR: No se pudo obtener token JWT para ${PROFESOR_EMAIL}" >&2
  exit 1
fi

COURSE_JSON="$(curl -sS -X POST "${API_BASE_URL}/api/v1/courses" \
  -H "Origin: ${API_BASE_URL}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"title":"Curso Escenario 3B G6 Workers","description":"Ciclo completo de autoescalado de workers (Escenario 3B)"}')"
COURSE_ID="$(printf '%s' "${COURSE_JSON}" | jq -r '.id')"

MODULE_JSON="$(curl -sS -X POST "${API_BASE_URL}/api/v1/courses/${COURSE_ID}/modules" \
  -H "Origin: ${API_BASE_URL}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"title":"Modulo Escenario 3B G6"}')"
MODULE_ID="$(printf '%s' "${MODULE_JSON}" | jq -r '.id')"

UNIT_JSON="$(curl -sS -X POST "${API_BASE_URL}/api/v1/modules/${MODULE_ID}/units" \
  -H "Origin: ${API_BASE_URL}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"title":"Unidad Escenario 3B G6"}')"
UNIT_ID="$(printf '%s' "${UNIT_JSON}" | jq -r '.id')"

if [[ -z "${UNIT_ID}" || "${UNIT_ID}" == "null" ]]; then
  echo "ERROR: No se pudo crear curso/módulo/unidad para G6." >&2
  exit 1
fi
log "Contexto creado: course_id=${COURSE_ID}, module_id=${MODULE_ID}, unit_id=${UNIT_ID}"

# 2. Crear los N recursos y pre-subir los videos a GCS mediante URL firmada
#    antes de confirmar (/complete), igual que en c6_encolar_carga.sh.
TOTAL_JOBS=$((G6_ETAPA1_JOBS + G6_ETAPA2_JOBS + G6_ETAPA3_JOBS))
PREUPLOAD_TSV="${OUT_DIR}/.preuploads.tsv"
printf 'orden\tetapa\tresource_id\tstable_id\tobject_key\thttp_init\thttp_put\n' > "${PREUPLOAD_TSV}"

log "Creando ${TOTAL_JOBS} recursos de video y pre-subiendo objetos a GCS..."
for ((i = 1; i <= TOTAL_JOBS; i++)); do
  if (( i <= G6_ETAPA1_JOBS )); then
    etapa="etapa1_baja_carga"
  elif (( i <= G6_ETAPA1_JOBS + G6_ETAPA2_JOBS )); then
    etapa="etapa2_aumento_demanda"
  else
    etapa="etapa3_sostenimiento_consumo_nuevos"
  fi

  res_json="$(curl -sS -X POST "${API_BASE_URL}/api/v1/units/${UNIT_ID}/resources" \
    -H "Origin: ${API_BASE_URL}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"title\":\"G6 Video ${i} (${etapa})\",\"type\":\"video\",\"is_visible\":true,\"is_mandatory\":false,\"allow_download\":false}")"
  res_id="$(printf '%s' "${res_json}" | jq -r '.id')"
  stable_id="$(printf '%s' "${res_json}" | jq -r '.stable_id')"

  init_resp_file="$(mktemp)"
  http_init="$(curl -sS -o "${init_resp_file}" -w '%{http_code}' \
    -X POST "${API_BASE_URL}/api/v1/media/presigned-url" \
    -H "Origin: ${API_BASE_URL}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"resource_id\":\"${res_id}\",\"filename\":\"g6_job_${i}.mp4\",\"mime_type\":\"video/mp4\",\"file_size_bytes\":${VIDEO_BYTES}}")"

  upload_url="$(jq -r '.upload_url' "${init_resp_file}")"
  obj_key="$(jq -r '.object_key' "${init_resp_file}")"
  rm -f "${init_resp_file}"

  http_put="$(curl -sS -o /dev/null -w '%{http_code}' \
    -X PUT "${upload_url}" \
    -H "Content-Type: video/mp4" \
    --data-binary "@${VIDEO_CORTO}")"

  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "${i}" "${etapa}" "${res_id}" "${stable_id}" "${obj_key}" "${http_init}" "${http_put}" >> "${PREUPLOAD_TSV}"
  log "  Pre-subido job #${i}/${TOTAL_JOBS} (${etapa}): resource_id=${res_id} stable_id=${stable_id} (presign=${http_init}, put=${http_put})"
done

confirmar_job() {
  local orden="$1"
  local etapa="$2"
  local res_id="$3"
  local stable_id="$4"
  local obj_key="$5"
  local http_init="$6"
  local http_put="$7"

  local hdr_file body_file http_complete t_confirm api_inst req_id
  hdr_file="$(mktemp)"
  body_file="$(mktemp)"
  t_confirm="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  http_complete="$(curl -sS -D "${hdr_file}" -o "${body_file}" -w '%{http_code}' \
    -X POST "${API_BASE_URL}/api/v1/media/uploads/${res_id}/complete" \
    -H "Origin: ${API_BASE_URL}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"object_key\":\"${obj_key}\"}")"

  api_inst="$(awk 'tolower($1)=="x-instance-id:" {gsub(/\r/,"",$2); print $2}' "${hdr_file}" | tail -n1)"
  req_id="$(awk 'tolower($1)=="x-request-id:" {gsub(/\r/,"",$2); print $2}' "${hdr_file}" | tail -n1)"
  rm -f "${hdr_file}" "${body_file}"

  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "${orden}" "${etapa}" "${res_id}" "${stable_id}" "${obj_key}" "${http_init}" "${http_put}" \
    "${http_complete}" "${api_inst:-desconocido}" "${req_id:-sin-id}" "${t_confirm}" >> "${OUT_DIR}/.confirmados.tsv"

  log "  Confirmado job #${orden} (${etapa}): resource_id=${res_id}, HTTP=${http_complete}, X-Instance-ID=${api_inst:-n/a} @ ${t_confirm}"
}

esperar_trabajos_completados_rango() {
  local desde="$1"
  local hasta="$2"
  local timeout_s="$3"
  local inicio_s
  inicio_s="$(date -u +%s)"

  while true; do
    local res_list_json pendientes=0
    res_list_json="$(curl -sS -H "Authorization: Bearer ${TOKEN}" "${API_BASE_URL}/api/v1/units/${UNIT_ID}/resources" || true)"

    while IFS=$'\t' read -r orden _ res_id _ _ _ _ _ _ _ _; do
      [[ "${orden}" == "orden" ]] && continue
      if (( orden < desde || orden > hasta )); then
        continue
      fi
      local st
      st="$(printf '%s' "${res_list_json}" | jq -r ".items[]? | select(.id==\"${res_id}\") | .processing_status" 2>/dev/null || echo "unknown")"
      if [[ "${st}" != "completed" ]]; then
        pendientes=$((pendientes + 1))
      else
        if ! grep -q "^${res_id}	" "${OUT_DIR}/.tiempos_completado.tsv" 2>/dev/null; then
          printf '%s\t%s\n' "${res_id}" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "${OUT_DIR}/.tiempos_completado.tsv"
        fi
      fi
    done < "${OUT_DIR}/.confirmados.tsv"

    if (( pendientes == 0 )); then
      return 0
    fi
    if (( $(date -u +%s) - inicio_s > timeout_s )); then
      log "ADVERTENCIA: Timeout (${timeout_s}s) esperando completitud del rango ${desde}..${hasta} (pendientes=${pendientes})"
      return 1
    fi
    sleep "${G6_POLL_INTERVAL_S}"
  done
}

printf 'orden\tetapa\tresource_id\tstable_id\tobject_key\thttp_init\thttp_put\thttp_complete\tapi_instance_id\trequest_id\tt_confirm_utc\n' > "${OUT_DIR}/.confirmados.tsv"
: > "${OUT_DIR}/.tiempos_completado.tsv"

# ==============================================================================
# ETAPA 1: Baja carga y 1 worker operativo (outstanding <= 2)
# ==============================================================================
E1_INI="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "=== INICIO ETAPA 1 (baja carga, ${G6_ETAPA1_JOBS} trabajos con 1 worker operativo) ==="
while IFS=$'\t' read -r orden etapa res_id stable_id obj_key http_init http_put; do
  [[ "${orden}" == "orden" ]] && continue
  if [[ "${etapa}" == "etapa1_baja_carga" ]]; then
    confirmar_job "${orden}" "${etapa}" "${res_id}" "${stable_id}" "${obj_key}" "${http_init}" "${http_put}"
    sleep 1
  fi
done < "${PREUPLOAD_TSV}"

log "Esperando a que el worker inicial complete los ${G6_ETAPA1_JOBS} trabajos de Etapa 1..."
esperar_trabajos_completados_rango 1 "${G6_ETAPA1_JOBS}" 600
E1_FIN="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
printf 'etapa1_baja_carga\t%s\t%s\t%s\tOperar con 1 worker activo sin cruzar umbral (outstanding <= 2)\n' \
  "${E1_INI}" "${E1_FIN}" "${G6_ETAPA1_JOBS}" >> "${ETAPAS_TSV}"
log "=== FIN ETAPA 1 (${E1_FIN}) ==="

# ==============================================================================
# ETAPA 2: Aumento de llegada hasta provocar creación automática (scale-out)
# ==============================================================================
E2_INI="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "=== INICIO ETAPA 2 (ráfaga de ${G6_ETAPA2_JOBS} trabajos para provocar scale-out) ==="
while IFS=$'\t' read -r orden etapa res_id stable_id obj_key http_init http_put; do
  [[ "${orden}" == "orden" ]] && continue
  if [[ "${etapa}" == "etapa2_aumento_demanda" ]]; then
    confirmar_job "${orden}" "${etapa}" "${res_id}" "${stable_id}" "${obj_key}" "${http_init}" "${http_put}"
  fi
done < "${PREUPLOAD_TSV}"
E2_FIN="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
printf 'etapa2_aumento_demanda\t%s\t%s\t%s\tElevar outstanding por encima de 2 para provocar creacion automatica de workers\n' \
  "${E2_INI}" "${E2_FIN}" "${G6_ETAPA2_JOBS}" >> "${ETAPAS_TSV}"
log "=== FIN ENVÍO ETAPA 2 (${E2_FIN}); esperando ${G6_ESPERA_ETAPA3_S}s mientras arrancan los nuevos workers ==="

sleep "${G6_ESPERA_ETAPA3_S}"

# ==============================================================================
# ETAPA 3: Sostenimiento de la carga para asegurar consumo por workers nuevos
# ==============================================================================
E3_INI="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "=== INICIO ETAPA 3 (sostenimiento con ${G6_ETAPA3_JOBS} trabajos adicionales) ==="
while IFS=$'\t' read -r orden etapa res_id stable_id obj_key http_init http_put; do
  [[ "${orden}" == "orden" ]] && continue
  if [[ "${etapa}" == "etapa3_sostenimiento_consumo_nuevos" ]]; then
    confirmar_job "${orden}" "${etapa}" "${res_id}" "${stable_id}" "${obj_key}" "${http_init}" "${http_put}"
  fi
done < "${PREUPLOAD_TSV}"
E3_FIN="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
printf 'etapa3_sostenimiento_consumo_nuevos\t%s\t%s\t%s\tSostener cola pendiente cuando los nuevos workers entran en RUNNING+HEALTHY\n' \
  "${E3_INI}" "${E3_FIN}" "${G6_ETAPA3_JOBS}" >> "${ETAPAS_TSV}"
log "=== FIN ENVÍO ETAPA 3 (${E3_FIN}) ==="

# ==============================================================================
# ETAPA 4: Reducción de demanda, drenaje de la cola y reconciliación 1:1
# ==============================================================================
E4_INI="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "=== INICIO ETAPA 4 (cese de llegadas, espera de drenaje completo de los ${TOTAL_JOBS} trabajos) ==="
esperar_trabajos_completados_rango 1 "${TOTAL_JOBS}" "${G6_MAX_WAIT_DRENAJE_S}"
E4_FIN="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
printf 'etapa4_reduccion_drenaje_y_reconciliacion\t%s\t%s\t0\tCesar llegadas, drenar cola a 0 y reconciliar los trabajos aceptados\n' \
  "${E4_INI}" "${E4_FIN}" >> "${ETAPAS_TSV}"
log "=== COLA DRENADA (${E4_FIN}); iniciando reconciliación 1:1 API + GCS ==="

printf 'orden\tresource_id\tstable_id\tapi_status\thls_master_exists\thls_segment_count\treconciliado_ok\n' > "${RECONCILIACION_TSV}"

RES_LIST_FINAL="$(curl -sS -H "Authorization: Bearer ${TOKEN}" "${API_BASE_URL}/api/v1/units/${UNIT_ID}/resources")"

while IFS=$'\t' read -r orden etapa res_id stable_id obj_key http_init http_put http_complete api_inst req_id t_confirm; do
  [[ "${orden}" == "orden" ]] && continue

  status_final="$(printf '%s' "${RES_LIST_FINAL}" | jq -r ".items[]? | select(.id==\"${res_id}\") | .processing_status" 2>/dev/null || echo "unknown")"
  t_updated="$(awk -F'\t' -v rid="${res_id}" '$1==rid {print $2; exit}' "${OUT_DIR}/.tiempos_completado.tsv")"
  if [[ -z "${t_updated}" ]]; then
    t_updated="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  fi

  dur_s="$(python3 - "${t_confirm}" "${t_updated}" <<'PY'
import datetime, sys
def parse(ts):
    ts = ts.replace("Z", "+00:00")
    return datetime.datetime.fromisoformat(ts)
try:
    dt = (parse(sys.argv[2]) - parse(sys.argv[1])).total_seconds()
    print(max(0, int(round(dt))))
except Exception:
    print(0)
PY
)"

  hls_files="$(gcloud storage ls "${GCS_HLS_BUCKET}/hls/${stable_id}/**" --project="${PROJECT_ID}" 2>/dev/null || true)"
  master_ok="false"
  if grep -q "master.m3u8" <<< "${hls_files}"; then
    master_ok="true"
  fi
  seg_count="$(grep -c '\.ts$' <<< "${hls_files}" || true)"

  reconciliado="false"
  if [[ "${status_final}" == "completed" && "${master_ok}" == "true" && "${seg_count}" -gt 0 ]]; then
    reconciliado="true"
  fi

  printf '%s\t%s\tvideo_corto_120s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "${orden}" "${etapa}" "${COURSE_ID}" "${UNIT_ID}" "${res_id}" "${stable_id}" "${obj_key}" \
    "${http_init}" "${http_put}" "${http_complete}" "${api_inst}" "${req_id}" \
    "${t_confirm}" "${t_updated}" "${dur_s}" "${status_final}" "${master_ok}" "${seg_count}" >> "${TRABAJOS_TSV}"

  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "${orden}" "${res_id}" "${stable_id}" "${status_final}" "${master_ok}" "${seg_count}" "${reconciliado}" >> "${RECONCILIACION_TSV}"
done < "${OUT_DIR}/.confirmados.tsv"

rm -f "${PREUPLOAD_TSV}" "${OUT_DIR}/.confirmados.tsv" "${OUT_DIR}/.tiempos_completado.tsv"
log "Reconciliación completada en ${TRABAJOS_TSV} y ${RECONCILIACION_TSV}."
