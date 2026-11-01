#!/usr/bin/env bash
# Retira las reglas de cortafuegos sobrantes de la Entrega 2 (F1, issue #219), con confirmacion.
#
# BORRA. Por eso es un guion aparte de f1_superficie_red.sh (que es de solo lectura) y pregunta regla por
# regla. Nada se borra sin un "s" escrito por quien lo ejecuta.
#
# QUE CUENTA COMO SOBRANTE
#
#   - cualquier regla de cortafuegos del proyecto que NO sea una de las cinco que declara el codigo de la
#     Entrega 3 (network.tf, queue.tf, worker_group.tf), en cualquier red
#   - la red `default`, si existe y no tiene ninguna instancia (hallazgo 2 de la Entrega 2). Solo se
#     ofrece despues de vaciarla de reglas: GCE no borra una red que tenga reglas
#
# Las cinco del codigo NO se tocan nunca desde aqui, aunque esten distintas de lo declarado: esas las
# gobierna Terraform y se corrigen con un `terraform apply` desde main (nota 1). Borrarlas a mano solo
# haria que el siguiente apply las recreara, y entre tanto la API o la cola se quedarian sin entrada.
#
# USO
#
#   bash ./scripts/f1_retirar_reglas_sobrantes.sh
#
# Deja el registro en docs/entrega3/evidencias/F1/retiro_reglas.txt: que habia, que se borro, quien
# decidio (solo "se confirmo en la terminal"; sin correos) y el resultado. Despues hay que volver a correr
# f1_superficie_red.sh, que es la comprobacion.
set -uo pipefail

PROYECTO="${PROYECTO:-plataforma-mooc-entrega2}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
REG="${ROOT}/docs/entrega3/evidencias/F1/retiro_reglas.txt"
ESPERADAS=" mooc-allow-ssh-iap mooc-allow-lb-health mooc-allow-worker-health mooc-allow-queue mooc-allow-internal-icmp "

mkdir -p "$(dirname "${REG}")"
ahora() { date -u +%Y-%m-%dT%H:%M:%SZ; }
log() { echo "$*" | tee -a "${REG}"; }

# Si ya habia un registro (otra pasada), se anade; no se pisa.
{
    echo
    echo "# Retiro de reglas sobrantes - $(ahora) - scripts/f1_retirar_reglas_sobrantes.sh"
    echo "# Proyecto: ${PROYECTO}"
} >>"${REG}"

mapfile -t REGLAS < <(gcloud compute firewall-rules list --project="${PROYECTO}" \
    --format="csv[no-heading,separator='|'](name,network.basename(),direction,sourceRanges.list(),allowed[].map().firewall_rule().list(),targetTags.list())" \
    2>/dev/null | tr -d '\r')
[ "${#REGLAS[@]}" -gt 0 ] || { log "ERROR: no se pudieron listar las reglas (sesion de gcloud?)"; exit 2; }

log ""
log "== Reglas encontradas =="
SOBRANTES=()
for r in "${REGLAS[@]}"; do
    IFS='|' read -r n red dir orig perm dest <<<"${r}"
    if [[ "${ESPERADAS}" == *" ${n} "* ]]; then
        log "   se queda   ${n} (${red}) ${dir} ${perm} desde ${orig:--} hacia ${dest:-toda la red}"
    else
        log "   SOBRANTE   ${n} (${red}) ${dir} ${perm} desde ${orig:--} hacia ${dest:-toda la red}"
        SOBRANTES+=("${n}|${red}")
    fi
done

BORRADAS=0
if [ "${#SOBRANTES[@]}" -eq 0 ]; then
    log ""
    log "No hay reglas sobrantes."
else
    log ""
    log "== ${#SOBRANTES[@]} regla(s) sobrante(s). Se pregunta una por una =="
    for s in "${SOBRANTES[@]}"; do
        n="${s%%|*}"; red="${s##*|}"
        printf '¿Borrar la regla %s (red %s)? Escribe s para borrarla [s/N]: ' "${n}" "${red}"
        read -r resp </dev/tty
        if [ "${resp}" = "s" ] || [ "${resp}" = "S" ]; then
            if gcloud compute firewall-rules delete "${n}" --project="${PROYECTO}" --quiet >/dev/null 2>&1; then
                log "   $(ahora) BORRADA   ${n} (confirmado en la terminal)"
                BORRADAS=$((BORRADAS + 1))
            else
                log "   $(ahora) ERROR     ${n}: gcloud no pudo borrarla"
            fi
        else
            log "   $(ahora) se mantiene ${n} (no confirmado)"
        fi
    done
fi

# --- la red default ---------------------------------------------------------------------------------
if gcloud compute networks describe default --project="${PROYECTO}" --format="value(name)" >/dev/null 2>&1; then
    EN_USO="$(gcloud compute instances list --project="${PROYECTO}" \
        --filter="networkInterfaces.network~/networks/default$" --format="value(name)" 2>/dev/null | tr -d '\r')"
    QUEDAN="$(gcloud compute firewall-rules list --project="${PROYECTO}" \
        --filter="network~/networks/default$" --format="value(name)" 2>/dev/null | tr -d '\r')"
    log ""
    log "== Red default =="
    if [ -n "${EN_USO}" ]; then
        log "   se mantiene: tiene instancias ($(echo ${EN_USO}))"
    elif [ -n "${QUEDAN}" ]; then
        log "   se mantiene: todavia tiene reglas ($(echo ${QUEDAN})); GCE no borra una red con reglas"
    else
        printf '¿Borrar la red default (sin instancias ni reglas)? Escribe s para borrarla [s/N]: '
        read -r resp </dev/tty
        if [ "${resp}" = "s" ] || [ "${resp}" = "S" ]; then
            if gcloud compute networks delete default --project="${PROYECTO}" --quiet >/dev/null 2>&1; then
                log "   $(ahora) BORRADA   red default (confirmado en la terminal)"
            else
                log "   $(ahora) ERROR     red default: gcloud no pudo borrarla (subredes en uso por otro recurso?)"
            fi
        else
            log "   $(ahora) se mantiene la red default (no confirmado)"
        fi
    fi
fi

log ""
log "Reglas borradas en esta pasada: ${BORRADAS}. Comprobacion: bash ./scripts/f1_superficie_red.sh"
log "Este archivo no contiene credenciales, llaves ni secretos."
