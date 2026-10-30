#!/usr/bin/env bash
# ==============================================================================
# Plataforma MOOC — Verificación de integridad de estado compartido, continuidad
# de sesión, retiro de instancia de API y retiro de worker con trabajo activo
# (Issue E1 / #216)
#
# Ejecuta de forma determinista y con aserciones estrictas (Nota Técnica 36):
#   Fase 0: Comprobación de sessionAffinity=NONE en el balanceador, garantía de
#           >=2 réplicas sanas en mooc-api-group e inventario inicial de
#           mooc-api-group y mooc-worker-group (nombre + ID numérico).
#   Fase 1: Recorrido extremo a extremo ampliado (scripts/e2e_cloud) registrando
#           X-Instance-ID junto a X-Request-ID, probando continuidad de sesión
#           entre >=2 instancias, revocación por logout en todas las réplicas,
#           corte de sesiones por suspensión de usuario y reconciliación base.
#   Fase 2: Retiro determinista de una instancia de API bajo tráfico continuo
#           (gcloud compute instance-groups managed delete-instances mooc-api-group),
#           documentando el comportamiento de peticiones en curso, el reemplazo
#           por ID numérico y la ausencia de pérdida de datos de negocio.
#   Fase 3: Retiro determinista de un worker con un trabajo activo bajo tráfico
#           (gcloud compute instance-groups managed delete-instances mooc-worker-group),
#           demostrando en Redis que la tarea interrumpida regresa por drenaje
#           ordenado (asynq:{default}:pending con retry=0 tras ~90 s) y no por
#           muerte súbita (asynq:{default}:retry), y que el reemplazo la completa.
#   Fase 4: Comprobación de las tres defensas contra efectos duplicados (API,
#           worker e idempotencia del prefijo HLS en GCS) y reconciliación total
#           de los trabajos aceptados de la corrida.
#
# Nota: Estas comprobaciones controladas (delete-instances) complementan el ciclo
# automático de escalado por carga de G5 y G6 y no lo sustituyen.
# ==============================================================================

set -euo pipefail

URL="${URL:-}"
PROJECT="${PROJECT:-plataforma-mooc-entrega2}"
ZONE="${ZONE:-us-east1-b}"
API_MIG="${API_MIG:-mooc-api-group}"
WORKER_MIG="${WORKER_MIG:-mooc-worker-group}"
QUEUE_VM="${QUEUE_VM:-mooc-queue-server}"
BACKEND_SERVICE="${BACKEND_SERVICE:-mooc-api-backend}"
HLS_BUCKET="${HLS_BUCKET:-plataforma-mooc-entrega2-hls}"
SALIDA="${SALIDA:-docs/entrega3/evidencias/E1}"
SOLO_E2E=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --url)             URL="$2"; shift 2 ;;
    --project)         PROJECT="$2"; shift 2 ;;
    --zone)            ZONE="$2"; shift 2 ;;
    --api-mig)         API_MIG="$2"; shift 2 ;;
    --worker-mig)      WORKER_MIG="$2"; shift 2 ;;
    --queue-vm)        QUEUE_VM="$2"; shift 2 ;;
    --backend-service) BACKEND_SERVICE="$2"; shift 2 ;;
    --hls-bucket)      HLS_BUCKET="$2"; shift 2 ;;
    --salida)          SALIDA="$2"; shift 2 ;;
    --solo-e2e)        SOLO_E2E=1; shift ;;
    --help|-h)
      sed -n '2,32p' "$0" | sed 's/^# \?//'
      exit 0
      ;;
    *)
      echo "Opción desconocida: $1" >&2
      exit 2
      ;;
  esac
done

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

# Aseguramos que go esté en el PATH también fuera del sandbox interactivo
export PATH="${PATH}:${HOME}/.local/bin:${HOME}/.local/go/bin:/usr/local/go/bin:/snap/bin"

if [[ -z "${URL}" ]]; then
  URL="$(jq -r '.values[] | select(.key=="baseUrl") | .value' docs/postman/mooc_cloud.postman_environment.json 2>/dev/null || true)"
[[ -n "${URL}" ]] || { echo "Indique --url o URL=https://..." >&2; exit 2; }
fi
URL="${URL%/}"

mkdir -p "${SALIDA}"
LOG_RET="${SALIDA}/retiro_y_reconciliacion.txt"
TSV_API="${SALIDA}/retiro_api_trafico.tsv"
TSV_REDIS="${SALIDA}/retiro_worker_redis.tsv"

log() {
  printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" | tee -a "${LOG_RET}"
}

fail() {
  log "FALLO: $*"
  exit 1
}

assert_eq() {
  local got="$1" want="$2" msg="$3"
  if [[ "${got}" != "${want}" ]]; then
    fail "${msg} (obtenido='${got}', esperado='${want}')"
  fi
  log "OK: ${msg} (${got})"
}

assert_ge() {
  local got="$1" min="$2" msg="$3"
  if (( got < min )); then
    fail "${msg} (obtenido=${got}, mínimo=${min})"
  fi
  log "OK: ${msg} (${got} >= ${min})"
}

