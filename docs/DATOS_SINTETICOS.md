# Generación y Gestión de Datos Sintéticos Determinísticos — Plataforma MOOC

> **Referencia de Pruebas y Carga de Semillas para el Equipo de Desarrollo y QA**  
> **Alineación Normativa:** Secciones 6, 9 y 10.2 del Pliego de Especificaciones y Directrices de Arquitectura (`docs/PROJECT_KEY_ASPECTS.md`).

---

## 1. Propósito y Principios de Diseño

Para garantizar que las pruebas funcionales, suites automatizadas, pruebas de integración, colecciones de Postman y demostraciones del sistema sean **100% reproducibles y determinísticas**, la plataforma incorpora un mecanismo estandarizado de generación y limpieza de datos sintéticos.

### Principios Clave:
1. **Determinismo Estricto:** Todos los identificadores primarios (`id`), llaves estables (`stable_id`), correos electrónicos, códigos de verificación y marcas temporales son fijos. Cada corrida produce exactamente el mismo estado sin colisiones.
2. **Cobertura de los Tres Roles Globales:** Incluye cuentas con roles `administrador`, `profesor` y `estudiante`, cubriendo los distintos estados del ciclo de vida (`active`, `pending_verification`, `suspended`).
3. **Cursos en Múltiples Estados:** Contempla cursos en estado **`published`** (con jerarquía completa e inmutable), **`draft` completo** (listo para validar publicación exitosa), **`draft` incompleto** (para verificar listas de errores 422 agregadas) y **`unpublished`** (para validar el flujo de despublicación temporal del MVP).
4. **Contraseña Unificada y Segura:** Todos los usuarios de prueba comparten la credencial predecible `Password123!`, precalculada con el algoritmo de derivación bcrypt (`$2a$10$...`) compatible con `auth.VerifyPassword`.
5. **Idempotencia y Limpieza Segura:** Los scripts permiten re-ejecución sin duplicar registros (`ON CONFLICT DO UPDATE`) y limpieza atómica en cascada (`TRUNCATE ... CASCADE`), respetando la inmutabilidad de la tabla `audit_logs`.

---

## 2. Catálogo de Usuarios Sintéticos

Todos los usuarios de prueba tienen configurada la contraseña: **`Password123!`**.

| Rol | Correo Electrónico | Estado | UUID Determinístico (`id`) | Propósito en Pruebas |
|---|---|---|---|---|
| **Administrador** | `admin@plataforma-mooc.test` | `active` | `a0000000-0000-0000-0000-000000000001` | Gestión de usuarios, cambio de roles/estados y lectura del registro inmutable de auditoría. |
| **Administrador** | `admin.secundario@plataforma-mooc.test` | `active` | `a0000000-0000-0000-0000-000000000002` | Permite probar la protección del último administrador activo (`last_admin_protected`). |
| **Profesor** | `profesor1@plataforma-mooc.test` | `active` | `b0000000-0000-0000-0000-000000000001` | Autor del Curso Publicado y del Borrador Completo. Pruebas de autoría y publicación. |
| **Profesor** | `profesor2@plataforma-mooc.test` | `active` | `b0000000-0000-0000-0000-000000000002` | Autor del Borrador Incompleto y Curso Despublicado. Pruebas de aislamiento de propiedad. |
| **Estudiante** | `estudiante1@plataforma-mooc.test` | `active` | `c0000000-0000-0000-0000-000000000001` | Estudiante activo con inscripción y 50% de avance en el curso publicado. |
| **Estudiante** | `estudiante2@plataforma-mooc.test` | `active` | `c0000000-0000-0000-0000-000000000002` | Estudiante que ha completado y aprobado el curso, con una insignia digital emitida. |
| **Estudiante** | `estudiante.pendiente@plataforma-mooc.test` | `pending_verification` | `c0000000-0000-0000-0000-000000000003` | Verificación de rechazo de login (403 `email_not_verified`) antes de activar el correo. |
| **Estudiante** | `estudiante.suspendido@plataforma-mooc.test` | `suspended` | `c0000000-0000-0000-0000-000000000004` | Verificación de rechazo de login (403 `account_suspended`) para cuentas sancionadas. |

