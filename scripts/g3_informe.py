#!/usr/bin/env python3
"""Tablas de G3 (issue #223) a partir de las corridas del escenario 1 sobre la arquitectura elastica.

Uso: python scripts/g3_informe.py <directorio_de_resultados> [salida.md]

Lee, de cada corrida, resumen.json, resultados.jtl, estado.txt, destino.json, metricas_gcp.json y
contraste_generador.json (los que dejan scripts/h3_nube.sh y scripts/capacity_postproceso.sh). Solo lectura. Aplica
las reglas de A6 (capacity_criterios): un percentil sin observaciones suficientes se escribe `n/d (n=k)`.

Etiquetas: <operando>-<serie>... donde operando es R (API fija en 1) o E (elastica) y serie A, B, C o S con su nivel
(`E-A-nivel200`, `E-A200-r2`, `R-B200-r1`, `E-S200-r7`). Las dos formas de A y B son la misma serie.
"""
import csv
import glob
import json
import os
import re
import statistics as st
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import capacity_criterios as cc  # noqa: E402

PATRON = re.compile(r"^(?P<op>[RE])-(?P<serie>[ABCS])-?(?:nivel)?(?P<n>\d+)(?:-r(?P<r>\d+))?_\d{8}_\d{6}$")
A7 = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "docs", "entrega3", "evidencias", "A7",
                  "escenario1-1st-architecture")


def g(x, *ks):
    for k in ks:
        x = x.get(k) if isinstance(x, dict) else None
    return x


def leer_jtl(d):
    with open(os.path.join(d, "resultados.jtl"), encoding="utf-8", errors="replace", newline="") as f:
        return list(csv.DictReader(f))


def calientes(filas, omitir_s=0.0):
    """Latencia del servidor sobre conexion ya abierta (Connect = 0), tras el calentamiento (regla de A7 §1.4)."""
    if not filas:
        return []
    t0 = min(int(r["timeStamp"]) for r in filas) + int(omitir_s * 1000)
    return [int(r["elapsed"]) for r in filas
            if r.get("Connect", "0") in ("0", "") and int(r["timeStamp"]) >= t0 and r["responseCode"] == "200"]


def pct(vals, p):
    return cc.formatear(cc.percentiles(vals, (p,))[p], len(vals)) if vals else "n/d"


def cargar_corrida(d):
    nombre = os.path.basename(d)
    m = PATRON.match(nombre)
    if not m or not os.path.exists(os.path.join(d, "resumen.json")):
        return None
    r = json.load(open(os.path.join(d, "resumen.json"), encoding="utf-8"))
    met = json.load(open(os.path.join(d, "metricas_gcp.json"), encoding="utf-8")) if os.path.exists(os.path.join(d, "metricas_gcp.json")) else {}
    con = json.load(open(os.path.join(d, "contraste_generador.json"), encoding="utf-8")) if os.path.exists(os.path.join(d, "contraste_generador.json")) else {}
    estado = open(os.path.join(d, "estado.txt"), encoding="utf-8").read().strip() if os.path.exists(os.path.join(d, "estado.txt")) else "?"
    omitir = r.get("omitidas_calentamiento_s", 0)
    warm = calientes(leer_jtl(d), omitir)
    return {
        "nombre": nombre.split("_2026")[0], "op": m["op"], "serie": m["serie"], "n": int(m["n"]), "rep": int(m["r"] or 1),
        "clave": f"{m['serie']}{m['n']}", "r": r, "met": met, "estado_ok": estado.endswith("OK"),
        "limita": con.get("veredicto") or con.get("resultado") or "?", "warm": warm,
        "inicio": r["destino"]["inicio_utc"],
    }


def celda_pct(r, p):
    pub = r.get("ms_publicables")
    if pub is not None:
        return cc.formatear(pub.get(f"p{p}"), r.get("muestras", 0))
    return f"{r['ms'][f'p{p}']}*"


