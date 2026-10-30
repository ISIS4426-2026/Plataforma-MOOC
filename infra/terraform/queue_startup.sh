#!/usr/bin/env bash
# Arranque de la VM de la cola compartida (B1 - issue #200), que desde C6 (#209)
# tambien levanta el exportador de la senal de autoescalado.
#
# Lo rellena Terraform con `templatefile`, que sustituye $${queue_ip}. Vive en su
# propio fichero y no dentro de `metadata_startup_script` porque escribe un
# redis.conf con heredoc: un heredoc anidado dentro del heredoc de Terraform
# depende de cómo `<<-EOT` recorta la sangría común, y un espacio de más al
# principio de una directiva deja a redis-server sin arrancar en una máquina sin
# IP pública a la que hay que entrar por IAP para averiguarlo.
set -euo pipefail

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

$APT update
$APT install -y redis-server

# La configuración se escribe entera y no se parchea la de Debian, para que lo
# que corre sea exactamente lo que este fichero declara.
cat >/etc/redis/redis.conf <<CONF
# Cola compartida de asynq y cache de sesiones (B1, issue #200).
# Generado por Terraform desde infra/terraform/queue_startup.sh.
# No editar a mano en la VM: el siguiente apply lo sobreescribe.

# Escucha en la IP privada estatica y en loopback. Sin IP publica en la
# instancia y con el cortafuegos acotado a la subred, la cola no se alcanza
# desde fuera de la VPC.
bind ${queue_ip} 127.0.0.1
port 6379
protected-mode no

# PERSISTENCIA: AOF, que es la razon de no usar Memorystore.
#
# \`everysec\` es el compromiso correcto para una cola de trabajos: pierde como
# maximo un segundo de escrituras y no paga un fsync por operacion. Con
# \`always\` el encolado se vuelve sincrono con el disco; con \`no\` se pierde lo
# que el sistema operativo tenga en cache cuando cae la maquina.
appendonly yes
appendfsync everysec
appendfilename "appendonly.aof"
dir /var/lib/redis

# Instantaneas RDB, ademas del AOF. No son la garantia --el AOF lo es-- sino un
# segundo camino de recuperacion si el AOF se corrompe.
save 900 1
save 300 10
save 60 10000

# EXPULSION: asynq la prohibe. Un trabajo expulsado es un trabajo perdido, y se
# pierde en silencio, porque un LPUSH rechazado se ve igual que un trabajo que
# nunca llego. Con \`noeviction\` el fallo es ruidoso, que es lo que se quiere.
# Ver la nota tecnica 23.
maxmemory 1400mb
maxmemory-policy noeviction

# Dos bases logicas: la 0 es la cola de asynq y la 1 la cache de sesiones
# (REDIS_SESSION_DB). No aisla memoria --la politica de expulsion es de la
# instancia entera-- sino operacion: inspeccionar, medir o vaciar una no toca la
# otra. Memorystore no documenta SELECT, y en modo Cluster solo existe la 0.
databases 16

# Dos comandos desactivados por nombre:
#
#  * FLUSHALL se lleva la cola Y las sesiones de una sola vez.
#  * FLUSHDB sobre la base 0 destruye el registro \`asynq:queues\`, y entonces el
#    inspector responde \`NOT_FOUND: queue "default" does not exist\` mientras los
#    trabajos siguen fluyendo. Es la nota tecnica 27, y costo tres corridas de A7
#    medidas a oscuras.
#
# Mientras la cola era local a cada VM esto era un riesgo de laboratorio. Con una
# cola compartida, \`scripts/seed.sh\` apuntado a produccion se llevaria la cola
# real, asi que el candado va en el servidor y no en la buena memoria de quien
# ejecuta el guion.
rename-command FLUSHALL ""
rename-command FLUSHDB ""

daemonize no
supervised systemd
loglevel notice
logfile ""
CONF

chown redis:redis /etc/redis/redis.conf
chmod 640 /etc/redis/redis.conf

# `enable` y `restart`, NO `enable --now`.
#
# `apt-get install redis-server` **ya deja el servicio corriendo** con la
# configuracion por omision de Debian, que escucha solo en 127.0.0.1. Y
# `--now` no hace nada sobre un servicio que ya esta activo: no es `restart`.
#
# Con `--now`, redis seguia con la configuracion vieja aunque el fichero en
# disco fuera el correcto, y la API moria en bucle con
# `dial tcp 10.0.1.8:6379: connect: connection refused`. Es el mismo error que
# las unidades de B3 evitan a proposito usando `restart`.
systemctl enable redis-server
systemctl restart redis-server

