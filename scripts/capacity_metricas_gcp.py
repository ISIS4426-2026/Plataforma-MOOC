#!/usr/bin/env python3
"""Recoge de Cloud Monitoring las metricas de una corrida (G1, issue #222; antes H3, #133).

Para una ventana de tiempo (la de la corrida) resume, de SOLO LECTURA, todo lo que pide el
enunciado de la Entrega 3 para la arquitectura elastica:

  grupos         CPU, memoria y conexiones TCP de cada GRUPO (api / worker / cola), con el maximo de
                 una instancia sola; y el numero de instancias EXISTENTES y OPERATIVAS en el tiempo
  cloudsql       CPU, memoria, conexiones abiertas, transacciones por segundo (carga de PostgreSQL)
  cola           profundidad (pendientes, activas, total), antiguedad del mas viejo y tasa de
                 procesamiento (trabajos completados y fallidos por segundo)
  balanceador    solicitudes por clase de respuesta y latencia p50/p95/p99 vistas por el balanceador
  autoescalador  utilizacion y tamano recomendado de cada autoescalador
  api            latencia de las peticiones medida por la propia API

Ademas de `metricas_gcp.json` (resumenes) escribe `serie_temporal.csv`: una fila por minuto con lo
que hace falta para graficar el ciclo de escalamiento (G5, G6) y relacionar la carga con las
instancias. Los resumenes de latencia que SI se publican salen de JMeter (capacity_resumen_jtl.py);
los del balanceador son una segunda medida, independiente del generador, para contrastarlas.

Uso:
  python scripts/capacity_metricas_gcp.py <inicio_UTC> <fin_UTC> [salida.json]
  python scripts/capacity_metricas_gcp.py --todas <directorio_de_resultados>
      (un metricas_gcp.json y un serie_temporal.csv por cada corrida que tenga ventana.txt)
  python scripts/capacity_metricas_gcp.py --autoprueba
  ej. python scripts/capacity_metricas_gcp.py 2026-11-01T02:13:00Z 2026-11-01T02:16:00Z

Las horas son UTC (ISO 8601). Usa el token de gcloud de quien lo ejecuta (`gcloud auth login`), solo
en memoria: no imprime ni guarda credenciales. Lo que escribe son numeros.

Variables: MOOC_PROJECT (por omision plataforma-mooc-entrega2) y MOOC_DB_INSTANCE
(por omision <proyecto>:mooc-db-1; si la base se recrea con otra generacion, cambia el nombre).
"""
import csv
import json
import os
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
from collections import defaultdict

PROJECT = os.environ.get("MOOC_PROJECT", "plataforma-mooc-entrega2")
DB_INSTANCE = os.environ.get("MOOC_DB_INSTANCE", f"{PROJECT}:mooc-db-1")

# El grupo de una instancia sale de su nombre: las plantillas de Terraform las llaman
# mooc-api-<sufijo> y mooc-worker-<sufijo>; la cola es una VM propia.
PREFIJOS_GRUPO = (("mooc-api-", "api"), ("mooc-worker-", "worker"), ("mooc-queue", "cola"))


def grupo_de(nombre):
    for prefijo, grupo in PREFIJOS_GRUPO:
        if nombre and nombre.startswith(prefijo):
            return grupo
    return "otro"


def minuto(t):
    """'2026-11-01T02:13:00Z' -> '2026-11-01T02:13'. Todas las series se alinean al minuto."""
    return t[:16]


# ----------------------------------------------------------------------------- acceso a la API

TOK = None


def token():
    out = subprocess.run("gcloud auth print-access-token", shell=True, capture_output=True, text=True)
    if not out.stdout.strip():
        sys.exit("No hay sesion de gcloud: corre `gcloud auth login`.")
    return out.stdout.strip()


def api(url):
    global TOK
    TOK = TOK or token()
    req = urllib.request.Request(url, headers={"Authorization": "Bearer " + TOK})
    return json.load(urllib.request.urlopen(req, timeout=90))


