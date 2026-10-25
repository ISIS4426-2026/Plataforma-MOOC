#!/usr/bin/env bash
# Genera los tres perfiles multimedia de prueba y comprueba que son lo que dicen.
#
# POR QUE EXISTE  (issue #197, A5)
#
# La revision de la Entrega 1 lo dijo sin rodeos: «los perfiles declarados de 2,
# 10 y 30 minutos fueron clips de 8, 20 y 40 segundos». Costo un tercio del
# criterio del escenario 2.
#
# El sembrador --cmd/seed-media-- ya declaraba las duraciones correctas. Lo que
# fallaba era que la campana de carga no usaba sus archivos: leia de scratch/ lo
# que hubiera, y lo que habia eran clips. Nada comprobaba la diferencia.
#
# Este script genera los tres archivos Y **verifica cada uno campo por campo**
# contra el perfil declarado. Un archivo que no coincide no se da por bueno: el
# script falla y dice en que campo.
#
# QUE GENERA
#
#   scratch/video_corto.mp4   120 s   que es lo que espera scripts/capacity_escenario2
#   scratch/video_medio.mp4   600 s
#   scratch/video_largo.mp4  1800 s
#
# Los tres a 1280x720, H.264 + AAC, con los mismos parametros de codificacion
# que usa cmd/seed-media. Que coincidan importa: si la campana midiera archivos
# codificados de otra forma, estaria midiendo otra cosa.
#
# Junto a cada archivo queda su `<nombre>.ffprobe.json` con la salida completa,
# que es la evidencia de que el perfil es el declarado.
#
# POR QUE NO SE SUBEN AL REPOSITORIO
#
# El largo ronda los 40 MB. Se generan, no se versionan; lo versionado es este
# script. `scratch/` esta en .gitignore. Las salidas de ffprobe si se conservan
# como evidencia, porque pesan unos kilobytes.
#
# SOBRE LA RESOLUCION
#
# Los tres se generan **nativos a 1280x720**, no reescalados desde algo menor.
# El enunciado prohibe inflar la resolucion del original, y con razon: un 720p
# agrandado desde 360p no produce el trabajo de transcodificacion que se quiere
# medir, solo lo aparenta. Ademas 720p es exactamente el techo que declara
# internal/transcode.DefaultLadder, asi que la escalera 360p+720p se aplica sin
# que nada se agrande.
#
# USO
#
#   bash ./scripts/generar_media_prueba.sh              # los tres
#   bash ./scripts/generar_media_prueba.sh corto        # solo uno
#   bash ./scripts/generar_media_prueba.sh --verificar  # no genera; solo comprueba
#
# Si no hay ffmpeg en el PATH usa Docker. El largo tarda unos minutos.

set -euo pipefail

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DESTINO="${RAIZ}/scratch"

# Las mismas tres duraciones que declara cmd/seed-media y que costea
# docs/entrega2/CONFIGURACION_Y_COSTOS.md. No son elecciones libres: la tabla de
# costes solo es comparable con lo que corre si ambas coinciden.
declare -A DURACION=([corto]=120 [medio]=600 [largo]=1800)
readonly ANCHO=1280 ALTO=720 FPS=25 AUDIO_HZ=44100

MODO="generar"
SELECCION=(corto medio largo)

case "${1:-}" in
    --verificar) MODO="verificar" ;;
    --help | -h)
        sed -n '2,52p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
        exit 0
        ;;
    "") ;;
    corto | medio | largo) SELECCION=("$1") ;;
    *)
        echo "perfil desconocido: $1  (corto, medio, largo, --verificar, --help)" >&2
        exit 2
        ;;
esac

# --- ffmpeg, del PATH o de Docker -------------------------------------------
#
# En Windows no suele estar instalado, y pedirlo seria un requisito mas para
# reproducir una evidencia. La imagen trae la misma version para todos, que
# ademas hace el resultado mas comparable entre maquinas.
readonly IMAGEN_FFMPEG="jrottenberg/ffmpeg:6.1-alpine"

if command -v ffmpeg >/dev/null 2>&1 && command -v ffprobe >/dev/null 2>&1; then
    ffmpeg_() { ffmpeg "$@"; }
    ffprobe_() { ffprobe "$@"; }
    echo "==> ffmpeg del sistema: $(ffmpeg -version 2>/dev/null | head -1)"
else
    command -v docker >/dev/null 2>&1 || {
        echo "ERROR: hacen falta ffmpeg y ffprobe, o Docker para usarlos desde una imagen." >&2
        exit 1
    }
    # La ruta de Windows es la que Docker entiende; la de Git Bash monta vacio.
    monta() { cygpath -w "$1" 2>/dev/null || echo "$1"; }
    ffmpeg_() {
        MSYS_NO_PATHCONV=1 docker run --rm -v "$(monta "${DESTINO}"):/salida" \
            --entrypoint ffmpeg "${IMAGEN_FFMPEG}" "$@"
    }
    ffprobe_() {
        MSYS_NO_PATHCONV=1 docker run --rm -v "$(monta "${DESTINO}"):/salida" \
            --entrypoint ffprobe "${IMAGEN_FFMPEG}" "$@"
    }
    echo "==> ffmpeg desde Docker (${IMAGEN_FFMPEG})"
fi

mkdir -p "${DESTINO}"

