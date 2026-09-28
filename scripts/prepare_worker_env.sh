#!/usr/bin/env bash
set -Eeuo pipefail

# Builds the runtime .env used by docker-compose.worker.yml on the Worker Server
# VM. Secret values are fetched with the VM service account and never printed.
#
# Sibling of prepare_web_env.sh, and it exists for the reason that one does:
# until G3, the Worker Server's .env was written by hand. When the VMs were
# recreated it went with them, the whole asynchronous half of the deployment
# stayed down, and nothing said so -- the API happily enqueued into a broker
# nobody read. Configuration that only exists on a VM is configuration that a
# recreation deletes. See note 19 of docs/entrega2/NOTAS_TECNICAS.md.
#
# Two differences from the web script, both deliberate:
#
#   - REDIS_URL is the Compose service on this very VM. Here `redis:6379` is
#     correct: the queue lives on this machine and the worker is its neighbour
#     in the same Compose network. It is the *web* VM that must reach it across
#     the VPC.
#   - No SMTP. The worker processes video and sends nothing, so it is not
#     granted the mail credential -- that separation is in mail.tf, and reading
#     a secret here that the worker has no access to would only fail.

# Resolve the real script location before deriving the repository directory, so
# an invocation through a symbolic link at the deployment root prepares the same
# file as a direct one.
SCRIPT_PATH="$(readlink -f "${BASH_SOURCE[0]}")"
SCRIPT_DIR="$(cd "$(dirname "${SCRIPT_PATH}")" && pwd)"
REPO_DIR="${MOOC_REPO_DIR:-$(cd "${SCRIPT_DIR}/.." && pwd)}"
CONFIG_FILE="${MOOC_WORKER_CONFIG_FILE:-/etc/mooc/worker.conf}"
ENV_FILE="${REPO_DIR}/.env"

fail() {
  echo "error: $*" >&2
  exit 1
}

[[ -r "${CONFIG_FILE}" ]] || fail "missing ${CONFIG_FILE}; create it from the E1 instructions in infra/terraform/ADMINISTRACION.md"

# This file contains deployment configuration, never passwords. It is owned by
# root on the VM so the systemd service can consume the same values on reboot.
set -a
# shellcheck disable=SC1090
source "${CONFIG_FILE}"
set +a

GCP_PROJECT_ID="${GCP_PROJECT_ID:-plataforma-mooc-entrega2}"
S3_BUCKET="${S3_BUCKET:-plataforma-mooc-entrega2-media}"

# El bucket de los derivados, que es el unico de lectura publica (#167). Va
# aparte porque un prefijo publico dentro de un bucket privado no se puede
# expresar en IAM, y separarlos deja los originales con
# public_access_prevention = "enforced".
MEDIA_HLS_BUCKET="${MEDIA_HLS_BUCKET:-plataforma-mooc-entrega2-hls}"

# One worker per vCPU of the e2-highcpu-2 profile. Raising it past the core
# count does not transcode faster, it just risks the OOM killer on 2 GiB.
WORKER_CONCURRENCY="${WORKER_CONCURRENCY:-2}"
MEDIA_WORK_DIR="${MEDIA_WORK_DIR:-/tmp/mooc-media}"
MEDIA_MAX_ORIGINAL_MB="${MEDIA_MAX_ORIGINAL_MB:-2048}"

required=(IMAGE_TAG DB_PRIVATE_IP)
for name in "${required[@]}"; do
  [[ -n "${!name:-}" ]] || fail "${name} is empty in ${CONFIG_FILE}"
done

# The same tag the Web Server runs. A worker built from a different commit than
# the API can read a schema or an object layout the other does not write.
[[ "${IMAGE_TAG}" =~ ^[0-9a-f]{40}$ ]] ||
  fail "IMAGE_TAG must be the full 40-character commit SHA"
[[ "${DB_PRIVATE_IP}" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] ||
  fail "DB_PRIVATE_IP must be the Cloud SQL instance's private IPv4 address"
[[ "${WORKER_CONCURRENCY}" =~ ^[0-9]+$ ]] ||
  fail "WORKER_CONCURRENCY must be a number"

command -v gcloud >/dev/null || fail "gcloud is required on the VM"
command -v python3 >/dev/null || fail "python3 is required on the VM"

DB_PASSWORD="$(gcloud secrets versions access latest --secret=db-password --project="${GCP_PROJECT_ID}")"

# Keep this normalization aligned with trimspace(var.db_password) on the
# google_sql_user resource, and with prepare_web_env.sh. A secret created from
# PowerShell or Git Bash can retain a trailing carriage return even though
# command substitution removes the trailing line feed. If the two VMs disagree
# on the same secret, one of them gets 28P01 and the other does not.
DB_PASSWORD="$(DB_PASSWORD="${DB_PASSWORD}" python3 -c 'import os; print(os.environ["DB_PASSWORD"].strip(), end="")')"

[[ -n "${DB_PASSWORD}" ]] || fail "db-password has no usable value"

# A database password is part of a URI, so reserved characters must be encoded.
ENCODED_DB_PASSWORD="$(DB_PASSWORD="${DB_PASSWORD}" python3 -c 'import os, urllib.parse; print(urllib.parse.quote(os.environ["DB_PASSWORD"], safe=""))')"

umask 077
TMP_FILE="$(mktemp "${REPO_DIR}/.env.tmp.XXXXXX")"
trap 'rm -f "${TMP_FILE:-}"' EXIT

cat >"${TMP_FILE}" <<EOF
IMAGE_TAG='${IMAGE_TAG}'
APP_ENV='production'
DATABASE_URL='postgres://moocuser:${ENCODED_DB_PASSWORD}@${DB_PRIVATE_IP}:5432/moocdb?sslmode=require'
REDIS_URL='redis:6379'
STORAGE_BACKEND='gcs'
S3_BUCKET='${S3_BUCKET}'
MEDIA_HLS_BUCKET='${MEDIA_HLS_BUCKET}'
GCP_PROJECT_ID='${GCP_PROJECT_ID}'
WORKER_CONCURRENCY='${WORKER_CONCURRENCY}'
MEDIA_WORK_DIR='${MEDIA_WORK_DIR}'
MEDIA_MAX_ORIGINAL_MB='${MEDIA_MAX_ORIGINAL_MB}'
DB_MAX_OPEN_CONNS='25'
DB_MAX_IDLE_CONNS='25'
DB_CONN_MAX_LIFETIME='5m'
EOF

mv "${TMP_FILE}" "${ENV_FILE}"
chmod 600 "${ENV_FILE}"
trap - EXIT
unset DB_PASSWORD ENCODED_DB_PASSWORD

echo "Runtime environment prepared at ${ENV_FILE} (mode 600)."
