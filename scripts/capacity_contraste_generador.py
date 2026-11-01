#!/usr/bin/env python3
"""Comprueba que el generador de carga NO limita la medicion de una corrida (G1, issue #222).

El enunciado de la Entrega 3 pide ejecutar el generador fuera de las maquinas de aplicacion y
"comprobar que no limita las mediciones". Un generador saturado mide su propia cola, no a la
plataforma (A7 lo vio: la red del generador no reproduce lo publicado). Esta comprobacion no confia
en una sola senal; cruza tres, y cada una puede rechazar la corrida:

  1. RECURSOS DEL GENERADOR  la CPU y la memoria que muestreo `generador.csv` durante la corrida
                             (CPU del contenedor de JMeter, en % de UN nucleo, y memoria usada)
  2. LATENCIA CONTRASTADA    la latencia que ve el generador (resumen.json, JMeter) frente a la que ve el
                             balanceador (metricas_gcp.json, Cloud Monitoring). La diferencia es red mas
                             cola en el generador; si es grande, el generador esta esperando
  3. SOLICITUDES CONTADAS    las que dice haber enviado el generador frente a las que contó el balanceador;
                             si faltan, se perdieron antes de salir (conexiones, puertos, red)

Uso:
  python scripts/capacity_contraste_generador.py <directorio_de_una_corrida> [--nucleos N] [--memoria-gib G]
  python scripts/capacity_contraste_generador.py --todas <directorio_de_resultados> [--nucleos N]
      (sin --nucleos usa los que run_escenario1.sh dejo en destino.json: nucleos_generador)
  python scripts/capacity_contraste_generador.py --autoprueba

El directorio de una corrida trae resumen.json, generador.csv y metricas_gcp.json. Escribe
`contraste_generador.json` en el y sale con 1 si ALGUNA corrida queda limitada por el generador:
una corrida asi no se publica (A6). No lee ni imprime credenciales.
"""
import csv
import json
import os
import re
import sys

# Umbrales, escritos aqui y en el documento G1 antes de la primera corrida (A6: criterios previos).
CPU_MAX_POR_NUCLEO_PCT = 70.0       # el contenedor de JMeter no pasa del 70 % de los nucleos que tiene
MEMORIA_MAX_PCT = 80.0              # ni del 80 % de la memoria de la maquina
BRECHA_P50_MAX_MS = 150.0           # generador - balanceador, p50: red + cola. Mas que esto, el generador espera
BRECHA_P95_MAX_MS = 400.0
SOLICITUDES_MIN_RATIO = 0.95        # el balanceador debe haber contado al menos el 95 % de lo enviado


def num(texto):
    m = re.match(r"\s*([\d.]+)", texto or "")
    return float(m.group(1)) if m else None


def memoria_gib(texto):
    """'532.2MiB / 7.701GiB' -> (usada_gib, total_gib)."""
    partes = [p.strip() for p in (texto or "").split("/")]
    if len(partes) != 2:
        return None, None

    def a_gib(p):
        v = num(p)
        if v is None:
            return None
        return v / 1024.0 if "MiB" in p else v / (1024.0 * 1024.0) if "KiB" in p else v

    return a_gib(partes[0]), a_gib(partes[1])


