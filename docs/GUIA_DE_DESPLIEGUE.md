# Guía Definitiva de Despliegue, Puesta en Marcha y Operación — Plataforma MOOC

> **Audiencia:** Evaluadores, nuevos desarrolladores, arquitectos y revisores externos que no participaron del desarrollo original.  
> **Objetivo:** Permitir a cualquier persona clonar el repositorio, levantar la plataforma completa con Docker Compose, sembrar los datos sintéticos, ejecutar las colecciones de pruebas en Postman/Newman y dejar el sistema 100% operativo sin necesidad de asistencia externa.  
> **Alineación Normativa:** Secciones 6, 7, 9 y 10 del Pliego de Especificaciones (`docs/2026-20 proyecto-plataforma-mooc (2).pdf`) y Directrices de Arquitectura ([`docs/PROJECT_KEY_ASPECTS.md`](./PROJECT_KEY_ASPECTS.md)).

---

## 1. Requisitos Previos del Sistema

Antes de iniciar, verifique que su estación de trabajo (Linux, macOS o Windows con WSL2) cuente con las siguientes herramientas mínimas:

| Herramienta | Versión Mínima | Propósito en la Plataforma | Comando de Verificación |
|---|:---:|---|---|
| **Docker Engine** | 24.0+ | Virtualización de contenedores para todos los servicios de la plataforma | `docker --version` |
| **Docker Compose** | v2.20+ | Orquestación multi-contenedor (`api`, `worker`, `postgres`, `redis`, `minio`, `mailpit`) | `docker compose version` |
| **GNU Make** | 4.0+ | Automatización de tareas de compilación, siembra de datos, pruebas y demos | `make --version` |
| **curl** y **jq** | Cualquier versión moderna | Inspección rápida de endpoints HTTP y formateo de respuestas JSON desde terminal | `curl --version && jq --version` |
| **Postman Desktop** *(Opcional)* | v10+ | Interfaz gráfica para importar colecciones y ejecutar pruebas interactivas | — |

> [!NOTE]
> **No es obligatorio tener Go instalado localmente** para levantar y operar el sistema: tanto la API REST como el Background Worker se compilan dentro de imágenes Docker multi-stage optimizadas basadas en Go 1.24 Alpine.

### Verificación de Puertos Disponibles en el Host
El despliegue por defecto utiliza los siguientes puertos en su máquina local. Asegúrese de que no estén siendo ocupados por otros procesos:
* `8080`: Servidor API REST en Go y Swagger UI.
* `5432`: Base de datos PostgreSQL 16.
* `6379`: Almacén en memoria Redis 7 (sesiones, rate limit y colas Asynq).
* `8025`: Interfaz Web de Mailpit (buzón de correos de prueba).
* `1025`: Servidor SMTP de Mailpit (recepción de emails transaccionales).
* `9000`: API S3 de almacenamiento de objetos MinIO.
* `9001`: Consola Web administrativa de MinIO.
* `9090`: Servidor de métricas Prometheus del Background Worker.

---

## 2. Paso a Paso: Despliegue desde Cero con Docker Compose

Siga esta secuencia ordenada de comandos para iniciar la plataforma desde cero:

### Paso 2.1: Clonar el Repositorio y Posicionarse en el Directorio
```bash
git clone https://github.com/ISIS4426-2026/Plataforma-MOOC.git
cd Plataforma-MOOC
```

### Paso 2.2: Configurar las Variables de Entorno
El repositorio incluye una plantilla exhaustiva (`.env.example`) con valores predeterminados listos para desarrollo y pruebas locales. Copie la plantilla para crear el archivo `.env`:

```bash
cp .env.example .env
```

> [!TIP]
> **No requiere modificar ningún valor en `.env` para la ejecución local estándar.** Todos los servicios se comunican internamente mediante nombres de dominio DNS de Docker Compose (`postgres`, `redis`, `minio`, `mailpit`, `api`) y exponen los puertos estándar en `localhost`.

### Paso 2.3: Construir y Levantar los Contenedores
Inicie toda la infraestructura en segundo plano con el comando estándar de Docker Compose:

```bash
docker compose up --build -d
```
*(Alternativa rápida con Makefile: `make docker-up`)*

