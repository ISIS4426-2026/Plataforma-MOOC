#!/usr/bin/env bash
# Observador del ciclo de escalamiento automatico de la API (G5, issue #225): escenario 3A.
#
# SE CORRE EN EL PORTATIL, en su propia terminal, ANTES de arrancar la carga. No genera carga: la genera
# scripts/g5_carga_escalonada.sh en la VM del generador. Separarlos deja que este mida sin competir por
# la red, y que la carga se pueda repetir sin perder la observacion.
#
# QUE HACE
#
#   1. COMPROBAR que el punto de partida es el que pide el issue: autoescalador activo, una sola
#      instancia operativa y nadie mas tocando el grupo. Si no, para: medir un ciclo que empieza en 2
#      instancias no demuestra nada (y fijar el minimo en 2 a mano tampoco: "la ampliacion manual del
#      grupo no aparece como evidencia").
#
#   2. OBSERVAR cada POLL_S segundos, hasta que el grupo suba y vuelva a bajar solo:
#        - tamano objetivo del grupo y lo que recomienda el autoescalador, con su estado
#        - cada instancia: nombre, ID, estado, accion en curso y salud ante el balanceador
#        - QUIEN atiende: MUESTRAS peticiones a /api/v1/ready, agrupadas por la cabecera X-Instance-ID
#
#      Cada instancia se identifica por su ID numerico, no por su nombre: un grupo reutiliza nombres
#      (nota 36). El X-Instance-ID es sha256(ID)[:12] (internal/http/middleware/instance.go con el
#      INSTANCE_ID que pone scripts/prepare_web_env.sh), asi que se calcula aqui sin entrar por SSH, y
#      con eso se demuestra que la instancia NUEVA atiende peticiones que entran por el balanceador.
#
#   3. ANUNCIAR los hitos en cuanto ocurren, con la captura que toca en ese momento (las capturas
#      bloquean el cierre de G5 y no se pueden recrear despues).
#
#   4. AFIRMAR al final: sale con 0 solo si hubo ampliacion por el autoescalador, la instancia nueva
#      atendio trafico, y hubo reduccion posterior. Un comprobador que solo imprime no comprueba (nota 36).
#
# USO
#
#   bash ./scripts/g5_observar_escalamiento_api.sh docs/entrega3/evidencias/G5/observacion_<fecha>
#
# Variables: PROYECTO, ZONA, GRUPO, URL (por omision, terraform output -raw lb_url),
#            POLL_S (15), MUESTRAS (10), PLAZO_MIN (80), RUTA (/api/v1/ready)
#
# SALIDA en el directorio dado:
#   observacion.tsv   una fila por sondeo (lo que se grafica y lo que lee g5_linea_de_tiempo.py)
#   instancias.tsv    nombre, ID, X-Instance-ID, creacion, y cuando se vio cada estado por primera vez
#   hitos.tsv         los hitos fechados
#   cronologia.txt    la misma informacion, legible, tal como salio por pantalla
#
# Sin jq a proposito: se corre desde Windows con Git Bash, como c6_prueba_autoescalado.sh.
set -uo pipefail

PROYECTO="${PROYECTO:-plataforma-mooc-entrega2}"
ZONA="${ZONA:-us-east1-b}"
GRUPO="${GRUPO:-mooc-api-group}"
POLL_S="${POLL_S:-15}"
MUESTRAS="${MUESTRAS:-10}"
PLAZO_MIN="${PLAZO_MIN:-80}"
RUTA="${RUTA:-/api/v1/ready}"

SALIDA_DIR="${1:?Uso: g5_observar_escalamiento_api.sh <directorio-de-salida>}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [ -z "${URL:-}" ]; then
    URL="$(cd "${ROOT}/infra/terraform" && terraform output -raw lb_url 2>/dev/null | tr -d '\r')" || URL=""