---

## 3. Catálogo de Cursos Sintéticos y Estructura Académica

| Título del Curso | Estado | Versión | UUID Curso (`id`) | Stable ID (`stable_id`) | Autor | Estructura Jerárquica |
|---|---|---|---|---|---|---|
| **Arquitectura Cloud y Sistemas Distribuidos** | `published` | 1 | `d0000000-0000-0000-0000-000000000001` | `e0000000-0000-0000-0000-000000000001` | `profesor1` | **2 Módulos, 2 Unidades, 3 Recursos:**<br>• Recurso 1: Texto Markdown (`d3...01`)<br>• Recurso 2: Video HLS (`d3...02`)<br>• Recurso 3: Quiz con 2 preguntas (`d3...03`) |
| **Desarrollo Backend Concurrente con Go** | `draft` | 1 | `d0000000-0000-0000-0000-000000000002` | `e0000000-0000-0000-0000-000000000002` | `profesor1` | **1 Módulo, 1 Unidad, 1 Recurso:**<br>Borrador completo y listo para probar publicación en vivo vía API (`POST /publish`). |
| **Seguridad en la Nube y Criptografía Aplicada** | `draft` | 1 | `d0000000-0000-0000-0000-000000000003` | `e0000000-0000-0000-0000-000000000003` | `profesor2` | **0 Módulos (Incompleto):**<br>Diseñado para probar la lista exhaustiva de errores de validación (HTTP 422). |
| **Introducción a DevSecOps y Contenedores Docker** | `unpublished` | 1 | `d0000000-0000-0000-0000-000000000004` | `e0000000-0000-0000-0000-000000000004` | `profesor2` | **1 Módulo, 1 Unidad, 1 Recurso:**<br>Curso previamente publicado y despublicado temporalmente para edición. |

### Detalle del Cuestionario y Evaluación (Curso 1)
* **Quiz ID:** `fa000000-0000-0000-0000-000000000001` (Nota de aprobación: 70%).
* **Pregunta 1 (`fb...01`):** *¿Dónde deben almacenarse los binarios multimedia según la Sección 7 del pliego?*
  * Opción Correcta (`fc...01`): *Exclusivamente en almacenamiento de objetos (S3/MinIO) vía URLs prefirmadas*.
* **Pregunta 2 (`fb...02`):** *¿Cómo se valida y computa el progreso de los estudiantes en la plataforma?*
  * Opción Correcta (`fc...04`): *Estrictamente en servidor mediante heartbeats verificados, permanencia y eventos*.

### Avance Estudiantil e Insignia Emitida
* **Estudiante 1:** Progreso de 50% registrado en `student_progress` para el curso `e0000000-0000-0000-0000-000000000001`.
* **Estudiante 2:** Progreso del 100% (`is_approved = true`) con insignia digital emitida en `badges`:
  * **Insignia ID:** `fe000000-0000-0000-0000-000000000001`
  * **Código de Verificación Pública:** `fe100000-0000-0000-0000-000000000001`
  * **Clave de Imagen en Objeto:** `badges/arquitectura-cloud-fe100000.png`

---

## 4. Cómo Cargar y Limpiar Datos en Docker Compose

Existen varias formas sencillas de gestionar los datos sintéticos según la preferencia del desarrollador o pipeline:

### 4.1 Método 1: Comandos Rápidos con `Makefile` (Recomendado)

Con los contenedores de Docker Compose levantados (`make docker-up` o `docker compose up -d`):

```bash
# 1. Cargar el dataset sintético
make seed

# 2. Consultar el resumen de entidades existentes en BD
make seed-status

# 3. Limpiar todas las tablas (TRUNCATE) y vaciar caché Redis
make seed-clean

# 4. Restaurar el estado inicial (Limpieza + Carga en un solo paso)
make seed-reset
```

---

### 4.2 Método 2: Uso del Script CLI (`./scripts/seed.sh`)

