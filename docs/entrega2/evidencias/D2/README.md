# Evidencia D2 — Web Server: VM en GCP, Docker Compose, Proxy Inverso y Cierre de Ola 2 (Issue #123)

Este directorio documenta el despliegue del punto de acceso público de la **Plataforma MOOC**, cumpliendo con la rúbrica de **Despliegue e integración (50%)** de la Entrega 2.

Las evidencias contenidas aquí son salidas sanitizadas de ejecución real en Google Cloud Platform. Nunca incluyen credenciales, contraseñas, llaves privadas ni secretos.

---

## Criterios de Aceptación y Archivos de Evidencia

| Archivo | Criterio del Enunciado | Estado |
| :--- | :--- | :---: |
| [`terraform_plan.txt`](./terraform_plan.txt) | **VM con perfil B1 declarada en Terraform**: Instancia `mooc-web-server` (`e2-highcpu-2`, 30 GiB `pd-balanced`, `us-east1-b`) con IP pública estática y cuenta de servicio `sa-web-server`. | ✅ Cumplido |
| [`health_publico.txt`](./health_publico.txt) | **`health` responde 200 desde Internet**: Petición pública a `http://34.24.52.111/api/v1/health` a través del puerto 80 vía Nginx, consultando la base Cloud SQL (`10.171.240.3`) con SSL obligatorio. | ✅ Cumplido |
| [`docs_publico.txt`](./docs_publico.txt) | **Verificar `GET /api/docs` y `/api/v1/openapi.yaml`**: Interfaz Swagger UI y especificación de contrato OpenAPI accesibles desde Internet. | ✅ Cumplido |
| [`supervivencia_reinicio.txt`](./supervivencia_reinicio.txt) | **El contenedor sobrevive a un reinicio de la VM**: Servicio systemd `mooc-web.service` y Docker `restart: unless-stopped` restauran automáticamente todos los contenedores a estado saludable (`healthy`) tras `sudo reboot`. | ✅ Cumplido |
| [`cierre_ola2_c1_c4.txt`](./cierre_ola2_c1_c4.txt) | **Cerrar los criterios de ola 2 de C1 y C4**: Demostración de persistencia transaccional y auditoría en Cloud SQL (C1) y flujo completo de carga directa desacoplada (*offloading*) a Cloud Storage (C4). | ✅ Cumplido |

---

## 1. Topología del Despliegue en la Nube

La máquina virtual `mooc-web-server` actúa como el punto público de entrada para los clientes y navegadores, desacoplando la terminación HTTP y el backend mediante contenedores:

```
                            INTERNET (0.0.0.0/0)
                                     │
                                     │ HTTP :80 (público)
                                     ▼
 ┌────────────────────────────────────────────────────────────────────────┐
 │ VM: mooc-web-server (34.24.52.111, e2-highcpu-2, us-east1-b)          │
 │                                                                        │
 │   ┌──────────────────────┐               ┌─────────────────────────┐  │
 │   │  proxy               │ ────────────> │  api                    │  │
 │   │  (nginx:1.27-alpine) │  :8080        │  (plataforma-mooc-api)  │  │
 │   │  Puerto 80           │  (red Docker) │  Healthcheck activo     │  │
 │   └──────────────────────┘               └───────────┬─────────────┘  │
 │                                                      │                │
 │                                                      ▼                │
 │                                          ┌─────────────────────────┐  │
 │                                          │  redis (redis:7-alpine) │  │
 │                                          │  Sesiones, rate-limit   │  │
 │                                          └─────────────────────────┘  │
 └──────────────────────────────┬───────────────────────────────┬─────────┘
                                │                               │
                   Private VPC  │                               │ Private Access
                   (Port 5432)  ▼                               ▼ (HTTPS)
                    ┌──────────────────────┐        ┌──────────────────────┐
                    │ Cloud SQL (C1)       │        │ Cloud Storage (C3/C4)│
                    │ mooc-db-1            │        │ Bucket administrado  │
                    │ 10.171.240.3         │        │ media (us-east1)     │
                    │ sslmode=require      │        │                      │
                    └──────────────────────┘        └──────────────────────┘
```

1. **Firewall perimetral (B3):** La regla `mooc-allow-web-ingress` filtra todo el tráfico entrante, permitiendo exclusivamente los puertos TCP `80` y `443` hacia las instancias etiquetadas con `web-server`. El puerto de la API (`8080`) no está expuesto a Internet.
2. **Proxy Inverso Nginx (`nginx.conf`):** Recibe las solicitudes HTTP en el puerto 80, inyecta las cabeceras `Host`, `X-Real-IP`, `X-Forwarded-For` y `X-Forwarded-Proto`, y reenvía internamente al contenedor `api:8080`.
3. **Persistencia transaccional privada (C1):** La API se comunica con Cloud SQL (`10.171.240.3`) a través del peering de servicios de la VPC privada con TLS obligatorio (`sslmode=require`), garantizando que la base de datos nunca tenga IP pública.
4. **Almacenamiento de objetos (C4):** La API firma peticiones V4 utilizando la identidad `sa-web-server` adjunta a la VM mediante ADC (Application Default Credentials), delegando la firma a Cloud IAM sin almacenar credenciales en el sistema de archivos.

