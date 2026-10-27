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

systemctl enable --now redis-server

# Comprobacion de arranque: si la configuracion es invalida, redis-server no
# levanta y el fallo tiene que verse en el registro serie de la instancia, no
# descubrirse cuando el primer worker no encuentra la cola.
for _ in $(seq 1 30); do
    if redis-cli -h 127.0.0.1 ping 2>/dev/null | grep -q PONG; then
        echo "B1: redis responde PONG en 127.0.0.1:6379"
        redis-cli -h 127.0.0.1 CONFIG GET appendonly
        redis-cli -h 127.0.0.1 CONFIG GET maxmemory-policy
        exit 0
    fi
    sleep 2
done

echo "B1: ERROR, redis-server no respondio en 60 s" >&2
systemctl status redis-server --no-pager >&2 || true
exit 1