#### ¿Qué ocurre automáticamente durante este proceso?
1. **Compilación Multi-Stage:** Se construyen las imágenes de la API REST (`Dockerfile.api`) y del procesador asíncrono (`Dockerfile.worker`), generando binarios ligeros y seguros sin dependencias locales.
2. **PostgreSQL 16 & Migraciones Automáticas:** El contenedor de base de datos arranca y ejecuta inmediatamente el script `scripts/init-db.sh`, el cual aplica secuencialmente todas las migraciones SQL (`migrations/*.up.sql`), asegurando la creación del esquema relacional, reglas inmutables de auditoría y cero tipos BLOB en base de datos.
3. **Redis 7:** Se inicializa el servicio en memoria para soporte de sesiones revocables, rate limiting con ventanas deslizantes y colas Asynq.
4. **MinIO & Inicialización del Bucket S3:** Se levanta el servidor MinIO y el contenedor efímero `minio-init` ejecuta `scripts/init-minio.sh`, configurando las credenciales y aprovisionando de inmediato el bucket `mooc-storage` para almacenamiento de objetos.
5. **Mailpit:** Se levanta el servidor SMTP simulado y su API de inspección de correos.
6. **Healthchecks de Dependencias:** Tanto la `api` como el `worker` esperan a que PostgreSQL, Redis, MinIO y Mailpit reporten estado `healthy` antes de recibir tráfico.

---

## 3. Verificación de Salud y Disponibilidad de Servicios

Una vez finalizado el comando anterior, espere entre 5 y 10 segundos para que todos los servicios alcancen el estado saludable.

### Paso 3.1: Comprobar el Estado de los Contenedores
Ejecute:
```bash
docker compose ps
```

**Salida esperada:** Todos los servicios deben mostrar estado `Up` y `(healthy)`:
```
NAME                       IMAGE                    STATUS                    PORTS
plataforma-mooc-api-1      plataforma-mooc-api      Up (healthy)              0.0.0.0:8080->8080/tcp
plataforma-mooc-worker-1   plataforma-mooc-worker   Up (healthy)              0.0.0.0:9090->9090/tcp
plataforma-mooc-postgres-1 postgres:16-alpine       Up (healthy)              0.0.0.0:5432->5432/tcp
plataforma-mooc-redis-1    redis:7-alpine           Up (healthy)              0.0.0.0:6379->6379/tcp
plataforma-mooc-minio-1    minio/minio:latest       Up (healthy)              0.0.0.0:9000-9001->9000-9001/tcp
plataforma-mooc-mailpit-1  axllent/mailpit:latest   Up (healthy)              0.0.0.0:1025->1025/tcp, 0.0.0.0:8025->8025/tcp
```

### Paso 3.2: Verificar los Endpoints Principales vía cURL o Navegador

| Servicio | URL / Endpoint | Cómo Verificar (Terminal) | Salida Esperada |
|---|---|---|---|
| **Healthcheck API** | `http://localhost:8080/api/v1/health` | `curl -s http://localhost:8080/api/v1/health \| jq .` | `{"status":"pass","service":"plataforma-mooc-api",...}` |
| **Documentación Swagger UI** | `http://localhost:8080/api/docs` | Abrir en navegador web | Interfaz visual interactiva OpenAPI 3.1 lista para "Try it out" |
| **Contrato OpenAPI Crudo** | `http://localhost:8080/api/v1/openapi.yaml` | `curl -sI http://localhost:8080/api/v1/openapi.yaml` | `HTTP/1.1 200 OK`, `Content-Type: application/yaml` |
| **Mailpit Web UI (Emails)** | `http://localhost:8025` | Abrir en navegador web | Bandeja de entrada visual de correos transaccionales |
| **MinIO Console (S3)** | `http://localhost:9001` | Abrir en navegador (User: `minioadmin` / Pass: `minioadmin`) | Consola de administración con bucket `mooc-storage` creado |
| **Métricas API (Prometheus)** | `http://localhost:8080/api/v1/metrics` | `curl -s http://localhost:8080/api/v1/metrics \| head -n 10` | Formato estándar de métricas Prometheus con latencias y contadores |
| **Métricas Worker (Prometheus)** | `http://localhost:9090/metrics` | `curl -s http://localhost:9090/metrics \| head -n 10` | Métricas de tareas procesadas y fallidas en el background worker |

---

## 4. Carga y Gestión de Datos Sintéticos Determinísticos

Para probar la plataforma con usuarios, roles, cursos en diversos estados e insignias sin tener que crearlos manualmente, se proporciona un dataset determinístico.

### Paso 4.1: Cargar el Dataset Sintético
Ejecute el siguiente comando para sembrar los datos en PostgreSQL:

