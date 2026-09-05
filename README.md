# Plataforma MOOC — Monorepo Setup

Plataforma web de Cursos Masivos Abiertos en Línea (MOOC) para el curso Cloud (ISIS4426).

---

##  Estructura del Monorepo

```
.
├── api/                   # Contratos y especificaciones OpenAPI 3.1 (/api/v1)
│   └── openapi.yaml
├── cmd/                   # Puntos de entrada ejecutables (main.go)
│   ├── api/               # Servidor HTTP API REST (/api/v1)
│   └── worker/            # Procesador de tareas asíncronas (Redis + Asynq)
├── docs/                  # Documentación del proyecto y guías arquitectónicas
│   ├── 2026-20 proyecto-plataforma-mooc (2).pdf
│   └── PROJECT_KEY_ASPECTS.md
├── internal/              # Código interno de la aplicación (Encapsulado)
│   ├── config/            # Carga de variables de entorno y configuración
│   ├── domain/            # Modelos del dominio y puertos (Desacoplado de HTTP y Cloud)
│   │   ├── badge.go
│   │   ├── course.go
│   │   ├── errors.go
│   │   ├── ports.go       # Interfaces de almacenamiento y colas
│   │   ├── progress.go
│   │   ├── quiz.go
│   │   └── user.go
│   ├── http/              # Adaptadores HTTP (handlers, router, middlewares)
│   │   ├── handler/
│   │   └── server.go
│   └── worker/            # Handlers de tareas asíncronas para el worker
│       ├── handler/
│       └── worker.go
├── migrations/            # Scripts de migración SQL para PostgreSQL
│   ├── 000001_init_schema.up.sql
│   └── 000001_init_schema.down.sql
├── scripts/               # Scripts de automatización y formateo/linting
│   └── lint.sh
├── Dockerfile.api         # Imagen Docker para el API Server
├── Dockerfile.worker      # Imagen Docker para el Background Worker
├── docker-compose.yml     # Infraestructura local (API, Worker, Postgres, Redis, MinIO, Mailpit)
├── Makefile               # Comandos de compilación, linteo y pruebas
├── go.mod                 # Módulo principal de Go
└── README.md
```

---

##  Desacoplamiento del Dominio (Restricción de la Sección 7)

El código dentro de `internal/domain/` implementa una arquitectura limpia (Hexagonal):
* **Cero dependencias** con frameworks HTTP (ej. `net/http`, `gin`, `fiber`).
* **Cero dependencias** con proveedores de nube o drivers de base de datos (ej. `aws-sdk-go`, `minio-go`, `pq`).
* Declara entidades del negocio (`User`, `Course`, `Quiz`, `Progress`, `Badge`) e interfaces de repositorio/puertos (`StorageProvider`, `TaskQueue`).

---

##  Cómo Correr el Proyecto Localmente

### Requisitos Previos
* **Go** 1.22+
* **Docker** y **Docker Compose**
* **GNU Make**

---

### Configuración de Variables de Entorno

Puedes copiar el archivo de ejemplo para configurar tus variables de entorno locales:
```bash
cp .env.example .env
```

### Option A: Ejecución Completa con Docker Compose (Recomendado)

Inicia todos los servicios (API, Worker, PostgreSQL, Redis, MinIO, Mailpit):

```bash
make docker-up
```
o directamente con Docker Compose:
```bash
docker compose up
```

Para detener los servicios:
```bash
make docker-down
```

Servicios disponibles:
* **API REST**: `http://localhost:8080/api/v1/health`
* **MinIO Console**: `http://localhost:9001` (User: `minioadmin` / Pass: `minioadmin`)
* **Mailpit (Email Testing)**: `http://localhost:8025`
* **PostgreSQL**: `localhost:5432` (`moocdb` / `moocuser` / `moocpassword`)
* **Redis**: `localhost:6379`

---

### Option B: Compilación y Ejecución Manual

1. **Compilar los ejecutables del API y Worker**:
   ```bash
   make build
   ```
   Los binarios se generarán en la carpeta `./bin/api` y `./bin/worker`.

2. **Correr el API localmente**:
   ```bash
   make run-api
   ```

3. **Correr el Worker localmente**:
   ```bash
   make run-worker
   ```

---

##  Comandos de Calidad y Verificación (`Makefile`)

El proyecto incluye un comando único para validar la integridad del código:

```bash
make check
```

El comando `make check` ejecuta secuencialmente:
1. `make fmt`: Verifica y corrige el formato de Go (`gofmt`).
2. `make vet`: Analiza posibles errores estáticos (`go vet`).
3. `make lint`: Corre el script `./scripts/lint.sh` (ejecuta `golangci-lint` si está instalado).
4. `make test`: Ejecuta todas las pruebas unitarias (`go test -v ./...`).
5. `make build`: Compila los binarios `bin/api` y `bin/worker`.

### Otros comandos útiles:
* `make lint` — Ejecuta únicamente la verificación de linteo y formato.
* `make test` — Ejecuta la suite de pruebas unitarias.
* `make clean` — Elimina los binarios compilados en `bin/`.

---

##  Contrato OpenAPI 3.1

El contrato oficial de la API v1 se encuentra en `api/openapi.yaml`. Puedes visualizarlo importándolo en la UI de Swagger o Postman.