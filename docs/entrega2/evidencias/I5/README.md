# Evidencia I5 — Video de Sustentación Técnica (Issue #140)

Este directorio contiene el guion de producción, la matriz de minutaje y el índice de evidencias que respaldan el **Video de Sustentación** de la **Entrega 2: Despliegue Básico en la Nube Pública**, en cumplimiento con los requerimientos estipulados en la sección **Sustentación → Video** (página 7) del pliego de especificaciones (`docs/2026-20 Entrega 2 - Despliegue Básico en la Nube.pdf`).

---

## 1. Enlace Oficial al Video de Sustentación

> **Enlace del Video:** `[PENDIENTE: Insertar URL del video (YouTube / Google Drive / Vimeo) tras finalizar la grabación y edición]`  
> **Acceso para el Equipo Docente:** El video se encuentra configurado con permisos de visualización directa para los correos del cuerpo docente del curso ISIS4426.  
> **Coordinación con README Principal:** La inserción de este enlace en el archivo [`README.md`](../../../../README.md) raíz del repositorio es gestionada de manera coordinada a través del **Issue I4** ([#139](../../../../issues/139)).

---

## 2. Ficha Técnica de la Sustentación

| Parámetro | Especificación / Valor de Referencia |
| :--- | :--- |
| **Duración del Video** | **18 minutos** (Cumple con el tope reglamentario de máximo 20 minutos, reservando 2 min de margen). |
| **Formato y Calidad** | Captura de pantalla a 1080p (1920×1080), 30 fps, audio balanceado con locución en voz en off en español. |
| **Plataforma Demostrada** | Google Cloud Platform (GCP) en región `us-east1` (zona única `us-east1-b`). **Cero mocks**: cómputo en VMs Debian 12, Nginx 1.27, API modular en Go 1.24, Worker FFmpeg, Redis 7 Asynq, Cloud SQL PostgreSQL 16 y Cloud Storage. |
| **Equipo de Desarrollo** | `CrispisCas9` (Stevan Peralta), `fredyxander` (Fredy Alexander), `DiegoOrtizRuiz` (Diego Ortiz), `tmichelldiaz` (Tania Michel Díaz). |
| **Presentador del Video** | **`CrispisCas9` (Stevan Peralta)** — Relator único en representación del equipo de desarrollo. |
| **Guion Técnico Oficial** | [`GUION_VIDEO.md`](./GUION_VIDEO.md) — Libreto paso a paso con comandos exactos, locución oral y validaciones. |

---

## 3. Índice de Evidencias que Respaldan Cada Segmento del Video

A continuación se detalla la correspondencia entre los bloques temáticos del video, los criterios evaluados, el presentador y las rutas de los registros completos en el repositorio:

| Minutaje | Segmento Temático | Presentador | Criterio Demostrado | Evidencias y Registros en el Repositorio | Estado |
| :---: | :--- | :--- | :--- | :--- | :---: |
| `00:00 - 00:45` | **0. Introducción y Pre-flight Check** | `CrispisCas9` | Verificación de salud y conectividad HTTPS | • [`docs/entrega2/evidencias/D2/health_publico.txt`](../D2/health_publico.txt)<br>• [`docs/entrega2/evidencias/D3/certificado_valido.txt`](../D3/certificado_valido.txt) | ✅ Nube |
| `00:45 - 04:15` | **1. Arquitectura y Correspondencia GCP** | `CrispisCas9` | Despliegue e integración (50%), Red (10%), Base administrada (10%) | • [`docs/entrega2/ARQUITECTURA.md`](../../ARQUITECTURA.md)<br>• [`docs/entrega2/OPERACION_Y_CAPACIDAD.md`](../../OPERACION_Y_CAPACIDAD.md)<br>• [`docs/entrega2/CONFIGURACION_Y_COSTOS.md`](../../CONFIGURACION_Y_COSTOS.md)<br>• [`docs/entrega2/evidencias/I2/consumo_observado.md`](../I2/consumo_observado.md)<br>• [`docs/entrega2/evidencias/B3/DIAGRAMA_RED.md`](../B3/DIAGRAMA_RED.md)<br>• [`docs/entrega2/evidencias/B3/DOCUMENTACION_RED_Y_ADMINISTRACION.md`](../B3/DOCUMENTACION_RED_Y_ADMINISTRACION.md)<br>• [`infra/terraform/`](../../../../infra/terraform/) (`compute.tf`, `database.tf`, `storage.tf`, `network.tf`)<br>• [`docs/entrega2/evidencias/C1/instancia_configuracion.txt`](../C1/instancia_configuracion.txt)<br>• [`docs/entrega2/evidencias/G4/README.md`](../G4/README.md) | ✅ Nube |
| `04:15 - 08:45` | **2. Recorrido Funcional en la Nube** | `CrispisCas9` | Funcionamiento de la plataforma en la nube (10%) | • [`docs/entrega2/evidencias/G3/resultados.md`](../G3/resultados.md) (61 pasos / 0 fallos)<br>• [`docs/entrega2/evidencias/G3/corrida_e2e_cloud.txt`](../G3/corrida_e2e_cloud.txt)<br>• [`docs/entrega2/evidencias/G2/README.md`](../G2/README.md) (166 peticiones Postman)<br>• [`docs/entrega2/evidencias/F1/README.md`](../F1/README.md) (SMTP Brevo)<br>• [`docs/entrega2/evidencias/A5/README.md`](../A5/README.md) y [`A6/README.md`](../A6/README.md) | ✅ Nube |
| `08:45 - 09:45` | **3.1 Carga Directa y URLs Firmadas** | `CrispisCas9` | Almacenamiento administrado sin cursar bytes por API | • [`docs/entrega2/evidencias/C4/carga_directa_completa_sin_api.txt`](../C4/carga_directa_completa_sin_api.txt)<br>• [`docs/entrega2/evidencias/C3/objeto_no_publico_sin_firma.txt`](../C3/objeto_no_publico_sin_firma.txt)<br>• [`docs/entrega2/evidencias/C4/rechazo_firma_alterada_o_vencida.txt`](../C4/rechazo_firma_alterada_o_vencida.txt) | ✅ Nube |
| `09:45 - 10:30` | **3.2 Menor Privilegio e IAM Diferenciado** | `CrispisCas9` | Aislamiento por componentes con condiciones CEL | • [`docs/entrega2/evidencias/C3/api_rechaza_escritura_derivados.txt`](../C3/api_rechaza_escritura_derivados.txt)<br>• [`docs/entrega2/evidencias/C4/worker_escribe_derivados_api_rechazada.txt`](../C4/worker_escribe_derivados_api_rechazada.txt)<br>• [`infra/terraform/storage.tf`](../../../../infra/terraform/storage.tf) | ✅ Nube |
| `10:30 - 11:30` | **3.3 Procesamiento Asíncrono y Persistencia** | `CrispisCas9` | Cola privada, worker FFmpeg, estado available | • [`docs/entrega2/evidencias/E1/procesamiento_video_e2e.txt`](../E1/procesamiento_video_e2e.txt)<br>• [`docs/entrega2/evidencias/E1/README.md`](../E1/README.md)<br>• [`docs/entrega2/evidencias/C1/README.md`](../C1/README.md) | ✅ Nube |
| `11:30 - 12:15` | **3.4 Idempotencia ante Entrega Duplicada** | `CrispisCas9` | Tolerancia a reentregas sin efectos redundantes | • [`docs/entrega2/evidencias/E1/idempotencia_entrega_duplicada.txt`](../E1/idempotencia_entrega_duplicada.txt)<br>• [`docs/entrega2/evidencias/A6/concurrencia_e_idempotencia.txt`](../A6/concurrencia_e_idempotencia.txt) | ✅ Nube |
| `12:15 - 13:15` | **3.5 Fallo con Reintento, Backoff y DLQ** | `CrispisCas9` | Backoff exponencial (2s/4s/8s), alerta JSON y DLQ | • [`docs/entrega2/evidencias/E1/fallo_reintento_backoff_dlq.txt`](../E1/fallo_reintento_backoff_dlq.txt) | ✅ Nube |
| `13:15 - 14:30` | **4.1 Capacidad: Escenario 1 (Académico)** | `CrispisCas9` | Análisis de capacidad Escenario 1 (10%) | • [`docs/entrega2/evidencias/H3/seccion_escenario1_para_informe.md`](../H3/seccion_escenario1_para_informe.md)<br>• [`docs/entrega2/evidencias/H3/resultados/tabla_escalera.txt`](../H3/resultados/tabla_escalera.txt)<br>• [`capacity-planning/pruebas_de_carga_entrega2.md#escenario-1-actividad-académica-concurrente-10`](../../../../capacity-planning/pruebas_de_carga_entrega2.md#escenario-1-actividad-académica-concurrente-10)<br>• [`docs/entrega2/evidencias/H2/resultados/nube-piloto_20260927_211339/resumen.txt`](../H2/resultados/nube-piloto_20260927_211339/resumen.txt)<br>• [`docs/entrega2/evidencias/I3/resumen_ejecutivo_capacidad.md`](../I3/resumen_ejecutivo_capacidad.md) | ✅ Nube (H3 1–200 u) |
| `14:30 - 15:45` | **4.2 Capacidad: Escenario 2 (Multimedia)** | `CrispisCas9` | Análisis de capacidad Escenario 2 (10%) | • [`capacity-planning/pruebas_de_carga_entrega2.md#escenario-2-carga-procesamiento-y-consumo-multimedia-10`](../../../../capacity-planning/pruebas_de_carga_entrega2.md#escenario-2-carga-procesamiento-y-consumo-multimedia-10)<br>• [`docs/entrega2/evidencias/H5/resumen_ejecutivo_escenario2.md`](../H5/resumen_ejecutivo_escenario2.md)<br>• [`docs/entrega2/evidencias/H5/drenaje_cola_verificacion.txt`](../H5/drenaje_cola_verificacion.txt)<br>• [`docs/entrega2/evidencias/G1/manifest.json`](../G1/manifest.json)<br>• [`docs/entrega2/evidencias/I3/resumen_ejecutivo_capacidad.md`](../I3/resumen_ejecutivo_capacidad.md) | ✅ Nube (H5 Replicabilidad) |
| `15:45 - 17:15` | **4.3 Cuello de Botella y Propuestas** | `CrispisCas9` | Caracterización de cuello de botella y evolución | • [`docs/entrega2/OPERACION_Y_CAPACIDAD.md`](../../OPERACION_Y_CAPACIDAD.md)<br>• [`docs/entrega2/evidencias/I3/resumen_ejecutivo_capacidad.md`](../I3/resumen_ejecutivo_capacidad.md)<br>• [`docs/entrega2/ARQUITECTURA.md`](../../ARQUITECTURA.md)<br>• [`capacity-planning/pruebas_de_carga_entrega2.md#8-identificación-del-cuello-de-botella-primario-sustentado-con-evidencia`](../../../../capacity-planning/pruebas_de_carga_entrega2.md#8-identificación-del-cuello-de-botella-primario-sustentado-con-evidencia)<br>• [`docs/entrega2/evidencias/H5/analisis_cuello_de_botella.md`](../H5/analisis_cuello_de_botella.md) | ✅ Consolidado |
| `17:15 - 18:00` | **5. Conclusiones y Cierre** | `CrispisCas9` | Resumen de entrega y release versionado | • [`docs/entrega2/BASE.md`](../../BASE.md)<br>• Tag de git `entrega-2` | ✅ Nube |

---

## 4. Lineamientos para la Sustentación Síncrona

De acuerdo con el enunciado (p. 7):
> *«El equipo docente podrá solicitar una sustentación síncrona y la repetición de una prueba funcional o de desempeño. Para ese encuentro, la aplicación deberá estar desplegada y operativa con el servicio administrado de bases de datos relacionales. Si los recursos se eliminaron por control de costos, será responsabilidad del equipo recrearlos previamente mediante el procedimiento documentado.»*

En caso de ser convocados a sustentación síncrona:
1. **Reactivación de Infraestructura y Base:** Seguir el runbook de recreación automatizado en [`scripts/recrear-entorno.sh`](../../../../scripts/recrear-entorno.sh) y documentado formalmente en [`docs/entrega2/OPERACION_Y_CAPACIDAD.md` §2.7 y §2.8](../../OPERACION_Y_CAPACIDAD.md#27-respaldo-y-reconstrucción-de-la-base) (ensayo medido en ~6 minutos para aprovisionamiento, migraciones y datos sintéticos).
2. **Re-aplicación de Migraciones y Semilla:** Si solo se suspendió la base de datos o se reiniciaron VMs, seguir el orden de dependencias en `OPERACION_Y_CAPACIDAD.md` §2.6 (primero Worker/Redis y luego Web Server).
3. **Repetición en Vivo:** El equipo tiene listos los comandos de validación rápida en [`scripts/e2e_cloud/`](../../../../scripts/e2e_cloud/) (para flujos funcionales E2E en ~18 s) y [`scripts/run_pilot_escenario2.sh`](../../../../scripts/run_pilot_escenario2.sh) (para transcodificación multimedia con instrumentación desacoplada).
