# Evidencia D1 — Imágenes de despliegue

Esta carpeta debe contener únicamente salidas sanitizadas. Nunca incluir
contraseñas, tokens, llaves privadas, cookies, cabeceras `Authorization`,
archivos `.env` ni salidas completas de autenticación de `gcloud`.

## Criterios

| Evidencia | Criterio |
| :--- | :--- |
| `publicacion_por_commit.txt` | API y worker publicados en Artifact Registry con el SHA completo del commit y digest verificable. |
| `worker_ffmpeg.txt` | `ffmpeg -version` y `ffprobe -version` responden dentro de la imagen del worker. |
| `imagenes_sin_secretos.txt` | `docker history --no-trunc` y el contenido final no contienen secretos, `.env`, llaves, `.git` ni estado Terraform. |
| `despliegue_sin_build.txt` | Compose productivo hace `pull` de las imágenes y no construye en la VM. |

## Procedimiento reproducible

Desde la raíz del repositorio, después de integrar A3, A5 y A6:

```bash
terraform -chdir=infra/terraform fmt -check -recursive
terraform -chdir=infra/terraform validate
terraform -chdir=infra/terraform apply
./scripts/publish_images.sh
```

El publicador usa la identidad personal de `gcloud`, no una llave almacenada en
el repositorio. El tag es el SHA completo de `git rev-parse HEAD` y el
repositorio tiene `immutable_tags = true`, por lo que el mismo tag no puede
apuntar después a otro contenido.

## Comprobaciones que deben registrarse

```bash
IMAGE_TAG="$(git rev-parse HEAD)"
REGISTRY="us-east1-docker.pkg.dev/plataforma-mooc-entrega2/mooc"

gcloud artifacts docker images list "${REGISTRY}" --include-tags
docker run --rm --entrypoint ffmpeg "${REGISTRY}/worker:${IMAGE_TAG}" -version
docker run --rm --entrypoint ffprobe "${REGISTRY}/worker:${IMAGE_TAG}" -version
docker history --no-trunc "${REGISTRY}/api:${IMAGE_TAG}"
docker history --no-trunc "${REGISTRY}/worker:${IMAGE_TAG}"
IMAGE_TAG="${IMAGE_TAG}" docker compose -f docker-compose.prod.yml config
```

Al copiar las salidas a esta carpeta, conservar solo el SHA, los digests, las
versiones, tamaños y resultados `0`/`1` de las búsquedas. Sustituir cualquier
identificador sensible por `[REDACTED]`.