fi
[ -n "${URL}" ] || { echo "ERROR: no hay URL del balanceador. Pasa URL=https://... o enciende con -var lb_enabled=true" >&2; exit 2; }

mkdir -p "${SALIDA_DIR}"
CRONO="${SALIDA_DIR}/cronologia.txt"
OBS="${SALIDA_DIR}/observacion.tsv"
INST="${SALIDA_DIR}/instancias.tsv"
HITOS="${SALIDA_DIR}/hitos.tsv"
: >"${CRONO}"
printf 'utc\tt_s\ttamano_objetivo\trecomendado\testado_autoescalador\tinstancias\treparto\n' >"${OBS}"
printf 'utc\tt_s\thito\tdetalle\n' >"${HITOS}"

T0=$(date +%s)
ahora_utc() { date -u +%Y-%m-%dT%H:%M:%SZ; }
emitir() { echo "$*" | tee -a "${CRONO}"; }
marca() { emitir "$(printf '[%s] T+%-6s %s' "$(ahora_utc)" "$(($(date +%s) - T0))s" "$*")"; }
hito() {
    printf '%s\t%s\t%s\t%s\n' "$(ahora_utc)" "$(($(date +%s) - T0))" "$1" "$2" >>"${HITOS}"
    marca ">>> $1: $2 <<<"
}

if command -v sha256sum >/dev/null 2>&1; then
    resumen() { printf '%s' "$1" | sha256sum | cut -c1-12; }
else
    resumen() { printf '%s' "$1" | shasum -a 256 | cut -c1-12; }
fi

# --- lecturas de GCE -----------------------------------------------------------------------------------
# El estado del autoescalador se lee POR EL GRUPO: `gcloud compute autoscalers` no existe en el SDK que
# usa el equipo (nota 43).
#
# Separador `|` y no `value()`: `value()` separa con tabuladores, y `read` junta tabuladores seguidos
# porque son espacio en blanco. Con un campo vacio --el recomendado antes del primer sondeo, la salud de
# una instancia que arranca-- todos los de la derecha se correrian un puesto sin que nada fallara.
estado_grupo() {
    gcloud compute instance-groups managed describe "${GRUPO}" \
        --zone="${ZONA}" --project="${PROYECTO}" \
        --format="csv[no-heading,separator='|'](targetSize,autoscaler.recommendedSize,autoscaler.status,autoscaler.autoscalingPolicy.minNumReplicas)" \
        2>/dev/null | tr -d '\r' | head -1
}

detalle_autoescalador() {
    gcloud compute instance-groups managed describe "${GRUPO}" \
        --zone="${ZONA}" --project="${PROYECTO}" \
        --format="value(autoscaler.statusDetails[0].message)" 2>/dev/null | tr -d '\r'
}

# nombre, id, estado, accion, salud. El id del ManagedInstance es el de la instancia de GCE.
instancias_grupo() {
    gcloud compute instance-groups managed list-instances "${GRUPO}" \
        --zone="${ZONA}" --project="${PROYECTO}" \
        --format="csv[no-heading,separator='|'](name,id,instanceStatus,currentAction,instanceHealth[0].detailedHealthState)" \
        2>/dev/null | tr -d '\r'
}

creacion_de() {
    gcloud compute instances describe "$1" --zone="${ZONA}" --project="${PROYECTO}" \
        --format="value(id,creationTimestamp)" 2>/dev/null | tr -d '\r'
}

# --- quien atiende ---------------------------------------------------------------------------------------
# Cada peticion lleva un parametro distinto y Cache-Control: no-cache. Desde un portatil, algo entre el
# equipo y el balanceador llego a contestar lecturas en la calibracion de G1 (nota 49); una respuesta
# servida por un intermediario repetiria un X-Instance-ID viejo y fabricaria un reparto que no existe.
quien() {
    local id
    id="$(curl -sS -o /dev/null -D - --max-time 8 -H 'Cache-Control: no-cache' \
        "${URL}${RUTA}?g5=$(date +%s%N)${RANDOM}" 2>/dev/null |
        tr -d '\r' | awk 'tolower($1)=="x-instance-id:"{print $2}')"
    printf '%s' "${id:-sin-respuesta}"
}

