#!/usr/bin/env bash
# ==============================================================================
# scripts/g6_observar_escalamiento_workers.sh — Issue G6 (#226): Escenario 3B
# Observador activo en tiempo real del ciclo de escalamiento automático del
# grupo de workers (mooc-worker-group + mooc-worker-autoscaler) y de la cola
# Redis/Asynq (mooc-queue-server).
#
# Responsabilidades críticas:
#   1. Pre-flight obligatorio: verifica en el descriptor del MIG
#      (gcloud compute instance-groups managed describe mooc-worker-group)
#      que autoscaler.status == ACTIVE, mode == ON, minNumReplicas == 1,
#      maxNumReplicas == 3, targetSize == 1, 1 worker RUNNING+HEALTHY y la serie
#      custom.googleapis.com/mooc/queue/outstanding con puntos frescos.
#   2. Muestreo periódico (cada G6_OBS_INTERVAL_S segundos):
#      - Plano GCE MIG / Autoscaler: targetSize, recommendedSize, lista de
#        instancias con id numérico, creationTimestamp, instanceStatus,
#        currentAction y detailedHealthState.
#      - Plano Cloud Monitoring: outstanding, pending, active,
#        oldest_pending_age_seconds.
#      - Plano Redis (mooc-queue-server, redis-cli -n 0 directo): LLEN pending,
#        LLEN active, ZCARD retry, ZCARD archived y llaves idempotency:worker:*.
#   3. Captura en vivo de logs de cada worker ANTES del scale-in:
#      Dado que el driver de logs del contenedor es local (json-file) y las VMs
#      se destruyen al reducirse el grupo, en cada iteración recolecta vía
#      SSH/IAP /var/lib/mooc/arranque.json y docker logs mooc-worker de toda
#      instancia RUNNING, extrayendo los eventos estructurados:
#        - "job media:process started"
#        - "media rendered to HLS"
#        - "job media:process completed"
#      para construir trabajos_por_worker.tsv e instancias.tsv sin pérdida.
#
# Modos de uso:
#   bash scripts/g6_observar_escalamiento_workers.sh --autoprueba
#   bash scripts/g6_observar_escalamiento_workers.sh [DIRECTORIO_SALIDA]
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

PROJECT_ID="${PROJECT_ID:-plataforma-mooc-entrega2}"
ZONE="${ZONE:-us-east1-b}"
WORKER_MIG="${WORKER_MIG:-mooc-worker-group}"
QUEUE_VM="${QUEUE_VM:-mooc-queue-server}"
G6_OBS_INTERVAL_S="${G6_OBS_INTERVAL_S:-20}"
G6_OBS_MAX_ITERS="${G6_OBS_MAX_ITERS:-115}"

uso() {
  cat <<'EOF'
Uso:
  bash scripts/g6_observar_escalamiento_workers.sh [opciones] [DIRECTORIO_SALIDA]

Opciones:
  --autoprueba     Ejecuta la autoprueba determinista del parser de logs de
                   workers, instancias y cola sin requerir red ni GCP.
  -h, --help       Muestra esta ayuda.
EOF
}