def series(metric_filter, start, end, period=60, aligner="ALIGN_MEAN", reducer=None, group_by=None):
    """Lee series de tiempo. Devuelve lista de (etiquetas, [(t, valor)]).

    En las etiquetas va `_nombre`: el nombre de la VM segun los metadatos de Monitoring. Es lo que
    permite agrupar tambien las instancias que YA no existen (un grupo elastico las destruye)."""
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
            params.setdefault("aggregation.groupByFields", []).append(g)
    q = urllib.parse.urlencode(params, doseq=True)
    out, page = [], ""
    while True:
        try:
            d = api(f"https://monitoring.googleapis.com/v3/projects/{PROJECT}/timeSeries?{q}{page}")
        except urllib.error.HTTPError as e:
            if e.code in (400, 404):  # la metrica no existe (todavia): se reporta como sin datos
                return []
            raise
        for ts in d.get("timeSeries", []):
            labels = {**ts.get("resource", {}).get("labels", {}), **ts.get("metric", {}).get("labels", {})}
            nombre = ((ts.get("metadata") or {}).get("systemLabels") or {}).get("name")
            if nombre:
                labels["_nombre"] = nombre
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
    """n, minimo, media, p95 y maximo de una lista. Con menos de 20 puntos el p95 es el maximo: se
    declara con `n`, como pide la nota 25 (un percentil sin muestra suficiente no se presenta como medido)."""
    if not valores:
        return None
    v = sorted(valores)
    p95 = v[min(len(v) - 1, int(round(0.95 * len(v) + 0.5)) - 1)]
    r = {"n": len(v), "min": round(v[0], 2), "media": round(sum(v) / len(v), 2), "max": round(v[-1], 2)}
    r["p95"] = round(p95, 2) if len(v) >= 20 else None
    return r


def api_post(url, cuerpo):
    global TOK
    TOK = TOK or token()
    req = urllib.request.Request(url, data=json.dumps(cuerpo).encode(), method="POST",
                                 headers={"Authorization": "Bearer " + TOK, "Content-Type": "application/json"})
    return json.load(urllib.request.urlopen(req, timeout=90))


def nombres_de_vm():
    """{instance_id: nombre}. Las instancias vivas salen de Compute; las que un grupo elastico YA destruyo,
    del registro de auditoria de su creacion (Monitoring solo guarda el id numerico de una VM borrada)."""
    d = api(f"https://compute.googleapis.com/compute/v1/projects/{PROJECT}/aggregated/instances")
    m = {}
    for zona in d.get("items", {}).values():
        for i in zona.get("instances", []):
            m[str(i["id"])] = i["name"]
    try:
        cuerpo = {"resourceNames": [f"projects/{PROJECT}"], "pageSize": 1000, "orderBy": "timestamp desc",
                  "filter": 'resource.type="gce_instance" AND protoPayload.methodName:"instances.insert" AND protoPayload.request.name:"mooc-"'}
        for e in api_post("https://logging.googleapis.com/v2/entries:list", cuerpo).get("entries", []):
            iid = str(e.get("resource", {}).get("labels", {}).get("instance_id", ""))
            nombre = (e.get("protoPayload", {}).get("resourceName") or "").rsplit("/", 1)[-1]
            if iid and nombre:
                m.setdefault(iid, nombre)
    except urllib.error.HTTPError:
        pass  # sin permiso de lectura de registros: las VM borradas quedaran como "otro"
    return m


# ----------------------------------------------------------------------------- agregacion pura

def agregar_por_grupo(puntos_por_instancia, modo="media"):
    """puntos_por_instancia: {nombre_vm: {minuto: valor}} -> {grupo: {minuto: valor, ...}}.

    `media` promedia las instancias del grupo en ese minuto; `suma` las suma (conexiones). Devuelve ademas
    el maximo de UNA instancia y cuantas instancias reportaron (las OPERATIVAS: las que mandan metricas)."""
    por_grupo = defaultdict(lambda: defaultdict(list))
    for nombre, tiempos in puntos_por_instancia.items():
        for t, v in tiempos.items():
            por_grupo[grupo_de(nombre)][t].append(v)
    res = {}
    for g, tiempos in por_grupo.items():
        res[g] = {
            "valor": {t: (sum(vs) / len(vs) if modo == "media" else sum(vs)) for t, vs in tiempos.items()},
            "max_instancia": {t: max(vs) for t, vs in tiempos.items()},
            "reportan": {t: len(vs) for t, vs in tiempos.items()},
        }
    return res