for cmd in curl jq gcloud go ffmpeg; do
  command -v "${cmd}" >/dev/null 2>&1 || fail "Falta dependencia requerida: ${cmd}"
done

: > "${LOG_RET}"
log "======================================================================"
log "E1 (#216) — Integridad de estado compartido, continuidad y retiro bajo tráfico"
log "Proyecto: ${PROJECT} | Zona: ${ZONE} | URL: ${URL}"
log "Directorio de evidencia: ${SALIDA}"
log "======================================================================"

# ==============================================================================
# FASE 0: Afinidad de sesión (NONE) e inventario inicial de grupos
# ==============================================================================
log "--- Fase 0: Comprobación de afinidad de sesión e inventario inicial ---"

AFFINITY="$(gcloud compute backend-services describe "${BACKEND_SERVICE}" \
  --global --project="${PROJECT}" --format="value(sessionAffinity)")"
DRAIN_SEC="$(gcloud compute backend-services describe "${BACKEND_SERVICE}" \
  --global --project="${PROJECT}" --format="value(connectionDraining.drainingTimeoutSec)")"

assert_eq "${AFFINITY}" "NONE" "El backend service ${BACKEND_SERVICE} tiene sessionAffinity=NONE"
assert_eq "${DRAIN_SEC}" "60" "El backend service ${BACKEND_SERVICE} tiene connection_draining_timeout_sec=60"

mapfile -t API_INSTANCES < <(gcloud compute instance-groups managed list-instances "${API_MIG}" \
  --zone="${ZONE}" --project="${PROJECT}" \
  --filter="instanceStatus=RUNNING" \
  --format="value(instance.basename())" | sort)

ORIG_MIN_REPLICAS="$(gcloud compute instance-groups managed describe "${API_MIG}" \
  --zone="${ZONE}" --project="${PROJECT}" --format="value(autoscaler.autoscalingPolicy.minNumReplicas)")"

