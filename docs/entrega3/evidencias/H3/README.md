# Evidencias H3 — Primer documento de deuda técnica y línea base final (#230)

**Bloque:** H — Calidad del código y deuda técnica
**Rúbrica:** Documentación y deuda
**Responsable:** `@DiegoOrtizRuiz`
**Depende de:** C7 (#204), E1 (#211), F2 (#216), H1 (#228), H2 (#229)
**Desbloquea:** I3 (#233), I4 (#234)
**Rama:** `H3-Primer-documento-de-deuda-tecnica`
**Entregable principal (Entregable 6):** [`docs/entrega3/deuda-tecnica.md`](../../deuda-tecnica.md)

---

## 1. Cumplimiento de las tareas y criterios de aceptación de H3 (#230)

| # | Criterio / Tarea del issue #230 | Estado | Sección en `docs/entrega3/deuda-tecnica.md` y evidencia verificable |
| :---: | :--- | :---: | :--- |
| 1 | **Entregable 6 completo en la ruta exigida (`docs/entrega3/deuda-tecnica.md`)** | `CUMPLIDO` | [`docs/entrega3/deuda-tecnica.md`](../../deuda-tecnica.md) (`523` líneas, 7 secciones estructuradas, verificado por [`a_estructura_y_campos_inventario.txt`](./a_estructura_y_campos_inventario.txt)). |
| 2 | **Alcance y método del diagnóstico**: componentes revisados, herramientas y actividades de revisión del equipo | `CUMPLIDO` | [§1 de `deuda-tecnica.md`](../../deuda-tecnica.md#1-alcance-y-método-del-diagnóstico): detalla los 8 componentes auditados (`21 682` líneas productivas en `135` archivos + `14 485` líneas en `63` archivos de pruebas + `21` archivos HCL), las herramientas automáticas (`SonarQube Cloud`, `golangci-lint`, `govet`, `eslint`, `tsc -b`, `terraform validate`, `f2_identidades_y_secretos.sh`), las 5 actividades de revisión manual y experimental del equipo, y la tabla de equivalencia de severidad e impacto. |
| 3 | **Resumen de la medición inicial**: duplicación, mantenibilidad y deuda estimada, con unidades, interpretación y enlace al reporte del commit final | `CUMPLIDO` | [§2 de `deuda-tecnica.md`](../../deuda-tecnica.md#2-resumen-de-la-medición-inicial-y-línea-base-verificable): documenta duplicación (`0.8 %` global / `0.0 %` en código de aplicación vs. máximo `20.0 %`), mantenibilidad (`A` / `1.0` vs. umbral `C` / `3.0`, `sqale_debt_ratio = 0.0 %`), y deuda estimada con su unidad en minutos y jornadas SonarQube (`sqale_index = 664 min` global inicial / `60 min → 0 min` en `CODE_SMELL` del código de aplicación; esfuerzo total de remediación `785 min → 690 min`), con interpretación técnica y enlaces al reporte del commit final y al pipeline CI. |
| 4 | **Correcciones hechas en este ejercicio, distinguidas de los pendientes que forman la línea base final** | `CUMPLIDO` | [§3 de `deuda-tecnica.md`](../../deuda-tecnica.md#3-correcciones-realizadas-durante-este-ejercicio): distingue explícitamente las **8 correcciones cerradas en el sprint** (`CORR-H1-01` en `90647b0` y `CORR-H2-01`..`CORR-H2-07` en `868e43d`, `bde3686`, `28967df`, `d469eb6`, `48a304a`) frente a los **12 elementos pendientes/en tratamiento** que conforman la línea base final en §4. |
| 5 | **Inventario de deuda de código, arquitectura, infraestructura, pruebas y operación, sustentado en evidencia concreta** | `CUMPLIDO` | [§4 de `deuda-tecnica.md`](../../deuda-tecnica.md#4-inventario-de-deuda-técnica-identificada): registra **12 ítems** distribuidos en las 5 categorías exigidas: **Código** (`TD-COD-01`..`TD-COD-04`), **Arquitectura** (`TD-ARQ-01`..`TD-ARQ-02`), **Infraestructura** (`TD-INF-01`..`TD-INF-03`), **Pruebas** (`TD-PRU-01`) y **Operación** (`TD-OPE-01`..`TD-OPE-02`). |
| 6 | **Cada elemento del inventario tiene los ocho campos obligatorios del enunciado** | `CUMPLIDO` | [§4.2 de `deuda-tecnica.md`](../../deuda-tecnica.md#42-fichas-detalladas-del-inventario-con-los-8-campos-obligatorios-por-elemento): los 12 ítems incluyen `1. Identificador y categoría`, `2. Descripción y ubicación`, `3. Evidencia`, `4. Impacto`, `5. Prioridad y justificación`, `6. Tratamiento propuesto`, `7. Responsable y plazo`, `8. Estado y decisión`. Verificado automáticamente en [`a_estructura_y_campos_inventario.txt`](./a_estructura_y_campos_inventario.txt) (`12/12` ítems con `8/8` campos). |
| 7 | **Registrar todo hallazgo Major o superior que quede abierto con su regla, ubicación, descripción, impacto y motivo de aplazamiento** | `CUMPLIDO` | [§5 de `deuda-tecnica.md`](../../deuda-tecnica.md#5-registro-completo-de-hallazgos-major-o-superiores-abiertos): certifica `0` `BLOCKER` y `0` `CRITICAL` abiertos en el código de aplicación y enumera individualmente las **33 ocurrencias `MAJOR` abiertas** de la regla `go:S2077` en `internal/postgres/*.go` con su clave de SonarQube Cloud (`AaGYz7mFYJKVmQ5dynOB` a `AaGYz7mwYJKVmQ5dynOf`), archivo, línea exacta, constante concatenada, impacto y motivo de aplazamiento. Verificado en [`b_trazabilidad_sonar_y_commits.txt`](./b_trazabilidad_sonar_y_commits.txt). |
| 8 | **Incluir la deuda que el analizador no detecta (decisiones de arquitectura, configuración, pruebas y operación)** | `CUMPLIDO` | `9` de los `12` elementos del inventario (`TD-COD-03`, `TD-COD-04`, `TD-ARQ-01`, `TD-ARQ-02`, `TD-INF-01`, `TD-INF-02`, `TD-PRU-01`, `TD-OPE-01`, `TD-OPE-02`) documentan compromisos de arquitectura, infraestructura, contrato HTTP, pruebas y operación hallados en las campañas experimentales (A7, B1, C4, C7, D2–D5, E1–E3, F1–F2, G3–G6) que ningún analizador estático detecta. |
| 9 | **Priorización y plan de tratamiento con orden de atención, acciones, responsables y entrega objetivo** | `CUMPLIDO` | [§6 de `deuda-tecnica.md`](../../deuda-tecnica.md#6-priorización-y-plan-de-tratamiento): incluye diagrama de flujo por horizontes y tabla priorizada del **Orden #1 al #12** con acciones técnicas verificables, responsables nominales, entrega objetivo (`I6` y Sprints 1–3 de la `Entrega 4`) y criterio de cierre. |
| 10 | **Ningún requisito obligatorio de la entrega aparece registrado como deuda en lugar de cumplirse** | `CUMPLIDO` | Todos los requisitos obligatorios de la Entrega 3 (balanceador HTTPS, grupo elástico de API 1–3, cola dedicada fuera de grupos elásticos, grupo elástico de workers 1–3 por métrica de cola, drenaje/recuperación segura, frontend funcional y campañas 1 vs. 1–3) están cumplidos y cerrados; el inventario sólo registra evoluciones hacia alta disponibilidad multizonal, pooling avanzado y mejoras de observabilidad/mantenibilidad. |
| 11 | **Reporte completo o exportación verificable del commit final y enlace a la ejecución del pipeline** | `CUMPLIDO` | [§7 de `deuda-tecnica.md`](../../deuda-tecnica.md#7-reporte-verificable-del-commit-final-y-enlaces-al-pipeline) y archivo [`reporte_sonarcloud_commit_final.json`](./reporte_sonarcloud_commit_final.json) junto con [`../H1/reporte_sonarcloud.json`](../H1/reporte_sonarcloud.json) y [`../H2/reporte_h2_estado_hallazgos.json`](../H2/reporte_h2_estado_hallazgos.json). |
| 12 | **Anotación en `docs/entrega3/NOTAS_TECNICAS.md`** | `CUMPLIDO` | Registrada en la **Nota Técnica #57** de [`docs/entrega3/NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md#57) con impacto para `I1, I2, I3, I4, I5, I6`. |

---

## 2. Artefactos y evidencias en `docs/entrega3/evidencias/H3/`

| Archivo | Propósito |
| :--- | :--- |
| [`../../deuda-tecnica.md`](../../deuda-tecnica.md) | **Entregable 6**: Primer documento de deuda técnica y línea base final de calidad de la Entrega 3. |
| [`reporte_sonarcloud_commit_final.json`](./reporte_sonarcloud_commit_final.json) | Exportación JSON verificable consolidada del commit final con las métricas de SonarQube Cloud, las 8 correcciones cerradas en el sprint, el resumen de los 12 ítems del inventario y las 33 ocurrencias `MAJOR` auditadas de `go:S2077`. |
| [`a_estructura_y_campos_inventario.txt`](./a_estructura_y_campos_inventario.txt) | Verificación automatizada de las 7 secciones obligatorias, presencia de las 5 dimensiones, los 8 campos obligatorios en los 12 ítems y las 35 anclas Markdown internas (Nota Técnica #14). |
| [`b_trazabilidad_sonar_y_commits.txt`](./b_trazabilidad_sonar_y_commits.txt) | Verificación cruzada contra `H1/reporte_sonarcloud.json`, `H2/reporte_h2_estado_hallazgos.json` y el historial Git (`33/33` claves `go:S2077`, `6/6` commits de corrección, cifras exactas de medición inicial). |
| [`c_seguridad_y_evidencias.txt`](./c_seguridad_y_evidencias.txt) | Auditoría de seguridad: `0` credenciales, llaves, tokens o direcciones IP privadas en `docs/entrega3/deuda-tecnica.md` y en `docs/entrega3/evidencias/H3/`. |
| [`resumen.txt`](./resumen.txt) | Resumen consolidado de controles (`18 OK · 0 FALLA · 0 OBS`). |
| [`capturas/2026-11-01_h3_estructura_e_inventario_8_campos.png`](./capturas/2026-11-01_h3_estructura_e_inventario_8_campos.png) | Captura PNG de la verificación de estructura y de los 8 campos obligatorios en los 12 elementos del inventario. |
| [`capturas/2026-11-01_h3_trazabilidad_sonar_y_linea_base.png`](./capturas/2026-11-01_h3_trazabilidad_sonar_y_linea_base.png) | Captura PNG de la verificación de trazabilidad contra SonarQube Cloud (`33/33` claves `MAJOR`), commits de corrección y generación de `reporte_sonarcloud_commit_final.json`. |
| [`capturas/2026-11-01_h3_resumen_verificacion.png`](./capturas/2026-11-01_h3_resumen_verificacion.png) | Captura PNG del resumen consolidado de verificación de H3 (`18 OK · 0 FALLA · 0 OBS`). |

---

## 3. Cómo reproducir la verificación

```bash
bash ./scripts/h3_verificar_deuda_tecnica.sh
```

Este directorio y sus capturas no contienen credenciales, llaves, tokens ni direcciones IP privadas.

