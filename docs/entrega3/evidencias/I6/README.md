# Evidencia I6 — Cierre: eliminación controlada de recursos y reconstrucción ([#236](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/236))

| | |
| :--- | :--- |
| **Bloque** | I — Documentación y cierre |
| **Rúbrica** | Documentación y deuda |
| **Procedimiento** | [`RUNBOOK.md`](./RUNBOOK.md) — escrito antes de la sesión, ejecutado por alguien distinto |
| **Desbloquea** | I5 (#235) — **en ese orden**, ver abajo |

**Estado: pendiente de ejecutar.** El runbook está escrito y verificado contra
`infra/terraform/` y los guiones del repositorio; falta la sesión.

> **Hay un paso previo que no se puede saltar.** El pre-vuelo del 2026-11-02
> encontró que **no hay imágenes publicadas para `HEAD`**: el entorno vivo corre
> `97120ca1`, ~70 commits atrás, y entre medias cambió código —`internal/auth` y
> 10 ficheros del frontend, las correcciones de H2—. Antes de la sesión hay que
> correr `bash ./scripts/publish_images.sh` desde `main` limpio, o la recreación
> del §6 arrancará instancias que no pueden descargar su imagen. Es el §1.1 del
> runbook.

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
| Evidencias y respaldos conservados antes de eliminar | §2 del runbook · captura 2 | ⏳ |
| Recursos eliminados y **coste residual medido** | §4 y §5 · capturas 3, 4 y 5 | ⏳ |
| Procedimiento de reconstrucción documentado, paso a paso y verificado | [`RUNBOOK.md`](./RUNBOOK.md) | ✅ escrito |
| Reconstrucción ensayada al menos una vez, con grupos elásticos, balanceador, cola y frontend | §6 · captura 6 | ⏳ |
| **Ensayado por alguien que no lo escribió** | quién lo ejecutó, abajo | ⏳ |
| La verificación funcional pasa sobre el entorno recreado, con su salida registrada | §7 · `verificacion_entorno_recreado.txt` · captura 7 | ⏳ |
| Las pruebas y la sustentación usan **PostgreSQL administrado** | Cloud SQL `mooc-db-1`, recreada en §6 | ⏳ |
| `deletion_protection` en su valor seguro antes de cerrar | §8 | ⏳ |
| **Bloquea el cierre:** la destrucción y la recreación capturadas mientras ocurren | capturas 4 y 7 | ⏳ |
| Anotado en `NOTAS_TECNICAS.md` | al cerrar; si no hubo nada, decirlo | ⏳ |

---

## Quién ejecutó la sesión

> Rellenar al terminar. Es criterio de aceptación que **no sea quien escribió el
> runbook**.

| | |
| :--- | :--- |
| Runbook escrito por | |
| Sesión ejecutada por | |
| Fecha | |
| Pasos que hubo que corregir sobre la marcha | |

Esa última fila es la que da valor al criterio: un procedimiento que se ejecuta
sin tropiezos por alguien que no lo escribió es un procedimiento sin pasos
implícitos. Si hubo tropiezos, **el arreglo va al runbook**, no solo aquí.

---

## Decisión sobre la dirección global y el certificado

> Rellenar antes de la sesión.

`136.68.55.254.sslip.io` está citada en el `README.md`, en `arquitectura.md`, en
`VERIFICACION_ENDPOINTS.md`, en el entorno de Postman **y en el vídeo de
sustentación, que ya está grabado**. Un `destroy` completo se lleva `mooc-lb-ip`
y `mooc-lb-cert`, y al recrear salen otros: habría que corregir esos documentos,
y el vídeo ya no se puede corregir.

| Opción | Consecuencia |
| :--- | :--- |
| **Conservar IP y certificado** (recomendada en el runbook §0) | ~0 USD aparcados, la URL del vídeo sigue siendo válida, y la sustentación síncrona no espera la hora de aprovisionamiento del certificado |
| Destroy total | Cumple «eliminar» en su lectura más estricta, pero cambia la URL de toda la entrega |

**Decidido:** _(rellenar)_

---

## Coste residual

> Rellenar con las medidas del §5 del runbook.

| Concepto | Medido | USD/mes |
| :--- | ---: | ---: |
| Buckets de medios y HLS | | |
| Artifact Registry | | |
| Secret Manager | | |
| Dirección global reservada | | |
| **Total aparcado** | | |
| **Entorno completo, de referencia** | — | **168,69** (F3 §2) |

---

## Capturas

> `capturas/AAAA-MM-DD_descripcion.png`. Las siete están listadas en el runbook,
> cada una en el paso donde se toma. **Sin credenciales, tokens ni IP privadas.**

| # | Archivo | Qué prueba |
| ---: | :--- | :--- |
| 1 | `_estado-inicial.png` | el entorno completo antes de tocarlo |
| 2 | `_respaldo-conservado.png` | el respaldo existe antes de destruir |
| 3 | `_destruccion-en-curso.png` | la salida del `destroy` |
| 4 | `_entorno-destruido.png` | **bloqueante** — no queda nada vivo |
| 5 | `_coste-residual.png` | lo que sigue costando y cuánto |
| 6 | `_recreacion-completa.png` | los dos grupos arriba otra vez |
| 7 | `_verificacion-sobre-entorno-recreado.png` | **bloqueante** — la verificación pasa sobre lo recreado |

---

Este archivo no contiene credenciales, llaves ni secretos.