---

## 2. Inyección de Configuración y Manejo de Secretos

Siguiendo las restricciones de validación en arranque (`internal/config/validate.go`), el entorno productivo rechaza cualquier parámetro propio del entorno local:

| Parámetro | Valor Efectivo | Validación de Seguridad |
| :--- | :--- | :--- |
| `APP_ENV` | `production` | Activa comprobaciones estrictas de arranque. |
| `DATABASE_URL` | `postgres://moocuser:<PASSWORD>@10.171.240.3:5432/moocdb?sslmode=require` | La contraseña se lee dinámicamente de Secret Manager (`db-password`) en tiempo de arranque con la cuenta `sa-web-server`. |
| `REDIS_URL` | `redis:6379` | Almacenamiento volátil para sesiones y colas en memoria. |
| `STORAGE_BACKEND` | `gcs` | Adaptador nativo de Google Cloud Storage. |
| `S3_BUCKET` | `plataforma-mooc-entrega2-media` | Bucket regional administrado en `us-east1`. |
| `GCS_SIGNER_ACCOUNT` | `sa-web-server@plataforma-mooc-entrega2.iam.gserviceaccount.com` | Cuenta autorizada con rol `roles/iam.serviceAccountTokenCreator`. |
| `APP_BASE_URL` | `http://34.24.52.111` | Dirección pública real (rechaza `localhost` y `127.0.0.1`). |
| `DB_MAX_OPEN_CONNS` | `25` | Acotado según el presupuesto de conexiones de C1. |
| `DB_MAX_IDLE_CONNS` | `25` | Igual al máximo abierto para reutilizar handshakes TLS. |

---

## 3. Supervivencia y Recuperación Autónoma

Para garantizar que el stack sobreviva a reinicios accidentales o programados de la máquina virtual:

1. **Política Docker Compose:** Todos los contenedores (`proxy`, `api`, `redis`) tienen configurada la directiva `restart: unless-stopped`.
2. **Servicio Systemd (`mooc-web.service`):**
   * Se ejecuta automáticamente tras `network-online.target` y `docker.service`.
   * En `ExecStartPre`, invoca `prepare_env.sh`, el cual consulta la versión más reciente del secreto `db-password` en Secret Manager y formatea las variables de entorno de manera segura.
   * En `ExecStart`, ejecuta `docker compose up -d`, asegurando la convergencia al estado deseado.
3. **Evidencia:** Tras ejecutar `sudo reboot`, la máquina se reinició en menos de 30 segundos, los tres contenedores arrancaron de forma ordenada respetando el grafo de dependencias (`redis` $\rightarrow$ `api` $\rightarrow$ `proxy`), el healthcheck reportó `healthy` y `curl http://34.24.52.111/api/v1/health` respondió `HTTP 200 OK` inmediatamente.

---

## 4. Cierre de Criterios de Ola 2 (C1 y C4)

El levantamiento exitoso del Web Server permitió validar las capacidades de Ola 2 de los componentes C1 y C4 contra la infraestructura real en la nube:

* **Ola 2 de C1 (Cloud SQL):**
  * Autenticación exitosa mediante `POST /api/v1/auth/login`.
  * Consulta de usuarios vía endpoint administrativo `GET /api/v1/admin/users`.
  * Modificación de estado de cuenta mediante `PATCH /api/v1/admin/users/{userID}/status`.
  * Verificación de persistencia de la auditoría en la tabla `audit_logs` de la instancia `mooc-db-1`.
* **Ola 2 de C4 (Cloud Storage):**
  * Solicitud de URL prefirmada de carga mediante `POST /api/v1/media/presigned-url`.
  * Transferencia binaria directa del cliente a Google Cloud Storage (`PUT storage.googleapis.com/...`).
  * Cero bytes del archivo multimedia transitaron a través de la API Web.
  * Confirmación de carga mediante `POST /api/v1/media/uploads/{resourceID}/complete`, validación de metadatos en GCS con `StatObject` y encolamiento del procesamiento en Redis (`HTTP 202 Accepted`).
  * Emisión de URL prefirmada de descarga (`GET /api/v1/media/resources/{resourceID}/download-url`).
