#!/usr/bin/env python3
"""Dibuja el tablero de una corrida a partir de su serie_temporal.csv (G1, issue #222).

Es la exportacion grafica de lo que recoge capacity_metricas_gcp.py: una figura de cuatro paneles que
sirve igual para una corrida de G3/G4 que para el ciclo de escalamiento de G5 y G6.

  1. Instancias por grupo: existentes (lo que el grupo administrado cree que hay) y operativas (las que mandan
     metricas), de la API y de los workers. El ciclo de escalamiento se ve aqui.
  2. CPU media de cada grupo y de la base.
  3. La cola: total, pendientes y antiguedad del mas viejo (eje derecho).
  4. El balanceador: solicitudes por minuto y latencia p50/p95/p99 (por minuto).

Uso:
  python scripts/capacity_graficas.py <serie_temporal.csv> [salida.png] [--titulo "texto"]

Solo lee el CSV: no usa la red ni credenciales. Los paneles sin datos se dibujan vacios, con su leyenda
"sin datos", para que la ausencia se vea y no se confunda con un cero.
"""
import csv
import sys

import matplotlib

matplotlib.use("Agg")
import matplotlib.dates as mdates  # noqa: E402
import matplotlib.pyplot as plt  # noqa: E402
from datetime import datetime  # noqa: E402


def leer(ruta):
    with open(ruta, encoding="utf-8", newline="") as f:
        filas = list(csv.DictReader(f))
    tiempos = [datetime.strptime(f["minuto_utc"], "%Y-%m-%dT%H:%M") for f in filas]

    def col(nombre):
        vals = []
        for f in filas:
            v = f.get(nombre, "")
            vals.append(float(v) if v not in ("", None) else None)
        return vals

    return tiempos, col


def linea(ax, t, ys, etiqueta, **kw):
    puntos = [(a, b) for a, b in zip(t, ys) if b is not None]
    if not puntos:
        return False
    ax.plot([a for a, _ in puntos], [b for _, b in puntos], label=etiqueta, **kw)
    return True


def sin_datos(ax):
    ax.text(0.5, 0.5, "sin datos en esta ventana", transform=ax.transAxes, ha="center", va="center", color="#888")


def dibujar(csv_ruta, salida, titulo):
    t, col = leer(csv_ruta)
    fig, ejes = plt.subplots(4, 1, figsize=(11, 12), sharex=True)
    a1, a2, a3, a4 = ejes

    hay = [linea(a1, t, col("api_existentes"), "API existentes", color="#4a5ce6", drawstyle="steps-post", lw=2.4),
           linea(a1, t, col("api_operativas"), "API operativas", color="#4a5ce6", drawstyle="steps-post", ls="--"),
           linea(a1, t, col("worker_existentes"), "workers existentes", color="#e0517a", drawstyle="steps-post", lw=2.4),
           linea(a1, t, col("worker_operativas"), "workers operativos", color="#e0517a", drawstyle="steps-post", ls="--")]
    a1.set_ylabel("instancias")
    a1.set_ylim(0, 4)
    a1.set_yticks([0, 1, 2, 3, 4])
    a1.set_title("Instancias por grupo (mínimo 1, máximo 3)")
    if not any(hay):
        sin_datos(a1)

    hay = [linea(a2, t, col("api_cpu_media_pct"), "API", color="#4a5ce6"),
           linea(a2, t, col("worker_cpu_media_pct"), "workers", color="#e0517a"),
           linea(a2, t, col("cola_cpu_media_pct"), "VM de la cola", color="#999"),
           linea(a2, t, col("db_cpu_pct"), "PostgreSQL", color="#2f9e6e")]
    a2.set_ylabel("CPU ocupada (%)")
    a2.set_title("CPU media por grupo y de la base")
    if not any(hay):
        sin_datos(a2)

    hay = [linea(a3, t, col("cola_total_en_cola"), "total en cola", color="#e0517a", lw=2),
           linea(a3, t, col("cola_pendientes"), "pendientes", color="#f2a7b6"),
           linea(a3, t, col("cola_activas"), "activas", color="#4a5ce6")]
    a3.set_ylabel("trabajos")
    a3.set_title("Cola de procesamiento")
    if not any(hay):
        sin_datos(a3)
    b3 = a3.twinx()
    if linea(b3, t, col("cola_antiguedad_del_mas_viejo_s"), "antigüedad del más viejo (s, eje derecho)", color="#555", ls=":"):
        b3.set_ylabel("segundos")
        h1, l1 = a3.get_legend_handles_labels()
        h2, l2 = b3.get_legend_handles_labels()
        a3.legend(h1 + h2, l1 + l2, loc="upper left", fontsize=8)
    else:
        b3.set_yticks([])
        a3.legend(loc="upper left", fontsize=8)

    hay = [linea(a4, t, col("lb_p50_ms"), "p50", color="#2f9e6e"),
           linea(a4, t, col("lb_p95_ms"), "p95", color="#e8a317"),
           linea(a4, t, col("lb_p99_ms"), "p99", color="#d64545")]
    a4.set_ylabel("latencia (ms)")
    a4.set_title("Balanceador: latencia por minuto y solicitudes (barras, eje derecho)")
    if not any(hay):
        sin_datos(a4)
    b4 = a4.twinx()
    pares = [(a, b) for a, b in zip(t, col("lb_solicitudes")) if b is not None]
    if pares:
        b4.bar([a for a, _ in pares], [b for _, b in pares], width=0.0006, color="#4a5ce6", alpha=0.35, label="solicitudes/min")
        b4.set_ylabel("solicitudes por minuto")
    else:
        b4.set_yticks([])
    h1, l1 = a4.get_legend_handles_labels()
    if h1:
        a4.legend(h1, l1, loc="upper left", fontsize=8)

    for ax in (a1, a2):
        if ax.get_legend_handles_labels()[0]:
            ax.legend(loc="upper left", fontsize=8, ncol=2)
    for ax in ejes:
        ax.grid(alpha=0.25)
    a4.xaxis.set_major_formatter(mdates.DateFormatter("%H:%M"))
    a4.set_xlabel("hora UTC")
    fig.suptitle(titulo or "Tablero de la corrida", fontsize=13, y=0.995)
    fig.tight_layout()
    fig.savefig(salida, dpi=130)
    plt.close(fig)


def main():
    args = sys.argv[1:]
    titulo = None
    if "--titulo" in args:
        i = args.index("--titulo")
        titulo = args[i + 1]
        del args[i:i + 2]
    if not args:
        sys.exit(__doc__)
    salida = args[1] if len(args) > 1 else args[0].rsplit(".", 1)[0] + ".png"
    dibujar(args[0], salida, titulo)
    print("escrito", salida)


if __name__ == "__main__":
    main()
