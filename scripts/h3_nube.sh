#!/usr/bin/env bash
# Ejecucion del escenario 1 contra la nube (issue #133, H3): la escalera completa
# de niveles, sin intervencion, con el plan de capacity-planning/escenario1.md.
#
# Por cada nivel: reinicia las cuentas de carga, corre el recorrido, verifica el
# estado directo en la base (3 envios por estudiante, sin duplicados) y guarda
# todo en docs/entrega2/evidencias/H3/resultados/<corrida>/. Si un nivel activa un
# criterio de PARADA, no sigue subiendo.
#
#   bash scripts/h3_nube.sh escalera
#       linea base (1) + niveles 10/25/50/100/200 con la pausa del plan (4-6 s), y,
#       si ninguno satura, serie B (100 y 200 con pausas de 1-2 s) y la rafaga de login.
#   bash scripts/h3_nube.sh repetir <usuarios> <think_ms> <rango_ms> <veces> [etiqueta]
#       repite un nivel (repeticion cerca del limite).
#
# Antes: gcloud con sesion, Docker abierto y el equipo enchufado y sin suspender
# (la escalera dura ~1 hora). La contrasena de las cuentas de carga se pide una vez.
# MODO=local corre lo mismo contra docker compose (para ensayar el script).

set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
ROOT="$(pwd)"

MODO="${MODO:-nube}"
if [ "$MODO" = "nube" ]; then export BASE_URL="${BASE_URL:-https://34.24.52.111.sslip.io}"
else export BASE_URL="${BASE_URL:-http://host.docker.internal:8080}"; fi
export RESULTS_ROOT="${RESULTS_ROOT:-${ROOT}/docs/entrega2/evidencias/H3/resultados}"
mkdir -p "${RESULTS_ROOT}"

NIVELES_A="${NIVELES_A:-1 10 25 50 100 200}"
NIVELES_B="${NIVELES_B:-100 200}"
PAUSA_ENTRE_CORRIDAS="${PAUSA_ENTRE_CORRIDAS:-120}"
THINK_A="${THINK_A:-4000}";  RANGO_A="${RANGO_A:-2000}"
THINK_B="${THINK_B:-1000}";  RANGO_B="${RANGO_B:-1000}"
PY=python; command -v python >/dev/null 2>&1 || PY=python3

log() { echo "[$(date +%H:%M:%S)] $*"; }

# ---------- acceso a la base (reinicio y verificacion) ----------
sql() { # $1 = archivo .sql ; imprime la salida de psql
  if [ "$MODO" = "nube" ]; then
    bash scripts/cargar_cuentas_carga_nube.sh "$1" 2>&1
  else
    docker compose exec -T postgres psql -U moocuser -d moocdb < "$1" 2>&1
  fi
}

reiniciar_cuentas() { sql scripts/seeds/capacity_reset.sql > /dev/null; }

# Comprueba el estado tras una corrida de N usuarios; escribe verificacion_bd.txt y devuelve 0 si cuadra.
verificar_estado() { # $1 = directorio de la corrida, $2 = N
  sql scripts/seeds/capacity_verificar_estado.sql > "$1/verificacion_bd.txt"
  "$PY" - "$1/verificacion_bd.txt" "$2" <<'PYEOF'
import re, sys
texto = open(sys.argv[1], encoding="utf-8", errors="replace").read().replace(chr(13), "")
n = int(sys.argv[2])
fila = re.search(r"^\s*(\d+)\s*\|\s*(\d+)\s*\|\s*(\d+)\s*\|\s*(\d+)\s*$", texto, re.M)
sueltos = [int(x) for x in re.findall(r"^\s*(\d+)\s*$", texto, re.M)]
if not fila or len(sueltos) < 3:
    print("VERIFICACION ILEGIBLE"); sys.exit(2)
est, mn, mx, tot = map(int, fila.groups())
insc, insig, dup = sueltos[:3]   # (la nube agrega una cuarta fila con el total de cuentas)
ok = (est == n and mn == 3 and mx == 3 and tot == 3 * n and insc == n and dup == 0)
print(f"estado en la base: {est} estudiantes con envios (min {mn}, max {mx}, total {tot}), "
      f"{insc} inscripciones, {dup} duplicados -> {'OK' if ok else 'NO CUADRA (esperado 3 envios por estudiante y 0 duplicados)'}")
sys.exit(0 if ok else 1)
PYEOF
}