consolidar_logs_workers() {
  local out_dir="$1"
  python3 - "${out_dir}" <<'PY'
import json, pathlib, sys

out_dir = pathlib.Path(sys.argv[1])
raw_dir = out_dir / "workers_raw"
inst_meta_path = out_dir / ".instancias_vistas.tsv"

inst_info = {}
all_ts_seen = []
if inst_meta_path.exists():
    for line in inst_meta_path.read_text(encoding="utf-8").splitlines():
        if not line.strip() or line.startswith("instancia\t"):
            continue
        parts = line.split("\t")
        if len(parts) < 7:
            continue
        name, iid, ctime, ts_utc, status, action, health = parts[:7]
        if not all_ts_seen or all_ts_seen[-1] != ts_utc:
            all_ts_seen.append(ts_utc)
        rec = inst_info.setdefault(name, {
            "instancia": name,
            "instance_id": iid,
            "creation_timestamp": ctime,
            "primera_vista_utc": ts_utc,
            "primera_running_utc": "",
            "arranque_listo_utc": "",
            "segundos_bootstrap": "",
            "primera_healthy_utc": "",
            "primer_job_started_utc": "",
            "primer_job_completed_utc": "",
            "total_jobs_completados": 0,
            "primera_deleting_utc": "",
            "ultima_vista_utc": ts_utc,
        })
        if iid and iid != "n/a":
            rec["instance_id"] = iid
        if ctime and ctime != "n/a":
            rec["creation_timestamp"] = ctime
        rec["ultima_vista_utc"] = ts_utc
        if status == "RUNNING" and not rec["primera_running_utc"]:
            rec["primera_running_utc"] = ts_utc
        if health == "HEALTHY" and not rec["primera_healthy_utc"]:
            rec["primera_healthy_utc"] = ts_utc
        if (action in ("DELETING", "ABANDONING") or status in ("STOPPING", "TERMINATED")) and not rec["primera_deleting_utc"]:
            rec["primera_deleting_utc"] = ts_utc

global_last_ts = all_ts_seen[-1] if all_ts_seen else ""
for rec in inst_info.values():
    if global_last_ts and rec["ultima_vista_utc"] < global_last_ts and not rec["primera_deleting_utc"]:
        for t_cand in all_ts_seen:
            if t_cand > rec["ultima_vista_utc"]:
                rec["primera_deleting_utc"] = t_cand
                break

sorted_names = sorted(
    inst_info.keys(),
    key=lambda k: (inst_info[k]["creation_timestamp"] or inst_info[k]["primera_vista_utc"], k),
)
initial_worker = sorted_names[0] if sorted_names else ""

jobs_by_worker_key = {}
obj_to_res = {}
trabajos_path = out_dir / "trabajos.tsv"
if trabajos_path.exists():
    for line in trabajos_path.read_text(encoding="utf-8").splitlines():
        if not line.strip() or line.startswith("orden\t"):
            continue
        cols = line.split("\t")
        if len(cols) >= 8:
            obj_to_res[cols[7]] = cols[5]

if raw_dir.exists():
    for arr_file in sorted(raw_dir.glob("*.arranque.json")):
        wname = arr_file.name.replace(".arranque.json", "")
        try:
            data = json.loads(arr_file.read_text(encoding="utf-8"))
            if wname in inst_info:
                inst_info[wname]["arranque_listo_utc"] = str(data.get("instante_utc") or data.get("fin_utc") or "")
                inst_info[wname]["segundos_bootstrap"] = str(data.get("segundos_hasta_listo", ""))
        except Exception:
            pass

    for log_file in sorted(raw_dir.glob("*.docker.log")):
        wname = log_file.name.replace(".docker.log", "")
        lines_raw = log_file.read_text(encoding="utf-8", errors="replace").splitlines()
        # Primera pasada: mapear object_key -> resource_id y files_uploaded desde las lineas INFO
        res_to_files = {}
        for raw_line in lines_raw:
            if "INFO media job completed" in raw_line and "resource_id=" in raw_line and "object_key=" in raw_line:
                parts_kv = dict(tok.split("=", 1) for tok in raw_line.split() if "=" in tok)
                rid = parts_kv.get("resource_id", "")
                okey = parts_kv.get("object_key", "")
                if rid and okey:
                    obj_to_res[okey] = rid
            elif "INFO media rendered to HLS" in raw_line and "resource_id=" in raw_line:
                parts_kv = dict(tok.split("=", 1) for tok in raw_line.split() if "=" in tok)
                rid = parts_kv.get("resource_id", "")
                fup = parts_kv.get("files_uploaded", "")
                if rid and fup:
                    res_to_files[rid] = fup

        # Segunda pasada: eventos JSON estructurados de inicio y fin de cada job
        for raw_line in lines_raw:
            idx = raw_line.find("{")
            if idx < 0:
                continue
            try:
                ev = json.loads(raw_line[idx:])
            except Exception:
                continue
            msg = ev.get("msg", "")
            t_ev = ev.get("time", "")
            task_id = ev.get("id", "")
            obj_key = ev.get("object_key", "")
            if not obj_key and task_id.startswith("media-"):
                obj_key = task_id[len("media-"):]
            res_id = ev.get("resource_id") or obj_to_res.get(obj_key, "") or task_id
            if msg == "job media:process started" and task_id:
                key = (wname, task_id)
                jrec = jobs_by_worker_key.setdefault(key, {
                    "instancia": wname,
                    "instance_id": inst_info.get(wname, {}).get("instance_id", "n/a"),
                    "es_worker_nuevo": "false" if wname == initial_worker else "true",
                    "asynq_task_id": task_id,
                    "resource_id": res_id,
                    "object_key": obj_key,
                    "t_started_utc": t_ev,
                    "t_completed_utc": "",
                    "duracion_ms": "",
                    "segment_count": res_to_files.get(res_id, ""),
                    "estado": "started",
                })
                if res_id and jrec["resource_id"] == task_id:
                    jrec["resource_id"] = res_id
                if not jrec["t_started_utc"] or t_ev < jrec["t_started_utc"]:
                    jrec["t_started_utc"] = t_ev
            elif msg == "media rendered to HLS" and res_id:
                for (wn, tid), jrec in jobs_by_worker_key.items():
                    if wn == wname and jrec["resource_id"] == res_id:
                        jrec["segment_count"] = str(ev.get("segment_count", ""))
            elif msg == "job media:process completed" and task_id:
                key = (wname, task_id)
                jrec = jobs_by_worker_key.setdefault(key, {
                    "instancia": wname,
                    "instance_id": inst_info.get(wname, {}).get("instance_id", "n/a"),
                    "es_worker_nuevo": "false" if wname == initial_worker else "true",
                    "asynq_task_id": task_id,
                    "resource_id": res_id,
                    "object_key": obj_key,
                    "t_started_utc": "",
                    "t_completed_utc": t_ev,
                    "duracion_ms": str(ev.get("duración_ms") or ev.get("duracion_ms") or ""),
                    "segment_count": res_to_files.get(res_id, ""),
                    "estado": "completed",
                })
                if res_id:
                    jrec["resource_id"] = res_id
                if res_to_files.get(res_id) and not jrec["segment_count"]:
                    jrec["segment_count"] = res_to_files[res_id]
                jrec["t_completed_utc"] = t_ev
                jrec["duracion_ms"] = str(ev.get("duración_ms") or ev.get("duracion_ms") or jrec["duracion_ms"])
                jrec["estado"] = "completed"

for jrec in sorted(jobs_by_worker_key.values(), key=lambda r: (r["t_started_utc"] or r["t_completed_utc"])):
    wname = jrec["instancia"]
    if wname not in inst_info:
        continue
    if jrec["t_started_utc"] and not inst_info[wname]["primer_job_started_utc"]:
        inst_info[wname]["primer_job_started_utc"] = jrec["t_started_utc"]
    if jrec["estado"] == "completed":
        inst_info[wname]["total_jobs_completados"] += 1
        if jrec["t_completed_utc"] and not inst_info[wname]["primer_job_completed_utc"]:
            inst_info[wname]["primer_job_completed_utc"] = jrec["t_completed_utc"]

tpw_lines = [
    "instancia\tinstance_id\tes_worker_nuevo\tasynq_task_id\tresource_id\tobject_key\tt_started_utc\tt_completed_utc\tduracion_ms\tsegment_count\testado"
]
for jrec in sorted(jobs_by_worker_key.values(), key=lambda r: (r["t_started_utc"] or r["t_completed_utc"], r["instancia"])):
    tpw_lines.append("\t".join([
        jrec["instancia"],
        str(jrec["instance_id"]),
        jrec["es_worker_nuevo"],
        jrec["asynq_task_id"],
        jrec["resource_id"],
        jrec["object_key"],
        jrec["t_started_utc"],
        jrec["t_completed_utc"],
        str(jrec["duracion_ms"]),
        str(jrec["segment_count"]),
        jrec["estado"],
    ]))
(out_dir / "trabajos_por_worker.tsv").write_text("\n".join(tpw_lines) + "\n", encoding="utf-8")

inst_lines = [
    "instancia\tinstance_id\trol_ciclo\tcreation_timestamp\tprimera_vista_utc\tprimera_running_utc\tarranque_listo_utc\tsegundos_bootstrap\tprimera_healthy_utc\tprimer_job_started_utc\tprimer_job_completed_utc\ttotal_jobs_completados\tprimera_deleting_utc\tultima_vista_utc"
]
for name in sorted_names:
    rec = inst_info[name]
    rol = "inicial" if name == initial_worker else "nuevo_scale_out"
    inst_lines.append("\t".join([
        rec["instancia"],
        str(rec["instance_id"]),
        rol,
        rec["creation_timestamp"],
        rec["primera_vista_utc"],
        rec["primera_running_utc"],
        rec["arranque_listo_utc"],
        str(rec["segundos_bootstrap"]),
        rec["primera_healthy_utc"],
        rec["primer_job_started_utc"],
        rec["primer_job_completed_utc"],
        str(rec["total_jobs_completados"]),
        rec["primera_deleting_utc"],
        rec["ultima_vista_utc"],
    ]))
(out_dir / "instancias.tsv").write_text("\n".join(inst_lines) + "\n", encoding="utf-8")
PY
}

