#!/usr/bin/env bash
# Devuelve el entorno a su linea base entre corridas del escenario 2 (A7, #199).
#
# POR QUE EXISTE
#
# Una campana interrumpida deja tres cosas: trabajos en la cola que siguen
# transcodificando, recursos en pending/processing en la base y objetos en los
# buckets. La corrida siguiente mediria una cola que no es suya y su
# verificacion terminal contaria trabajos ajenos.
#
# NUNCA PURGAR LA COLA CON FLUSHDB
#
# Esto costo tres corridas. `FLUSHDB` borra el conjunto `asynq:queues`, y el
# inspector de asynq responde
#
#     NOT_FOUND: queue "default" does not exist
#
# a cualquier cola que no este en ese conjunto. El cliente de asynq de la API
# **solo registra la cola una vez por proceso**, asi que un contenedor que
# llevaba arriba desde antes de la purga no la vuelve a anadir nunca.
#
# El efecto es el peor posible: la plataforma sigue funcionando --el worker
# consume porque tiene el nombre configurado-- y lo unico que muere es la
# observabilidad. La campana registraba "pico de cola: 0 tareas" habiendo
# confirmado 12 subidas y escribia "la cola se ha vaciado totalmente" con 13
# pendientes.
#
# Asi que aqui se borran las estructuras de la cola una por una y se CONSERVA
# el registro.
#
# USO
#
#   bash ./scripts/a7_purgar.sh          # cola, base y buckets
#   bash ./scripts/a7_purgar.sh --estado # solo informa, no cambia nada
#
# Lo que NO toca: la semilla sintetica (fechas de 2026-09-10) ni los 123 MB que
# la Entrega 2 dejo en los buckets.

set -uo pipefail

PROYECTO="${PROYECTO:-plataforma-mooc-entrega2}"
ZONA="${ZONA:-us-east1-b}"
BUCKET_MEDIA="${BUCKET_MEDIA:-plataforma-mooc-entrega2-media}"
BUCKET_HLS="${BUCKET_HLS:-plataforma-mooc-entrega2-hls}"
# Todo lo creado despues de esta fecha es de una campana; la semilla sintetica
# tiene fechas de septiembre. Asi no hace falta acertar con los UUID.
CORTE="${CORTE:-2026-10-01}"

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IP_BASE="${IP_BASE:-$(terraform -chdir="${RAIZ}/infra/terraform" output -raw db_private_ip 2>/dev/null)}"
[ -n "${IP_BASE}" ] || {
    echo "No se pudo determinar la IP privada de la base. Declara IP_BASE=..." >&2
    exit 1
}

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

titulo() { printf '\n\033[1m== %s\033[0m\n' "$*"; }
ssh_() { gcloud compute ssh "$1" --zone="${ZONA}" --tunnel-through-iap --project="${PROYECTO}" --quiet --command "$2"; }
scp_() { gcloud compute scp "$1" "$2" --zone="${ZONA}" --tunnel-through-iap --project="${PROYECTO}" --quiet >/dev/null 2>&1; }

SOLO_INFORMAR=0
[ "${1:-}" = "--estado" ] && SOLO_INFORMAR=1

# --- 1. Cola -----------------------------------------------------------------
titulo "Cola de asynq"
cat >"${TMP}/cola.sh" <<COLA
#!/usr/bin/env bash
set -u
Q='asynq:{default}'
r() { sudo docker exec mooc-queue redis-cli -n 0 "\$@"; }
informar() {
    printf '  pending %s | active %s | registradas: ' "\$(r llen "\${Q}:pending")" "\$(r llen "\${Q}:active")"
    r smembers asynq:queues
}
informar
[ "${SOLO_INFORMAR}" = "1" ] && exit 0

echo "  parando el contenedor del worker (no el servicio: incluye al propio redis)"
sudo docker stop mooc-worker >/dev/null
for k in pending active scheduled retry archived completed lease paused; do r del "\${Q}:\${k}" >/dev/null; done
n=0
while read -r k; do
    [ -z "\${k}" ] && continue
    r del "\${k}" >/dev/null
    n=\$((n + 1))
done < <(r --scan --pattern "\${Q}:t:*" | tr -d '\r')
echo "  \${n} clave(s) de tarea borradas"
# El registro se conserva; si algo lo dejo vacio, se restaura.
r sadd asynq:queues default >/dev/null
sudo systemctl restart mooc-worker.service && sleep 8
informar
COLA
scp_ "${TMP}/cola.sh" "mooc-worker-server:/tmp/a7_cola.sh"
ssh_ mooc-worker-server "bash /tmp/a7_cola.sh"

# --- 2. Base -----------------------------------------------------------------
titulo "Base de datos"
cat >"${TMP}/base.sh" <<BASE
#!/usr/bin/env bash
set -u
export PGPASSWORD=\$(gcloud secrets versions access latest --secret=db-password --project=${PROYECTO} | tr -d '\r\n')
q() { psql -h ${IP_BASE} -U moocuser -d moocdb -t -A -F'|' -c "\$1"; }
q "select distinct stable_id from resources where created_at > '${CORTE}'" > /tmp/a7_sid.txt
echo "  recursos de campana: \$(grep -c . /tmp/a7_sid.txt)"
if [ "${SOLO_INFORMAR}" != "1" ]; then
    q "delete from resources where created_at > '${CORTE}'"
    q "delete from units     where created_at > '${CORTE}'"
    q "delete from modules   where created_at > '${CORTE}'"
    q "delete from courses   where created_at > '${CORTE}'"
fi
echo "  estado:"
q "select coalesce(processing_status::text,'(nulo)'), count(*) from resources group by 1 order by 2 desc" | sed 's/^/    /'
BASE
scp_ "${TMP}/base.sh" "mooc-web-server:/tmp/a7_base.sh"
ssh_ mooc-web-server "bash /tmp/a7_base.sh"

# --- 3. Buckets --------------------------------------------------------------
#
# Desde aqui y no desde la VM: la cuenta de servicio del servidor web tiene
# prohibido borrar, asi que un `rm` lanzado alli falla en silencio.
titulo "Buckets"
scp_ "mooc-web-server:/tmp/a7_sid.txt" "${TMP}/sid.txt"
n=$(grep -c . "${TMP}/sid.txt" 2>/dev/null || echo 0)
if [ "${SOLO_INFORMAR}" = "1" ] || [ "${n}" = "0" ]; then
    echo "  ${n} prefijo(s) de campana; no se borra nada"
else
    for sid in $(tr -d '\r' <"${TMP}/sid.txt"); do
        gcloud storage rm -r \
            "gs://${BUCKET_MEDIA}/originals/${sid}" \
            "gs://${BUCKET_HLS}/hls/${sid}" 2>/dev/null
    done
    echo "  ${n} prefijo(s) borrados de los dos buckets"
fi
gcloud storage du -s "gs://${BUCKET_MEDIA}/originals/" "gs://${BUCKET_HLS}/" 2>/dev/null | sed 's/^/  /'
echo
echo "  (lo que queda es de la Entrega 2: los buckets no se vaciaron al destruir el entorno)"
