#!/usr/bin/env bash
# Aplica las migraciones pendientes contra una base ya existente (issue #118, C1).
#
# POR QUE HACE FALTA
#
# En local las migraciones las aplica el hook de inicializacion de Postgres
# (scripts/init-db.sh, montado en /docker-entrypoint-initdb.d), que corre una
# sola vez y solo si el directorio de datos esta vacio. Cloud SQL no tiene ese
# hook: la instancia nace con la base creada y vacia, y nadie aplica nada.
#
# Sin este script, migrar la instancia administrada seria copiar y pegar siete
# archivos a mano y recordar cuales ya se aplicaron. Eso funciona una vez.
#
# QUE LO HACE REPETIBLE
#
# Una tabla de control, `schema_migrations`, con el nombre de cada migracion
# aplicada. El script salta las que ya estan, asi que se puede volver a
# ejecutar tras anadir una migracion nueva y solo aplica esa.
#
# Cada migracion se aplica **en una sola transaccion junto con su propio
# registro** en la tabla de control. En PostgreSQL el DDL es transaccional, asi
# que una migracion que falla a mitad no deja ni esquema a medias ni registro:
# se vuelve a intentar desde cero. Lo que no puede pasar --y es el fallo que
# arruina una base-- es quedar aplicada sin registrar, o registrada sin aplicar.
#
# QUE LO HACE SEGURO CON VARIAS INSTANCIAS A LA VEZ  (issue #195, A3)
#
# Con una sola VM daba igual quien migrara: habia uno. Un grupo administrado
# puede levantar tres instancias simultaneas, y las tres arrancarian con el
# mismo esquema pendiente.
#
# Leer que falta y despues aplicarlo deja una ventana entre ambas cosas, y dos
# instancias caben dentro de ella: las dos ven 000008 pendiente, las dos la
# ejecutan, y la segunda muere con «relation already exists». La instancia no
# arranca, y el grupo la recrea para que vuelva a fallar igual.
#
# Por eso cada migracion toma un **advisory lock de transaccion** antes de
# comprobar si ya esta aplicada, y la comprobacion ocurre **dentro** del lock.
# La segunda instancia espera, entra, ve que ya esta, y no hace nada. Las tres
# terminan con exito y el esquema se aplico una vez.
#
# El lock es de transaccion, no de sesion: PostgreSQL lo suelta en el commit o
# en el rollback, tambien si el proceso muere. Un lock de sesion olvidado
# bloquearia los arranques siguientes hasta que alguien lo soltara a mano.
#
# USO
#
#   export DATABASE_URL='postgres://moocuser:<DB_PASSWORD>@10.x.x.x:5432/moocdb?sslmode=require'
#   bash ./scripts/migrate.sh            # aplica las pendientes
#   bash ./scripts/migrate.sh --status   # solo informa, no toca nada
#   bash ./scripts/migrate.sh --verify   # comprueba el esquema resultante
#   bash ./scripts/migrate.sh --baseline # adopta una base que ya tiene el esquema
#   bash ./scripts/migrate.sh --require  # falla si falta alguna; no aplica nada
#
# `--require` es la comprobacion que hace el arranque de una instancia antes de
# levantar la API: si el esquema no trae lo que esta imagen necesita, mas vale
# no atender peticiones que atenderlas contra una base que no corresponde.
#
# `--baseline` existe para las bases de desarrollo. En local el esquema lo creo
# el hook de inicializacion, que no dejo tabla de control, asi que este script
# las ve como vacias e intenta aplicar 000001 --que falla con «relation already
# exists»--. El baseline registra las migraciones como aplicadas sin ejecutarlas.
# Contra una base vacia NO se usa: ahi se aplica de verdad.
#
# Necesita `psql` (paquete postgresql-client) y la base ya creada.

set -euo pipefail

MIGRATIONS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../migrations" && pwd)"

MODO="aplicar"
case "${1:-}" in
    --status) MODO="estado" ;;
    --verify) MODO="verificar" ;;
    --baseline) MODO="baseline" ;;
    --require) MODO="exigir" ;;
    --help | -h)
        sed -n '2,40p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
        exit 0
        ;;
    "") ;;
    *)
        echo "opcion desconocida: $1  (--status, --verify, --baseline, --require, --help)" >&2
        exit 2
        ;;
esac

if [[ -z "${DATABASE_URL:-}" ]]; then
    echo "ERROR: falta DATABASE_URL." >&2
    echo "       La contrasena sale de Secret Manager, no de un archivo del repositorio:" >&2
    echo "       gcloud secrets versions access latest --secret=db-password" >&2
    exit 1
fi

if ! command -v psql >/dev/null 2>&1; then
    echo "ERROR: no hay psql. En Debian/Ubuntu: sudo apt-get install -y postgresql-client" >&2
    exit 1
fi

