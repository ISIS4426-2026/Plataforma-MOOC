# Notas técnicas de la Entrega 2

Hallazgos que afectan a más de un issue. Cada uno indica dónde falla, para que
no se redescubra a mitad de una corrida.

---

## 1. La URL firmada lleva el host dentro de la firma

**Afecta a:** C4 (apuntar API y workers al bucket), G2 (colecciones Postman), H4 y H5 (escenario 2).

Una URL prefirmada incluye el host en el cálculo de la firma. Cambiar el host
de la URL después de emitirla la invalida.

La consecuencia es que **`S3_ENDPOINT` tiene que ser la dirección que alcanza el
cliente, no la que usa la API internamente**. Es el mismo problema de clase que
`APP_BASE_URL`: un valor que solo sirve dentro de la red de contenedores rompe a
cualquier cliente externo.

### En local

`docker-compose.yml` inyecta `S3_ENDPOINT=http://minio:9000`. La API firma
correctamente, pero la URL apunta al nombre interno de la red:

```
http://minio:9000/mooc-storage/originals/<stable_id>/<uuid>.mp4?X-Amz-Signature=...
```

Desde el host eso no resuelve, y reescribir `minio` por `localhost` rompe la
firma. Para probar la carga directa en local hay dos caminos:

- Ejecutar el PUT desde dentro de la red, que es como se verificó A2:

  ```bash
  docker compose exec -T minio curl -X PUT \
    -H 'Content-Type: video/mp4' --upload-file /tmp/clase.mp4 "$UPLOAD_URL"
  ```

- O arrancar la API con `S3_ENDPOINT=http://localhost:9000`, que hace las URLs
  utilizables desde el host a cambio de que la API no alcance MinIO para
  `StatObject`. Sirve para inspeccionar una URL, no para el flujo completo.

### En la nube

No hay conflicto: el endpoint del bucket administrado es la misma dirección
para la API y para el cliente. Pero **hay que verificarlo explícitamente en C4**
en lugar de asumirlo, y el entorno de Postman de G2 debe apuntar al endpoint
público, no a un alias interno.

### Para el escenario 2

El plan de H4 separa el tráfico de control de la transferencia de archivos. El
generador de carga corre **fuera de las dos VMs**, así que consume las URLs
firmadas desde fuera de la VPC: si el endpoint quedó configurado con una
dirección privada, las transferencias fallarán con firma inválida y el error
parecerá un problema de capacidad cuando es de configuración.

---

## 2. MinIO retiró sus imágenes públicas

**Afecta a:** cualquier uso de `docker compose up`.

`quay.io/minio/minio` responde `401 Unauthorized` en todos los tags y
`docker.io/minio/minio` devuelve `404`: el repositorio ya no existe. Fijar una
versión sobre ese origen no resuelve nada.

El Compose pasó a `bitnamilegacy/minio:2025.7.23-debian-12-r5`, que es el mismo
servidor MinIO empaquetado por Bitnami. Diferencias que obligaron a ajustar el
servicio:

| | MinIO oficial | Bitnami |
| :--- | :--- | :--- |
| Arranque | `command: server /data` | entrypoint propio, sin `command` |
| Usuario | root | uid 1001 |
| Datos | `/data` | `/bitnami/minio/data` |
| Buckets iniciales | servicio `minio-init` con `mc` | variable `MINIO_DEFAULT_BUCKETS` |

Como Bitnami crea los buckets al arrancar, **el servicio `minio-init` y
`scripts/init-minio.sh` se eliminaron**: solo creaban el bucket. Están en el
historial de git si hiciera falta recuperarlos.

Todas las imágenes del Compose quedaron con versión fija. `latest` es
exactamente lo que convirtió esto en una sorpresa en vez de una decisión.

---

## 3. Las posiciones del seed estaban desfasadas en uno

**Afecta a:** G1 (datos sintéticos), G2 (Postman), G3 (E2E), y cualquier prueba
de carga que cree contenido.

Los repositorios asignan `Position = COUNT(hermanos)`, o sea **base 0**, de
forma consistente en módulos, unidades y recursos. El seed los insertaba **base
1**.

El efecto: sobre datos sembrados, la primera creación de un recurso por la API
siempre colisiona.

```
pq: duplicate key value violates unique constraint "uq_resources_unit_position"
```

Se corrigió `scripts/seeds/synthetic_data.sql` desplazando las posiciones a base
0. No se tocó el código: es consistente consigo mismo y el paquete
`internal/domain/ordering` también razona con índices base 0.

---

## 4. El estado `available` del enunciado es `completed` en el código

**Afecta a:** A3 (worker de media), I1 e I3 (documentación).

El enunciado habla del estado `available`. El esquema de la Entrega 1 usa
`completed` con ese mismo significado, y `course.validateForPublish` ya lo lee
para decidir si un curso puede publicarse.

Se conservó `completed`. Renombrarlo obligaría a una migración y a tocar las
reglas de publicación de la Entrega 1 sin ganar nada. **Conviene declarar esta
equivalencia en el documento de arquitectura** para que un evaluador que busque
`available` sepa dónde mirar.

---

## 5. Carga reanudable: brecha declarada

**Afecta a:** I1 (documentación de decisiones).

El enunciado del proyecto pide carga «multipart directa y reanudable durante 24
horas». Lo implementado es una URL PUT firmada con vigencia de 24 horas, que es
directa pero **no reanudable**: reanudar requiere sesiones de carga reanudable
del proveedor, que es otro mecanismo.

La Entrega 1 no construyó nada de media, así que no hay reanudación que
preservar, y el enunciado de la Entrega 2 excusa explícitamente lo que no
estuviera implementado antes. Se deja como brecha declarada en lugar de
arriesgar el camino que se evalúa.
