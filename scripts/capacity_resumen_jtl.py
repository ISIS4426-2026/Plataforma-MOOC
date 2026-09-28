#!/usr/bin/env python3
"""Resume un .jtl (CSV) del escenario 1 (issue #132, H2).

Separa lo que importa para el informe de capacidad:
  * exitos y fallos por paso (etiqueta),
  * fallos clasificados: regla de negocio / configuracion (4xx) frente a fallos
    reales (5xx, timeouts, errores de conexion),
  * fallos de VALIDACION: la respuesta fue 200 pero el estado no era el esperado,
  * latencias p50/p95/p99 y rendimiento.

Uso: python scripts/capacity_resumen_jtl.py resultados.jtl [segundos_de_calentamiento_a_omitir]
No imprime cabeceras ni cuerpos (el .jtl no los guarda).
"""
import csv
import sys
from collections import Counter, defaultdict


def pct(values, p):
    if not values:
        return 0
    values = sorted(values)
    k = max(0, min(len(values) - 1, int(round(p / 100 * len(values) + 0.5)) - 1))
    return values[k]


def clase(code, ok, msg):
    if code.isdigit():
        c = int(code)
        if 200 <= c < 300:
            return "validacion" if not ok else "ok"
        if c == 429:
            return "negocio: 429 limite de tasa"
        if 400 <= c < 500:
            return f"negocio/config: HTTP {c}"
        if c >= 500:
            return f"REAL: HTTP {c}"
    return "REAL: sin respuesta HTTP (" + (msg or code)[:40] + ")"


def main():
    path = sys.argv[1]
    skip_s = float(sys.argv[2]) if len(sys.argv) > 2 else 0.0
    rows = list(csv.DictReader(open(path, encoding="utf-8", newline="")))
    rows = [r for r in rows if not r["label"].startswith("_")]
    if not rows:
        print("El archivo no tiene muestras.")
        return 1
    t0 = min(int(r["timeStamp"]) for r in rows)
    medidas = [r for r in rows if (int(r["timeStamp"]) - t0) / 1000.0 >= skip_s]
    if not medidas:
        print("Ninguna muestra queda tras omitir el calentamiento: la corrida fue mas corta que ese margen.")
        return 1
    t1 = max(int(r["timeStamp"]) + int(r["elapsed"]) for r in rows)
    span = (t1 - t0) / 1000.0

    print(f"Muestras totales: {len(rows)}  | consideradas (tras omitir {skip_s:.0f} s de calentamiento): {len(medidas)}")
    print(f"Duracion de la corrida: {span:.0f} s  | hilos maximos: {max(int(r['allThreads']) for r in rows)}")

    clases = Counter(clase(r["responseCode"], r["success"] == "true", r["responseMessage"]) for r in medidas)
    total = len(medidas)
    print("\nResultado de las peticiones medidas:")
    for k, v in sorted(clases.items(), key=lambda kv: -kv[1]):
        print(f"  {v:7d}  {100 * v / total:6.2f}%  {k}")
    reales = sum(v for k, v in clases.items() if k.startswith("REAL"))
    valid = clases.get("validacion", 0)
    print(f"\nFallos reales: {reales} ({100 * reales / total:.2f}%)  | fallos de validacion funcional: {valid} ({100 * valid / total:.2f}%)")

    por_label = defaultdict(list)
    fallos = defaultdict(int)
    for r in medidas:
        por_label[r["label"]].append(int(r["elapsed"]))
        if r["success"] != "true":
            fallos[r["label"]] += 1
    print(f"\n{'paso':58s} {'n':>6s} {'p50':>6s} {'p95':>6s} {'p99':>6s} {'max':>6s} {'fallos':>7s}")
    for label in sorted(por_label):
        v = por_label[label]
        print(f"{label[:58]:58s} {len(v):6d} {pct(v,50):6d} {pct(v,95):6d} {pct(v,99):6d} {max(v):6d} {fallos[label]:7d}")

    todos = [int(r["elapsed"]) for r in medidas]
    ventana = max(1.0, (max(int(r["timeStamp"]) for r in medidas) - min(int(r["timeStamp"]) for r in medidas)) / 1000.0)
    print(f"\nGlobal ms: p50={pct(todos,50)} p95={pct(todos,95)} p99={pct(todos,99)}  | rendimiento: {len(medidas)/ventana:.1f} peticiones/s")

    # Timeouts: las peticiones sin respuesta HTTP cuyo motivo es un tiempo agotado.
    timeouts = sum(1 for r in medidas
                   if not r["responseCode"].isdigit() and "timed out" in (r["responseMessage"] + r["failureMessage"]).lower())
    print(f"Timeouts: {timeouts}")

    # Salida para maquina: la lee el orquestador de H3 (scripts/h3_nube.sh) para decidir
    # si sube de nivel, y el analisis de la evidencia.
    import json
    import os
    resumen_json = {
        "muestras": len(medidas), "omitidas_calentamiento_s": skip_s, "duracion_s": round(span, 1),
        "hilos_max": max(int(r["allThreads"]) for r in rows),
        "fallos_reales": reales, "fallos_reales_pct": round(100 * reales / total, 3),
        "fallos_validacion": valid,
        "rechazos_negocio": sum(v for k, v in clases.items() if k.startswith("negocio")),
        "timeouts": timeouts,
        "ms": {"p50": pct(todos, 50), "p95": pct(todos, 95), "p99": pct(todos, 99), "max": max(todos)},
        "rps": round(len(medidas) / ventana, 2),
        "pasos": {l: {"n": len(v), "p50": pct(v, 50), "p95": pct(v, 95), "p99": pct(v, 99), "max": max(v), "fallos": fallos[l]}
                  for l, v in por_label.items()},
    }
    destino = os.path.join(os.path.dirname(os.path.abspath(path)), "resumen.json")
    with open(destino, "w", encoding="utf-8", newline="\n") as f:
        json.dump(resumen_json, f, indent=2, ensure_ascii=False)

    msgs = Counter(r["failureMessage"][:110] for r in medidas if r["success"] != "true" and r["failureMessage"])
    if msgs:
        print("\nMensajes de fallo mas frecuentes:")
        for m, v in msgs.most_common(8):
            print(f"  {v:5d}  {m}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
