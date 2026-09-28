#!/usr/bin/env python3
"""Recoge de Cloud Monitoring las metricas de una corrida (issue #133, H3).

Para una ventana de tiempo (la de la corrida) resume, de SOLO LECTURA:
  * las dos VMs: CPU ocupada, memoria usada, conexiones TCP, uso de CPU por proceso
  * Cloud SQL (mooc-db-1): CPU, memoria, conexiones abiertas (num_backends) y
    transacciones por segundo
  * la cola (asynq): profundidad y antiguedad (H1), y si la API reporta metricas

Uso:
  python scripts/capacity_metricas_gcp.py <inicio_UTC> <fin_UTC> [salida.json]
  python scripts/capacity_metricas_gcp.py --todas <directorio_de_resultados>
      (una metricas_gcp.json por cada corrida que tenga ventana.txt)
  ej. python scripts/capacity_metricas_gcp.py 2026-09-28T02:13:00Z 2026-09-28T02:16:00Z

Las horas son UTC (ISO 8601). run_escenario1.sh deja las de cada corrida en su
resumen. Usa el token de gcloud de quien lo ejecuta (`gcloud auth login`), solo en
memoria: no imprime ni guarda credenciales. Lo que escribe son numeros.
"""
import json
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
from collections import defaultdict
from datetime import datetime, timezone

PROJECT = "plataforma-mooc-entrega2"
DB_INSTANCE = "plataforma-mooc-entrega2:mooc-db-1"


def token():
    out = subprocess.run("gcloud auth print-access-token", shell=True, capture_output=True, text=True)
    if not out.stdout.strip():
        sys.exit("No hay sesion de gcloud: corre `gcloud auth login`.")
    return out.stdout.strip()


TOK = None


def api(url):
    global TOK
    TOK = TOK or token()
    req = urllib.request.Request(url, headers={"Authorization": "Bearer " + TOK})
    return json.load(urllib.request.urlopen(req, timeout=90))


def series(metric_filter, start, end, period=60, aligner="ALIGN_MEAN", reducer=None, group_by=None, extra=None):
    """Lee series de tiempo. Devuelve lista de (labels, [(t, valor)])."""
    params = {
        "filter": metric_filter,
        "interval.startTime": start, "interval.endTime": end,
        "aggregation.alignmentPeriod": f"{period}s",
        "aggregation.perSeriesAligner": aligner,
        "view": "FULL",
    }
    if reducer:
        params["aggregation.crossSeriesReducer"] = reducer
        for g in group_by or []:
            params.setdefault("aggregation.groupByFields", [])
            params["aggregation.groupByFields"].append(g)
    q = urllib.parse.urlencode(params, doseq=True)
    out, page = [], ""
    while True:
        try:
            d = api(f"https://monitoring.googleapis.com/v3/projects/{PROJECT}/timeSeries?{q}{page}")
        except urllib.error.HTTPError as e:
            if e.code in (400, 404):   # la metrica no existe (todavia): se reporta como sin datos
                return []
            raise
        for ts in d.get("timeSeries", []):
            labels = {**ts.get("resource", {}).get("labels", {}), **ts.get("metric", {}).get("labels", {})}
            pts = []
            for p in ts.get("points", []):
                v = p["value"]
                val = v.get("doubleValue", v.get("int64Value"))
                if val is not None:
                    pts.append((p["interval"]["endTime"], float(val)))
            out.append((labels, sorted(pts)))
        tok_ = d.get("nextPageToken")
        if not tok_:
            return out
        page = "&pageToken=" + urllib.parse.quote(tok_)


def resumen(valores):
    if not valores:
        return None
    v = sorted(valores)
    p95 = v[min(len(v) - 1, int(round(0.95 * len(v) + 0.5)) - 1)]
    return {"n": len(v), "min": round(v[0], 2), "media": round(sum(v) / len(v), 2), "p95": round(p95, 2), "max": round(v[-1], 2)}


def nombres_de_vm():
    d = api(f"https://compute.googleapis.com/compute/v1/projects/{PROJECT}/aggregated/instances")
    m = {}
    for zona in d.get("items", {}).values():
        for i in zona.get("instances", []):
            m[str(i["id"])] = i["name"]
    return m


