#!/usr/bin/env python3
"""Resume un .jtl (CSV) del escenario 1 (issue #132, H2).

Separa lo que importa para el informe de capacidad:
  * exitos y fallos por paso (etiqueta),
  * fallos clasificados: regla de negocio / configuracion (4xx) frente a fallos
    reales (5xx, timeouts, errores de conexion),
  * fallos de VALIDACION: la respuesta fue 200 pero el estado no era el esperado,
  * latencias p50/p95/p99 y rendimiento.

Uso: python scripts/capacity_resumen_jtl.py resultados.jtl [segundos_de_calentamiento_a_omitir]
     [--exigir-destino] [--exigir-nube]
No imprime cabeceras ni cuerpos (el .jtl no los guarda).

Los percentiles y las reglas de publicacion salen de capacity_criterios.py
(issue #198, A6): un solo estimador, minimo de observaciones por percentil,
comprobacion de monotonia antes de publicar y destino de la corrida registrado.
"""
import csv
import sys
from collections import Counter, defaultdict

import capacity_criterios as cc

# Un solo estimador para todo el repositorio. Tener dos ya costo un criterio
# de la rubrica: ver capacity_criterios.py.
pct = cc.percentil


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
    import os

    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    banderas = {a for a in sys.argv[1:] if a.startswith("--")}
    path = args[0]
    skip_s = float(args[1]) if len(args) > 1 else 0.0

    # Destino de la corrida. La Entrega 2 publico como nube una corrida contra
    # Docker Compose porque la salida cruda no decia contra que se habia medido.
    directorio = os.path.dirname(os.path.abspath(path))
    publicable = True
    destino = None
    if "--exigir-destino" in banderas or "--exigir-nube" in banderas:
        destino = cc.leer_destino(directorio, exigir_remoto="--exigir-nube" in banderas)
    elif os.path.exists(os.path.join(directorio, cc.ARCHIVO_DESTINO)):
        destino = cc.leer_destino(directorio)
    else:
        publicable = False

    if destino:
        print(cc.describir_destino(destino))
    else:
        print("Destino      : SIN REGISTRAR")
        print("               Esta corrida NO ES PUBLICABLE (A6): no consta contra")
        print("               que entorno se midio. Reejecutala con el guion, que")
        print("               escribe destino.json, o no uses sus cifras.")
    print()

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
    # Los percentiles se imprimen con la regla de observaciones minimas: un p95
    # con 8 muestras es el maximo con otro nombre, y un paso que se ejecuta una
    # vez por sesion junta pocas muestras aunque la corrida sea larga.
    print(f"\n{'paso':50s} {'n':>6s} {'p50':>12s} {'p95':>12s} {'p99':>12s} {'max':>6s} {'fallos':>7s}")
    bloques = []
    for label in sorted(por_label):
        v = por_label[label]
        ps = cc.percentiles(v)
        bloques.append((label, ps))
        print(f"{label[:50]:50s} {len(v):6d} "
              f"{cc.formatear(ps[50], len(v)):>12s} {cc.formatear(ps[95], len(v)):>12s} "
              f"{cc.formatear(ps[99], len(v)):>12s} {max(v):6d} {fallos[label]:7d}")

    todos = [int(r["elapsed"]) for r in medidas]
    globales = cc.percentiles(todos)
    bloques.append(("global", globales))

    # Ventana de agregacion: desde el inicio de la primera peticion medida hasta
    # el fin de la ultima. Antes el rendimiento se dividia por la distancia entre
    # los *inicios*, que deja fuera la ultima peticion, mientras que la duracion
    # publicada incluia el calentamiento: dos ventanas distintas en la misma
    # tabla. A6 exige una sola, declarada.
    ventana = max(1.0, (max(int(r["timeStamp"]) + int(r["elapsed"]) for r in medidas)
                        - min(int(r["timeStamp"]) for r in medidas)) / 1000.0)
    print(f"\nVentana de agregacion: {ventana:.0f} s (medidas, sin el calentamiento)"
          f"  | corrida completa: {span:.0f} s")
    print(f"Global ms: p50={cc.formatear(globales[50], len(todos))} "
          f"p95={cc.formatear(globales[95], len(todos))} "
          f"p99={cc.formatear(globales[99], len(todos))}"
          f"  | rendimiento: {len(medidas)/ventana:.1f} peticiones/s")

    # La comprobacion vive aqui, en el script que publica, y no en una revision
    # aparte: una revision aparte es exactamente lo que no ocurrio en la
    # Entrega 2.
    cc.exigir_monotonia(bloques)

    # Timeouts: las peticiones sin respuesta HTTP cuyo motivo es un tiempo agotado.
    timeouts = sum(1 for r in medidas
                   if not r["responseCode"].isdigit() and "timed out" in (r["responseMessage"] + r["failureMessage"]).lower())
    print(f"Timeouts: {timeouts}")

    # Salida para maquina: la lee el orquestador de H3 (scripts/h3_nube.sh) para decidir
    # si sube de nivel, y el analisis de la evidencia.
    import json
    resumen_json = {
        # `publicable` es lo que decide si estas cifras pueden entrar en un
        # informe. Falso cuando no consta el destino de la corrida.
        "publicable": publicable,
        "destino": destino,
        "muestras": len(medidas), "omitidas_calentamiento_s": skip_s,
        "duracion_corrida_s": round(span, 1), "ventana_agregacion_s": round(ventana, 1),
        "hilos_max": max(int(r["allThreads"]) for r in rows),
        "fallos_reales": reales, "fallos_reales_pct": round(100 * reales / total, 3),
        "fallos_validacion": valid,
        "rechazos_negocio": sum(v for k, v in clases.items() if k.startswith("negocio")),
        "timeouts": timeouts,
        # `ms` son los percentiles en crudo: los lee el orquestador para decidir
        # si para la escalera, y para eso conviene el valor aunque haya pocas
        # muestras --un criterio de parada es conservador, no una publicacion--.
        # `ms_publicables` es lo que puede salir en una tabla: nulo donde no hay
        # observaciones para sostenerlo.
        "ms": {"p50": pct(todos, 50), "p95": pct(todos, 95), "p99": pct(todos, 99), "max": max(todos)},
        "ms_publicables": {f"p{p}": v for p, v in globales.items()},
        "rps": round(len(medidas) / ventana, 2),
        "pasos": {l: {"n": len(v), "p50": pct(v, 50), "p95": pct(v, 95), "p99": pct(v, 99), "max": max(v), "fallos": fallos[l],
                      "publicables": {f"p{p}": v2 for p, v2 in cc.percentiles(v).items()}}
                  for l, v in por_label.items()},
    }
    salida = os.path.join(directorio, "resumen.json")
    with open(salida, "w", encoding="utf-8", newline="\n") as f:
        json.dump(resumen_json, f, indent=2, ensure_ascii=False)

    msgs = Counter(r["failureMessage"][:110] for r in medidas if r["success"] != "true" and r["failureMessage"])
    if msgs:
        print("\nMensajes de fallo mas frecuentes:")
        for m, v in msgs.most_common(8):
            print(f"  {v:5d}  {m}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
