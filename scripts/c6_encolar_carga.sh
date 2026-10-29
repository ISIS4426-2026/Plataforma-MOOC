#!/usr/bin/env bash
# Encola trabajo de transcodificacion suficiente para que el autoescalador de C6
# (#209) tenga algo que decidir, por el camino de produccion.
#
# POR QUE NO SIRVE `a7_verificar_e2e.sh` PARA ESTO
#
# Ese guion es el correcto para **C5**: demuestra que una instancia nueva consume
# y termina su primer trabajo. Pero sube un PDF y un PNG, que solo pasan por el
# escaneo --segundos-- y espera a que cada uno llegue a `completed` antes de
# seguir. La cola nunca acumula: se vacia tan rapido como se llena.
#
# Y la cola es justo lo que C6 necesita que crezca. Con
# `single_instance_assignment = 2` hacen falta mas de **4 trabajos sin terminar a
# la vez** para que el autoescalador pida la tercera replica.
#
# EL ORDEN IMPORTA: PRIMERO TODAS LAS SUBIDAS, DESPUES TODAS LAS CONFIRMACIONES
#
# El trabajo se encola en la **confirmacion** (`/complete` devuelve 202), no en el
# PUT al bucket. Si se hiciera subir-confirmar-subir-confirmar, la cola crecería
# al ritmo de la subida: con un enlace domestico, el primer trabajo estaria
# terminado antes de que el sexto se encolara, y `outstanding` no pasaria de 2.
#
# Subiendo los N objetos primero y confirmandolos luego uno tras otro, la cola
# salta a N en unos segundos **sea cual sea la velocidad de subida**. Es la unica
# parte no obvia de este guion.
#
# POR QUE VIDEO Y NO UN DOCUMENTO
#
# Un documento se escanea en segundos; un video de 120 s se transcodifica en unos
# 144 s (A7 midio 1,2x la duracion). Con 10 trabajos y un worker a concurrencia
# 2, la cola tarda unos 12 min en vaciarse: tiempo de sobra para que el
# autoescalador muestree, decida, arranque la replica --unos 420 s-- y se vea en
# la consola con trabajo por delante.
#
# Usa `scratch/video_corto.mp4`, el perfil que `scripts/generar_media_prueba.sh`
# genera y **verifica campo por campo**. No vale un clip cualquiera: la Entrega 1
# perdio un tercio de la nota por declarar 2 minutos y subir 8 segundos.
#
# ESTE GUION NO MIDE NADA
#
# Solo encola. La cronologia y el veredicto son de `c6_prueba_autoescalado.sh`,
# que se lanza **antes** y en otra terminal.
#
# USO
#
#   BASE_URL=https://<dominio> bash ./scripts/c6_encolar_carga.sh [n]
#
#   n   trabajos a encolar; por omision 10
set -uo pipefail

BASE_URL="${BASE_URL:?Declara el destino, p. ej. BASE_URL=https://34.24.52.111.sslip.io}"
PROFESOR="${PROFESOR:-profesor1@plataforma-mooc.test}"
CLAVE="${CLAVE:-Password123!}"
VIDEO="${VIDEO:-scratch/video_corto.mp4}"
CUANTOS="${1:-10}"

[ -f "${VIDEO}" ] || {
    echo "No encuentro ${VIDEO}." >&2
    echo "Generalo y verificalo antes: bash ./scripts/generar_media_prueba.sh corto" >&2
    exit 1
}

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

TOKEN=""
TAM=$(wc -c <"${VIDEO}" | tr -d ' ')

# Los mismos auxiliares que `a7_verificar_e2e.sh`, y por las mismas razones: el
# `Origin` es obligatorio para la proteccion CSRF de todo lo que cambie estado, y
# el codigo HTTP va a un archivo porque casi toda llamada ocurre dentro de
# `$(...)`, donde una asignacion no sobrevive al subshell.
api() { # $1=metodo $2=ruta $3=cuerpo
    local m="$1" ruta="$2" cuerpo="${3:-}"
    local args=(-s -o "${TMP}/cuerpo" -w '%{http_code}' -X "$m"
        -H "Origin: ${BASE_URL}" -H "Content-Type: application/json")
    [ -n "${TOKEN:-}" ] && args+=(-H "Authorization: Bearer ${TOKEN}")
    [ -n "${cuerpo}" ] && args+=(-d "${cuerpo}")
    curl "${args[@]}" "${BASE_URL}${ruta}" >"${TMP}/codigo"
    cat "${TMP}/cuerpo"
}
codigo() { cat "${TMP}/codigo" 2>/dev/null || echo "?"; }
json() { python -c "import json,sys;d=json.load(sys.stdin);print(d$1)" 2>/dev/null; }

marca() { printf '[%s] %s\n' "$(date -u +%H:%M:%SZ)" "$*"; }

