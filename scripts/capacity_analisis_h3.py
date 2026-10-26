#!/usr/bin/env python3
"""Analisis de las corridas de H3 (issue #133): tablas por nivel, variacion entre repeticiones,
utilizacion de infraestructura y origen de la latencia de cola.

Uso: python scripts/capacity_analisis_h3.py [directorio_de_resultados]
Lee resumen.json, metricas_gcp.json, generador.csv y resultados.jtl de cada corrida. Solo lectura.
"""
import collections
import csv
import glob
import json
import os
import statistics as st
import sys

import capacity_criterios as cc

RAIZ = sys.argv[1] if len(sys.argv) > 1 else "docs/entrega2/evidencias/H3/resultados"


def celda(resumen, p, ancho=5):
    """Percentil publicable de un resumen.json, o por que no hay numero.

    Prefiere `ms_publicables`, que ya trae la regla de observaciones minimas
    aplicada. Un resumen.json anterior a A6 no la trae: entonces se usa el
    valor en crudo y se marca con un asterisco, porque no se sabe si tenia
    muestras para sostenerlo.
    """
    pub = resumen.get("ms_publicables")
    if pub is not None:
        return f"{cc.formatear(pub.get(f'p{p}'), resumen.get('muestras', 0)):>{ancho}s}"
    return f"{resumen['ms'][f'p{p}']}*".rjust(ancho)


def cargar(d):
    r = json.load(open(os.path.join(d, "resumen.json"), encoding="utf-8"))
    p = os.path.join(d, "metricas_gcp.json")
    m = json.load(open(p, encoding="utf-8")) if os.path.exists(p) else None
    return r, m


def g(x, *ks):
    for k in ks:
        x = (x or {}).get(k) if isinstance(x, dict) else None
    return x


