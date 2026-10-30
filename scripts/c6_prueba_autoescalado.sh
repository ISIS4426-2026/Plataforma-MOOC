#!/usr/bin/env bash
# Prueba del autoescalado de workers por senal de cola (C6, #209), con la
# cronologia que acompana a las capturas.
#
# QUE HACE, EN DOS FASES
#
#   1. COMPROBAR. Lee la serie con **el filtro exacto que usa el autoescalador**
#      y falla si no devuelve nada. Esto es lo primero y no un detalle: un filtro
#      que no encuentra la serie **no es un error** para GCE. El autoescalador sin
#      datos mantiene el tamano actual, asi que el sintoma no es un fallo sino un
#      grupo que no escala. Esta fase convierte eso en un error ruidoso.
#
#   2. OBSERVAR. Imprime una linea por sondeo con T+, hora UTC, el valor de
#      `outstanding`, lo que el autoescalador recomienda, el tamano del grupo y el
#      estado de cada instancia. Esa salida es la evidencia escrita; las capturas
#      de la consola se toman en paralelo mirando el mismo intervalo.
#
# EL GUION NO GENERA CARGA, Y ES A PROPOSITO
#
# La carga la encola `scripts/c6_encolar_carga.sh`, que se lanza **despues** y
# en otra terminal. Separarlos deja que este mida sin competir por la red con
# las subidas, y que el otro se pueda repetir si una subida falla.
#
# `scripts/a7_verificar_e2e.sh` **no sirve** para esto, aunque sea el correcto
# para C5: sube un PDF y un PNG --escaneo de segundos-- y espera a que cada uno
# llegue a `completed` antes de seguir, asi que la cola se vacia tan rapido como
# se llena y `outstanding` no pasa de 2.
#
# USO
#
#   bash ./scripts/c6_prueba_autoescalado.sh                 # a la salida estandar
#   bash ./scripts/c6_prueba_autoescalado.sh <fichero>       # y tambien a un fichero
#
# y en otra terminal, cuando este guion lo pida:
#
#   BASE_URL="$(cd infra/terraform && terraform output -raw lb_url)" \
#     bash ./scripts/c6_encolar_carga.sh 10
set -uo pipefail

PROYECTO="${PROYECTO:-plataforma-mooc-entrega2}"
ZONA="${ZONA:-us-east1-b}"
GRUPO="${GRUPO:-mooc-worker-group}"
AUTOESCALADOR="${AUTOESCALADOR:-mooc-worker-autoscaler}"

# Tienen que coincidir con `local.queue_metric_namespace` y con la constante de
# `cmd/queue-exporter/main.go`. Si no coinciden, la fase 1 falla, que es
# exactamente para lo que esta.
ESPACIO="${ESPACIO:-mooc-cola}"
METRICA="${METRICA:-custom.googleapis.com/mooc/queue/outstanding}"

PLAZO_SONDEOS="${PLAZO_SONDEOS:-120}" # 120 x 30 s = 60 min
SALIDA="${1:-}"

emitir() {
    if [ -n "${SALIDA}" ]; then echo "$*" | tee -a "${SALIDA}"; else echo "$*"; fi
}

[ -z "${SALIDA}" ] || : >"${SALIDA}"

T0=$(date +%s)
marca() { emitir "$(printf '[%s] T+%-6s %s' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(($(date +%s) - T0))s" "$*")"; }

# El filtro, escrito una vez. Es literalmente el de `worker_autoscaler.tf`, mas
# el `metric.type`, que alli lo lleva el campo `name` del bloque `metric`.
FILTRO="metric.type = \"${METRICA}\" AND resource.type = \"generic_node\" AND resource.labels.namespace = \"${ESPACIO}\""

# Sin `jq`: este guion se corre desde Windows con Git Bash y jq no viene con
# gcloud. Un `grep -o` sobre la respuesta basta porque solo hace falta el primer
# punto, y la API los devuelve del mas reciente al mas antiguo.
respuesta_monitoring() {
    local desde
    desde="$(date -u -d '-5 minutes' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null)" ||
        desde="$(date -u -v-5M +%Y-%m-%dT%H:%M:%SZ)"

    curl -s -G \
        -H "Authorization: Bearer $(gcloud auth print-access-token --project="${PROYECTO}" 2>/dev/null)" \
        "https://monitoring.googleapis.com/v3/projects/${PROYECTO}/timeSeries" \
        --data-urlencode "filter=${FILTRO}" \
        --data-urlencode "interval.startTime=${desde}" \
        --data-urlencode "interval.endTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        2>/dev/null
}