# ---------- tokens ----------
preparar_tokens() { # $1 = cuantas cuentas hacen falta
  if [ -z "${CAPACITY_PASSWORD:-}" ]; then
    read -rsp "Contrasena de prueba de las cuentas de carga (no se ve al escribir): " CAPACITY_PASSWORD; echo
    export CAPACITY_PASSWORD
  fi
  rm -f capacity-planning/datos/tokens.csv   # que no se cuelen tokens viejos (de otro entorno o vencidos)
  log "Iniciando sesion de $1 cuentas en segundo plano (a ~8/min; los niveles bajos empiezan antes)"
  LOGIN_PAUSE_SECONDS="${LOGIN_PAUSE_SECONDS:-7}" bash scripts/capacity_login_tokens.sh "$1" 1 > "${RESULTS_ROOT}/tokens.log" 2>&1 &
  TOKENS_PID=$!
}

tokens_o_reusar() { # $1 = N ; si ya hay N tokens vigentes (24 h) no vuelve a iniciar sesion
  if [ -f capacity-planning/datos/tokens.csv ] && [ $(( $(wc -l < capacity-planning/datos/tokens.csv) - 1 )) -ge "$1" ]; then
    log "Se reusan los tokens de capacity-planning/datos/tokens.csv (duran 24 h)"
  else
    preparar_tokens "$1"
  fi
}

esperar_tokens() { # $1 = N
  local tiene
  while :; do
    tiene=0; [ -f capacity-planning/datos/tokens.csv ] && tiene=$(( $(wc -l < capacity-planning/datos/tokens.csv) - 1 ))
    [ "$tiene" -ge "$1" ] && return 0
    kill -0 "${TOKENS_PID:-0}" 2>/dev/null || { log "El inicio de sesion termino con solo ${tiene} cuentas; no alcanza para $1."; return 1; }
    log "esperando tokens: ${tiene}/$1"; sleep 20
  done
}

# ---------- una corrida ----------
declare -a CORRIDAS=()
SATURADO=0; PARADA=0