# Comprobacion de arranque. AFIRMA, no imprime.
#
# La version anterior hacia `CONFIG GET appendonly` y mostraba el resultado sin
# comprobarlo, asi que el arranque salia con 0 aunque redis estuviera corriendo
# otra configuracion. Y `maxmemory-policy` coincidia por casualidad --`noeviction`
# es tambien el valor por omision de Debian--, asi que mirar solo esa clave no
# habria detectado nada.
fallos=0

for _ in $(seq 1 30); do
    redis-cli -h 127.0.0.1 ping 2>/dev/null | grep -q PONG && break
    sleep 2
done

if ! redis-cli -h 127.0.0.1 ping 2>/dev/null | grep -q PONG; then
    echo "B1: ERROR, redis-server no respondio en 60 s" >&2
    systemctl status redis-server --no-pager >&2 || true
    exit 1
fi
echo "B1: redis responde PONG en 127.0.0.1:6379"

comprobar() {
    local clave="$1" esperado="$2"
    local efectivo
    # Sin `--no-raw`: esa opcion numera la salida --`2) "yes"`-- y entonces
    # `sed -n 2p` devuelve el numero y los parentesis. La asercion fallaba sobre
    # una cola correctamente configurada: `appendonly = '2) yes'`.
    #
    # Fallo del lado seguro --un falso negativo, ruidoso e inmediato-- y no del
    # peligroso, que es dar por bueno lo que no se comprobo. Pero estaba mal.
    efectivo="$(redis-cli -h 127.0.0.1 CONFIG GET "$clave" 2>/dev/null | sed -n '2p')"
    if [ "$efectivo" = "$esperado" ]; then
        echo "B1: ok     $clave = $efectivo"
    else
        echo "B1: ERROR  $clave = '$efectivo', se esperaba '$esperado'" >&2
        fallos=$((fallos + 1))
    fi
}

# `appendonly` es la que delata una configuracion sin recargar: es la razon de no
# usar Memorystore y su valor por omision en Debian es `no`.
comprobar appendonly yes
comprobar maxmemory-policy noeviction

# Y que escuche de verdad en la IP privada, que es lo unico que la API y los
# workers pueden alcanzar. Escuchar solo en loopback era el sintoma del fallo.
if ss -ltn 2>/dev/null | grep -q "${queue_ip}:6379"; then
    echo "B1: ok     escucha en ${queue_ip}:6379"
else
    echo "B1: ERROR  no escucha en ${queue_ip}:6379" >&2
    ss -ltn 2>/dev/null | grep 6379 >&2 || true
    fallos=$((fallos + 1))
fi

if [ "$fallos" -gt 0 ]; then
    echo "B1: ERROR, $fallos comprobacion(es) fallaron. La cola NO esta como se declaro." >&2
    exit 1
fi

echo "B1: la cola esta en marcha y configurada como se declaro"

# ==============================================================================
# El exportador de la senal de cola (C6 - issue #209)
# ==============================================================================
# POR QUE CORRE AQUI Y NO EN EL WORKER
#
# `internal/worker/queue_metrics.go` ya publica la profundidad de la cola desde
# dentro de cada worker, y eso **no sirve para autoescalar**: el autoescalador de
# GCE **suma** las series que devuelve su consulta, y la profundidad es una
# magnitud de la cola, no de la instancia. Con tres workers veria el triple del
# que hay y escalaria por su propio reflejo. Y desaparece durante una rotacion,
# justo cuando la situacion esta cambiando.
#
# El enunciado lo dice desde el otro lado: la cola no debe «depender su
# disponibilidad de una maquina que el autoscaling pueda retirar», y la senal que
# gobierna el autoscaling es parte de esa disponibilidad. Esta VM es el unico
# computo del despliegue que no pertenece a un grupo elastico.
#
# POR QUE SUS FALLOS NO MATAN EL ARRANQUE
#
# La cola es el servicio critico de esta maquina; el exportador es telemetria. Un
# `docker pull` que falla no puede dejar la plataforma sin cola, asi que esta
# seccion corre **despues** de que las comprobaciones de Redis hayan pasado y sus
# fallos solo avisan.
#
# Que eso no lo vuelva invisible: sin exportador la serie desaparece de Cloud
# Monitoring, y un autoescalador sin datos **mantiene el tamano actual** --no baja
# al minimo ni se desborda--. El grupo se queda congelado: un fallo estatico y
# visible en la consola del autoescalador, no uno silencioso.
# ==============================================================================

IMAGEN_EXPORTADOR="${exporter_image}"
REGISTRO="${region}-docker.pkg.dev"

avisar() { echo "C6: AVISO, $1" >&2; }

