# Evidencia C4 — Almacenamiento Administrado: API y Workers contra Cloud Storage con Permisos Diferenciados (issue #121)

La parametrización del adaptador de almacenamiento administrado reside en [`../../../../internal/config/config.go`](../../../../internal/config/config.go), [`../../../../internal/config/validate.go`](../../../../internal/config/validate.go) y [`../../../../internal/storage/gcs.go`](../../../../internal/storage/gcs.go). La definición de Compose para producción sin MinIO se encuentra en [`../../../../docker-compose.prod.yml`](../../../../docker-compose.prod.yml), mientras que el entorno de desarrollo local conserva MinIO en [`../../../../docker-compose.yml`](../../../../docker-compose.yml).

---

## Criterios de Aceptación y Archivos de Evidencia

| Archivo | Criterio del Enunciado que Demuestra |
| :--- | :--- |
| [`emision_url_prefirmada_gcs.txt`](./emision_url_prefirmada_gcs.txt) | **El endpoint de URL prefirmada emite una URL válida contra el bucket real**. Generación de URL firmada V4 apuntando a `https://storage.googleapis.com/plataforma-mooc-entrega2-media/originals/...` con identidad firmante `sa-web-server`. |
| [`carga_directa_completa_sin_api.txt`](./carga_directa_completa_sin_api.txt) | **Carga directa completa sin que el archivo pase por la API**. Transferencia binaria de 5 MiB punto a punto cliente $\rightarrow$ Google Cloud Storage (HTTP/2 200 OK), verificación de metadatos en GCS con `StatObject`, y confirmación en la API (HTTP 202 Accepted) sin intermediación de bytes. |
| [`rechazo_firma_alterada_o_vencida.txt`](./rechazo_firma_alterada_o_vencida.txt) | **URL vencida o firma alterada es rechazada**. Comprobación exhaustiva: firma alterada rechazada con `HTTP 403 Forbidden` (`SignatureDoesNotMatch`), URL expirada rechazada con `HTTP 400 Bad Request` (`ExpiredToken`), y cabecera `Content-Type` adulterada rechazada con `HTTP 403 Forbidden`. |
| [`worker_escribe_derivados_api_rechazada.txt`](./worker_escribe_derivados_api_rechazada.txt) | **El worker escribe derivados y la API no puede**. Demostración empírica de menor privilegio IAM: `sa-worker-server` escribe en `hls/` (200 OK) y es bloqueado en `originals/` (403 Forbidden); `sa-web-server` escribe en `originals/` (200 OK) y es bloqueado en `hls/` (403 Forbidden por condición CEL). |

---

## 1. Parametrización por Variables de Entorno

Tanto la API como el Worker determinan el proveedor de almacenamiento en **tiempo de ejecución** mediante variables de entorno, evitando código bifurcado o compilaciones condicionales:

| Variable | Desarrollo Local (`.env`) | Producción (`docker-compose.prod.yml` / VM) | Propósito |
| :--- | :--- | :--- | :--- |
| `STORAGE_BACKEND` | `minio` | `gcs` | Selecciona el adaptador (`MinIO` vs `GCS`). |
| `STORAGE_BUCKET` / `S3_BUCKET` | `mooc-storage` | `plataforma-mooc-entrega2-media` | Nombre del bucket administrado. |
| `GCP_PROJECT_ID` | N/A | `plataforma-mooc-entrega2` | Proyecto de Google Cloud Platform. |
| `GCS_SIGNER_ACCOUNT` | N/A | `sa-web-server@plataforma-mooc-entrega2.iam.gserviceaccount.com` | Cuenta de servicio delegada para firmar URLs V4 vía IAM Credentials API. |
| `STORAGE_ENDPOINT` / `S3_ENDPOINT` | `http://localhost:9000` | N/A (usa endpoints nativos de GCS) | Endpoint S3 compatible para MinIO. |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | `minioadmin` / `minioadmin` | No se configuran en producción | Credenciales estáticas (solo MinIO). |