def evaluar(resumen, generador_filas, metricas, nucleos, memoria_total_gib=None):
    """Devuelve (veredicto, razones, medidas). Pura: la usan la autoprueba y las corridas."""
    razones, medidas = [], {}

    # 1. recursos del generador
    cpus = [num(f["cpu_pct"]) for f in generador_filas if num(f.get("cpu_pct")) is not None]
    if cpus:
        # el primer muestreo de un contenedor que arranca suele marcar un pico de arranque: se declara, no se oculta
        pico = max(cpus)
        ordenadas = sorted(cpus)
        p95 = ordenadas[min(len(ordenadas) - 1, int(0.95 * len(ordenadas)))]
        limite = CPU_MAX_POR_NUCLEO_PCT * nucleos
        medidas["generador_cpu_pct_de_un_nucleo"] = {"muestras": len(cpus), "p95": round(p95, 1), "max": round(pico, 1), "limite": limite}
        if p95 > limite:
            razones.append(f"CPU del generador p95={p95:.0f} % de un nucleo > {limite:.0f} % ({CPU_MAX_POR_NUCLEO_PCT:.0f} % de {nucleos} nucleos)")
    else:
        razones.append("generador.csv no trae muestras de CPU: no se puede demostrar que el generador no limita")
    mem = [memoria_gib(f.get("memoria"))[0] for f in generador_filas]
    total = memoria_total_gib or next((memoria_gib(f.get("memoria"))[1] for f in generador_filas if memoria_gib(f.get("memoria"))[1]), None)
    mem = [m for m in mem if m is not None]
    if mem and total:
        pct = 100.0 * max(mem) / total
        medidas["generador_memoria_pct"] = {"max_gib": round(max(mem), 2), "total_gib": round(total, 2), "max_pct": round(pct, 1), "limite": MEMORIA_MAX_PCT}
        if pct > MEMORIA_MAX_PCT:
            razones.append(f"memoria del generador {pct:.0f} % > {MEMORIA_MAX_PCT:.0f} %")

    # 2. latencia contrastada
    ms = (resumen.get("ms_publicables") or resumen.get("ms") or {})
    lb = ((metricas.get("balanceador") or {}).get("latencia_total_ms") or {})
    p50_lb = ((lb.get("p50_por_minuto") or {}).get("media"))
    p95_lb = ((lb.get("p95_por_minuto") or {}).get("media"))
    if ms.get("p50") is not None and p50_lb is not None:
        brecha50 = ms["p50"] - p50_lb
        brecha95 = (ms["p95"] - p95_lb) if (ms.get("p95") is not None and p95_lb is not None) else None
        medidas["latencia"] = {"generador_p50_ms": ms["p50"], "balanceador_p50_ms_media_por_minuto": round(p50_lb, 1), "brecha_p50_ms": round(brecha50, 1),
                               "generador_p95_ms": ms.get("p95"), "balanceador_p95_ms_media_por_minuto": None if p95_lb is None else round(p95_lb, 1),
                               "brecha_p95_ms": None if brecha95 is None else round(brecha95, 1)}
        if brecha50 > BRECHA_P50_MAX_MS:
            razones.append(f"el generador ve p50={ms['p50']} ms y el balanceador {p50_lb:.0f} ms: brecha {brecha50:.0f} ms > {BRECHA_P50_MAX_MS:.0f} ms")
        if brecha95 is not None and brecha95 > BRECHA_P95_MAX_MS:
            razones.append(f"brecha p95 {brecha95:.0f} ms > {BRECHA_P95_MAX_MS:.0f} ms")
    else:
        razones.append("falta la latencia del balanceador o del generador: no se puede contrastar")

    # 3. solicitudes contadas
    enviadas = resumen.get("muestras")
    contadas = (metricas.get("balanceador") or {}).get("solicitudes_totales")
    if enviadas and contadas is not None:
        ratio = contadas / enviadas
        medidas["solicitudes"] = {"enviadas_por_el_generador": enviadas, "contadas_por_el_balanceador": contadas, "ratio": round(ratio, 3), "minimo": SOLICITUDES_MIN_RATIO}
        if ratio < SOLICITUDES_MIN_RATIO:
            razones.append(f"el balanceador conto {contadas} de {enviadas} solicitudes ({ratio:.0%}): se perdieron antes de salir")
    else:
        razones.append("falta el conteo de solicitudes del generador o del balanceador")

    return ("NO LIMITA" if not razones else "LIMITADO"), razones, medidas


def leer_corrida(directorio):
    def cargar(nombre):
        ruta = os.path.join(directorio, nombre)
        return json.load(open(ruta, encoding="utf-8")) if os.path.exists(ruta) else {}

    filas = []
    ruta = os.path.join(directorio, "generador.csv")
    if os.path.exists(ruta):
        with open(ruta, encoding="utf-8") as f:
            filas = list(csv.DictReader(f))
    return cargar("resumen.json"), filas, cargar("metricas_gcp.json")


