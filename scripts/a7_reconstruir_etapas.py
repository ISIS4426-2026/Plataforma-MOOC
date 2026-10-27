#!/usr/bin/env python3
"""Reconstruye las etapas 4, 5 y 6 del escenario 2 desde los registros de la
plataforma, no desde el sondeo de la campana (A7, issue #199).

POR QUE EXISTE

La campana sondea la API para saber cuando un recurso llega a `completed`, y de
ahi deduce la espera en cola (etapa 4), el procesamiento (etapa 5) y el total
hasta disponible (etapa 6). Con perfiles reales eso se rompio: un trabajo
encolado detras de otros dieciseis no cambia de estado durante una hora --la
antiguedad maxima medida fue de 3 886 s-- y el detector de atasco de la campana
lo daba por perdido a los 12 minutos.

Pero la plataforma SI lo registro:

  * el worker escribe `job media:process started` y `completed` por trabajo,
    con su identificador y con `duración_ms`, que es la etapa 5 exacta;
  * el objeto original en el bucket tiene su hora de creacion, que es cuando
    termino el PUT, justo antes de la confirmacion que encola. Sirve de origen
    para las etapas 4 y 6 mejor que el `created_at` de la base, que es anterior
    a la subida.

Asi que las tres etapas se reconstruyen de los registros de quien hizo el
trabajo, en lugar de de quien lo miraba. Es mejor evidencia, no un apano.

QUE NO RECONSTRUYE

La cronologia del drenaje: eso solo lo veia el inspector de la cola, y en esa
corrida se perdio por tiempos de espera del tunel. El pico de cola (16 tareas) y
la antiguedad maxima (3 886 s) si quedaron registrados por la campana.

USO

  python scripts/a7_reconstruir_etapas.py \\
      --worker-log docs/entrega3/evidencias/A7/escenario2-1st-architecture/worker_crudo.log \\
      --objetos    <listado de gcloud storage ls -l>
"""
import argparse
import json
import os
import re
import statistics
import sys
from datetime import datetime

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import capacity_criterios as cc  # noqa: E402

# Tamano exacto de cada perfil, verificado por generar_media_prueba.sh. Es lo
# que identifica el perfil de cada trabajo: la base no guarda el tamano.
PERFILES = {
    2603805: ("corto", 120.0),
    13012943: ("medio", 600.0),
    39060469: ("largo", 1800.0),
}


def instante(texto):
    return datetime.fromisoformat(texto.replace("Z", "+00:00"))


def leer_worker(ruta):
    """{clave_objeto: {inicio, fin, duracion_s}} desde el log del worker."""
    trabajos = {}
    with open(ruta, encoding="utf-8", errors="replace") as f:
        for linea in f:
            linea = linea.strip()
            if not linea.startswith("{") or "job media:process" not in linea:
                continue
            try:
                ev = json.loads(linea)
            except json.JSONDecodeError:
                continue
            ident = ev.get("id", "")
            if not ident.startswith("media-originals/"):
                continue
            clave = ident[len("media-"):]
            t = trabajos.setdefault(clave, {})
            if ev.get("estado") == "started":
                t["inicio"] = instante(ev["time"])
            elif ev.get("estado") == "completed":
                t["fin"] = instante(ev["time"])
                ms = ev.get("duración_ms") or ev.get("duracion_ms")
                if ms is not None:
                    t["duracion_s"] = ms / 1000.0
    return trabajos


def leer_objetos(ruta):
    """{clave_objeto: (bytes, instante_de_subida)} desde `gcloud storage ls -l`."""
    objetos = {}
    patron = re.compile(r"^\s*(\d+)\s+(\S+)\s+gs://[^/]+/(originals/\S+)$")
    with open(ruta, encoding="utf-8", errors="replace") as f:
        for linea in f:
            m = patron.match(linea.rstrip())
            if m:
                objetos[m.group(3)] = (int(m.group(1)), instante(m.group(2)))
    return objetos


