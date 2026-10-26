#!/usr/bin/env python3
"""Reglas de medicion de la Entrega 3, en codigo (issue #198, A6).

POR QUE EXISTE

La revision de la Entrega 2 invalido el escenario 2 por cuatro motivos, y tres
de ellos son defectos de instrumentacion, no de la plataforma:

  * se midio contra Docker Compose y se publico como nube,
  * el tiempo al primer cuadro se reporto como latencia HTTP mas 45 ms fijos,
  * hay p95 por debajo del p50 en las tablas publicadas.

El tercero tiene una causa localizable: habia **tres estimadores de percentil
distintos** en cuatro archivos (este modulo los sustituye a todos), y uno de
ellos --el de scripts/capacity_escenario2--, con pocas observaciones, devolvia
un indice de p95 menor que el de p50. Con n=2 el p95 era literalmente el valor
MINIMO. Ver `--autoprueba`, que lo reproduce.

Un criterio que vive solo en un documento no se cumple: se olvida a las dos
corridas. Lo que esta aqui se ejecuta, y falla la corrida si no se cumple.

Las reglas, y el documento que las justifica:
capacity-planning/criterios_de_medicion.md

USO COMO BIBLIOTECA

    from capacity_criterios import percentiles, verificar_monotonia, leer_destino

USO DIRECTO

    python scripts/capacity_criterios.py --autoprueba   # comprueba las reglas
    python scripts/capacity_criterios.py --destino <directorio_de_corrida>
"""
import json
import math
import os
import random
import sys

# --- Regla 1: un solo estimador -------------------------------------------
#
# Rango mas cercano con techo: indice = ceil(p/100 * n) - 1. Es monotono en p
# por construccion --ceil es no decreciente-- asi que p50 <= p95 <= p99 no
# puede fallar por el estimador. Si una tabla los muestra invertidos, la causa
# esta en los datos o en la ventana, no aqui.
#
# No es el unico estimador defendible; es el unico que usamos. Mezclar dos ya
# costo un criterio de la rubrica.


def percentil(valores, p):
    """Percentil p (0-100) por rango mas cercano. None si no hay datos."""
    if not valores:
        return None
    v = sorted(valores)
    k = max(0, min(len(v) - 1, math.ceil(p / 100.0 * len(v)) - 1))
    return v[k]


# --- Regla 2: numero minimo de observaciones ------------------------------
#
# Un percentil es un estadistico de orden: el pX solo existe si hay suficientes
# observaciones para que ese orden signifique algo. La cota es n >= 1/(1-X):
# 20 muestras para un p95, 100 para un p99. Por debajo, el "p95" es el maximo
# con otro nombre.
#
# Esto invalida *todos* los p95 por etapa del escenario 2 de la Entrega 2: los
# niveles subian 1, 2, 4, 8 y 12 videos. Ninguno llega a 20. No es que esos p95
# esten mal calculados --ademas lo estaban--, es que no existen.
#
# Consecuencia para G4: o el nivel sube >= 20 videos, o se publica mediana,
# minimo, maximo y las observaciones en crudo. Lo que no vale es un p95 de 8.
OBS_MINIMAS = {50: 5, 95: 20, 99: 100}

NO_DISPONIBLE = "n/d"


def percentiles(valores, ps=(50, 95, 99)):
    """Percentiles con la regla de observaciones minimas aplicada.

    Devuelve {p: valor} y pone None donde no hay muestras para sostenerlo. Un
    None no es un error de ejecucion: es la respuesta correcta, y quien formatea
    la tabla debe imprimir 'n/d (n=k)' en vez de un numero inventado.
    """
    n = len(valores)
    return {p: (percentil(valores, p) if n >= OBS_MINIMAS.get(p, 1) else None) for p in ps}


def formatear(valor, n, sufijo=""):
    """Celda de tabla: el numero, o por que no hay numero."""
    if valor is None:
        return f"{NO_DISPONIBLE} (n={n})"
    return f"{valor}{sufijo}"


# --- Regla 3: comprobacion de cordura, dentro del script que publica ------
#
# La Entrega 2 publico percentiles invertidos porque nada los miraba. Esto se
# ejecuta en el mismo script que genera las tablas, no en una revision aparte:
# una revision aparte es exactamente lo que no ocurrio.


def verificar_monotonia(etiqueta, valores_por_p):
    """Lista de incoherencias en un juego de percentiles. Vacia si esta bien."""
    errores = []
    ps = sorted(p for p, v in valores_por_p.items() if v is not None)
    for anterior, siguiente in zip(ps, ps[1:]):
        if valores_por_p[siguiente] < valores_por_p[anterior]:
            errores.append(
                f"{etiqueta}: p{siguiente}={valores_por_p[siguiente]} < "
                f"p{anterior}={valores_por_p[anterior]}"
            )
    return errores


