#!/bin/sh
# Applies every forward migration, in order, when Postgres initialises.
#
# The previous setup mounted a single migration file by name, so adding
# 000002 without editing docker-compose.yml left a fresh database missing its
# tables and the API answering 500 on registration. Iterating over the
# directory removes that failure mode: any migration added later is picked up
# with no change here.
#
# Only *.up.sql is applied, and the shell glob sorts by the numeric prefix,
# which is what makes 000001 run before 000002.
#
# Note this runs only when the data directory is empty. An existing database
# needs the new migration applied by hand, or the volume recreated.
set -e

for migration in /migrations/*.up.sql; do
    echo "==> Applying migration: ${migration}"
    # ON_ERROR_STOP makes psql exit non-zero on the first failure, so a broken
    # migration aborts initialisation instead of leaving a half-built schema
    # that looks healthy.
    psql -v ON_ERROR_STOP=1 --username "${POSTGRES_USER}" --dbname "${POSTGRES_DB}" -f "${migration}"
done

echo "==> All migrations applied successfully"
