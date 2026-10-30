#!/usr/bin/env bash
# Reparto de trafico y exclusion por salud (C4, #207).
#
# QUE DEMUESTRA, EN TRES PARTES
#
#   1. Que el balanceador reparte entre dos replicas sanas, identificandolas por
#      la cabecera `X-Instance-ID` que pone la API (A2).
#   2. Que al degradar una replica a proposito el balanceador **deja de
#      mandarle peticiones nuevas**, y cuanto tarda.
#   3. Que al recuperarla vuelve a la rotacion, y cuanto tarda.
#
# COMO SE DEGRADA, Y POR QUE ASI
#
# Se para el contenedor `api` de una instancia, no el proxy. Entonces nginx sigue
# vivo y responde **502** a `/api/v1/ready`: la comprobacion de salud ve un codigo
# HTTP incorrecto, que es un fallo de aplicacion. Parar el proxy daria conexion
# rechazada, que es un fallo de maquina y prueba otra cosa.
#
# LO QUE NO MIDE, Y ES DELIBERADO
#
# La reparacion por el grupo administrado. El autohealing usa **la misma**
# comprobacion de salud, asi que se dispara a la vez; el guion lo observa y lo
# anota, pero quien lo mide de punta a punta es C1. La diferencia entre los dos
# mecanismos esta explicada en la evidencia de C4.
#
# USO
#
#   bash ./scripts/c4_reparto_y_salud.sh <fichero-de-salida>
set -uo pipefail

PROYECTO="${PROYECTO:-plataforma-mooc-entrega2}"
ZONA="${ZONA:-us-east1-b}"
GRUPO="${GRUPO:-mooc-api-group}"
URL="${URL:-https://136.68.55.254.sslip.io}"
RUTA="${RUTA:-/api/v1/ready}"
MUESTRAS="${MUESTRAS:-40}"
SALIDA="${1:-}"

emitir() { if [ -n "${SALIDA}" ]; then echo "$*" | tee -a "${SALIDA}"; else echo "$*"; fi; }
[ -z "${SALIDA}" ] || : >"${SALIDA}"

T0=$(date +%s)
marca() { emitir "$(printf '[%s] T+%-6s %s' "$(date -u +%H:%M:%SZ)" "$(($(date +%s) - T0))s" "$*")"; }

# Una peticion: devuelve el identificador de replica, o el codigo si falla.
quien() {
    local c id
    id="$(curl -sS -o /dev/null -D - --max-time 8 "${URL}${RUTA}" 2>/dev/null |
        tr -d '\r' | awk 'tolower($1)=="x-instance-id:"{print $2}')"
    [ -n "${id}" ] && { printf '%s' "${id}"; return; }
    c="$(curl -s -o /dev/null -w '%{http_code}' --max-time 8 "${URL}${RUTA}" 2>/dev/null)"
    printf 'sin-id(%s)' "${c:-timeout}"
}

# Reparto sobre N peticiones, como "id:cuenta id:cuenta".
reparto() {
    local n="$1" i
    for i in $(seq 1 "${n}"); do quien; echo; done | sort | uniq -c |
        awk '{printf "%s:%s ", $2, $1}'
}

instancias_del_grupo() {
    gcloud compute instance-groups managed list-instances "${GRUPO}" \
        --zone="${ZONA}" --project="${PROYECTO}" --format="value(name)" 2>/dev/null | tr -d '\r'
}

salud_backend() {
    gcloud compute instance-groups managed list-instances "${GRUPO}" \
        --zone="${ZONA}" --project="${PROYECTO}" \
        --format="value(name,instanceHealth[0].detailedHealthState,currentAction)" 2>/dev/null | tr -d '\r'
}

# El identificador que publica una instancia concreta, preguntado por dentro.
id_de_instancia() {
    gcloud compute ssh "$1" --zone="${ZONA}" --tunnel-through-iap --project="${PROYECTO}" \
        --quiet --command='curl -sI --max-time 5 http://127.0.0.1/api/v1/ready | tr -d "\r" | awk "tolower(\$1)==\"x-instance-id:\"{print \$2}"' \
        2>/dev/null | tr -d '\r' | tail -1
}

emitir "C4 - Reparto de trafico y exclusion por salud"
emitir "=============================================="
emitir "Destino : ${URL}${RUTA}"
emitir "Grupo   : ${GRUPO}  (${ZONA})"
emitir ""

# ---------------------------------------------------------------- 1. inventario
mapfile -t INSTANCIAS < <(instancias_del_grupo)
[ "${#INSTANCIAS[@]}" -ge 2 ] || {
    emitir "ERROR: hacen falta al menos 2 instancias y hay ${#INSTANCIAS[@]}."
    emitir "       Aplica con -var api_min_replicas=2."
    exit 1
}
marca "instancias en el grupo: ${INSTANCIAS[*]}"

