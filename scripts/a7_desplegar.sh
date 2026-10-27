#!/usr/bin/env bash
# Cambia entre el codigo del tag y el corregido sobre las MISMAS dos VMs (A7, #199).
#
# POR QUE EXISTE
#
# A7 necesita dos corridas del escenario 1 sobre la misma infraestructura: una
# con el codigo del tag entrega-2 y otra con el de main. La diferencia entre las
# dos es el coste por peticion de las correcciones de la Entrega 1, y es el
# numero que G3 resta al comparar contra la arquitectura elastica.
#
# CAMBIAR IMAGE_TAG NO BASTA
#
# La unidad de systemd no apunta a un checkout del repositorio:
#
#   WorkingDirectory=/home/<usuario>/mooc
#   ExecStartPre=/bin/bash /home/<usuario>/mooc/prepare_env.sh
#   ExecStart=/usr/bin/docker compose -f /home/<usuario>/mooc/docker-compose.prod.yml up -d
#
# `prepare_env.sh` y el compose son copias congeladas de la epoca del tag. La
# imagen cambia; esos dos archivos no. Y main anade en prepare_web_env.sh tres
# variables que importan: REDIS_SESSION_DB=1, INSTANCE_ID y
# SESSION_COOKIE_SAMESITE.
#
# Sin copiarlas, **las sesiones se quedan en la base logica 0 junto a la cola de
# asynq** --el riesgo de la nota tecnica 23-- y la corrida 2 no mediria la
# configuracion con la que correra la Entrega 3.
#
# USO
#
#   bash ./scripts/a7_desplegar.sh corregido   # imagen de main + su configuracion
#   bash ./scripts/a7_desplegar.sh tag         # vuelve al codigo del tag
#   bash ./scripts/a7_desplegar.sh --estado    # que hay desplegado ahora
#
# La primera vez que despliega `corregido` guarda los archivos del tag como
# `.tag-respaldo`; `tag` los restaura. Por eso se puede ir y volver.

set -uo pipefail

PROYECTO="${PROYECTO:-plataforma-mooc-entrega2}"
ZONA="${ZONA:-us-east1-b}"
REGION="${REGION:-us-east1}"
REGISTRO="${REGION}-docker.pkg.dev/${PROYECTO}/mooc"
TAG_BASE="${TAG_BASE:-entrega-2}"
URL_PUBLICA="${URL_PUBLICA:-https://34.24.52.111.sslip.io}"

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

ssh_() { gcloud compute ssh "$1" --zone="${ZONA}" --tunnel-through-iap --project="${PROYECTO}" --quiet "${@:2}"; }
scp_() { gcloud compute scp "$1" "$2" --zone="${ZONA}" --tunnel-through-iap --project="${PROYECTO}" --quiet; }

titulo() { printf '\n\033[1m== %s\033[0m\n' "$*"; }

# El home del servicio se lee de la unidad: no se adivina ni se escribe aqui,
# porque es el home de quien monto las VMs y puede cambiar.
home_servicio() { # $1 = vm, $2 = unidad
    ssh_ "$1" --command "systemctl cat $2 2>/dev/null | sed -n 's|^WorkingDirectory=||p' | head -1" 2>/dev/null | tr -d '\r\n'
}

estado() {
    titulo "Que hay desplegado"
    for vm in mooc-web-server mooc-worker-server; do
        conf="/etc/mooc/web.conf"
        [ "${vm}" = "mooc-worker-server" ] && conf="/etc/mooc/worker.conf"
        printf '  %s\n' "${vm}"
        ssh_ "${vm}" --command "
            printf '    IMAGE_TAG   %s\n' \"\$(sudo sed -n 's|^IMAGE_TAG=||p' ${conf})\"
            printf '    contenedores %s\n' \"\$(sudo docker ps --format '{{.Names}}' | tr '\n' ' ')\"
            for f in prepare_env.sh docker-compose.prod.yml docker-compose.worker.yml; do
                r=\"\$(systemctl cat mooc-web.service mooc-worker.service 2>/dev/null | sed -n 's|^WorkingDirectory=||p' | head -1)/\$f\"
                [ -f \"\$r\" ] && printf '    %-28s %s\n' \"\$f\" \"\$(sudo md5sum \"\$r\" | cut -c1-12)\"
            done
        " 2>/dev/null
    done
    printf '\n  salud publica: %s\n' "$(curl -s -o /dev/null -w '%{http_code}' "${URL_PUBLICA}/api/v1/health")"
}

case "${1:-}" in
--estado) estado; exit 0 ;;
tag) VARIANTE=tag ;;
corregido) VARIANTE=corregido ;;
*)
    echo "Uso: a7_desplegar.sh corregido | tag | --estado" >&2
    exit 2
    ;;
esac

# --- Que SHA toca -----------------------------------------------------------
if [ "${VARIANTE}" = tag ]; then
    # ^{commit}: entrega-2 es un tag ANOTADO, asi que rev-parse a secas
    # devuelve el objeto de tag y no el commit. prepare_env.sh solo valida que
    # sean 40 hexadecimales, asi que el objeto pasaria y la imagen no existiria.
    SHA="$(git -C "${RAIZ}" rev-parse "${TAG_BASE}^{commit}")"
else
    SHA="$(git -C "${RAIZ}" rev-parse HEAD)"
fi
titulo "Desplegando la variante ${VARIANTE}: ${SHA}"

