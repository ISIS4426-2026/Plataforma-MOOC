#!/usr/bin/env bash
# Tunel IAP duradero hacia la cola y la base, para medir el escenario 2 (A7, #199).
#
# POR QUE EXISTE
#
# La campana del escenario 2 necesita el inspector de la cola (Redis) y, para la
# verificacion terminal, la base. El cortafuegos solo abre el 6379 dentro de la
# subred y Cloud SQL no tiene IP publica, asi que desde el portatil se llega por
# un reenvio de puertos sobre el tunel SSH de IAP, que usa el 22 y no abre nada
# nuevo (F1 sigue pudiendo demostrar que no hay puertos de datos expuestos).
#
# EL PROBLEMA QUE RESUELVE
#
# `gcloud compute ssh -- -N -L ...` **no es duradero**. En Windows abre una
# consola de plink: si se cierra esa ventana --o si la sesion se corta-- el
# reenvio termina con **codigo 0**, como un final normal. En una corrida de tres
# horas eso deja al inspector ciego sin que nada falle a la vista.
#
# Paso de verdad: en el segundo intento del escenario 2 el nivel 4 registro
# "pico de cola: 0 tareas" habiendo confirmado 12 subidas, y el drenaje escribio
# "la cola se ha vaciado totalmente" con 13 trabajos pendientes en la base. La
# campana ya no se come ese error --lo cuenta y declara el drenaje no
# observable--, pero lo que hay que evitar es el hueco.
#
# QUE HACE
#
# Levanta el tunel y lo vuelve a levantar cada vez que cae, anotando cada
# reinicio con su hora. Ese registro es parte de la evidencia: dice si la
# ventana medida tuvo huecos de visibilidad y de cuanto.
#
# La campana tolera hasta 5 lecturas seguidas fallidas (unos 4 s), asi que un
# reinicio rapido no invalida nada; lo que invalida es un hueco largo.
#
# USO
#
#   bash ./scripts/a7_tunel.sh                    # en primer plano, Ctrl-C para parar
#   bash ./scripts/a7_tunel.sh --registro <ruta>  # ademas escribe el registro ahi
#
# Deja escuchando en localhost: 6379 (cola) y 5432 (base).

set -uo pipefail

PROYECTO="${PROYECTO:-plataforma-mooc-entrega2}"
ZONA="${ZONA:-us-east1-b}"
VM="${VM:-mooc-worker-server}"
PUERTO_COLA="${PUERTO_COLA:-6379}"
PUERTO_BASE="${PUERTO_BASE:-5432}"

# La IP privada de la base se lee de Terraform si no se declara: escribirla a
# mano es como se acaba midiendo contra la base de otro entorno.
RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IP_BASE="${IP_BASE:-$(terraform -chdir="${RAIZ}/infra/terraform" output -raw db_private_ip 2>/dev/null)}"
[ -n "${IP_BASE}" ] || {
    echo "No se pudo determinar la IP privada de la base. Declara IP_BASE=..." >&2
    exit 1
}

REGISTRO=""
if [ "${1:-}" = "--registro" ]; then
    REGISTRO="${2:?ruta del registro}"
    mkdir -p "$(dirname "${REGISTRO}")"
fi

anotar() {
    local linea="[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"
    echo "${linea}"
    [ -n "${REGISTRO}" ] && echo "${linea}" >>"${REGISTRO}"
}

# Si el tunel no se puede establecer, no tiene sentido reintentar sin parar:
# seria un bucle de fallos de autenticacion girando contra IAP.
MAX_FALLOS_SEGUIDOS=5
fallos=0
reinicios=0

anotar "tunel arriba: ${PUERTO_COLA} -> ${VM}:6379 | ${PUERTO_BASE} -> ${IP_BASE}:5432"

trap 'anotar "parado a mano tras ${reinicios} reinicio(s)"; exit 0' INT TERM

while :; do
    inicio=$(date +%s)

    gcloud compute ssh "${VM}" \
        --zone="${ZONA}" --tunnel-through-iap --project="${PROYECTO}" --quiet \
        -- -N \
        -L "${PUERTO_COLA}:localhost:6379" \
        -L "${PUERTO_BASE}:${IP_BASE}:5432" \
        >/dev/null 2>&1

    duro=$(($(date +%s) - inicio))

    # Un tunel que duro algo es una caida; uno que no llego a levantar es un
    # fallo de establecimiento, y esos no se reintentan indefinidamente.
    if [ "${duro}" -lt 10 ]; then
        fallos=$((fallos + 1))
        anotar "no se pudo establecer (intento ${fallos}/${MAX_FALLOS_SEGUIDOS})"
        if [ "${fallos}" -ge "${MAX_FALLOS_SEGUIDOS}" ]; then
            anotar "ERROR: ${MAX_FALLOS_SEGUIDOS} fallos seguidos al establecer el tunel. Se abandona."
            exit 1
        fi
        sleep 5
        continue
    fi

    fallos=0
    reinicios=$((reinicios + 1))
    anotar "el tunel cayo tras ${duro}s (salida 0: ventana cerrada o sesion cortada); reinicio ${reinicios}"
    sleep 1
done