### Validación Estricta en Arranque (`validate.go`)
El módulo [`internal/config/validate.go`](../../../../internal/config/validate.go) valida en el arranque (`APP_ENV=production`) que:
1. `STORAGE_BACKEND` no sea `minio`.
2. `S3_BUCKET` no conserve el valor por defecto de desarrollo (`mooc-storage`) ni esté vacío.
3. Las variables de MinIO no sean requeridas cuando se usa el backend administrado `gcs`.
4. El sistema aborte inmediatamente el arranque si detecta configuraciones locales en producción.

---

## 2. Retiro de MinIO en Producción

El archivo [`docker-compose.prod.yml`](../../../../docker-compose.prod.yml) define la topología de contenedores para producción sin MinIO:

- **Servicios removidos:**
  - `minio`: Eliminado completamente del Compose productivo.
  - `minio-policy`: Eliminado (la seguridad en nube se delega a las políticas IAM y condiciones CEL de Cloud Storage aprovisionadas en Terraform).
  - `mailpit`: Eliminado (reemplazado por SendGrid/SMTP administrado).
- **Servicios conservados:**
  - `api`: Con `STORAGE_BACKEND: gcs`, `STORAGE_BUCKET: plataforma-mooc-entrega2-media`, inyección de credenciales ADC mediante volumen de solo lectura `/app/secrets/gcp-credentials.json` y variable `GOOGLE_APPLICATION_CREDENTIALS`.
  - `worker`: Con `STORAGE_BACKEND: gcs` y credenciales de la cuenta `sa-worker-server`.
  - `redis`: Con persistencia AOF/RDB y autenticación con contraseña.
- **Entorno de desarrollo local (`docker-compose.yml`):**
  - Conserva MinIO intacto para que los desarrolladores continúen trabajando localmente sin conexión a internet ni consumo de recursos en GCP.

---

## 3. Permisos Diferenciados por Componente (Mínimo Privilegio)

El diseño de permisos garantiza que ningún componente posea facultades indebidas sobre el almacenamiento:

```
                                  +---------------------------------------+
                                  |   Bucket:                             |
                                  |   plataforma-mooc-entrega2-media      |
                                  +-------------------+-------------------+
                                                      |
                         +----------------------------+----------------------------+
                         |                                                         |
                         v                                                         v
             [Prefijo originals/]                                      [Prefijo hls/]
             +------------------------------+                          +------------------------------+
             | API (sa-web-server):         |                          | API (sa-web-server):         |
             | - Lectura: PERMITIDA         |                          | - Lectura: PERMITIDA         |
             | - Firma PUT: PERMITIDA       |                          | - Escritura: DENEGADA (403)  |
             +------------------------------+                          +------------------------------+
             | Worker (sa-worker-server):   |                          | Worker (sa-worker-server):   |
             | - Lectura: PERMITIDA         |                          | - Lectura: PERMITIDA         |
             | - Escritura: DENEGADA (403)  |                          | - Escritura: PERMITIDA (200) |
             +------------------------------+                          +------------------------------+
```

1. **API (`sa-web-server`):**
   - Posee `roles/storage.objectViewer` sobre el bucket (para `StatObject` y lectura).
   - Posee `roles/storage.objectCreator` condicionado por CEL exclusivamente a los prefijos de carga directa del usuario: `originals/`, `documents/` y `thumbnails/`.
   - **No puede escribir en `hls/`**: La condición CEL rechaza cualquier intento de escritura directa de la API sobre los derivados con `HTTP 403 Forbidden` ([evidencia](./worker_escribe_derivados_api_rechazada.txt)).
   - Posee el rol `roles/iam.serviceAccountTokenCreator` para firmar criptográficamente URLs V4 delegando la operación a Google Cloud sin almacenar llaves privadas en disco.

2. **Worker (`sa-worker-server`):**
   - Posee `roles/storage.objectViewer` sobre el bucket (para leer videos originales a procesar).
   - Posee `roles/storage.objectUser` condicionado por CEL estrictamente al prefijo `hls/`.
   - **No puede escribir en `originals/` ni `documents/`**: Cualquier intento del worker de alterar o sobrescribir archivos originales es rechazado con `HTTP 403 Forbidden` ([evidencia](./worker_escribe_derivados_api_rechazada.txt)).
   - No posee permisos de firma de URLs (el worker nunca interactúa directamente con los navegadores).

