#!/usr/bin/env python3
"""Linea de tiempo del escenario 3A: ciclo de escalamiento automatico de la API (G5, issue #225).

Cruza tres fuentes que se registran por separado y deja la linea de tiempo que pide el issue:

  carga        resultados.jtl y etapas.tsv de scripts/g5_carga_escalonada.sh (VM del generador):
               cuando subio la demanda DE VERDAD, no cuando estaba planificado que subiera
  observacion  observacion.tsv, instancias.tsv e hitos.tsv de scripts/g5_observar_escalamiento_api.sh:
               que decidio el autoescalador, cuando se creo, arranco, quedo sana y atendio cada instancia
  auditoria    el registro de auditoria de Compute Engine en la ventana (gcloud logging read): QUIEN
               cambio el tamano del grupo. "La ampliacion manual del grupo no aparece como evidencia"
               se demuestra aqui, no se afirma

y separa los dos tiempos que el issue pide por separado:

  reaccion       desde que la carga supera el umbral hasta que el servicio decide ampliar
  incorporacion  desde la decision hasta que la instancia nueva atiende peticiones del balanceador

Uso:
  python scripts/g5_linea_de_tiempo.py <dir_carga> <dir_observacion> [dir_salida] [--sin-auditoria]
  python scripts/g5_linea_de_tiempo.py --autoprueba

  <dir_carga>        la carpeta escenario3a_<fecha> que se trajo de la VM del generador
  <dir_observacion>  la carpeta que se le paso al observador
  [dir_salida]       por omision, la carpeta padre de <dir_observacion> (docs/entrega3/evidencias/G5)

Escribe linea_de_tiempo.md, tasa_10s.csv y auditoria.tsv. Sale con 1 si alguna comprobacion falla: un
comprobador que solo imprime no comprueba (nota 36).

Variables: MOOC_PROJECT (plataforma-mooc-entrega2), UMBRAL_PET_S (80: target 0,8 x max_rate 100, C2/C3),
ERRORES_MAX_PCT (1.0), TASA_MIN_PCT (90: la etapa alta tiene que alcanzar este % de la tasa pedida).

La auditoria usa el gcloud de quien lo ejecuta, de solo lectura. Los correos de personas NO se escriben:
salen como "persona". Los de cuentas de servicio si, porque no identifican a nadie.
"""
import csv
import gzip
import json
import os
import shutil
import subprocess
import sys
from collections import defaultdict
from datetime import datetime, timedelta, timezone

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import capacity_criterios as cc  # noqa: E402  (un solo estimador de percentiles: A6)

PROYECTO = os.environ.get("MOOC_PROJECT", "plataforma-mooc-entrega2")
UMBRAL = float(os.environ.get("UMBRAL_PET_S", "80"))
ERRORES_MAX_PCT = float(os.environ.get("ERRORES_MAX_PCT", "1.0"))
TASA_MIN_PCT = float(os.environ.get("TASA_MIN_PCT", "90"))
CUBETA_S = 10

# Metodos que cambian el tamano del grupo o su politica. Si los llama una PERSONA dentro de la ventana,
# el ciclo no es autoescalado y el issue no se puede cerrar con esta corrida.
METODOS_MANUALES = (
    "instanceGroupManagers.resize",
    "instanceGroupManagers.createInstances",
    "instanceGroupManagers.deleteInstances",
    "instanceGroupManagers.abandonInstances",
    "autoscalers.update",
    "autoscalers.patch",
    "autoscalers.insert",
    "autoscalers.delete",
)


# ------------------------------------------------------------------------------------------- tiempo
def parse_utc(texto):
    """ISO 8601 con Z, con offset o con fraccion -> datetime UTC. None si esta vacio o no es fecha."""
    if not texto or not texto[0].isdigit():
        return None
    t = texto.strip().replace("Z", "+00:00")
    try:
        d = datetime.fromisoformat(t)
    except ValueError:
        # Python < 3.11 no acepta fracciones de longitud arbitraria
        base, _, resto = t.partition(".")
        off = ""
        for signo in ("+", "-"):
            if signo in resto:
                off = signo + resto.split(signo, 1)[1]
        d = datetime.fromisoformat(base + off)
    if d.tzinfo is None:
        d = d.replace(tzinfo=timezone.utc)
    return d.astimezone(timezone.utc)