def una(valor, n):
    """Un percentil con una decimal, o por que no hay numero."""
    if valor is None:
        return f"n/d (n={n})"
    return f"{valor:.1f}"


def resumen(nombre, valores, unidad="s"):
    n = len(valores)
    if n == 0:
        return f"  {nombre:<34} sin observaciones"
    v = sorted(valores)
    ps = cc.percentiles(valores)
    return (f"  {nombre:<34} n={n:<3} min {v[0]:7.1f}  mediana {una(ps[50], n):>10}  "
            f"max {v[-1]:7.1f}  media {statistics.mean(v):7.1f} {unidad}")


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--worker-log", required=True)
    p.add_argument("--objetos", required=True)
    args = p.parse_args()

    trabajos = leer_worker(args.worker_log)
    objetos = leer_objetos(args.objetos)

    filas = []
    for clave, t in trabajos.items():
        if clave not in objetos or "duracion_s" not in t or "inicio" not in t:
            continue
        tam, subido = objetos[clave]
        perfil, segundos = PERFILES.get(tam, (f"desconocido({tam})", 0.0))
        espera = (t["inicio"] - subido).total_seconds()
        total = (t["fin"] - subido).total_seconds()
        filas.append({
            "perfil": perfil, "video_s": segundos,
            "etapa4_s": espera, "etapa5_s": t["duracion_s"], "etapa6_s": total,
            "factor": t["duracion_s"] / segundos if segundos else 0.0,
        })

    if not filas:
        print("No se pudo emparejar ningun trabajo. Revisa las rutas.", file=sys.stderr)
        return 1

    print(f"Trabajos reconstruidos: {len(filas)} de {len(trabajos)} en el log del worker\n")

    print("== Etapa 5: procesamiento real del worker, por perfil")
    print("   (duración_ms del propio worker; el factor es cuánto cuesta por segundo de vídeo)")
    for perfil in ("corto", "medio", "largo"):
        sub = [f for f in filas if f["perfil"] == perfil]
        if not sub:
            continue
        d = [f["etapa5_s"] for f in sub]
        fac = [f["factor"] for f in sub]
        print(resumen(f"{perfil} ({sub[0]['video_s']:.0f} s de vídeo)", d))
        print(f"  {'':34} factor medio {statistics.mean(fac):.2f}x la duración del vídeo")

    print("\n== Etapas 4 y 6, origen = fin del PUT del original")
    for perfil in ("corto", "medio", "largo"):
        sub = [f for f in filas if f["perfil"] == perfil]
        if not sub:
            continue
        print(resumen(f"{perfil}: espera en cola", [f["etapa4_s"] for f in sub]))
        print(resumen(f"{perfil}: total hasta disponible", [f["etapa6_s"] for f in sub]))

    print("\n== Agregado de los tres perfiles (mezcla de perfiles: úsalo con cuidado)")
    for etiqueta, clave in (("espera en cola", "etapa4_s"),
                            ("procesamiento", "etapa5_s"),
                            ("total hasta disponible", "etapa6_s")):
        vals = [f[clave] for f in filas]
        ps = cc.percentiles(vals)
        n = len(vals)
        print(f"  {etiqueta:<24} p50 {una(ps[50], n):>10}  "
              f"p95 {una(ps[95], n):>10}  p99 {una(ps[99], n):>10}")
    print("\n  Con 27 observaciones el p95 existe (hacen falta 20) y el p99 no (hacen falta 100).")
    print("  Por perfil no llegan a 20, así que ahí se publica mediana, mínimo y máximo.")

    bloques = [(et, cc.percentiles([f[cl] for f in filas]))
               for et, cl in (("etapa4", "etapa4_s"), ("etapa5", "etapa5_s"), ("etapa6", "etapa6_s"))]
    cc.exigir_monotonia(bloques)
    return 0


if __name__ == "__main__":
    sys.exit(main())