# Con min_replicas=1 el autoescalador retira la réplica extra en cuanto se restaura el mínimo (nota 43),
# así que se fija 2 aunque ya haya dos instancias y se espera a que ninguna esté en transición.
if (( ${#API_INSTANCES[@]} < 2 || ORIG_MIN_REPLICAS < 2 )); then
  restaurar_min_replicas() {
    if [[ -n "${ORIG_MIN_REPLICAS:-}" ]]; then
      log "Restaurando min-num-replicas=${ORIG_MIN_REPLICAS} en ${API_MIG} (valor de Terraform)"
      gcloud compute instance-groups managed update-autoscaling "${API_MIG}" \
        --zone="${ZONE}" --project="${PROJECT}" \
        --min-num-replicas="${ORIG_MIN_REPLICAS}" >> "${LOG_RET}" 2>&1 || true
    fi
  }
  trap restaurar_min_replicas EXIT
  log "El grupo ${API_MIG} tiene ${#API_INSTANCES[@]} instancia(s); ajustando min-num-replicas=2 en el autoescalador para E1..."
  gcloud compute instance-groups managed update-autoscaling "${API_MIG}" \
    --zone="${ZONE}" --project="${PROJECT}" \
    --min-num-replicas=2 >> "${LOG_RET}" 2>&1

  log "Esperando a que la segunda réplica de ${API_MIG} esté RUNNING y atienda tráfico en el balanceador..."
  for espera in $(seq 1 45); do
    mapfile -t API_INSTANCES < <(gcloud compute instance-groups managed list-instances "${API_MIG}" \
      --zone="${ZONE}" --project="${PROJECT}" \
      --filter="instanceStatus=RUNNING AND currentAction=NONE" \
      --format="value(instance.basename())" | sort)
    if (( ${#API_INSTANCES[@]} >= 2 )); then
      SEEN_READY="$(for k in $(seq 1 12); do
        curl -sI --max-time 6 "${URL}/api/v1/ready" 2>/dev/null | tr -d '\r' | awk 'tolower($1)=="x-instance-id:"{print $2}'
      done | sort -u | wc -l | tr -d ' ')"
      if (( SEEN_READY >= 2 )); then
        log "OK: Dos instancias distintas responden ya en ${URL}/api/v1/ready"
        break
      fi
    fi
    sleep 5
  done
fi

assert_ge "${#API_INSTANCES[@]}" 2 "Réplicas RUNNING en ${API_MIG}"

log "Inventario inicial de ${API_MIG}:"
declare -A API_INITIAL_ID
declare -A API_INITIAL_TS
for inst in "${API_INSTANCES[@]}"; do
  read -r iid its < <(gcloud compute instances describe "${inst}" \
    --zone="${ZONE}" --project="${PROJECT}" \
    --format="value(id,creationTimestamp)")
  API_INITIAL_ID["${inst}"]="${iid}"
  API_INITIAL_TS["${inst}"]="${its}"
  log "  API VM: name=${inst} id=${iid} creationTimestamp=${its}"
done

log "Inventario inicial de ${WORKER_MIG}:"
mapfile -t WORKER_INSTANCES < <(gcloud compute instance-groups managed list-instances "${WORKER_MIG}" \
  --zone="${ZONE}" --project="${PROJECT}" \
  --filter="instanceStatus=RUNNING" \
  --format="value(instance.basename())" | sort)

assert_ge "${#WORKER_INSTANCES[@]}" 1 "Réplicas RUNNING en ${WORKER_MIG}"

declare -A WORKER_INITIAL_ID
declare -A WORKER_INITIAL_TS
for inst in "${WORKER_INSTANCES[@]}"; do
  read -r wid wts < <(gcloud compute instances describe "${inst}" \
    --zone="${ZONE}" --project="${PROJECT}" \
    --format="value(id,creationTimestamp)")
  WORKER_INITIAL_ID["${inst}"]="${wid}"
  WORKER_INITIAL_TS["${inst}"]="${wts}"
  log "  Worker VM: name=${inst} id=${wid} creationTimestamp=${wts}"
done

# ==============================================================================
# FASE 1: Arnés e2e_cloud (X-Instance-ID, continuidad, logout, suspensión)
# ==============================================================================
log "--- Fase 1: Ejecución de scripts/e2e_cloud con registro de X-Instance-ID ---"
go run ./scripts/e2e_cloud -base "${URL}" -salida "${SALIDA}" -min-instancias 2 -video "${VIDEO_E2E:-scratch/video_corto.mp4}"
log "OK: scripts/e2e_cloud completó los 8 flujos sin fallos (ver ${SALIDA}/resultados.md y ${SALIDA}/corrida_e2e_cloud.txt)"

if [[ "${SOLO_E2E}" -eq 1 ]]; then
  log "Modo --solo-e2e solicitado; finalizando tras Fase 1."
  exit 0
fi

# ==============================================================================
# FASE 2: Retiro determinista de una instancia de API bajo tráfico
# ==============================================================================
log "--- Fase 2: Retiro determinista de una instancia de API bajo tráfico ---"

# Pausa breve para asegurar margen completo en el limitador de login (10/min/IP)
sleep 15

VICTIMA_API="${API_INSTANCES[0]}"
VICTIMA_API_OLD_ID="${API_INITIAL_ID[${VICTIMA_API}]}"
VICTIMA_API_OLD_TS="${API_INITIAL_TS[${VICTIMA_API}]}"
log "Instancia de API seleccionada para retiro: ${VICTIMA_API} (id=${VICTIMA_API_OLD_ID}, creationTimestamp=${VICTIMA_API_OLD_TS})"

LOGIN_RESP="$(curl -sS -X POST "${URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"estudiante1@plataforma-mooc.test","password":"Password123!"}')"
STUDENT_TOKEN="$(printf '%s' "${LOGIN_RESP}" | jq -r '.token')"
[[ -n "${STUDENT_TOKEN}" && "${STUDENT_TOKEN}" != "null" ]] || fail "No se pudo obtener token de estudiante antes del retiro de API"

PROF_RESP="$(curl -sS -X POST "${URL}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"profesor1@plataforma-mooc.test","password":"Password123!"}')"
PROF_TOKEN="$(printf '%s' "${PROF_RESP}" | jq -r '.token')"
[[ -n "${PROF_TOKEN}" && "${PROF_TOKEN}" != "null" ]] || fail "No se pudo obtener token de profesor antes del retiro de API"

COURSES_BEFORE="$(curl -sS "${URL}/api/v1/courses?limit=50" | jq '.items | length')"
ENROLLS_BEFORE="$(curl -sS -H "Authorization: Bearer ${STUDENT_TOKEN}" "${URL}/api/v1/me/enrollments?limit=50" | jq '.items | length')"

STAMP_E1="$(date -u +%Y%m%d-%H%M%S)"
COURSE_PRE_JSON="$(curl -sS -X POST "${URL}/api/v1/courses" \
  -H "Authorization: Bearer ${PROF_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"title\":\"Testigo E1 pre-retiro API ${STAMP_E1}\",\"description\":\"Curso creado antes de retirar ${VICTIMA_API}\"}")"
COURSE_PRE_ID="$(printf '%s' "${COURSE_PRE_JSON}" | jq -r '.id')"
[[ -n "${COURSE_PRE_ID}" && "${COURSE_PRE_ID}" != "null" ]] || fail "No se pudo crear curso testigo pre-retiro"

log "Estado de negocio ANTES del retiro de ${VICTIMA_API}: cursos_publicados=${COURSES_BEFORE}, inscripciones_estudiante=${ENROLLS_BEFORE}, curso_testigo_pre=${COURSE_PRE_ID}"

printf 'seq\ttimestamp\tfase\thttp_status\tx_instance_id\tx_request_id\tlatency_ms\n' > "${TSV_API}"

run_api_probe() {
  local seq="$1" fase="$2"
  local hdr_file body_file start_ns end_ns lat_ms status inst req_id
  hdr_file="$(mktemp)"
  body_file="$(mktemp)"
  start_ns="$(date +%s%N)"
  status="$(curl -sS -D "${hdr_file}" -o "${body_file}" -w '%{http_code}' \
    -H "Authorization: Bearer ${STUDENT_TOKEN}" \
    "${URL}/api/v1/auth/sessions" || echo "000")"
  end_ns="$(date +%s%N)"
  lat_ms=$(( (end_ns - start_ns) / 1000000 ))
  inst="$(awk 'tolower($1)=="x-instance-id:" {gsub(/\r/,"",$2); print $2}' "${hdr_file}" | tail -n1)"
  req_id="$(awk 'tolower($1)=="x-request-id:" {gsub(/\r/,"",$2); print $2}' "${hdr_file}" | tail -n1)"
  rm -f "${hdr_file}" "${body_file}"
  printf '%d\t%s\t%s\t%s\t%s\t%s\t%d\n' \
    "${seq}" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${fase}" "${status}" "${inst:-none}" "${req_id:-none}" "${lat_ms}" >> "${TSV_API}"
}

log "Enviando 10 peticiones previas al retiro para registrar reparto inicial..."
for seq in $(seq 1 10); do
  run_api_probe "${seq}" "pre-delete"
  sleep 0.3
done

log "Ejecutando retiro determinista de la instancia de API ${VICTIMA_API} bajo tráfico..."
(
  gcloud compute instance-groups managed delete-instances "${API_MIG}" \
    --instances="${VICTIMA_API}" \
    --zone="${ZONE}" \
    --project="${PROJECT}" \
    --quiet >> "${LOG_RET}" 2>&1
) &
DELETE_API_PID=$!

COURSE_INFLIGHT_ID=""
for seq in $(seq 11 45); do
  run_api_probe "${seq}" "during-delete"
  if [[ "${seq}" -eq 15 ]]; then
    COURSE_INFLIGHT_JSON="$(curl -sS -X POST "${URL}/api/v1/courses" \
      -H "Authorization: Bearer ${PROF_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "{\"title\":\"Testigo E1 en-vuelo API ${STAMP_E1}\",\"description\":\"Curso creado durante delete-instances de ${VICTIMA_API}\"}")"
    COURSE_INFLIGHT_ID="$(printf '%s' "${COURSE_INFLIGHT_JSON}" | jq -r '.id')"
    log "Escritura de negocio en pleno retiro de ${VICTIMA_API}: course_id=${COURSE_INFLIGHT_ID}"
  fi
  sleep 1
done

wait "${DELETE_API_PID}"
log "Comando delete-instances sobre ${VICTIMA_API} retornó 0. Esperando reemplazo en ${API_MIG} para mantener 2 réplicas..."

NEW_API_NAME=""
NEW_API_ID=""
NEW_API_TS=""
for intento in $(seq 1 45); do
  mapfile -t CUR_API < <(gcloud compute instance-groups managed list-instances "${API_MIG}" \
    --zone="${ZONE}" --project="${PROJECT}" \
    --filter="instanceStatus=RUNNING" \
    --format="value(instance.basename())" | sort)
  if (( ${#CUR_API[@]} >= 2 )); then
    for cand in "${CUR_API[@]}"; do
      read -r cid cts < <(gcloud compute instances describe "${cand}" \
        --zone="${ZONE}" --project="${PROJECT}" \
        --format="value(id,creationTimestamp)" 2>/dev/null || echo "")
      if [[ -n "${cid}" && "${cid}" != "${API_INITIAL_ID[${cand}]:-}" ]]; then
        NEW_API_NAME="${cand}"
        NEW_API_ID="${cid}"
        NEW_API_TS="${cts}"
        break 2
      fi
    done
  fi
  sleep 5
done

[[ -n "${NEW_API_ID}" ]] || fail "No se detectó instancia de reemplazo con ID numérico nuevo en ${API_MIG}"
log "OK: Instancia de reemplazo en ${API_MIG}: name=${NEW_API_NAME} id=${NEW_API_ID} creationTimestamp=${NEW_API_TS} (reemplazó a ${VICTIMA_API} id=${VICTIMA_API_OLD_ID})"

log "Sondeando peticiones post-reemplazo hasta que la nueva instancia (${NEW_API_NAME}) atienda tráfico..."
for seq in $(seq 46 85); do
  run_api_probe "${seq}" "post-replace"
  sleep 2
done

TOTAL_PROBES="$(awk 'NR>1 {c++} END {print c+0}' "${TSV_API}")"
OK_PROBES="$(awk 'NR>1 && $4=="200" {c++} END {print c+0}' "${TSV_API}")"
DIST_API_SEEN="$(awk 'NR>1 {print $5}' "${TSV_API}" | sort | uniq -c | tr '\n' ';' | sed 's/;$/\n/')"

log "Resumen de tráfico durante retiro y reemplazo de API: ${OK_PROBES}/${TOTAL_PROBES} respuestas 200 OK. Distribución por X-Instance-ID: ${DIST_API_SEEN}"
assert_eq "${OK_PROBES}" "${TOTAL_PROBES}" "100% de las peticiones en vuelo durante el retiro de API respondieron 200 OK sin cortes"

COURSES_AFTER="$(curl -sS "${URL}/api/v1/courses?limit=50" | jq '.items | length')"
ENROLLS_AFTER="$(curl -sS -H "Authorization: Bearer ${STUDENT_TOKEN}" "${URL}/api/v1/me/enrollments?limit=50" | jq '.items | length')"
PRE_CHECK_ID="$(curl -sS -H "Authorization: Bearer ${PROF_TOKEN}" "${URL}/api/v1/courses/${COURSE_PRE_ID}" | jq -r '.id')"
INFLIGHT_CHECK_ID="$(curl -sS -H "Authorization: Bearer ${PROF_TOKEN}" "${URL}/api/v1/courses/${COURSE_INFLIGHT_ID}" | jq -r '.id')"

assert_eq "${COURSES_AFTER}" "${COURSES_BEFORE}" "Conteo de catálogo público intacto tras retiro de API"
assert_eq "${ENROLLS_AFTER}" "${ENROLLS_BEFORE}" "Inscripciones del estudiante intactas tras retiro de API"
assert_eq "${PRE_CHECK_ID}" "${COURSE_PRE_ID}" "Curso testigo pre-retiro persistido sin pérdida"
assert_eq "${INFLIGHT_CHECK_ID}" "${COURSE_INFLIGHT_ID}" "Curso testigo creado en vuelo durante retiro persistido sin pérdida"

# ==============================================================================
# FASE 3: Retiro de un worker con un trabajo activo bajo tráfico y reconciliación
# ==============================================================================
log "--- Fase 3: Retiro de un worker con trabajo activo bajo tráfico y reconciliación en Redis ---"

mapfile -t WORKER_CUR < <(gcloud compute instance-groups managed list-instances "${WORKER_MIG}" \
  --zone="${ZONE}" --project="${PROJECT}" \
  --filter="instanceStatus=RUNNING" \
  --format="value(instance.basename())" | sort)
assert_ge "${#WORKER_CUR[@]}" 1 "Al menos un worker RUNNING antes de la prueba de retiro"

VICTIMA_WORKER="${WORKER_CUR[0]}"
read -r VICTIMA_WORKER_OLD_ID VICTIMA_WORKER_OLD_TS < <(gcloud compute instances describe "${VICTIMA_WORKER}" \
  --zone="${ZONE}" --project="${PROJECT}" \
  --format="value(id,creationTimestamp)")
log "Worker seleccionado para retiro con trabajo activo: ${VICTIMA_WORKER} (id=${VICTIMA_WORKER_OLD_ID}, creationTimestamp=${VICTIMA_WORKER_OLD_TS})"

MOD_JSON="$(curl -sS -X POST "${URL}/api/v1/courses/${COURSE_PRE_ID}/modules" \
  -H "Authorization: Bearer ${PROF_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"title":"Módulo E1 Drenaje Worker"}')"
MOD_ID="$(printf '%s' "${MOD_JSON}" | jq -r '.id')"

UNIT_JSON="$(curl -sS -X POST "${URL}/api/v1/modules/${MOD_ID}/units" \
  -H "Authorization: Bearer ${PROF_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"title":"Unidad E1 Video Transcodificación"}')"
UNIT_ID="$(printf '%s' "${UNIT_JSON}" | jq -r '.id')"

RES1_JSON="$(curl -sS -X POST "${URL}/api/v1/units/${UNIT_ID}/resources" \
  -H "Authorization: Bearer ${PROF_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"title":"Video E1-A bajo retiro de worker","type":"video","is_visible":true,"is_mandatory":true,"allow_download":false}')"
RES1_ID="$(printf '%s' "${RES1_JSON}" | jq -r '.id')"
RES1_STABLE="$(printf '%s' "${RES1_JSON}" | jq -r '.stable_id')"

RES2_JSON="$(curl -sS -X POST "${URL}/api/v1/units/${UNIT_ID}/resources" \
  -H "Authorization: Bearer ${PROF_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"title":"Video E1-B en cola durante retiro","type":"video","is_visible":true,"is_mandatory":true,"allow_download":false}')"
RES2_ID="$(printf '%s' "${RES2_JSON}" | jq -r '.id')"
RES2_STABLE="$(printf '%s' "${RES2_JSON}" | jq -r '.stable_id')"

log "Recursos creados para reconciliación bajo retiro de worker:"
log "  Video E1-A: id=${RES1_ID} stable_id=${RES1_STABLE}"
log "  Video E1-B: id=${RES2_ID} stable_id=${RES2_STABLE}"

# Usamos scratch/video_e1_drenaje.mp4 (240 s a 1280x720, ~180 s de transcodificado)
# para RES1_ID de modo que siga activo al agotarse los 90 s de WORKER_DRAIN_TIMEOUT
# y demuestre el reencolado limpio (asynq:{default}:pending con retry=0).
DRAIN_MP4="scratch/video_e1_drenaje.mp4"
if [[ ! -s "${DRAIN_MP4}" ]]; then
  log "Generando ${DRAIN_MP4} (240 s a 1280x720)..."
  mkdir -p scratch
  ffmpeg -hide_banner -loglevel error -y -nostdin \
    -f lavfi -i "testsrc=size=1280x720:rate=25:duration=240" \
    -f lavfi -i "sine=frequency=440:sample_rate=44100:duration=240" \
    -c:v libx264 -preset veryfast -pix_fmt yuv420p \
    -c:a aac -shortest "${DRAIN_MP4}"
fi

TMP_MP4="$(mktemp --suffix=.mp4)"
ffmpeg -hide_banner -loglevel error -y -nostdin \
  -f lavfi -i "testsrc=duration=8:size=640x360:rate=24" \
  -f lavfi -i "sine=frequency=440:duration=8" \
  -c:v libx264 -preset medium -pix_fmt yuv420p -c:a aac -b:a 64k \
  "${TMP_MP4}"

upload_and_confirm_video() {
  local res_id="$1" fname="$2" file_path="$3"
  local file_size up_json upload_url obj_key hdr_f body_f code inst req_id
  file_size="$(stat -c%s "${file_path}")"
  up_json="$(curl -sS -X POST "${URL}/api/v1/media/presigned-url" \
    -H "Authorization: Bearer ${PROF_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"resource_id\":\"${res_id}\",\"filename\":\"${fname}\",\"mime_type\":\"video/mp4\",\"file_size_bytes\":${file_size}}")"
  upload_url="$(printf '%s' "${up_json}" | jq -r '.upload_url')"
  obj_key="$(printf '%s' "${up_json}" | jq -r '.object_key')"

  curl -sS -X PUT "${upload_url}" \
    -H "Content-Type: video/mp4" \
    --data-binary @"${file_path}" >/dev/null

  hdr_f="$(mktemp)"
  body_f="$(mktemp)"
  code="$(curl -sS -D "${hdr_f}" -o "${body_f}" -w '%{http_code}' \
    -X POST "${URL}/api/v1/media/uploads/${res_id}/complete" \
    -H "Authorization: Bearer ${PROF_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"object_key\":\"${obj_key}\"}")"
  inst="$(awk 'tolower($1)=="x-instance-id:" {gsub(/\r/,"",$2); print $2}' "${hdr_f}" | tail -n1)"
  req_id="$(awk 'tolower($1)=="x-request-id:" {gsub(/\r/,"",$2); print $2}' "${hdr_f}" | tail -n1)"
  rm -f "${hdr_f}" "${body_f}"
  printf '%s|%s|%s|%s\n' "${code}" "${obj_key}" "${inst}" "${req_id}"
}

IFS='|' read -r CODE1 OBJ1 INST1 REQ1 < <(upload_and_confirm_video "${RES1_ID}" "e1_video_a_drenaje.mp4" "${DRAIN_MP4}")
IFS='|' read -r CODE2 OBJ2 INST2 REQ2 < <(upload_and_confirm_video "${RES2_ID}" "e1_video_b_corto.mp4" "${TMP_MP4}")
rm -f "${TMP_MP4}"

assert_eq "${CODE1}" "202" "Confirmación de subida de Video E1-A (240s) aceptada (202) por ${INST1} (req=${REQ1})"
assert_eq "${CODE2}" "202" "Confirmación de subida de Video E1-B (8s) aceptada (202) por ${INST2} (req=${REQ2})"

printf 'timestamp\tactive\tpending\tretry\tarchived\tidemp_keys\n' > "${TSV_REDIS}"

# En mooc-queue-server, Redis corre como servicio del sistema (ver C7/RUNBOOK.md §2).
probe_redis() {
  gcloud compute ssh "${QUEUE_VM}" --zone="${ZONE}" --project="${PROJECT}" --tunnel-through-iap --command='
    act=$(redis-cli -n 0 LLEN "asynq:{default}:active")
    pen=$(redis-cli -n 0 LLEN "asynq:{default}:pending")
    ret=$(redis-cli -n 0 ZCARD "asynq:{default}:retry")
    arc=$(redis-cli -n 0 ZCARD "asynq:{default}:archived")
    idm=$(redis-cli -n 0 KEYS "idempotency:worker:*" | sed "/^$/d" | wc -l)
    printf "%s\t%s\t%s\t%s\t%s\n" "$act" "$pen" "$ret" "$arc" "$idm"
  ' 2>/dev/null | tail -n1
}

for i in $(seq 1 15); do
  IFS=$'\t' read -r R_ACT R_PEN R_RET R_DEA R_IDM < <(probe_redis)
  TS_NOW="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "${TS_NOW}" "${R_ACT}" "${R_PEN}" "${R_RET}" "${R_DEA}" "${R_IDM}" >> "${TSV_REDIS}"
  log "Redis pre-retiro: active=${R_ACT} pending=${R_PEN} retry=${R_RET} archived=${R_DEA} idempotency_keys=${R_IDM}"
  if (( R_ACT >= 1 )); then
    break
  fi
  sleep 2
done
# Hay tareas archivadas de pruebas anteriores: lo que no puede pasar es que aumenten durante la corrida.
ARCHIVED_BASE="${R_DEA}"
assert_ge "${R_ACT}" 1 "El worker ${VICTIMA_WORKER} tiene un trabajo activo en vuelo (asynq:{default}:active >= 1)"

DRAIN_START_EPOCH="$(date +%s)"
log "Ejecutando retiro determinista del worker ${VICTIMA_WORKER} con trabajo activo..."
gcloud compute instance-groups managed delete-instances "${WORKER_MIG}" \
  --instances="${VICTIMA_WORKER}" \
  --zone="${ZONE}" \
  --project="${PROJECT}" \
  --quiet >> "${LOG_RET}" 2>&1
log "Solicitud delete-instances aceptada por el MIG. Sondeando Redis cada 5 s durante la ventana de drenaje (~90-105 s)..."

MAX_RETRY_SEEN=0
MAX_ARCHIVED_SEEN="${ARCHIVED_BASE}"
REQUEUE_OBSERVED=0
DRAIN_ELAPSED=0

for ciclo in $(seq 1 30); do
  IFS=$'\t' read -r R_ACT R_PEN R_RET R_DEA R_IDM < <(probe_redis)
  TS_NOW="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  ELAPSED_NOW="$(( $(date +%s) - DRAIN_START_EPOCH ))"
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "${TS_NOW}" "${R_ACT}" "${R_PEN}" "${R_RET}" "${R_DEA}" "${R_IDM}" >> "${TSV_REDIS}"
  log "Redis durante drenaje (${ELAPSED_NOW}s): active=${R_ACT} pending=${R_PEN} retry=${R_RET} archived=${R_DEA} idemp=${R_IDM}"
  if (( R_RET > MAX_RETRY_SEEN )); then MAX_RETRY_SEEN="${R_RET}"; fi
  if (( R_DEA > MAX_ARCHIVED_SEEN )); then MAX_ARCHIVED_SEEN="${R_DEA}"; fi
  if (( ELAPSED_NOW >= 75 && R_PEN >= 1 && R_RET == 0 )); then
    REQUEUE_OBSERVED=1
    DRAIN_ELAPSED="${ELAPSED_NOW}"
    log "OK: Tarea en vuelo devuelta a asynq:{default}:pending con retry=0 a los ${DRAIN_ELAPSED}s del delete-instances"
    break
  fi
  sleep 5
done

assert_eq "${MAX_RETRY_SEEN}" "0" "Ninguna tarea cayó en asynq:{default}:retry por muerte súbita (drenaje limpio requeue -> pending con retry=0)"
assert_eq "${MAX_ARCHIVED_SEEN}" "${ARCHIVED_BASE}" "Ninguna tarea nueva cayó en asynq:{default}:archived (línea base ${ARCHIVED_BASE})"
assert_eq "${REQUEUE_OBSERVED}" "1" "La tarea activa regresó a asynq:{default}:pending con retry=0 tras el drenaje"

log "Esperando a que el worker de reemplazo en ${WORKER_MIG} arranque y complete los trabajos reencolados..."
NEW_WORKER_NAME=""
NEW_WORKER_ID=""
NEW_WORKER_TS=""

for intento in $(seq 1 90); do
  mapfile -t W_CUR < <(gcloud compute instance-groups managed list-instances "${WORKER_MIG}" \
    --zone="${ZONE}" --project="${PROJECT}" \
    --filter="instanceStatus=RUNNING" \
    --format="value(instance.basename())" | sort)
  for cand in "${W_CUR[@]}"; do
    read -r wid wts < <(gcloud compute instances describe "${cand}" \
      --zone="${ZONE}" --project="${PROJECT}" \
      --format="value(id,creationTimestamp)" 2>/dev/null || echo "")
    if [[ -n "${wid}" && "${wid}" != "${VICTIMA_WORKER_OLD_ID}" ]]; then
      NEW_WORKER_NAME="${cand}"
      NEW_WORKER_ID="${wid}"
      NEW_WORKER_TS="${wts}"
    fi
  done

  IFS=$'\t' read -r R_ACT R_PEN R_RET R_DEA R_IDM < <(probe_redis)
  TS_NOW="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "${TS_NOW}" "${R_ACT}" "${R_PEN}" "${R_RET}" "${R_DEA}" "${R_IDM}" >> "${TSV_REDIS}"
  log "Redis post-reemplazo: active=${R_ACT} pending=${R_PEN} retry=${R_RET} archived=${R_DEA} idemp=${R_IDM} (worker_nuevo=${NEW_WORKER_NAME:-pendiente} id=${NEW_WORKER_ID:-pendiente})"

  if [[ -n "${NEW_WORKER_ID}" ]] && (( R_ACT == 0 && R_PEN == 0 && R_RET == 0 )); then
    break
  fi
  sleep 5
done

[[ -n "${NEW_WORKER_ID}" ]] || fail "No se detectó worker de reemplazo con ID numérico distinto en ${WORKER_MIG}"
log "OK: Worker de reemplazo verificado: name=${NEW_WORKER_NAME} id=${NEW_WORKER_ID} creationTimestamp=${NEW_WORKER_TS} (reemplazó a ${VICTIMA_WORKER} id=${VICTIMA_WORKER_OLD_ID})"
assert_eq "${R_ACT}" "0" "Cola active vacía tras completar el lote en el worker de reemplazo"
assert_eq "${R_PEN}" "0" "Cola pending vacía tras completar el lote en el worker de reemplazo"
assert_eq "${R_RET}" "0" "Cola retry en 0 durante toda la recuperación"
assert_eq "${R_DEA}" "${ARCHIVED_BASE}" "Cola archived sin aumentar (cero pérdidas; línea base ${ARCHIVED_BASE})"

# ==============================================================================
# FASE 4: Verificación de las 3 defensas contra duplicados y reconciliación final
# ==============================================================================
log "--- Fase 4: Verificación de las tres defensas contra efectos duplicados y reconciliación ---"

RESOURCES_AFTER="$(curl -sS -H "Authorization: Bearer ${PROF_TOKEN}" "${URL}/api/v1/units/${UNIT_ID}/resources")"
STATUS_RES1="$(printf '%s' "${RESOURCES_AFTER}" | jq -r ".items[] | select(.id==\"${RES1_ID}\") | .processing_status")"
STATUS_RES2="$(printf '%s' "${RESOURCES_AFTER}" | jq -r ".items[] | select(.id==\"${RES2_ID}\") | .processing_status")"

assert_eq "${STATUS_RES1}" "completed" "Video E1-A (${RES1_ID}) alcanzó processing_status=completed"
assert_eq "${STATUS_RES2}" "completed" "Video E1-B (${RES2_ID}) alcanzó processing_status=completed"

REPEAT_RESP="$(curl -sS -X POST "${URL}/api/v1/media/uploads/${RES1_ID}/complete" \
  -H "Authorization: Bearer ${PROF_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"object_key\":\"${OBJ1}\"}")"
REPEAT_STATUS="$(printf '%s' "${REPEAT_RESP}" | jq -r '.processing_status')"
IFS=$'\t' read -r R_ACT_AFTER R_PEN_AFTER R_RET_AFTER R_DEA_AFTER R_IDM_AFTER < <(probe_redis)

assert_eq "${REPEAT_STATUS}" "completed" "Defensa 1 (API ConfirmUpload): POST /complete repetido devuelve processing_status=completed sin mutar estado"
assert_eq "${R_PEN_AFTER}" "0" "Defensa 1 (API ConfirmUpload): POST /complete repetido no encola tarea nueva en asynq:{default}:pending"

assert_ge "${R_IDM_AFTER}" 2 "Defensa 2 (Worker idempotency): llaves idempotency:worker:* registradas en Redis compartido"

HLS1_COUNT="$(gcloud storage ls "gs://${HLS_BUCKET}/hls/${RES1_STABLE}/**" --project="${PROJECT}" | wc -l | tr -d ' ')"
HLS2_COUNT="$(gcloud storage ls "gs://${HLS_BUCKET}/hls/${RES2_STABLE}/**" --project="${PROJECT}" | wc -l | tr -d ' ')"
HLS1_MASTER="$(gcloud storage ls "gs://${HLS_BUCKET}/hls/${RES1_STABLE}/master.m3u8" --project="${PROJECT}" | wc -l | tr -d ' ')"
HLS2_MASTER="$(gcloud storage ls "gs://${HLS_BUCKET}/hls/${RES2_STABLE}/master.m3u8" --project="${PROJECT}" | wc -l | tr -d ' ')"

assert_eq "${HLS1_MASTER}" "1" "Defensa 3 (Prefijo HLS determinista): único master.m3u8 en gs://${HLS_BUCKET}/hls/${RES1_STABLE}/"
assert_eq "${HLS2_MASTER}" "1" "Defensa 3 (Prefijo HLS determinista): único master.m3u8 en gs://${HLS_BUCKET}/hls/${RES2_STABLE}/"
log "Objetos HLS reconciliados en GCS: hls/${RES1_STABLE}/=${HLS1_COUNT} objetos, hls/${RES2_STABLE}/=${HLS2_COUNT} objetos"

log "======================================================================"
log "RESULTADO GLOBAL E1 (#216): TODAS LAS COMPROBACIONES SUPERADAS"
log "  - sessionAffinity=NONE verificada en ${BACKEND_SERVICE}"
log "  - Continuidad de sesión, logout y suspensión probadas entre réplicas (ver ${SALIDA}/resultados.md)"
log "  - Retiro de instancia API (${VICTIMA_API} id=${VICTIMA_API_OLD_ID} -> ${NEW_API_NAME} id=${NEW_API_ID}): ${OK_PROBES}/${TOTAL_PROBES} 200 OK, 0 pérdida de datos"
log "  - Retiro de worker activo (${VICTIMA_WORKER} id=${VICTIMA_WORKER_OLD_ID} -> ${NEW_WORKER_NAME} id=${NEW_WORKER_ID}): drenaje en ${DRAIN_ELAPSED}s, requeue a pending con retry=0, 0 pérdidas y 0 duplicados"
log "======================================================================"
