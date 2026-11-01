#!/usr/bin/env python3
"""Convierte lineas de una bitacora de consola en una imagen PNG, como la captura de una terminal.

El issue E1 pide capturas "mientras ocurre": el entorno se destruye en I6 y las metricas caducan, asi que no se
puede recrear despues. Las bitacoras de los guiones llevan la hora de cada linea; esta herramienta las dibuja tal
cual (sin editar el texto) para que la prueba tenga forma de captura. No inventa nada: cada linea que aparece esta en
el archivo de origen, que se conserva al lado, y la imagen dice de que archivo y de que lineas salio.

Uso:
  python scripts/render_consola.py <bitacora> <salida.png> --titulo "texto" [--patron REGEX]... [--desde N --hasta M]
                                   [--max-ancho 150]

  --patron   se pueden repetir; se conservan las lineas que coincidan con ALGUNO (en el orden del archivo)
  --desde/--hasta  rango de lineas (1-indexado, inclusivo); si se dan junto con --patron, primero se recorta y luego se filtra
"""
import re
import sys
import textwrap

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt  # noqa: E402


def leer_argumentos(argv):
    args = {"patrones": [], "desde": None, "hasta": None, "titulo": "", "ancho": 150}
    pos = []
    i = 0
    while i < len(argv):
        a = argv[i]
        if a == "--patron":
            args["patrones"].append(argv[i + 1]); i += 2
        elif a == "--desde":
            args["desde"] = int(argv[i + 1]); i += 2
        elif a == "--hasta":
            args["hasta"] = int(argv[i + 1]); i += 2
        elif a == "--titulo":
            args["titulo"] = argv[i + 1]; i += 2
        elif a == "--max-ancho":
            args["ancho"] = int(argv[i + 1]); i += 2
        else:
            pos.append(a); i += 1
    if len(pos) != 2:
        sys.exit(__doc__)
    return pos[0], pos[1], args


def main():
    origen, salida, a = leer_argumentos(sys.argv[1:])
    with open(origen, encoding="utf-8", errors="replace") as f:
        lineas = [ln.rstrip("\n").rstrip("\r") for ln in f]
    numeradas = list(enumerate(lineas, start=1))
    if a["desde"] or a["hasta"]:
        numeradas = [(n, t) for n, t in numeradas if (a["desde"] or 1) <= n <= (a["hasta"] or len(lineas))]
    if a["patrones"]:
        rx = [re.compile(p) for p in a["patrones"]]
        numeradas = [(n, t) for n, t in numeradas if any(r.search(t) for r in rx)]
    if not numeradas:
        sys.exit("ninguna linea coincide: no se genera una imagen vacia")

    visibles = []
    for n, t in numeradas:
        partes = textwrap.wrap(t, a["ancho"], subsequent_indent="      ", break_long_words=True) or [""]
        visibles.extend(partes)
    pie = f"Render de la salida de consola de la corrida · origen: {origen.replace(chr(92), '/')} · lineas {numeradas[0][0]}–{numeradas[-1][0]} ({len(numeradas)} mostradas)"
    alto = 0.19 * (len(visibles) + 4) + 0.6
    fig = plt.figure(figsize=(14, max(2.0, alto)), facecolor="#0c0c0c")
    fig.text(0.012, 1 - 0.28 / fig.get_figheight(), a["titulo"], color="#9ecbff", family="monospace", fontsize=10.5, fontweight="bold", va="top")
    y = 1 - 0.68 / fig.get_figheight()
    paso = 0.19 / fig.get_figheight()
    for ln in visibles:
        color = "#e6e6e6"
        if "FALLO" in ln or "ERROR" in ln:
            color = "#ff7b72"
        elif "] OK:" in ln or ln.lstrip().startswith("[OK"):
            color = "#7ee787"
        elif "---" in ln or "===" in ln:
            color = "#9ecbff"
        fig.text(0.012, y, ln, color=color, family="monospace", fontsize=8.2, va="top")
        y -= paso
    fig.text(0.012, 0.2 / fig.get_figheight(), pie, color="#8b949e", family="monospace", fontsize=7.4, va="bottom")
    fig.savefig(salida, dpi=130, facecolor=fig.get_facecolor())
    plt.close(fig)
    print("escrito", salida, f"({len(visibles)} lineas dibujadas)")


if __name__ == "__main__":
    main()
