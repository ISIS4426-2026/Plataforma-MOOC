# Evidencia E3 — Estado externo: objetos y estado de trabajos accesibles desde cualquier instancia ([#218](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/218))

| Criterio de aceptación | Estado | Dónde |
| :--- | :---: | :--- |
| Un recurso cargado por una instancia es legible y procesable desde otra | ⏳ por diseño ✅; falta la prueba en vivo con 2 instancias de cada grupo | §1, §5 |
| El estado de un trabajo es consultable desde cualquier instancia de API | ⏳ por diseño ✅; falta la prueba en vivo | §2, §5 |
| Inventario de lo que queda en disco local y por qué es reconstruible | ✅ | §3 |
| Anotado en `NOTAS_TECNICAS.md` | ✅ nota 50 | |

Hallazgo con arreglo: **un trabajo interrumpido dejaba su carpeta de trabajo en el disco del worker, y nada la limpiaba** (§4).

## 1. Dónde vive cada cosa (revisado en el código)

| Dato | Dónde vive | Quién lo escribe y lo lee |
| :--- | :--- | :--- |
| Originales, documentos, imágenes | **Bucket de medios** (GCS) | El navegador sube **directo** con URL firmada; la API solo firma y confirma; el worker lo lee (`StatObject`/`GetObject`) |
| Derivados HLS | **Bucket de derivados** (GCS), `hls/<id>/…` | Solo el worker escribe; el navegador lee directo |
| Estado de procesamiento de un recurso | **PostgreSQL** (`processing_status`) | El worker lo cambia (en cola → `processing` → `completed`/`failed`); cualquier API lo lee |
| El trabajo en cola (asynq) | **Redis de la VM de la cola**, fuera de los dos grupos | Cualquier API encola; cualquier worker consume |
| Sesiones, limitador de tasa, idempotencia de la API | **Redis** | Cualquier API |
| Marcas de idempotencia del worker | **Redis** (C7); respaldo en memoria, y `mediaproc` no reprocesa un recurso ya `completed` sobre el mismo objeto | Cualquier worker |
| Archivos en disco que continúan una operación de negocio | **Ninguno** (§3) | — |

## 2. El estado de un trabajo se lee desde cualquier API

El estado no está en la memoria de ninguna instancia: sale de `processing_status` en PostgreSQL. La interfaz del profesor lo muestra al
listar los recursos de una unidad (D3 capturó el cambio en vivo: en cola a los 2,1 s y `Disponible` a los 6,2 s, con una sola instancia).
`mediaproc.go` es **idempotente**: si el recurso ya está `completed` sobre el mismo objeto, un segundo reparto del trabajo no hace nada.

## 3. Inventario del disco local

| Instancia | Qué hay en disco | ¿Por qué es reconstruible? | Limpieza |
| :--- | :--- | :--- | :--- |
| **API** | Nada de la aplicación: ninguna escritura de archivos en `internal/` ni en `cmd/api/` (revisado buscando `os.Create`, `WriteFile`, `MkdirTemp`). Solo capas de Docker y el registro JSON de los contenedores | No es dato de negocio | El disco de la instancia desaparece con ella |
| **Worker** | Volumen `worker_scratch` en `/tmp/mooc-media`: por trabajo, una carpeta `mooc-media-*` con **el original descargado** (hasta 2 GiB) y las renditions HLS | El original está en el bucket de medios; las renditions se regeneran de él. Un trabajo repetido **descarga de nuevo** | `defer os.RemoveAll` en cada salida normal, incluida la cancelación; **barrido al arrancar** (§4) para lo que deja una interrupción |
| **Cola** | Los datos de Redis (la cola y las sesiones) | Es la única copia del estado de la **cola**; vive fuera de los grupos a propósito (B1). Su punto único de fallo está en la nota 30 | Fuera del alcance de E3 |
| **Generador** (G1) | Resultados de JMeter | Se copian a la evidencia | La VM se destruye al terminar |

**Ninguna copia local es la única fuente de un dato necesario para continuar una operación.** El único dato que solo existe en una instancia es
la carpeta temporal del worker, y un trabajo que se reintenta la reconstruye desde el bucket.

## 4. Hallazgo y arreglo: lo que una interrupción dejaba en el disco

`defer os.RemoveAll(workDir)` no corre cuando el proceso muere a la fuerza: el plazo de drenaje de C7 vence con un video largo en codificación
(un video de 30 min tarda ~97 min en procesarse, nota 24), el contenedor se mata por memoria, o la VM se detiene. La carpeta queda en
`worker_scratch`, **que sobrevive al reinicio del contenedor**. El trabajo reintentado crea una carpeta **nueva**: nadie reclamaba la vieja.
Con un disco de 30 GiB y originales de hasta 2 GiB, unos pocos trabajos interrumpidos llenan el disco y los siguientes mueren con
«no space left».

Arreglo: `handler.SweepStaleScratch` (`internal/worker/handler/workdir.go`), que se ejecuta **una vez al arrancar el worker, antes de aceptar
trabajos**. Solo borra directorios con el prefijo exacto `mooc-media-`, no sigue enlaces simbólicos y no hace nada si no hay directorio de
trabajo configurado (sería el temporal del sistema, compartido). Cuatro pruebas en `workdir_test.go`: borra solo lo suyo y cuenta lo liberado;
no barre el temporal del sistema; tolera que no exista; no sigue enlaces (esta se omite en Windows, que no permite crearlos sin privilegios).
Su única suposición está escrita: el directorio pertenece a **un** solo worker, como en el despliegue (un contenedor por VM, un volumen con nombre).

## 5. Pendiente: la prueba en vivo con dos instancias de cada grupo

El criterio pide una prueba, no solo un razonamiento. Plan: `terraform apply` con `-var api_min_replicas=2 -var worker_min_replicas=2`
(existen para demostraciones), repetir varias veces *presign → subida directa → confirmación → lectura del estado* por el balanceador,
**registrando el `X-Instance-ID` de cada paso**, y comprobar en los registros qué worker procesó cada recurso. Luego volver a 1+1.
Coste aproximado: 0,15 USD/h durante ~30 min.

## Nunca

Credenciales, llaves ni secretos.
