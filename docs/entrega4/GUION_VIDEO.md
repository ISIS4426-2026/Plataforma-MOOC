# Guion del video — Entrega 4 (≈ 14 min, máximo 20)

Una sola persona puede grabarlo de corrido: cada bloque dice **qué se ve**, **qué se dice** y de dónde sale la cifra. Las cifras
están en [`README.md`](README.md); si el video y el documento difieren, manda el documento.

## Antes de grabar (10 min)

- [ ] Cloud Run, Cloud SQL y la VM `mooc-queue-server` **encendidos** (si no, el backend no arranca).
- [ ] **Calentar los workers:** sube un video de prueba un rato antes. Con mínimo 0, el primero tarda ~10 s más por el arranque en
      frío y se vería «en cola» demasiado tiempo.
- [ ] Un curso borrador con módulo, unidad y un recurso de video vacío (como en el video de la Entrega 3). Video corto para
      subir: `scratch/demo/clase_demo_2s.mp4`.
- [ ] **Iniciar sesión antes de grabar** como `profesor1@plataforma-mooc.test` (nadie escribe una contraseña en pantalla).
- [ ] Pestañas abiertas y en orden: (1) `docs/entrega4/README.md` en el diagrama, (2) consola de Cloud Run, (3) frontend,
      (4) consola de Pub/Sub, (5) Logs Explorer con la consulta de abajo, (6) consola de IAM, (7) las gráficas
      `docs/entrega4/evidencias/escalado/*.png`.
- [ ] Zoom del navegador al 125 %. Que no se vea tu correo personal, ni valores de secretos, ni el estado de Terraform.

Rangos de tiempo para el Logs Explorer (hora de Colombia, UTC-5): el recorrido completo con la reentrega tolerada fue el **3 de nov.
entre 19:21 y 19:22**; las tandas de la prueba de escalado, el **3 de nov. 20:23–20:28** (9 videos) y el **4 de nov. 00:51–00:57** (12 videos).
Los registros se conservan 30 días. Si se prefiere un rastro reciente, basta subir un video en vivo en el bloque anterior.

Consulta del Logs Explorer (pegarla antes de grabar):

```text
resource.type="cloud_run_revision"
(resource.labels.service_name="mooc-backend" OR resource.labels.service_name="mooc-workers")
("media job published" OR "media job received" OR "media job completed" OR "media job acknowledged")
```

## Guion