instalar_docker() {
    if command -v docker >/dev/null; then
        echo "C6: docker ya esta instalado"
        return 0
    fi

    # El repositorio oficial de Docker, igual que `infra/bootstrap/
    # mooc-bootstrap.sh`: una sola forma de instalar Docker en todo el repo.
    #
    # **Sin** `docker-buildx-plugin` ni `docker-compose-plugin`, que si instala
    # el arranque de las instancias. Esta maquina corre un contenedor con
    # `docker run`: no compone nada ni construye imagenes, y es una e2-small con
    # 10 GiB de disco.
    $APT install -y ca-certificates curl gnupg lsb-release || return 1

    install -m 0755 -d /etc/apt/keyrings
    if [ ! -f /etc/apt/keyrings/docker.gpg ]; then
        curl -fsSL https://download.docker.com/linux/debian/gpg |
            gpg --dearmor -o /etc/apt/keyrings/docker.gpg || return 1
        chmod a+r /etc/apt/keyrings/docker.gpg
    fi
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian $(lsb_release -cs) stable" \
        >/etc/apt/sources.list.d/docker.list

    $APT update || return 1
    $APT install -y docker-ce docker-ce-cli containerd.io || return 1
    systemctl enable --now docker || return 1
}

configurar_exportador() {
    # Credenciales del registro con la identidad de la instancia, del servidor de
    # metadatos. No hay archivo de llave que guardar ni rotar. Es el permiso que
    # C6 anade a `sa-queue-server` corrigiendo el inventario de B4, que decia que
    # esta cuenta «no baja imagenes».
    gcloud auth configure-docker "$REGISTRO" --quiet || return 1

    install -d -m 0755 /etc/mooc

    # El entorno del exportador. No lleva ningun secreto: la cola no tiene
    # contrasena --esta acotada por cortafuegos a dos cuentas de servicio, B2-- y
    # las credenciales de Cloud Monitoring salen del servidor de metadatos.
    cat >/etc/mooc/queue-exporter.env <<ENV
GOOGLE_CLOUD_PROJECT=${project_id}
GOOGLE_CLOUD_LOCATION=${zone}
REDIS_URL=redis://127.0.0.1:6379
QUEUE_NAMES=${queue_names}
EXPORT_INTERVAL=${export_interval}
NODE_ID=${node_id}
METRIC_NAMESPACE=${metric_namespace}
ENV
    chmod 644 /etc/mooc/queue-exporter.env

    # `--network host` para alcanzar 127.0.0.1:6379 sin publicar puertos ni
    # abrir la cola a la red de puentes de Docker.
    #
    # `ExecStartPre` con `-` delante: un `pull` que falla **no** impide arrancar
    # con la imagen que ya este en disco, asi que un reinicio sin red o sin
    # registro sigue exportando la senal.
    cat >/etc/systemd/system/mooc-queue-exporter.service <<UNIDAD
[Unit]
Description=Exportador de la senal de cola a Cloud Monitoring (C6, issue #209)
Documentation=https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/209
After=docker.service redis-server.service
Requires=docker.service

[Service]
Type=simple
Restart=always
RestartSec=15
ExecStartPre=-/usr/bin/docker pull $IMAGEN_EXPORTADOR
ExecStartPre=-/usr/bin/docker rm -f mooc-queue-exporter
ExecStart=/usr/bin/docker run --rm --name mooc-queue-exporter --network host --env-file /etc/mooc/queue-exporter.env $IMAGEN_EXPORTADOR /app/queue-exporter
ExecStop=/usr/bin/docker stop --time 10 mooc-queue-exporter

[Install]
WantedBy=multi-user.target
UNIDAD

    systemctl daemon-reload || return 1
    systemctl enable mooc-queue-exporter || return 1
    systemctl restart mooc-queue-exporter || return 1
}

# A partir de aqui nada aborta el arranque: `set -e` se desactiva a proposito
# para que un fallo de telemetria no se lleve una maquina con la cola sana.
set +e

if ! instalar_docker; then
    avisar "no se pudo instalar docker; la cola sigue en marcha sin exportador"
elif ! configurar_exportador; then
    avisar "no se pudo arrancar el exportador; la cola sigue en marcha"
    systemctl status mooc-queue-exporter --no-pager >&2
    journalctl -u mooc-queue-exporter --no-pager -n 40 >&2
else
    # No se afirma que la metrica llegue: la primera escritura tarda y Cloud
    # Monitoring tarda mas en indexar un tipo de metrica nuevo. Lo que si se
    # comprueba es que el contenedor este corriendo, que es lo que esta maquina
    # controla.
    sleep 10
    if docker ps --filter name=mooc-queue-exporter --filter status=running \
        --format '{{.Names}}' | grep -q mooc-queue-exporter; then
        echo "C6: ok     el exportador corre contra $IMAGEN_EXPORTADOR"
    else
        avisar "el exportador no quedo corriendo"
        journalctl -u mooc-queue-exporter --no-pager -n 40 >&2
    fi
fi

exit 0