# El sslmode no es opcional contra Cloud SQL: la instancia esta en
# ENCRYPTED_ONLY y rechaza las conexiones en claro. Se avisa aqui porque el
# error de psql en ese caso no menciona el cifrado.
if [[ "${DATABASE_URL}" != *"sslmode="* ]]; then
    echo "AVISO: DATABASE_URL no lleva sslmode. Contra Cloud SQL hace falta sslmode=require." >&2
fi

# Se ejecuta sin eco de la contrasena: psql recibe la cadena por argumento y
# nunca se imprime. `--no-psqlrc` evita que la configuracion personal de quien
# ejecuta cambie el comportamiento.
psql_() {
    psql "${DATABASE_URL}" --no-psqlrc -v ON_ERROR_STOP=1 "$@"
}

# Una consulta que devuelve un solo valor, sin cabeceras ni adornos.
psql_valor() {
    psql_ --quiet --no-align --tuples-only -c "$1"
}

echo "==> Base: $(psql_valor 'SELECT current_database() || $q$ en $q$ || version();' | head -1)"

# --- Tabla de control -------------------------------------------------------
# `IF NOT EXISTS` la hace idempotente. Se crea tambien en modo --status para
# que el primer informe no falle por una tabla que aun no existe.
psql_ --quiet -c "
    CREATE TABLE IF NOT EXISTS schema_migrations (
        version     TEXT PRIMARY KEY,
        applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );
    COMMENT ON TABLE schema_migrations IS
        'Migraciones aplicadas. La escribe scripts/migrate.sh, no la aplicacion.';
"

ya_aplicadas="$(psql_valor "SELECT version FROM schema_migrations ORDER BY version;")"

esta_aplicada() {
    grep -qxF "$1" <<<"${ya_aplicadas}"
}

# --- Modo --verify ----------------------------------------------------------
if [[ "${MODO}" == "verificar" ]]; then
    echo
    echo "==> Tablas del esquema public"
    psql_ -c "
        SELECT table_name, (
            SELECT COUNT(*) FROM information_schema.columns c
            WHERE c.table_schema = 'public' AND c.table_name = t.table_name
        ) AS columnas
        FROM information_schema.tables t
        WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
        ORDER BY table_name;
    "

    echo "==> Restricciones que sostienen las garantias del dominio"
    # Estas cinco no son decorativas: son las que hacen que las garantias no
    # dependan de una comprobacion en Go --un intento de quiz repetido, un
    # cuestionario duplicado sobre el mismo recurso, dos versiones de curso con
    # el mismo numero--. Si el esquema de la nube no las trae, la instancia
    # parece correcta y no lo es.
    #
    # Las tres `condeferrable` tienen que salir con `t`: el reordenamiento
    # academico (migracion 000003) necesita posiciones que choquen a mitad de la
    # transaccion y se resuelvan antes del commit. Sin diferir, reordenar falla.
    psql_ -c "
        SELECT conname,
               condeferrable AS diferible,
               pg_get_constraintdef(oid) AS definicion
        FROM pg_constraint
        WHERE conname IN (
            'uq_quiz_submissions_attempt',
            'uq_quizzes_resource',
            'uq_quiz_questions_quiz_position',
            'uq_quiz_options_question_position',
            'courses_stable_id_version_key'
        )
        ORDER BY conname;
    "

    # Que salgan las cinco. Un `WHERE ... IN` que no encuentra nada devuelve
    # cero filas sin error, asi que sin este recuento «verificado» podria
    # significar «no hay ninguna».
    faltan="$(psql_valor "
        SELECT 5 - COUNT(*) FROM pg_constraint WHERE conname IN (
            'uq_quiz_submissions_attempt', 'uq_quizzes_resource',
            'uq_quiz_questions_quiz_position', 'uq_quiz_options_question_position',
            'courses_stable_id_version_key');
    ")"
    if [[ "${faltan// /}" != "0" ]]; then
        echo "    ERROR: faltan ${faltan// /} restriccion(es) de las cinco esperadas." >&2
        exit 1
    fi
    echo "    Las cinco presentes."

    echo "==> Disparadores de inmutabilidad de la auditoria (migracion 000004)"
    psql_ -c "
        SELECT c.relname AS tabla, t.tgname AS disparador
        FROM pg_trigger t JOIN pg_class c ON c.oid = t.tgrelid
        WHERE NOT t.tgisinternal AND c.relname LIKE 'audit%'
        ORDER BY 1, 2;
    "

    echo "==> Limite de conexiones de la instancia frente al pool"
    # El numero contra el que hay que dimensionar DB_MAX_OPEN_CONNS en la API y
    # en el worker. El reparto esta en infra/terraform/database.tf.
    psql_ -c "
        SELECT current_setting('max_connections') AS max_connections,
               current_setting('superuser_reserved_connections') AS reservadas,
               (SELECT COUNT(*) FROM pg_stat_activity) AS en_uso_ahora;
    "

    echo "==> Migraciones registradas"
    psql_ -c "SELECT version, applied_at FROM schema_migrations ORDER BY version;"
    exit 0
