#!/usr/bin/env bash
# Prueba del autohealing del grupo de la API (C1, #204) y del arranque
# desatendido (B3, #202), con la cronologia que acompana a las capturas.
#
# QUE HACE
#
# Borra a mano la instancia que tenga el grupo y cronometra lo que pasa hasta que
# la nueva queda sana, imprimiendo una linea por sondeo con su T+ y su hora UTC.
# Esa salida es la evidencia escrita; las capturas de pantalla de la consola se
# toman en paralelo, mirando el mismo intervalo.
#
# POR QUE IDENTIFICA POR ID Y NO POR NOMBRE
#
# **Un grupo administrado reutiliza el nombre de la instancia.** La primera
# version de esta prueba esperaba un nombre distinto para darla por nueva, asi
# que siguio sondeando 38 minutos sobre una instancia que llevaba ese tiempo sana
# y acabo imprimiendo «no quedo sana en el plazo». Los datos eran correctos; el
# veredicto, no.
#
# Lo que distingue una instancia nueva es su **id numerico** y su
# `creationTimestamp`. Ver la nota tecnica 36.
#
# USO
#
#   bash ./scripts/c1_prueba_autohealing.sh                 # a la salida estandar
#   bash ./scripts/c1_prueba_autohealing.sh <fichero>       # y tambien a un fichero
set -uo pipefail

PROYECTO="${PROYECTO:-plataforma-mooc-entrega2}"
ZONA="${ZONA:-us-east1-b}"
GRUPO="${GRUPO:-mooc-api-group}"
PLAZO_SONDEOS="${PLAZO_SONDEOS:-80}" # 80 x 15 s = 20 min
SALIDA="${1:-}"

emitir() {
    if [ -n "${SALIDA}" ]; then echo "$*" | tee -a "${SALIDA}"; else echo "$*"; fi
}

[ -z "${SALIDA}" ] || : >"${SALIDA}"

T0=$(date +%s)
marca() { emitir "$(printf '[%s] T+%-6s %s' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(($(date +%s) - T0))s" "$*")"; }

# `value(name)`, NO `value(instance)`.
#
# `instance` suena a lo que uno quiere y devuelve **la zona**: `us-east1-b`. La
# primera version de este guion lo uso, intento borrar una instancia llamada
# `us-east1-b` --que no existe--, el borrado fallo, y como el guion no miraba el
# codigo de salida declaro «el grupo se recupero sin intervencion» sin haber
# tocado nada. Un falso positivo, que es el lado peligroso del error (nota 36).
instancia_del_grupo() {
    gcloud compute instance-groups managed list-instances "${GRUPO}" \
        --zone="${ZONA}" --project="${PROYECTO}" \
        --format="value(name)" 2>/dev/null | tr -d '\r' | head -1
}

# El id y la hora de creacion, que es lo que de verdad distingue una instancia.
identidad() {
    gcloud compute instances describe "$1" --zone="${ZONA}" --project="${PROYECTO}" \
        --format="value(id,creationTimestamp)" 2>/dev/null | tr -d '\r'
}

estado_del_grupo() {
    gcloud compute instance-groups managed list-instances "${GRUPO}" \
        --zone="${ZONA}" --project="${PROYECTO}" \
        --format="value(name,instanceStatus,currentAction,instanceHealth[0].detailedHealthState)" \
        2>/dev/null | tr -d '\r'
}

emitir "Prueba de autohealing del grupo ${GRUPO} (C1 #204, B3 #202)"
emitir "=============================================================="
emitir "Proyecto : ${PROYECTO}   Zona: ${ZONA}"
emitir ""

VIEJA="$(instancia_del_grupo)"
[ -n "${VIEJA}" ] || { emitir "ERROR: el grupo no tiene ninguna instancia. Hace falta lb_enabled/api_autoscaling_enabled."; exit 1; }

# El nombre tiene que parecer uno de instancia del grupo. Sin esta comprobacion,
# un campo mal leido pasa como nombre y lo siguiente que falla es el borrado.
case "${VIEJA}" in
    mooc-api-*) ;;
    *) emitir "ERROR: '${VIEJA}' no parece una instancia de ${GRUPO}. Revisa el formato de gcloud."; exit 1 ;;
esac

ID_VIEJO="$(identidad "${VIEJA}" | awk '{print $1}')"
# Sin id no se puede distinguir la instancia nueva de la vieja, y seguir seria
# garantizar un veredicto sin fundamento.
[ -n "${ID_VIEJO}" ] || { emitir "ERROR: no se pudo leer el id de ${VIEJA}."; exit 1; }
marca "instancia actual: ${VIEJA}  id=${ID_VIEJO}"
marca ">>> TOMA LA CAPTURA DE 'ANTES' AHORA, con el grupo en NONE / HEALTHY <<<"
emitir ""
emitir "$(estado_del_grupo | sed 's/^/    /')"
emitir ""

marca "borrando a mano ${VIEJA}"
# El codigo de salida se mira. Ignorarlo es lo que convirtio un borrado fallido
# en un «el grupo se recupero».
if ! salida_borrado="$(gcloud compute instances delete "${VIEJA}" \
    --zone="${ZONA}" --project="${PROYECTO}" --quiet 2>&1)"; then
    emitir "ERROR: el borrado de ${VIEJA} fallo:"
    emitir "$(printf '%s' "${salida_borrado}" | tr -d '\r' | tail -n 5 | sed 's/^/    /')"
    exit 1
fi
marca "orden de borrado aceptada"
marca ">>> LA CONSOLA YA MUESTRA LA RECREACION. CAPTURA AHORA Y EN LOS PROXIMOS 2 MIN <<<"
emitir ""

VISTA_NUEVA=""
for _ in $(seq 1 "${PLAZO_SONDEOS}"); do
    LINEAS="$(estado_del_grupo)"
    while IFS=$'\t' read -r n st ac hs; do
        [ -n "${n}" ] || continue
        marca "$(printf '%-14s %-12s %-12s %s' "${n}" "${st:-?}" "${ac:-?}" "${hs:-?}")"

        # Nueva se decide por id, no por nombre.
        ident="$(identidad "${n}")"
        id_actual="$(printf '%s' "${ident}" | awk '{print $1}')"
        [ -n "${id_actual}" ] || continue

        # `ID_VIEJO` no puede estar vacio aqui --se comprobo arriba-- asi que
        # esta comparacion significa algo.
        if [ -n "${id_actual}" ] && [ "${id_actual}" != "${ID_VIEJO}" ] && [ -z "${VISTA_NUEVA}" ]; then
            VISTA_NUEVA="${id_actual}"
            marca ">>> el grupo creo una instancia NUEVA: id=${id_actual} creada=$(printf '%s' "${ident}" | awk '{print $2}') <<<"
        fi

        if [ -n "${VISTA_NUEVA}" ] && [ "${hs}" = "HEALTHY" ]; then
            marca ">>> SANA. El grupo se recupero sin intervencion. <<<"
            emitir ""
            emitir "Para la evidencia de B3, el arranque de esta instancia:"
            emitir "  gcloud compute instances get-serial-port-output ${n} \\"
            emitir "    --zone=${ZONA} --project=${PROYECTO} | grep -E 'B3:|B3/migrar:'"
            exit 0
        fi
    done <<<"${LINEAS}"
    sleep 15
done

marca "ERROR: no quedo sana en el plazo de $((PLAZO_SONDEOS * 15)) s"
exit 1
