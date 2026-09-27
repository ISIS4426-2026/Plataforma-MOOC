#!/usr/bin/env bash
set -Eeuo pipefail

# Builds the runtime .env used by docker-compose.prod.yml on the Web Server VM.
# Secret values are fetched with the VM service account and never printed.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="${MOOC_REPO_DIR:-$(cd "${SCRIPT_DIR}/.." && pwd)}"
CONFIG_FILE="${MOOC_WEB_CONFIG_FILE:-/etc/mooc/web.conf}"
ENV_FILE="${REPO_DIR}/.env"

fail() {
  echo "error: $*" >&2
  exit 1
}

[[ -r "${CONFIG_FILE}" ]] || fail "missing ${CONFIG_FILE}; create it from the F1 instructions"

# This file contains deployment configuration, never passwords. It is owned by
# root on the VM so the systemd service can consume the same values on reboot.
set -a
# shellcheck disable=SC1090
source "${CONFIG_FILE}"
set +a

GCP_PROJECT_ID="${GCP_PROJECT_ID:-plataforma-mooc-entrega2}"
SMTP_HOST="${SMTP_HOST:-smtp-relay.brevo.com}"
SMTP_PORT="${SMTP_PORT:-587}"
S3_BUCKET="${S3_BUCKET:-plataforma-mooc-entrega2-media}"
GCS_SIGNER_ACCOUNT="${GCS_SIGNER_ACCOUNT:-sa-web-server@${GCP_PROJECT_ID}.iam.gserviceaccount.com}"

required=(IMAGE_TAG APP_DOMAIN DB_PRIVATE_IP SMTP_FROM SMTP_USERNAME)
for name in "${required[@]}"; do
  [[ -n "${!name:-}" ]] || fail "${name} is empty in ${CONFIG_FILE}"
done

[[ "${IMAGE_TAG}" =~ ^[0-9a-f]{40}$ ]] ||
  fail "IMAGE_TAG must be the full 40-character commit SHA"
[[ "${SMTP_PORT}" =~ ^(587|465|2525)$ ]] ||
  fail "SMTP_PORT must be 587, 465, or 2525"

for name in APP_DOMAIN SMTP_FROM SMTP_USERNAME; do
  value="${!name}"
  [[ "${value}" != *$'\n'* && "${value}" != *$'\r'* && "${value}" != *"'"* ]] ||
    fail "${name} contains a character that cannot be written safely to .env"
done

command -v gcloud >/dev/null || fail "gcloud is required on the VM"
command -v python3 >/dev/null || fail "python3 is required on the VM"

DB_PASSWORD="$(gcloud secrets versions access latest --secret=db-password --project="${GCP_PROJECT_ID}")"
SMTP_PASSWORD="$(gcloud secrets versions access latest --secret=smtp-password --project="${GCP_PROJECT_ID}")"

[[ -n "${DB_PASSWORD}" ]] || fail "db-password has no usable value"
[[ -n "${SMTP_PASSWORD}" ]] || fail "smtp-password has no usable value"
[[ "${SMTP_PASSWORD}" != *$'\n'* && "${SMTP_PASSWORD}" != *$'\r'* && "${SMTP_PASSWORD}" != *"'"* ]] ||
  fail "smtp-password contains a character that cannot be written safely to .env"

# A database password is part of a URI, so reserved characters must be encoded.
ENCODED_DB_PASSWORD="$(DB_PASSWORD="${DB_PASSWORD}" python3 -c 'import os, urllib.parse; print(urllib.parse.quote(os.environ["DB_PASSWORD"], safe=""))')"

umask 077
TMP_FILE="$(mktemp "${REPO_DIR}/.env.tmp.XXXXXX")"
trap 'rm -f "${TMP_FILE:-}"' EXIT

cat >"${TMP_FILE}" <<EOF
IMAGE_TAG='${IMAGE_TAG}'
APP_ENV='production'
APP_DOMAIN='${APP_DOMAIN}'
APP_BASE_URL='https://${APP_DOMAIN}'
CSRF_ALLOWED_ORIGINS='https://${APP_DOMAIN}'
TRUSTED_PROXY_IP='172.30.0.2'
DATABASE_URL='postgres://moocuser:${ENCODED_DB_PASSWORD}@${DB_PRIVATE_IP}:5432/moocdb?sslmode=require'
REDIS_URL='redis:6379'
STORAGE_BACKEND='gcs'
S3_BUCKET='${S3_BUCKET}'
GCP_PROJECT_ID='${GCP_PROJECT_ID}'
GCS_SIGNER_ACCOUNT='${GCS_SIGNER_ACCOUNT}'
SMTP_HOST='${SMTP_HOST}'
SMTP_PORT='${SMTP_PORT}'
SMTP_FROM='${SMTP_FROM}'
SMTP_USERNAME='${SMTP_USERNAME}'
SMTP_PASSWORD='${SMTP_PASSWORD}'
DB_MAX_OPEN_CONNS='25'
DB_MAX_IDLE_CONNS='25'
DB_CONN_MAX_LIFETIME='5m'
EOF

mv "${TMP_FILE}" "${ENV_FILE}"
chmod 600 "${ENV_FILE}"
trap - EXIT
unset DB_PASSWORD ENCODED_DB_PASSWORD SMTP_PASSWORD

echo "Runtime environment prepared at ${ENV_FILE} (mode 600)."