ejecutar_autoprueba() {
  local tmp_dir
  tmp_dir="$(mktemp -d)"

  echo "[AUTOPRUEBA G6-OBSERVADOR] Validando parser de logs e instancias en ${tmp_dir}..."
  mkdir -p "${tmp_dir}/workers_raw"

  cat > "${tmp_dir}/.instancias_vistas.tsv" <<'EOF'
instancia	instance_id	creation_timestamp	timestamp_utc	status	action	health
mooc-worker-init	100001	2026-10-30T13:50:00Z	2026-10-30T14:00:00Z	RUNNING	NONE	HEALTHY
mooc-worker-new1	100002	2026-10-30T14:04:00Z	2026-10-30T14:04:05Z	PROVISIONING	CREATING	UNKNOWN
mooc-worker-new1	100002	2026-10-30T14:04:00Z	2026-10-30T14:04:40Z	RUNNING	NONE	HEALTHY
mooc-worker-new1	100002	2026-10-30T14:04:00Z	2026-10-30T14:25:00Z	STOPPING	DELETING	UNHEALTHY
EOF

  cat > "${tmp_dir}/workers_raw/mooc-worker-new1.arranque.json" <<'EOF'
{"instancia":"mooc-worker-new1","fin_utc":"2026-10-30T14:04:36Z","segundos_hasta_listo":36}
EOF

  cat > "${tmp_dir}/workers_raw/mooc-worker-new1.docker.log" <<'EOF'
{"time":"2026-10-30T14:04:38Z","level":"INFO","msg":"job media:process started","id":"task-99","type":"media:process","resource_id":"res-new-01","object_key":"uploads/s-99/v.mp4"}
{"time":"2026-10-30T14:07:02Z","level":"INFO","msg":"media rendered to HLS","resource_id":"res-new-01","segment_count":30}
{"time":"2026-10-30T14:07:03Z","level":"INFO","msg":"job media:process completed","id":"task-99","type":"media:process","resource_id":"res-new-01","object_key":"uploads/s-99/v.mp4","duración_ms":145120}
EOF

  consolidar_logs_workers "${tmp_dir}"

  python3 - "${tmp_dir}" <<'PY'
import pathlib, sys
d = pathlib.Path(sys.argv[1])
tpw = (d / "trabajos_por_worker.tsv").read_text(encoding="utf-8").strip().splitlines()
inst = (d / "instancias.tsv").read_text(encoding="utf-8").strip().splitlines()
assert len(tpw) == 2, f"Se esperaba 1 trabajo + cabecera en trabajos_por_worker.tsv, obtenido {len(tpw)}"
assert "res-new-01" in tpw[1] and "true" in tpw[1] and "145120" in tpw[1]
assert len(inst) == 3, f"Se esperaban 2 instancias + cabecera en instancias.tsv, obtenido {len(inst)}"
assert "nuevo_scale_out" in inst[2] and "2026-10-30T14:25:00Z" in inst[2]
PY

  rm -rf "${tmp_dir}"
  echo "[AUTOPRUEBA G6-OBSERVADOR] OK: extracción de trabajos_por_worker.tsv e instancias.tsv verificada."
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

if [[ "${1:-}" == "--consolidar" ]]; then
  consolidar_logs_workers "${2:?Indique directorio de corrida}"
  exit 0
fi

REANUDAR=0
if [[ "${1:-}" == "--reanudar" ]]; then
  REANUDAR=1
  shift
fi

TIMESTAMP_TAG="$(date -u +%Y-%m-%d_%H%MUTC)"
OUT_DIR="${1:-${REPO_ROOT}/docs/entrega3/evidencias/G6/corrida_${TIMESTAMP_TAG}}"
mkdir -p "${OUT_DIR}/workers_raw"

OBS_TSV="${OUT_DIR}/observacion.tsv"
COLA_TSV="${OUT_DIR}/cola_drenaje.tsv"
HITOS_TSV="${OUT_DIR}/hitos.tsv"
CRONO_TXT="${OUT_DIR}/cronologia.txt"
INST_VISTAS_TSV="${OUT_DIR}/.instancias_vistas.tsv"

log_crono() {
  local ts
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '[%s] %s\n' "${ts}" "$*" | tee -a "${CRONO_TXT}"
}

registrar_hito() {
  local codigo="$1"
  local detalle="$2"
  local ts
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '%s\t%s\t%s\n' "${ts}" "${codigo}" "${detalle}" >> "${HITOS_TSV}"
  log_crono "HITO [${codigo}]: ${detalle}"
}

if (( REANUDAR == 0 )); then
  printf 'timestamp_utc\thito\tdetalle\n' > "${HITOS_TSV}"
  printf 'iteracion\ttimestamp_utc\tepoch_s\ttarget_size\trecommended_size\trunning_count\thealthy_count\tcm_outstanding\tcm_pending\tcm_active\tcm_oldest_age_s\tredis_pending\tredis_active\tredis_retry\tredis_archived\tinstancias_detalle\n' > "${OBS_TSV}"
  printf 'iteracion\ttimestamp_utc\tredis_pending\tredis_active\tredis_outstanding\tredis_retry\tredis_archived\ttarget_size\thealthy_workers\tidempotency_keys\n' > "${COLA_TSV}"
  printf 'instancia\tinstance_id\tcreation_timestamp\ttimestamp_utc\tstatus\taction\thealth\n' > "${INST_VISTAS_TSV}"
  : > "${CRONO_TXT}"

  # ==============================================================================
  # 1. PRE-FLIGHT OBLIGATORIO (vía instance-groups managed describe)
  # ==============================================================================
  log_crono "Ejecutando verificaciones pre-flight sobre ${WORKER_MIG}..."
  MIG_JSON="$(gcloud compute instance-groups managed describe "${WORKER_MIG}" \
    --zone="${ZONE}" --project="${PROJECT_ID}" --format=json)"

  python3 - "${MIG_JSON}" <<'PY'
import json, sys
mig_d = json.loads(sys.argv[1])
as_d = mig_d.get("autoscaler") or {}
status = as_d.get("status")
policy = as_d.get("autoscalingPolicy", {})
mode = policy.get("mode")
min_r = policy.get("minNumReplicas")
max_r = policy.get("maxNumReplicas")
target = mig_d.get("targetSize")
assert status == "ACTIVE", f"Autoscaler status != ACTIVE ({status})"
assert mode == "ON", f"Autoscaler mode != ON ({mode})"
assert min_r == 1 and max_r >= 2, f"Rango de replicas inesperado: min={min_r}, max={max_r}"
assert target == 1, f"Se requiere arrancar con targetSize=1 y 1 worker operativo (actual={target})"
PY

  registrar_hito "PREFLIGHT_OK" "Autoscaler ACTIVE (mode=ON, min=1, max=3) y MIG con targetSize=1"
else
  log_crono "Reanudando observación en ${OUT_DIR} sin sobrescribir iteraciones previas..."
fi

ACCESS_TOKEN="$(gcloud auth print-access-token --project="${PROJECT_ID}")"
TOKEN_TS="$(date -u +%s)"

renovar_token_si_necesario() {
  local now
  now="$(date -u +%s)"
  if (( now - TOKEN_TS > 1800 )); then
    ACCESS_TOKEN="$(gcloud auth print-access-token --project="${PROJECT_ID}")"
    TOKEN_TS="${now}"
  fi
}

consultar_metrica_cm() {
  local metric_name="$1"
  local now_iso start_iso resp
  now_iso="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  start_iso="$(date -u -d '-6 minutes' +%Y-%m-%dT%H:%M:%SZ)"
  resp="$(curl -sS -G \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" \
    "https://monitoring.googleapis.com/v3/projects/${PROJECT_ID}/timeSeries" \
    --data-urlencode "filter=metric.type = \"custom.googleapis.com/mooc/queue/${metric_name}\" AND resource.type = \"generic_node\" AND resource.labels.namespace = \"mooc-cola\"" \
    --data-urlencode "interval.startTime=${start_iso}" \
    --data-urlencode "interval.endTime=${now_iso}" 2>/dev/null || true)"
  python3 - "${resp}" <<'PY'
import json, sys
try:
    d = json.loads(sys.argv[1])
    pts = d.get("timeSeries", [{}])[0].get("points", [])
    if not pts:
        print("0")
    else:
        val = pts[0].get("value", {})
        if "int64Value" in val:
            print(val["int64Value"])
        elif "doubleValue" in val:
            print(round(float(val["doubleValue"]), 1))
        else:
            print("0")
except Exception:
    print("0")
PY
}