def fila(c):
    r, m = c["r"], c["met"]
    api = g(m, "grupos", "api") or {}
    return (f"| `{c['nombre']}` | {c['n']} | {r['rps']:.1f} | {celda_pct(r, 50)} | {celda_pct(r, 95)} | {celda_pct(r, 99)} | "
            f"{pct(c['warm'], 50)} / {pct(c['warm'], 95)} / {pct(c['warm'], 99)} | "
            f"{r['fallos_reales']} | {r['timeouts']} | {r['fallos_validacion']} | {'OK' if c['estado_ok'] else '**NO CUADRA**'} | "
            f"{g(api, 'instancias_operativas', 'max')} | {g(api, 'cpu_ocupada_pct_max_de_una_instancia', 'max')} | "
            f"{g(m, 'cloudsql', 'cpu_pct', 'max')} | {g(m, 'cloudsql', 'conexiones_abiertas', 'max')} | "
            f"{g(m, 'cloudsql', 'transacciones_por_segundo', 'max')} | {c['limita']} |")


def est(vals):
    vals = [v for v in vals if v is not None]
    if not vals:
        return "n/d"
    if len(vals) == 1:
        return f"{vals[0]:.1f} (1 corrida)"
    return f"{st.mean(vals):.1f} ± {st.stdev(vals):.1f} [{min(vals):.1f}–{max(vals):.1f}]"


def numeros(c, que):
    r, m = c["r"], c["met"]
    api = g(m, "grupos", "api") or {}
    return {
        "rps": r["rps"], "p50": r["ms"]["p50"], "p95": r["ms"]["p95"], "p99": r["ms"]["p99"],
        "w50": cc.percentiles(c["warm"], (50,))[50] if c["warm"] else None,
        "w95": cc.percentiles(c["warm"], (95,))[95] if len(c["warm"]) >= 20 else None,
        "cpu_api": g(api, "cpu_ocupada_pct_max_de_una_instancia", "max"), "inst": g(api, "instancias_operativas", "max"),
        "cpu_sql": g(m, "cloudsql", "cpu_pct", "max"), "conx": g(m, "cloudsql", "conexiones_abiertas", "max"),
        "tps": g(m, "cloudsql", "transacciones_por_segundo", "max"),
    }[que]


def a7_referencia():
    out = []
    for etiqueta in ("A-nivel200", "B-nivel200"):
        ds = sorted(glob.glob(os.path.join(A7, etiqueta + "_*")))
        if not ds:
            continue
        d = ds[0]
        r = json.load(open(os.path.join(d, "resumen.json"), encoding="utf-8"))
        warm = calientes(leer_jtl(d), r.get("omitidas_calentamiento_s", 0))
        out.append((etiqueta, r, warm))
    return out