def hhmmss(d):
    return d.strftime("%H:%M:%S") if d else "—"


def seg(a, b):
    """b - a en segundos, o None."""
    if a is None or b is None:
        return None
    return round((b - a).total_seconds())


def fmt_s(s):
    if s is None:
        return "n/d"
    m, r = divmod(abs(int(s)), 60)
    signo = "-" if s < 0 else ""
    return f"{signo}{s} s ({signo}{m} min {r:02d} s)" if m else f"{s} s"


# ------------------------------------------------------------------------------------------- lectura
def leer_tsv(ruta):
    with open(ruta, encoding="utf-8", newline="") as f:
        return list(csv.DictReader(f, delimiter="\t"))


def leer_jtl(ruta):
    """Filas del .jtl: (inicio datetime, elapsed ms, etiqueta, codigo, exito)."""
    filas = []
    # El .jtl original se versiona comprimido (resultados.jtl.gz) cuando pesa mucho; se lee igual.
    if not os.path.exists(ruta) and os.path.exists(ruta + ".gz"):
        ruta += ".gz"
    abrir = gzip.open if ruta.endswith(".gz") else open
    with abrir(ruta, "rt", encoding="utf-8", newline="") as f:
        for r in csv.DictReader(f):
            try:
                ts = datetime.fromtimestamp(int(r["timeStamp"]) / 1000, tz=timezone.utc)
                filas.append((ts, int(r["elapsed"]), r.get("label", ""), r.get("responseCode", ""),
                              r.get("success", "").lower() == "true"))
            except (KeyError, ValueError):
                continue
    filas.sort(key=lambda x: x[0])
    return filas


def es_error_real(codigo, exito):
    """5xx, timeouts y errores de conexion. Un 4xx en lecturas publicas tambien seria raro, pero no
    es lo que mide la integridad del ciclo: se cuenta aparte."""
    if exito:
        return False
    return not (codigo.isdigit() and 400 <= int(codigo) < 500)


# ------------------------------------------------------------------------------------------- carga
def por_etapa(jtl, etapas):
    tabla = []
    for e in etapas:
        ini, fin = parse_utc(e["inicio_utc"]), parse_utc(e["fin_utc"])
        dentro = [x for x in jtl if ini <= x[0] < fin]
        dur = max(1.0, (fin - ini).total_seconds())
        lat = [x[1] for x in dentro if x[4]]
        ps = cc.percentiles(lat)
        reales = sum(1 for x in dentro if es_error_real(x[3], x[4]))
        cuatro = sum(1 for x in dentro if not x[4] and not es_error_real(x[3], x[4]))
        tabla.append({
            "etapa": e["etapa"], "nombre": e["nombre"], "pedida": float(e["pet_s"]),
            "lograda": len(dentro) / dur, "n": len(dentro), "ini": ini, "fin": fin,
            "p50": ps[50], "p95": ps[95], "p99": ps[99], "n_lat": len(lat),
            "err_reales": reales, "err_4xx": cuatro,
        })
    return tabla