def exigir_monotonia(bloques):
    """Aborta si algun juego de percentiles esta invertido.

    `bloques` es un iterable de (etiqueta, {p: valor}). Se comprueban todos
    antes de salir, para que un solo paso no esconda a los demas.
    """
    errores = [e for etiqueta, vs in bloques for e in verificar_monotonia(etiqueta, vs)]
    if errores:
        print("\nERROR: percentiles no monotonos. Esto no se publica.", file=sys.stderr)
        for e in errores:
            print(f"  {e}", file=sys.stderr)
        print(
            "\n       Un p95 por debajo del p50 es imposible con el estimador de\n"
            "       capacity_criterios.py, asi que la causa esta en los datos:\n"
            "       muestras de ventanas distintas mezcladas, o un conteo que\n"
            "       cambia entre un percentil y otro. Revisa antes de publicar;\n"
            "       la Entrega 2 publico esto y costo el criterio.",
            file=sys.stderr,
        )
        raise SystemExit(3)
    return True


# --- Regla 4: sin destino registrado no se publica ------------------------
#
# El defecto mas caro de la Entrega 2 no fue un calculo: fue publicar como
# "nube" una corrida cuyo destino era `http://api:8080`. Nada en la salida cruda
# decia contra que se habia medido, asi que nada lo contradecia.
#
# Ahora cada corrida escribe `destino.json` junto a sus datos crudos, y el
# script que agrega se niega a producir tablas si no esta.

ARCHIVO_DESTINO = "destino.json"

# Hosts que son el entorno local de desarrollo, no la nube. `api` y `minio` son
# nombres de servicio de Docker Compose: si aparecen como destino, se midio
# dentro de la red de Compose.
HOSTS_LOCALES = (
    "localhost",
    "127.0.0.1",
    "host.docker.internal",
    "api",
    "web",
    "minio",
    "postgres",
    "redis",
)


def clasificar_destino(url):
    """'local' o 'remoto' a partir de la URL. No se puede declarar a mano."""
    resto = url.split("://", 1)[-1]
    host = resto.split("/", 1)[0].split(":", 1)[0].lower()
    return "local" if host in HOSTS_LOCALES else "remoto"


def leer_destino(directorio, exigir_remoto=False):
    """Lee destino.json de una corrida. Aborta si falta o si es local cuando no toca."""
    ruta = os.path.join(directorio, ARCHIVO_DESTINO)
    if not os.path.exists(ruta):
        print(
            f"ERROR: falta {ruta}.\n"
            "       Una medicion sin destino registrado no se publica (A6). El\n"
            "       guion de la corrida debe escribirlo; si la corrida es vieja,\n"
            "       no se puede saber contra que se midio y hay que repetirla.",
            file=sys.stderr,
        )
        raise SystemExit(4)

    with open(ruta, encoding="utf-8") as f:
        destino = json.load(f)

    clase = clasificar_destino(destino.get("base_url", ""))
    destino["clase"] = clase
    if exigir_remoto and clase == "local":
        print(
            f"ERROR: la corrida de {directorio} apunta a {destino.get('base_url')},\n"
            "       que es el entorno local. No se publica como medicion de la nube.\n"
            "       Es exactamente el defecto que invalido el escenario 2 de la\n"
            "       Entrega 2.",
            file=sys.stderr,
        )
        raise SystemExit(5)
    return destino


def describir_destino(destino):
    """Linea de cabecera para la salida cruda y para las tablas."""
    return (
        f"Destino      : {destino.get('base_url', '?')}  [{destino.get('clase', '?')}]\n"
        f"Entorno      : {destino.get('entorno', '?')}\n"
        f"Codigo       : {destino.get('commit', '?')}\n"
        f"Generador    : {destino.get('generador', '?')}\n"
        f"Ventana UTC  : {destino.get('inicio_utc', '?')} -> {destino.get('fin_utc', 'en curso')}"
    )


# --- Regla 5: ninguna metrica derivada de una constante -------------------
#
# La Entrega 2 publico el tiempo al primer cuadro como
# `manifiesto + variante + segmento + 45 ms`, donde los 45 ms eran una
# constante escrita en el codigo y presentada como "inicializacion del
# decodificador". Sumar una constante a una latencia de red no mide
# decodificacion: devuelve la misma latencia desplazada.
#
# La regla no se puede comprobar sola --no hay forma de detectar una constante
# en una cifra ya publicada--, asi que se cumple de la unica manera posible:
# una metrica de reproduccion solo se publica si el que la produjo fue un
# reproductor. El detalle esta en el documento de criterios, seccion 6.

METRICAS_QUE_EXIGEN_REPRODUCTOR = (
    "tiempo al primer cuadro",
    "ttff",
    "interrupciones",
    "stalls",
    "rebuffering",
)


# --- Autoprueba -----------------------------------------------------------