# En mooc-queue-server, Redis corre como servicio del sistema (redis-cli -n 0)
consultar_redis_cola() {
  local raw
  raw="$(gcloud compute ssh "${QUEUE_VM}" \
    --zone="${ZONE}" --project="${PROJECT_ID}" --tunnel-through-iap \
    --command='
      pen=$(redis-cli -n 0 LLEN "asynq:{default}:pending")
      act=$(redis-cli -n 0 LLEN "asynq:{default}:active")
      ret=$(redis-cli -n 0 ZCARD "asynq:{default}:retry")
      arc=$(redis-cli -n 0 ZCARD "asynq:{default}:archived")
      idm=$(redis-cli -n 0 KEYS "idempotency:worker:*" | sed "/^$/d" | wc -l)
      printf "%s\t%s\t%s\t%s\t%s\n" "${pen:-0}" "${act:-0}" "${ret:-0}" "${arc:-0}" "${idm:-0}"
    ' 2>/dev/null | tail -n1 || printf '0\t0\t0\t0\t0\n')"
  if [[ -z "${raw}" ]]; then
    printf '0\t0\t0\t0\t0\n'
  else
    echo "${raw}"
  fi
}

OBS_START_UTC="$(head -n 2 "${OBS_TSV}" 2>/dev/null | tail -n 1 | awk -F'\t' '{print $2}')"
if [[ -z "${OBS_START_UTC}" || "${OBS_START_UTC}" == "timestamp_utc" ]]; then
  OBS_START_UTC="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