for img in api worker; do
    gcloud artifacts docker images describe "${REGISTRO}/${img}:${SHA}" --project "${PROYECTO}" >/dev/null 2>&1 ||
        {
            echo "  FALLA  no existe ${img}:${SHA:0:12} en el registro." >&2
            echo "         Publicala primero: cd <arbol de esa variante> && bash ./scripts/publish_images.sh" >&2
            exit 1
        }
done
echo "  ok     las dos imagenes estan publicadas"

# --- Configuracion y compose ------------------------------------------------
desplegar_vm() { # $1=vm  $2=unidad  $3=conf  $4=guion de entorno  $5=compose
    local vm="$1" unidad="$2" conf="$3" guion="$4" compose="$5"
    titulo "${vm}"

    local home
    home="$(home_servicio "${vm}" "${unidad}")"
    [ -n "${home}" ] || {
        echo "  FALLA  no se pudo leer WorkingDirectory de ${unidad}" >&2
        return 1
    }
    echo "  home del servicio: ${home}"

    if [ "${VARIANTE}" = corregido ]; then
        cp "${RAIZ}/scripts/${guion}" "${TMP}/prepare_env.sh"
        cp "${RAIZ}/${compose}" "${TMP}/${compose}"
        scp_ "${TMP}/prepare_env.sh" "${vm}:/tmp/a7_prepare_env.sh" >/dev/null 2>&1
        scp_ "${TMP}/${compose}" "${vm}:/tmp/a7_${compose}" >/dev/null 2>&1
    fi

    # El guion remoto se copia como archivo: las comillas anidadas no sobreviven
    # a plink en Windows.
    cat >"${TMP}/remoto.sh" <<REMOTO
#!/usr/bin/env bash
set -u
HOME_SRV="${home}"
CONF="${conf}"
COMPOSE="${compose}"
VARIANTE="${VARIANTE}"
SHA="${SHA}"
UNIDAD="${unidad}"

# Respaldo de los archivos del tag, una sola vez. Es lo que permite volver.
for f in prepare_env.sh "\${COMPOSE}"; do
    [ -f "\${HOME_SRV}/\${f}" ] || continue
    [ -f "\${HOME_SRV}/\${f}.tag-respaldo" ] || sudo cp "\${HOME_SRV}/\${f}" "\${HOME_SRV}/\${f}.tag-respaldo"
done
[ -f "\${CONF}.a7-respaldo" ] || sudo cp "\${CONF}" "\${CONF}.a7-respaldo"

if [ "\${VARIANTE}" = corregido ]; then
    sudo cp /tmp/a7_prepare_env.sh "\${HOME_SRV}/prepare_env.sh"
    sudo cp "/tmp/a7_\${COMPOSE}" "\${HOME_SRV}/\${COMPOSE}"
    echo "  configuracion de main copiada"
else
    for f in prepare_env.sh "\${COMPOSE}"; do
        if [ -f "\${HOME_SRV}/\${f}.tag-respaldo" ]; then
            sudo cp "\${HOME_SRV}/\${f}.tag-respaldo" "\${HOME_SRV}/\${f}"
        fi
    done
    echo "  configuracion del tag restaurada"
fi

sudo sed -i "s|^IMAGE_TAG=.*|IMAGE_TAG=\${SHA}|" "\${CONF}"
echo "  IMAGE_TAG = \$(sudo sed -n 's|^IMAGE_TAG=||p' "\${CONF}")"

sudo systemctl restart "\${UNIDAD}" || exit 1
sleep 12

# Comprobacion que de verdad importa: que la imagen EN EJECUCION sea la pedida.
# Un restart que falla deja los contenedores viejos corriendo y todo parece bien.
mal=0
for c in \$(sudo docker ps --format '{{.Names}}'); do
    img="\$(sudo docker inspect --format '{{.Config.Image}}' "\$c" 2>/dev/null)"
    case "\$img" in
    *mooc/api:* | *mooc/worker:*)
        if [ "\${img##*:}" = "\${SHA}" ]; then
            echo "  ok   \$c -> \${SHA:0:12}"
        else
            echo "  MAL  \$c -> \${img##*:} (se pidio \${SHA:0:12})"
            mal=1
        fi
        ;;
    *) echo "  --   \$c (\${img})" ;;
    esac
done
exit \$mal
REMOTO

    scp_ "${TMP}/remoto.sh" "${vm}:/tmp/a7_remoto.sh" >/dev/null 2>&1
    ssh_ "${vm}" --command "bash /tmp/a7_remoto.sh"
}

desplegar_vm mooc-worker-server mooc-worker.service /etc/mooc/worker.conf prepare_worker_env.sh docker-compose.worker.yml || exit 1
desplegar_vm mooc-web-server mooc-web.service /etc/mooc/web.conf prepare_web_env.sh docker-compose.prod.yml || exit 1

titulo "Verificacion"
salud="$(curl -s -o /dev/null -w '%{http_code}' "${URL_PUBLICA}/api/v1/health")"
echo "  salud publica: ${salud}"
[ "${salud}" = "200" ] || {
    echo "  FALLA  la API no contesta 200; no midas hasta resolverlo." >&2
    exit 1
}

if [ "${VARIANTE}" = corregido ]; then
    cat <<'AVISO'

  Falta el esquema. Con la imagen de main desplegada:
      bash ./scripts/migrate.sh --require
  se niega a seguir si falta la migracion 000008 (snapshot de quiz).
  Las migraciones se aplican DENTRO de la VPC: la base no tiene IP publica.
AVISO
fi
echo "  Listo."