El script `./scripts/seed.sh` detecta automáticamente si Docker Compose está activo o si existe una variable `DATABASE_URL`:

```bash
# Cargar datos
./scripts/seed.sh --load

# Ver resumen de datos
./scripts/seed.sh --status

# Limpiar datos
./scripts/seed.sh --clean

# Restaurar estado inicial
./scripts/seed.sh --reset
```

---

### 4.3 Método 3: Ejecución Directa con `docker compose` y `psql`

Si se desea ejecutar los scripts SQL directamente a bajo nivel:

```bash
# Cargar el seed directamente por stdin
docker compose exec -T postgres psql -U moocuser -d moocdb < scripts/seeds/synthetic_data.sql

# Limpiar las tablas directamente
docker compose exec -T postgres psql -U moocuser -d moocdb < scripts/seeds/clean_data.sql

# Opcional: Limpiar la base de datos de Redis
docker compose exec -T redis redis-cli FLUSHDB
```

---

### 4.4 Método 4: Recreación Total del Entorno (Cold Reset)

Para reiniciar Docker Compose desde cero, eliminando volúmenes persistentes y volviendo a aplicar todas las migraciones:

```bash
# Apagar contenedores y eliminar volúmenes
docker compose down -v

# Levantar nuevamente (aplica migraciones automáticamente al inicializar)
docker compose up -d

# Cargar los datos sintéticos
make seed
```

---

## 5. Ejemplos de Validación Inmediata con la API

Una vez cargados los datos con `make seed`, puede ejecutar los siguientes comandos para comprobar el funcionamiento:

### 1. Iniciar Sesión como Administrador
```bash
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@plataforma-mooc.test","password":"Password123!"}'
```

### 2. Iniciar Sesión como Profesor y Consultar Cursos Propios
```bash
# Obtener token de profesor
PROF_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"profesor1@plataforma-mooc.test","password":"Password123!"}' | grep -o '"token":"[^"]*' | cut -d'"' -f4)

# Consultar el borrador completo
curl -s -H "Authorization: Bearer $PROF_TOKEN" http://localhost:8080/api/v1/courses/d0000000-0000-0000-0000-000000000002
```

### 3. Probar Rechazo de Publicación en Curso Incompleto (HTTP 422)
```bash
# Token del profesor 2
PROF2_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"profesor2@plataforma-mooc.test","password":"Password123!"}' | grep -o '"token":"[^"]*' | cut -d'"' -f4)

# Intentar publicar el curso incompleto (d0...03)
curl -i -X POST "http://localhost:8080/api/v1/courses/d0000000-0000-0000-0000-000000000003/publish" \
  -H "Authorization: Bearer $PROF2_TOKEN"
# Retorna 422 Unprocessable Entity con los errores estructurales y de aprobación.
```

### 4. Probar Login con Cuentas Pendientes y Suspendidas (HTTP 403)
```bash
# Cuenta pendiente de verificación
curl -i -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"estudiante.pendiente@plataforma-mooc.test","password":"Password123!"}'
# Retorna 403 Forbidden: email_not_verified

# Cuenta suspendida
curl -i -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"estudiante.suspendido@plataforma-mooc.test","password":"Password123!"}'
# Retorna 403 Forbidden: account_suspended
```

---

## 6. Pruebas Automatizadas de la Semilla

La integridad, compatibilidad de contraseñas y consistencia del dataset sintético está respaldada por una prueba automatizada en Go:

```bash
DATABASE_URL="postgres://moocuser:moocpassword@localhost:5432/moocdb?sslmode=disable" \
go test -v ./internal/postgres -run TestSyntheticDataSeed
```

Esta prueba verifica programáticamente:
* La correcta inserción de los usuarios de los 3 roles y sus estados.
* La verificación criptográfica del hash de contraseñas.
* La existencia de los cursos en sus estados (`published`, `draft`, `unpublished`).
* La integridad referencial de módulos, unidades, recursos, quizzes e insignias.
* La idempotencia de re-ejecución del script `synthetic_data.sql`.
* La efectividad del script de limpieza `clean_data.sql` y el truncado sin errores.
