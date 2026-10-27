#!/usr/bin/env bash
# Inicializacion automatica y reproducible de cada instancia nueva (B3, #202).
#
# Lo ejecuta el agente invitado como `metadata_startup_script`, en la VM fija y
# en cada instancia que levanten los grupos administrados de C1 y C5. Al terminar
# la maquina atiende peticiones (rol web) o consume trabajos (rol worker) sin que
# nadie haya entrado por SSH.
#
# QUE SUSTITUYE
#
# Hasta B3 el arranque dejaba la maquina a medias: instalaba Docker y nada mas.
# El resto era manual y estaba descrito en ADMINISTRACION.md: crear
# /etc/mooc/<rol>.conf a mano, clonar el repositorio en el directorio personal de
# alguien, escribir la unidad de systemd con esa ruta dentro y levantar Compose.
# Dos cosas de ahi no sobreviven a un grupo administrado:
#
#   * `/home/dfortizr1/mooc` no existe en una instancia nueva, y la unidad
#     llevaba esa ruta escrita dentro;
#   * `/etc/mooc/<rol>.conf` escrito a mano es configuracion que solo existe en
#     un disco, y una recreacion la borra. Es la nota tecnica 19.
#
# ESTE GUION NO SE PARAMETRIZA EN TERRAFORM
#
# Todo lo que necesita lo lee del **servidor de metadatos en tiempo de
# ejecucion**, no por interpolacion de `templatefile`. Asi los bytes del guion
# son identicos para los dos roles y para todas las instancias, y el criterio
# «todas las instancias de un grupo y una corrida ejecutan la misma version de
# codigo y configuracion» se cumple por construccion y no por disciplina: lo
# unico que distingue una instancia de otra es su metadata.
#
# METADATA QUE CONSUME
#
#   mooc-role               web | worker                              (obligatoria)
#   mooc-image-tag          SHA de 40 hex del commit publicado         (obligatoria)
#   mooc-db-ip              IP privada de Cloud SQL                    (obligatoria)
#   mooc-queue-ip           IP privada de la cola compartida (B1)      (obligatoria)
#   mooc-project-id         proyecto de GCP                            (por omision abajo)
#   mooc-region             region del Artifact Registry               (por omision us-east1)
#   mooc-repo-url           repositorio publico a clonar               (por omision abajo)
#   mooc-app-domain         dominio de la aplicacion            (rol web, obligatoria)
#   mooc-nginx-config       perfil de nginx a servir        (rol web, por omision ./nginx.mig.conf)
#   mooc-smtp-from          remitente verificado                (rol web, obligatoria)
#   mooc-smtp-username      login SMTP                          (rol web, obligatoria)
#   mooc-hls-bucket         bucket de derivados              (rol worker, obligatoria)
#   mooc-worker-concurrency procesos de transcodificacion        (rol worker, por omision 2)
#
# IDEMPOTENCIA
#
# Se puede ejecutar las veces que haga falta. El agente invitado lo corre en cada
# arranque, asi que eso no es una propiedad deseable sino un requisito: instalar
# paquetes, clonar, escribir configuracion y habilitar la unidad comprueban antes
# si el trabajo ya esta hecho, y `docker compose up -d` reutiliza los
# contenedores que ya existen en vez de crear otros.
#
# SI APLICA EL ESQUEMA, Y ESO LO DECIDIO A3
#
# A3 (#195) eligio «arranque protegido con el lock, no paso explicito de
# despliegue», y su criterio de aceptacion lo dice: «el procedimiento vive en el
# script de arranque, no en pasos manuales». Lo ejecuta la unidad de systemd
# antes de levantar los contenedores, via `mooc-migrate.sh`.
#
# Es seguro que tres instancias lo hagan a la vez porque cada migracion toma un
# advisory lock de transaccion antes de comprobar si ya esta aplicada, y la
# comprobacion ocurre dentro del lock. Sin eso --y A3 lo reprodujo-- tres
# instancias simultaneas daban dos arranques fallidos de tres, y el grupo
# administrado las recreaba para que fallaran igual, en bucle. La nota tecnica 5
# describe el defecto; A3 es su arreglo.
#
# LO QUE DELIBERADAMENTE NO HACE
#
#   * **No instala FFmpeg en el anfitrion.** Va dentro de la imagen del worker
#     (`Dockerfile.worker`, `apk add ffmpeg`). Instalarlo tambien aqui seria una
#     dependencia duplicada que nadie usa.
#   * **No siembra datos.** `seed.sh` es una herramienta de desarrollo.
set -Eeuo pipefail

REGISTRO="/var/log/mooc-bootstrap.log"
MEDIDA="/var/lib/mooc/arranque.json"
DESTINO="/opt/mooc"