def autoprueba():
    ok = True

    def check(cond, msg):
        nonlocal ok
        print(("ok  " if cond else "FALLA ") + msg)
        ok = ok and cond

    check(grupo_de("mooc-api-c8w5") == "api" and grupo_de("mooc-worker-w7s3") == "worker", "el grupo sale del nombre de la VM")
    check(grupo_de("mooc-queue-server") == "cola" and grupo_de("otra") == "otro", "la cola y lo desconocido tienen su grupo")
    pts = {"mooc-api-a": {"t1": 40.0, "t2": 60.0}, "mooc-api-b": {"t2": 80.0}, "mooc-worker-x": {"t1": 10.0}}
    g = agregar_por_grupo(pts)
    check(g["api"]["valor"]["t2"] == 70.0 and g["api"]["max_instancia"]["t2"] == 80.0, "la media y el maximo de una instancia se separan")
    check(g["api"]["reportan"] == {"t1": 1, "t2": 2}, "las instancias operativas se cuentan por minuto (una nueva aparece en t2)")
    check(agregar_por_grupo(pts, "suma")["api"]["valor"]["t2"] == 140.0, "las conexiones se suman")
    check(resumen([1.0] * 5)["p95"] is None, "con 5 puntos no se presenta un p95")
    check(resumen([float(i) for i in range(1, 41)])["p95"] is not None, "con 40 puntos si")
    check(minuto("2026-11-01T02:13:00Z") == "2026-11-01T02:13", "alineacion al minuto")
    print("\nTodas las reglas se cumplen." if ok else "\nHay reglas que fallan.")
    return 0 if ok else 1


# ----------------------------------------------------------------------------- recoleccion

def por_instancia(mtype, start, end, vms, filtro_extra="", aligner="ALIGN_MEAN", suma_etiquetas=False):
    """{nombre_vm: {minuto: valor}} para una metrica del agente. Las etiquetas extra (estado de CPU,
    puerto...) se SUMAN en cada minuto cuando `suma_etiquetas`."""
    out = defaultdict(lambda: defaultdict(float))
    for lab, pts in series(f'metric.type="{mtype}" AND resource.type="gce_instance"{filtro_extra}', start, end, aligner=aligner):
        nombre = lab.get("_nombre") or vms.get(lab.get("instance_id")) or lab.get("instance_id", "?")
        for t, v in pts:
            if suma_etiquetas:
                out[nombre][minuto(t)] += v
            else:
                out[nombre][minuto(t)] = v
    return out


def serie_simple(filtro, start, end, aligner="ALIGN_MEAN", suma=False):
    """{minuto: valor}; si hay varias series se SUMAN (suma) o se promedian en cada minuto."""
    acum = defaultdict(list)
    for _, pts in series(filtro, start, end, aligner=aligner):
        for t, v in pts:
            acum[minuto(t)].append(v)
    return {t: (sum(vs) if suma else sum(vs) / len(vs)) for t, vs in acum.items()}