# Devuelve el valor, o la cadena vacia si el filtro no encontro serie. La
# diferencia entre «no hay serie» y «la peticion fallo» la decide quien llama:
# por eso esta funcion solo extrae y no interpreta.
valor_de() {
    printf '%s' "$1" | grep -o '"int64Value" *: *"[0-9]*"' | head -1 |
        sed 's/.*"\([0-9]*\)"$/\1/'
}

# EL ESTADO SE LEE POR EL GRUPO, NO POR EL AUTOESCALADOR.
#
# `gcloud compute autoscalers` **no existe** en el SDK instalado --comprobado
# en 586.0.0: `Invalid choice: 'autoscalers'`--. El unico sitio donde este
# gcloud expone el estado es el campo `autoscaler` del grupo gestionado, que
# trae lo mismo: `status`, `recommendedSize` y `statusDetails`.
#
# El `sed` final no es decorativo: si el grupo existe y **no tiene**
# autoescalador, gcloud no imprime una linea vacia sino una linea de
# tabuladores, que para `[ -z ]` no esta vacia. Sin el, la comprobacion de mas
# abajo daria por bueno un autoescalador que no existe.
estado_autoescalador() {
    gcloud compute instance-groups managed describe "${GRUPO}" \
        --zone="${ZONA}" --project="${PROYECTO}" \
        --format="value(autoscaler.status,autoscaler.recommendedSize,autoscaler.statusDetails[0].message)" \
        2>/dev/null | tr -d '\r' | sed 's/^[[:space:]]*$//'
}

estado_del_grupo() {
    gcloud compute instance-groups managed list-instances "${GRUPO}" \
        --zone="${ZONA}" --project="${PROYECTO}" \
        --format="value(name,instanceStatus,currentAction,instanceHealth[0].detailedHealthState)" \
        2>/dev/null | tr -d '\r'
}

tamano_del_grupo() {
    gcloud compute instance-groups managed describe "${GRUPO}" \
        --zone="${ZONA}" --project="${PROYECTO}" \
        --format="value(targetSize)" 2>/dev/null | tr -d '\r'
}

emitir "Autoescalado de workers por senal de cola (C6 #209)"
emitir "===================================================="
emitir "Proyecto : ${PROYECTO}   Zona: ${ZONA}"
emitir "Grupo    : ${GRUPO}"
emitir "Metrica  : ${METRICA}"
emitir "Filtro   : ${FILTRO}"
emitir ""

# ------------------------------------------------------------------- 1. comprobar
marca "fase 1: comprobando que el autoescalador puede VER la serie"

RESP="$(respuesta_monitoring)"
if printf '%s' "${RESP}" | grep -q '"error"'; then
    emitir "ERROR: la API de Monitoring devolvio un error:"
    emitir "$(printf '%s' "${RESP}" | tr -d '\r' | head -c 600 | sed 's/^/    /')"
    exit 1
fi

VALOR="$(valor_de "${RESP}")"
if [ -z "${VALOR}" ]; then
    emitir "ERROR: el filtro del autoescalador NO devuelve ninguna serie."
    emitir ""
    emitir "Esto es el fallo silencioso de C6: para GCE no es un error, y el"
    emitir "autoescalador se limita a mantener el tamano actual. Donde mirar, en"
    emitir "este orden:"
    emitir ""
    emitir "  1. El exportador corre en la VM de la cola:"
    emitir "       gcloud compute ssh mooc-queue-server --zone=${ZONA} --tunnel-through-iap \\"
    emitir "         --command='sudo systemctl status mooc-queue-exporter'"
    emitir "  2. Sus registros, que dicen contra que proyecto y cola escribe:"
    emitir "       ... --command='sudo journalctl -u mooc-queue-exporter -n 50 --no-pager'"
    emitir "  3. El espacio de nombres coincide en los dos lados:"
    emitir "       terraform output worker_scaling_metric"
    emitir "       ... --command='sudo cat /etc/mooc/queue-exporter.env'"
    emitir "  4. La cola nunca recibio un trabajo: entonces la serie existe y vale 0."
    emitir "     Que no salga NADA es distinto de que salga 0."
    exit 1
fi
marca ">>> ok, la serie existe y vale ${VALOR}. El autoescalador ve lo mismo. <<<"

DETALLE="$(estado_autoescalador)"
if [ -z "${DETALLE}" ]; then
    emitir "ERROR: no existe el autoescalador ${AUTOESCALADOR}."
    emitir "       Hace falta: terraform apply -var worker_autoscaling_enabled=true"
    exit 1
fi
marca "autoescalador: ${DETALLE}"

