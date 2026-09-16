# Plataforma MOOC — Monorepo y Guía Maestra de Despliegue

[![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Docker Compose](https://img.shields.io/badge/Docker-Compose_v2-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16_Alpine-4169E1?style=flat&logo=postgresql)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7_Alpine-DC382D?style=flat&logo=redis)](https://redis.io/)
[![MinIO S3](https://img.shields.io/badge/MinIO-S3_Storage-C72C48?style=flat&logo=minio)](https://min.io/)
[![Mailpit](https://img.shields.io/badge/Mailpit-Email_Testing-FFA500?style=flat)](https://github.com/axllent/mailpit)
[![OpenAPI 3.1](https://img.shields.io/badge/OpenAPI-3.1-6BA539?style=flat&logo=openapiinitiative)](https://swagger.io/specification/)
[![Tests E2E](https://img.shields.io/badge/Tests_E2E-100%25_PASS_(216%2F216)-brightgreen?style=flat)](docs/e2e/REPORTE_BUGS_Y_CALIDAD_ETAPA.md)

Plataforma web de **Cursos Masivos Abiertos en Línea (MOOC)** diseñada para una única organización operadora, desarrollada bajo una arquitectura de **Monolito Modular en Go con Workers Asíncronos Independientes**.

Este repositorio contiene la implementación completa del backend, contratos OpenAPI 3.1, esquemas transaccionales de PostgreSQL, colas distribuidas Asynq/Redis, almacenamiento de objetos S3/MinIO, suite de observabilidad OpenTelemetry y colecciones automatizadas E2E en Postman/Newman.

** Video Sustentación :  **  https://www.youtube.com/watch?v=0qUeBiy9yfE

> [!TIP]
> **¿Es su primera vez con el proyecto?**  
> Siga la [Guía Rápida de Despliegue Paso a Paso](#guía-rápida-de-despliegue-paso-a-paso-desde-cero) para tener todo el sistema operativo con datos sintéticos y pruebas funcionales en menos de 3 minutos sin ayuda externa.  
> También puede consultar la [Guía Detallada de Despliegue y Operación](docs/GUIA_DE_DESPLIEGUE.md).

---

## Índice de Contenidos
1. [Propósito y Arquitectura del Sistema](#propósito-y-arquitectura-del-sistema)
2. [Estructura del Monorepo](#estructura-del-monorepo)
3. [Requisitos Previos](#requisitos-previos)
4. [Guía Rápida de Despliegue Paso a Paso (desde Cero)](#guía-rápida-de-despliegue-paso-a-paso-desde-cero)
5. [Carga y Gestión de Datos Sintéticos (`make seed`)](#carga-y-gestión-de-datos-sintéticos-make-seed)
6. [Importación y Ejecución de Colecciones Postman](#importación-y-ejecución-de-colecciones-postman)
7. [Observabilidad, Trazas y Métricas Prometheus](#observabilidad-trazas-y-métricas-prometheus)
8. [Pipeline de Calidad, Pruebas y Demos de la Etapa](#pipeline-de-calidad-pruebas-y-demos-de-la-etapa)
9. [Solución de Problemas Frecuentes (Troubleshooting)](#solución-de-problemas-frecuentes-troubleshooting)
10. [Índice de Documentación y Enlaces Oficiales](#índice-de-documentación-y-enlaces-oficiales)

---

## Propósito y Arquitectura del Sistema

La solución atiende un objetivo de escala inicial de hasta **50.000 usuarios registrados** y **2.000 usuarios concurrentes**, articulada en torno a tres roles globales y una jerarquía académica estricta:

* **Roles Globales:**
  * **Administrador (`administrador`):** Control administrativo de usuarios, roles, estados, sesiones revocables, auditoría inmutable y protección del último administrador activo.
  * **Profesor (`profesor`):** Autoría y estructuración de cursos, previsualización de borradores, validación de publicación y gestión de versiones. Creado exclusivamente por administradores (el registro público de profesores está prohibido).
  * **Estudiante (`estudiante`):** Autoregistro público con verificación de correo transaccional (Mailpit), inicio de sesión seguro, consumo de contenidos, presentación de quizzes con calificación en servidor y progreso validado.
* **Jerarquía Académica de 4 Niveles:**
  $$\text{Curso} \longrightarrow \text{Módulo} \longrightarrow \text{Unidad} \longrightarrow \text{Recurso}$$
* **Guardrails Inquebrantables de Arquitectura ([`docs/PROJECT_KEY_ASPECTS.md`](docs/PROJECT_KEY_ASPECTS.md)):**
  1. **Quiz Key Secrecy:** La clave de respuestas correctas NUNCA viaja al cliente; la evaluación ocurre 100% en el servidor.
  2. **Progreso Verificado en Servidor:** Rechazo y auditoría de porcentajes enviados por clientes; avance computado mediante permanencia y heartbeats.
  3. **Inmutabilidad de Versiones Publicadas:** Los cursos publicados no se pueden mutar (retornan `409 Conflict`); la edición en MVP requiere despublicación temporal.
  4. **Identificadores Estables (`stable_id`):** Preservan la continuidad del progreso ante reordenamientos y versiones actualizadas.
  5. **Cero Binarios en PostgreSQL:** Archivos multimedia y PDFs residen exclusivamente en MinIO/S3 y se acceden mediante URLs prefirmadas.

```
┌─────────────────┐       REST JSON / OpenAPI 3.1      ┌──────────────────────────────────┐
│  Cliente Web /  │ ─────────────────────────────────> │  API REST Go (Stateless)        │
│  Swagger / Post │                                    │  Puerto 8080                     │
└─────────────────┘                                    └──────────────────────────────────┘
         │                                                       │             │
         │ Direct Presigned Upload                               │             │ Encola tareas
         ▼                                                       ▼             ▼
┌─────────────────┐                                    ┌──────────────┐  ┌────────────────┐
│  MinIO (S3)     │                                    │ PostgreSQL 16│  │ Redis 7 (Asynq)│
│  Puerto 9000    │                                    │ Puerto 5432  │  │ Puerto 6379    │
└─────────────────┘                                    └──────────────┘  └────────────────┘
                                                                               ▲
                                                                               │ Consume
                                                       ┌───────────────────────┴──────────┐
                                                       │  Worker Go (Stateless)           │
                                                       │  Idempotencia / DLQ (Puerto 9090)│
                                                       └──────────────────────────────────┘
```

---

## Estructura del Monorepo

```
.
├── api/                       # Contratos y especificación OpenAPI 3.1 (/api/v1)
│   └── openapi.yaml
├── cmd/                       # Puntos de entrada ejecutables (main.go)
│   ├── api/                   # Servidor HTTP API REST
│   └── worker/                # Procesador de tareas asíncronas en background
├── docs/                      # Documentación arquitectónica, técnica y normativa
│   ├── 2026-20 proyecto...pdf# Especificación técnica oficial del curso
│   ├── DATABASE_DESIGN.md     # Modelo relacional y diagramas de base de datos
│   ├── DATOS_SINTETICOS.md    # Catálogo detallado de entidades y semillas sintéticas
│   ├── DOMINIO_ACADEMICO_IMPLEMENTACION.md # Autoría, jerarquía, auditoría y observabilidad: qué se hizo y por qué
│   ├── GUIA_DE_DESPLIEGUE.md  # Guía exhaustiva de despliegue paso a paso
│   ├── PLAN_DE_PRUEBAS_ETAPA.md # Plan de pruebas formal de la etapa (Sec. 6, 9 y 10.2)
│   ├── PROJECT_KEY_ASPECTS.md # Directrices y guardrails arquitectónicos no negociables
│   ├── e2e/                   # Evidencias y reportes de pruebas E2E
│   │   ├── DEMO_SEGMENTOS_1_Y_2.md # Runbook para sustentación en vivo
│   │   ├── REPORTE_BUGS_Y_CALIDAD_ETAPA.md # Certificación de Sección 10 y matriz de bugs
│   │   ├── REPORTE_E2E_IDENTIDAD_Y_AUTORIA.md # Reporte 100% PASS de Newman
│   │   └── evidencia/         # Respuestas JSON capturadas y logs de contenedores
│   └── postman/               # Colecciones Postman v2.1 y entornos parametrizados
│       ├── README.md          # Especificación de peticiones y aserciones Postman
│       ├── collection_admin.postman_collection.json
│       ├── collection_api.postman_collection.json
│       ├── collection_authoring.postman_collection.json
│       ├── mooc_docker.postman_environment.json
│       └── mooc_local.postman_environment.json
├── internal/                  # Código modular encapsulado (Hexagonal / Clean Architecture)
│   ├── admin/                 # Casos de uso de administración de usuarios y roles
│   ├── auth/                  # Casos de uso de autenticación, sesiones y recuperación
│   ├── config/                # Carga de variables de entorno y validación
│   ├── course/                # Casos de uso de autoría, jerarquía y validación de cursos
│   ├── domain/                # Entidades puras y puertos (desacoplado de HTTP y Cloud)
│   ├── http/                  # Adaptadores HTTP (handlers, router, middlewares, RBAC)
│   ├── mailer/                # Adaptador de envío SMTP para Mailpit
│   ├── observability/         # Tracing OpenTelemetry, métricas Prometheus y logs slog
│   ├── postgres/              # Adaptadores de persistencia relacional PostgreSQL
│   ├── structure/             # Gestión de módulos, unidades y recursos jerárquicos
│   └── worker/                # Adaptador del procesador asíncrono con Asynq
├── migrations/                # Scripts SQL de migración numerados (Up/Down reversibles)
├── scripts/                   # Scripts de automatización, validación, linting y semillas
│   ├── demo_segment4_idempotency.sh
│   ├── demo_segments_1_and_2.sh
│   ├── generate_e2e_report.go
│   ├── init-db.sh             # Aplicador automático de migraciones en PostgreSQL
│   ├── init-minio.sh          # Aprovisionador del bucket S3 en MinIO
│   ├── lint.sh                # Linter estático (Go fmt, vet, golangci-lint, Spectral)
│   ├── run_e2e_identity_authoring.sh # Ejecutor E2E y cosechador de evidencias
│   ├── seed.sh                # Script CLI de carga, limpieza y status de datos
│   ├── seeds/                 # Datos determinísticos en SQL
│   └── validate_stage.sh      # Suite integral de verificación de la etapa
├── Dockerfile.api             # Imagen multi-stage en Go 1.24 para API Server
├── Dockerfile.worker          # Imagen multi-stage en Go 1.24 para Worker
├── docker-compose.yml         # Pila completa de infraestructura multi-servicio
├── Makefile                   # Automatización de tareas de desarrollo y pruebas
├── go.mod                     # Dependencias del módulo Go
└── README.md                  # Este documento
```

---

## Requisitos Previos

Para ejecutar la plataforma localmente solo se requiere:
* **Docker Engine** 24.0+ y **Docker Compose** v2.20+ ([Instrucciones oficiales de Docker](https://docs.docker.com/get-docker/)).
* **GNU Make** (disponible por defecto en Linux/macOS; en Windows disponible mediante WSL2 o Chocolatey).
* *(Opcional)* **Postman Desktop** si desea ejecutar pruebas visuales interactivas.
* *(Opcional)* **Go 1.22+** únicamente si desea compilar o depurar binarios fuera de Docker.

---

## Guía Rápida de Despliegue Paso a Paso (desde Cero)

Siga estos 3 pasos exactos en su terminal para levantar el sistema completo:

### Paso 1: Clonar y Configurar Entorno
```bash
# 1. Clonar el repositorio
git clone https://github.com/ISIS4426-2026/Plataforma-MOOC.git
cd Plataforma-MOOC

# 2. Copiar variables de entorno por defecto (listas para usar sin cambios)
cp .env.example .env
```

### Paso 2: Construir y Levantar con Docker Compose
```bash
docker compose up --build -d
```
*(O simplemente: `make docker-up`)*

El sistema descargará las imágenes base, compilará el código de Go, aplicará automáticamente las migraciones en PostgreSQL, creará el bucket `mooc-storage` en MinIO y esperará a que todos los health checks estén saludables.

### Paso 3: Verificar Salud de los Servicios
Espere 10 segundos y compruebe que todos los contenedores reporten `(healthy)`:
```bash
docker compose ps
```

#### Servicios Disponibles de Inmediato en su Máquina Local:

| Servicio | URL Local | Descripción / Credenciales |
|---|---|---|
| **API REST (Salud)** | [`http://localhost:8080/api/v1/health`](http://localhost:8080/api/v1/health) | Endpoint de verificación rápida del backend. |
| **Documentación Swagger UI** | [`http://localhost:8080/api/docs`](http://localhost:8080/api/docs) | Interfaz visual interactiva OpenAPI 3.1 lista para "Try it out". |
| **Mailpit (Web UI)** | [`http://localhost:8025`](http://localhost:8025) | Bandeja de entrada visual de correos transaccionales. |
| **MinIO Console (S3)** | [`http://localhost:9001`](http://localhost:9001) | Consola de almacenamiento de objetos (`minioadmin` / `minioadmin`). |
| **Métricas API** | [`http://localhost:8080/api/v1/metrics`](http://localhost:8080/api/v1/metrics) | Métricas en formato estándar Prometheus. |
| **Métricas Worker** | [`http://localhost:9090/metrics`](http://localhost:9090/metrics) | Métricas Prometheus del procesador de tareas asíncronas. |
| **PostgreSQL 16** | `localhost:5432` | BD: `moocdb`, Usuario: `moocuser`, Contraseña: `moocpassword`. |
| **Redis 7** | `localhost:6379` | Almacén de sesiones, rate limiting y colas Asynq. |

---

## Carga y Gestión de Datos Sintéticos (`make seed`)

La plataforma incorpora un mecanismo de datos sintéticos determinísticos que permite poblar la base de datos con cuentas y cursos preconfigurados para pruebas funcionales.

### 1. Cargar Datos
Con los contenedores activos, ejecute:
```bash
make seed
```
*(O de forma directa: `./scripts/seed.sh --load`)*

### 2. Verificar Entidades Sembradas
```bash
make seed-status
```

### 3. Cuentas Preconfiguradas para Pruebas

> [!IMPORTANT]
> **Todos los usuarios de prueba comparten la contraseña:**  
> `Password123!`

| Rol | Correo Electrónico | Estado | Propósito de Prueba |
|---|---|:---:|---|
| **Administrador** | `admin@plataforma-mooc.test` | `active` | Gestión administrativa de usuarios, roles y consulta de auditoría. |
| **Administrador 2** | `admin.secundario@plataforma-mooc.test` | `active` | Verificación de protección del último administrador activo (409). |
| **Profesor** | `profesor1@plataforma-mooc.test` | `active` | Autor de cursos. Flujos de creación, jerarquía y publicación. |
| **Profesor 2** | `profesor2@plataforma-mooc.test` | `active` | Autor de cursos adicionales para aislamiento de permisos. |
| **Estudiante 1** | `estudiante1@plataforma-mooc.test` | `active` | Estudiante con inscripción activa y avance del 50%. |
| **Estudiante 2** | `estudiante2@plataforma-mooc.test` | `active` | Estudiante con 100% de avance e insignia digital emitida. |
| **Estudiante Pendiente** | `estudiante.pendiente@plataforma-mooc.test` | `pending_verification` | Valida rechazo de login previo a confirmación de correo (403). |
| **Estudiante Suspendido**| `estudiante.suspendido@plataforma-mooc.test` | `suspended` | Valida rechazo de login a cuentas suspendidas (403). |

### 4. Limpieza y Reinicio
* **Limpiar tablas y vaciar Redis:** `make seed-clean`
* **Reiniciar atómicamente al estado inicial:** `make seed-reset`

*(Consulte la documentación completa en [`docs/DATOS_SINTETICOS.md`](docs/DATOS_SINTETICOS.md))*.

---

## Importación y Ejecución de Colecciones Postman

La suite de pruebas Postman / Newman evalúa exhaustivamente el sistema con **97 casos de prueba únicos, 112 peticiones ejecutadas y 216 aserciones automáticas**.

### Opción A: Ejecución Automatizada desatendida con Newman en Docker (Recomendado)
**No requiere instalar nada en su máquina.** Se ejecuta dentro de la red de Docker Compose con un solo comando:

```bash
make test-postman
```

También puede correr colecciones de manera individual:
* `make test-postman-identity`: Identidad, verificación en Mailpit, login, logout, revocación de sesión, rate limiting (110 aserciones).
* `make test-postman-admin`: Administración de roles, suspensión, protección del último admin, auditoría (46 aserciones).
* `make test-postman-authoring`: Jerarquía de 4 niveles, ETag, validación multi-error 422, inmutabilidad 409, stable_id (60 aserciones).

### Opción B: Ejecución en Postman Desktop
1. Abra **Postman Desktop**.
2. Haga clic en **Import** y seleccione los archivos de la carpeta `docs/postman/`:
   * `collection_api.postman_collection.json`
   * `collection_admin.postman_collection.json`
   * `collection_authoring.postman_collection.json`
   * `mooc_local.postman_environment.json`
3. En la esquina superior derecha, seleccione el entorno: **`Plataforma MOOC - Local`**.
4. Abra una colección, entre a la pestaña **Runner** y presione **Run Collection**.
5. Las aserciones pasarán automáticamente al 100% (los scripts extraen tokens y correos de Mailpit sin intervención manual).

*(Consulte los detalles en [`docs/postman/README.md`](docs/postman/README.md))*.

---

## Observabilidad, Trazas y Métricas Prometheus

El sistema implementa observabilidad integral nativa con **OpenTelemetry** y logs JSON estructurados (`slog`):

* **Correlación Transversal de Peticiones:**  
  Cada petición HTTP genera o recibe una cabecera `X-Request-ID`. El middleware inyecta este identificador junto al `trace_id` de OpenTelemetry en cada registro de log.
* **Búsqueda Inmediata en Logs de Contenedor:**  
  Dado un `request_id` devuelto en las cabeceras de respuesta de la API, localice la traza completa con:
  ```bash
  docker compose logs api | grep <request_id>
  ```
* **Métricas Prometheus para Scrapeo:**
  * **API REST:** `GET /api/v1/metrics` (latencia `http.server.request.duration`, errores `http.server.request.errors`, saturación).
  * **Worker Engine:** `GET :9090/metrics` (`worker.jobs.processed`, `worker.jobs.failed`, latencias de procesamiento).

---

## Pipeline de Calidad, Pruebas y Demos de la Etapa

El archivo `Makefile` provee comandos estandarizados para asegurar la calidad de la plataforma:

| Comando | Acción Realizada |
|---|---|
| **`make check`** | Pipeline local completo: formato (`fmt`), análisis estático (`vet`), linters (`lint`), pruebas unitarias (`test`), reversibilidad de migraciones (`test-migrations`) y compilación de binarios (`build`). |
| **`make test-stage`** | Suite automatizada integral de la etapa: ejecuta análisis estático, migraciones, pruebas de dominio, workers con DLQ y las 3 colecciones Newman en red Docker. |
| **`make test-e2e`** | Ejecuta la batería completa E2E contra el sistema desplegado, cosecha respuestas JSON crudas, logs y regenera el informe Markdown. |
| **`make demo-segments-1-2`** | Ejecuta la **Demostración Interactiva en Vivo** de Identidad y Autoría (Segmentos 1 y 2 de la Sección 10.2). |
| **`make demo-segment4`** | Ejecuta la **Demostración Automatizada del Worker** (Segmento 4: idempotencia ante doble entrega, reintentos con backoff y paso a DLQ con alertas). |
| **`make docker-down`** | Detiene todos los contenedores de Docker Compose de forma limpia. |

---

## Solución de Problemas Frecuentes (Troubleshooting)

* **Puerto ocupado en el host (ej. `port 8080: bind: address already in use`):**  
  Modifique el puerto afectado en su archivo `.env` (ej. `API_PORT=8090` o `POSTGRES_PORT=5433`) y vuelva a ejecutar `docker compose up -d`.
* **Bloqueo por Rate Limiting (`429 Too Many Requests`) durante pruebas repetitivas:**  
  Vacíe las claves de rate limit en Redis con:  
  `docker compose exec redis redis-cli EVAL "for _,k in ipairs(redis.call('keys','ratelimit:*')) do redis.call('del',k) end" 0`
* **Limpiar el estado completo de la base de datos:**  
  Ejecute `make seed-reset` para restaurar los datos determinísticos o `docker compose down -v` para recrear los volúmenes de almacenamiento desde cero.

---

## Índice de Documentación y Enlaces Oficiales

* [Guía Detallada de Despliegue y Operación (`docs/GUIA_DE_DESPLIEGUE.md`)](docs/GUIA_DE_DESPLIEGUE.md): Manual paso a paso para personas que no participaron del desarrollo.
* [Directrices Clave de Arquitectura (`docs/PROJECT_KEY_ASPECTS.md`)](docs/PROJECT_KEY_ASPECTS.md): Reglas no negociables y guardrails de seguridad.
* [Diseño de Base de Datos (`docs/DATABASE_DESIGN.md`)](docs/DATABASE_DESIGN.md): Esquema relacional, diagrama ER y política de ordenamiento sin colisiones.
* [Dominio Académico, Auditoría y Observabilidad — Implementación (`docs/DOMINIO_ACADEMICO_IMPLEMENTACION.md`)](docs/DOMINIO_ACADEMICO_IMPLEMENTACION.md): Qué se construyó y por qué para la autoría de cursos, su jerarquía interna, la auditoría inmutable y la observabilidad básica (issues #15–#21).
* [Plan Maestro de Pruebas de la Etapa (`docs/PLAN_DE_PRUEBAS_ETAPA.md`)](docs/PLAN_DE_PRUEBAS_ETAPA.md): Mapeo normativo Secciones 6, 9 y 10.2.
* [Reporte de Aseguramiento de Calidad y Certificación Sección 10 (`docs/e2e/REPORTE_BUGS_Y_CALIDAD_ETAPA.md`)](docs/e2e/REPORTE_BUGS_Y_CALIDAD_ETAPA.md): Certificación formal de 100% pass y ausencia de bugs críticos.
* [Guía de Demostración para Evaluadores: Segmentos 1 y 2 (`docs/e2e/DEMO_SEGMENTOS_1_Y_2.md`)](docs/e2e/DEMO_SEGMENTOS_1_Y_2.md): Runbook interactivo para sustentar la entrega.
* [Guion Técnico y Libreto del Video de Demostración (`docs/e2e/README_GUION_VIDEO_DEMO.md`)](docs/e2e/README_GUION_VIDEO_DEMO.md): Guion completo para grabación del video cubriendo los 9 segmentos de la Sección 10.2.
* [Catálogo de Datos Sintéticos (`docs/DATOS_SINTETICOS.md`)](docs/DATOS_SINTETICOS.md): Semillas determinísticas, credenciales y cursos.
* [Especificación de Colecciones Postman (`docs/postman/README.md`)](docs/postman/README.md): Detalle técnico de las 97 peticiones y 216 aserciones.
