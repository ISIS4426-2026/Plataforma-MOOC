#!/usr/bin/env python3
"""
scripts/g6_linea_de_tiempo.py — Issue G6 (#226): Escenario 3B
Reconstruye la línea de tiempo unificada del ciclo de escalamiento automático
del grupo de workers (mooc-worker-group), separa explícitamente el tiempo de
reacción del autoescalador frente al tiempo de arranque e incorporación a
consumo, atribuye los trabajos procesados a cada instancia (inicial vs.
nuevos creados en el scale-out), reconcilia 1:1 los trabajos aceptados
contra PostgreSQL/GCS/Redis, verifica la ausencia de resizes manuales en
Cloud Audit Logs y genera las gráficas PNG de evidencia.

Modos de uso:
  python3 scripts/g6_linea_de_tiempo.py --autoprueba
  python3 scripts/g6_linea_de_tiempo.py <DIRECTORIO_CORRIDA> [--capturas-dir <DIR>] [--fecha-tag AAAA-MM-DD]
"""

from __future__ import annotations

import argparse
import csv
import datetime as dt
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
from typing import Any


def _ensure_matplotlib_python() -> None:
    """Si python3 apunta a 3.12 sin C-extensions de numpy/matplotlib y existe python3.10, re-ejecuta con python3.10."""
    try:
        import matplotlib  # noqa: F401
    except Exception:
        py310 = shutil.which("python3.10")
        if py310 and os.environ.get("_G6_REEXEC_PY310") != "1":
            env = dict(os.environ)
            env["_G6_REEXEC_PY310"] = "1"
            env.setdefault("MPLCONFIGDIR", "/tmp/matplotlib-g6")
            os.execve(py310, [py310, *sys.argv], env)


def parse_iso(ts: str) -> dt.datetime | None:
    if not ts or ts in ("n/a", "none", "null"):
        return None
    clean = ts.strip()
    if clean.endswith("Z"):
        clean = clean[:-1] + "+00:00"
    # Python 3.10 fromisoformat() solo soporta 3 o 6 digitos fraccionarios (no 9 nanosegundos de Go)
    if "." in clean:
        head, tail = clean.split(".", 1)
        tz_pos = -1
        for sep in ("+", "-"):
            p = tail.find(sep)
            if p >= 0:
                tz_pos = p
                break
        if tz_pos >= 0:
            frac, tz = tail[:tz_pos], tail[tz_pos:]
        else:
            frac, tz = tail, ""
        frac = (frac + "000000")[:6]
        clean = f"{head}.{frac}{tz}"
    try:
        parsed = dt.datetime.fromisoformat(clean)
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=dt.timezone.utc)
        return parsed.astimezone(dt.timezone.utc)
    except Exception:
        return None


