# Evidencia I6 — Cierre: eliminación controlada de recursos y reconstrucción ([#236](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/236))

| | |
| :--- | :--- |
| **Bloque** | I — Documentación y cierre |
| **Rúbrica** | Documentación y deuda |
| **Procedimiento** | [`RUNBOOK.md`](./RUNBOOK.md) — escrito antes de la sesión, ejecutado por alguien distinto |
| **Desbloquea** | I5 (#235) — **en ese orden**, ver abajo |

**Estado: ejecutado el 2026-11-02.** Entorno destruido y reconstruido de punta a
punta, con la verificación funcional pasando sobre lo recreado: **20
comprobaciones, 0 fallos**. La sesión dejó **nueve correcciones**, dos de ellas
en código de infraestructura.


---

## Por qué I6 va antes que I5

El cuerpo de #236 dice «Depende de: I5» y es al revés. Tres razones, por orden
de peso:

1. **I5 no puede cumplir su propio criterio hasta que I6 capture.** I5 exige que
   `scripts/check_evidencias.sh` salga con código 0 sobre el commit que se
   taguea. Hoy sale con 1, y el único que falta es I6:

   ```
   18 de 19 issues OK · FALTA I6: la destruccion y la recreacion completa
   ```

2. **El enunciado (p. 13) pide que el tag registre «el commit evaluado».** I6
   produce commits —capturas, este README, la salida de la verificación—. Si el
   tag va antes, deja de apuntar al commit final.

3. **El enunciado (p. 7) solo ordena una cosa**: «después de conservar las
   evidencias y respaldos necesarios, eliminar los recursos de laboratorio que
   no deban seguir activos». Del tag no dice nada ahí, así que no hay ninguna
   exigencia que empuje en sentido contrario.

---

## Criterios de aceptación

| Criterio | Dónde se demuestra | Estado |
| :--- | :--- | :---: |
| Evidencias y respaldos conservados antes de eliminar | respaldo de **8,50 MiB** en el bucket y copia del estado de **224 KB**, los dos verificados no vacíos · captura `respaldo-conservado` | ✅ |
| Recursos eliminados y **coste residual medido** | §«Coste residual», abajo | ✅ |
| Procedimiento de reconstrucción documentado, paso a paso y verificado | [`RUNBOOK.md`](./RUNBOOK.md), corregido con lo que encontró la ejecución | ✅ |
| Reconstrucción ensayada, con grupos elásticos, balanceador, cola y frontend | los 14 recursos recreados; backend `HEALTHY` · captura `reconstruccion` | ✅ |
| **Ensayado por alguien que no lo escribió** | sí, ver abajo | ✅ |
| La verificación funcional pasa sobre el entorno recreado, con su salida registrada | [`verificacion_entorno_recreado.txt`](./verificacion_entorno_recreado.txt): **20 correctas, 0 fallos** · captura `verificacion-sobre-entorno-recreado` | ✅ |
| Las pruebas y la sustentación usan **PostgreSQL administrado** | Cloud SQL `mooc-db-1`, recreada y sembrada en esta sesión | ✅ |
| `deletion_protection` en su valor seguro antes de cerrar | `true` **en el estado**, no solo en el fichero (nota 67) | ✅ |
| **Bloquea el cierre:** la destrucción y la recreación capturadas mientras ocurren | capturas `destroy applied` (las dos listas vacías) y `verificacion-sobre-entorno-recreado` | ✅ |
| Anotado en `NOTAS_TECNICAS.md` | notas **67** y **68** | ✅ |

---

## Tiempos de la reconstrucción

| Fase | Tiempo |
| :--- | ---: |
| `terraform destroy` de los 14 recursos | ~8 min |
| `terraform apply` de recreación | ~12 min |
| VM temporal + copia + migrar y sembrar (`recrear-entorno.sh --solo-base`) | **120 s** |
| Migraciones | **0 s** — ya aplicadas por el arranque de las instancias (A3) |
| Verificación funcional | ~1 min |

Que las migraciones no costaran nada es el resultado que vale la pena señalar:
las instancias nuevas aplicaron el esquema solas al arrancar, con el *advisory
lock* de A3, antes de que nadie se conectara. El guion de siembra encontró las
**8 migraciones ya aplicadas** y solo tuvo que cargar los datos.

---

## Quién ejecutó la sesión

| | |
| :--- | :--- |
| Runbook escrito por | asistente, a partir de `infra/terraform/`, `ADMINISTRACION.md` y el runbook de I6 de la Entrega 2 |
| Sesión ejecutada por | `@fredyxander` |
| Fecha | 2026-11-02, de 19:00 a 20:40 UTC |
| Pasos que hubo que corregir sobre la marcha | **nueve**, listados abajo |

El criterio pide que ensaye el procedimiento alguien que no lo escribió, y esta
es la razón: el runbook estaba escrito contra el código y era coherente, pero
**nadie lo había ejecutado nunca**. Nueve cosas solo aparecen al hacerlo de
verdad, y dos de ellas eran defectos reales del repositorio, no del documento.

### Las nueve correcciones

| # | Qué falló | Dónde está el arreglo |
| ---: | :--- | :--- |
| 1 | El `gcloud sql export` falla: la cuenta de Cloud SQL no tenía ningún rol sobre el bucket | §2a del runbook: `objectCreator`, temporal |
| 2 | `add-iam-policy-binding` pregunta por una condición, porque el bucket tiene *bindings* condicionales de F2 | §2a: responder `2` (None) |
| 3 | El `plan` del §3 sin los cinco interruptores propone **aparcar** el entorno: `6 to destroy` en vez de `1 to change` | §3, con las cinco `-var` |
| 4 | `deletion_protection` seguía en `true` **en el estado** tras editar el fichero | §3, con la comprobación añadida · **nota 67** |
| 5 | La lista de `-target` del §4 era incorrecta: GCP rechaza borrar un grupo que un backend service usa | §4: el `destroy` cascadea hacia arriba; se planifica y se lee antes |
| 6 | `DROP USER moocuser` falla: es dueño de 20 objetos y `moocdb` tiene `ABANDON` | `database.tf`: `deletion_policy = "ABANDON"` en el usuario · **nota 67** |
| 7 | `recrear-entorno.sh` sin `--solo-base` corre `terraform apply -auto-approve` **sin `-var`**: desmonta lo que el paso anterior acaba de crear | §6 del runbook, con la bandera |
| 8 | La siembra falla con `password authentication failed`: el secreto tiene un `\r` que Terraform recorta y el guion no | `recrear-entorno.sh`, las dos lecturas · **nota 68** |
| 9 | La guardia del endpoint fantasma pedía `/users/professors` sin prefijo: nginx responde 405, no 404 | `verificar_endpoints_nube.sh`: la ruta va bajo `/api/v1/` |

Las correcciones **6, 8 y 9** son defectos del repositorio que llevaban ahí
tiempo y que **solo se manifiestan reconstruyendo desde cero**. La 9 además
llevaba rota desde que D1 metió el frontend en la imagen del proxy: hasta
entonces la comprobación pasaba por accidente.

---

## Decisión sobre la dirección global y el certificado

`136.68.55.254.sslip.io` está citada en el `README.md`, en `arquitectura.md`, en
`VERIFICACION_ENDPOINTS.md`, en el entorno de Postman **y en el vídeo de
sustentación, que ya está grabado**. Un `destroy` completo se lleva `mooc-lb-ip`
y `mooc-lb-cert`, y al recrear salen otros: habría que corregir esos documentos,
y el vídeo ya no se puede corregir.

| Opción | Consecuencia |
| :--- | :--- |
| **Conservar IP y certificado** (recomendada en el runbook §0) | ~0 USD aparcados, la URL del vídeo sigue siendo válida, y la sustentación síncrona no espera la hora de aprovisionamiento del certificado |
| Destroy total | Cumple «eliminar» en su lectura más estricta, pero cambia la URL de toda la entrega |

**Decidido el 2026-11-02: se conservan `mooc-lb-ip` y `mooc-lb-cert`.**

El motivo que decide es el vídeo. Es un entregable obligatorio, ya está grabado
y enlazado desde el `README.md`, y cita esa URL: una dirección muerta en él no
se puede corregir después. Los dos recursos cuestan del orden de 0 USD
aparcados, así que «eliminar los recursos de laboratorio **que no deban seguir
activos**» (enunciado, p. 7) no los alcanza: deben seguir activos precisamente
para que la entrega sea verificable.

Como efecto secundario buscado, una sustentación síncrona no tiene que esperar
hasta una hora a que se aprovisione un certificado nuevo (enunciado, p. 13:
«si los recursos se retiraron por control de costos, el equipo deberá
recrearlos previamente»).

El `destroy` del §4 del runbook va con `-target`, no a secas. Lo que sí se
destruye y se vuelve a crear: los dos grupos administrados, la VM de la cola y
la instancia de Cloud SQL — que es lo que #236 pide ensayar.

---

## Coste residual

Medido el 2026-11-02 con el entorno ya destruido, antes de recrearlo.

| Concepto | Medido | USD/mes |
| :--- | ---: | ---: |
| Bucket `media` | 0,73 GiB | ~0,01 |
| Bucket `hls` | 7,73 GiB | ~0,16 |
| Bucket del estado de Terraform | 0,10 MiB | ~0,00 |
| Artifact Registry | 1 287 MB (F3, 1-nov) | ~0,07 |
| Secret Manager | 2 secretos | ~0,12 |
| Dirección global `mooc-lb-ip`, **reservada sin uso** | — | **7,30** |
| **Total aparcado** | | **~7,66** |
| | | |
| **Entorno completo, de referencia** | — | **168,69** (F3 §2) |

Aparcado cuesta el **4,5 %** de lo que cuesta encendido.

Dos cosas que la sesión dejó claras y que no son obvias:

- **La regla de redirección del 80 sobrevivió al `destroy`.** No depende de los
  grupos administrados —cuelga de su propio url map y proxy—, así que nunca
  entró en los 14 recursos. Mientras existió, el coste dominante era ella:
  0,025 USD/h. El `apply` del §6 la reconcilió y el §9 la retiró.
- **La dirección global solo factura cuando NO está en uso.** Con la regla de
  reenvío viva valía 0; al aparcar pasa a `RESERVED` y cuesta 0,01 USD/h, que
  es el 95 % de lo que cuesta el entorno aparcado. Es la corrección a la nota 38
  que ya documentó F3 §4, confirmada aquí.

---

## Capturas

`capturas/`, del 2026-11-02. Sin credenciales, tokens ni IP privadas.

| Archivo | Qué prueba |
| :--- | :--- |
| `_estado-inicial.PNG` | el entorno completo antes de tocarlo |
| `_respaldo-conservado.PNG` | el respaldo existe y no está vacío, antes de destruir |
| `_destroy applied.PNG` | **bloqueante** — `instances list` y `sql instances list`, las dos vacías |
| `_reconstruccion.PNG` | los dos grupos arriba otra vez y el backend sano |
| `_verificacion-sobre-entorno-recreado.PNG` | **bloqueante** — 20 comprobaciones, 0 fallos sobre lo recreado |

No se guardó captura del volcado del `destroy` ni de las medidas de coste
residual: la primera la sustituye con ventaja `destroy applied` —las listas
vacías prueban el resultado, no solo el intento— y las segundas están en la
tabla de arriba, tomadas con el entorno ya destruido.

---

Este archivo no contiene credenciales, llaves ni secretos.