def main():
    raiz = sys.argv[1]
    salida = sys.argv[2] if len(sys.argv) > 2 else None
    corridas = [c for c in (cargar_corrida(d) for d in sorted(glob.glob(os.path.join(raiz, "*_2026*")))) if c]
    corridas.sort(key=lambda c: c["inicio"])
    # Una corrida que el contraste del generador no puede dar por buena no se publica (A6/G1): se lista aparte.
    descartadas = [c for c in corridas if str(c["limita"]).startswith("LIMITADO")]
    corridas = [c for c in corridas if c not in descartadas]
    L = []
    w = L.append

    w("## 1. Cada corrida\n")
    w("`p50/p95/p99` son los globales del `.jtl` (incluyen abrir la conexión); `caliente` es la latencia del servidor sobre conexión ya abierta, "
      "que es la comparable entre generadores (A7 §1.4). Instancias, CPU de la API y la base, conexiones y transacciones son el máximo de la ventana medida "
      "(Cloud Monitoring, una muestra por minuto). `generador` es el veredicto de `capacity_contraste_generador.py`.\n")
    w("| corrida | usr | pet./s | p50 | p95 | p99 | caliente p50 / p95 / p99 | fallos | t/o | valid. | base | API inst. | CPU API máx % | CPU SQL máx % | conx. SQL | tps SQL | generador |")
    w("| :--- | ---: | ---: | ---: | ---: | ---: | :---: | ---: | ---: | ---: | :---: | ---: | ---: | ---: | ---: | ---: | :---: |")
    for c in corridas:
        w(fila(c))

    w("\n## 2. Variación entre repeticiones (media ± desviación estándar [mínimo–máximo])\n")
    w("| operando | serie | corridas | pet./s | p95 (ms) | p99 (ms) | caliente p50 (ms) | caliente p95 (ms) | CPU API máx % | CPU SQL máx % | conx. SQL máx |")
    w("| :---: | :---: | ---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: |")
    grupos = {}
    for c in corridas:
        if c["serie"] == "S":
            continue
        grupos.setdefault((c["op"], c["clave"]), []).append(c)
    orden = {"A": 0, "B": 1, "C": 2}
    for (op, clave), cs in sorted(grupos.items(), key=lambda kv: (orden[kv[0][1][0]], int(kv[0][1][1:]), kv[0][0])):
        w(f"| {op} | {clave} | {len(cs)} | " + " | ".join(est([numeros(c, k) for c in cs]) for k in
                                                       ("rps", "p95", "p99", "w50", "w95", "cpu_api", "cpu_sql", "conx")) + " |")

    w("\n## 3. R frente a E, mismo generador y mismo código (el efecto de la elasticidad)\n")
    w("| serie | operando | pet./s | p95 caliente (ms) | CPU API máx % | CPU SQL máx % | instancias de API (máx.) |")
    w("| :--- | :---: | :---: | :---: | :---: | :---: | :---: |")
    for clave in ("A200", "B200", "C200"):
        for op in ("R", "E"):
            cs = grupos.get((op, clave), [])
            if cs:
                w(f"| {clave} | {op} | {est([numeros(c, 'rps') for c in cs])} | {est([numeros(c, 'w95') for c in cs])} | "
                  f"{est([numeros(c, 'cpu_api') for c in cs])} | {est([numeros(c, 'cpu_sql') for c in cs])} | {est([numeros(c, 'inst') for c in cs])} |")

    s = [c for c in corridas if c["serie"] == "S"]
    if s:
        w("\n## 4. Serie sostenida (E): una corrida tras otra\n")
        w("| corrida | inicio (UTC) | pet./s | p95 caliente (ms) | CPU API máx % | instancias de API (máx.) | CPU SQL máx % |")
        w("| :--- | :---: | ---: | ---: | ---: | ---: | ---: |")
        for c in s:
            w(f"| `{c['nombre']}` | {c['inicio'][11:19]} | {c['r']['rps']:.1f} | {pct(c['warm'], 95)} | {numeros(c, 'cpu_api')} | {numeros(c, 'inst')} | {numeros(c, 'cpu_sql')} |")

    ref = a7_referencia()
    if ref:
        w("\n## 5. Referencia: línea base de A7 (primera arquitectura, portátil, código `3affa11`)\n")
        w("Solo las magnitudes comparables entre generadores (A7 §1.4): rendimiento, latencia del servidor sobre conexión caliente y errores. "
          "Los p95 y p99 globales **no** se comparan.\n")
        w("| corrida de A7 | pet./s | caliente p50 / p95 / p99 (ms) | fallos |")
        w("| :--- | ---: | :---: | ---: |")
        for etiqueta, r, warm in ref:
            w(f"| `{etiqueta}` | {r['rps']:.1f} | {pct(warm, 50)} / {pct(warm, 95)} / {pct(warm, 99)} | {r['fallos_reales']} |")

    texto = "\n".join(L) + "\n"
    if salida:
        open(salida, "w", encoding="utf-8", newline="\n").write(texto)
        print("escrito", salida)
    else:
        print(texto)


if __name__ == "__main__":
    main()