# Un solo login, a proposito: el limitador son 10 intentos por minuto y por IP.
marca "iniciando sesion como ${PROFESOR}"
TOKEN="$(api POST /api/v1/auth/login "{\"email\":\"${PROFESOR}\",\"password\":\"${CLAVE}\"}" | json "['token']")"
[ -n "${TOKEN}" ] || {
    echo "ERROR: no se pudo iniciar sesion (HTTP $(codigo)). Hace falta sembrar la base." >&2
    exit 1
}

marca "creando el curso de la prueba"
CURSO="$(api POST /api/v1/courses \
    '{"title":"C6 carga de autoescalado","description":"Curso temporal de la prueba de autoescalado de C6."}' | json "['id']")"
MODULO="$(api POST "/api/v1/courses/${CURSO}/modules" '{"title":"Modulo C6"}' | json "['id']")"
UNIDAD="$(api POST "/api/v1/modules/${MODULO}/units" '{"title":"Unidad C6"}' | json "['id']")"
[ -n "${UNIDAD}" ] || {
    echo "ERROR: no se pudo crear la estructura del curso (HTTP $(codigo))." >&2
    exit 1
}
marca "curso ${CURSO}"

# ----------------------------------------------- fase 1: subir los N objetos
# Nada se encola todavia. Esta fase puede tardar lo que tarde el enlace.
RECURSOS=()
CLAVES=()
for i in $(seq 1 "${CUANTOS}"); do
    rid="$(api POST "/api/v1/units/${UNIDAD}/resources" \
        "{\"title\":\"C6 video ${i}\",\"type\":\"video\",\"is_visible\":true,\"is_mandatory\":false,\"allow_download\":false}" |
        json "['id']")"
    [ -n "${rid}" ] || {
        marca "  ${i}: no se pudo crear el recurso (HTTP $(codigo)), lo salto"
        continue
    }

    resp="$(api POST /api/v1/media/presigned-url \
        "{\"resource_id\":\"${rid}\",\"filename\":\"c6_${i}.mp4\",\"mime_type\":\"video/mp4\",\"file_size_bytes\":${TAM}}")"
    url="$(printf '%s' "${resp}" | json "['upload_url']")"
    clave="$(printf '%s' "${resp}" | json "['object_key']")"
    [ -n "${url}" ] || {
        marca "  ${i}: sin URL prefirmada (HTTP $(codigo)), lo salto"
        continue
    }

    put="$(curl -s -o /dev/null -w '%{http_code}' -X PUT \
        -H "Content-Type: video/mp4" --data-binary "@${VIDEO}" "${url}")"
    [ "${put}" = "200" ] || [ "${put}" = "204" ] || {
        marca "  ${i}: el PUT al bucket devolvio ${put}, lo salto"
        continue
    }

    RECURSOS+=("${rid}")
    CLAVES+=("${clave}")
    marca "subido ${i}/${CUANTOS}"
done

TOTAL="${#RECURSOS[@]}"
[ "${TOTAL}" -gt 0 ] || {
    echo "ERROR: no se subio ningun objeto. No hay nada que encolar." >&2
    exit 1
}

if [ "${TOTAL}" -lt 5 ]; then
    marca "AVISO: solo ${TOTAL} objetos. Con single_instance_assignment=2 el"
    marca "       autoescalador pide 3 replicas a partir de 5 trabajos sin"
    marca "       terminar; con ${TOTAL} puede quedarse en 2 o en 1."
fi

# -------------------------------- fase 2: confirmar, una detras de otra
# Aqui es donde se encola. Sin pausas entre llamadas: lo que se busca es que
# `outstanding` salte a ${TOTAL} antes de que el worker termine el primero.
marca ">>> CONFIRMANDO ${TOTAL} subidas SEGUIDAS: la cola salta ahora <<<"
ENCOLADOS=0
for i in $(seq 0 $((TOTAL - 1))); do
    api POST "/api/v1/media/uploads/${RECURSOS[$i]}/complete" \
        "{\"object_key\":\"${CLAVES[$i]}\"}" >/dev/null
    if [ "$(codigo)" = "202" ]; then
        ENCOLADOS=$((ENCOLADOS + 1))
    else
        marca "  la confirmacion de ${RECURSOS[$i]} devolvio $(codigo), se esperaba 202"
    fi
done

marca "encolados ${ENCOLADOS} de ${TOTAL}"
echo
echo "La cronologia la lleva la otra terminal:"
echo "  bash ./scripts/c6_prueba_autoescalado.sh docs/entrega3/evidencias/C6/cronologia.txt"
echo
echo "El curso ${CURSO} se queda para inspeccionarlo. La API no permite borrar"
echo "cursos, solo despublicarlos, asi que no hay nada que limpiar aqui; los"
echo "derivados HLS se van con el entorno."

[ "${ENCOLADOS}" -gt 0 ] || exit 1
exit 0
