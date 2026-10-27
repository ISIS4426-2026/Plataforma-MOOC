#!/usr/bin/env bash
# Arranque de la VM de la cola compartida (B1 - issue #200).
#
# Lo rellena Terraform con `templatefile`, que sustituye $${queue_ip}. Vive en su
# propio fichero y no dentro de `metadata_startup_script` porque escribe un
# redis.conf con heredoc: un heredoc anidado dentro del heredoc de Terraform
# depende de cómo `<<-EOT` recorta la sangría común, y un espacio de más al
# principio de una directiva deja a redis-server sin arrancar en una máquina sin
# IP pública a la que hay que entrar por IAP para averiguarlo.
set -euo pipefail

while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do
    sleep 2
done

apt-get update -qq
apt-get install -y -qq redis-server

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
    efectivo="$(redis-cli -h 127.0.0.1 --no-raw CONFIG GET "$clave" 2>/dev/null | sed -n '2p' | tr -d '"')"
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
exit 0
