#!/usr/bin/env python3
"""E4: pide a Cloud Monitoring las metricas de la ventana de una fase y las deja en CSV y PNG.

    python scripts/e4_escalado/metricas.py docs/entrega4/evidencias/escalado/fase_api_ventana.json
    python scripts/e4_escalado/metricas.py .../fase_workers_ventana.json

Por fase:
  api      instancias de mooc-backend y peticiones por segundo.
  workers  instancias de mooc-workers y mensajes sin entregar en la suscripcion push (y la edad del mas viejo).

Usa las credenciales de gcloud de quien lo corre (`gcloud auth print-access-token`); no guarda ninguna.
Cloud Monitoring publica estas metricas cada 60 s, asi que la resolucion es de un minuto.
"""
import csv
import json
import os
import subprocess
import sys
import urllib.parse
import urllib.request
from datetime import datetime, timedelta, timezone

import matplotlib
import matplotlib.dates

matplotlib.use("Agg")
import matplotlib.pyplot as plt  # noqa: E402

PROJECT = os.environ.get("GCP_PROJECT_ID", "plataforma-mooc-entrega2")
SUBSCRIPTION = "mooc-media-jobs-push"
MARGEN = timedelta(minutes=2)


def token():
    cmd = ["gcloud", "auth", "print-access-token"]
    return subprocess.run(cmd, capture_output=True, text=True, check=True, shell=(os.name == "nt")).stdout.strip()


def consulta(tok, filtro, ini, fin, aligner, reducer=None, group=None):
    params = [
        ("filter", filtro),
        ("interval.startTime", ini.strftime("%Y-%m-%dT%H:%M:%SZ")),
        ("interval.endTime", fin.strftime("%Y-%m-%dT%H:%M:%SZ")),
        ("aggregation.alignmentPeriod", "60s"),
        ("aggregation.perSeriesAligner", aligner),
    ]
    if reducer:
        params.append(("aggregation.crossSeriesReducer", reducer))
        for g in group or []:
            params.append(("aggregation.groupByFields", g))
    url = f"https://monitoring.googleapis.com/v3/projects/{PROJECT}/timeSeries?" + urllib.parse.urlencode(params)
    req = urllib.request.Request(url, headers={"Authorization": "Bearer " + tok})
    with urllib.request.urlopen(req, timeout=60) as r:
        data = json.load(r)
    puntos = {}
    for serie in data.get("timeSeries", []):
        for p in serie["points"]:
            t = datetime.fromisoformat(p["interval"]["endTime"].replace("Z", "+00:00"))
            v = p["value"]
            val = v.get("doubleValue", v.get("int64Value", 0))
            puntos[t] = puntos.get(t, 0) + float(val)
    return sorted(puntos.items())


def run_filter(metrica, servicio):
    return (f'metric.type="run.googleapis.com/{metrica}" AND resource.type="cloud_run_revision" '
            f'AND resource.labels.service_name="{servicio}"')


def sub_filter(metrica):
    return (f'metric.type="pubsub.googleapis.com/subscription/{metrica}" AND resource.type="pubsub_subscription" '
            f'AND resource.labels.subscription_id="{SUBSCRIPTION}"')


def guardar(nombre, series, salida):
    """series: {etiqueta: [(t, valor)]}. Escribe CSV y devuelve las etiquetas de tiempo comunes."""
    tiempos = sorted({t for pts in series.values() for t, _ in pts})
    with open(os.path.join(salida, nombre + ".csv"), "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(["minuto_utc"] + list(series))
        for t in tiempos:
            fila = [t.strftime("%H:%M")]
            for pts in series.values():
                fila.append(dict(pts).get(t, ""))
            w.writerow(fila)


def grafica(nombre, titulo, series, ylabel, salida, ini, fin):
    fig, ax = plt.subplots(figsize=(9, 3.6))
    for etiqueta, pts in series.items():
        if pts:
            ax.step([t for t, _ in pts], [v for _, v in pts], where="post", marker="o", ms=3, label=etiqueta)
    ax.axvspan(ini, fin, color="#999", alpha=0.12, label="carga")
    ax.set_title(titulo)
    ax.set_ylabel(ylabel)
    ax.set_xlabel("hora UTC")
    ax.grid(alpha=0.3)
    ax.legend(loc="upper left", fontsize=8)
    ax.xaxis.set_major_formatter(matplotlib.dates.DateFormatter("%H:%M"))
    fig.tight_layout()
    fig.savefig(os.path.join(salida, nombre + ".png"), dpi=130)
    plt.close(fig)


def resumen(pts):
    return max((v for _, v in pts), default=0)


def main():
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    ruta = sys.argv[1]
    salida = os.path.dirname(os.path.abspath(ruta))
    with open(ruta, encoding="utf-8") as f:
        v = json.load(f)
    ini = datetime.fromisoformat(v["inicio_utc"].replace("Z", "+00:00")) - MARGEN
    fin = datetime.fromisoformat(v["fin_utc"].replace("Z", "+00:00")) + MARGEN + timedelta(minutes=1)
    carga_ini = datetime.fromisoformat(v["inicio_utc"].replace("Z", "+00:00"))
    carga_fin = datetime.fromisoformat(v["fin_utc"].replace("Z", "+00:00"))
    tok = token()
    fase = v["fase"]

    if fase == "api":
        inst = consulta(tok, run_filter("container/instance_count", "mooc-backend"), ini, fin, "ALIGN_MAX", "REDUCE_SUM", ["resource.label.service_name"])
        rps = consulta(tok, run_filter("request_count", "mooc-backend"), ini, fin, "ALIGN_RATE", "REDUCE_SUM", ["resource.label.service_name"])
        guardar("metricas_api", {"instancias_backend": inst, "peticiones_por_s": rps}, salida)
        grafica("api_instancias", "mooc-backend: instancias bajo carga creciente", {"instancias": inst}, "instancias", salida, carga_ini, carga_fin)
        grafica("api_peticiones", "mooc-backend: peticiones por segundo", {"req/s": rps}, "req/s", salida, carga_ini, carga_fin)
        print(f"api: maximo de instancias {resumen(inst):.0f} (limite 3); maximo {resumen(rps):.0f} req/s")
    else:
        inst = consulta(tok, run_filter("container/instance_count", "mooc-workers"), ini, fin, "ALIGN_MAX", "REDUCE_SUM", ["resource.label.service_name"])
        pend = consulta(tok, sub_filter("num_undelivered_messages"), ini, fin, "ALIGN_MAX")
        edad = consulta(tok, sub_filter("oldest_unacked_message_age"), ini, fin, "ALIGN_MAX")
        guardar("metricas_workers", {"instancias_workers": inst, "mensajes_pendientes": pend, "edad_mas_viejo_s": edad}, salida)
        grafica("workers_instancias", "mooc-workers: instancias mientras se drena la cola", {"instancias": inst}, "instancias", salida, carga_ini, carga_fin)
        grafica("workers_pendientes", "Pub/Sub: mensajes pendientes en la suscripcion push", {"mensajes sin entregar": pend}, "mensajes", salida, carga_ini, carga_fin)
        print(f"workers: maximo de instancias {resumen(inst):.0f} (limite 3); maximo de mensajes pendientes {resumen(pend):.0f}; "
              f"mensaje mas viejo esperando {resumen(edad):.0f} s")
    print("escrito en", salida)


if __name__ == "__main__":
    main()