reparto() {
    for _ in $(seq 1 "${MUESTRAS}"); do quien; echo; done | sort | uniq -c |
        awk '{printf "%s:%s ", $2, $1}' | sed 's/ $//'
}

# --- registro de instancias, por ID ----------------------------------------------------------------------
declare -A NOMBRE_DE ID_DE_HASH HASH_DE CREADA VISTA RUNNING SANA ATIENDE RETIRADA VISTA_ANUNCIADA
ORDEN=()
REG_ID=""

# Deja en REG_ID el ID de la instancia (el que trae el grupo o, si no, el de describe).
registrar() { # nombre id
    local nombre="$1" id="$2" c
    REG_ID=""
    if [ -z "${id}" ]; then
        c="$(creacion_de "${nombre}")"
        id="$(printf '%s' "$c" | awk '{print $1}')"
    fi
    [ -n "${id}" ] || return 1
    REG_ID="${id}"
    [ -n "${NOMBRE_DE[$id]:-}" ] && return 0
    c="$(creacion_de "${nombre}")"
    NOMBRE_DE[$id]="${nombre}"
    HASH_DE[$id]="$(resumen "${id}")"
    ID_DE_HASH[${HASH_DE[$id]}]="${id}"
    CREADA[$id]="$(printf '%s' "$c" | awk '{print $2}')"
    VISTA[$id]="$(ahora_utc)"
    ORDEN+=("${id}")
    return 0
}

volcar_instancias() {
    {
        printf 'nombre\tid\tx_instance_id\tcreada\tvista_por_primera_vez\tprimer_running\tprimer_sana\tprimera_peticion_atendida\tdesaparecio\n'
        local id
        for id in "${ORDEN[@]}"; do
            printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "${NOMBRE_DE[$id]}" "${id}" "${HASH_DE[$id]}" \
                "${CREADA[$id]:-}" "${VISTA[$id]:-}" "${RUNNING[$id]:-}" "${SANA[$id]:-}" "${ATIENDE[$id]:-}" "${RETIRADA[$id]:-}"
        done
    } >"${INST}"
}

emitir "G5 - Escenario 3A: ciclo de escalamiento automatico de la API (#225)"
emitir "====================================================================="
emitir "Proyecto : ${PROYECTO}   Zona: ${ZONA}   Grupo: ${GRUPO}"
emitir "Destino  : ${URL}${RUTA}   (${MUESTRAS} muestras por sondeo, cada ${POLL_S} s)"
emitir "Plazo    : ${PLAZO_MIN} min"
emitir ""

# ================================================================ 1. comprobar el punto de partida
marca "fase 1: comprobando el punto de partida"

IFS='|' read -r TAM REC ESTADO MINIMO <<<"$(estado_grupo)"
[ -n "${TAM:-}" ] || { emitir "ERROR: no se pudo leer ${GRUPO}. Hay sesion de gcloud y permisos sobre ${PROYECTO}?"; exit 1; }
if [ -z "${ESTADO:-}" ]; then
    emitir "ERROR: ${GRUPO} no tiene autoescalador. Enciende con -var api_autoscaling_enabled=true."
    exit 1
fi
marca "autoescalador: estado=${ESTADO} recomienda=${REC:-?} minimo=${MINIMO:-?}  grupo: objetivo=${TAM}"

if [ "${ESTADO}" != "ACTIVE" ]; then
    emitir "ERROR: el autoescalador no esta ACTIVE (${ESTADO}): $(detalle_autoescalador)"
    emitir "       Recien creado puede tardar unos minutos (nota 43). Espera y vuelve a lanzar esto."
    exit 1