def _estimador_entrega2_go(valores, p):
    """El estimador que produjo las tablas invertidas, para reproducirlo.

    Copia fiel de computeStageMetrics en scripts/capacity_escenario2/main.go
    antes de este issue: dos formulas distintas, una por percentil.
    """
    v = sorted(valores)
    if p == 50:
        return v[len(v) // 2]
    return v[int((len(v) - 1) * 0.95)]


def autoprueba():
    # Sin esto, la salida de la puerta de monotonia --que escribe en el canal de
    # errores, sin bufer-- aparece antes que las lineas que la explican cuando
    # todo se redirige a un archivo de evidencia.
    try:
        sys.stdout.reconfigure(line_buffering=True)
    except AttributeError:
        pass

    fallos = 0

    # 1. El estimador viejo invierte los percentiles con pocas muestras, que es
    #    justo el regimen del escenario 2 (1, 2, 4, 8 y 12 videos por nivel).
    muestras = [1000, 5000]  # ms: una subida rapida y una lenta
    viejo_p50 = _estimador_entrega2_go(muestras, 50)
    viejo_p95 = _estimador_entrega2_go(muestras, 95)
    if viejo_p95 >= viejo_p50:
        print("FALLO: la autoprueba ya no reproduce el defecto de la Entrega 2")
        fallos += 1
    else:
        print(f"ok  reproducido: con n=2 el estimador viejo da p50={viejo_p50} y p95={viejo_p95}")
        print("    (el p95 era el valor minimo de la muestra)")

    # 2. El estimador nuevo es monotono. Se comprueba sobre datos al azar, no
    #    sobre un caso elegido: lo que se quiere garantizar es que no exista
    #    ningun tamano de muestra en el que se cruce.
    azar = random.Random(4426)
    for n in range(1, 400):
        v = [azar.randint(0, 10000) for _ in range(n)]
        ps = {p: percentil(v, p) for p in (50, 95, 99)}
        if verificar_monotonia(f"n={n}", ps):
            print(f"FALLO: el estimador nuevo se cruzo con n={n}")
            fallos += 1
            break
    else:
        print("ok  el estimador nuevo es monotono para n = 1..399")

    # 3. La regla de observaciones minimas tapa los p95 que no existen.
    pocos = percentiles([1, 2, 3, 4, 5, 6, 7, 8])
    if pocos[95] is not None:
        print("FALLO: se publico un p95 con 8 observaciones")
        fallos += 1
    else:
        print(f"ok  con 8 observaciones el p95 sale '{formatear(pocos[95], 8)}', no un numero")
    if pocos[50] is None:
        print("FALLO: se oculto un p50 que si existe con 8 observaciones")
        fallos += 1

    muchos = percentiles(list(range(1, 121)))
    if any(v is None for v in muchos.values()):
        print("FALLO: con 120 observaciones deberian existir los tres percentiles")
        fallos += 1
    else:
        print("ok  con 120 observaciones los tres percentiles se publican")

    # 4. La puerta tiene dientes: con los percentiles de la Entrega 2, aborta.
    #    Sin esto el estimador nuevo haria la comprobacion decorativa --nunca
    #    se cruza-- y nadie sabria que dejo de protegerse de los datos.
    invertido = {50: viejo_p50, 95: viejo_p95}
    try:
        exigir_monotonia([("tabla de la Entrega 2", invertido)])
        print("FALLO: la puerta de monotonia acepto un p95 por debajo del p50")
        fallos += 1
    except SystemExit as e:
        if e.code == 3:
            print("ok  la puerta de monotonia aborta con los percentiles de la Entrega 2")
        else:
            print(f"FALLO: la puerta salio con codigo {e.code}, se esperaba 3")
            fallos += 1

    # 5. La clasificacion del destino no se puede declarar a mano.
    casos = [
        ("http://api:8080", "local"),
        ("http://host.docker.internal:8080", "local"),
        ("https://34.24.52.111.sslip.io", "remoto"),
        ("https://mooc.ejemplo.test/api", "remoto"),
    ]
    mal = [
        (url, clasificar_destino(url), esperado)
        for url, esperado in casos
        if clasificar_destino(url) != esperado
    ]
    for url, obtenido, esperado in mal:
        print(f"FALLO: {url} se clasifico como {obtenido}, se esperaba {esperado}")
    fallos += len(mal)
    if not mal:
        print("ok  el destino de la Entrega 2 (http://api:8080) se clasifica como local")

    print()
    if fallos:
        print(f"{fallos} comprobacion(es) fallaron.")
        return 1
    print("Todas las reglas se cumplen.")
    return 0


def main():
    if "--autoprueba" in sys.argv:
        return autoprueba()
    if "--destino" in sys.argv:
        i = sys.argv.index("--destino")
        print(describir_destino(leer_destino(sys.argv[i + 1])))
        return 0
    print(__doc__)
    return 0


if __name__ == "__main__":
    sys.exit(main())