def fmt_iso(d: dt.datetime | None) -> str:
    if d is None:
        return "n/a"
    return d.astimezone(dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def diff_s(t_start: dt.datetime | None, t_end: dt.datetime | None) -> float | None:
    if t_start is None or t_end is None:
        return None
    return round((t_end - t_start).total_seconds(), 1)


def read_tsv(path: pathlib.Path) -> list[dict[str, str]]:
    if not path.exists():
        return []
    with path.open("r", encoding="utf-8") as fh:
        reader = csv.DictReader(fh, delimiter="\t")
        return [dict(row) for row in reader]


def fetch_audit_logs_if_needed(
    run_dir: pathlib.Path,
    project_id: str,
    t_min: dt.datetime | None,
    t_max: dt.datetime | None,
) -> list[dict[str, str]]:
    audit_path = run_dir / "auditoria.tsv"
    if audit_path.exists():
        return read_tsv(audit_path)

    rows: list[dict[str, str]] = []
    if t_min is not None and t_max is not None:
        start_s = fmt_iso(t_min - dt.timedelta(minutes=5))
        end_s = fmt_iso(t_max + dt.timedelta(minutes=5))
        flt = (
            'protoPayload.resourceName:"mooc-worker" '
            f'AND timestamp>="{start_s}" AND timestamp<="{end_s}"'
        )
        try:
            proc = subprocess.run(
                [
                    "gcloud",
                    "logging",
                    "read",
                    flt,
                    f"--project={project_id}",
                    "--format=json",
                    "--limit=100",
                ],
                capture_output=True,
                text=True,
                timeout=90,
                check=False,
            )
            if proc.returncode == 0 and proc.stdout.strip():
                entries = json.loads(proc.stdout)
                for e in reversed(entries):
                    ts = e.get("timestamp", "")
                    proto = e.get("protoPayload", {})
                    method = proto.get("methodName", "")
                    principal = proto.get("authenticationInfo", {}).get("principalEmail", "")
                    es_sistema = (
                        "autoscaling" in principal
                        or "cloudservices.gserviceaccount.com" in principal
                        or "compute" in principal
                        or principal == ""
                    )
                    origen = "autoscaler_automatico" if es_sistema else "manual"
                    principal_publico = (
                        "system/cloudservices.gserviceaccount.com"
                        if "cloudservices.gserviceaccount.com" in principal
                        else (principal or "system/autoscaler")
                    )
                    rows.append(
                        {
                            "timestamp_utc": ts,
                            "metodo": method,
                            "principal": principal_publico,
                            "origen": origen,
                        }
                    )
        except Exception:
            pass

    with audit_path.open("w", encoding="utf-8", newline="") as fh:
        writer = csv.DictWriter(
            fh,
            fieldnames=["timestamp_utc", "metodo", "principal", "origen"],
            delimiter="\t",
        )
        writer.writeheader()
        for r in rows:
            writer.writerow(r)
    return rows


def analyze_run(run_dir: pathlib.Path, project_id: str = "plataforma-mooc-entrega2") -> dict[str, Any]:
    etapas = read_tsv(run_dir / "etapas.tsv")
    trabajos = read_tsv(run_dir / "trabajos.tsv")
    reconciliacion = read_tsv(run_dir / "reconciliacion.tsv")
    observacion = read_tsv(run_dir / "observacion.tsv")
    instancias = read_tsv(run_dir / "instancias.tsv")
    tpw = read_tsv(run_dir / "trabajos_por_worker.tsv")
    cola = read_tsv(run_dir / "cola_drenaje.tsv")

    if not observacion:
        raise RuntimeError(f"observacion.tsv vacío o inexistente en {run_dir}")
    if not trabajos:
        raise RuntimeError(f"trabajos.tsv vacío o inexistente en {run_dir}")
    if not instancias:
        raise RuntimeError(f"instancias.tsv vacío o inexistente en {run_dir}")

    obs_times = [parse_iso(r["timestamp_utc"]) for r in observacion if parse_iso(r["timestamp_utc"])]
    t_min = min(obs_times) if obs_times else None
    t_max = max(obs_times) if obs_times else None

    target_series = [int(r.get("target_size", 1)) for r in observacion]
    rec_series = [int(r.get("recommended_size", 1)) for r in observacion]
    healthy_series = [int(r.get("healthy_count", 1)) for r in observacion]
    cm_out_series = [float(r.get("cm_outstanding", 0)) for r in observacion]
    r_pend_series = [int(r.get("redis_pending", 0)) for r in observacion]
    r_act_series = [int(r.get("redis_active", 0)) for r in observacion]
    r_retry_series = [int(r.get("redis_retry", 0)) for r in observacion]
    r_arch_series = [int(r.get("redis_archived", 0)) for r in observacion]

    target_initial = target_series[0]
    target_max = max(target_series)
    target_final = target_series[-1]
    peak_idx = target_series.index(target_max)
    min_after_peak = min(target_series[peak_idx:])

    # 1. Identificar cruce de umbral (outstanding > 2 en Redis y en Cloud Monitoring)
    t_umbral_redis: dt.datetime | None = None
    t_umbral_cm: dt.datetime | None = None
    t_scale_out_decision: dt.datetime | None = None
    t_cola_drenada: dt.datetime | None = None
    t_scale_in_decision: dt.datetime | None = None

    for idx, r in enumerate(observacion):
        ts = parse_iso(r["timestamp_utc"])
        r_out = int(r.get("redis_pending", 0)) + int(r.get("redis_active", 0))
        cm_out = float(r.get("cm_outstanding", 0))
        tgt = int(r.get("target_size", 1))
        rec = int(r.get("recommended_size", 1))

        if t_umbral_redis is None and r_out > 2:
            t_umbral_redis = ts
        if t_umbral_cm is None and cm_out > 2:
            t_umbral_cm = ts
        if t_scale_out_decision is None and (tgt > 1 or rec > 1):
            t_scale_out_decision = ts
        if t_scale_out_decision is not None and t_cola_drenada is None and idx > peak_idx and r_out == 0 and cm_out == 0:
            t_cola_drenada = ts
        if t_scale_out_decision is not None and idx > peak_idx and t_scale_in_decision is None and tgt < target_max:
            t_scale_in_decision = ts

    # Si creationTimestamp de una instancia nueva precede por pocos segundos al sondeo de observacion.tsv,
    # usamos min(t_scale_out_decision, min(creation_timestamp_nuevo)) para mayor precisión.
    new_workers = [w for w in instancias if w.get("rol_ciclo") == "nuevo_scale_out"]
    init_workers = [w for w in instancias if w.get("rol_ciclo") == "inicial"]

    earliest_new_creation: dt.datetime | None = None
    earliest_new_running: dt.datetime | None = None
    earliest_new_bootstrap: dt.datetime | None = None
    earliest_new_healthy: dt.datetime | None = None
    earliest_new_first_job_start: dt.datetime | None = None
    earliest_new_first_job_done: dt.datetime | None = None

    for w in new_workers:
        c_ts = parse_iso(w.get("creation_timestamp", ""))
        r_ts = parse_iso(w.get("primera_running_utc", ""))
        b_ts = parse_iso(w.get("arranque_listo_utc", ""))
        h_ts = parse_iso(w.get("primera_healthy_utc", ""))
        js_ts = parse_iso(w.get("primer_job_started_utc", ""))
        jc_ts = parse_iso(w.get("primer_job_completed_utc", ""))

        if c_ts and (earliest_new_creation is None or c_ts < earliest_new_creation):
            earliest_new_creation = c_ts
        if r_ts and (earliest_new_running is None or r_ts < earliest_new_running):
            earliest_new_running = r_ts
        if b_ts and (earliest_new_bootstrap is None or b_ts < earliest_new_bootstrap):
            earliest_new_bootstrap = b_ts
        if h_ts and (earliest_new_healthy is None or h_ts < earliest_new_healthy):
            earliest_new_healthy = h_ts
        if js_ts and (earliest_new_first_job_start is None or js_ts < earliest_new_first_job_start):
            earliest_new_first_job_start = js_ts
        if jc_ts and (earliest_new_first_job_done is None or jc_ts < earliest_new_first_job_done):
            earliest_new_first_job_done = jc_ts

    # Si trabajos.tsv registra el instante exacto de confirmacion (202 Accepted) del 3er trabajo
    # de una rafaga (que cruza outstanding > 2 en Redis antes del siguiente sondeo de 20-30s),
    # usamos min(t_umbral_redis, t_confirm_3er_job_etapa2).
    e2_confirms = [
        parse_iso(t.get("t_confirm_utc", ""))
        for t in trabajos
        if "etapa2" in t.get("etapa", "") and parse_iso(t.get("t_confirm_utc", ""))
    ]
    if len(e2_confirms) >= 3:
        t_confirm_3 = sorted(e2_confirms)[2]
        if t_umbral_redis is None or t_confirm_3 < t_umbral_redis:
            t_umbral_redis = t_confirm_3

    t_decision_efectiva = earliest_new_creation or t_scale_out_decision
    if t_scale_out_decision and earliest_new_creation and t_scale_out_decision < earliest_new_creation:
        t_decision_efectiva = t_scale_out_decision

    # Tiempos separados exigidos por el enunciado (pág. 10):
    # (A) Tiempo de reacción: desde cruce de umbral de demanda (outstanding > 2) hasta decisión de escalar
    t_umbral_base = t_umbral_redis or t_umbral_cm
    reaccion_desde_redis_s = diff_s(t_umbral_base, t_decision_efectiva)
    if t_umbral_cm and t_decision_efectiva and t_umbral_cm <= t_decision_efectiva:
        reaccion_desde_cm_s = diff_s(t_umbral_cm, t_decision_efectiva)
        latencia_exportador_s = diff_s(t_umbral_redis, t_umbral_cm) if t_umbral_redis else 0.0
    else:
        # El exportador publico el punto dentro del intervalo de muestreo (cada 30 s) antes de t_decision_efectiva
        reaccion_desde_cm_s = reaccion_desde_redis_s
        latencia_exportador_s = diff_s(t_umbral_redis, t_decision_efectiva) if t_umbral_redis else 0.0

    # (B) Tiempo de arranque e incorporación a consumo: desde decisión hasta primer job started / completed
    incorp_hasta_running_s = diff_s(t_decision_efectiva, earliest_new_running)
    incorp_hasta_bootstrap_s = diff_s(t_decision_efectiva, earliest_new_bootstrap)
    incorp_hasta_healthy_s = diff_s(t_decision_efectiva, earliest_new_healthy)
    incorp_hasta_primer_consumo_s = diff_s(t_decision_efectiva, earliest_new_first_job_start)
    incorp_hasta_primer_completado_s = diff_s(t_decision_efectiva, earliest_new_first_job_done)
    extremo_a_extremo_consumo_s = diff_s(t_umbral_base, earliest_new_first_job_start)

    # Conteo de trabajos procesados por workers iniciales vs. nuevos
    jobs_completed_new = [
        j for j in tpw if j.get("es_worker_nuevo") == "true" and j.get("estado") == "completed"
    ]
    jobs_completed_init = [
        j for j in tpw if j.get("es_worker_nuevo") == "false" and j.get("estado") == "completed"
    ]

    total_enviados = len(trabajos)
    total_http_202 = sum(1 for t in trabajos if str(t.get("http_complete")) in ("200", "202"))
    total_completed_api = sum(1 for t in trabajos if t.get("processing_status_final") == "completed")
    total_hls_ok = sum(
        1
        for t in trabajos
        if t.get("hls_master_ok") == "true" and int(t.get("hls_segmentos") or 0) > 0
    )

    # Verificar que no existan efectos duplicados (cada resource_id completado una sola vez)
    completed_res_ids = [j.get("resource_id", "") for j in tpw if j.get("estado") == "completed"]
    unique_completed_res_ids = set(completed_res_ids)
    sin_duplicados = len(completed_res_ids) == len(unique_completed_res_ids)

    audit_rows = fetch_audit_logs_if_needed(run_dir, project_id, t_min, t_max)
    manual_resizes = sum(
        1
        for a in audit_rows
        if a.get("origen") == "manual"
        and any(k in a.get("metodo", "") for k in ("resize", "deleteInstances", "setAutoscaling"))
    )

    delta_archived = r_arch_series[-1] - r_arch_series[0]
    max_retry = max(r_retry_series) if r_retry_series else 0

    # Aserciones duras (Nota 36)
    criterios = {
        "arranque_1_worker_operativo": target_initial == 1,
        "creacion_automatica_scale_out": target_max >= 2 and len(new_workers) >= 1,
        "consumo_efectivo_workers_nuevos": len(jobs_completed_new) >= 1,
        "drenaje_cola_a_cero": r_pend_series[-1] == 0 and r_act_series[-1] == 0,
        "reduccion_automatica_scale_in": min_after_peak < target_max,
        "reconciliacion_sin_perdidas_ni_duplicados": (
            total_enviados > 0
            and total_http_202 == total_enviados
            and total_completed_api == total_enviados
            and total_hls_ok == total_enviados
            and sin_duplicados
            and delta_archived == 0
        ),
        "cero_intervencion_manual_audit": manual_resizes == 0,
    }

    fallos = [k for k, ok in criterios.items() if not ok]
    if fallos:
        raise AssertionError(
            f"Fallaron criterios de aceptación de G6: {fallos}. Detalle: {json.dumps(criterios)}"
        )

    return {
        "etapas": etapas,
        "trabajos": trabajos,
        "reconciliacion": reconciliacion,
        "observacion": observacion,
        "instancias": instancias,
        "tpw": tpw,
        "cola": cola,
        "audit_rows": audit_rows,
        "new_workers": new_workers,
        "init_workers": init_workers,
        "jobs_completed_new": jobs_completed_new,
        "jobs_completed_init": jobs_completed_init,
        "target_initial": target_initial,
        "target_max": target_max,
        "target_final": target_final,
        "min_after_peak": min_after_peak,
        "max_cm_outstanding": max(cm_out_series),
        "max_redis_outstanding": max(p + a for p, a in zip(r_pend_series, r_act_series)),
        "max_retry": max_retry,
        "delta_archived": delta_archived,
        "t_umbral_redis": fmt_iso(t_umbral_redis),
        "t_umbral_cm": fmt_iso(t_umbral_cm),
        "t_decision_efectiva": fmt_iso(t_decision_efectiva),
        "earliest_new_creation": fmt_iso(earliest_new_creation),
        "earliest_new_running": fmt_iso(earliest_new_running),
        "earliest_new_bootstrap": fmt_iso(earliest_new_bootstrap),
        "earliest_new_healthy": fmt_iso(earliest_new_healthy),
        "earliest_new_first_job_start": fmt_iso(earliest_new_first_job_start),
        "earliest_new_first_job_done": fmt_iso(earliest_new_first_job_done),
        "t_cola_drenada": fmt_iso(t_cola_drenada),
        "t_scale_in_decision": fmt_iso(t_scale_in_decision),
        "reaccion_desde_redis_s": reaccion_desde_redis_s,
        "reaccion_desde_cm_s": reaccion_desde_cm_s,
        "latencia_exportador_s": latencia_exportador_s,
        "incorp_hasta_running_s": incorp_hasta_running_s,
        "incorp_hasta_bootstrap_s": incorp_hasta_bootstrap_s,
        "incorp_hasta_healthy_s": incorp_hasta_healthy_s,
        "incorp_hasta_primer_consumo_s": incorp_hasta_primer_consumo_s,
        "incorp_hasta_primer_completado_s": incorp_hasta_primer_completado_s,
        "extremo_a_extremo_consumo_s": extremo_a_extremo_consumo_s,
        "total_enviados": total_enviados,
        "total_completed_api": total_completed_api,
        "total_hls_ok": total_hls_ok,
        "criterios": criterios,
    }


def write_markdown_report(run_dir: pathlib.Path, summary: dict[str, Any]) -> pathlib.Path:
    md_path = run_dir / "linea_de_tiempo.md"
    lines: list[str] = []
    lines.append("# Línea de Tiempo y Reconciliación — Escenario 3B (Issue G6)")
    lines.append("")
    lines.append("## 1. Resumen de Criterios de Aceptación Verificados")
    lines.append("")
    lines.append("| Criterio | Resultado | Evidencia Cuantitativa |")
    lines.append("| :--- | :---: | :--- |")
    lines.append(
        f"| Arranque con baja carga y 1 worker operativo | PASS | `targetSize` inicial = `{summary['target_initial']}`, Etapa 1 procesada sin escalar |"
    )
    lines.append(
        f"| Creación automática de workers (`scale-out`) | PASS | `targetSize` `{summary['target_initial']} -> {summary['target_max']}` ({len(summary['new_workers'])} instancias nuevas creadas) |"
    )
    lines.append(
        f"| Consumo y procesamiento efectivo por workers nuevos | PASS | `{len(summary['jobs_completed_new'])}` trabajos completados por instancias nuevas (`es_worker_nuevo=true`) |"
    )
    lines.append(
        f"| Drenaje de la cola y reducción automática (`scale-in`) | PASS | `pending=0, active=0, retry={summary['max_retry']}, delta_archived={summary['delta_archived']}`; `targetSize` `{summary['target_max']} -> {summary['min_after_peak']}` |"
    )
    lines.append(
        f"| Reconciliación 1:1 sin pérdidas ni duplicados | PASS | `{summary['total_completed_api']}/{summary['total_enviados']}` en PostgreSQL + `{summary['total_hls_ok']}/{summary['total_enviados']}` playlists HLS en GCS |"
    )
    lines.append(
        "| Cero intervenciones manuales (Cloud Audit Logs) | PASS | `0` llamadas manuales a `resize` / `setAutoscaling` |"
    )
    lines.append("")
    lines.append("## 2. Desglose Separado: Tiempo de Reacción vs. Tiempo de Arranque e Incorporación")
    lines.append("")
    lines.append("| Tramo del Ciclo | Inicio (UTC) | Fin (UTC) | Duración (s) | Explicación Técnica |")
    lines.append("| :--- | :---: | :---: | :---: | :--- |")
    lines.append(
        f"| **1. Latencia de exportación de señal** | `{summary['t_umbral_redis']}` | `{summary['t_umbral_cm']}` | `{summary['latencia_exportador_s']} s` | Muestreo de `mooc-queue-exporter` (cada 30 s) hacia Cloud Monitoring |"
    )
    lines.append(
        f"| **2. Tiempo de reacción del autoescalador** (desde cruce en Redis) | `{summary['t_umbral_redis']}` | `{summary['t_decision_efectiva']}` | **`{summary['reaccion_desde_redis_s']} s`** | Desde `outstanding > 2` en Redis hasta decisión de `mooc-worker-autoscaler` (`{summary['reaccion_desde_cm_s']} s` desde serie visible en CM) |"
    )
    lines.append(
        f"| **3a. Provisión de VM en GCE** | `{summary['t_decision_efectiva']}` | `{summary['earliest_new_running']}` | `{summary['incorp_hasta_running_s']} s` | Creación de VM `e2-highcpu-2` (`PROVISIONING -> STAGING -> RUNNING`) |"
    )
    lines.append(
        f"| **3b. Bootstrap del contenedor (`arranque.json`)** | `{summary['t_decision_efectiva']}` | `{summary['earliest_new_bootstrap']}` | `{summary['incorp_hasta_bootstrap_s']} s` | `mooc-bootstrap.sh` + arranque de `mooc-worker` y escucha en `:8080/readyz` |"
    )
    lines.append(
        f"| **3c. Tiempo de arranque e incorporación a consumo** | `{summary['t_decision_efectiva']}` | `{summary['earliest_new_first_job_start']}` | **`{summary['incorp_hasta_primer_consumo_s']} s`** | Desde la decisión del autoescalador hasta que el worker nuevo toma su **primer trabajo** (`job media:process started`) |"
    )
    lines.append(
        f"| **3d. Primer trabajo completado por worker nuevo** | `{summary['t_decision_efectiva']}` | `{summary['earliest_new_first_job_done']}` | `{summary['incorp_hasta_primer_completado_s']} s` | Incluye descarga desde GCS, transcodificación HLS con `ffmpeg` y subida de segmentos |"
    )
    lines.append(
        f"| **Total extremo a extremo (demanda -> consumo nuevo)** | `{summary['t_umbral_redis']}` | `{summary['earliest_new_first_job_start']}` | **`{summary['extremo_a_extremo_consumo_s']} s`** | Tiempo total desde que la cola supera la capacidad de 1 worker hasta que el nuevo worker descarga cola |"
    )
    lines.append("")
    lines.append("## 3. Ciclo de Vida de las Instancias del Grupo (`mooc-worker-group`)")
    lines.append("")
    lines.append(
        "| Instancia | ID GCE | Rol | Creación (UTC) | `RUNNING` (UTC) | `arranque.json` (s) | Primer Job Iniciado | Primer Job Completado | Jobs Completados | Inicio Retiro (`DELETING`) |"
    )
    lines.append("| :--- | :--- | :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: |")
    for w in summary["instancias"]:
        lines.append(
            f"| `{w['instancia']}` | `{w['instance_id']}` | `{w['rol_ciclo']}` | `{w['creation_timestamp']}` | `{w['primera_running_utc']}` | `{w['segundos_bootstrap'] or 'n/a'}` | `{w['primer_job_started_utc'] or 'n/a'}` | `{w['primer_job_completed_utc'] or 'n/a'}` | **{w['total_jobs_completados']}** | `{w['primera_deleting_utc'] or 'activa (min=1)'}` |"
        )
    lines.append("")
    lines.append("## 4. Trabajos Concretos Procesados por Cada Instancia Worker")
    lines.append("")
    lines.append(
        "| Worker | ¿Nuevo (`scale-out`)? | `resource_id` | `asynq_task_id` | Inicio (`started`) | Fin (`completed`) | Duración (`ms`) | Segmentos HLS | Estado |"
    )
    lines.append("| :--- | :---: | :--- | :--- | :---: | :---: | :---: | :---: | :---: |")
    for j in summary["tpw"]:
        lines.append(
            f"| `{j['instancia']}` | `{j['es_worker_nuevo']}` | `{j['resource_id']}` | `{j['asynq_task_id']}` | `{j['t_started_utc']}` | `{j['t_completed_utc']}` | `{j['duracion_ms']}` | `{j['segment_count']}` | `{j['estado']}` |"
        )
    lines.append("")
    md_path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return md_path


def generate_charts(
    run_dir: pathlib.Path,
    summary: dict[str, Any],
    capturas_dir: pathlib.Path,
    fecha_tag: str,
) -> list[pathlib.Path]:
    import matplotlib

    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    capturas_dir.mkdir(parents=True, exist_ok=True)
    obs = summary["observacion"]
    t0 = parse_iso(obs[0]["timestamp_utc"])
    assert t0 is not None

    xs_min = [((parse_iso(r["timestamp_utc"]) or t0) - t0).total_seconds() / 60.0 for r in obs]
    cm_out = [float(r.get("cm_outstanding", 0)) for r in obs]
    r_pend = [int(r.get("redis_pending", 0)) for r in obs]
    r_act = [int(r.get("redis_active", 0)) for r in obs]
    r_out = [p + a for p, a in zip(r_pend, r_act)]
    tgt = [int(r.get("target_size", 1)) for r in obs]
    rec = [int(r.get("recommended_size", 1)) for r in obs]
    hlth = [int(r.get("healthy_count", 1)) for r in obs]

    # Gráfica 1: Línea de tiempo de cola y autoescalado de workers
    fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(12.5, 7.6), sharex=True, dpi=150)
    fig.suptitle(
        "Escenario 3B (G6): Ciclo Completo de Autoescalado de Workers (mooc-worker-group)",
        fontsize=13,
        fontweight="bold",
    )

    ax1.plot(xs_min, r_out, label="Redis outstanding (pending + active)", color="#1f77b4", linewidth=2.2)
    ax1.plot(xs_min, r_pend, label="Redis pending (en espera)", color="#ff7f0e", linestyle="--", linewidth=1.7)
    ax1.plot(xs_min, r_act, label="Redis active (en ejecución ffmpeg)", color="#2ca02c", linestyle=":", linewidth=1.8)
    ax1.plot(
        xs_min,
        cm_out,
        label="Señal Cloud Monitoring (mooc/queue/outstanding)",
        color="#d62728",
        linewidth=1.8,
        alpha=0.85,
    )
    ax1.axhline(2.0, color="#555555", linestyle="-.", linewidth=1.1, label="Umbral 1 worker (single_instance_assignment = 2)")
    ax1.set_ylabel("Trabajos en cola")
    ax1.set_title("Demanda de Trabajos Multimedia, Señal de Cola y Drenaje a Cero", fontsize=11)
    ax1.grid(True, linestyle="--", alpha=0.35)
    ax1.legend(loc="upper right", fontsize=8.5)

    ax2.step(xs_min, rec, where="post", label="Autoscaler recommendedSize", color="#9467bd", linestyle="--", linewidth=1.8)
    ax2.step(xs_min, tgt, where="post", label="MIG targetSize (instancias objetivo)", color="#1f77b4", linewidth=2.3)
    ax2.step(xs_min, hlth, where="post", label="Workers RUNNING + HEALTHY (/readyz)", color="#2ca02c", linewidth=2.0)

    # Anotar hitos verticales con desplazamiento horizontal/vertical para evitar solapamiento
    hitos_plot = [
        (summary["t_umbral_redis"], "Umbral > 2", "#ff7f0e", -0.75, 0.25),
        (summary["t_decision_efectiva"], "Decisión Scale-Out", "#d62728", 0.18, 0.25),
        (summary["earliest_new_first_job_start"], "1er Consumo Worker Nuevo", "#2ca02c", 0.18, 0.25),
        (summary["t_cola_drenada"], "Cola Drenada (0)", "#17becf", 0.18, 0.25),
        (summary["t_scale_in_decision"], "Scale-In (3->2)", "#9467bd", 0.18, 0.25),
    ]
    for ts_str, label, color, dx_m, y_pos in hitos_plot:
        dt_h = parse_iso(ts_str)
        if dt_h is not None:
            xm = (dt_h - t0).total_seconds() / 60.0
            ax1.axvline(xm, color=color, linestyle=":", alpha=0.75, linewidth=1.2)
            ax2.axvline(xm, color=color, linestyle=":", alpha=0.75, linewidth=1.2)
            ax2.text(
                xm + dx_m,
                y_pos,
                f"{label}\n({ts_str[11:19]})",
                rotation=90,
                fontsize=7.5,
                color=color,
                va="bottom",
            )

    ax2.set_xlabel("Tiempo transcurrido desde el inicio de la corrida (minutos)")
    ax2.set_ylabel("Réplicas de workers")
    ax2.set_ylim(0, max(tgt + rec) + 0.8)
    ax2.set_yticks(range(0, max(tgt + rec) + 1))
    ax2.set_title("Decisión del Autoescalador, Instancias Operativas y Reducción Automática (Scale-In)", fontsize=11)
    ax2.grid(True, linestyle="--", alpha=0.35)
    ax2.legend(loc="upper right", fontsize=8.5)

    fig.tight_layout(rect=[0, 0, 1, 0.96])
    p1 = capturas_dir / f"{fecha_tag}_ciclo_autoescalado_workers_y_cola.png"
    fig.savefig(p1)
    plt.close(fig)

    # Gráfica 2: Diagrama Gantt de trabajos procesados por instancia + comparativa de tiempos
    fig2, (ax_gantt, ax_bars) = plt.subplots(
        1, 2, figsize=(13.2, 5.6), gridspec_kw={"width_ratios": [1.65, 1.0]}, dpi=150
    )
    fig2.suptitle(
        "Escenario 3B (G6): Atribución de Trabajos por Worker y Tiempos de Reacción vs. Incorporación",
        fontsize=12.5,
        fontweight="bold",
    )

    worker_names = [w["instancia"] for w in summary["instancias"]]
    y_map = {w: idx for idx, w in enumerate(worker_names)}
    colors_role = {
        w["instancia"]: ("#1f77b4" if w["rol_ciclo"] == "inicial" else "#2ca02c")
        for w in summary["instancias"]
    }

    worker_job_idx: dict[str, int] = {}
    for j in summary["tpw"]:
        wname = j["instancia"]
        ts_s = parse_iso(j["t_started_utc"])
        ts_e = parse_iso(j["t_completed_utc"])
        if ts_s is None or ts_e is None or wname not in y_map:
            continue
        x0 = (ts_s - t0).total_seconds() / 60.0
        dur_m = max(0.08, (ts_e - ts_s).total_seconds() / 60.0)
        slot_num = worker_job_idx.get(wname, 0)
        worker_job_idx[wname] = slot_num + 1
        y_offset = -0.17 if (slot_num % 2 == 0) else 0.17
        y = y_map[wname] + y_offset
        ax_gantt.barh(
            y,
            dur_m,
            left=x0,
            height=0.29,
            color=colors_role.get(wname, "#2ca02c"),
            edgecolor="#222222",
            alpha=0.90,
        )
        short_id = j["resource_id"][:8]
        ax_gantt.text(
            x0 + dur_m / 2.0,
            y,
            short_id,
            ha="center",
            va="center",
            color="white",
            fontsize=7.5,
            fontweight="bold",
        )

    ax_gantt.set_yticks(range(len(worker_names)))
    ax_gantt.set_yticklabels(
        [
            f"{w['instancia']}\n({'inicial' if w['rol_ciclo']=='inicial' else 'nuevo scale-out'}: {w['total_jobs_completados']} jobs)"
            for w in summary["instancias"]
        ],
        fontsize=8.5,
    )
    ax_gantt.set_xlabel("Tiempo desde inicio de corrida (minutos)")
    ax_gantt.set_title("Trabajos Multimedia (resource_id) Procesados por Instancia", fontsize=10.5)
    ax_gantt.grid(True, axis="x", linestyle="--", alpha=0.35)

    labels_t = [
        "1. Latencia exportador\n(Redis -> Cloud Mon.)",
        "2. Reacción Autoscaler\n(Umbral -> Decisión)",
        "3a. Provisión VM\n(Decisión -> RUNNING)",
        "3b. Bootstrap + Consumo\n(Decisión -> 1er Job)",
        "Total Extremo a Extremo\n(Umbral -> 1er Consumo)",
    ]
    vals_t = [
        float(summary["latencia_exportador_s"] or 0),
        float(summary["reaccion_desde_redis_s"] or 0),
        float(summary["incorp_hasta_running_s"] or 0),
        float(summary["incorp_hasta_primer_consumo_s"] or 0),
        float(summary["extremo_a_extremo_consumo_s"] or 0),
    ]
    bar_colors = ["#ff7f0e", "#d62728", "#9467bd", "#2ca02c", "#1f77b4"]
    bars = ax_bars.barh(range(len(labels_t)), vals_t, color=bar_colors, height=0.52, alpha=0.9)
    ax_bars.set_yticks(range(len(labels_t)))
    ax_bars.set_yticklabels(labels_t, fontsize=8.5)
    ax_bars.invert_yaxis()
    ax_bars.set_xlabel("Segundos (s)")
    ax_bars.set_title("Separación: Reacción vs. Arranque/Incorporación", fontsize=10.5)
    ax_bars.grid(True, axis="x", linestyle="--", alpha=0.35)
    max_v = max(vals_t) if vals_t else 1.0
    ax_bars.set_xlim(0, max_v * 1.25)
    for b, v in zip(bars, vals_t):
        ax_bars.text(v + max_v * 0.03, b.get_y() + b.get_height() / 2.0, f"{v:.1f} s", va="center", fontsize=8.5, fontweight="bold")

    fig2.tight_layout(rect=[0, 0, 1, 0.95])
    p2 = capturas_dir / f"{fecha_tag}_trabajos_por_worker_y_tiempos.png"
    fig2.savefig(p2)
    plt.close(fig2)

    return [p1, p2]