# --- Generacion -------------------------------------------------------------
#
# Mismos parametros que generateSyntheticVideo en cmd/seed-media. El patron de
# testsrc se mueve, asi que el codificador hace trabajo real: una imagen fija
# comprimiria a casi nada y daria tiempos de transcodificacion que no se
# parecerian a los de un video de verdad.
generar() {
    local perfil="$1" segundos="${DURACION[$1]}"
    echo "==> Generando ${perfil}: ${segundos}s a ${ANCHO}x${ALTO}"
    ffmpeg_ -hide_banner -loglevel error -y -nostdin \
        -f lavfi -i "testsrc=size=${ANCHO}x${ALTO}:rate=${FPS}:duration=${segundos}" \
        -f lavfi -i "sine=frequency=440:sample_rate=${AUDIO_HZ}:duration=${segundos}" \
        -c:v libx264 -preset veryfast -pix_fmt yuv420p \
        -c:a aac -shortest "/salida/video_${perfil}.mp4"
}

# --- Verificacion campo por campo -------------------------------------------
#
# Esto es lo que el criterio de aceptacion pide y lo que no existia. Comparar a
# ojo una tabla del plan con un archivo del disco es exactamente como se cuelan
# clips de 8 segundos declarados como 2 minutos.
verificar() {
    local perfil="$1" esperado="${DURACION[$1]}"
    local archivo="${DESTINO}/video_${perfil}.mp4"
    local json="${DESTINO}/video_${perfil}.ffprobe.json"

    [[ -f "${archivo}" ]] || {
        echo "    FALTA     video_${perfil}.mp4" >&2
        return 1
    }

    ffprobe_ -v error -print_format json -show_format -show_streams \
        "/salida/video_${perfil}.mp4" >"${json}"

    # Los valores se leen con el propio ffprobe, campo a campo. Sin jq ni
    # python: una dependencia mas es una razon mas para que esto no se ejecute,
    # y lo que no se ejecuta no verifica nada.
    local linea duracion bytes ancho alto codec_v codec_a hz
    linea="$(ffprobe_ -v error -show_entries format=duration,size         -of default=nw=1:nk=1 "/salida/video_${perfil}.mp4" | tr -d '')"
    duracion="$(sed -n 1p <<<"${linea}")"
    duracion="${duracion%%.*}"
    bytes="$(sed -n 2p <<<"${linea}")"

    linea="$(ffprobe_ -v error -select_streams v:0         -show_entries stream=width,height,codec_name         -of default=nw=1:nk=1 "/salida/video_${perfil}.mp4" | tr -d '')"
    codec_v="$(sed -n 1p <<<"${linea}")"
    ancho="$(sed -n 2p <<<"${linea}")"
    alto="$(sed -n 3p <<<"${linea}")"

    linea="$(ffprobe_ -v error -select_streams a:0         -show_entries stream=codec_name,sample_rate         -of default=nw=1:nk=1 "/salida/video_${perfil}.mp4" | tr -d '')"
    codec_a="$(sed -n 1p <<<"${linea}")"
    hz="$(sed -n 2p <<<"${linea}")"

    local fallos=0
    comprobar() {
        local campo="$1" obtenido="$2" quiero="$3"
        if [[ "${obtenido}" != "${quiero}" ]]; then
            echo "    ${perfil}: ${campo} = ${obtenido}, se declaro ${quiero}" >&2
            fallos=$((fallos + 1))
        fi
    }

    # Un segundo de margen en la duracion: el ultimo fotograma y la cola de
    # audio no caen siempre en el mismo instante. Mas margen que eso volveria a
    # admitir un clip disfrazado.
    if [[ "${duracion}" -lt $((esperado - 1)) || "${duracion}" -gt $((esperado + 1)) ]]; then
        echo "    ${perfil}: duracion = ${duracion}s, se declaro ${esperado}s" >&2
        fallos=$((fallos + 1))
    fi
    comprobar "ancho" "${ancho}" "${ANCHO}"
    comprobar "alto" "${alto}" "${ALTO}"
    comprobar "codec de video" "${codec_v}" "h264"
    comprobar "codec de audio" "${codec_a}" "aac"
    comprobar "frecuencia de audio" "${hz}" "${AUDIO_HZ}"

    if [[ ${fallos} -gt 0 ]]; then
        return 1
    fi
    printf '    ok  %-6s %4ss  %sx%s  %s/%s  %s MiB\n' \
        "${perfil}" "${duracion}" "${ancho}" "${alto}" "${codec_v}" "${codec_a}" \
        "$((bytes / 1024 / 1024))"
}

# --- Ejecucion --------------------------------------------------------------
if [[ "${MODO}" == "generar" ]]; then
    for perfil in "${SELECCION[@]}"; do
        generar "${perfil}"
    done
    echo
fi

echo "==> Verificando contra el perfil declarado"
malos=0
for perfil in "${SELECCION[@]}"; do
    verificar "${perfil}" || malos=$((malos + 1))
done

echo
if [[ ${malos} -gt 0 ]]; then
    echo "ERROR: ${malos} perfil(es) no coinciden con lo declarado." >&2
    echo "       No se usan en una campana: un archivo que no es lo que dice" >&2
    echo "       invalida la medicion entera, y ya paso una vez." >&2
    exit 1
fi
echo "==> Los ${#SELECCION[@]} perfil(es) coinciden campo por campo."
echo "    Salidas de ffprobe en ${DESTINO}/video_*.ffprobe.json"
