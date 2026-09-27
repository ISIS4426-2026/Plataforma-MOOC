# Evidencia C3 — Cloud Storage: bucket, organización de objetos, IAM por componente y CORS (issue #120)

La configuración declarativa de almacenamiento está en [`../../../../infra/terraform/storage.tf`](../../../../infra/terraform/storage.tf) y sus variables y salidas en [`variables.tf`](../../../../infra/terraform/variables.tf) y [`outputs.tf`](../../../../infra/terraform/outputs.tf).

## Criterios de Aceptación y Archivos de Evidencia

| Archivo | Criterio del Enunciado que Demuestra |
| :--- | :--- |
| [`api_rechaza_escritura_derivados.txt`](./api_rechaza_escritura_derivados.txt) | **La cuenta de la API no puede escribir en el prefijo de derivados** (HTTP 403 Forbidden). El worker sí escribe en `hls/` y es rechazado en `originals/`. |
| [`objeto_no_publico_sin_firma.txt`](./objeto_no_publico_sin_firma.txt) | **Un objeto no es legible por URL pública sin firma** (HTTP 403 Forbidden para accesos anónimos; HTTP 200 OK únicamente con URL firmada V4). |
| [`verificacion_cors_carga_directa.txt`](./verificacion_cors_carga_directa.txt) | **Una carga directa desde un origen externo pasa la verificación CORS** (Preflight `OPTIONS` responde 200 con cabeceras `Access-Control-Allow-*` y subida `PUT` autorizada). |
| [`plan_c3.txt`](./plan_c3.txt) | Plan declarativo de Terraform (`Plan: 9 to add, 0 to change, 0 to destroy`). |
| [`apply_c3.txt`](./apply_c3.txt) | Aprovisionamiento efectivo en GCP (`Apply complete! Resources: 9 added, 0 changed, 0 destroyed`). |
| [`iam_politica_almacenamiento.txt`](./iam_politica_almacenamiento.txt) | Política IAM efectiva del bucket con condiciones CEL y configuración de seguridad y ciclo de vida. |

---

## 1. Bucket en la Misma Región y sin Acceso Público

El bucket se aprovisiona en la misma región definida en B1 y B2 (`us-east1`, South Carolina) para asegurar mínima latencia y cero costo de transferencia de datos interna con las máquinas virtuales de la aplicación:

- **Nombre:** `plataforma-mooc-entrega2-media` (definido mediante `var.storage_bucket_name`).
- **Región:** `us-east1` (alineada con `var.region`).
- **Nivel de acceso:** `uniform_bucket_level_access = true` (garantiza gobierno centralizado mediante IAM y habilita condiciones CEL).
- **Aislamiento de acceso público:** `public_access_prevention = "enforced"`. Esta directiva de Google Cloud prohíbe de forma irrevocable cualquier asignación a `allUsers` o `allAuthenticatedUsers`, impidiendo que los objetos sean accesibles públicamente sin firma o credenciales.

---

## 2. Prefijos Lógicos del Dominio

Siguiendo el contrato de organización declarado en [`internal/storage/keys.go`](../../../../internal/storage/keys.go), el bucket implementa cuatro áreas lógicas mediante objetos marcadores aprovisionados por Terraform:

1. `originals/`: Almacenamiento de archivos multimedia originales subidos directamente por profesores.
2. `hls/`: Derivados transcodificados (manifiestos maestro `master.m3u8`, variantes y segmentos `.ts`) producidos por el worker.
3. `documents/`: Documentos académicos complementarios (PDF, DOCX, PPTX).
4. `thumbnails/`: Miniaturas e imágenes de portada (`poster.jpg`).

---

## 3. IAM Diferenciado por Componente (Mínimo Privilegio)

En lugar de compartir una única cuenta de servicio o un rol genérico de administración sobre el almacenamiento, se asignan permisos estrictamente disjuntos gobernados por condiciones CEL (*Common Expression Language*):