def recolectar(start, end):
    vms = nombres_de_vm()
    res = {"ventana": {"inicio": start, "fin": end}, "proyecto": PROJECT, "grupos": {}, "vm": {}, "cloudsql": {},
           "cola": {}, "balanceador": {}, "autoescalador": {}, "api": {}, "cola_y_api": {}}
    tabla = defaultdict(dict)  # minuto -> columna -> valor (serie_temporal.csv)

    # ---- CPU ocupada: 100 - idle, por instancia, y de ahi por grupo ----
    cpu = defaultdict(lambda: defaultdict(dict))
    for lab, pts in series('metric.type="agent.googleapis.com/cpu/utilization" AND resource.type="gce_instance"', start, end):
        nombre = lab.get("_nombre") or vms.get(lab.get("instance_id")) or lab.get("instance_id", "?")
        for t, v in pts:
            cpu[nombre][minuto(t)][lab.get("cpu_state", "?")] = v
    cpu_ocupada = {n: {t: 100.0 - e["idle"] for t, e in ts.items() if "idle" in e} for n, ts in cpu.items()}
    memoria = por_instancia("agent.googleapis.com/memory/percent_used", start, end, vms, ' AND metric.labels.state="used"')
    tcp = por_instancia("agent.googleapis.com/network/tcp_connections", start, end, vms, suma_etiquetas=True)

    for n, ts in cpu_ocupada.items():
        res["vm"].setdefault(n, {})["cpu_ocupada_pct"] = resumen(list(ts.values()))
    for n, ts in memoria.items():
        res["vm"].setdefault(n, {})["memoria_usada_pct"] = resumen(list(ts.values()))
    for n, ts in tcp.items():
        res["vm"].setdefault(n, {})["conexiones_tcp"] = resumen(list(ts.values()))

    cpu_g, mem_g, tcp_g = agregar_por_grupo(cpu_ocupada), agregar_por_grupo(memoria), agregar_por_grupo(tcp, "suma")
    for g in sorted(set(cpu_g) | set(mem_g) | set(tcp_g)):
        entrada = {}
        if g in cpu_g:
            entrada["cpu_ocupada_pct_media_del_grupo"] = resumen(list(cpu_g[g]["valor"].values()))
            entrada["cpu_ocupada_pct_max_de_una_instancia"] = resumen(list(cpu_g[g]["max_instancia"].values()))
            entrada["instancias_operativas"] = resumen([float(x) for x in cpu_g[g]["reportan"].values()])
            for t, v in cpu_g[g]["valor"].items():
                tabla[t][f"{g}_cpu_media_pct"] = round(v, 2)
            for t, n in cpu_g[g]["reportan"].items():
                tabla[t][f"{g}_operativas"] = n
        if g in mem_g:
            entrada["memoria_usada_pct_media_del_grupo"] = resumen(list(mem_g[g]["valor"].values()))
        if g in tcp_g:
            entrada["conexiones_tcp_del_grupo"] = resumen(list(tcp_g[g]["valor"].values()))
        res["grupos"][g] = entrada

    # ---- instancias EXISTENTES por grupo (el tamano que ve el grupo administrado) ----
    for lab, pts in series('metric.type="compute.googleapis.com/instance_group/size" AND resource.type="instance_group"',
                           start, end, aligner="ALIGN_MAX"):
        g = grupo_de(lab.get("instance_group_name", ""))
        if g == "otro":
            continue
        res["grupos"].setdefault(g, {})["instancias_existentes"] = resumen([v for _, v in pts])
        for t, v in pts:
            tabla[minuto(t)][f"{g}_existentes"] = int(v)

    # ---- Cloud SQL ----
    db = f'resource.type="cloudsql_database" AND resource.labels.database_id="{DB_INSTANCE}"'
    for clave, mtype, escala in [("cpu_pct", "cloudsql.googleapis.com/database/cpu/utilization", 100.0),
                                 ("memoria_pct", "cloudsql.googleapis.com/database/memory/utilization", 100.0)]:
        s = {t: v * escala for t, v in serie_simple(f'metric.type="{mtype}" AND {db}', start, end).items()}
        res["cloudsql"][clave] = resumen(list(s.values()))
        if clave == "cpu_pct":
            for t, v in s.items():
                tabla[t]["db_cpu_pct"] = round(v, 2)
    conexiones = serie_simple(f'metric.type="cloudsql.googleapis.com/database/postgresql/num_backends" AND {db}', start, end, suma=True)
    res["cloudsql"]["conexiones_abiertas"] = resumen(list(conexiones.values()))
    for t, v in conexiones.items():
        tabla[t]["db_conexiones"] = int(v)
    res["cloudsql"]["transacciones_por_segundo"] = resumen(list(serie_simple(
        f'metric.type="cloudsql.googleapis.com/database/postgresql/transaction_count" AND {db}', start, end, aligner="ALIGN_RATE", suma=True).values()))

    # ---- la cola: profundidad, antiguedad y tasa de procesamiento ----
    for clave, mtype in [("pendientes", "custom.googleapis.com/mooc/queue/pending"),
                         ("activas", "custom.googleapis.com/mooc/queue/active"),
                         ("total_en_cola", "custom.googleapis.com/mooc/queue/outstanding"),
                         ("antiguedad_del_mas_viejo_s", "custom.googleapis.com/mooc/queue/oldest_pending_age_seconds")]:
        s = serie_simple(f'metric.type="{mtype}"', start, end, aligner="ALIGN_MAX")
        res["cola"][clave] = resumen(list(s.values()))
        for t, v in s.items():
            tabla[t][f"cola_{clave}"] = round(v, 1)
    for clave, mtype in [("trabajos_completados_por_s", "worker_jobs_processed_total/counter"),
                         ("trabajos_fallidos_por_s", "worker_jobs_failed_total/counter")]:
        s = serie_simple(f'metric.type="prometheus.googleapis.com/{mtype}"', start, end, aligner="ALIGN_RATE", suma=True)
        res["cola"][clave] = resumen(list(s.values()))
        for t, v in s.items():
            tabla[t][f"cola_{clave}"] = round(v, 4)

    # ---- balanceador: solicitudes por clase y latencia vista desde el balanceador ----
    por_clase = defaultdict(float)
    por_clase_min = defaultdict(lambda: defaultdict(float))
    for lab, pts in series('metric.type="loadbalancing.googleapis.com/https/request_count"', start, end, aligner="ALIGN_SUM"):
        clase = lab.get("response_code_class", "?")
        for t, v in pts:
            por_clase[clase] += v
            por_clase_min[minuto(t)][clase] += v
    res["balanceador"]["solicitudes_por_clase"] = {k: int(v) for k, v in sorted(por_clase.items())}
    res["balanceador"]["solicitudes_totales"] = int(sum(por_clase.values()))
    for t, clases in por_clase_min.items():
        tabla[t]["lb_solicitudes"] = int(sum(clases.values()))
        tabla[t]["lb_5xx"] = int(sum(v for c, v in clases.items() if str(c).startswith("5")))
    for etiqueta, mtype in [("latencia_total_ms", "loadbalancing.googleapis.com/https/total_latencies"),
                            ("latencia_del_backend_ms", "loadbalancing.googleapis.com/https/backend_latencies")]:
        entrada = {}
        for pct in (50, 95, 99):
            s = serie_simple(f'metric.type="{mtype}"', start, end, aligner=f"ALIGN_PERCENTILE_{pct}")
            entrada[f"p{pct}_por_minuto"] = resumen(list(s.values()))
            if etiqueta == "latencia_total_ms":
                for t, v in s.items():
                    tabla[t][f"lb_p{pct}_ms"] = round(v, 1)
        res["balanceador"][etiqueta] = entrada

    # ---- autoescaladores ----
    for lab, pts in series('metric.type="autoscaler.googleapis.com/current_utilization"', start, end, aligner="ALIGN_MEAN"):
        nombre = lab.get("autoscaler_name", lab.get("_nombre", "autoescalador"))
        res["autoescalador"].setdefault(nombre, {})["utilizacion"] = resumen([v for _, v in pts])
        g = "api" if "api" in nombre else "worker" if "worker" in nombre else "otro"
        for t, v in pts:
            tabla[minuto(t)][f"{g}_autoescalador_utilizacion"] = round(v, 3)
    for lab, pts in series('metric.type="autoscaler.googleapis.com/recommended_size"', start, end, aligner="ALIGN_MAX"):
        nombre = lab.get("autoscaler_name", lab.get("_nombre", "autoescalador"))
        res["autoescalador"].setdefault(nombre, {})["tamano_recomendado"] = resumen([v for _, v in pts])
        g = "api" if "api" in nombre else "worker" if "worker" in nombre else "otro"
        for t, v in pts:
            tabla[minuto(t)][f"{g}_autoescalador_recomendado"] = int(v)

    # ---- la API: latencia medida por ella misma (sin red ni balanceador de por medio) ----
    for pct in (50, 95, 99):
        s = serie_simple('metric.type="prometheus.googleapis.com/http_server_request_duration_seconds/histogram"',
                         start, end, aligner=f"ALIGN_PERCENTILE_{pct}")
        res["api"][f"latencia_p{pct}_s_por_minuto"] = resumen(list(s.values()))

    # compatibilidad con las corridas de la Entrega 2 (H3, A7)
    res["cola_y_api"] = {"cola_pending": res["cola"].get("pendientes"), "cola_antiguedad_s": res["cola"].get("antiguedad_del_mas_viejo_s")}
    res["_serie_temporal"] = [{"minuto_utc": t, **cols} for t, cols in sorted(tabla.items())]
    return res