def recolectar(start, end):
    vms = nombres_de_vm()
    res = {"ventana": {"inicio": start, "fin": end}, "vm": {}, "cloudsql": {}, "cola_y_api": {}}

    # ---- CPU ocupada por VM: 100 - idle, sumando los estados no ociosos ----
    por_vm = defaultdict(lambda: defaultdict(dict))
    for lab, pts in series('metric.type="agent.googleapis.com/cpu/utilization" AND resource.type="gce_instance"', start, end):
        nombre = vms.get(lab.get("instance_id"), lab.get("instance_id"))
        for t, v in pts:
            por_vm[nombre][t][lab.get("cpu_state", "?")] = v
    for nombre, tiempos in por_vm.items():
        busy = [100.0 - est["idle"] for est in tiempos.values() if "idle" in est]
        res["vm"].setdefault(nombre, {})["cpu_ocupada_pct"] = resumen(busy)

    # ---- memoria usada, conexiones TCP ----
    for clave, mtype, filtro_extra in [
        ("memoria_usada_pct", "agent.googleapis.com/memory/percent_used", ' AND metric.labels.state="used"'),
        ("conexiones_tcp", "agent.googleapis.com/network/tcp_connections", ""),
    ]:
        acum = defaultdict(lambda: defaultdict(float))
        for lab, pts in series(f'metric.type="{mtype}" AND resource.type="gce_instance"{filtro_extra}', start, end):
            nombre = vms.get(lab.get("instance_id"), lab.get("instance_id"))
            for t, v in pts:
                acum[nombre][t] += v
        for nombre, tiempos in acum.items():
            res["vm"].setdefault(nombre, {})[clave] = resumen(list(tiempos.values()))

    # ---- CPU por proceso (que contenedor/proceso consume): tiempo de CPU acumulado -> tasa ----
    procs = defaultdict(lambda: defaultdict(list))
    for lab, pts in series('metric.type="agent.googleapis.com/processes/cpu_time" AND resource.type="gce_instance"',
                           start, end, aligner="ALIGN_RATE"):
        nombre = vms.get(lab.get("instance_id"), lab.get("instance_id"))
        proc = lab.get("process", "?")
        if pts:
            procs[nombre][proc].extend(v for _, v in pts)
    for nombre, d in procs.items():
        top = sorted(((p, resumen(v)) for p, v in d.items() if v), key=lambda x: -(x[1]["max"] if x[1] else 0))[:6]
        # ALIGN_RATE de un contador de segundos de CPU = nucleos usados (1.0 = un nucleo completo)
        res["vm"].setdefault(nombre, {})["procesos_top_nucleos_usados"] = {p: r for p, r in top}

    # ---- Cloud SQL ----
    db = f'resource.type="cloudsql_database" AND resource.labels.database_id="{DB_INSTANCE}"'
    for clave, mtype, scale in [
        ("cpu_pct", "cloudsql.googleapis.com/database/cpu/utilization", 100.0),
        ("memoria_pct", "cloudsql.googleapis.com/database/memory/utilization", 100.0),
    ]:
        vals = []
        for _, pts in series(f'metric.type="{mtype}" AND {db}', start, end):
            vals.extend(v * scale for _, v in pts)
        res["cloudsql"][clave] = resumen(vals)
    # num_backends viene desglosada (por base/estado): se suma en cada instante.
    tot = defaultdict(float)
    for _, pts in series(f'metric.type="cloudsql.googleapis.com/database/postgresql/num_backends" AND {db}', start, end):
        for t, v in pts:
            tot[t] += v
    res["cloudsql"]["conexiones_abiertas"] = resumen(list(tot.values()))
    tps = []
    for _, pts in series(f'metric.type="cloudsql.googleapis.com/database/postgresql/transaction_count" AND {db}',
                         start, end, aligner="ALIGN_RATE"):
        tps.extend(v for _, v in pts)
    res["cloudsql"]["transacciones_por_segundo"] = resumen(tps)

    # ---- cola y API (H1) ----
    for clave, mtype in [
        ("cola_pending", "prometheus.googleapis.com/worker_queue_pending/gauge"),
        ("cola_antiguedad_s", "prometheus.googleapis.com/worker_queue_oldest_pending_age_seconds/gauge"),
        ("scrape_up", "prometheus.googleapis.com/up/gauge"),
    ]:
        vals = []
        for _, pts in series(f'metric.type="{mtype}"', start, end):
            vals.extend(v for _, v in pts)
        res["cola_y_api"][clave] = resumen(vals)

    texto = json.dumps(res, indent=2, ensure_ascii=False)
    print(texto)
    if salida:
        with open(salida, "w", encoding="utf-8", newline="\n") as f:
            f.write(texto + "\n")


if __name__ == "__main__":
    main()