```bash
make seed
```
*(O de forma directa mediante script: `./scripts/seed.sh --load`)*

### Paso 4.2: Comprobar el Resumen de Datos Cargados
Para verificar que las entidades fueron sembradas correctamente, ejecute:

```bash
make seed-status
```
*(O con script: `./scripts/seed.sh --status`)*

**Salida esperada:**
```
========================================================================
  Plataforma MOOC — Resumen de Datos en Base de Datos
========================================================================
  Usuarios:           8 (Admins: 2 | Profesores: 2 | Estudiantes: 4)
  Cursos:             4 (Publicados: 1 | Borradores: 2 | Despublicados: 1)
  Módulos:            4
  Unidades:           4
  Recursos:           5 (Texto: 2 | Video: 1 | Quiz: 1 | Descargable: 1)
  Cuestionarios:      1 (Preguntas: 2 | Opciones: 6)
  Inscripciones:      2 (Progreso registrado: 2 | Aprobados: 1)
  Insignias:          1 (Con código de verificación pública)
  Registros de Auditoría: Disponibles con inmutabilidad en BD
========================================================================
```

### Paso 4.3: Catálogo de Cuentas Preconfiguradas para Pruebas

> [!IMPORTANT]
> **Todos los usuarios de prueba comparten la contraseña unificada:**  
> `Password123!`

| Rol Global | Correo Electrónico | Contraseña | Estado de Cuenta | Propósito Principal en la Plataforma |
|---|---|:---:|:---:|---|
| **Administrador Principal** | `admin@plataforma-mooc.test` | `Password123!` | `active` | Gestión total de usuarios, roles, estados y consulta de auditoría inmutable. |
| **Administrador Secundario** | `admin.secundario@plataforma-mooc.test` | `Password123!` | `active` | Pruebas de protección del último administrador (`last_admin_protected`). |
| **Profesor Autor 1** | `profesor1@plataforma-mooc.test` | `Password123!` | `active` | Autor del Curso 1 (Publicado) y Curso 2 (Borrador completo). |
| **Profesor Autor 2** | `profesor2@plataforma-mooc.test` | `Password123!` | `active` | Autor del Curso 3 (Incompleto para error 422) y Curso 4 (Despublicado). |
| **Estudiante Activo** | `estudiante1@plataforma-mooc.test` | `Password123!` | `active` | Estudiante con avance parcial (50%) en curso publicado. |
| **Estudiante Aprobado** | `estudiante2@plataforma-mooc.test` | `Password123!` | `active` | Estudiante con 100% de avance e insignia digital emitida. |
| **Estudiante Pendiente** | `estudiante.pendiente@plataforma-mooc.test` | `Password123!` | `pending_verification` | Validación de rechazo de login (403 `email_not_verified`). |
| **Estudiante Suspendido** | `estudiante.suspendido@plataforma-mooc.test` | `Password123!` | `suspended` | Validación de bloqueo de acceso (403 `account_suspended`). |

### Paso 4.4: Comandos de Limpieza y Reinicio de Datos
Si desea reiniciar o limpiar la base de datos durante sus pruebas:
* **Limpiar todas las tablas y vaciar Redis:** `make seed-clean`
* **Reiniciar atómicamente a estado inicial (Clean + Seed):** `make seed-reset`

---

## 5. Importación y Ejecución de Colecciones Postman

