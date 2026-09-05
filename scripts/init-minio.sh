#!/bin/sh
set -e

MINIO_ENDPOINT="${S3_ENDPOINT:-http://minio:9000}"
MINIO_ACCESS_KEY="${MINIO_ROOT_USER:-minioadmin}"
MINIO_SECRET_KEY="${MINIO_ROOT_PASSWORD:-minioadmin}"
BUCKET_NAME="${S3_BUCKET:-mooc-storage}"

echo "Configuring MinIO client connection to ${MINIO_ENDPOINT}..."
until mc alias set local "${MINIO_ENDPOINT}" "${MINIO_ACCESS_KEY}" "${MINIO_SECRET_KEY}"; do
  echo "MinIO is not ready yet, retrying in 2 seconds..."
  sleep 2
done

echo "Ensuring initial bucket '${BUCKET_NAME}' exists..."
mc mb --ignore-existing "local/${BUCKET_NAME}"

echo "MinIO bootstrap completed successfully!"