fi
# El minimo tiene que ser 1. C4 y E2 lo suben a 2 para sus pruebas; si se quedo asi, el grupo ya esta
# en 2 sin carga y lo que se mediria seria configuracion, no autoescalado.
if [ -n "${MINIMO:-}" ] && [ "${MINIMO}" != "1" ]; then
    emitir "ERROR: el minimo del autoescalador es ${MINIMO}, no 1. Alguien lo dejo subido (C4/E2)."
    emitir "       Vuelve a aplicar sin -var api_min_replicas, o:"
    emitir "       gcloud compute instance-groups managed update-autoscaling ${GRUPO} --zone=${ZONA} --min-num-replicas=1"
    emitir "       y espera a que el grupo baje a 1 antes de lanzar esto."
    exit 1
fi
if [ "${TAM}" != "1" ]; then
    emitir "ERROR: el grupo tiene ${TAM} instancias. El escenario 3A arranca con UNA operativa."
    emitir "       Si hay carga de otra persona, espera; si no, espera a que el autoescalador reduzca."
    exit 1
fi

mapfile -t FILAS < <(instancias_grupo)
SANAS=0
for f in "${FILAS[@]}"; do
    IFS='|' read -r n id st ac sal <<<"${f}"
    registrar "${n}" "${id}" || { emitir "ERROR: no se pudo leer el ID de ${n}"; exit 1; }
    [ "${st}" = "RUNNING" ] && [ "${ac}" = "NONE" ] && [ "${sal}" = "HEALTHY" ] && SANAS=$((SANAS + 1))
done
for id in "${ORDEN[@]}"; do
    RUNNING[$id]="anterior a la corrida"; SANA[$id]="anterior a la corrida"
    emitir "    inicial: ${NOMBRE_DE[$id]}  id=${id}  X-Instance-ID=${HASH_DE[$id]}  creada=${CREADA[$id]}"
done
[ "${SANAS}" -eq 1 ] || { emitir "ERROR: ${SANAS} instancias RUNNING+HEALTHY+NONE; hace falta exactamente 1."; exit 1; }
INICIALES=("${ORDEN[@]}")

R="$(reparto)"
marca "reparto en reposo: ${R}"
# Un timeout suelto no dice nada del backend; que NADA responda, si.
printf ' %s' "${R}" | grep -qv ' sin-respuesta:[0-9]*$' || {
    emitir "ERROR: ninguna de ${MUESTRAS} peticiones a ${URL}${RUTA} respondio. Existen las reglas de reenvio?"
    exit 1
}
for par in ${R}; do
    h="${par%%:*}"
    [ "${h}" = "sin-respuesta" ] && continue
    if [ -z "${ID_DE_HASH[$h]:-}" ]; then
        emitir "ERROR: respondio un X-Instance-ID (${h}) que no es de ninguna instancia del grupo."
        emitir "       O hay otro backend detras del balanceador, o INSTANCE_ID no sale del servidor de"
        emitir "       metadatos (prepare_web_env.sh). Sin esa correspondencia no se puede demostrar que la"
        emitir "       instancia nueva atiende, asi que se para aqui."
        exit 1
    fi
done
for id in "${INICIALES[@]}"; do ATIENDE[$id]="anterior a la corrida"; VISTA_ANUNCIADA[$id]=1; done
volcar_instancias

hito "PARTIDA" "1 instancia operativa (${NOMBRE_DE[${INICIALES[0]}]}), autoescalador ACTIVE, minimo 1"
marca ">>> CAPTURA 1 AHORA: consola del grupo mooc-api-group con 1 instancia y el autoescalador (pestana Supervision) <<<"
emitir ""
marca "fase 2: observando. ARRANCA LA CARGA EN LA VM DEL GENERADOR (ver el runbook)."
emitir ""
emitir "    T+      objetivo recomienda  instancias (nombre estado accion salud)   reparto (X-Instance-ID:peticiones)"
emitir ""