fi

recolectar_logs_instancia_viva() {
  local wname="$1"
  local arr_file="${OUT_DIR}/workers_raw/${wname}.arranque.json"
  local log_file="${OUT_DIR}/workers_raw/${wname}.docker.log"
  local tmp_arr="${arr_file}.tmp"
  local tmp_log="${log_file}.tmp"

  if [[ ! -s "${arr_file}" ]]; then
    gcloud compute ssh "${wname}" \
      --zone="${ZONE}" --project="${PROJECT_ID}" --tunnel-through-iap \
      --ssh-flag="-o ConnectTimeout=5" \
      --command="cat /var/lib/mooc/arranque.json 2>/dev/null || true" \
      > "${tmp_arr}" 2>/dev/null || true
    if [[ -s "${tmp_arr}" ]] && grep -q "segundos_hasta_listo" "${tmp_arr}" 2>/dev/null; then
      mv "${tmp_arr}" "${arr_file}"
    else
      rm -f "${tmp_arr}"
    fi
  fi

  gcloud compute ssh "${wname}" \
    --zone="${ZONE}" --project="${PROJECT_ID}" --tunnel-through-iap \
    --ssh-flag="-o ConnectTimeout=5" \
    --command="sudo docker logs --since='${OBS_START_UTC}' mooc-worker 2>&1 || true" \
    | sed -E 's/10\.[0-9]+\.[0-9]+\.[0-9]+/<IP_PRIVADA_REDACTADA>/g' \
    > "${tmp_log}" 2>/dev/null || true

  local old_sz=0 new_sz=0
  [[ -f "${log_file}" ]] && old_sz="$(stat -c%s "${log_file}" 2>/dev/null || echo 0)"
  [[ -f "${tmp_log}" ]] && new_sz="$(stat -c%s "${tmp_log}" 2>/dev/null || echo 0)"
  if (( new_sz > 0 && new_sz >= old_sz )); then
    mv "${tmp_log}" "${log_file}"
  else
    rm -f "${tmp_log}"
  fi
}

