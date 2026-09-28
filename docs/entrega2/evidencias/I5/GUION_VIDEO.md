# Guion Técnico de Producción — Video de Sustentación (Entrega 2 · Issue #140 [I5])

> **Proyecto:** Plataforma MOOC (Massive Open Online Courses) — Despliegue Básico en la Nube Pública (GCP)  
> **Versión del Guion:** `v1.1-single-presenter`  
> **Fecha:** 2026-09-28  
> **Commit Base de Revisión:** `f18703e97a944fda9ee794e48a3c31ff57f4f3f7` (`main` @ merge PR #174)  
> **Duración Objetivo:** **18 minutos** (margen de seguridad de 2 min respecto al tope de 20 min del pliego)  
> **Equipo de Desarrollo:** `CrispisCas9` (Cristian Castañeda), `fredyxander` (Fredy Alexander), `DiegoOrtizRuiz` (Diego Ortiz), `tmichelldiaz` (Tania Michel Díaz)  
> **Presentador Único del Video:** `CrispisCas9` (Cristian Castañeda)  

---

## Control de Cambios

| Versión | Fecha | Commit SHA | Descripción de Cambios | Autor |
| :---: | :---: | :---: | :--- | :--- |
| `v1.0-draft` | 2026-09-28 | `f18703e` | Borrador inicial consolidado con evidencias de `main` (bloques B, C, D, E, F, G, H1, H2, H4, H5). Clasificación formal de estados (✅/⚠️/⏳) y registro de salvedades metodológicas de capacidad. | Antigravity (Issue #140) |
| `v1.1-single-presenter` | 2026-09-28 | `f18703e` | Consolidación de la presentación en un único orador (`CrispisCas9` / Cristian Castañeda). Adaptación de todas las locuciones, transiciones narrativas y tablas al rol de relator único en representación del equipo de desarrollo. | `CrispisCas9` |

---

## 1. Presupuesto Global de Tiempos y Estructura de la Presentación

El tiempo total planificado es de **18:00 minutos**, reservando un margen de holgura de 2:00 minutos frente al límite reglamentario de 20:00 minutos establecido en la p. 7 del enunciado (`docs/2026-20 Entrega 2 - Despliegue Básico en la Nube.pdf`). La presentación es conducida en su totalidad por **Cristian Castañeda (`CrispisCas9`)**, exponiendo de manera articulada los aportes y evidencias construidas por los cuatro integrantes del equipo:

| Bloque | Segmento Temático | Duración Sugerida | Minutaje Acumulado | Presentador Responsable | Estado Base |
| :---: | :--- | :---: | :---: | :--- | :---: |
| **0** | **Introducción y Pre-flight Check** | 0:45 | `00:00 - 00:45` | `CrispisCas9` | ✅ Nube |
| **1** | **Arquitectura Desplegada y Correspondencia GCP** | 3:30 | `00:45 - 04:15` | `CrispisCas9` | ✅ Nube |
| **2** | **Recorrido Funcional en el Entorno Cloud** | 4:30 | `04:15 - 08:45` | `CrispisCas9` | ✅ Nube |
| **3** | **Evidencias Específicas de Infraestructura y Resiliencia** | 4:30 | `08:45 - 13:15` | `CrispisCas9` | ✅ Nube |
| **4** | **Análisis de Capacidad, Cuello de Botella y Evolución** | 4:00 | `13:15 - 17:15` | `CrispisCas9` | ⚠️ Con Salvedad / ⏳ |
| **5** | **Conclusiones, Criterios de Aceptación y Cierre** | 0:45 | `17:15 - 18:00` | `CrispisCas9` | ✅ Nube |

---

## 2. Parámetros y Archivos Variados Utilizados en la Sustentación

Para dar estricto cumplimiento a la directriz del pliego de emplear **parámetros variados, entradas válidas e inválidas, diversos roles y formatos multimedia heterogéneos**, la siguiente matriz especifica cada archivo y valor empleado, junto con su fuente oficial en el repositorio:

| Dimensión | Variante / Valor Empleado | Propósito / Comportamiento Demostrado | Fuente en el Repositorio |
| :--- | :--- | :--- | :--- |
| **Roles de Usuario** | `administrador` (`admin`) | Gestión administrativa, auditoría y protección de último admin | [`scripts/seeds/synthetic_data.sql`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/scripts/seeds/synthetic_data.sql) |
| | `profesor` (`profesor1`, `profesor2`) | Autoría de jerarquía, control de propiedad y emisión de URLs | [`scripts/seeds/synthetic_data.sql`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/scripts/seeds/synthetic_data.sql) |
| | `estudiante` (`estudiante1`) | Consumo, inscripción, quizzes, avance e insignias | [`scripts/seeds/synthetic_data.sql`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/scripts/seeds/synthetic_data.sql) |
| | `anonimo` (sin sesión) | Verificación pública de insignias (200) y rechazo en módulos (401) | [`docs/entrega2/evidencias/G3/resultados.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G3/resultados.md) |
| **Perfiles Multimedia** | **Corto:** 2 min, 1280×720, 2.6 MB | Línea base y transcodificación rápida (38 objetos reales, 43.7 s) | [`docs/entrega2/evidencias/G1/manifest.json`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G1/manifest.json) |
| | **Medio:** 10 min, 1280×720, 13.0 MB | Carga media y streaming paced (172 objetos reales, 3m 52s) | [`docs/entrega2/evidencias/G1/manifest.json`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G1/manifest.json) |
| | **Largo:** 30 min, 1280×720, 39.0 MB | Estrés de worker y transcodificación pesada (504 objetos, 11m 13s) | [`docs/entrega2/evidencias/G1/manifest.json`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G1/manifest.json) |
| **Tipos de Archivo** | Video: `.mp4` (`video/mp4`) | Transcodificación HLS (escalera 360p + 720p sin upscaling) | [`internal/storage/keys.go#L44`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/internal/storage/keys.go#L44) |
| | Audio: `.mp3`, `.wav` | Formatos de solo audio admitidos en prefijo `originals/` | [`internal/storage/keys.go#L48-L50`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/internal/storage/keys.go#L48-L50) |
| | Documentos: `.pdf`, `.docx` | Archivos complementarios admitidos en prefijo `documents/` | [`internal/storage/keys.go#L53-L57`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/internal/storage/keys.go#L53-L57) |
| | Miniaturas: `.jpg`, `.png` | Portadas de curso admitidas en prefijo `thumbnails/` | [`internal/storage/keys.go#L59-L62`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/internal/storage/keys.go#L59-L62) |
| **Casos Inválidos** | Registro con rol `profesor` | Rechazo 400 `invalid_registration_role` (docentes solo por admin) | [`docs/entrega2/evidencias/G3/resultados.md#L20`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G3/resultados.md#L20) |
| | Extensión no admitida `.exe` | Rechazo 400 `invalid_input` en solicitud de URL firmada | [`docs/entrega2/evidencias/G3/resultados.md#L63`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G3/resultados.md#L63) |
| | Publicar curso sin módulos | Rechazo 422 `publication_validation_failed` multi-error acumulado | [`docs/entrega2/evidencias/G3/resultados.md#L46`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G3/resultados.md#L46) |
| | Modificar curso publicado | Rechazo 409 `course_immutable` (inmutabilidad estricta) | [`docs/entrega2/evidencias/G3/resultados.md#L80`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G3/resultados.md#L80) |
| | Firma V4 alterada o vencida | Rechazo 403 `SignatureDoesNotMatch` / 400 `ExpiredToken` en bucket | [`docs/entrega2/evidencias/C4/rechazo_firma_alterada_o_vencida.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/C4/rechazo_firma_alterada_o_vencida.txt) |
| | Objeto original sin firma | Rechazo 403 Forbidden directo en Cloud Storage | [`docs/entrega2/evidencias/C3/objeto_no_publico_sin_firma.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/C3/objeto_no_publico_sin_firma.txt) |
| | Ráfaga de login (> 10 req/min) | Rechazo 429 `rate_limit_exceeded` respaldado por Redis | [`docs/entrega2/evidencias/H2/resultados/local-login-rafaga_20260927_202603/resumen.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/H2/resultados/local-login-rafaga_20260927_202603/resumen.txt) |

---

## 3. Disposición de Pantalla Recomendada (Layout de Grabación)

Para garantizar legibilidad profesional y correlación en vivo durante los 18 minutos:

```
+---------------------------------------------------+---------------------------------------------------+
|  PANEL IZQUIERDO: CLIENTE HTTP / CONSOLAS WEB     |  PANEL DERECHO: TERMINAL MULTI-PANEL Y LOGS GCP   |
|                                                   |                                                   |
|  • Postman Desktop / Newman CLI Runner:           |  [Panel Superior: Logs en Vivo vía IAP Tunnel]    |
|    - Entorno: mooc_cloud (https://34.24.52.111...) |  $ gcloud compute ssh mooc-web-server ...         |
|    - Colecciones: Admin, Authoring, Quizzes, etc. |    $ docker logs -f mooc-api                      |
|                                                   |                                                   |
|  • Pestañas del Navegador Web:                    |  ------------------------------------------------ |
|    - Swagger UI: https://34.24.52.111.sslip.io... |  [Panel Inferior: Cloud SQL psql / Worker Logs]   |
|    - Cloud Monitoring: Métricas de VMs y Colas    |  $ gcloud compute ssh mooc-worker-server ...      |
|    - Verificación pública de Insignias            |    $ docker logs -f mooc-worker                   |
+---------------------------------------------------+---------------------------------------------------+
```

> [!IMPORTANT]
> **Checklist Pre-Grabación (T-Minus 5 Minutos):**
> 1. Verificar en GCP Console que las VMs `mooc-web-server` y `mooc-worker-server` y la base `mooc-db-1` estén encendidas (`CONFIGURACION_Y_COSTOS.md` §5).
> 2. Confirmar que el presupuesto de 50 USD esté activo y no agotado ([`docs/entrega2/evidencias/B1/presupuesto_y_alertas1.PNG`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/B1/presupuesto_y_alertas1.PNG)).
> 3. Verificar certificado TLS vigente (`curl -Iv https://34.24.52.111.sslip.io/api/v1/health`).
> 4. Recordatorio estricto: **Cero contraseñas, tokens JWT, firmas V4 ni credenciales deben mostrarse en pantalla ni verbalizarse.**

---

## 4. Guion Técnico Detallado — Recorrido Cronológico

---

### Segmento 0: Introducción, Objetivos y Pre-flight Check
* **Tiempo:** `00:00 - 00:45` (Duración: 0:45)
* **Presentador:** `CrispisCas9` (Cristian Castañeda)
* **Estado:** ✅ **Nube**
* **Disposición en Pantalla:** Diapositiva inicial con título de la sustentación, nombres de los cuatro integrantes del equipo y arquitectura general; luego transición a terminal mostrando verificación de salud de la plataforma en la nube.
* **Comando / Acción Exacta:**
  ```bash
  curl -s -i https://34.24.52.111.sslip.io/api/v1/health
  ```
* **Texto Sugerido para la Locución:**  
  *"Bienvenidos a la sustentación técnica de la Entrega 2 de Desarrollo de Soluciones Cloud. Mi nombre es Cristian Castañeda (`CrispisCas9`), y en representación de nuestro equipo de proyecto —conformado además por Tania Michel Díaz, Fredy Alexander y Diego Ortiz— presentaré la sustentación completa de la migración de nuestra plataforma MOOC a la nube pública en Google Cloud Platform. En este proyecto hemos operado bajo las restricciones formales del pliego: capacidad fija de cómputo en dos máquinas virtuales, cero mecanismos de autoescalado y persistencia delegada en servicios administrados relacionales y de almacenamiento de objetos. Como observan en pantalla, nuestro punto de acceso público bajo dominio HTTPS responde de manera saludable contra la base administrada. Durante los próximos dieciocho minutos expondré la correspondencia con los servicios del proveedor, el recorrido funcional completo en la nube, las evidencias de resiliencia y los hallazgos de nuestros análisis de capacidad."*
* **Ruta de Registro en el Repositorio:** [`docs/entrega2/evidencias/D2/health_publico.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/D2/health_publico.txt) y [`docs/entrega2/evidencias/G3/resultados.md#L13`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G3/resultados.md#L13).

---

### Segmento 1: Arquitectura Desplegada y Correspondencia con Servicios GCP
* **Tiempo:** `00:45 - 04:15` (Duración: 3:30)
* **Presentador:** `CrispisCas9` (Cristian Castañeda)
* **Estado:** ✅ **Nube**
* **Disposición en Pantalla:** Diagrama de red de B3 en Mermaid ([`docs/entrega2/evidencias/B3/DIAGRAMA_RED.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/B3/DIAGRAMA_RED.md)) en panel izquierdo; en panel derecho, consola de Terraform mostrando recursos aprovisionados (`compute.tf`, `database.tf`, `storage.tf`, `network.tf`).

```mermaid
flowchart TD
    subgraph Internet ["🌐 Internet (0.0.0.0/0)"]
        User["Cliente / Navegador Web"]
        Admin["Operador DevOps (IAP)"]
    end

    subgraph GCP ["☁️ Proyecto GCP: plataforma-mooc-entrega2 (us-east1)"]
        subgraph VPC ["🔒 mooc-vpc (10.0.0.0/16)"]
            subgraph Subnet ["🖥️ mooc-subnet (10.0.1.0/24 en us-east1-b)"]
                WebServer["🌐 Web Server VM (34.24.52.111 / 10.0.1.2)\nPerfil: e2-highcpu-2 (2 vCPU, 2 GiB)\nProxy Nginx 1.27 + Go API Modular"]
                WorkerServer["⚙️ Worker Server VM (10.0.1.3)\nPerfil: e2-highcpu-2 (2 vCPU, 2 GiB)\nWorker Go + FFmpeg + Redis Asynq"]
            end
            CloudNAT["📡 Cloud Router + NAT (Salida Egress)"]
            subgraph Peering ["🔐 Service Networking Peering (10.0.2.0/20)"]
                CloudSQL[("🗄️ Cloud SQL PostgreSQL 16 (mooc-db-1)\n1 vCPU dedicada, 3.75 GiB RAM, 10 GiB SSD\nIP Privada: 10.171.240.3 (Sin IP Pública)")]
            end
        end
        subgraph Storage ["📦 Cloud Storage Buckets (us-east1)"]
            BucketPrivado[("gs://plataforma-mooc-entrega2-media\nPrivado (enforced) - originals/, docs/, thumbs/")]
            BucketPublico[("gs://plataforma-mooc-entrega2-hls\nPúblico (inherited) - derivados HLS")]
        end
    end

    User -->|HTTPS 443 / HTTP 80| WebServer
    Admin -->|SSH Túnel IAP (puerto 22)| WebServer
    Admin -->|SSH Túnel IAP (puerto 22)| WorkerServer
    WebServer -->|TCP 6379 (Privado)| WorkerServer
    WebServer -->|TCP 5432 (SSL require)| CloudSQL
    WorkerServer -->|TCP 5432 (SSL require)| CloudSQL
    WorkerServer -->|Egress HTTP/S| CloudNAT
    CloudNAT --> Internet
    WebServer -.->|Firma URLs V4 (ADC)| BucketPrivado
    WorkerServer -->|Lectura Originales / Escritura HLS| Storage
```

* **Puntos Clave y Cifras a Exponer:**
  1. **Cómputo (Compute Engine):**
     - Dos máquinas virtuales dedicadas en zona única `us-east1-b`: `mooc-web-server` y `mooc-worker-server`.
     - Perfil exacto del pliego: `e2-highcpu-2` (2 vCPU dedicadas, 2 GiB RAM, 30 GiB disco `pd-balanced`). Se descartó `e2-small` porque solo garantiza 0.5 vCPU compartida ([`CONFIGURACION_Y_COSTOS.md` §2](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/CONFIGURACION_Y_COSTOS.md#2-configuracion-efectiva)).
  2. **Topología de Red y Aislamiento Perimetral (VPC):**
     - Red `mooc-vpc` (`10.0.0.0/16`) con subred `mooc-subnet` (`10.0.1.0/24`).
     - Web Server es el **único punto de entrada público** (puertos 80 y 443 expuestos por la regla de firewall `mooc-allow-web-ingress`).
     - Worker Server, la cola Redis (puerto 6379) y Cloud SQL están totalmente aislados de Internet. La cola solo acepta conexiones desde la subred privada interna `10.0.1.0/24` ([`evidencias/B3/DOCUMENTACION_RED_Y_ADMINISTRACION.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/B3/DOCUMENTACION_RED_Y_ADMINISTRACION.md)).
     - Administración segura por SSH sin abrir puerto 22 a `0.0.0.0/0`: uso exclusivo de Google Identity-Aware Proxy (`35.235.240.0/20`).
  3. **Base de Datos Administrada (Cloud SQL):**
     - Instancia `mooc-db-1` en PostgreSQL 16 Enterprise, zona `us-east1-b`, sin réplicas de lectura.
     - 1 vCPU dedicada, 3.75 GiB RAM, 10 GiB SSD. Se descartaron núcleos compartidos (`db-f1-micro`) por estar fuera de SLA y distorsionar pruebas de carga ([`CONFIGURACION_Y_COSTOS.md` §2](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/CONFIGURACION_Y_COSTOS.md#la-base-de-datos-por-que-1-vcpu-dedicada-y-no-un-perfil-compartido)).
     - Acceso privado vía Private Services Access (`10.171.240.3`), `ipv4_enabled = false`, SSL obligatorio (`ENCRYPTED_ONLY`).
     - Presupuesto de conexiones explícito: `max_connections = 100` (API 25 + Worker 25 + reservas = ~58 comprometidas, ~42 de margen).
  4. **Almacenamiento de Objetos (Cloud Storage):**
     - Separación arquitectural en dos buckets en `us-east1` (Nota Técnica 1b, PR #167):
       * `plataforma-mooc-entrega2-media`: bucket privado con `public_access_prevention = enforced`. Aloja `originals/`, `documents/` y `thumbnails/`.
       * `plataforma-mooc-entrega2-hls`: bucket público con `public_access_prevention = inherited` para derivados HLS (`master.m3u8` y `.ts`), resolviendo que los reproductores web no heredan firmas V4 en rutas relativas.
     - Mínimo privilegio IAM con condiciones CEL: la API solo crea objetos en originales y no puede escribir en derivados; el worker solo escribe en el bucket HLS.
  5. **Costos y Presupuesto:**
     - Estimación 24×7 de lista: **133.73 USD/mes** ([`CONFIGURACION_Y_COSTOS.md` §4](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/CONFIGURACION_Y_COSTOS.md#4-estimacion-de-costos)).
     - Techo operativo real gobernado por cupones educativos de 50 USD redimidos secuencialmente. Presupuesto activo de 50 USD con alertas al 25%, 50%, 80% y 100% sobre gasto bruto (excluyendo créditos promocionales para evitar retrasos de aviso).
* **Texto Sugerido para la Locución:**  
  *"En la pantalla visualizan la correspondencia exacta entre nuestra arquitectura y los servicios administrados de GCP en la región us-east1, zona única us-east1-b. Para cómputo, aprovisionamos dos máquinas virtuales con el perfil exacto e2-highcpu-2, con 2 vCPU dedicadas y 2 GiB de memoria RAM. Descartamos perfiles como e2-small porque en GCP garantizan apenas media vCPU compartida, incumpliendo la especificación. En red, mooc-vpc aísla por completo nuestros componentes: el Web Server es la única máquina con puertos HTTP y HTTPS abiertos al mundo. El Worker Server y la cola de mensajería Redis en el puerto 6379 carecen de acceso público y solo se comunican por la subred privada 10.0.1.0/24. La administración no expone el puerto SSH 22 a internet, sino que canaliza los accesos autenticados a través de Google Cloud IAP. Nuestra base de datos relacional es una instancia Cloud SQL PostgreSQL 16 con una vCPU dedicada y 3.75 GiB de memoria en la IP privada 10.171.240.3; no posee IP pública y exige cifrado TLS. Para el almacenamiento, aplicamos un hallazgo crítico documentado en la Nota Técnica 1b: debido a que un reproductor HLS resuelve variantes relativas sin propagar la query string de la firma, separamos el almacenamiento en dos buckets: uno privado bajo estricta prevención de acceso público para originales y documentos, y un bucket público exclusivo para derivados HLS. Todo el aprovisionamiento está codificado en Terraform en la carpeta infra/terraform."*
* **Rutas de Registro en el Repositorio:**
  - Cómputo: [`infra/terraform/compute.tf`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/infra/terraform/compute.tf) y [`docs/entrega2/evidencias/D2/terraform_plan.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/D2/terraform_plan.txt).
  - Red y Firewall: [`docs/entrega2/evidencias/B3/DIAGRAMA_RED.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/B3/DIAGRAMA_RED.md) y [`docs/entrega2/evidencias/G4/a_escaneo_puertos_externo.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G4/a_escaneo_puertos_externo.txt).
  - Cloud SQL: [`infra/terraform/database.tf`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/infra/terraform/database.tf) y [`docs/entrega2/evidencias/C1/instancia_configuracion.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/C1/instancia_configuracion.txt).
  - Storage: [`infra/terraform/storage.tf`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/infra/terraform/storage.tf) y [`docs/entrega2/evidencias/C3/README.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/C3/README.md).
  - Costos y Presupuesto: [`docs/entrega2/CONFIGURACION_Y_COSTOS.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/CONFIGURACION_Y_COSTOS.md) y [`docs/entrega2/evidencias/B1/README.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/B1/README.md).

---

### Segmento 2: Recorrido Funcional en el Entorno Cloud
* **Tiempo:** `04:15 - 08:45` (Duración: 4:30)
* **Presentador:** `CrispisCas9` (Cristian Castañeda)
* **Estado:** ✅ **Nube** (Basado en la corrida E2E de 61 pasos y suite Postman de 166 peticiones contra `https://34.24.52.111.sslip.io`)
* **Disposición en Pantalla:** Postman Desktop ejecutando peticiones clave contra el entorno `mooc_cloud` en la mitad izquierda; en la mitad derecha, terminal con salida del runner E2E (`scripts/e2e_cloud`) o Newman.
* **Flujos Funcionales Demostrados:**

#### 1. Identidad, Registro y SMTP en la Nube (`04:15 - 05:15`)
* **Acción:**
  - Registro de estudiante: `POST /api/v1/auth/register` con payload válido $\to$ `HTTP 201 Created` (`status: "pending_verification"`). El 201 acredita que el relevo Brevo SMTP aceptó el correo transaccional en el puerto 587 con STARTTLS.
  - Caso negativo: Registro con rol profesor $\to$ `HTTP 400 Bad Request` (`invalid_registration_role`). Los profesores solo se crean por administración.
  - Login con cuenta activa (admin, profesor, estudiante) $\to$ `HTTP 200 OK`, token Bearer y cabecera `Set-Cookie: __Host-mooc_session=...; Secure; HttpOnly; SameSite=Lax`.
* **Texto Sugerido para la Locución:**  
  *"A continuación realizo el recorrido funcional sobre nuestro entorno en la nube pública. Iniciamos con el módulo de identidad. En pantalla ejecuto el registro de un nuevo estudiante contra el Web Server en GCP. El sistema responde HTTP 201 Created con estado pendiente de verificación. Esta respuesta acredita la integración con nuestro servidor SMTP en la nube usando Brevo en el puerto 587 con STARTTLS; Mailpit ha sido retirado en producción. Si intento registrar directamente un usuario con rol profesor, la API rechaza la solicitud con HTTP 400, preservando la regla de negocio que exige la creación docente por vía administrativa. Al iniciar sesión con una cuenta activa, el servidor entrega el token Bearer y fija la cookie segura __Host- con atributos Secure y HttpOnly."*
* **Ruta de Registro en el Repositorio:** [`docs/entrega2/evidencias/G3/resultados.md#L15-L28`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G3/resultados.md#L15-L28) y [`docs/entrega2/evidencias/F1/README.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/F1/README.md).

#### 2. Control de Acceso, Autoría de 4 Niveles y Validación Multi-error (`05:15 - 06:45`)
* **Acción:**
  - Estudiante intenta crear curso $\to$ `HTTP 403 Forbidden` (`code: "forbidden"`).
  - Profesor autenticado crea curso en borrador $\to$ `HTTP 201 Created` (`status: "draft"`, `version: 1`, `stable_id` emitido).
  - Catálogo público anónimo (`GET /courses`) confirma que el borrador no es visible.
  - Intento de publicar curso incompleto $\to$ `HTTP 422 Unprocessable Entity` (`publication_validation_failed`) con lista acumulativa de incumplimientos (falta módulo, falta unidad, falta recurso).
  - Construcción jerárquica de 4 niveles:
    * `POST /courses/{id}/modules` $\to$ 201 (pos 0, stable_id).
    * `POST /modules/{id}/units` $\to$ 201 (pos 0, stable_id).
    * `POST /units/{id}/resources` $\to$ 201 (recurso Video pos 0, Texto Markdown pos 1, Quiz pos 2).
  - El profesor define el cuestionario con sus preguntas y respuestas correctas: `POST /resources/{id}/quiz` y `POST /quizzes/{id}/questions`.
  - Publicación exitosa $\to$ `POST /courses/{id}/publish` responde `HTTP 200 OK` (`status: "published"`).
  - Intento de mutar el curso publicado $\to$ `HTTP 409 Conflict` (`course_immutable`).
* **Texto Sugerido para la Locución:**  
  *"Demostramos el control de acceso basado en roles y la autoría de cursos. Cuando simulo un estudiante intentando crear un curso, recibe HTTP 403 Forbidden. Autenticado como docente, creo el curso en estado borrador con su stable_id. El catálogo público no lista este borrador. Al intentar publicarlo vacío, el validador no se detiene en el primer error: retorna un HTTP 422 con todos los requisitos pendientes de manera simultánea. Procedo a estructurar la jerarquía completa de cuatro niveles: creo el módulo, la unidad y tres recursos obligatorios: un video, una lectura en Markdown canónico y un cuestionario. Como docente configuro las preguntas del quiz con sus opciones y respuestas correctas. Con la estructura completa, la publicación responde HTTP 200 OK. De inmediato, cualquier intento de mutar metadatos o estructura del curso publicado es rechazado con HTTP 409 Conflict, garantizando inmutabilidad estricta."*
* **Ruta de Registro en el Repositorio:** [`docs/entrega2/evidencias/G3/resultados.md#L30-L57`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G3/resultados.md#L30-L57) y [`docs/entrega2/evidencias/G3/resultados.md#L78-L83`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G3/resultados.md#L78-L83).

#### 3. Inscripción, Quiz Key Secrecy, Progreso Verificado e Insignia (`06:45 - 08:45`)
* **Acción:**
  - Estudiante se inscribe en el curso publicado: `POST /courses/{id}/enrollments` $\to$ `HTTP 200 OK`.
  - Estudiante accede al contenido del curso $\to$ `GET /courses/{id}/modules` responde 200 (antes de inscribirse respondía 403).
  - Quiz Key Secrecy: Estudiante consulta el cuestionario $\to$ `GET /resources/{id}/quiz`. El JSON contiene las preguntas pero **carece por completo del atributo `is_correct`**.
  - Presentación y Calificación: `POST /quizzes/{id}/submissions` con `Idempotency-Key` $\to$ `HTTP 200 OK` (`score: 100`, `passed: true`, `attempt: 1`).
  - Progreso verificado en servidor: Al aprobar el quiz, el servidor computa automáticamente el recurso como completado (avance 1/3, 33.33%).
  - Envío de latidos de lectura y video $\to$ `POST /progress/heartbeat` $\times 2 \to$ avance 3/3 (100.00%, `approved: true`).
  - Emisión y Verificación Pública de Insignia: El estudiante obtiene su insignia (`GET /badges/{id}` con ETag). Un tercero anónimo sin sesión ejecuta `GET /api/v1/badges/verify/{code}` $\to$ `HTTP 200 OK` confirmando validez sin exponer el email ni identidad del estudiante.
* **Texto Sugerido para la Locución:**  
  *"Completo el recorrido funcional como estudiante. Al inscribirme, desbloqueo el acceso a los módulos que antes me devolvían 403. Al consultar el cuestionario académico, observen cómo el servidor oculta rigurosamente la clave de respuestas correctas: el campo is_correct no existe en el payload JSON, respetando nuestro principio de Quiz Key Secrecy. El estudiante envía sus respuestas con su Idempotency-Key y recibe calificación de 100 puntos aprobando en el primer intento. El servidor registra el avance de forma autónoma sin confiar en latidos manipulables del cliente: el progreso pasa a 33.33%. Al reportar la lectura y el video mediante latidos, el curso alcanza el 100% de avance y el sistema emite una insignia digital con código criptográfico único. Demuestro la verificación pública: cualquier evaluador externo, sin iniciar sesión ni enviar tokens, consulta el endpoint de verificación y comprueba que la insignia es legítima sin que se filtre el correo electrónico ni datos personales del alumno."*
* **Ruta de Registro en el Repositorio:** [`docs/entrega2/evidencias/G3/resultados.md#L85-L110`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G3/resultados.md#L85-L110), [`docs/entrega2/evidencias/A5/README.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/A5/README.md) y [`docs/entrega2/evidencias/A6/README.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/A6/README.md).

---

### Segmento 3: Evidencias Específicas de Infraestructura, Asincronía y Resiliencia
* **Tiempo:** `08:45 - 13:15` (Duración: 4:30)
* **Presentador:** `CrispisCas9` (Cristian Castañeda)
* **Estado:** ✅ **Nube** (Ejecutado sobre GCP Compute Engine, Cloud Storage, Cloud SQL y Worker Server en red privada)
* **Disposición en Pantalla:** Terminal dividida en tres paneles: izquierda con cliente curl enviando peticiones de almacenamiento y fallos inyectados; derecha superior con logs del worker (`docker logs mooc-worker`); derecha inferior con cliente psql hacia Cloud SQL.

#### 1. Carga Directa sin Pasar por la API y URLs Firmadas V4 (`08:45 - 09:45`)
* **Acción:**
  - Solicitar URL prefirmada: `POST /api/v1/media/presigned-url` $\to$ HTTP 200 con `upload_url` firmada V4 apuntando a `https://storage.googleapis.com/plataforma-mooc-entrega2-media/originals/...`.
  - Carga directa: `curl -X PUT "$UPLOAD_URL" -H "Content-Type: video/mp4" --upload-file video.mp4` $\to$ `HTTP/2 200 OK` emitido directamente por Google Cloud Storage. La VM de la API no transfirió ni un solo byte binario.
  - Validación de seguridad de firmas:
    * Intento de acceder al original en `originals/` sin firma $\to$ `HTTP 403 Forbidden` (`public_access_prevention = enforced`).
    * Intento con firma alterada $\to$ `HTTP 403 Forbidden` (`SignatureDoesNotMatch`).
    * Intento con URL vencida $\to$ `HTTP 400 Bad Request` (`ExpiredToken`).
* **Texto Sugerido para la Locución:**  
  *"En este segmento evidencio los requisitos no funcionales críticos de infraestructura. Primero, la descarga de tráfico o carga directa. La API emite una URL prefirmada V4 con expiración de 24 horas firmada criptográficamente por la cuenta de servicio sa-web-server mediante ADC. El cliente sube el archivo binario directamente a Google Cloud Storage mediante un PUT HTTP/2; la API modular queda totalmente liberada del tráfico pesado de subida. Compruebo la seguridad del almacenamiento: si un usuario intenta leer el objeto original sin firma, Cloud Storage devuelve HTTP 403 Forbidden. Si altero un solo carácter de la firma, el bucket responde SignatureDoesNotMatch; y ante una URL expirada, responde con HTTP 400."*
* **Ruta de Registro en el Repositorio:** [`docs/entrega2/evidencias/C4/carga_directa_completa_sin_api.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/C4/carga_directa_completa_sin_api.txt), [`docs/entrega2/evidencias/C3/objeto_no_publico_sin_firma.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/C3/objeto_no_publico_sin_firma.txt) y [`docs/entrega2/evidencias/C4/rechazo_firma_alterada_o_vencida.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/C4/rechazo_firma_alterada_o_vencida.txt).

#### 2. IAM Diferenciado por Componente (Mínimo Privilegio con CEL) (`09:45 - 10:30`)
* **Acción:**
  - Probar que la cuenta de la API (`sa-web-server`) intenta escribir en el prefijo de derivados `hls/` $\to$ `HTTP 403 Forbidden` bloqueado por condición CEL (`api_deny_derivatives_write`).
  - Probar que la cuenta del Worker (`sa-worker-server`) intenta escribir en `originals/` $\to$ `HTTP 403 Forbidden`. El worker solo puede escribir en el bucket público de derivados HLS.
* **Texto Sugerido para la Locución:**  
  *"Compruebo el principio de menor privilegio con IAM diferenciado. Nuestra política en Terraform no concede roles de almacenamiento globales. Mediante expresiones CEL, la cuenta sa-web-server tiene permiso de creación restringido a originales, documentos y miniaturas; si intenta escribir en derivados hls/, Google Cloud Storage lo rechaza con HTTP 403. Inversamente, el Worker Server solo tiene permiso de escritura sobre el bucket público de HLS y tiene prohibido escribir en originales."*
* **Ruta de Registro en el Repositorio:** [`docs/entrega2/evidencias/C3/api_rechaza_escritura_derivados.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/C3/api_rechaza_escritura_derivados.txt) y [`docs/entrega2/evidencias/C4/worker_escribe_derivados_api_rechazada.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/C4/worker_escribe_derivados_api_rechazada.txt).

#### 3. Procesamiento Asíncrono y Persistencia en Cloud SQL (`10:30 - 11:30`)
* **Acción:**
  - Confirmar carga ante la API: `POST /api/v1/resources/{id}/confirm-upload` $\to$ `HTTP 202 Accepted` (`processing_status: "pending"`). La API valida existencia con `StatObject` y despacha la tarea a Redis en el Worker Server vía VPC.
  - Mostrar logs en vivo de `mooc-worker`: el procesador descarga el original, ejecuta FFmpeg en `/tmp/mooc-media` con concurrencia fija en 2 (`WORKER_CONCURRENCY=2`), genera la escalera 360p y 720p sin upscaling, sube `master.m3u8` y segmentos `.ts` al bucket público HLS, y actualiza el estado en Cloud SQL a `completed` (estado `available` de dominio).
  - Reproductor o cliente consulta el manifiesto HLS sin firma: `GET https://storage.googleapis.com/plataforma-mooc-entrega2-hls/hls/{id}/master.m3u8` $\to$ `HTTP 200 OK`.
* **Texto Sugerido para la Locución:**  
  *"Al confirmar la subida, la API responde HTTP 202 Accepted y coloca la tarea en la cola Redis del Worker Server a través de la VPC privada. El worker independiente toma la tarea respetando su concurrencia configurada en 2, ejecuta FFmpeg para transcodificar a 360p y 720p sin upscaling, deposita los segmentos HLS en el bucket público y actualiza el estado en Cloud SQL a completed, equivalente al estado available del pliego. Compruebo que el manifiesto master.m3u8 y sus segmentos son consumibles inmediatamente por un reproductor sin firma."*
* **Ruta de Registro en el Repositorio:** [`docs/entrega2/evidencias/E1/procesamiento_video_e2e.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/E1/procesamiento_video_e2e.txt) y [`docs/entrega2/evidencias/G3/resultados.md#L60-L74`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G3/resultados.md#L60-L74).

#### 4. Idempotencia ante Entrega Duplicada (`11:30 - 12:15`)
* **Acción:**
  - Reencolar forzadamente la misma tarea de transcodificación con el mismo `idempotency_key` en Redis Asynq.
  - Mostrar logs de `mooc-worker`: el middleware de idempotencia detecta la clave y la validación de estado comprueba que ya está `completed`.
  - Resultado: la tarea finaliza en 4.1 ms (`job skipped; this object is already processed`), sin invocar FFmpeg, sin reescribir objetos en Cloud Storage y sin registrar filas redundantes en la tabla de auditoría de Cloud SQL.
* **Texto Sugerido para la Locución:**  
  *"Demuestro la idempotencia ante fallos de red o entregas duplicadas. Reencolo intencionalmente la misma tarea multimedia en Redis. Como observan en los logs del worker, el middleware de idempotencia intercepta el identificador en cuatro milisegundos y omite la transcodificación. Verifico en Cloud Storage que las marcas de tiempo de los archivos HLS permanecen intactas, y en Cloud SQL confirmo mediante consulta SQL que no se generó ninguna transición redundante en la tabla de auditoría."*
* **Ruta de Registro en el Repositorio:** [`docs/entrega2/evidencias/E1/idempotencia_entrega_duplicada.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/E1/idempotencia_entrega_duplicada.txt).

#### 5. Fallo con Reintento, Backoff Exponencial y Cola de Fallidos (DLQ) (`12:15 - 13:15`)
* **Acción:**
  - Despachar una tarea con error transitorio inducido al Worker Server en la nube (`test:ping`, tarea de demostración técnica inyectada con `max_retry: 3`).
  - Mostrar el log cronológico de reintentos:
    * Intento inicial ($t = 0\text{ s}$): Falla con error simulado.
    * Reintento 1 ($t = 2\text{ s}$): Espera de 2 segundos ($2 \times 2^0$).
    * Reintento 2 ($t = 6\text{ s}$): Espera de 4 segundos ($2 \times 2^1$).
    * Reintento 3 ($t = 14\text{ s}$): Espera de 8 segundos ($2 \times 2^2$).
    * Agotamiento ($t = 14\text{ s}$): Emisión de alerta estructurada `[ALERT] Job moved to Dead-Letter Queue (DLQ)` con código `DLQ_JOB_FAILED`.
  - Inspeccionar Redis en `mooc-worker-server`: consulta a `asynq:archived` muestra la tarea archivada con su stack trace para diagnóstico.
* **Texto Sugerido para la Locución:**  
  *"Para evidenciar la tolerancia a fallos, despacho una tarea de prueba inyectada test:ping en el Worker Server de la nube configurada con un máximo de tres reintentos. Ante el fallo transitorio, Asynq aplica la función de backoff exponencial: el primer reintento ocurre a los dos segundos, el segundo a los cuatro segundos y el tercero a los ocho segundos. Al agotar los reintentos, el manejador emite una alerta estructurada en JSON con código DLQ_JOB_FAILED y mueve la tarea al conjunto asynq:archived en Redis, preservando el payload y el rastro del error para soporte operativo sin perder el trabajo."*
* **Ruta de Registro en el Repositorio:** [`docs/entrega2/evidencias/E1/fallo_reintento_backoff_dlq.txt`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/E1/fallo_reintento_backoff_dlq.txt).

---

### Segmento 4: Análisis de Capacidad, Cuellos de Botella y Propuestas de Evolución
* **Tiempo:** `13:15 - 17:15` (Duración: 4:00)
* **Presentador:** `CrispisCas9` (Cristian Castañeda)
* **Estado:** ⚠️ **Solo local / con salvedad en Escenario 2** y ⏳ **Pendiente de ejecución en nube para Escenario 1**
* **Disposición en Pantalla:** Gráficas de percentiles y tablas comparativas de [`capacity-planning/pruebas_de_carga_entrega2.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/capacity-planning/pruebas_de_carga_entrega2.md) en panel izquierdo; en panel derecho, Cloud Monitoring y consola de JMeter / motor Go.

#### 1. Escenario 1: Actividad Académica Concurrente (`13:15 - 14:30`)
* **Puntos Clave y Cifras a Exponer:**
  - **Plan Acordado:** Documentado en [`capacity-planning/escenario1.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/capacity-planning/escenario1.md). Recorrido completo de 14 pasos (catálogo $\to$ curso $\to$ inscripción $\to$ módulos $\to$ unidades $\to$ recursos $\to$ 2 latidos de progreso $\to$ envío de quiz con idempotencia $\to$ reenvío duplicado $\to$ verificación de calificación única).
  - **Mezcla Constante:** 9 lecturas (64%) y 5 escrituras (36%) por sesión; 3 sesiones consecutivas por usuario cubriendo los 3 intentos del quiz con notas 50 $\to$ 100 $\to$ 0.
  - **Decisión de Autenticación:** El login se ejecuta fuera del recorrido medido mediante tokens pre-generados (`capacity_login_tokens.sh`) debido a que el limitador de tasa de 10 logins/minuto por IP estrangularía artificialmente la prueba desde la máquina generadora.
  - **Estado Actual (⏳ / ⚠️):** En `main` se completaron con éxito los **pilotos locales** de 3 y 10 usuarios concurrentes sin ningún fallo (`0 fallos reales, 0 de validación`), demostrando que 30 reenvíos duplicados no generaron calificaciones dobles ([`docs/entrega2/evidencias/H2/README.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/H2/README.md)). La variante de ráfaga de login demostró que ante 15 intentos simultáneos, 10 tienen éxito y 5 reciben 429 como rechazo de negocio esperado. La corrida formal en la nube corresponde al issue abierto H3.
* **Texto Sugerido para la Locución:**  
  *"En el análisis de capacidad, abordo primero el Escenario 1 de actividad académica concurrente. El plan acordado modela el recorrido completo de un estudiante: consulta de catálogo, inscripción activa, lectura de contenidos, registro de avance y presentación de cuestionarios en tres sesiones sucesivas con calificaciones controladas de 50, 100 y 0 puntos. La mezcla se mantiene fija en 64% lecturas y 36% escrituras. La autenticación se realiza con tokens pre-generados para no falsear la medición con el rate limiting de 10 logins por minuto. En las pruebas piloto locales verificamos 420 peticiones con cero fallos y comprobamos en la base de datos que el envío duplicado con la misma Idempotency-Key jamás produce doble calificación. La ejecución formal escalonada hasta 200 usuarios contra la base en Cloud SQL se encuentra programada en el issue H3."*
* **Ruta de Registro en el Repositorio:** [`capacity-planning/escenario1.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/capacity-planning/escenario1.md) y [`docs/entrega2/evidencias/H2/README.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/H2/README.md).

#### 2. Escenario 2: Carga, Procesamiento y Consumo Multimedia (`14:30 - 15:45`)
* **Puntos Clave y Cifras a Exponer:**
  - **Instrumentación Desacoplada en 6 Etapas:** Medición segregada del plano de control frente al plano de datos:
    * Etapa 1: Emisión URL firmada en API (p95: **3–5 ms**).
    * Etapa 2: PUT directo a Storage (p95: **5–38 ms**, rendimiento 30–48 MB/s).
    * Etapa 3: Confirmación y encolado en API (p95: **9–18 ms**).
    * Etapa 4: Espera en cola Asynq (de **303 ms** en baja carga hasta **3.03 s** en saturación).
    * Etapa 5: Transcodificación FFmpeg en Worker (p95: **3.95 s a 6.08 s**).
    * Etapa 6: Tiempo total a available (p95: **4.25 s a 6.07 s**).
  - **Streaming HLS y Pacing:** Petición con cadencia nominal de 6.0s ($\pm 0.5\text{ s}$) demostró margen de buffer positivo (+5.99s) y **0 interrupciones (stalls)**. La descarga en ráfaga (*greedy*) alcanzó entre 120 y 199 MB/s, aislada por el riesgo de costo de egreso advertido en la Nota Técnica 13.
  - **Salvedad Metodológica de QoE (⚠️):** Aclarar con total transparencia que el cálculo de TTFF en el piloto y H5 utilizó una **sonda emuladora de eventos MSE** con un tiempo simulado de decodificación (`simulatedDecodMs = 45 ms`), arrojando un TTFF de 48 a 52 ms. Dado que el pliego exige un reproductor real para certificar TTFF y stalls, dejamos consignado que se trata de una aproximación analítica y ofrecemos su re-medición con un reproductor headless.
  - **Salvedad de Perfiles y Entorno (⚠️):** Las cifras de H5 se obtuvieron con clips sintéticos cortos de 8, 20 y 40 segundos en entorno local; los tres perfiles reales sembrados en G1 son de 2, 10 y 30 minutos (los cuales demandan 43.7s, 3m 52s y 11m 13s de CPU y generan 38, 172 y 504 objetos respectivamente, totalizando 714 objetos en `manifest.json`).
  - **Drenaje de Cola:** Tras la ráfaga de 12 profesores concurrentes, la cola drenó 12 tareas en 6.402 segundos, alcanzando estado final de 0 tareas activas y 57 tareas completadas en base de datos.
* **Texto Sugerido para la Locución:**  
  *"En el Escenario 2 evaluamos la carga, procesamiento y streaming HLS a lo largo de cinco niveles escalonados con concurrencia fija en 2 workers. Nuestra instrumentación desacoplada separa rigurosamente el plano de control del plano de datos. En las etapas 1 y 3, la API responde siempre en menos de 18 milisegundos porque no transfiere video. En la etapa 2, el almacenamiento absorbe subidas a más de 30 megabytes por segundo. El streaming a cadencia real mantiene un margen de buffer de casi seis segundos y cero congelamientos. Es fundamental señalar dos salvedades metodológicas: primero, la métrica de TTFF reportada de 48 milisegundos proviene de una sonda emuladora de eventos HTML5 con cuarenta y cinco milisegundos de decodificación simulada, ya que no se utilizó un navegador físico; y segundo, esta corrida empleó clips sintéticos en entorno de prueba, por lo que recomendamos una re-corrida en la nube con los perfiles reales de G1 de 2, 10 y 30 minutos antes del cierre definitivo. Al concluir la inyección, observamos el drenaje total de la cola en 6.4 segundos, validando en PostgreSQL que el cien por ciento de las tareas finalizaron en estado completed sin trabajos huérfanos."*
* **Ruta de Registro en el Repositorio:** [`capacity-planning/pruebas_de_carga_entrega2.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/capacity-planning/pruebas_de_carga_entrega2.md), [`docs/entrega2/evidencias/H5/README.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/H5/README.md) y [`docs/entrega2/evidencias/G1/manifest.json`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/G1/manifest.json).

#### 3. Cuello de Botella Identificado y Propuestas de Evolución (`15:45 - 17:15`)
* **Puntos Clave y Cifras a Exponer:**
  - **Descarte Cuantitativo:**
    * Servidor Web (API): Utilización de CPU < 15%, latencias p95 < 18 ms, cero errores 5xx. $\to$ **Descartado**.
    * Almacenamiento de Objetos: Subidas > 30 MB/s, descargas > 120 MB/s, cero códigos 429/503. $\to$ **Descartado**.
    * Base de Datos Cloud SQL: Transacciones de confirmación < 10 ms, pool de conexiones estable. $\to$ **Descartado**.
  - **Identificación:** El **cuello de botella primario** es la **capacidad de cómputo (vCPU) en el Worker Server durante la transcodificación FFmpeg (Etapa 5)**. Con `WORKER_CONCURRENCY=2`, cada transcodificación satura al 100% una vCPU física. Cuando la tasa de llegada supera 2 videos simultáneos, las tareas se acumulan en Redis y el tiempo de espera en cola se incrementa en un 1000% (de 303 ms a 3.03 s).
  - **Propuestas de Evolución Sustentadas:**
    1. **Cloud CDN delante de derivados HLS:** Segmentos estáticos en caché edge con > 95% de *cache hit ratio*, reduciendo latencias a < 15 ms y eliminando costos críticos de egreso a internet (Nota Técnica 13).
    2. **Escalado Horizontal de Workers (MIG / Autoescalado):** Disparo de escalamiento basado en `worker.queue.oldest_pending_age_seconds > 45s`, duplicando la capacidad de procesamiento (+2 vCPUs por nodo `e2-highcpu-2`).
    3. **Re-dimensionamiento Vertical:** Migración a instancias `c2-standard-4` (4 vCPUs dedicadas, 16 GiB RAM) permitiendo elevar de forma segura `WORKER_CONCURRENCY=4` sin riesgo de contención de CPU ni caída por falta de memoria (OOM).
* **Texto Sugerido para la Locución:**  
  *"El análisis cuantitativo descarta de forma categórica a la API, a la base de datos y al almacenamiento como limitantes: todos operaron con holgura. El cuello de botella primario radica de forma indiscutible en la capacidad de cómputo de la CPU del Worker Server durante la transcodificación con FFmpeg. Con dos vCPUs dedicadas y concurrencia fija en dos, cada flujo de video satura un núcleo completo; cuando la tasa de llegada supera la tasa de servicio, la cola se satura linealmente. Para evolucionar el sistema proponemos tres mejoras sustentadas en datos: primero, interponer Cloud CDN para servir los segmentos HLS desde el edge con un cache hit superior al 95%, reduciendo latencias a menos de quince milisegundos y protegiendo nuestro presupuesto del costo de egreso; segundo, configurar un grupo de instancias administrado que autoescale los workers cuando la antigüedad de tareas en cola supere los cuarenta y cinco segundos; y tercero, migrar a máquinas c2-standard-4 para duplicar la concurrencia a cuatro procesos por nodo sin riesgo de sobrecargar la memoria."*
* **Ruta de Registro en el Repositorio:** [`capacity-planning/pruebas_de_carga_entrega2.md#8-identificación-del-cuello-de-botella-primario-sustentado-con-evidencia`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/capacity-planning/pruebas_de_carga_entrega2.md#8-identificación-del-cuello-de-botella-primario-sustentado-con-evidencia) y [`docs/entrega2/evidencias/H5/analisis_cuello_de_botella.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/H5/analisis_cuello_de_botella.md).

---

### Segmento 5: Conclusiones, Checklist y Cierre
* **Tiempo:** `17:15 - 18:00` (Duración: 0:45)
* **Presentador:** `CrispisCas9` (Cristian Castañeda)
* **Estado:** ✅ **Nube**
* **Disposición en Pantalla:** Tabla resumen de criterios cumplidos, enlace al repositorio GitHub y tag de entrega `entrega-2`.
* **Texto Sugerido para la Locución:**  
  *"En conclusión, en representación de nuestro equipo de trabajo, hemos demostrado el cumplimiento integral de los cinco componentes evaluados para esta segunda entrega: cómputo distribuido en máquinas virtuales con contenedores Docker, base de datos administrada privada Cloud SQL, almacenamiento de objetos Cloud Storage con permisos diferenciados e IAM de menor privilegio, y la caracterización rigurosa de capacidad identificando el cuello de botella en transcodificación y sus rutas de evolución. El código, los manifiestos de Terraform y la totalidad de los registros de evidencia se encuentran versionados en nuestro repositorio bajo el tag entrega-2, y el enlace al presente video queda consignado en el README principal con acceso para el equipo docente. Muchas gracias."*
* **Ruta de Registro en el Repositorio:** [`docs/entrega2/evidencias/I5/README.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/docs/entrega2/evidencias/I5/README.md) y [`README.md`](file:///mnt/c/Users/User/Desktop/MBC_IV/Soluciones Cloud/Proyectos/P2_data/Plataforma-MOOC/README.md) (coordinado con issue I4).

---

## 5. Matriz de Pendientes y Bloqueos para la Grabación Final

Antes de iniciar la sesión de grabación definitiva del video de sustentación, se debe coordinar la atención de las siguientes tareas asociadas a los issues abiertos y salvedades metodológicas:

| Item | Bloqueo / Asunto Pendiente | Issue Ligado | Responsable Sugerido | Acción Requerida antes de Grabar |
| :---: | :--- | :---: | :---: | :--- |
| **1** | **Ejecución Formal Escenario 1 en la Nube** | Issue #133 (H3) | `tmichelldiaz` | Cargar `capacity_data.sql` en Cloud SQL por SSH IAP, ejecutar `run_escenario1.sh` contra la nube y registrar latencias reales p95 de catálogo, inscripción y quizzes en el informe consolidado. |
| **2** | **Re-corrida Escenario 2 con Perfiles Reales en Nube** | Issue H5 / H4 | `DiegoOrtizRuiz` | Ejecutar `run_capacity_escenario2.sh` contra `https://34.24.52.111.sslip.io` empleando los perfiles de G1 (2, 10 y 30 min) para sustituir las cifras locales de clips sintéticos por métricas cloud definitivas. |
| **3** | **Decisión Metodológica de Reproductor Real (TTFF)** | Issue H5 | `DiegoOrtizRuiz` | Decidir si se conserva la declaración explícita de "sonda emuladora de eventos MSE" o si se ejecuta una prueba complementaria con navegador real headless (Playwright) para medir TTFF sin decodificación simulada. |
| **4** | **Actualización del README Principal** | Issue I4 | Dueño de I4 / `fredyxander` | Actualizar el `README.md` raíz (que aún referencia MinIO y Docker local de la Entrega 1) incorporando la URL de la nube, credenciales docentes privadas y el enlace a este video de sustentación. |
| **5** | **Documento de Arquitectura Consolidado** | Issue I1 / I2 | Equipo | Consolidar las decisiones arquitecturales finales en `docs/entrega2/` referenciando las evidencias de B3, C1, C3, D2, E1 y G4. |
| **6** | **Procedimiento de Apagado y Recreación** | Issue I6 | `CrispisCas9` | Verificar el runbook de recreación para la sustentación síncrona en caso de que la base administrada sea suspendida por política de costos (`CONFIGURACION_Y_COSTOS.md` §5). |

---

## 6. Checklist de Criterios de Aceptación del Issue #140 ([I5])

| # | Criterio de Aceptación del Enunciado (p. 7) | Estado en este Guion | Dónde se Evidencia en el Repo |
| :---: | :--- | :---: | :--- |
| **1** | **Duración máxima de 20 minutos** | ✅ Cumplido | Presupuesto fijado en **18:00 minutos** con desglose por bloques (margen de 2 min). |
| **2** | **Correspondencia con servicios del proveedor (GCP)** | ✅ Cumplido | Segmento 1 detalla Compute Engine, VPC, Cloud SQL y Cloud Storage referenciando Terraform. |
| **3** | **Recorrido funcional sobre el entorno cloud** | ✅ Cumplido | Segmento 2 cubre identidad, SMTP, autoría, publicación, inscripción, quizzes y badges sobre `34.24.52.111.sslip.io`. |
| **4** | **Carga directa al almacenamiento de objetos** | ✅ Cumplido | Segmento 3.1 muestra URL firmada V4 y subida directa con PUT sin cursar bytes por la API. |
| **5** | **Acceso autorizado (URLs firmadas y roles)** | ✅ Cumplido | Segmento 3.1 y 3.2 evidencian URLs firmadas V4, rechazo anónimo (403) y condiciones CEL disjuntas. |
| **6** | **Procesamiento asíncrono y estado terminal** | ✅ Cumplido | Segmento 3.3 muestra encolado en Redis Asynq, transcodificación HLS en worker y persistencia `completed`. |
| **7** | **Persistencia en base de datos administrada** | ✅ Cumplido | Segmentos 1, 2 y 3 evidencian Cloud SQL PostgreSQL en IP privada, migraciones y auditoría inmutable. |
| **8** | **Idempotencia verificada** | ✅ Cumplido | Segmento 3.4 demuestra entrega duplicada en worker (4 ms) y Segmento 2 demuestra quiz submission idempotente. |
| **9** | **Fallo con reintento y backoff exponencial** | ✅ Cumplido | Segmento 3.5 detalla tarea `test:ping` en worker cloud con 3 reintentos (2s, 4s, 8s) y paso a DLQ (`asynq:archived`). |
| **10** | **Resultados de ambos escenarios de capacidad** | ⚠️ Parcial | Escenario 2 documentado con métricas de 6 etapas; Escenario 1 con pilotos locales y plan formal (H3 pendiente en nube). |
| **11** | **Identificación de cuello de botella y evolución** | ✅ Cumplido | Segmento 4.3 sustenta saturación de vCPU en workers y modela Cloud CDN, autoescalado MIG y `c2-standard-4`. |
| **12** | **Parámetros y archivos variados (perfiles, extensiones, roles)** | ✅ Cumplido | Sección 2 consolida tabla exhaustiva con perfiles corto/medio/largo, tipos video/audio/doc/img, y roles diversos. |
| **13** | **Presentación unificada del equipo de desarrollo** | ✅ Cumplido | Conducción integral a cargo de `CrispisCas9` (Cristian Castañeda), articulando el trabajo de los 4 integrantes (`CrispisCas9`, `fredyxander`, `DiegoOrtizRuiz`, `tmichelldiaz`). |
| **14** | **Enlace desde README con acceso docente** | ⏳ Pendiente | Coordinación formal establecida con responsable del issue I4. |
