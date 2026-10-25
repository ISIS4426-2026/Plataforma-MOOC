#!/usr/bin/env bash
# Comprueba que los issues que capturan evidencia efimera la tengan de verdad.
#
# POR QUE HACE FALTA
#
# Dieciocho issues de la Entrega 3 llevan un criterio de aceptacion que dice
# que la captura «bloquea el cierre». El motivo es que su evidencia no se puede
# rehacer: I6 destruye el entorno, las metricas de Cloud Monitoring caducan y
# relanzar un ciclo de autoscaling cuesta dinero.
#
# Pero un criterio es una casilla, y quien cierra el issue es quien lo trabajo.
# No hay nadie entre el y el cierre. Este script es ese alguien.
#
# QUE COMPRUEBA
#
#   1. Que cada issue marcado en la columna «Captura» de
#      docs/entrega3/evidencias/README.md tenga al menos un archivo en su
#      directorio capturas/ que no sea el .gitkeep.
#   2. Que ningun archivo de capturas/ pase del limite de tamano. Un video
#      pesado en el repositorio es el otro error facil de cometer: lo que no
#      quepa va a la plataforma de video y se enlaza.
#
# La lista de issues NO esta aqui. Sale de la tabla del README de evidencias,
# que a su vez se genera desde el backlog. Una lista mas que mantener es una
# lista mas que se queda vieja.
#
# USO
#
#   bash ./scripts/check_evidencias.sh            # falla si falta algo
#   bash ./scripts/check_evidencias.sh --informe  # solo informa, siempre sale 0
#
# CUANDO SE EJECUTA
#
# No en la CI: fallaria desde el primer dia hasta que los dieciocho terminen.
# Es criterio de aceptacion de I3 (README) y de I5 (tag): el tag `entrega-3` no
# se crea con capturas faltantes. Si falla ahi todavia hay margen para recrear
# el entorno --caro, pero posible--; si se descubre en la sustentacion, no.

set -euo pipefail

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INDICE="${RAIZ}/docs/entrega3/evidencias/README.md"
EVIDENCIAS="${RAIZ}/docs/entrega3/evidencias"

# 10 MiB. Por encima de esto, al repositorio no.
readonly MAX_BYTES=$((10 * 1024 * 1024))

MODO="exigir"
case "${1:-}" in
    --informe) MODO="informe" ;;
    --help | -h)
        sed -n '2,40p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
        exit 0
        ;;
    "") ;;
    *)
        echo "opcion desconocida: $1  (--informe, --help)" >&2
        exit 2
        ;;
esac

[[ -f "${INDICE}" ]] || {
    echo "ERROR: no esta ${INDICE}." >&2
    echo "       Lo genera A1; sin el no hay lista de que capturar." >&2
    exit 2
}

# --- Quien debe capturar, segun la tabla ------------------------------------
#
# Las filas son  | [`C5`](./C5/) | ... | @quien | que capturar |  y la ultima
# columna es «—» cuando no aplica. Se leen la clave y esa columna.
claves=()
declare -A que_capturar=()

while IFS= read -r fila; do
    clave="$(sed -n 's/^| \[`\([A-Z][0-9.]*\)`\].*/\1/p' <<<"${fila}")"
    [[ -n "${clave}" ]] || continue

    # La ultima celda, sin los adornos de la tabla ni el negrita.
    captura="$(awk -F'|' '{print $(NF-1)}' <<<"${fila}" | sed 's/^ *//; s/ *$//; s/\*\*//g')"
    [[ "${captura}" != "—" && -n "${captura}" ]] || continue

    claves+=("${clave}")
    que_capturar["${clave}"]="${captura}"
done <"${INDICE}"

if [[ ${#claves[@]} -eq 0 ]]; then
    echo "ERROR: la tabla de ${INDICE} no marco ningun issue con captura." >&2
    echo "       O el formato cambio, o se perdio la columna. Revisalo: un" >&2
    echo "       verificador que no verifica nada es peor que no tenerlo." >&2
    exit 2
fi

# --- Comprobacion -----------------------------------------------------------
faltan=0
pesados=0

echo "==> ${#claves[@]} issue(s) con captura obligatoria"
echo

for clave in "${claves[@]}"; do
    dir="${EVIDENCIAS}/${clave}/capturas"

    archivos=0
    if [[ -d "${dir}" ]]; then
        while IFS= read -r -d '' archivo; do
            archivos=$((archivos + 1))
            bytes="$(stat -c %s "${archivo}" 2>/dev/null || stat -f %z "${archivo}")"
            if [[ "${bytes}" -gt "${MAX_BYTES}" ]]; then
                echo "    PESADO   ${clave}/capturas/$(basename "${archivo}")" \
                    "— $((bytes / 1024 / 1024)) MiB, por encima de 10 MiB"
                pesados=$((pesados + 1))
            fi
        done < <(find "${dir}" -type f ! -name '.gitkeep' -print0)
    fi

    if [[ ${archivos} -eq 0 ]]; then
        printf '    FALTA    %-4s %s\n' "${clave}" "${que_capturar[${clave}]}"
        faltan=$((faltan + 1))
    else
        printf '    ok       %-4s %d archivo(s)\n' "${clave}" "${archivos}"
    fi
done

echo
if [[ ${faltan} -eq 0 && ${pesados} -eq 0 ]]; then
    echo "==> Todas las capturas obligatorias estan, y ninguna pasa de 10 MiB."
    exit 0
fi

[[ ${faltan} -eq 0 ]] || echo "==> Faltan ${faltan} captura(s)."
[[ ${pesados} -eq 0 ]] || echo "==> ${pesados} archivo(s) por encima del limite."

if [[ "${MODO}" == "informe" ]]; then
    echo "    (modo informe: no se devuelve error)"
    exit 0
fi

echo >&2
echo "Lo que falta no se puede rehacer despues: I6 destruye el entorno, las" >&2
echo "metricas de Cloud Monitoring caducan y relanzar un ciclo de autoscaling" >&2
echo "cuesta dinero. Captura antes de cerrar, no despues." >&2
exit 1
