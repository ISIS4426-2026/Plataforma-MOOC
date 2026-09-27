#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_ID="${GCP_PROJECT_ID:-plataforma-mooc-entrega2}"
REGION="${GCP_REGION:-us-east1}"
REPOSITORY="${GCP_ARTIFACT_REGISTRY_REPOSITORY:-mooc}"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "error: publish from a clean commit; uncommitted changes are present" >&2
  exit 1
fi

COMMIT_SHA="$(git rev-parse HEAD)"
REGISTRY="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY}"

gcloud auth configure-docker "${REGION}-docker.pkg.dev" --quiet

docker buildx build \
  --platform linux/amd64 \
  --file Dockerfile.api \
  --tag "${REGISTRY}/api:${COMMIT_SHA}" \
  --push \
  .

docker buildx build \
  --platform linux/amd64 \
  --file Dockerfile.worker \
  --tag "${REGISTRY}/worker:${COMMIT_SHA}" \
  --push \
  .

cat <<EOF
Published immutable images for commit ${COMMIT_SHA}:
  ${REGISTRY}/api:${COMMIT_SHA}
  ${REGISTRY}/worker:${COMMIT_SHA}

Deploy with:
  IMAGE_TAG=${COMMIT_SHA} docker compose -f docker-compose.prod.yml pull
  IMAGE_TAG=${COMMIT_SHA} docker compose -f docker-compose.prod.yml up -d
EOF