| Tiempo | Qué se ve | Qué decir |
| :--- | :--- | :--- |
| **0:00–1:00** Arquitectura | El diagrama del README (§2) | «Tres servicios independientes en Cloud Run: **frontend**, **backend** y **workers**. El backend **publica** los trabajos de video en un topic de **Pub/Sub** y responde sin esperar; Pub/Sub los empuja con una suscripción **autenticada** al servicio de workers, que confirma solo cuando termina y guarda. Se conservan Cloud SQL y los buckets; Redis ya no es la cola, solo sesiones y limitador.» |
| **1:00–3:00** Los tres servicios | Consola de Cloud Run: lista de servicios → entrar a cada uno → pestaña **Revisiones** o **YAML** | «Región **us-east1**, **máximo 3 instancias** en los tres. Frontend: 1 vCPU, 256 MiB, concurrencia 80, mínimo 0. Backend: 1 vCPU, 512 MiB, concurrencia 80, **mínimo 1** para evitar el arranque en frío del login. Workers: **2 vCPU, 4 GiB, concurrencia 1, timeout 590 s**, mínimo 0. Un video por instancia; 4 GiB porque `/tmp` en Cloud Run es memoria. Una sola aplicación por instancia.» Mostrar las URLs. |
| **3:00–5:30** Recorrido en vivo | Frontend: el profesor abre el curso, sube `clase_demo_2s.mp4`, se ve **en cola → disponible**, publica | «El navegador solo conoce el frontend: nginx reenvía `/api` al backend, así que la **cookie de sesión y el CSRF no cambian**. El video va **directo al bucket** con URL firmada. Al confirmar, el backend publica el mensaje y responde 202; el worker lo procesa.» |
| **5:30–7:00** Pub/Sub | Consola de Pub/Sub: topic `mooc-media-jobs` → suscripción `mooc-media-jobs-push` (Detalles) | «Suscripción **push** con **token OIDC** firmado por una cuenta de servicio; **plazo de confirmación de 600 s**, reintento de 10 a 600 s, y **mensajes muertos tras 20 intentos**. El **timeout del worker (590 s) es menor que el plazo de confirmación**: así Cloud Run corta antes de que Pub/Sub dé el mensaje por perdido y no hay dos instancias haciendo el mismo trabajo.» |
| **7:00–9:00** Seguimiento de una tarea | Logs Explorer con la consulta; abrir las 4 líneas de **un mismo `message_id`** | «Este es el recorrido de una tarea: **`media job published`** en el backend, **`received`** en el worker, **`completed`** y **`acknowledged`**, todos con el mismo `message_id`. La confirmación llega **después** de terminar y guardar el resultado.» Mostrar el `delivery_attempt`. |
| **9:00–10:30** Reentregas | En el mismo Logs Explorer, buscar `"redelivered after it finished"` | «Pub/Sub entrega **al menos una vez**. Si el mismo trabajo llega dos veces, el worker lo reconoce —clave de idempotencia en Redis, y la canalización omite un recurso ya `completed`— y **confirma sin repetir el trabajo**. Aquí un video se publicó dos veces y se procesó una sola.» |
| **10:30–12:00** Acceso y secretos | (a) Terminal: `curl -X POST <url-workers>/pubsub/push -d '{}'` → **403**. (b) IAM de `mooc-workers` (Seguridad/Permisos): solo `sa-pubsub-push` con *Invoker*. (c) Cloud Run → Variables: `DATABASE_URL` como **referencia a un secreto** | «El servicio de **workers es privado**: sin credencial recibe 403 y no llega al código; solo la cuenta de la suscripción tiene `run.invoker`. La contraseña de la base y la de correo **no están en el repositorio ni en la imagen**: vienen de **Secret Manager**. Cada cuenta lee solo lo que usa, con permisos **mínimos**.» No abrir el valor de ningún secreto. |
| **12:00–14:00** Escalado | `api_instancias.png`, `workers_pendientes.png`, `workers_instancias.png` y la tabla de §9.2 | «Carga controlada desde una VM en la misma región. **Backend:** de 50 a 500 usuarios concurrentes, de **1 a 3 instancias** y hasta 1.379 peticiones por segundo sin errores. **Workers:** 12 videos a la vez; el backend acepta las confirmaciones en 195 ms; los workers pasan de **0 a 3 instancias**, los mensajes pendientes llegan a 11 y se drenan en 4 min 11 s.» |
| **14:00–15:00** Lo que encontramos | Tabla de §9.2, las dos columnas | «En la primera corrida, con **5 intentos**, dos mensajes se confirmaron en el último intento permitido: cuando hay más mensajes que instancias, Cloud Run rechaza y **cada rechazo cuenta como intento**. Subimos el límite a 20 y repetimos con más carga: 11 de 12 en el primer intento y ninguno a mensajes muertos.» |
| **15:00–16:00** Límites y cierre | README §10 | «Limitaciones: Redis sigue en una VM; en Cloud Run no hay IP de proxy fija, así que el limitador de login comparte cupo y lo subimos; y un video cuyo procesamiento pase de ~10 minutos no cabe en el plazo de Pub/Sub. Todo está en la guía reproducible del repositorio.» |

Duración estimada 15–16 min. Si se pasa de 18, recortar el bloque de reentregas o el de Pub/Sub a la mitad.

## Qué no mostrar

- Contraseñas, tokens, el valor de un secreto, ni el archivo de estado de Terraform.
- La consola de facturación con montos.
- Archivos `.tfplan` (llevan la contraseña de la base).

## Después de grabar

Apagar lo que cobra: la guía §7, paso 5 (`terraform apply` sin `cloudrun_enabled`, base en `NEVER`, parar la VM de Redis).
