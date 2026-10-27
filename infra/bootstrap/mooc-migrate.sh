#!/usr/bin/env bash
# Aplica el esquema al arrancar una instancia (B3, #202, siguiendo A3, #195).
#
# POR QUE ESTO EXISTE Y NO ES UNA LINEA EN LA UNIDAD
#
# `migrate.sh` espera `DATABASE_URL` en el entorno, y esa cadena la genera
# `prepare_env.sh` dentro de `/opt/mooc/.env` en modo 600. Pasarla desde la
# unidad de systemd obligaria a citarla a mano en un `ExecStartPre`, con la
# contrasena dentro: aqui se lee del fichero y nunca aparece en la linea de
# ordenes ni en el registro de systemd.
#
# QUE EJECUTA, Y POR QUE DOS VECES
#
# Es literalmente lo que decidio A3 §4:
#
#   migrate.sh             aplica lo que falte, protegido por el advisory lock
#   migrate.sh --require   y se niega a seguir si aun falta algo
#
# La segunda parece redundante y no lo es: cubre el caso en que la primera no
# llegue a ejecutarse --un arranque que la salta, alguien que despliega a mano--
# y convierte un esquema desfasado en un arranque que falla, en vez de en una
# instancia que atiende contra una base que no corresponde.
#
# POR QUE ES SEGURO QUE TRES INSTANCIAS LO HAGAN A LA VEZ
#
# Porque A3 lo arreglo. Cada migracion toma un advisory lock **de transaccion**
# antes de comprobar si ya esta aplicada, y la comprobacion ocurre dentro del
# lock. Antes de eso, tres instancias simultaneas daban dos arranques fallidos de
# tres, y el grupo administrado las recreaba para que fallaran igual, en bucle.
#
# Lo corre cada rol, web y worker. Con el lock da igual quien llegue primero, y
# que el worker tambien exija el esquema con `--require` evita que procese contra
# una base que no corresponde.
set -Eeuo pipefail

ENV_FILE="${1:-/opt/mooc/.env}"
REPO_DIR="${2:-/opt/mooc}"

[[ -r "${ENV_FILE}" ]] || {
    echo "B3/migrar: falta ${ENV_FILE}; prepare_env.sh tenia que haberlo creado" >&2
    exit 1
}

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

[[ -n "${DATABASE_URL:-}" ]] || {
    echo "B3/migrar: ${ENV_FILE} no trae DATABASE_URL" >&2
    exit 1
}

echo "B3/migrar: aplicando migraciones pendientes"
bash "${REPO_DIR}/scripts/migrate.sh"

echo "B3/migrar: comprobando que no falta ninguna"
bash "${REPO_DIR}/scripts/migrate.sh" --require

echo "B3/migrar: esquema al dia"
