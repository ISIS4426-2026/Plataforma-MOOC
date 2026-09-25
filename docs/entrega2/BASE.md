# Base congelada de la Entrega 2

## Commit base

| | |
| :--- | :--- |
| **Rama** | `main` |
| **Commit** | `4fe7284` — Add video link and deployment guides to README |
| **Fecha de congelación** | 2026-09-24 |
| **Tag** | `entrega-2-base` |

Todo el trabajo de la Entrega 2 parte de este commit. Las ramas de cada issue se crean
desde `main` y se integran por PR, como en la Entrega 1.

## Verificación de CI

El pipeline `CI Pipeline` pasó sobre este commit exacto:

| | |
| :--- | :--- |
| **Run** | [35062713844](https://github.com/ISIS4426-2026/Plataforma-MOOC/actions/runs/35062713844) |
| **Resultado** | `success` |
| **Duración** | 1m18s |
| **Fecha** | 2026-09-16T06:12:54Z |

El pipeline cubre lint (`gofmt`, `go vet`, `golangci-lint`, Spectral sobre el OpenAPI),
build de `cmd/api` y `cmd/worker`, migraciones sobre una base limpia, pruebas unitarias
y `make check`.

## Verificación local

`go build ./...`, `go vet ./...` y `go test ./...` pasan sobre el commit base con los
servicios de Docker Compose activos (`docker compose up -d redis postgres`):

**21 paquetes, 0 fallos.** Con cobertura de pruebas: `auth`, `course`, `domain`,
`http/handler`, `http/middleware`, `observability`, `postgres`, `structure`, `worker`,
`worker/handler`, `worker/task`, `migrations`.

Las pruebas de `internal/worker` necesitan Redis y las de `internal/postgres` y
`migrations` necesitan PostgreSQL. Sin Docker activo fallan 8 pruebas de `internal/worker`
con `dial tcp [::1]:6379: connection refused`; no es un defecto del código.

## Alcance de la retroalimentación

El enunciado de la Entrega 2 pide partir de la base de la Entrega 1 «con las
correcciones de la retroalimentación aplicadas». **No se recibió retroalimentación de
la Entrega 1**, así que no hay correcciones que incorporar. La base congelada es el
estado con el que se cerró la Entrega 1, sin cambios.