emitir ""
emitir "Correspondencia entre instancia y el identificador que publica:"
declare -A ID_DE
for n in "${INSTANCIAS[@]}"; do
    id="$(id_de_instancia "${n}")"
    [ -n "${id}" ] || { emitir "ERROR: no se pudo leer el X-Instance-ID de ${n}"; exit 1; }
    ID_DE["${n}"]="${id}"
    emitir "    ${n}  ->  ${id}"
done
emitir ""

# ------------------------------------------------------- 2. reparto en reposo
marca "midiendo el reparto con las dos sanas (${MUESTRAS} peticiones)"
REPARTO_SANO="$(reparto "${MUESTRAS}")"
marca "reparto: ${REPARTO_SANO}"
distintos="$(printf '%s' "${REPARTO_SANO}" | tr ' ' '\n' | grep -c ':' || true)"
if [ "${distintos}" -ge 2 ]; then
    marca ">>> OK: el balanceador reparte entre ${distintos} replicas <<<"
else
    marca ">>> ATENCION: todas las peticiones fueron a la misma replica <<<"
fi
emitir ""

# ------------------------------------------------------------- 3. degradacion
VICTIMA="${INSTANCIAS[0]}"
ID_VICTIMA="${ID_DE[${VICTIMA}]}"
marca ">>> DEGRADANDO ${VICTIMA} (${ID_VICTIMA}): se para su contenedor api <<<"
marca ">>> TOMA LA CAPTURA DEL REPARTO SANO AHORA, antes de que cambie <<<"

if ! gcloud compute ssh "${VICTIMA}" --zone="${ZONA}" --tunnel-through-iap \
    --project="${PROYECTO}" --quiet \
    --command='sudo docker compose -p mooc -f /opt/mooc/docker-compose.prod.yml --env-file /opt/mooc/.env stop api' >/dev/null 2>&1; then
    emitir "ERROR: no se pudo parar el contenedor api de ${VICTIMA}"
    exit 1
fi
T_DEGRADA=$(date +%s)
marca "contenedor api parado en ${VICTIMA}"

# --------------------------------------------- 4. cuanto tarda en excluirla
marca "esperando a que el balanceador deje de mandarle peticiones"
EXCLUIDA=0
for _ in $(seq 1 40); do
    r="$(reparto 10)"
    s="$(salud_backend | tr '\t' ' ' | tr '\n' ' | ')"
    marca "reparto: ${r}  salud: ${s}"
    case "${r}" in
        *"${ID_VICTIMA}:"*) ;;
        *)
            EXCLUIDA=$(( $(date +%s) - T_DEGRADA ))
            marca ">>> EXCLUIDA: 0 peticiones a ${ID_VICTIMA} tras ${EXCLUIDA}s <<<"
            marca ">>> CAPTURA AHORA: la consola muestra la replica no sana <<<"
            break
            ;;
    esac
    sleep 10
done
[ "${EXCLUIDA}" -gt 0 ] || marca "ATENCION: seguia recibiendo trafico al agotarse el plazo"
emitir ""

# ------------------------------------------------------------ 5. recuperacion
marca ">>> RECUPERANDO ${VICTIMA}: se arranca su contenedor api <<<"
gcloud compute ssh "${VICTIMA}" --zone="${ZONA}" --tunnel-through-iap \
    --project="${PROYECTO}" --quiet \
    --command='sudo docker compose -p mooc -f /opt/mooc/docker-compose.prod.yml --env-file /opt/mooc/.env start api' >/dev/null 2>&1
T_RECUPERA=$(date +%s)
marca "contenedor api arrancado"

VUELTA=0
for _ in $(seq 1 40); do
    r="$(reparto 10)"
    s="$(salud_backend | tr '\t' ' ' | tr '\n' ' | ')"
    marca "reparto: ${r}  salud: ${s}"
    case "${r}" in
        *"${ID_VICTIMA}:"*)
            VUELTA=$(( $(date +%s) - T_RECUPERA ))
            marca ">>> REINCORPORADA: vuelve a recibir trafico tras ${VUELTA}s <<<"
            break
            ;;
    esac
    sleep 10
done
[ "${VUELTA}" -gt 0 ] || marca "ATENCION: no volvio a la rotacion en el plazo (quiza el grupo la recreo)"

emitir ""
emitir "=============================================="
emitir "RESUMEN"
emitir "  reparto con dos sanas        : ${REPARTO_SANO}"
emitir "  exclusion tras degradar      : ${EXCLUIDA}s"
emitir "  reincorporacion tras reparar : ${VUELTA}s"
emitir ""
emitir "La reparacion por el grupo administrado es otro mecanismo y otra escala:"
emitir "la mide C1, y son minutos frente a estos segundos."