corrida() { # $1=usuarios $2=think $3=rango $4=rampa $5=etiqueta
  local n="$1" think="$2" rango="$3" rampa="$4" etiqueta="$5"
  esperar_tokens "$n" || { PARADA=1; return 1; }
  log ">>> ${etiqueta}: reiniciando cuentas"; reiniciar_cuentas
  log ">>> ${etiqueta}: corriendo ${n} usuarios (pausa ${think}+${rango} ms, rampa ${rampa} s)"
  RAMP_SECONDS="$rampa" THINK_MS="$think" THINK_RANGE_MS="$rango" WARMUP_SKIP_SECONDS=$((rampa + ${MARGEN_CALENTAMIENTO:-15})) \
    bash scripts/run_escenario1.sh "$n" "$etiqueta" > "${RESULTS_ROOT}/ultima_corrida.log" 2>&1
  local dir; dir="$(ls -d "${RESULTS_ROOT}/${etiqueta}_"* 2>/dev/null | tail -1)"
  [ -f "${dir}/resumen.json" ] || { log "ERROR: la corrida no dejo resumen (ver ${RESULTS_ROOT}/ultima_corrida.log)"; PARADA=1; return 1; }
  log ">>> ${etiqueta}: verificando estado en la base"
  local estado; estado="$(verificar_estado "$dir" "$n")"; local rc=$?
  echo "$estado" | tee "${dir}/estado.txt"
  echo "${n},${think},${rango},${rampa},${dir##*/}" >> "${RESULTS_ROOT}/corridas.csv"
  CORRIDAS+=("${dir}")
  # criterios del plan: saturacion (8.2) y parada (8.3)
  local evaluacion; evaluacion="$("$PY" - "${dir}/resumen.json" <<'PYEOF'
import json, sys
r = json.load(open(sys.argv[1]))
saturado = r["fallos_reales_pct"] > 1 or r["ms"]["p95"] > 1000 or r["fallos_validacion"] > 0
parada = r["fallos_reales_pct"] > 5 or r["ms"]["p95"] > 5000
print(f"{int(saturado)} {int(parada)} p95={r['ms']['p95']}ms reales={r['fallos_reales_pct']}% validacion={r['fallos_validacion']} rps={r['rps']}")
PYEOF
)"
  local sat par info; read -r sat par info <<< "${evaluacion}"
  log ">>> ${etiqueta}: ${info}"
  [ "${sat}" = "1" ] && [ "$n" -ge 10 ] && SATURADO=1
  [ "${par}" = "1" ] && PARADA=1
  [ "$rc" -ne 0 ] && { log "ATENCION: el estado en la base no cuadra en ${etiqueta}"; }
  log "pausa de ${PAUSA_ENTRE_CORRIDAS} s"; sleep "${PAUSA_ENTRE_CORRIDAS}"
  return 0
}

tabla_final() {
  "$PY" - "${RESULTS_ROOT}" <<'PYEOF'
import csv, json, os, sys
raiz = sys.argv[1]
filas = []
if os.path.exists(os.path.join(raiz, "corridas.csv")):
    for n, think, rango, rampa, nombre in csv.reader(open(os.path.join(raiz, "corridas.csv"))):
        p = os.path.join(raiz, nombre, "resumen.json")
        if not os.path.exists(p): continue
        r = json.load(open(p))
        est = open(os.path.join(raiz, nombre, "estado.txt"), encoding="utf-8").read().strip().split("->")[-1].strip() if os.path.exists(os.path.join(raiz, nombre, "estado.txt")) else "?"
        filas.append((nombre, n, f"{int(think)/1000:.1f}-{(int(think)+int(rango))/1000:.1f}s", r["muestras"], r["rps"], r["ms"]["p50"], r["ms"]["p95"], r["ms"]["p99"], r["fallos_reales"], r["timeouts"], r["fallos_validacion"], est[:2]))
print(f"{'corrida':44s} {'usr':>4s} {'pausa':>6s} {'n':>6s} {'rps':>7s} {'p50':>5s} {'p95':>6s} {'p99':>6s} {'real':>5s} {'t/o':>4s} {'val':>4s} {'bd':>3s}")
for f in filas:
    print(f"{f[0][:44]:44s} {f[1]:>4s} {f[2]:>6s} {f[3]:6d} {f[4]:7.1f} {f[5]:5d} {f[6]:6d} {f[7]:6d} {f[8]:5d} {f[9]:4d} {f[10]:4d} {f[11]:>3s}")
PYEOF
}

rampa_para() { [ -n "${RAMPA_FIJA:-}" ] && { echo "${RAMPA_FIJA}"; return; }; [ "$1" -ge 100 ] && echo 60 || echo 30; }