# `CUSTOM_METRIC_INVALID` RECIEN CREADO EL AUTOESCALADOR NO ES UN FALLO.
#
# GCE valida la metrica al crear el recurso y deja el veredicto pegado en
# `statusDetails` hasta el primer sondeo que si la lee. Como el `apply` crea el
# autoescalador y la VM de la cola a la vez, durante unos minutos el estado es
# `ERROR / The monitoring metric that was specified does not exist`. Se limpia
# solo --medido: ACTIVE a los ~4 min-- y mientras tanto el grupo se queda en
# `min_replicas`, que es su comportamiento normal con la cola vacia.
#
# La fase 1 ya ha demostrado que la serie existe, asi que aqui solo se avisa.
if printf '%s' "${DETALLE}" | grep -q "does not exist"; then
    emitir ""
    emitir "AVISO: el autoescalador dice todavia CUSTOM_METRIC_INVALID. Es el"
    emitir "       veredicto del momento en que se creo, y la fase 1 acaba de"
    emitir "       leer la serie: espera unos minutos y vuelve a lanzar esto."
    emitir "       NO toques nada; no hay nada que arreglar."
    emitir ""
fi

marca ">>> TOMA LA CAPTURA DE 'ANTES': grupo en 1 replica y cola vacia <<<"
emitir ""

# ------------------------------------------------------------------- 2. observar
marca "fase 2: observando. GENERA LA CARGA AHORA (ver la cabecera del guion)."
emitir ""
emitir "    T+      outstanding  recomienda  tamano  instancias"
emitir ""

MAXIMO_VISTO=0
TAMANO_INICIAL="$(tamano_del_grupo)"
# Si `gcloud` no devolviera el tamano, las comparaciones de abajo serian errores
# de `test` y los dos hitos no se anunciarian nunca: un falso negativo silencioso.
# Mejor parar aqui, que es el lado seguro.
[ -n "${TAMANO_INICIAL}" ] || { emitir "ERROR: no se pudo leer targetSize de ${GRUPO}."; exit 1; }

for _ in $(seq 1 "${PLAZO_SONDEOS}"); do
    RESP="$(respuesta_monitoring)"
    PENDIENTE="$(valor_de "${RESP}")"
    DETALLE="$(estado_autoescalador)"
    RECOMENDADO="$(printf '%s' "${DETALLE}" | awk '{print $2}')"
    TAMANO="$(tamano_del_grupo)"

    marca "$(printf '%-12s %-11s %-7s %s' \
        "${PENDIENTE:-sin-serie}" "${RECOMENDADO:-?}" "${TAMANO:-?}" \
        "$(estado_del_grupo | awk '{printf "%s(%s/%s) ", $1, $3, $4}')")"

    # El hito que C6 tiene que demostrar: el grupo crecio **por la senal**, no
    # porque alguien lo redimensionara. Que lo pida el autoescalador y no un
    # `resize` es lo que el enunciado distingue: «la ampliacion manual del grupo
    # no acredita autoscaling».
    if [ -n "${TAMANO}" ] && [ -n "${TAMANO_INICIAL}" ] &&
        [ "${TAMANO}" -gt "${TAMANO_INICIAL}" ] && [ "${TAMANO}" -gt "${MAXIMO_VISTO}" ]; then
        MAXIMO_VISTO="${TAMANO}"
        marca ">>> EL AUTOESCALADOR AMPLIO EL GRUPO A ${TAMANO}. CAPTURA AHORA Y EN LOS PROXIMOS 2 MIN <<<"
    fi

    # Y el otro hito: la reduccion. `scale_in_control` la limita a una replica
    # cada 900 s, asi que de 3 a 1 tarda al menos media hora: el plazo por
    # omision de este guion esta dimensionado para verla.
    if [ "${MAXIMO_VISTO}" -gt 0 ] && [ -n "${TAMANO}" ] &&
        [ "${TAMANO}" -eq "${TAMANO_INICIAL}" ] && [ "${PENDIENTE:-1}" = "0" ]; then
        marca ">>> CICLO COMPLETO: subio a ${MAXIMO_VISTO} y volvio a ${TAMANO_INICIAL}. <<<"
        emitir ""
        emitir "Para la nota 10, exporta los puntos crudos ANTES de que caduquen:"
        emitir "  los de esta corrida estan en la salida de arriba; guardala en"
        emitir "  docs/entrega3/evidencias/C6/."
        exit 0
    fi

    sleep 30
done

marca "fin del plazo de $((PLAZO_SONDEOS * 30)) s. Maximo tamano visto: ${MAXIMO_VISTO:-ninguno}"
[ "${MAXIMO_VISTO}" -gt 0 ] || {
    emitir "ERROR: el grupo nunca crecio. Con ${VALOR} trabajos por atender y"
    emitir "       single_instance_assignment=2, el autoescalador solo amplia si"
    emitir "       outstanding pasa de 2. Comprueba que la carga llego a la cola."
    exit 1
}
exit 0