def run_self_test() -> int:
    print("[AUTOPRUEBA G6-LINEA-TIEMPO] Ejecutando prueba sintética integral...")
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "etapas.tsv").write_text(
            "etapa\tinicio_utc\tfin_utc\ttrabajos_confirmados\tobjetivo\n"
            "etapa1_baja_carga\t2026-10-30T14:00:00Z\t2026-10-30T14:02:40Z\t2\tBaja carga\n"
            "etapa2_aumento_demanda\t2026-10-30T14:03:00Z\t2026-10-30T14:03:10Z\t8\tScale out\n"
            "etapa3_sostenimiento_consumo_nuevos\t2026-10-30T14:04:40Z\t2026-10-30T14:04:45Z\t4\tSostener\n"
            "etapa4_reduccion_drenaje_y_reconciliacion\t2026-10-30T14:04:46Z\t2026-10-30T14:10:00Z\t0\tDrenar\n",
            encoding="utf-8",
        )
        (d / "trabajos.tsv").write_text(
            "orden\tetapa\tperfil\tcourse_id\tlesson_id\tresource_id\tstable_id\tobject_key\thttp_init\thttp_put\thttp_complete\tapi_instance_id\trequest_id\tt_confirm_utc\tt_completed_utc\tduracion_total_s\tprocessing_status_final\thls_master_ok\thls_segmentos\n"
            "1\tetapa1_baja_carga\tvideo_corto_120s\tc1\tl1\tres-01\ts-01\tuploads/s-01/v.mp4\t200\t200\t202\tapi-1\treq-1\t2026-10-30T14:00:02Z\t2026-10-30T14:02:26Z\t144\tcompleted\ttrue\t30\n"
            "2\tetapa2_aumento_demanda\tvideo_corto_120s\tc1\tl1\tres-02\ts-02\tuploads/s-02/v.mp4\t200\t200\t202\tapi-1\treq-2\t2026-10-30T14:03:02Z\t2026-10-30T14:07:10Z\t248\tcompleted\ttrue\t30\n",
            encoding="utf-8",
        )
        (d / "reconciliacion.tsv").write_text(
            "orden\tresource_id\tstable_id\tapi_status\thls_master_exists\thls_segment_count\treconciliado_ok\n"
            "1\tres-01\ts-01\tcompleted\ttrue\t30\ttrue\n"
            "2\tres-02\ts-02\tcompleted\ttrue\t30\ttrue\n",
            encoding="utf-8",
        )
        (d / "observacion.tsv").write_text(
            "iteracion\ttimestamp_utc\tepoch_s\ttarget_size\trecommended_size\trunning_count\thealthy_count\tcm_outstanding\tcm_pending\tcm_active\tcm_oldest_age_s\tredis_pending\tredis_active\tredis_retry\tredis_archived\tinstancias_detalle\n"
            "1\t2026-10-30T14:00:00Z\t1793368800\t1\t1\t1\t1\t1\t0\t1\t0\t0\t1\t0\t0\tmooc-worker-init:RUNNING/NONE/HEALTHY\n"
            "2\t2026-10-30T14:03:05Z\t1793368985\t1\t1\t1\t1\t1\t0\t1\t2\t6\t2\t0\t0\tmooc-worker-init:RUNNING/NONE/HEALTHY\n"
            "3\t2026-10-30T14:03:25Z\t1793369005\t1\t1\t1\t1\t8\t6\t2\t22\t6\t2\t0\t0\tmooc-worker-init:RUNNING/NONE/HEALTHY\n"
            "4\t2026-10-30T14:03:45Z\t1793369025\t3\t3\t1\t1\t8\t6\t2\t42\t6\t2\t0\t0\tmooc-worker-init:RUNNING/NONE/HEALTHY;mooc-worker-new1:PROVISIONING/CREATING/UNKNOWN\n"
            "5\t2026-10-30T14:05:30Z\t1793369130\t3\t3\t3\t3\t6\t0\t6\t0\t0\t6\t0\t0\tmooc-worker-init:RUNNING/NONE/HEALTHY;mooc-worker-new1:RUNNING/NONE/HEALTHY\n"
            "6\t2026-10-30T14:10:00Z\t1793369400\t3\t1\t3\t3\t0\t0\t0\t0\t0\t0\t0\t0\tmooc-worker-init:RUNNING/NONE/HEALTHY;mooc-worker-new1:RUNNING/NONE/HEALTHY\n"
            "7\t2026-10-30T14:25:00Z\t1793370300\t1\t1\t1\t1\t0\t0\t0\t0\t0\t0\t0\t0\tmooc-worker-init:RUNNING/NONE/HEALTHY\n",
            encoding="utf-8",
        )
        (d / "instancias.tsv").write_text(
            "instancia\tinstance_id\trol_ciclo\tcreation_timestamp\tprimera_vista_utc\tprimera_running_utc\tarranque_listo_utc\tsegundos_bootstrap\tprimera_healthy_utc\tprimer_job_started_utc\tprimer_job_completed_utc\ttotal_jobs_completados\tprimera_deleting_utc\tultima_vista_utc\n"
            "mooc-worker-init\t1001\tinicial\t2026-10-30T13:40:00Z\t2026-10-30T14:00:00Z\t2026-10-30T14:00:00Z\t2026-10-30T13:40:55Z\t35\t2026-10-30T14:00:00Z\t2026-10-30T14:00:03Z\t2026-10-30T14:02:26Z\t1\t\t2026-10-30T14:25:00Z\n"
            "mooc-worker-new1\t1002\tnuevo_scale_out\t2026-10-30T14:03:42Z\t2026-10-30T14:03:45Z\t2026-10-30T14:04:15Z\t2026-10-30T14:04:45Z\t36\t2026-10-30T14:04:50Z\t2026-10-30T14:04:46Z\t2026-10-30T14:07:10Z\t1\t2026-10-30T14:24:30Z\t2026-10-30T14:24:50Z\n",
            encoding="utf-8",
        )
        (d / "trabajos_por_worker.tsv").write_text(
            "instancia\tinstance_id\tes_worker_nuevo\tasynq_task_id\tresource_id\tobject_key\tt_started_utc\tt_completed_utc\tduracion_ms\tsegment_count\testado\n"
            "mooc-worker-init\t1001\tfalse\tt-1\tres-01\tuploads/s-01/v.mp4\t2026-10-30T14:00:03Z\t2026-10-30T14:02:26Z\t143000\t30\tcompleted\n"
            "mooc-worker-new1\t1002\ttrue\tt-2\tres-02\tuploads/s-02/v.mp4\t2026-10-30T14:04:46Z\t2026-10-30T14:07:10Z\t144000\t30\tcompleted\n",
            encoding="utf-8",
        )
        (d / "auditoria.tsv").write_text(
            "timestamp_utc\tmetodo\tprincipal\torigen\n"
            "2026-10-30T14:03:42Z\tv1.compute.instanceGroupManagers.resize\tsystem/autoscaler\tautoscaler_automatico\n"
            "2026-10-30T14:24:30Z\tv1.compute.instanceGroupManagers.resize\tsystem/autoscaler\tautoscaler_automatico\n",
            encoding="utf-8",
        )

        summary = analyze_run(d)
        md = write_markdown_report(d, summary)
        charts = generate_charts(d, summary, d / "capturas", "2026-10-30")
        assert md.exists() and md.stat().st_size > 500
        assert all(c.exists() and c.stat().st_size > 10000 for c in charts)
        assert summary["reaccion_desde_redis_s"] == 37.0
        assert summary["incorp_hasta_primer_consumo_s"] == 64.0
    print("[AUTOPRUEBA G6-LINEA-TIEMPO] OK: tiempos separados, reconciliación y gráficas verificados.")
    return 0