exec > >(tee -a "${REGISTRO}") 2>&1

INICIO_EPOCH="$(date +%s)"

anotar() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] B3: $*"; }
morir() { anotar "ERROR: $*"; exit 1; }

anotar "===== inicio del arranque ====="

# ---------------------------------------------------------------- 1. metadata
meta() {
    curl -fsS --max-time 5 -H 'Metadata-Flavor: Google' \
        "http://metadata.google.internal/computeMetadata/v1/instance/attributes/$1" 2>/dev/null || true
}

ROL="$(meta mooc-role)"
IMAGE_TAG="$(meta mooc-image-tag)"
DB_IP="$(meta mooc-db-ip)"
QUEUE_IP="$(meta mooc-queue-ip)"
PROJECT_ID="$(meta mooc-project-id)"; PROJECT_ID="${PROJECT_ID:-plataforma-mooc-entrega2}"
REGION="$(meta mooc-region)"; REGION="${REGION:-us-east1}"
REPO_URL="$(meta mooc-repo-url)"
REPO_URL="${REPO_URL:-https://github.com/ISIS4426-2026/Plataforma-MOOC.git}"

[[ "${ROL}" == "web" || "${ROL}" == "worker" ]] ||
    morir "mooc-role debe ser 'web' o 'worker', no '${ROL}'"

# El SHA completo y no una etiqueta movil. Con `latest` dos instancias del mismo
# grupo pueden quedarse en imagenes distintas segun cuando arrancaron, y eso es
# precisamente lo que el criterio de aceptacion prohibe.
[[ "${IMAGE_TAG}" =~ ^[0-9a-f]{40}$ ]] ||
    morir "mooc-image-tag debe ser el SHA completo de 40 caracteres"