| Componente | Identidad | Rol en el Bucket | Condición IAM (CEL) | Efecto Práctico |
| :--- | :--- | :--- | :--- | :--- |
| **API** | `sa-web-server` | `roles/storage.objectViewer` | Ninguna | Lectura y consulta de metadatos (`StatObject`, `GetObject`) en cualquier prefijo. |
| **API** | `sa-web-server` | `roles/storage.objectCreator` | `resource.type == "storage.googleapis.com/Object" && (resource.name.startsWith(".../originals/") \|\| resource.name.startsWith(".../documents/") \|\| resource.name.startsWith(".../thumbnails/"))` | Permite autorizar cargas directas únicamente en originales, documentos y miniaturas. **Rechaza con HTTP 403 Forbidden cualquier intento de escritura en `hls/`**. |
| **Worker** | `sa-worker-server` | `roles/storage.objectViewer` | Ninguna | Lectura de videos originales para el pipeline de procesamiento y transcodificación con FFmpeg. |
| **Worker** | `sa-worker-server` | `roles/storage.objectUser` | `resource.type == "storage.googleapis.com/Object" && resource.name.startsWith(".../objects/hls/")` | Permite escribir, actualizar y gestionar exclusivamente los derivados HLS. **No puede modificar ni sobreescribir originales**. |

Adicionalmente, tal como se configuró en B2, `sa-web-server` posee `roles/iam.serviceAccountTokenCreator` sobre su propia identidad para firmar URLs criptográficamente (sin requerir llaves privadas en disco), mientras que `sa-worker-server` carece de ese rol (el worker nunca emite URLs firmadas).

La verificación empírica en [`api_rechaza_escritura_derivados.txt`](./api_rechaza_escritura_derivados.txt) demuestra que al intentar subir un objeto en `hls/` con la cuenta de la API se obtiene:
```json
{
  "error": {
    "code": 403,
    "message": "sa-web-server@plataforma-mooc-entrega2.iam.gserviceaccount.com does not have storage.objects.create access to the Google Cloud Storage object. Permission 'storage.objects.create' denied on resource '//storage.googleapis.com/projects/_/buckets/plataforma-mooc-entrega2-media/objects/hls/test_api_forbidden.txt'..."
  }
}
```

---

## 4. Política CORS para Cargas Directas

Para permitir que los clientes web o navegadores frontend carguen archivos de gran tamaño directamente al bucket mediante la URL firmada emitida por la API (evitando saturar el ancho de banda del Web Server):

```hcl
cors {
  origin          = var.storage_cors_origins # ["*"]
  method          = ["GET", "HEAD", "PUT", "OPTIONS"]
  response_header = ["*"]
  max_age_seconds = 3600
}
```

La verificación en [`verificacion_cors_carga_directa.txt`](./verificacion_cors_carga_directa.txt) demuestra:
1. Una solicitud de inspección previa (preflight `OPTIONS`) con cabeceras `Origin: https://plataforma-mooc-frontend.example.com` y `Access-Control-Request-Method: PUT` responde `HTTP/2 200` con `access-control-allow-origin: *` y `access-control-allow-methods: GET,HEAD,PUT,OPTIONS`.
2. Una petición de carga directa `PUT` con URL firmada V4 se procesa exitosamente (HTTP 200 OK) respetando la política de origen cruzado.

---

## 5. Política de Ciclo de Vida para Temporales

El bucket incluye una regla de ciclo de vida administrada:

```hcl
lifecycle_rule {
  action {
    type = "AbortIncompleteMultipartUpload"
  }
  condition {
    age = 1
  }
}
```

Esta regla aborta y elimina automáticamente cualquier carga fragmentada (multipart upload) que quede incompleta tras 24 horas, evitando cargos acumulados por segmentos huérfanos que nunca llegaron a consolidarse.

---

## 6. Seguridad y Manejo de Secretos

Siguiendo las políticas de seguridad de la entrega:
- **Cero archivos de llave (.json de servicio):** No se crearon ni descargaron llaves para las cuentas de servicio.
- **Sin credenciales en las evidencias:** Todos los tokens de autenticación utilizados en los scripts de prueba son efímeros y han sido omitidos o redactados (`[REDACTED]`).
- **Estado protegido:** El archivo de estado de Terraform (`terraform.tfstate`) se almacena en el bucket privado remoto del proyecto y no reside en el repositorio.