---

## 4. Carga Directa Desacoplada y Verificación en Ola 2

El flujo de carga multimedia implementado en [`internal/http/handler/media.go`](../../../../internal/http/handler/media.go) cumple con la arquitectura de descarga de tráfico (*offloading*):

```
  1. POST /api/v1/media/presigned-url
  +-----------+ ---------------------------------> +------------+
  |           |                                    |            |
  |  Cliente  | <--------------------------------- | API Server |
  | (Browser) |   2. 200 OK con upload_url V4      |            |
  |           |                                    +------------+
  |           |                                          ^
  |           | 3. PUT binario (5 MiB)                   | 4. POST /complete
  |           |    directo a GCS                         |    (solo object_key)
  v           v                                          v
+---------------------------------------------------------------+
|         Google Cloud Storage (storage.googleapis.com)         |
+---------------------------------------------------------------+
```

1. **Paso 1 (Autorización y Emisión):**
   - El cliente autenticado (profesor) solicita autorización enviando metadatos (`filename`, `mime_type`, `file_size_bytes`).
   - La API valida permisos del profesor sobre el curso y módulo, verifica que el tipo MIME coincida con el contrato del recurso y genera una clave de almacenamiento inmutable y determinística: `originals/{stable_id}/{uuid}.mp4`.
   - La API emite una URL firmada V4 con Google Cloud Storage ([evidencia](./emision_url_prefirmada_gcs.txt)).

2. **Paso 2 (Transferencia Directa Cliente $\rightarrow$ GCS):**
   - El cliente envía la carga binaria mediante `PUT` directamente al endpoint de Cloud Storage: `https://storage.googleapis.com/plataforma-mooc-entrega2-media/originals/...`.
   - Google Cloud Storage procesa la carga (HTTP/2 200 OK) respetando la política CORS configurada en C3.
   - **Cero bytes del archivo binario transitan por la API**, protegiendo la CPU, memoria y ancho de banda de la instancia web ([evidencia](./carga_directa_completa_sin_api.txt)).

3. **Paso 3 (Confirmación y Encolamiento):**
   - El cliente notifica la finalización a la API mediante `POST /api/v1/media/uploads/{resourceID}/complete` enviando únicamente la clave del objeto y la clave de idempotencia.
   - La API consulta los atributos reales del objeto en el bucket administrado (`StatObject`), comprobando existencia, tamaño exacto y tipo MIME.
   - Una vez comprobada la integridad física del archivo en GCS, la API actualiza el estado del recurso y encola el trabajo de transcodificación en Redis para los workers (HTTP 202 Accepted).

4. **Paso 4 (Defensa Criptográfica):**
   - Modificar un solo bit de la firma invalida el digest SHA-256 (`HTTP 403 Forbidden` con código `SignatureDoesNotMatch`).
   - Consumir la URL una vez transcurrido el TTL es bloqueado inmediatamente (`HTTP 400 Bad Request` con código `ExpiredToken`).
   - Intentar transferir un archivo con un encabezado `Content-Type` diferente al firmado altera la cabecera canónica y es rechazado (`HTTP 403 Forbidden`) ([evidencia](./rechazo_firma_alterada_o_vencida.txt)).

---

## 5. Medidas de Seguridad y Manejo de Secretos

- **Sin credenciales en el repositorio:** Los archivos de evidencia contienen exclusivamente trazas HTTP sanitizadas (`[REDACTED_PROFESSOR_SESSION_TOKEN]`, `[GENERATED_UUID]`). No se registran tokens JWT, Bearer tokens, contraseñas de bases de datos ni llaves privadas de servicio.
- **Firma por delegación (SignBlob):** La API utiliza `sa-web-server` como `GoogleAccessID`, permitiendo generar firmas criptográficas válidas tanto en desarrollo como en Compute Engine sin necesidad de almacenar llaves `.json` en el disco de la aplicación.