# ================================================================ 2. observar
MAXIMO=1
SUBIO=0
BAJO=0
ESTABLE_EN_1=0
LIMITE=$((T0 + PLAZO_MIN * 60))

while [ "$(date +%s)" -lt "${LIMITE}" ]; do
    IFS='|' read -r TAM REC ESTADO _ <<<"$(estado_grupo)"
    mapfile -t FILAS < <(instancias_grupo)
    R="$(reparto)"
    UTC="$(ahora_utc)"
    TS=$(($(date +%s) - T0))

    # Decision de ampliar: el autoescalador recomienda mas, o el objetivo ya subio. Lo que sea primero.
    # Va antes que los hitos de instancias para que la cronologia quede en el orden en que ocurre.
    if [ "${SUBIO}" -eq 0 ] && { [ "${REC:-0}" -gt 1 ] 2>/dev/null || [ "${TAM:-0}" -gt 1 ] 2>/dev/null; }; then
        SUBIO=1
        hito "DECISION_AMPLIAR" "el autoescalador recomienda ${REC:-?} y el objetivo es ${TAM:-?} (partida: 1)"
        marca ">>> CAPTURA 2 AHORA: pestana Supervision del grupo, tamano recomendado > 1 y la utilizacion del balanceador <<<"
    fi
    if [ -n "${TAM:-}" ] && [ "${TAM}" -gt "${MAXIMO}" ] 2>/dev/null; then MAXIMO="${TAM}"; fi

    if [ "${SUBIO}" -eq 1 ] && [ "${BAJO}" -eq 0 ] && [ -n "${TAM:-}" ] && [ "${TAM}" -lt "${MAXIMO}" ] 2>/dev/null; then
        BAJO=1
        hito "DECISION_REDUCIR" "el objetivo baja de ${MAXIMO} a ${TAM}"
        marca ">>> CAPTURA 5 AHORA: el grupo reduciendo (instancia en DELETING o ya retirada) <<<"
    fi

    PRESENTES=" "
    RESUMEN_INST=""
    for f in "${FILAS[@]}"; do
        IFS='|' read -r n id st ac sal <<<"${f}"
        [ -n "${n}" ] || continue
        if ! registrar "${n}" "${id}"; then
            RESUMEN_INST+="${n}(sin-id) "
            continue
        fi
        id="${REG_ID}"
        PRESENTES+="${id} "
        RESUMEN_INST+="${n}/${HASH_DE[$id]}/${st:-?}/${ac:-?}/${sal:-?} "
        if [ -z "${RUNNING[$id]:-}" ] && [ "${st}" = "RUNNING" ]; then
            RUNNING[$id]="${UTC}"
            hito "INSTANCIA_RUNNING" "${n} (id ${id}) arrancada; creada ${CREADA[$id]}"
        fi
        if [ -z "${SANA[$id]:-}" ] && [ "${sal}" = "HEALTHY" ] && [ "${ac}" = "NONE" ]; then
            SANA[$id]="${UTC}"
            hito "INSTANCIA_SANA" "${n} HEALTHY ante el balanceador y sin accion en curso"
            marca ">>> CAPTURA 3 AHORA: el grupo con la instancia nueva sana (lista de instancias del grupo) <<<"
        fi
    done

    # Hitos de las instancias nuevas: aparicion, primera peticion atendida, retiro.
    for id in "${ORDEN[@]}"; do
        if [ -z "${VISTA_ANUNCIADA[$id]:-}" ]; then
            VISTA_ANUNCIADA[$id]=1
            case " ${INICIALES[*]} " in *" ${id} "*) ;; *)
                hito "INSTANCIA_CREADA" "${NOMBRE_DE[$id]} (id ${id}, X-Instance-ID ${HASH_DE[$id]}), creationTimestamp ${CREADA[$id]}" ;;
            esac
        fi
        if [ -z "${ATIENDE[$id]:-}" ] && printf ' %s ' "${R}" | grep -q " ${HASH_DE[$id]}:"; then
            ATIENDE[$id]="${UTC}"
            hito "INSTANCIA_ATIENDE" "${NOMBRE_DE[$id]} respondio peticiones del balanceador (${R})"
            marca ">>> CAPTURA 4 AHORA: esta linea de la terminal, con el reparto entre dos X-Instance-ID <<<"
        fi
        if [ -z "${RETIRADA[$id]:-}" ] && [[ "${PRESENTES}" != *" ${id} "* ]]; then
            RETIRADA[$id]="${UTC}"
            hito "INSTANCIA_RETIRADA" "${NOMBRE_DE[$id]} (id ${id}) ya no esta en el grupo"
        fi
    done

    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "${UTC}" "${TS}" "${TAM:-}" "${REC:-}" "${ESTADO:-}" \
        "${RESUMEN_INST% }" "${R}" >>"${OBS}"
    marca "$(printf '%-8s %-11s %s  | %s' "${TAM:-?}" "${REC:-?}" "${RESUMEN_INST}" "${R}")"

    # Fin: subio, bajo, el objetivo esta en 1 y solo queda una instancia, dos sondeos seguidos.
    if [ "${BAJO}" -eq 1 ] && [ "${TAM:-0}" = "1" ] && [ "${#FILAS[@]}" -eq 1 ]; then
        ESTABLE_EN_1=$((ESTABLE_EN_1 + 1))
        if [ "${ESTABLE_EN_1}" -ge 2 ]; then
            hito "CICLO_COMPLETO" "subio a ${MAXIMO} y volvio a 1 (que nadie lo toco lo confirma la auditoria en g5_linea_de_tiempo.py)"
            marca ">>> CAPTURA 6 AHORA: el grupo de nuevo en 1 instancia, con la grafica del ciclo completo <<<"
            break
        fi
    else
        ESTABLE_EN_1=0
    fi
    volcar_instancias
    sleep "${POLL_S}"