COLUMNAS_FIJAS = ["minuto_utc"]


def escribir(res, ruta_json):
    serie = res.pop("_serie_temporal", [])
    with open(ruta_json, "w", encoding="utf-8", newline=chr(10)) as f:
        json.dump(res, f, indent=2, ensure_ascii=False)
        f.write(chr(10))
    if serie:
        cols = COLUMNAS_FIJAS + sorted({c for fila in serie for c in fila if c not in COLUMNAS_FIJAS})
        ruta_csv = os.path.join(os.path.dirname(os.path.abspath(ruta_json)), "serie_temporal.csv")
        with open(ruta_csv, "w", encoding="utf-8", newline="") as f:
            w = csv.DictWriter(f, fieldnames=cols, lineterminator=chr(10))
            w.writeheader()
            w.writerows(serie)


def main():
    if len(sys.argv) >= 2 and sys.argv[1] == "--autoprueba":
        sys.exit(autoprueba())
    if len(sys.argv) >= 3 and sys.argv[1] == "--todas":
        raiz = sys.argv[2]
        for nombre in sorted(os.listdir(raiz)):
            ventana = os.path.join(raiz, nombre, "ventana.txt")
            destino = os.path.join(raiz, nombre, "metricas_gcp.json")
            if not os.path.exists(ventana) or os.path.exists(destino):
                continue
            inicio, fin = open(ventana, encoding="utf-8").read().split()[:2]
            print("->", nombre, inicio, fin, file=sys.stderr)
            escribir(recolectar(inicio, fin), destino)
        return
    if len(sys.argv) < 3:
        sys.exit(__doc__)
    res = recolectar(sys.argv[1], sys.argv[2])
    if len(sys.argv) > 3:
        escribir(dict(res), sys.argv[3])
        res.pop("_serie_temporal", None)
        print(f"escrito {sys.argv[3]} y serie_temporal.csv", file=sys.stderr)
    else:
        res.pop("_serie_temporal", None)
        print(json.dumps(res, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
