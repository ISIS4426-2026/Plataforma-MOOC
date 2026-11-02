# Evidencia I2 — `docs/entrega3/arquitectura.md`: decisiones de escalamiento, operación y capacidad ([#232](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/232))

| Campo | Detalle |
| :--- | :--- |
| **Bloque** | I — Documentación y cierre |
| **Rúbrica** | Documentación y deuda |
| **Responsable** | `@DiegoOrtizRuiz` |
| **Depende de** | B4 (#204), F1 (#219), F2 (#220), F3 (#221), I1 (#231) |
| **Desbloquea** | I3 (#233), I6 (#236) |
| **Entregable principal** | [`docs/entrega3/arquitectura.md`](../../arquitectura.md) (secciones 1 a 8 completas; I2 redacta y verifica §5, §6 y §7 e integra §1–§4 y §8 de I1) |
| **Script verificador** | [`scripts/i2_verificar_arquitectura.sh`](../../../../scripts/i2_verificar_arquitectura.sh) (`15 OK · 0 FALLA · 0 OBS`) |

---

## 1. Matriz de cumplimiento de tareas y criterios de aceptación

| Tarea / Criterio de aceptación del issue #232 | Dónde se cumple en [`docs/entrega3/arquitectura.md`](../../arquitectura.md) | Evidencia verificable |
| :--- | :--- | :--- |
| **Tarea 1 — Decisiones de escalamiento**: plantillas, arranque automático, políticas de ambos grupos, verificaciones de salud, consumo de la cola y retiro seguro de workers | **§5 (`## 5. Decisiones de escalamiento`)**: §5.1 Plantillas `mooc-api-*` y `mooc-worker-*` (`create_before_destroy`, `PROACTIVE`/`REPLACE`/`SUBSTITUTE`); §5.2 Contrato de metadata hacia `mooc-bootstrap.sh` y migraciones con *advisory lock* `4426202603` (`mooc-migrate.sh`); §5.3 Tabla de políticas de `mooc-api-autoscaler` y `mooc-worker-autoscaler` con justificación de por qué no se usa CPU; §5.4 Sondeos `mooc-api-ready` (`/api/v1/ready`) y `mooc-worker-ready` (`:9090/readyz`) y diferencia entre exclusión del LB (~30 s) y recreación por autohealing (`152 s`); §5.5 Cola en `generic_node`, drenaje `90s < 120s < 180s`, `requeue()` con `retry=0` y las 3 capas de idempotencia + limpieza de `worker_scratch` | [`verificacion_i2.txt`](./verificacion_i2.txt) (controles `[3/10]` y `[4/10]`) y captura [`2026-11-01_verificacion-arquitectura-i2.png`](./capturas/2026-11-01_verificacion-arquitectura-i2.png) |
| **Tarea 2 — Operación y recuperación**: instrucciones de despliegue, verificación, reconstrucción y retiro de recursos, con el manejo de configuración y secretos | **§6 (`## 6. Operación y recuperación`)**: §6.1 Herramientas y separación de las 2 capas de identidad (IAM de GCP sin `roles/cloudsql.client` vs usuario PostgreSQL `moocuser`); §6.2 Taxonomía de 3 niveles de configuración/secretos (`B4`/`F2`) y rotación en Secret Manager; §6.3 Despliegue desde cero (Caso A) y encendido desde aparcado (Caso B); §6.4 Tabla de 8 controles de verificación post-despliegue; §6.5 Aparcado en 5 pasos, retiro definitivo para `I6` (`terraform state rm google_sql_user.app` antes de `terraform destroy`) y reconstrucción; §6.6 Tabla de diagnóstico | [`verificacion_i2.txt`](./verificacion_i2.txt) (control `[5/10]`) y captura [`2026-11-01_reconstruccion-y-controles-i2.png`](./capturas/2026-11-01_reconstruccion-y-controles-i2.png) |
| **Tarea 3 — Capacidad, costo y limitaciones**: configuración efectiva, estimación y consumo observado, puntos únicos de falla restantes y relación con los resultados de las pruebas | **§7 (`## 7. Capacidad, costo y limitaciones`)**: §7.1 Configuración efectiva en `us-east1`; §7.2 Tabla comparativa de costos (`133,73 USD/mes` E2 vs `183,29 USD/mes` a 1 inst/grupo y `343,83 USD/mes` a 3 inst/grupo, `0,78 USD` por sesión de 2 h, `0,33` y `0,74 USD/día` aparcado) y contraste con el consumo observado (`F3`); §7.3 Los 4 puntos únicos de falla restantes explícitamente nombrados; §7.4 Cruce causal con `G3`, `G5`, `G6`, `C4`, `C7`, `E1`, `E2` y con `TD-ARQ-01/02`, `TD-INF-01/02`, `TD-OPE-01/02` ([`deuda-tecnica.md`](../../deuda-tecnica.md)) | [`verificacion_i2.txt`](./verificacion_i2.txt) (controles `[6/10]`, `[7/10]` y `[8/10]`) y captura [`2026-11-01_estimacion-costos-f3.png`](./capturas/2026-11-01_estimacion-costos-f3.png) |
| **Criterio 1 — Un integrante que no montó la infraestructura reconstruye el entorno siguiendo solo el documento** | Verificación cruzada documentada en el §2 de este README siguiendo únicamente los pasos del §6.3 y §6.4 de [`docs/entrega3/arquitectura.md`](../../arquitectura.md) | Tabla §2 de este README y captura [`2026-11-01_reconstruccion-y-controles-i2.png`](./capturas/2026-11-01_reconstruccion-y-controles-i2.png) |
| **Criterio 2 — Las políticas de ambos grupos aparecen con sus parámetros efectivos del proveedor** | Tabla del **§5.3** (`mode = ON`, `min_replicas = 1`, `max_replicas = 3`, `cooldown_period = 300 / 420`, `load_balancing_utilization.target = 0.8` sobre `balancing_mode = RATE` y `max_rate_per_instance = 100`, `metric` `custom.googleapis.com/mooc/queue/outstanding` sobre `generic_node` con `single_instance_assignment = 2`, `scale_in_control` `1` réplica cada `600 / 900 s`) | Contrastado línea por línea contra `infra/terraform/api_autoscaler.tf` y `worker_autoscaler.tf` en `[3/10]` de `scripts/i2_verificar_arquitectura.sh` |
| **Criterio 3 — Los puntos únicos de falla restantes están nombrados, no omitidos** | Tabla del **§7.3**: 1) `mooc-queue-server` (Redis + exportador), 2) Cloud SQL `mooc-db-1` (`ZONAL` sin *standby*), 3) concentración en la zona única `us-east1-b`, y 4) relay SMTP externo Brevo | Control `[7/10]` de [`verificacion_i2.txt`](./verificacion_i2.txt) |
| **Criterio 4 — Entregable 3 completo en la ruta exigida** | [`docs/entrega3/arquitectura.md`](../../arquitectura.md) con sus 8 secciones completas y 0 marcadores pendientes, enlazado desde [`README.md`](../../../../README.md) | Control `[1/10]` y `[2/10]` de [`verificacion_i2.txt`](./verificacion_i2.txt) |
| **Criterio 5 — Anotado en `docs/entrega3/NOTAS_TECNICAS.md` con la columna de a qué issues afecta** | Notas técnicas **[#60](../../NOTAS_TECNICAS.md#60)** (I1+I2), **[#61](../../NOTAS_TECNICAS.md#61)** (F3) y **[#62](../../NOTAS_TECNICAS.md#62)** (I2) añadidas en el índice y cuerpo de [`docs/entrega3/NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md) | Control `[9/10]` de [`verificacion_i2.txt`](./verificacion_i2.txt) |

---

## 2. Verificación cruzada de reproducibilidad por quien no escribió cada bloque

En la Entrega 3, la infraestructura de Terraform se construyó en dos franjas complementarias (Nota Técnica #1):
- `@fredyxander` implementó la red privada/Cloud NAT (`B2`), la VM dedicada de la cola (`B1`), el grupo y autoescalador de la API (`C1`, `C2`) y el balanceador HTTPS global (`C3`, `C4`).
- `@DiegoOrtizRuiz` implementó el bootstrap automatizado (`B3`), el manejo de secretos (`B4`) y el grupo, autoescalador y drenaje de workers (`C5`, `C6`, `C7`).
- `@CrispisCas9` redactó las secciones 1–4 de `arquitectura.md` (`I1`), el modelo de costos (`F3`) y ejecutó las campañas de validación en la nube (`G1`, `G3`) siguiendo el runbook de encendido y aparcado.

Para verificar que las instrucciones del **§6 de `docs/entrega3/arquitectura.md`** son 100 % autosuficientes y reproducibles sin conocimiento implícito:

| Paso del §6 de `arquitectura.md` | Quién montó el recurso original | Quién verificó las instrucciones siguiendo solo el documento | Resultado observado |
| :--- | :--- | :--- | :--- |
| **§6.3 Caso B (Desaparcar)** — Encender `mooc-db-1` (`ALWAYS`) y `mooc-queue-server` antes de `terraform apply -var="api_min_replicas=1" -var="lb_enabled=true"` | `@fredyxander` (`B1`, `C1`–`C3`) / `@DiegoOrtizRuiz` (`C5`–`C6`) | `@DiegoOrtizRuiz` (sobre los recursos de red/LB/API de `@fredyxander`) y `@CrispisCas9` (al levantar el entorno para `G1`/`G3`) | El orden documentado evita que `mooc-bootstrap.sh` falle en `mooc-migrate.sh`; ambos grupos alcanzan `IS_STABLE = True` y `mooc-lb-cert` permanece `ACTIVE` porque `mooc-lb-ip` se conservó durante el aparcado |
| **§6.4 Controles 1 a 5 (API, LB, TLS 1.2 y Frontend)** — `get-health mooc-api-backend`, `mooc-lb-cert`, redirección 80 → 443, `/api/v1/ready` con `X-Instance-ID` y `/version.json` | `@fredyxander` (`C3`, `F1`) y `@caupolican11` (`D1`, `D5`) | `@DiegoOrtizRuiz` | Los 5 comandos devuelven `HEALTHY`, `ACTIVE`, `301 Moved Permanently`, `200 OK {"status":"ready"}` con `X-Instance-ID` y el SHA desplegado en `/version.json` |
| **§6.4 Controles 6 a 8 (Cola, exportador y `/readyz` de workers)** — `redis-cli ping`, `systemctl is-active mooc-queue-exporter`, serie `outstanding` en `generic_node` y `:9090/readyz` | `@DiegoOrtizRuiz` (`C5`–`C7`) | `@CrispisCas9` / `@fredyxander` (durante las sesiones de `G3`, `G5` y `G6`) | `PONG`, `active`, puntos cada 30 s en `custom.googleapis.com/mooc/queue/outstanding` y `200 OK` (`ok`) en `:9090/readyz` |
| **§6.5.A (Aparcado en 5 pasos) y §6.5.B (Retiro `I6`)** — `destroy -target` de autoescaladores, `lb_enabled=false`, `resize --size=0`, parada de `mooc-queue-server` y `mooc-db-1`, y `terraform state rm google_sql_user.app` | `@DiegoOrtizRuiz` (`C6/RUNBOOK.md`) y `@fredyxander` (`Entrega 2 I6`) | `@DiegoOrtizRuiz` y `@CrispisCas9` (`F3`) | El entorno queda en `0,33 USD/día` con la cola detenida; `terraform validate` y la revisión de estado confirman que quitar `google_sql_user.app` del estado evita el bloqueo `dependent objects still exist` al destruir `mooc-db-1` |

---

## 3. Capturas y artefactos de evidencia

| Archivo | Descripción |
| :--- | :--- |
| [`verificacion_i2.txt`](./verificacion_i2.txt) | Salida íntegra de `bash ./scripts/i2_verificar_arquitectura.sh` (`15 OK · 0 FALLA · 0 OBS`) |
| [`capturas/2026-11-01_verificacion-arquitectura-i2.png`](./capturas/2026-11-01_verificacion-arquitectura-i2.png) | Ejecución completa del verificador `scripts/i2_verificar_arquitectura.sh` comprobando estructura, anclas, enlaces, paridad con Terraform, drenaje, operación, costos, SPOF, deuda técnica y barrido de secretos |
| [`capturas/2026-11-01_estimacion-costos-f3.png`](./capturas/2026-11-01_estimacion-costos-f3.png) | Ejecución de `python3 scripts/f3_estimacion_costos.py --autoprueba` y salida de las tablas de costos integradas en el §7.2 de `docs/entrega3/arquitectura.md` |
| [`capturas/2026-11-01_reconstruccion-y-controles-i2.png`](./capturas/2026-11-01_reconstruccion-y-controles-i2.png) | Validación de la configuración de Terraform (`terraform fmt -check`, `terraform validate`), verificación de identidades y secretos (`scripts/f2_identidades_y_secretos.sh`) y paridad de recursos para reconstrucción y cierre (`I6`) |

---

Este archivo no contiene credenciales, llaves ni secretos.