def main():
    dirs = sorted(glob.glob(os.path.join(RAIZ, "A-nivel*")) + glob.glob(os.path.join(RAIZ, "B-*")),
                  key=lambda d: open(os.path.join(d, "ventana.txt")).read())
    # Destino de cada corrida, antes de cualquier tabla. Una corrida sin destino
    # registrado no es publicable (A6), y la Entrega 2 publico como nube una
    # medicion contra Docker Compose precisamente porque esto no se imprimia.
    print("== 0. Entorno medido en cada corrida")
    for d in dirs:
        n = os.path.basename(d).split("_2026")[0]
        ruta = os.path.join(d, cc.ARCHIVO_DESTINO)
        if not os.path.exists(ruta):
            print(f"{n[:40]:40s} SIN REGISTRAR - no publicable")
            continue
        dest = cc.leer_destino(d)
        print(f"{n[:40]:40s} {dest.get('base_url', '?')}  [{dest['clase']}]  entorno={dest.get('entorno', '?')}")

    print("\n== 1. Resultados y utilizacion por corrida (ventana medida; metricas de Cloud Monitoring, muestras de 60 s)")
    print(f"{'corrida':30s} {'usr':>4s} {'rps':>6s} {'p50':>5s} {'p95':>5s} {'p99':>5s} {'real':>4s} {'t/o':>3s} {'val':>3s} | {'webCPU%':>9s} {'webMem%':>7s} {'sqlCPU%':>9s} {'sqlConx':>7s} {'tps':>5s}")
    filas = {}
    for d in dirs:
        r, m = cargar(d)
        n = os.path.basename(d).split("_2026")[0]
        w = (m or {}).get("vm", {}).get("mooc-web-server", {})
        s = (m or {}).get("cloudsql", {})
        usr = r["hilos_max"]
        filas[n] = (usr, r, w, s)
        print(f"{n[:30]:30s} {usr:4d} {r['rps']:6.1f} {celda(r, 50)} {celda(r, 95)} {celda(r, 99)} {r['fallos_reales']:4d} {r['timeouts']:3d} {r['fallos_validacion']:3d} | "
              f"{g(w,'cpu_ocupada_pct','media')!s:>4}/{g(w,'cpu_ocupada_pct','max')!s:<4} {g(w,'memoria_usada_pct','max')!s:>7} "
              f"{g(s,'cpu_pct','media')!s:>4}/{g(s,'cpu_pct','max')!s:<4} {g(s,'conexiones_abiertas','max')!s:>7} {g(s,'transacciones_por_segundo','max')!s:>5}")

    print("\n== 2. Nivel de 200 usuarios: variacion entre las 3 corridas")
    reps = [v for k, v in filas.items() if k.startswith("A-nivel200")]
    def est(nombre, vals):
        print(f"{nombre:26s} media {st.mean(vals):8.1f}  desv.est. {st.pstdev(vals):7.1f}  min {min(vals):8.1f}  max {max(vals):8.1f}  CV {100*st.pstdev(vals)/st.mean(vals):5.1f}%")
    est("peticiones/s", [v[1]["rps"] for v in reps])
    est("p50 (ms)", [v[1]["ms"]["p50"] for v in reps])
    est("p95 (ms)", [v[1]["ms"]["p95"] for v in reps])
    est("p99 (ms)", [v[1]["ms"]["p99"] for v in reps])
    est("CPU web maxima (%)", [g(v[2], "cpu_ocupada_pct", "max") for v in reps])
    est("CPU Cloud SQL maxima (%)", [g(v[3], "cpu_pct", "max") for v in reps])
    est("conexiones a la base max", [g(v[3], "conexiones_abiertas", "max") for v in reps])

    print("\n== 3. De donde viene la latencia de cola: conexiones nuevas (JMeter 'Connect' = TCP + TLS)")
    for d in dirs:
        n = os.path.basename(d).split("_2026")[0]
        if not n.startswith("A-nivel200"):
            continue
        filas_jtl = list(csv.DictReader(open(os.path.join(d, "resultados.jtl"), encoding="utf-8")))
        nuevas = [int(x["Connect"]) for x in filas_jtl if int(x["Connect"]) > 0]
        lentas = [x for x in filas_jtl if 1200 <= int(x["elapsed"]) <= 1700]
        ps = cc.percentiles(nuevas, (50, 95))
        gen = list(csv.DictReader(open(os.path.join(d, "generador.csv"))))
        cpu_gen = max(float(x["cpu_pct"].strip("%")) for x in gen)
        print(f"{n[:30]:30s} peticiones {len(filas_jtl)} | con conexion nueva {len(nuevas)} ({100*len(nuevas)/len(filas_jtl):.1f}%) | "
              f"Connect p50 {cc.formatear(ps[50], len(nuevas))} ms p95 {cc.formatear(ps[95], len(nuevas))} ms max {max(nuevas)} ms")
        print(f"{'':30s} peticiones de 1,2-1,7 s: {len(lentas)} | de ellas con Connect > 900 ms: {sum(int(x['Connect']) > 900 for x in lentas)} | "
              f"CPU maxima del generador (docker stats, % de un nucleo): {cpu_gen:.0f}")

    print("\n== 4. Latencia del servidor sin el costo de conectar (peticiones sobre conexion ya abierta)")
    for d in dirs:
        n = os.path.basename(d).split("_2026")[0]
        if not n.startswith("A-nivel200"):
            continue
        filas_jtl = list(csv.DictReader(open(os.path.join(d, "resultados.jtl"), encoding="utf-8")))
        reuso = [int(x["elapsed"]) for x in filas_jtl if int(x["Connect"]) == 0]
        ps = cc.percentiles(reuso)
        print(f"{n[:30]:30s} n={len(reuso)} p50 {cc.formatear(ps[50], len(reuso))} ms  "
              f"p95 {cc.formatear(ps[95], len(reuso))} ms  p99 {cc.formatear(ps[99], len(reuso))} ms")


if __name__ == "__main__":
    main()