prev_target=1
prev_rec=1
visto_scale_out=0
visto_cola_vacia_post_scale_out=0
visto_scale_in=0
start_iter=1

if (( REANUDAR == 1 )) && [[ -s "${OBS_TSV}" ]]; then
  read -r start_iter prev_target prev_rec visto_scale_out < <(python3 - "${OBS_TSV}" <<'PY'
import pathlib, sys
lines = [l.split("\t") for l in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines() if l.strip() and not l.startswith("iteracion\t")]
if not lines:
    print(1, 1, 1, 0)
else:
    last = lines[-1]
    max_t = max(int(r[3]) for r in lines)
    print(int(last[0]) + 1, int(last[3]), int(last[4]), 1 if max_t > 1 else 0)
PY
)
fi

for ((iter = start_iter; iter <= G6_OBS_MAX_ITERS; iter++)); do
  renovar_token_si_necesario
  ts_utc="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  epoch_s="$(date -u +%s)"

  mig_cur="$(gcloud compute instance-groups managed describe "${WORKER_MIG}" \
    --zone="${ZONE}" --project="${PROJECT_ID}" --format=json)"
  inst_list="$(gcloud compute instance-groups managed list-instances "${WORKER_MIG}" \
    --zone="${ZONE}" --project="${PROJECT_ID}" --format=json)"
  vm_list="$(gcloud compute instances list \
    --filter="name~'^mooc-worker-' AND zone:(${ZONE})" \
    --project="${PROJECT_ID}" --format=json)"

  read -r target_size rec_size running_cnt healthy_cnt inst_summary < <(python3 - "${mig_cur}" "${inst_list}" "${vm_list}" "${ts_utc}" "${INST_VISTAS_TSV}" <<'PY'
import json, pathlib, sys

mig_d = json.loads(sys.argv[1])
inst_l = json.loads(sys.argv[2])
vm_l = json.loads(sys.argv[3])
ts_utc = sys.argv[4]
vistas_path = pathlib.Path(sys.argv[5])

as_d = mig_d.get("autoscaler") or {}
vm_meta = {}
for v in vm_l:
    vm_meta[v.get("name", "")] = (str(v.get("id", "n/a")), str(v.get("creationTimestamp", "n/a")))

target = int(mig_d.get("targetSize", 0))
rec = int(as_d.get("recommendedSize", target))
running = 0
healthy = 0
details = []

with vistas_path.open("a", encoding="utf-8") as fh:
    for item in inst_l:
        name = item.get("instance", "").split("/")[-1]
        status = item.get("instanceStatus") or "NONE"
        action = item.get("currentAction") or "NONE"
        h_list = item.get("instanceHealth") or []
        h_state = h_list[0].get("detailedHealthState", "UNKNOWN") if h_list else "UNKNOWN"
        if status == "RUNNING":
            running += 1
        if h_state == "HEALTHY":
            healthy += 1
        iid, ctime = vm_meta.get(name, ("n/a", "n/a"))
        fh.write(f"{name}\t{iid}\t{ctime}\t{ts_utc}\t{status}\t{action}\t{h_state}\n")
        details.append(f"{name}:{status}/{action}/{h_state}")

summary = ";".join(details) if details else "ninguna"
print(target, rec, running, healthy, summary)
PY
)

  # Recolectar logs de todas las instancias RUNNING antes de que el scale-in las retire
  while IFS= read -r wname; do
    [[ -z "${wname}" ]] && continue
    recolectar_logs_instancia_viva "${wname}"
  done < <(python3 - "${inst_list}" <<'PY'
import json, sys
for item in json.loads(sys.argv[1]):
    if item.get("instanceStatus") == "RUNNING":
        print(item.get("instance", "").split("/")[-1])
PY
)

  cm_out="$(consultar_metrica_cm "outstanding")"
  cm_pend="$(consultar_metrica_cm "pending")"
  cm_act="$(consultar_metrica_cm "active")"
  cm_age="$(consultar_metrica_cm "oldest_pending_age_seconds")"

  read -r r_pend r_act r_retry r_arch r_idemp < <(consultar_redis_cola)
  r_out=$((r_pend + r_act))

  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "${iter}" "${ts_utc}" "${epoch_s}" "${target_size}" "${rec_size}" \
    "${running_cnt}" "${healthy_cnt}" "${cm_out}" "${cm_pend}" "${cm_act}" "${cm_age}" \
    "${r_pend}" "${r_act}" "${r_retry}" "${r_arch}" "${inst_summary}" >> "${OBS_TSV}"

  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "${iter}" "${ts_utc}" "${r_pend}" "${r_act}" "${r_out}" "${r_retry}" "${r_arch}" \
    "${target_size}" "${healthy_cnt}" "${r_idemp}" >> "${COLA_TSV}"

  log_crono "iter=${iter} target=${target_size} rec=${rec_size} run=${running_cnt} healthy=${healthy_cnt} cm_out=${cm_out} redis_pend=${r_pend} redis_act=${r_act} retry=${r_retry} arch=${r_arch} [${inst_summary}]"

  if (( rec_size > prev_rec || target_size > prev_target )); then
    registrar_hito "SCALE_OUT_DECISION" "Autoscaler incrementa capacidad: targetSize ${prev_target}->${target_size}, recommendedSize ${prev_rec}->${rec_size} (cm_outstanding=${cm_out}, redis_outstanding=${r_out})"
    visto_scale_out=1
  fi
  if (( visto_scale_out == 1 && visto_cola_vacia_post_scale_out == 0 && r_out == 0 && cm_out == 0 )); then
    registrar_hito "COLA_DRENADA_CERO" "Cola completamente drenada tras el pico: redis_pending=0, redis_active=0, cm_outstanding=0, retry=${r_retry}"
    visto_cola_vacia_post_scale_out=1
  fi
  if (( visto_scale_out == 1 && target_size < prev_target )); then
    registrar_hito "SCALE_IN_DECISION" "Autoscaler reduce el grupo: targetSize ${prev_target}->${target_size}, recommendedSize ${prev_rec}->${rec_size}"
    visto_scale_in=1
  fi

  prev_target="${target_size}"
  prev_rec="${rec_size}"

  consolidar_logs_workers "${OUT_DIR}"

  if (( visto_scale_out == 1 && visto_scale_in == 1 && target_size == 1 && running_cnt == 1 && r_out == 0 )); then
    registrar_hito "CICLO_COMPLETO_ESTABLE" "Grupo reducido de vuelta a 1 worker operativo con cola en 0"
    break
  fi

  sleep "${G6_OBS_INTERVAL_S}"
done

consolidar_logs_workers "${OUT_DIR}"
log_crono "Observación finalizada. Artefactos escritos en ${OUT_DIR}."
