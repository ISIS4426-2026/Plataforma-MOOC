# Plataforma MOOC — Monorepo y Guía Maestra de Despliegue

[![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Docker Compose](https://img.shields.io/badge/Docker-Compose_v2-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16_Alpine-4169E1?style=flat&logo=postgresql)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7_Alpine-DC382D?style=flat&logo=redis)](https://redis.io/)
[![MinIO S3](https://img.shields.io/badge/MinIO-S3_local-C72C48?style=flat&logo=minio)](https://min.io/)
[![Mailpit](https://img.shields.io/badge/Mailpit-correo_local-FFA500?style=flat)](https://github.com/axllent/mailpit)
[![OpenAPI 3.1](https://img.shields.io/badge/OpenAPI-3.1-6BA539?style=flat&logo=openapiinitiative)](https://swagger.io/specification/)
[![Google Cloud](https://img.shields.io/badge/Google_Cloud-us--east1-4285F4?style=flat&logo=googlecloud&logoColor=white)](https://cloud.google.com/)
[![Terraform](https://img.shields.io/badge/Terraform-1.16-7B42BC?style=flat&logo=terraform)](https://www.terraform.io/)
[![E2E en la nube](https://img.shields.io/badge/E2E_en_la_nube-61%2F61-brightgreen?style=flat)](docs/entrega2/evidencias/G3/README.md)
[![Colecciones Postman](https://img.shields.io/badge/Postman-202_peticiones_%C2%B7_413_aserciones-FF6C37?style=flat&logo=postman&logoColor=white)](docs/postman/README.md)

Plataforma web de **Cursos Masivos Abiertos en Línea (MOOC)** diseñada para una única organización operadora, desarrollada bajo una arquitectura de **Monolito Modular en Go con Workers Asíncronos Independientes**.

Este repositorio contiene la implementación completa del backend, contratos OpenAPI 3.1, esquemas transaccionales de PostgreSQL, colas distribuidas Asynq/Redis, almacenamiento de objetos, suite de observabilidad OpenTelemetry y colecciones automatizadas E2E en Postman/Newman.

Desde la **Entrega 2** incluye además el despliegue en **Google Cloud Platform**: infraestructura como código con Terraform, dos máquinas virtuales, base de datos administrada, almacenamiento de objetos gestionado y el análisis de capacidad de la plataforma desplegada.

- **Video de sustentación de la Entrega 1:** <https://www.youtube.com/watch?v=0qUeBiy9yfE>
- **Video de sutentación de la Entrega 2 (Analisis-pruebas):** <https://youtu.be/tn9JgmWbn_M>
- **Video corto de flujo happy path postman-gcp de Entrega 2 multimedia:** <https://www.youtube.com/watch?v=Zh9pefwJNEA>

---

## Entrega 2 — Despliegue en la nube

### Los entregables del enunciado, y qué documento satisface cada uno

| | Entregable | Dónde está |
| :---: | :--- | :--- |
| 1 | **Plataforma desplegada en la nube pública** | <https://34.24.52.111.sslip.io> · [documentación de la API](https://34.24.52.111.sslip.io/api/docs) · verificada de punta a punta en [`evidencias/G3/`](docs/entrega2/evidencias/G3/README.md) |
| 2 | **Release del código y la configuración** | Tag `entrega-2` sobre el commit evaluado · infraestructura declarada en [`infra/terraform/`](infra/terraform/README.md) |
| 3 | **Documento de arquitectura** | [`docs/entrega2/ARQUITECTURA.md`](docs/entrega2/ARQUITECTURA.md) — componentes, despliegue y decisiones<br/>[`docs/entrega2/OPERACION_Y_CAPACIDAD.md`](docs/entrega2/OPERACION_Y_CAPACIDAD.md) — operación, recuperación, capacidad, costos y limitaciones |
| 4 | **Informe de capacidad** | [`capacity-planning/pruebas_de_carga_entrega2.md`](capacity-planning/pruebas_de_carga_entrega2.md) |

El enunciado pide **cinco secciones** en el documento de arquitectura. Las tres
primeras —modelo de componentes, modelo de despliegue, decisiones y
adaptaciones— están en `ARQUITECTURA.md`; las dos últimas —operación y
recuperación, capacidad, costo y limitaciones— en `OPERACION_Y_CAPACIDAD.md`.
Por eso el documento de arquitectura se reparte entre dos archivos y no uno.

### Los demás documentos de la entrega

| Documento | Para qué sirve |
| :--- | :--- |
| [`docs/entrega2/CONFIGURACION_Y_COSTOS.md`](docs/entrega2/CONFIGURACION_Y_COSTOS.md) | Proveedor, región, perfiles de VM y por qué se eligieron; estimación fechada, presupuesto, alertas y política de encendido y apagado |
| [`docs/entrega2/EJECUCION_PRUEBAS_MULTIMEDIA.md`](docs/entrega2/EJECUCION_PRUEBAS_MULTIMEDIA.md) | Paso a paso para correr la colección de multimedia: carga directa al bucket, confirmación, idempotencia e inmutabilidad |
| [`docs/GUIA_DE_DESPLIEGUE.md`](docs/GUIA_DE_DESPLIEGUE.md) | Levantar el entorno local paso a paso y, en §9, el ciclo de despliegue en la nube |
| [`infra/terraform/README.md`](infra/terraform/README.md) | Aprovisionar la infraestructura y cómo trabaja el equipo sobre el mismo estado sin pisarse |
| [`infra/terraform/ADMINISTRACION.md`](infra/terraform/ADMINISTRACION.md) | Configurar las VMs, rotar secretos y recrear el entorno completo |
| [`docs/DATOS_SINTETICOS.md`](docs/DATOS_SINTETICOS.md) | Catálogo de los datos sembrados: cuentas, cursos, inscripciones y avance |
| [`docs/postman/README.md`](docs/postman/README.md) | Las siete colecciones, petición por petición, y cómo correrlas contra local o contra la nube |
| [`docs/entrega2/evidencias/`](docs/entrega2/evidencias/) | Toda la evidencia, una carpeta por issue |

### Dónde se sustenta cada criterio de evaluación

| Criterio | Sustento |
| :--- | :--- |
| Despliegue e integración de componentes | [`ARQUITECTURA.md`](docs/entrega2/ARQUITECTURA.md) §2 y §3 · evidencias [`D2`](docs/entrega2/evidencias/D2/README.md), [`D3`](docs/entrega2/evidencias/D3/README.md), [`E1`](docs/entrega2/evidencias/E1/README.md), [`C4`](docs/entrega2/evidencias/C4/README.md) |
| Servicio administrado de base de datos | [`C1`](docs/entrega2/evidencias/C1/README.md) y [`C2`](docs/entrega2/evidencias/C2/README.md) — instancia privada, respaldo y restauración |
| Funcionamiento y configuración de red | [`G3`](docs/entrega2/evidencias/G3/README.md) — 61 de 61 pasos · [`G4`](docs/entrega2/evidencias/G4/) y [`B3`](docs/entrega2/evidencias/B3/DIAGRAMA_RED.md) — red y firewall |
| Análisis de capacidad · escenario 1 | [Informe](capacity-planning/pruebas_de_carga_entrega2.md) §1 · evidencias [`H2`](docs/entrega2/evidencias/H2/) y [`H3`](docs/entrega2/evidencias/H3/README.md) |
| Análisis de capacidad · escenario 2 | [Informe](capacity-planning/pruebas_de_carga_entrega2.md) §2 · evidencias [`H4`](docs/entrega2/evidencias/H4/README.md) y [`H5`](docs/entrega2/evidencias/H5/) |
| Documentación de arquitectura | Los dos documentos de la fila 3 de arriba |

El dominio es `sslip.io` sobre la IPv4 estática reservada del Web Server, así que
la URL **sobrevive a que se recree la máquina**. El equipo no compró dominio
propio; la decisión y su efecto en el correo saliente están en el documento de
configuración y costos.

> [!IMPORTANT]
> **El entorno se apaga cuando no se está usando**, por la política de control de
> costos que el propio enunciado pide. Si la URL no responde, no está caída: está
> apagada. El procedimiento para encenderla y, si hiciera falta, para reconstruir
> el entorno completo desde cero, está en
> [`infra/terraform/ADMINISTRACION.md`](infra/terraform/ADMINISTRACION.md).

---

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
  * **Estudiante (`estudiante`):** Autoregistro público con verificación de correo transaccional (Mailpit en local, proveedor SMTP real en la nube), inicio de sesión seguro, consumo de contenidos, presentación de quizzes con calificación en servidor y progreso validado.
* **Jerarquía Académica de 4 Niveles:**
  $$\text{Curso} \longrightarrow \text{Módulo} \longrightarrow \text{Unidad} \longrightarrow \text{Recurso}$$
* **Guardrails Inquebrantables de Arquitectura ([`docs/PROJECT_KEY_ASPECTS.md`](docs/PROJECT_KEY_ASPECTS.md)):**
  1. **Quiz Key Secrecy:** La clave de respuestas correctas NUNCA viaja al cliente; la evaluación ocurre 100% en el servidor.
  2. **Progreso Verificado en Servidor:** Rechazo y auditoría de porcentajes enviados por clientes; avance computado mediante permanencia y heartbeats.
  3. **Inmutabilidad de Versiones Publicadas:** Los cursos publicados no se pueden mutar (retornan `409 Conflict`); la edición en MVP requiere despublicación temporal.
  4. **Identificadores Estables (`stable_id`):** Preservan la continuidad del progreso ante reordenamientos y versiones actualizadas.
  5. **Cero Binarios en PostgreSQL:** Archivos multimedia y PDFs residen exclusivamente en el almacenamiento de objetos —MinIO en local, Cloud Storage en la nube— y se acceden mediante URLs prefirmadas.

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

### La arquitectura desplegada es otra

El diagrama de arriba es el **entorno local**: un solo host, con MinIO y Mailpit
como sustitutos de servicios que en la nube son administrados. Desde la Entrega 2
la plataforma corre además sobre GCP, y ahí el reparto cambia:

| | Local (Docker Compose) | Nube (GCP, `us-east1`) |
| :--- | :--- | :--- |
| Cómputo | Un host, todos los contenedores | **Dos VMs**: `mooc-web-server` y `mooc-worker-server` |
| Base de datos | Contenedor PostgreSQL 16 | **Cloud SQL**, sin IP pública, alcanzable solo desde la VPC |
| Objetos | MinIO | **Dos buckets de Cloud Storage**: uno privado para originales, uno público para los derivados HLS |
| Cola | Redis en el mismo host | Redis en el **Worker Server**, alcanzable por la red privada |
| Correo | Mailpit | Proveedor SMTP real, con STARTTLS |
| Entrada | `http://localhost:8080` | HTTPS con certificado, solo la VM web acepta tráfico de internet |

**El modelo de componentes y el de despliegue completos, con sus diagramas y las
decisiones que los explican, están en
[`docs/entrega2/ARQUITECTURA.md`](docs/entrega2/ARQUITECTURA.md).** Esta sección
solo sitúa la diferencia para que nadie tome el diagrama local por el desplegado.

---

## Estructura del Monorepo

```
.
├── api/                       # Contrato OpenAPI 3.1 de /api/v1
├── capacity-planning/         # Análisis de capacidad de la Entrega 2
│   ├── pruebas_de_carga_entrega2.md  # Informe oficial consolidado
│   ├── escenario1.md          # Plan del escenario académico
│   └── escenario2.md          # Plan del escenario multimedia
├── cmd/                       # Puntos de entrada ejecutables
│   ├── api/                   # Servidor HTTP
│   ├── worker/                # Procesador asíncrono
│   └── seed-media/            # Generador del conjunto multimedia de prueba
├── docs/
│   ├── entrega2/              # Documentación de la Entrega 2
│   │   ├── ARQUITECTURA.md    # Documento de arquitectura desplegada (I1)
│   │   ├── OPERACION_Y_CAPACIDAD.md   # Operación, recuperación, costos y límites (I2)
│   │   ├── CONFIGURACION_Y_COSTOS.md  # Proveedor, perfiles, estimación y presupuesto
│   │   ├── NOTAS_TECNICAS.md  # Hallazgos transversales, con el issue que resuelve cada uno
│   │   └── evidencias/        # Evidencia por issue (A5 … I3)
│   ├── e2e/                   # Reportes y evidencias E2E de la Entrega 1
│   ├── postman/               # Colecciones v2.1 y los tres entornos
│   │   ├── mooc_cloud.postman_environment.json   # Contra el despliegue en GCP
│   │   ├── mooc_docker.postman_environment.json  # Contra Compose
│   │   └── mooc_local.postman_environment.json   # Contra binarios locales
│   ├── DATABASE_DESIGN.md · DATOS_SINTETICOS.md · PROJECT_KEY_ASPECTS.md
│   └── GUIA_DE_DESPLIEGUE.md  # Despliegue local paso a paso, y ruta a la nube
├── infra/                     # Infraestructura de la Entrega 2
│   ├── terraform/             # Aprovisionamiento declarativo de GCP
│   │   ├── network.tf · database.tf · storage.tf · compute.tf · mail.tf
│   │   ├── README.md          # Cómo trabaja el equipo sobre el mismo estado
│   │   └── ADMINISTRACION.md  # Tareas de una vez, operación de las VMs y recreación
│   └── ops-agent/             # Configuración del agente: métricas y logs de contenedores
├── internal/                  # Código modular (puertos y adaptadores)
│   ├── domain/                # Entidades y reglas; sin dependencias de infraestructura
│   ├── admin/ · auth/ · course/ · structure/   # Identidad y autoría
│   ├── enrollment/ · quiz/ · progress/         # Inscripción, evaluación y avance
│   ├── media/ · transcode/                     # Carga directa y derivados HLS
│   ├── http/                  # Router, handlers y los middlewares (CSRF, límites, idempotencia)
│   ├── postgres/ · cache/ · storage/ · mailer/ # Adaptadores: base, Redis, objetos y SMTP
│   ├── observability/         # Trazas, métricas y logs estructurados
│   ├── config/                # Carga y validación de la configuración de arranque
│   └── worker/                # Consumidor de la cola asynq
├── migrations/                # Migraciones SQL numeradas y reversibles
├── scripts/
│   ├── e2e_cloud/             # Recorrido E2E continuo contra la nube (make test-e2e-cloud)
│   ├── prepare_web_env.sh     # Genera el .env del Web Server desde Secret Manager
│   ├── prepare_worker_env.sh  # Ídem para el Worker Server
│   ├── publish_images.sh      # Publica las imágenes con el SHA del commit
│   ├── migrate.sh             # Aplica el esquema contra Cloud SQL
│   ├── test_postman_cloud.sh  # Las siete colecciones contra el despliegue
│   ├── h2_nube.sh · h3_nube.sh · run_escenario1.sh   # Campañas de carga
│   ├── capacity_escenario2/ · pilot_escenario2/      # Motor del escenario 2
│   ├── seed.sh · seeds/       # Datos sintéticos
│   └── lint.sh · validate_stage.sh · verify_g4.sh    # Calidad y verificación
├── docker-compose.yml         # Entorno local completo (incluye MinIO y Mailpit)
├── docker-compose.prod.yml    # Web Server en la nube
├── docker-compose.worker.yml  # Worker Server en la nube
└── nginx.conf                 # Proxy inverso con TLS del despliegue
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
| **MinIO (API S3)** | `http://localhost:9000` | Almacenamiento de objetos (`minioadmin` / `minioadmin`). **Sin consola web:** MinIO la retiró del servidor comunitario. Para inspeccionar el bucket use `mc` — ver [`docs/entrega2/EJECUCION_PRUEBAS_MULTIMEDIA.md`](docs/entrega2/EJECUCION_PRUEBAS_MULTIMEDIA.md). |
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

La suite de pruebas Postman / Newman evalúa exhaustivamente el sistema con **202 peticiones y 413 aserciones automáticas** repartidas en siete colecciones.

### Opción A: Ejecución Automatizada desatendida con Newman en Docker (Recomendado)
**No requiere instalar nada en su máquina.** Se ejecuta dentro de la red de Docker Compose con un solo comando:

```bash
make test-postman
```

También puede correr colecciones de manera individual:
* `make test-postman-identity`: Identidad, verificación en Mailpit, login, logout, revocación de sesión, rate limiting (110 aserciones).
* `make test-postman-admin`: Administración de roles, suspensión, protección del último admin, auditoría (46 aserciones).
* `make test-postman-authoring`: Jerarquía de 4 niveles, ETag, validación multi-error 422, inmutabilidad 409, stable_id (60 aserciones).
* `make test-postman-enrollments`: Inscripción, retiro, reinscripción conservando el registro, y el control de acceso al contenido (36 aserciones).
* `make test-postman-progress`: Latidos de progreso, avance que no se infla al repetirlos, insignia al completar, GET condicional con ETag y verificación pública sin sesión (64 aserciones).
* `make test-postman-quizzes`: Autoría de cuestionarios, calificación en servidor, límite de intentos, envío idempotente y la clave de respuestas que nunca llega al estudiante (56 aserciones).
* `make test-postman-media`: Carga directa al almacenamiento de objetos, confirmación idempotente, URLs firmadas, inmutabilidad 409 (43 aserciones).

### Opción B: Ejecución en Postman Desktop
1. Abra **Postman Desktop**.
2. Haga clic en **Import** y seleccione los archivos de la carpeta `docs/postman/`:
   * `collection_api.postman_collection.json`
   * `collection_admin.postman_collection.json`
   * `collection_authoring.postman_collection.json`
   * `collection_enrollments.postman_collection.json`
   * `collection_progress.postman_collection.json`
   * `collection_quizzes.postman_collection.json`
   * `collection_media.postman_collection.json`
   * `mooc_local.postman_environment.json`
3. En la esquina superior derecha, seleccione el entorno: **`Plataforma MOOC - Local`**.
4. Abra una colección, entre a la pestaña **Runner** y presione **Run Collection**.
5. Las aserciones pasarán automáticamente al 100% (los scripts extraen tokens y correos de Mailpit sin intervención manual).

> **Para la colección de multimedia, cualquier cliente que corra en su máquina** —Postman Desktop, o
> newman instalado por npm— necesita `127.0.0.1 minio` en su archivo de hosts. Lo que decide es desde
> dónde corre, no qué cliente use: la URL prefirmada lleva el host dentro de la firma, así que
> `minio:9000` también tiene que resolver en el anfitrión; el puerto ya está publicado. Sin esa entrada
> fallan las 7 peticiones de carga directa y el resto pasa. No hace falta cuando newman corre dentro de
> la red de Compose, como en la Opción A.
> Paso a paso en [`docs/entrega2/EJECUCION_PRUEBAS_MULTIMEDIA.md`](docs/entrega2/EJECUCION_PRUEBAS_MULTIMEDIA.md).

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

* [Guía Detallada de Despliegue y Operación (`docs/GUIA_DE_DESPLIEGUE.md`)](docs/GUIA_DE_DESPLIEGUE.md): Manual paso a paso para personas que no participaron del desarrollo. Cubre el entorno **local** con Docker Compose y, en su §9, la **ruta del despliegue en la nube**: cómo comprobar que está viva, cómo se despliega un cambio y cómo probar contra ella.
* [Directrices Clave de Arquitectura (`docs/PROJECT_KEY_ASPECTS.md`)](docs/PROJECT_KEY_ASPECTS.md): Reglas no negociables y guardrails de seguridad.
* [Diseño de Base de Datos (`docs/DATABASE_DESIGN.md`)](docs/DATABASE_DESIGN.md): Esquema relacional, diagrama ER y política de ordenamiento sin colisiones.
* [Dominio Académico, Auditoría y Observabilidad — Implementación (`docs/DOMINIO_ACADEMICO_IMPLEMENTACION.md`)](docs/DOMINIO_ACADEMICO_IMPLEMENTACION.md): Qué se construyó y por qué para la autoría de cursos, su jerarquía interna, la auditoría inmutable y la observabilidad básica (issues #15–#21).
* [Plan Maestro de Pruebas de la Etapa (`docs/PLAN_DE_PRUEBAS_ETAPA.md`)](docs/PLAN_DE_PRUEBAS_ETAPA.md): Mapeo normativo Secciones 6, 9 y 10.2.
* [Reporte de Aseguramiento de Calidad y Certificación Sección 10 (`docs/e2e/REPORTE_BUGS_Y_CALIDAD_ETAPA.md`)](docs/e2e/REPORTE_BUGS_Y_CALIDAD_ETAPA.md): Certificación formal de 100% pass y ausencia de bugs críticos.
* [Guía de Demostración para Evaluadores: Segmentos 1 y 2 (`docs/e2e/DEMO_SEGMENTOS_1_Y_2.md`)](docs/e2e/DEMO_SEGMENTOS_1_Y_2.md): Runbook interactivo para sustentar la entrega.
* [Guion Técnico y Libreto del Video de Demostración (`docs/e2e/README_GUION_VIDEO_DEMO.md`)](docs/e2e/README_GUION_VIDEO_DEMO.md): Guion completo para grabación del video cubriendo los 9 segmentos de la Sección 10.2.
* [Catálogo de Datos Sintéticos (`docs/DATOS_SINTETICOS.md`)](docs/DATOS_SINTETICOS.md): Semillas determinísticas, credenciales y cursos.
* [Especificación de Colecciones Postman (`docs/postman/README.md`)](docs/postman/README.md): Detalle técnico de las 202 peticiones y 413 aserciones, colección por colección.
* [Infraestructura como Código (`infra/terraform/README.md`)](infra/terraform/README.md): Aprovisionamiento con Terraform, estado remoto compartido y **cómo trabaja el equipo sobre la misma infraestructura sin conflictos**. Instalación para Windows, macOS y Linux.
* [Administración del Proyecto de GCP (`infra/terraform/ADMINISTRACION.md`)](infra/terraform/ADMINISTRACION.md): Tareas de una sola vez — bootstrap del estado remoto, altas y bajas de integrantes.
* [**Arquitectura en la Nube — Entrega 2** (`docs/entrega2/ARQUITECTURA.md`)](docs/entrega2/ARQUITECTURA.md): **Documento de arquitectura de la entrega.** Correspondencia con los servicios de GCP, modelo de componentes, modelo de despliegue, decisiones y adaptaciones, y diferencias frente a la arquitectura objetivo. Los diagramas son Mermaid y su bloque de código es el archivo fuente.
* [**Operación, Recuperación, Capacidad, Costos y Limitaciones** (`docs/entrega2/OPERACION_Y_CAPACIDAD.md`)](docs/entrega2/OPERACION_Y_CAPACIDAD.md): El documento que se abre cuando hay que **levantar, mantener, recuperar o evaluar** el entorno. Escrito para quien no participó en el despliegue: qué existe, cómo se reconstruye, qué cuesta y qué no se pudo medir.
* [**Informe Consolidado de Pruebas de Carga y Capacidad** (`capacity-planning/pruebas_de_carga_entrega2.md`)](capacity-planning/pruebas_de_carga_entrega2.md): **Informe oficial de capacidad.** Análisis integral de los dos escenarios (actividad académica concurrente y procesamiento/streaming multimedia), caracterización de niveles, sustentación de cuellos de botella y evolución respaldada por mediciones.
* [Configuración Efectiva y Marco de Costos (`docs/entrega2/CONFIGURACION_Y_COSTOS.md`)](docs/entrega2/CONFIGURACION_Y_COSTOS.md): Proveedor, región, perfiles de VM, estimación fechada, presupuesto y política de encendido y apagado.
* [Notas Técnicas de la Entrega 2 (`docs/entrega2/NOTAS_TECNICAS.md`)](docs/entrega2/NOTAS_TECNICAS.md): Hallazgos que afectan a más de un issue, con el issue al que le toca resolver cada uno. **Conviene leerlo antes de empezar un issue y revisarlo al cerrarlo.**
* [Ejecución de las Pruebas de Multimedia (`docs/entrega2/EJECUCION_PRUEBAS_MULTIMEDIA.md`)](docs/entrega2/EJECUCION_PRUEBAS_MULTIMEDIA.md): Paso a paso de la carga directa al bucket y la lectura de un manifiesto HLS.