def procesar(directorio, nucleos, memoria):
    resumen, filas, metricas = leer_corrida(directorio)
    # Sin --nucleos, los nucleos y la memoria del generador salen de lo que la corrida dejo en destino.json.
    destino = resumen.get("destino") or {}
    nucleos = nucleos or destino.get("nucleos_generador")
    memoria = memoria or destino.get("memoria_generador_gib") or None
    if not nucleos:
        print(f"LIMITADO   {os.path.basename(os.path.normpath(directorio))}  <- la corrida no registra los nucleos del generador (destino.json) y no se paso --nucleos")
        return False
    veredicto, razones, medidas = evaluar(resumen, filas, metricas, nucleos, memoria)
    salida = {"veredicto": veredicto, "razones": razones, "medidas": medidas, "nucleos_del_generador": nucleos}
    with open(os.path.join(directorio, "contraste_generador.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(salida, f, indent=2, ensure_ascii=False)
        f.write("\n")
    nombre = os.path.basename(os.path.normpath(directorio))
    print(f"{veredicto:10s} {nombre}" + ("" if not razones else "  <- " + "; ".join(razones)))
    return veredicto == "NO LIMITA"


def autoprueba():
    ok = True

    def check(cond, msg):
        nonlocal ok
        print(("ok  " if cond else "FALLA ") + msg)
        ok = ok and cond

    filas_sanas = [{"cpu_pct": f"{c}%", "memoria": "600MiB / 7.7GiB"} for c in (20, 30, 40, 35, 25)]
    resumen = {"muestras": 1000, "ms_publicables": {"p50": 90, "p95": 300}}
    lb = {"balanceador": {"solicitudes_totales": 990, "latencia_total_ms": {"p50_por_minuto": {"media": 40}, "p95_por_minuto": {"media": 200}}}}
    v, r, _ = evaluar(resumen, filas_sanas, lb, nucleos=4)
    check(v == "NO LIMITA" and not r, "un generador holgado, con brecha de red normal, no limita")
    filas_cpu = [{"cpu_pct": "390%", "memoria": "600MiB / 7.7GiB"}] * 10
    v, r, _ = evaluar(resumen, filas_cpu, lb, nucleos=4)
    check(v == "LIMITADO" and any("CPU" in x for x in r), "con la CPU del contenedor al 97 % de 4 nucleos, limita")
    lento = {"muestras": 1000, "ms_publicables": {"p50": 900, "p95": 2500}}
    v, r, _ = evaluar(lento, filas_sanas, lb, nucleos=4)
    check(v == "LIMITADO" and any("brecha" in x for x in r), "si el generador ve 900 ms y el balanceador 40 ms, el generador espera")
    perdido = {"balanceador": {"solicitudes_totales": 600, "latencia_total_ms": {"p50_por_minuto": {"media": 40}, "p95_por_minuto": {"media": 200}}}}
    v, r, _ = evaluar(resumen, filas_sanas, perdido, nucleos=4)
    check(v == "LIMITADO" and any("solicitudes" in x for x in r), "si el balanceador conto el 60 %, hay perdidas antes de salir")
    v, r, _ = evaluar(resumen, [], lb, nucleos=4)
    check(v == "LIMITADO", "sin muestras de CPU no se demuestra nada, y no se da por bueno")
    check(memoria_gib("532.2MiB / 7.701GiB")[0] and abs(memoria_gib("532.2MiB / 7.701GiB")[0] - 0.52) < 0.01, "la memoria se lee en GiB")
    print("\nTodas las reglas se cumplen." if ok else "\nHay reglas que fallan.")
    return 0 if ok else 1


def main():
    args = sys.argv[1:]
    if args[:1] == ["--autoprueba"]:
        sys.exit(autoprueba())
    nucleos = None
    memoria = None
    if "--nucleos" in args:
        nucleos = float(args[args.index("--nucleos") + 1])
    if "--memoria-gib" in args:
        memoria = float(args[args.index("--memoria-gib") + 1])
    if not args:
        sys.exit(__doc__)
    if args[0] == "--todas":
        raiz = args[1]
        bien = True
        for nombre in sorted(os.listdir(raiz)):
            d = os.path.join(raiz, nombre)
            if os.path.exists(os.path.join(d, "resumen.json")):
                bien = procesar(d, nucleos, memoria) and bien
        sys.exit(0 if bien else 1)
    sys.exit(0 if procesar(args[0], nucleos, memoria) else 1)


if __name__ == "__main__":
    main()