[[ "${DB_IP}" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] || morir "mooc-db-ip no es una IPv4"
[[ "${QUEUE_IP}" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] || morir "mooc-queue-ip no es una IPv4"

anotar "rol=${ROL} commit=${IMAGE_TAG:0:12} proyecto=${PROJECT_ID} region=${REGION}"

# ------------------------------------------------------------- 2. dependencias
# `DPkg::Lock::Timeout` hace que apt **espere** por cualquiera de sus cerrojos en
# vez de fallar. Esperar solo el de dpkg --como hacia la version anterior-- no
# basta: en una VM recien creada, `unattended-upgrades` corre `apt-get update` en
# paralelo y retiene `/var/lib/apt/lists/lock`, que es otro.
#
# Paso de verdad: la cola recreada murio con
#   E: Could not get lock /var/lib/apt/lists/lock. It is held by process 736
#   Script "startup-script" failed with error: exit status 100
# y redis nunca se instalo. Con `set -e`, un apt que falla se lleva el arranque.
APT="apt-get -o DPkg::Lock::Timeout=600 -qq"


if ! command -v docker >/dev/null || ! command -v git >/dev/null ||
    ! command -v psql >/dev/null; then
    anotar "instalando dependencias del anfitrion"
    $APT update
    # `postgresql-client` no es opcional: `scripts/migrate.sh` necesita `psql`,
    # y sin el la instancia arranca sin esquema. El guion de arranque anterior
    # lo instalaba solo en la VM web; aqui lo necesitan los dos roles, porque
    # los dos aplican el esquema (A3).
    $APT install -y ca-certificates curl gnupg lsb-release git jq postgresql-client

    install -m 0755 -d /etc/apt/keyrings
    if [[ ! -f /etc/apt/keyrings/docker.gpg ]]; then
        curl -fsSL https://download.docker.com/linux/debian/gpg |
            gpg --dearmor -o /etc/apt/keyrings/docker.gpg
        chmod a+r /etc/apt/keyrings/docker.gpg
    fi
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian $(lsb_release -cs) stable" \
        >/etc/apt/sources.list.d/docker.list

    $APT update
    $APT install -y docker-ce docker-ce-cli containerd.io \
        docker-buildx-plugin docker-compose-plugin
else
    anotar "docker, git y psql ya estan instalados"
fi

systemctl enable --now docker
anotar "docker activo"

# ------------------------------------------------- 3. credenciales del registro
# Con la identidad de la instancia, del servidor de metadatos. No hay archivo de
# llave que guardar ni rotar: por eso las cuentas de servicio van adjuntas a las
# VMs y no se genera ninguna clave (service_accounts.tf).
#
# Se escribe en la configuracion de docker de root porque la unidad de systemd
# corre como root.
anotar "autenticando contra ${REGION}-docker.pkg.dev"
gcloud auth configure-docker "${REGION}-docker.pkg.dev" --quiet ||
    morir "no se pudo configurar el ayudante de credenciales de Docker"

# ------------------------------------------------------------- 4. el repositorio
# Se clona **en el commit que nombra IMAGE_TAG**. Es lo que hace que el codigo de
# los contenedores y los ficheros de despliegue --compose, guiones de entorno,
# unidades-- sean del mismo commit. Clonar `main` y correr una imagen de otro
# commit es como se acaba depurando una diferencia que no esta en el codigo que
# se esta leyendo.
if [[ ! -d "${DESTINO}/.git" ]]; then
    anotar "clonando ${REPO_URL} en ${DESTINO}"
    rm -rf "${DESTINO}"
    git clone --quiet "${REPO_URL}" "${DESTINO}" || morir "no se pudo clonar el repositorio"
fi

git -C "${DESTINO}" config --global --add safe.directory "${DESTINO}" || true
git -C "${DESTINO}" fetch --quiet --all --tags || morir "no se pudo actualizar el repositorio"
git -C "${DESTINO}" checkout --quiet --detach "${IMAGE_TAG}" ||
    morir "el commit ${IMAGE_TAG} no existe en el repositorio"
anotar "repositorio en $(git -C "${DESTINO}" rev-parse HEAD)"

# ------------------------------------------------------- 5. /etc/mooc/<rol>.conf
# Generado de la metadata, no escrito a mano. Nunca contiene contrasenas: los
# guiones de preparacion las leen de Secret Manager con la identidad de la
# instancia, y de ahi van al .env en modo 600.
install -d -m 0755 /etc/mooc
CONF="/etc/mooc/${ROL}.conf"
TMP_CONF="$(mktemp)"
trap 'rm -f "${TMP_CONF:-}"' EXIT

if [[ "${ROL}" == "web" ]]; then
    APP_DOMAIN="$(meta mooc-app-domain)"
    SMTP_FROM="$(meta mooc-smtp-from)"
    SMTP_USERNAME="$(meta mooc-smtp-username)"

    # Que perfil de nginx sirve esta instancia. La VM fija se queda con
    # `nginx.conf`, que termina TLS con un certificado de Let's Encrypt; una
    # instancia de grupo no puede --no hay certificado en un disco nuevo y sin IP
    # publica certbot no podria obtenerlo-- asi que sirve HTTP plano en el 80 y
    # el balanceador termina TLS. Un 301 en el 80 ademas tumbaria la
    # comprobacion de salud de C1, y una instancia no sana se recrea, en bucle.
    NGINX_CONFIG="$(meta mooc-nginx-config)"; NGINX_CONFIG="${NGINX_CONFIG:-./nginx.mig.conf}"
    [[ -n "${APP_DOMAIN}" ]] || morir "falta mooc-app-domain"
    [[ -n "${SMTP_FROM}" ]] || morir "falta mooc-smtp-from"
    [[ -n "${SMTP_USERNAME}" ]] || morir "falta mooc-smtp-username"
    cat >"${TMP_CONF}" <<CONFEOF
# Generado por infra/bootstrap/mooc-bootstrap.sh (B3). No editar a mano: el
# siguiente arranque lo sobreescribe con lo que diga la metadata.
IMAGE_TAG=${IMAGE_TAG}
APP_DOMAIN=${APP_DOMAIN}
DB_PRIVATE_IP=${DB_IP}
QUEUE_PRIVATE_IP=${QUEUE_IP}
GCP_PROJECT_ID=${PROJECT_ID}
SMTP_FROM=${SMTP_FROM}
SMTP_USERNAME=${SMTP_USERNAME}
NGINX_CONFIG_FILE=${NGINX_CONFIG}
CONFEOF
else
    HLS_BUCKET="$(meta mooc-hls-bucket)"
    CONCURRENCY="$(meta mooc-worker-concurrency)"; CONCURRENCY="${CONCURRENCY:-2}"
    [[ -n "${HLS_BUCKET}" ]] || morir "falta mooc-hls-bucket"
    cat >"${TMP_CONF}" <<CONFEOF
# Generado por infra/bootstrap/mooc-bootstrap.sh (B3). No editar a mano.
IMAGE_TAG=${IMAGE_TAG}
DB_PRIVATE_IP=${DB_IP}
QUEUE_PRIVATE_IP=${QUEUE_IP}
GCP_PROJECT_ID=${PROJECT_ID}
MEDIA_HLS_BUCKET=${HLS_BUCKET}
WORKER_CONCURRENCY=${CONCURRENCY}
CONFEOF
fi

install -m 0640 -o root -g root "${TMP_CONF}" "${CONF}"
rm -f "${TMP_CONF}"
trap - EXIT
anotar "configuracion escrita en ${CONF} (modo 0640)"

# Las unidades invocan `prepare_env.sh`, que es el enlace que elige el guion del
# rol. Asi la unidad no tiene que saber cual de los dos le toca.
ln -sfn "${DESTINO}/scripts/prepare_${ROL}_env.sh" "${DESTINO}/prepare_env.sh"

# ------------------------------------------------------ 6. retirar lo heredado
# Antes de B3 las unidades se escribian a mano con una ruta dentro de un
# directorio personal. Si este disco viene de aquella epoca, esa unidad seguiria
# levantando Compose desde la otra ruta: los nombres de contenedor son fijos
# (`mooc-api`, `mooc-worker`), asi que no se duplicarian, fallaria una de las
# dos. Se retira la vieja antes de instalar la nueva.
for heredada in /etc/systemd/system/mooc-web.service /etc/systemd/system/mooc-worker.service; do
    if [[ -f "${heredada}" ]] && ! grep -q "${DESTINO}" "${heredada}"; then
        anotar "retirando unidad heredada $(basename "${heredada}") que apunta a otra ruta"
        systemctl disable --now "$(basename "${heredada}")" >/dev/null 2>&1 || true
        mv "${heredada}" "${heredada}.b3-respaldo"
    fi
done

# ------------------------------------------------------------- 7. la unidad
UNIDAD="mooc-${ROL}.service"
install -m 0644 "${DESTINO}/infra/bootstrap/${UNIDAD}" "/etc/systemd/system/${UNIDAD}"
systemctl daemon-reload
anotar "unidad ${UNIDAD} instalada"

systemctl enable "${UNIDAD}" >/dev/null 2>&1 || true

# `restart` y no `start`: en un rearranque la unidad ya esta activa, y `start`
# sobre una unidad activa no vuelve a pasar por ExecStartPre, asi que no
# regeneraria el .env con el IMAGE_TAG nuevo.
systemctl restart "${UNIDAD}" || morir "la unidad ${UNIDAD} no arranco"
anotar "unidad ${UNIDAD} activa"

# ----------------------------------------------------------------- 8. listo
# El «listo» del rol web es /api/v1/ready, de A2: prueba la base y la bandera de
# arranque, no solo que el proceso respire. Es el mismo que usara el balanceador
# en C3, asi que lo que aqui se mide es lo que alli decide si la instancia
# recibe trafico.
listo_web() { curl -fsS --max-time 3 "http://127.0.0.1:8080/api/v1/ready" >/dev/null 2>&1; }

# El worker no atiende peticiones, asi que su «listo» es que el proceso este
# arriba y exponiendo metricas: es lo que el agente de operaciones raspa y lo
# que da la senal de cola de C6.
listo_worker() {
    docker ps --filter name=mooc-worker --filter status=running --format '{{.Names}}' |
        grep -q mooc-worker &&
        curl -fsS --max-time 3 "http://127.0.0.1:9090/metrics" >/dev/null 2>&1
}

anotar "esperando a que la instancia quede lista"
LISTO=0
for _ in $(seq 1 90); do
    if [[ "${ROL}" == "web" ]]; then
        listo_web && { LISTO=1; break; }
    else
        listo_worker && { LISTO=1; break; }
    fi
    sleep 2
done

FIN_EPOCH="$(date +%s)"
SEGUNDOS=$((FIN_EPOCH - INICIO_EPOCH))

# El numero que se publica es desde el inicio de ESTE guion, no desde la creacion
# de la instancia: el arranque del sistema operativo y del agente invitado van
# antes y no son nuestros. El tiempo de creacion de la instancia esta en
# `creation_timestamp`, y la diferencia entre los dos esta documentada en la
# evidencia de B3 para que nadie compare dos numeros distintos.
install -d -m 0755 /var/lib/mooc
cat >"${MEDIDA}" <<JSONEOF
{
  "rol": "${ROL}",
  "commit": "${IMAGE_TAG}",
  "listo": $([[ ${LISTO} -eq 1 ]] && echo true || echo false),
  "segundos_hasta_listo": ${SEGUNDOS},
  "medido_desde": "inicio de mooc-bootstrap.sh, no desde la creacion de la instancia",
  "sonda": "$([[ "${ROL}" == "web" ]] && echo "GET /api/v1/ready" || echo "contenedor en marcha + GET /metrics")",
  "instante_utc": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
JSONEOF

if [[ ${LISTO} -eq 1 ]]; then
    anotar "===== LISTO en ${SEGUNDOS}s (rol ${ROL}, commit ${IMAGE_TAG:0:12}) ====="
else
    anotar "===== NO LISTO tras ${SEGUNDOS}s ====="
    anotar "estado de la unidad:"
    systemctl status "${UNIDAD}" --no-pager || true
    docker ps -a || true
    exit 1
fi