case "${1:-}" in
  escalera)
    maximo=1; for n in $NIVELES_A $NIVELES_B; do [ "$n" -gt "$maximo" ] && maximo="$n"; done
    if [ "$MODO" = "nube" ]; then preparar_tokens "$maximo"
    else log "MODO=local: se usan los tokens que ya existan en capacity-planning/datos/tokens.csv"; fi
    inicio="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    for n in $NIVELES_A; do
      corrida "$n" "$THINK_A" "$RANGO_A" "$(rampa_para "$n")" "A-nivel${n}" || break
      [ "$PARADA" = "1" ] && { log "PARADA: ${n} usuarios activo un criterio de parada; no se sube mas."; break; }
    done
    if [ "$PARADA" = "0" ] && [ "$SATURADO" = "0" ]; then
      log "Ningun nivel de la serie A satura: serie B (pausas de 1-2 s), mismo recorrido y mezcla"
      for n in $NIVELES_B; do
        MARGEN_CALENTAMIENTO=10 corrida "$n" "$THINK_B" "$RANGO_B" "${RAMPA_FIJA:-15}" "B-nivel${n}" || break
        [ "$PARADA" = "1" ] && { log "PARADA en la serie B."; break; }
      done
    fi
    fin="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "${inicio} ${fin}" > "${RESULTS_ROOT}/ventana_escalera.txt"
    if [ "$MODO" = "nube" ] && [ "$PARADA" = "0" ]; then
      log "Variante separada: rafaga de login (20 cuentas)"
      VARIANT=login bash scripts/run_escenario1.sh 20 "login-rafaga" > "${RESULTS_ROOT}/login_rafaga.log" 2>&1 || true
    fi
    [ -n "${TOKENS_PID:-}" ] && kill "${TOKENS_PID}" 2>/dev/null
    log "TERMINADO. Resumen:"; tabla_final | tee "${RESULTS_ROOT}/tabla_escalera.txt"
    ;;
  repetir)
    n="${2:?usuarios}"; think="${3:?think_ms}"; rango="${4:?rango_ms}"; veces="${5:?veces}"; etiqueta="${6:-repeticion-nivel${n}}"
    if [ "$MODO" = "nube" ]; then tokens_o_reusar "$n"; fi
    for i in $(seq 1 "$veces"); do
      corrida "$n" "$think" "$rango" "$(rampa_para "$n")" "${etiqueta}-r${i}" || break
      [ "$PARADA" = "1" ] && { log "PARADA en la repeticion ${i}."; break; }
    done
    [ -n "${TOKENS_PID:-}" ] && kill "${TOKENS_PID}" 2>/dev/null
    log "TERMINADO. Resumen:"; tabla_final | tee -a "${RESULTS_ROOT}/tabla_escalera.txt"
    ;;
  presion)
    # Sube el ritmo con la misma cantidad de usuarios acortando la pausa entre pasos (misma mezcla
    # lectura/escritura). Cada usuario solo puede hacer 3 sesiones (3 intentos de quiz), asi que a
    # menor pausa la corrida es mas corta: la rampa es de 10 s y se omiten 15 s. Se detiene en el primer nivel que satura: ahi esta el punto de degradacion.
    n="${2:?usuarios}"; shift 2; pares="${*:?pares think:rango, p. ej. 1000:1000 400:400}"
    if [ "$MODO" = "nube" ]; then tokens_o_reusar "$n"; fi
    for par in $pares; do
      think="${par%%:*}"; rango="${par##*:}"
      MARGEN_CALENTAMIENTO=5 corrida "$n" "$think" "$rango" "${RAMPA_FIJA:-10}" "B-u${n}-pausa${think}" || break
      [ "$PARADA" = "1" ] && { log "PARADA con pausa ${think}+${rango} ms."; break; }
      [ "$SATURADO" = "1" ] && { log "SATURACION con pausa ${think}+${rango} ms: se detiene aqui."; break; }
    done
    [ -n "${TOKENS_PID:-}" ] && kill "${TOKENS_PID}" 2>/dev/null
    log "TERMINADO. Resumen:"; tabla_final | tee -a "${RESULTS_ROOT}/tabla_escalera.txt"
    ;;
  *)
    echo "Uso: bash scripts/h3_nube.sh escalera | presion <usuarios> <think:rango ...> | repetir <usuarios> <think_ms> <rango_ms> <veces> [etiqueta]"; exit 1 ;;
esac