done
volcar_instancias

# ================================================================ 3. afirmar
emitir ""
marca "fase 3: comprobaciones"
FALLOS=0
comprobar() { # condicion-ok mensaje
    if [ "$1" -eq 1 ]; then emitir "    OK     $2"; else emitir "    FALLA  $2"; FALLOS=$((FALLOS + 1)); fi
}
NUEVAS=0
NUEVA_ATIENDE=0
for id in "${ORDEN[@]}"; do
    case " ${INICIALES[*]} " in *" ${id} "*) continue ;; esac
    NUEVAS=$((NUEVAS + 1))
    [ -n "${ATIENDE[$id]:-}" ] && NUEVA_ATIENDE=1
done
comprobar "${SUBIO}" "el autoescalador decidio ampliar (recomendado u objetivo > 1)"
comprobar "$([ "${NUEVAS}" -ge 1 ] && echo 1 || echo 0)" "se creo al menos una instancia nueva (${NUEVAS}), por ID y no por nombre"
comprobar "${NUEVA_ATIENDE}" "una instancia nueva atendio peticiones que entraron por el balanceador"
comprobar "${BAJO}" "el autoescalador redujo el grupo despues"
comprobar "$([ "${ESTABLE_EN_1}" -ge 2 ] && echo 1 || echo 0)" "el grupo volvio a 1 instancia dentro del plazo (${PLAZO_MIN} min)"
emitir ""
emitir "Siguiente paso: python scripts/g5_linea_de_tiempo.py <carpeta de la carga> ${SALIDA_DIR}"
emitir "(cruza esto con la tasa real de JMeter y con el registro de auditoria: que nadie redimensiono a mano)."
[ "${FALLOS}" -eq 0 ] || { marca "${FALLOS} comprobacion(es) fallida(s)"; exit 1; }
marca "todas las comprobaciones pasan"
exit 0