fi

# --- Modo --baseline --------------------------------------------------------
if [[ "${MODO}" == "baseline" ]]; then
    # Solo tiene sentido si el esquema ya esta ahi. Aplicado por error sobre una
    # base vacia dejaria la tabla de control diciendo que todo se aplico y una
    # base sin una sola tabla, que es el peor estado posible: el script no
    # volveria a intentar nada.
    tablas="$(psql_valor "
        SELECT COUNT(*) FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name <> 'schema_migrations';
    ")"
    if [[ "${tablas// /}" == "0" ]]; then
        echo "ERROR: la base no tiene tablas, asi que no hay nada que adoptar." >&2
        echo "       Ejecuta el script sin --baseline para aplicar las migraciones." >&2
        exit 1
    fi

    echo "==> Adoptando un esquema existente de ${tablas// /} tabla(s)."
    for ruta in "${MIGRATIONS_DIR}"/*.up.sql; do
        version="$(basename "${ruta}" .up.sql)"
        psql_ --quiet -c "
            INSERT INTO schema_migrations (version) VALUES ('${version}')
            ON CONFLICT (version) DO NOTHING;
        "
        echo "    registrada   ${version}"
    done
    echo
    echo "==> Listo. Las migraciones nuevas ya se aplicaran con normalidad."
    exit 0
fi

# --- Modo --require ---------------------------------------------------------
# No aplica nada: solo responde si esta base trae lo que esta copia del
# repositorio necesita. Es lo que ejecuta el arranque de una instancia antes de
# levantar la API.
if [[ "${MODO}" == "exigir" ]]; then
    faltantes=0
    for ruta in "${MIGRATIONS_DIR}"/*.up.sql; do
        version="$(basename "${ruta}" .up.sql)"
        if ! esta_aplicada "${version}"; then
            echo "    FALTA        ${version}" >&2
            faltantes=$((faltantes + 1))
        fi
    done
    if [[ ${faltantes} -gt 0 ]]; then
        echo >&2
        echo "ERROR: el esquema no trae ${faltantes} migracion(es) que esta imagen necesita." >&2
        echo "       Aplicalas antes de que la instancia empiece a atender:" >&2
        echo "         bash ./scripts/migrate.sh" >&2
        exit 1
    fi
    echo "==> El esquema trae todo lo que esta imagen necesita."
    exit 0
fi

# --- Aplicar / informar -----------------------------------------------------
pendientes=0
aplicadas=0

echo
for ruta in "${MIGRATIONS_DIR}"/*.up.sql; do
    version="$(basename "${ruta}" .up.sql)"

    if [[ "${MODO}" == "estado" ]]; then
        if esta_aplicada "${version}"; then
            echo "    ya aplicada  ${version}"
        else
            echo "    PENDIENTE    ${version}"
            pendientes=$((pendientes + 1))
        fi
        continue
    fi

    # La lista leida al empezar sirve para informar, no para decidir: entre
    # aquella lectura y este momento otra instancia puede haber aplicado esta
    # misma migracion. La decision se toma dentro del lock.
    #
    # El orden importa y es el unico que funciona:
    #
    #   1. BEGIN                   -- lo pone --single-transaction
    #   2. tomar el lock           -- la segunda instancia espera aqui
    #   3. preguntar si ya esta    -- ya con el lock en la mano
    #   4. aplicar y registrar, o no hacer nada
    #   5. COMMIT                  -- suelta el lock y libera a la siguiente
    #
    # Preguntar antes del paso 2 seria volver a la carrera que esto evita.
    salida="$(psql_ --quiet --single-transaction \
        -v ver="${version}" -v ruta="${ruta}" -f - <<'SQL'
-- En un bloque DO para que no imprima la tabla de una columna vacia que
-- devuelve la funcion. Lo que interesa es el efecto, no el valor.
DO $$ BEGIN PERFORM pg_advisory_xact_lock(4426195); END $$;
SELECT NOT EXISTS (
    SELECT 1 FROM schema_migrations WHERE version = :'ver'
) AS pendiente \gset
\if :pendiente
\echo '==> Aplicando   ' :ver
\i :ruta
INSERT INTO schema_migrations (version) VALUES (:'ver');
\else
\echo '    ya aplicada  ' :ver
\endif
SQL
    )"
    echo "${salida}"

    if [[ "${salida}" == *"Aplicando"* ]]; then
        aplicadas=$((aplicadas + 1))
    fi
done

echo
if [[ "${MODO}" == "estado" ]]; then
    echo "==> ${pendientes} migracion(es) pendiente(s)."
    exit 0
fi

if [[ ${aplicadas} -eq 0 ]]; then
    echo "==> Nada que aplicar: el esquema ya esta al dia."
else
    echo "==> ${aplicadas} migracion(es) aplicada(s)."
fi

echo "==> Comprueba el resultado con: bash ./scripts/migrate.sh --verify"