La plataforma cuenta con 3 colecciones completas en formato Postman v2.1 y entornos parametrizados en la carpeta `docs/postman/`:
1. `collection_api.postman_collection.json` (Identidad y Seguridad — Issue #24).
2. `collection_admin.postman_collection.json` (Administración y RBAC — Issue #25).
3. `collection_authoring.postman_collection.json` (Autoría y Publicación — Issue #26).

Se ofrecen dos métodos de ejecución: **Método Automatizado (Línea de comandos con Newman)** y **Método Gráfico (Postman Desktop)**.

---

### Método A: Ejecución Automatizada con Newman en Docker (Recomendado)

Este método **no requiere instalar Postman ni Node.js en su máquina host**: utiliza la imagen oficial `postman/newman:alpine` dentro de la red Docker Compose.

#### 1. Ejecutar Todas las Colecciones en un Solo Paso
```bash
make test-postman
```
Este comando ejecuta secuencialmente las colecciones de Identidad, Administración y Autoría, limpiando automáticamente los límites de tasa en Redis antes de cada ejecución.

#### 2. Ejecutar Colecciones de Manera Individual
* **Solo Identidad (#24):**
  ```bash
  make test-postman-identity
  ```
  *(59 peticiones, 110 aserciones, valida registro, activación por email Mailpit, login, logout, revocación de sesión, rate limit e idempotencia)*.

* **Solo Administración (#25):**
  ```bash
  make test-postman-admin
  ```
  *(23 peticiones, 46 aserciones, valida listado de usuarios, cambio de roles, suspensión de cuentas, rechazo RBAC 403 y protección del último admin con 409)*.

* **Solo Autoría de Cursos (#26):**
  ```bash
  make test-postman-authoring
  ```
  *(30 peticiones, 60 aserciones, valida jerarquía de 4 niveles, ETag, validación multi-error 422, inmutabilidad 409, stable_id y despublicación temporal)*.

#### Resultado Esperado Consolidado
```
┌─────────────────────────┬───────────────────┬──────────────────┐
│                         │          executed │           failed │
├─────────────────────────┼───────────────────┼──────────────────┤
│              iterations │                 3 │                0 │
├─────────────────────────┼───────────────────┼──────────────────┤
│                requests │               112 │                0 │
├─────────────────────────┼───────────────────┼──────────────────┤
│              assertions │               216 │                0 │
└─────────────────────────┴───────────────────┴──────────────────┘
```
**Total: 216 aserciones aprobadas con 0 fallos (100% PASS).**

---

### Método B: Ejecución Gráfica en Postman Desktop

Si prefiere inspeccionar las peticiones interactivamente a través de la interfaz gráfica de Postman Desktop:

#### 1. Importar las Colecciones y el Entorno Local
1. Abra **Postman Desktop**.
2. Haga clic en el botón **Import** (esquina superior izquierda).
3. Arrastre o seleccione los siguientes archivos ubicados en la carpeta `docs/postman/`:
   * `collection_api.postman_collection.json`
   * `collection_admin.postman_collection.json`
   * `collection_authoring.postman_collection.json`
   * `mooc_local.postman_environment.json`

#### 2. Activar el Entorno
En el selector desplegable de entornos (esquina superior derecha de Postman), elija:  
👉 **`Plataforma MOOC - Local`**

> [!NOTE]
> El entorno `mooc_local` contiene las variables apuntando a `http://localhost:8080` (API) y `http://localhost:8025` (Mailpit).

#### 3. Ejecutar las Colecciones con el Collection Runner
1. En la barra lateral izquierda, seleccione la colección deseada (ej. **Plataforma MOOC - Autoria de Cursos**).
2. Haga clic en el botón **Run** (o **Run Collection**).
3. Asegúrese de que todas las peticiones estén seleccionadas y en el orden original.
4. Haga clic en **Run Plataforma MOOC - ...**.
5. Las peticiones se ejecutarán en cadena; las variables dinámicas (tokens Bearer, IDs de curso, tokens de verificación de correo) se transfieren automáticamente entre peticiones mediante scripts de Postman sin necesidad de copiar y pegar nada a mano.

---

## 6. Validación Automatizada del Sistema y Demos de la Etapa

Para verificar la integridad del código fuente, la observabilidad y los flujos de demostración requeridos por la **Sección 10.2**:

### 6.1 Suite de Validación Integral de la Etapa
Ejecute la suite completa de calidad:
```bash
make test-stage
```
Este script valida:
1. Análisis estático y formato (`go fmt`, `go vet`, Spectral linter de OpenAPI 3.1).
2. Migraciones reversibles de base de datos (`migrations_test.go`).
3. Batería de pruebas unitarias y de dominio (`auth`, `course`, `domain`, `http`, `postgres`, `worker`).
4. Pruebas de tolerancia a fallos, reintentos y DLQ en Redis.
5. Compilación de binarios (`bin/api` y `bin/worker`).
6. Ejecución de las 3 colecciones Newman contra el sistema desplegado.

### 6.2 Demostración Interactiva de los Segmentos 1 y 2 (Identidad y Autoría)
Para presenciar la demostración paso a paso en consola con logs explicativos:
```bash
make demo-segments-1-2
```
*(Consulte la guía detallada de sustentación en [`docs/e2e/DEMO_SEGMENTOS_1_Y_2.md`](./e2e/DEMO_SEGMENTOS_1_Y_2.md))*.

### 6.3 Demostración del Worker Asíncrono, Tolerancia a Fallos y DLQ (Segmento 4)
Para verificar la idempotencia ante entregas duplicadas, los 3 reintentos con backoff exponencial y el desvío a Dead-Letter Queue con alertas estructuradas:
```bash
make demo-segment4
```

---

## 7. Detención y Limpieza del Entorno

Cuando desee apagar los servicios:

* **Detener los contenedores preservando los datos de los volúmenes:**
  ```bash
  docker compose down
  ```
  *(O con Makefile: `make docker-down`)*

* **Detener y eliminar todos los volúmenes para un reinicio limpio absoluto:**
  ```bash
  docker compose down -v
  ```

---

## 8. Solución de Problemas Frecuentes (Troubleshooting)

### Problema 1: Conflicto de puertos en el host (ej. `port 8080 or 5432 already in use`)
* **Causa:** Tiene otro servicio local (como un PostgreSQL nativo o servidor web) ocupando el puerto.
* **Solución:** Modifique los puertos en su archivo `.env`:
  ```ini
  API_PORT=8090
  POSTGRES_PORT=5433
  MAILPIT_PORT=8026
  ```
  Luego reinicie con `docker compose up -d`. Si usa Postman Desktop, actualice el puerto en la variable `baseUrl` del entorno `mooc_local`.

### Problema 2: El endpoint responde `429 Too Many Requests` durante pruebas continuas
* **Causa:** El limitador de tasa de Redis protegió la ruta tras múltiples intentos sucesivos.
* **Solución:** Vacíe las claves de rate limit en Redis con el siguiente comando:
  ```bash
  docker compose exec redis redis-cli EVAL "for _,k in ipairs(redis.call('keys','ratelimit:*')) do redis.call('del',k) end" 0
  ```

### Problema 3: Las pruebas reportan datos inconsistentes o colisiones tras pruebas manuales
* **Causa:** Se modificaron datos manualmente en la base de datos o se ejecutaron pruebas desordenadas.
* **Solución:** Restaure el estado determinístico inicial en un solo comando:
  ```bash
  make seed-reset
  ```

### Problema 4: Permisos de Docker en Linux (`permission denied while trying to connect to the Docker daemon socket`)
* **Causa:** Su usuario de Linux no pertenece al grupo `docker`.
* **Solución:** Agregue su usuario al grupo docker:
  ```bash
  sudo usermod -aG docker $USER
  newgrp docker
  ```

---

## 9. Referencia Documental y Arquitectónica

Para profundizar en el diseño y los estándares del proyecto, consulte los documentos oficiales en el repositorio:

* [Directrices Clave de Arquitectura (`docs/PROJECT_KEY_ASPECTS.md`)](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones%20Cloud/Proyectos/P1_data/Plataforma-MOOC/docs/PROJECT_KEY_ASPECTS.md): Reglas no negociables (Quiz Key Secrecy, server-side progress, inmutabilidad de cursos publicados, cero binarios en PostgreSQL).
* [Plan Maestro de Pruebas de la Etapa (`docs/PLAN_DE_PRUEBAS_ETAPA.md`)](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones%20Cloud/Proyectos/P1_data/Plataforma-MOOC/docs/PLAN_DE_PRUEBAS_ETAPA.md): Mapeo completo de las Secciones 6, 9 y 10.2.
* [Reporte de Aseguramiento de Calidad y Certificación Sección 10 (`docs/e2e/REPORTE_BUGS_Y_CALIDAD_ETAPA.md`)](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones%20Cloud/Proyectos/P1_data/Plataforma-MOOC/docs/e2e/REPORTE_BUGS_Y_CALIDAD_ETAPA.md): Matriz formal de bugs, fichas de reproducción y certificación de ausencia de fallas críticas.
* [Guion Técnico y Libreto del Video de Demostración (`docs/e2e/README_GUION_VIDEO_DEMO.md`)](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones%20Cloud/Proyectos/P1_data/Plataforma-MOOC/docs/e2e/README_GUION_VIDEO_DEMO.md): Guion completo para grabación del video cubriendo los 9 segmentos de la Sección 10.2.
* [Catálogo Detallado de Datos Sintéticos (`docs/DATOS_SINTETICOS.md`)](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones%20Cloud/Proyectos/P1_data/Plataforma-MOOC/docs/DATOS_SINTETICOS.md): Descripción pormenorizada de cursos, módulos, quizzes y usuarios sembrados.
* [Guía de Colecciones Postman (`docs/postman/README.md`)](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones%20Cloud/Proyectos/P1_data/Plataforma-MOOC/docs/postman/README.md): Especificación técnica de las 97 peticiones y 216 aserciones automatizadas.