def cubetas(jtl):
    """Por cubeta de CUBETA_S: peticiones/s, errores reales y p95 si hay 20 observaciones."""
    if not jtl:
        return []
    t0 = jtl[0][0].replace(microsecond=0)
    t0 -= timedelta(seconds=t0.second % CUBETA_S)
    grupos = defaultdict(list)
    for x in jtl:
        k = int((x[0] - t0).total_seconds() // CUBETA_S)
        grupos[k].append(x)
    salida = []
    for k in range(0, max(grupos) + 1):
        xs = grupos.get(k, [])
        lat = [x[1] for x in xs if x[4]]
        salida.append({
            "inicio": t0 + timedelta(seconds=k * CUBETA_S),
            "pet_s": len(xs) / CUBETA_S,
            "errores_reales": sum(1 for x in xs if es_error_real(x[3], x[4])),
            "p95_ms": cc.percentiles(lat)[95],
        })
    return salida


def primer_cruce(cubs, desde):
    """Primera cubeta, desde `desde`, cuya tasa supera el umbral de UNA instancia."""
    for c in cubs:
        if c["inicio"] >= desde - timedelta(seconds=CUBETA_S) and c["pet_s"] > UMBRAL:
            return c["inicio"]
    return None


# ------------------------------------------------------------------------------------------- auditoria
def anonimizar(principal):
    if not principal:
        return "desconocido"
    if principal.endswith("@cloudservices.gserviceaccount.com"):
        return "agente de servicio de Google APIs (grupo administrado / autoescalador)"
    if principal.endswith(".gserviceaccount.com"):
        return principal
    return "persona"


def leer_auditoria(desde, hasta):
    filtro = (
        f'logName="projects/{PROYECTO}/logs/cloudaudit.googleapis.com%2Factivity" '
        f'AND timestamp>="{desde.strftime("%Y-%m-%dT%H:%M:%SZ")}" '
        f'AND timestamp<="{hasta.strftime("%Y-%m-%dT%H:%M:%SZ")}" '
        '(protoPayload.methodName:"instanceGroupManagers" OR protoPayload.methodName:"autoscalers" '
        'OR protoPayload.methodName:"compute.instances.insert" OR protoPayload.methodName:"compute.instances.delete")'
    )
    # Lista de argumentos y no una linea de shell: el filtro lleva comillas, y entre cmd.exe (Git Bash en
    # Windows) y bash el escape de comillas no es el mismo. shutil.which encuentra gcloud.cmd en Windows.
    gcloud = shutil.which("gcloud") or "gcloud"
    cmd = [gcloud, "logging", "read", filtro, f"--project={PROYECTO}", "--format=json", "--order=asc", "--limit=500"]
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode != 0:
        raise RuntimeError((r.stderr or r.stdout).strip()[:500])
    entradas = json.loads(r.stdout or "[]")
    filas = []
    for e in entradas:
        pp = e.get("protoPayload", {})
        metodo = pp.get("methodName", "")
        recurso = (pp.get("resourceName", "") or "").rsplit("/", 1)[-1]
        quien = anonimizar(pp.get("authenticationInfo", {}).get("principalEmail", ""))
        fase = e.get("operation", {})
        estado = "fin" if fase.get("last") else ("inicio" if fase.get("first") else "")
        filas.append((e.get("timestamp", ""), metodo, recurso, quien, estado))
    return filas


# ------------------------------------------------------------------------------------------- informe
def informe(dir_carga, dir_obs, dir_salida, con_auditoria=True):
    etapas = leer_tsv(os.path.join(dir_carga, "etapas.tsv"))
    jtl = leer_jtl(os.path.join(dir_carga, "resultados.jtl"))
    destino = json.load(open(os.path.join(dir_carga, "destino.json"), encoding="utf-8"))
    obs = leer_tsv(os.path.join(dir_obs, "observacion.tsv"))
    inst = leer_tsv(os.path.join(dir_obs, "instancias.tsv"))
    hitos = {h["hito"]: h for h in leer_tsv(os.path.join(dir_obs, "hitos.tsv"))}
    if not jtl:
        sys.exit("ERROR: resultados.jtl esta vacio o no tiene el formato CSV de JMeter")

    comprobaciones = []

    def comprobar(ok, texto):
        comprobaciones.append((bool(ok), texto))

    tabla = por_etapa(jtl, etapas)
    cubs = cubetas(jtl)
    alta = next(t for t in tabla if t["etapa"] == "e4")
    baja = next(t for t in tabla if t["etapa"] == "e5")

    # --- tiempos -----------------------------------------------------------------------------------
    t_carga = primer_cruce(cubs, next(t["ini"] for t in tabla if t["etapa"] == "e3"))
    t_dec_obs = parse_utc(hitos.get("DECISION_AMPLIAR", {}).get("utc"))
    nuevas = [i for i in inst if i["primer_running"] != "anterior a la corrida"]
    nuevas.sort(key=lambda i: parse_utc(i["creada"]) or datetime.max.replace(tzinfo=timezone.utc))
    n1 = nuevas[0] if nuevas else {}
    t_creada = parse_utc(n1.get("creada"))
    t_running = parse_utc(n1.get("primer_running"))
    t_sana = parse_utc(n1.get("primer_sana"))
    t_atiende = parse_utc(n1.get("primera_peticion_atendida"))
    # La decision es lo primero que se ve de dos fuentes: el sondeo (resolucion POLL_S) o la creacion de
    # la instancia, que GCE fecha al milisegundo y no puede ser anterior a la decision.
    candidatas = [t for t in (t_dec_obs, t_creada) if t]
    t_decision = min(candidatas) if candidatas else None

    t_red = parse_utc(hitos.get("DECISION_REDUCIR", {}).get("utc"))
    # El autoescalador puede retirar la nueva o la original: cuenta la primera que desaparezca.
    retiros = [parse_utc(i["desaparecio"]) for i in inst if parse_utc(i.get("desaparecio"))]
    t_retirada = min(retiros) if retiros else None
    t_ciclo = parse_utc(hitos.get("CICLO_COMPLETO", {}).get("utc"))

    reaccion = seg(t_carga, t_decision)
    incorporacion = seg(t_decision, t_atiende)
    poll = None
    if len(obs) > 2:
        poll = round((parse_utc(obs[-1]["utc"]) - parse_utc(obs[1]["utc"])).total_seconds() / max(1, len(obs) - 2))

    # --- comprobaciones --------------------------------------------------------------------------
    comprobar(alta["lograda"] >= alta["pedida"] * TASA_MIN_PCT / 100,
              f"la etapa alta alcanzo la tasa pedida: {alta['lograda']:.1f} de {alta['pedida']:.0f} pet/s "
              f"(minimo {TASA_MIN_PCT:.0f} %); si no, el generador o sus hilos limitaron la corrida")
    comprobar(t_carga is not None, f"la carga supero el umbral de {UMBRAL:.0f} pet/s con una instancia")
    comprobar(t_decision is not None and t_carga is not None and t_decision >= t_carga - timedelta(seconds=CUBETA_S),
              "la decision de ampliar es POSTERIOR al aumento de carga (si fuera anterior, la causa no es esta carga)")
    comprobar(bool(nuevas), "se creo al menos una instancia nueva")
    comprobar(t_atiende is not None, "la instancia nueva atendio peticiones entrando por el balanceador")
    comprobar(t_red is not None and t_red >= baja["ini"],
              "la reduccion empezo despues de bajar la carga")
    comprobar(t_ciclo is not None, "el grupo volvio a 1 instancia")

    total = len(jtl)
    reales = sum(1 for x in jtl if es_error_real(x[3], x[4]))
    pct_err = 100.0 * reales / total if total else 0.0
    comprobar(pct_err <= ERRORES_MAX_PCT,
              f"errores reales (5xx, timeouts, conexion) en todo el ciclo: {reales} de {total} = {pct_err:.3f} % "
              f"(maximo {ERRORES_MAX_PCT} %)")

    # Errores en las dos transiciones: alrededor de la incorporacion y alrededor del retiro.
    def errores_entre(a, b):
        if not a or not b:
            return None, 0
        xs = [x for x in jtl if a <= x[0] <= b]
        return sum(1 for x in xs if es_error_real(x[3], x[4])), len(xs)

    err_inc, n_inc = errores_entre(t_decision, (t_atiende + timedelta(minutes=2)) if t_atiende else None)
    err_ret, n_ret = errores_entre((t_red - timedelta(minutes=1)) if t_red else None,
                                   (t_retirada + timedelta(minutes=1)) if t_retirada else None)

    bloques = [(f"etapa {t['etapa']}", {50: t["p50"], 95: t["p95"], 99: t["p99"]}) for t in tabla]
    invertidos = [err for etq, ps in bloques for err in cc.verificar_monotonia(etq, ps)]
    comprobar(not invertidos, "percentiles monotonos p50 <= p95 <= p99 en todas las etapas" +
              (f": {invertidos}" if invertidos else ""))

    # --- auditoria -------------------------------------------------------------------------------
    auditoria = []
    error_auditoria = None
    if con_auditoria:
        desde = parse_utc(obs[0]["utc"]) - timedelta(minutes=5)
        hasta = parse_utc(obs[-1]["utc"]) + timedelta(minutes=5)
        try:
            auditoria = leer_auditoria(desde, hasta)
        except Exception as exc:  # noqa: BLE001 - se informa y la comprobacion falla
            error_auditoria = str(exc)
        manuales = [a for a in auditoria if a[3] == "persona" and any(m in a[1] for m in METODOS_MANUALES)]
        comprobar(error_auditoria is None,
                  "se pudo leer el registro de auditoria de la ventana" +
                  (f" ({error_auditoria})" if error_auditoria else ""))
        comprobar(error_auditoria is None and not manuales,
                  "ninguna PERSONA cambio el tamano ni la politica del grupo durante la ventana"
                  + (f": {len(manuales)} llamada(s) manual(es)" if manuales else ""))
        with open(os.path.join(dir_salida, "auditoria.tsv"), "w", encoding="utf-8", newline="") as f:
            w = csv.writer(f, delimiter="\t")
            w.writerow(["timestamp", "metodo", "recurso", "quien", "fase"])
            w.writerows(auditoria)

    # --- salidas -----------------------------------------------------------------------------------
    # tamano objetivo en cada cubeta: el del ultimo sondeo anterior
    obs_t = [(parse_utc(o["utc"]), o["tamano_objetivo"]) for o in obs]
    with open(os.path.join(dir_salida, "tasa_10s.csv"), "w", encoding="utf-8", newline="") as f:
        w = csv.writer(f)
        w.writerow(["inicio_utc", "peticiones_s", "errores_reales", "p95_ms", "instancias_objetivo"])
        for c in cubs:
            previos = [s for t, s in obs_t if t and t <= c["inicio"]]
            w.writerow([c["inicio"].strftime("%Y-%m-%dT%H:%M:%SZ"), f"{c['pet_s']:.1f}", c["errores_reales"],
                        "" if c["p95_ms"] is None else c["p95_ms"], previos[-1] if previos else ""])

    lineas = []
    add = lineas.append
    add("# G5 — Línea de tiempo del escenario 3A (generada)")
    add("")
    add(f"Generada por `scripts/g5_linea_de_tiempo.py` el {datetime.now(timezone.utc):%Y-%m-%d %H:%M} UTC. "
        "No editar a mano: se regenera con los datos crudos de esta carpeta.")
    add("")
    add(f"- Destino: `{destino.get('base_url')}` · entorno `{destino.get('entorno')}` · commit `{destino.get('commit')}`")
    add(f"- Generador: {destino.get('generador')} ({destino.get('nucleos_generador')} núcleos)")
    add(f"- Umbral de la política (C2): **{UMBRAL:.0f} pet/s por instancia** (target 0,8 × max_rate 100)")
    if poll:
        add(f"- Resolución del observador: un sondeo cada ~{poll} s; los hitos observados tienen ese margen")
    add("")
    add("## Los dos tiempos")
    add("")
    add("| Tiempo | Desde | Hasta | Duración |")
    add("| :--- | :--- | :--- | ---: |")
    add(f"| **Reacción** | la carga supera {UMBRAL:.0f} pet/s ({hhmmss(t_carga)}) | decisión de ampliar ({hhmmss(t_decision)}) | **{fmt_s(reaccion)}** |")
    add(f"| **Incorporación** | decisión ({hhmmss(t_decision)}) | primera petición atendida por la nueva ({hhmmss(t_atiende)}) | **{fmt_s(incorporacion)}** |")
    add("")
    add("La incorporación, desglosada:")
    add("")
    add("| Tramo | Duración | Qué lo explica |")
    add("| :--- | ---: | :--- |")
    add(f"| decisión → instancia creada | {fmt_s(seg(t_decision, t_creada))} | el grupo pide la VM |")
    add(f"| creada → `RUNNING` | {fmt_s(seg(t_creada, t_running))} | arranque de la VM (resolución del sondeo) |")
    add(f"| `RUNNING` → sana ante el balanceador | {fmt_s(seg(t_running, t_sana))} | arranque de la imagen y `/api/v1/ready` (B3: 62 s medidos) |")
    add(f"| sana → atiende | {fmt_s(seg(t_sana, t_atiende))} | el balanceador la incorpora al reparto |")
    add("")
    add("## Línea de tiempo")
    add("")
    add("| Hora UTC | Fuente | Qué |")
    add("| :--- | :--- | :--- |")
    eventos = []
    for t in tabla:
        eventos.append((t["ini"], "carga", f"empieza {t['etapa']} ({t['nombre']}): {t['pedida']:.0f} pet/s pedidas"))
    if t_carga:
        eventos.append((t_carga, "carga", f"primera cubeta de {CUBETA_S} s por encima de {UMBRAL:.0f} pet/s"))
    for nombre, h in hitos.items():
        eventos.append((parse_utc(h["utc"]), "observador", f"{nombre}: {h['detalle']}"))
    for i in nuevas:
        if parse_utc(i["creada"]):
            eventos.append((parse_utc(i["creada"]), "GCE", f"creationTimestamp de {i['nombre']} (id {i['id']})"))
    for a in auditoria:
        if any(k in a[1] for k in ("instanceGroupManagers", "autoscalers", "instances.insert", "instances.delete")) and a[4] != "fin":
            eventos.append((parse_utc(a[0]), "auditoría", f"`{a[1]}` sobre {a[2]} por {a[3]}"))
    for t, fuente, que in sorted((e for e in eventos if e[0]), key=lambda e: e[0]):
        add(f"| {t:%H:%M:%S} | {fuente} | {que} |")
    add("")
    add("## Carga por etapa (JMeter, desde la VM del generador)")
    add("")
    add("| Etapa | Pedida (pet/s) | Lograda (pet/s) | n | p50 ms | p95 ms | p99 ms | Errores reales | 4xx |")
    add("| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |")
    for t in tabla:
        add(f"| {t['etapa']} {t['nombre']} | {t['pedida']:.0f} | {t['lograda']:.1f} |".replace(".", ",") + f" {t['n']} | "
            f"{cc.formatear(t['p50'], t['n_lat'])} | {cc.formatear(t['p95'], t['n_lat'])} | "
            f"{cc.formatear(t['p99'], t['n_lat'])} | {t['err_reales']} | {t['err_4xx']} |")
    add("")
    add("Latencias de las peticiones exitosas, por rango más cercano (`capacity_criterios.py`, A6). "
        "Son latencias HTTP de lecturas; no se mezclan con trabajos asíncronos.")
    add("")
    add("## Integridad durante el ciclo")
    add("")
    add(f"- Errores reales en todo el ciclo: **{reales} de {total}** ({pct_err:.3f} %).")
    if err_inc is not None:
        add(f"- Durante la incorporación (decisión → atiende + 2 min): {err_inc} de {n_inc}.")
    if err_ret is not None:
        add(f"- Durante el retiro (decisión de reducir − 1 min → retirada + 1 min): {err_ret} de {n_ret}. "
            "El backend drena 60 s (`connection_draining_timeout_sec`, C3).")
    add("- La integridad de **escrituras** entre réplicas (envíos, inscripciones, progreso) la cubre la corrida "
        "del arnés de consistencia lanzada con el grupo ya en 2 instancias: ver el README de G5.")
    add("")
    if con_auditoria:
        add("## Quién cambió el grupo (registro de auditoría)")
        add("")
        if error_auditoria:
            add(f"No se pudo leer: `{error_auditoria}`")
        else:
            add("| Hora UTC | Método | Recurso | Quién |")
            add("| :--- | :--- | :--- | :--- |")
            for a in auditoria:
                if a[4] != "fin":
                    add(f"| {(parse_utc(a[0]) or datetime.min):%H:%M:%S} | `{a[1]}` | {a[2]} | {a[3]} |")
            if not auditoria:
                add("| — | sin entradas en la ventana | | |")
        add("")
        add("Los correos de personas no se publican: aparecen como «persona».")
        add("")
    add("## Comprobaciones")
    add("")
    for ok, texto in comprobaciones:
        add(f"- {'✅' if ok else '❌'} {texto}")
    add("")
    add("Este archivo no contiene credenciales, llaves ni secretos.")
    with open(os.path.join(dir_salida, "linea_de_tiempo.md"), "w", encoding="utf-8") as f:
        f.write("\n".join(lineas) + "\n")

    for ok, texto in comprobaciones:
        print(("OK     " if ok else "FALLA  ") + texto)
    print(f"reaccion: {fmt_s(reaccion)}   incorporacion: {fmt_s(incorporacion)}")
    print(f"==> {os.path.join(dir_salida, 'linea_de_tiempo.md')}")
    return all(ok for ok, _ in comprobaciones)


# ------------------------------------------------------------------------------------------- autoprueba
def autoprueba():
    import tempfile

    d = tempfile.mkdtemp()
    carga, obs = os.path.join(d, "carga"), os.path.join(d, "obs")
    os.makedirs(carga)
    os.makedirs(obs)
    t0 = datetime(2026, 11, 2, 15, 0, 0, tzinfo=timezone.utc)
    plan = [("e1", "base", 10, 300), ("e2", "subida-40", 40, 120), ("e3", "subida-70", 70, 120),
            ("e4", "alta-sostenida", 130, 900), ("e5", "bajada", 10, 1200)]
    with open(os.path.join(carga, "etapas.tsv"), "w") as f:
        f.write("etapa\tnombre\tpet_s\tinicio_utc\tfin_utc\n")
        t = t0
        for e, n, r, s in plan:
            f.write(f"{e}\t{n}\t{r}\t{t:%Y-%m-%dT%H:%M:%SZ}\t{t + timedelta(seconds=s):%Y-%m-%dT%H:%M:%SZ}\n")
            t += timedelta(seconds=s)
    with open(os.path.join(carga, "resultados.jtl"), "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["timeStamp", "elapsed", "label", "responseCode", "success"])
        t = t0
        for e, n, r, s in plan:
            paso = 1.0 / r
            k = 0
            while k * paso < s:
                ts = t + timedelta(seconds=k * paso)
                lat = 15 if r < 100 else 40
                ok, cod = True, "200"
                if e == "e5" and k == 5:
                    ok, cod = False, "502"  # un error real durante el retiro
                w.writerow([int(ts.timestamp() * 1000), lat + (k % 7), f"{e} GET readiness", cod, str(ok).lower()])
                k += 1
            t += timedelta(seconds=s)
    json.dump({"base_url": "https://x.sslip.io", "entorno": "prueba", "commit": "abc", "generador": "jmeter",
               "nucleos_generador": 4}, open(os.path.join(carga, "destino.json"), "w"))
    e4 = t0 + timedelta(seconds=540)
    e5 = e4 + timedelta(seconds=900)
    with open(os.path.join(obs, "observacion.tsv"), "w") as f:
        f.write("utc\tt_s\ttamano_objetivo\trecomendado\testado_autoescalador\tinstancias\treparto\n")
        for k in range(0, 40 * 60, 15):
            t = t0 + timedelta(seconds=k)
            tam = 2 if e4 + timedelta(seconds=75) <= t < e5 + timedelta(seconds=600) else 1
            f.write(f"{t:%Y-%m-%dT%H:%M:%SZ}\t{k}\t{tam}\t{tam}\tACTIVE\tx\ty\n")
    h = {"PARTIDA": t0, "DECISION_AMPLIAR": e4 + timedelta(seconds=75), "INSTANCIA_CREADA": e4 + timedelta(seconds=90),
         "INSTANCIA_RUNNING": e4 + timedelta(seconds=105), "INSTANCIA_SANA": e4 + timedelta(seconds=195),
         "INSTANCIA_ATIENDE": e4 + timedelta(seconds=210), "DECISION_REDUCIR": e5 + timedelta(seconds=600),
         "INSTANCIA_RETIRADA": e5 + timedelta(seconds=690), "CICLO_COMPLETO": e5 + timedelta(seconds=720)}
    with open(os.path.join(obs, "hitos.tsv"), "w") as f:
        f.write("utc\tt_s\thito\tdetalle\n")
        for k, v in h.items():
            f.write(f"{v:%Y-%m-%dT%H:%M:%SZ}\t0\t{k}\tprueba\n")
    with open(os.path.join(obs, "instancias.tsv"), "w") as f:
        f.write("nombre\tid\tx_instance_id\tcreada\tvista_por_primera_vez\tprimer_running\tprimer_sana\t"
                "primera_peticion_atendida\tdesaparecio\n")
        f.write("mooc-api-a\t1\tabc\t2026-11-02T07:00:00.123-08:00\t\tanterior a la corrida\tanterior a la corrida\t"
                "anterior a la corrida\t\n")
        c = (e4 + timedelta(seconds=82)).astimezone(timezone(timedelta(hours=-8)))
        f.write(f"mooc-api-b\t2\tdef\t{c.isoformat(timespec='milliseconds')}\t\t"
                f"{h['INSTANCIA_RUNNING']:%Y-%m-%dT%H:%M:%SZ}\t{h['INSTANCIA_SANA']:%Y-%m-%dT%H:%M:%SZ}\t"
                f"{h['INSTANCIA_ATIENDE']:%Y-%m-%dT%H:%M:%SZ}\t{h['INSTANCIA_RETIRADA']:%Y-%m-%dT%H:%M:%SZ}\n")

    fallos = []
    ok = informe(carga, obs, d, con_auditoria=False)
    texto = open(os.path.join(d, "linea_de_tiempo.md"), encoding="utf-8").read()
    if not ok:
        fallos.append("un ciclo correcto no deberia fallar")
    # la carga cruza 80 pet/s al empezar e4 (los 70 de e3 no); la decision por creacion (82 s) gana al sondeo (75 s)? no:
    # el sondeo es a los 75 s y la creacion a los 82 s -> decision = 75 s; incorporacion = 210 - 75 = 135 s
    if "**75 s (1 min 15 s)**" not in texto:
        fallos.append("reaccion esperada 75 s")
    if "**135 s (2 min 15 s)**" not in texto:
        fallos.append("incorporacion esperada 135 s")
    if "1 de " not in texto:
        fallos.append("deberia contar el 502 del retiro")
    # Si la etapa alta no alcanza la tasa, tiene que fallar.
    filas = open(os.path.join(carga, "resultados.jtl")).read().splitlines()
    e4_ini, e4_fin = int(e4.timestamp() * 1000), int(e5.timestamp() * 1000)
    recortado = [filas[0]] + [ln for i, ln in enumerate(filas[1:])
                              if not (e4_ini <= int(ln.split(",")[0]) < e4_fin and i % 3 == 0)]
    open(os.path.join(carga, "resultados.jtl"), "w").write("\n".join(recortado) + "\n")
    if informe(carga, obs, d, con_auditoria=False):
        fallos.append("con la etapa alta al 67 % de la tasa deberia fallar")
    for f in fallos:
        print("AUTOPRUEBA FALLA:", f)
    print("autoprueba:", "OK" if not fallos else f"{len(fallos)} fallo(s)")
    return not fallos


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    if "--autoprueba" in sys.argv:
        sys.exit(0 if autoprueba() else 1)
    if len(args) < 2:
        sys.exit(__doc__)
    dir_carga, dir_obs = args[0], args[1]
    dir_salida = args[2] if len(args) > 2 else os.path.dirname(os.path.abspath(dir_obs))
    os.makedirs(dir_salida, exist_ok=True)
    ok = informe(dir_carga, dir_obs, dir_salida, con_auditoria="--sin-auditoria" not in sys.argv)
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