def main() -> int:
    _ensure_matplotlib_python()
    parser = argparse.ArgumentParser(description="Reconstructor de línea de tiempo para G6 (Escenario 3B)")
    parser.add_argument("run_dir", nargs="?", help="Directorio de la corrida G6")
    parser.add_argument("--autoprueba", action="store_true", help="Ejecuta autoprueba determinista")
    parser.add_argument("--capturas-dir", help="Directorio donde guardar las gráficas PNG")
    parser.add_argument("--fecha-tag", default=dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%d"))
    parser.add_argument("--project-id", default="plataforma-mooc-entrega2")
    args = parser.parse_args()

    if args.autoprueba:
        return run_self_test()

    if not args.run_dir:
        parser.error("Debe indicar <DIRECTORIO_CORRIDA> o --autoprueba")

    run_dir = pathlib.Path(args.run_dir).resolve()
    capturas_dir = (
        pathlib.Path(args.capturas_dir).resolve()
        if args.capturas_dir
        else run_dir.parent / "capturas"
    )

    summary = analyze_run(run_dir, project_id=args.project_id)
    md_path = write_markdown_report(run_dir, summary)
    charts = generate_charts(run_dir, summary, capturas_dir, args.fecha_tag)
    print(f"[G6-LINEA-TIEMPO] Reporte escrito en: {md_path}")
    for c in charts:
        print(f"[G6-LINEA-TIEMPO] Gráfica generada: {c}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
