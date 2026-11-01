# Evidencia G1 — Instrumentación de las corridas, generador externo y criterios de parada ([#222](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/222))

| Criterio de aceptación | Estado | Dónde |
| :--- | :---: | :--- |
| Tablero o exportaciones que cubran todas las métricas exigidas por el enunciado | ✅ | §1 · [`ejemplo_ventana_del_despliegue/`](./ejemplo_ventana_del_despliegue/) · [gráfica del tablero](./capturas/2026-10-31_tablero-ventana-del-despliegue.png) |
| Generador externo verificado como no limitante | ⏳ herramienta lista y probada; falta encender la VM y la primera corrida | §2, §3 |
| Toda corrida de la Entrega 3 pasa las comprobaciones de A6 antes de publicarse | ✅ el paso quedó en el posproceso | §4 |
| Una respuesta HTTP exitosa no sustituye la validación del resultado de negocio | ✅ ya lo hacen los planes (3 envíos por estudiante, sin duplicados, en la base) y A6 | §4 |
| Anotado en `NOTAS_TECNICAS.md` | ✅ nota 49 | |

## 1. Métricas: qué se recoge y de dónde

`scripts/capacity_metricas_gcp.py` (antes, solo las dos VM fijas de la Entrega 2) ahora cubre la arquitectura elástica.
Escribe `metricas_gcp.json` (resúmenes) y `serie_temporal.csv` (una fila por minuto, para graficar el ciclo de
escalamiento de G5 y G6).

| Lo que pide el enunciado | Fuente en Cloud Monitoring |
| :--- | :--- |
| CPU y memoria de **ambos grupos** | agente de las VM, agregado por grupo (media y máximo de **una** instancia) |
| Instancias **existentes** y **operativas** por grupo en el tiempo | `compute…/instance_group/size` y las que reportan métricas ese minuto |
| Conexiones y carga de PostgreSQL | `cloudsql…/cpu`, `memory`, `postgresql/num_backends`, `transaction_count` |
| Profundidad, antigüedad y tasa de procesamiento de la cola | `custom…/mooc/queue/{pending,active,outstanding,oldest_pending_age_seconds}` y `worker_jobs_{processed,failed}_total` |
| Latencias p50/p95/p99, throughput, errores | **JMeter** (`capacity_resumen_jtl.py`, por operación) y, como contraste, `loadbalancing…/https/{request_count,total_latencies}` |
| Autoescaladores | `autoscaler…/{current_utilization,recommended_size}` |

**Probado contra el entorno real**, en la ventana del despliegue del 2026-10-31 (22:10–22:45 UTC), en la que el grupo de la API
pasó de 1 a 2 instancias y volvió a 1: [`ejemplo_ventana_del_despliegue/`](./ejemplo_ventana_del_despliegue/) muestra
`instancias_existentes` máx 2, `instancias_operativas` máx 2, la cola vacía, 348 solicitudes por el balanceador y los dos
autoescaladores en el mínimo. La gráfica que sale de ese CSV (`scripts/capacity_graficas.py`, cuatro paneles: instancias por grupo, CPU, cola y balanceador) es
[`capturas/2026-10-31_tablero-ventana-del-despliegue.png`](./capturas/2026-10-31_tablero-ventana-del-despliegue.png); sirve igual para el ciclo
de escalamiento de G5 y G6. Esa ventana **no es una corrida de carga**: demuestra que el recolector lee todo, no que la
plataforma rinda.

## 2. El generador externo

La línea base de A7 se midió desde un portátil, y A7 documentó que su red no reproduce lo publicado. G3 y G4 salen de una
VM en la misma región: [`infra/terraform/generator.tf`](../../../../infra/terraform/generator.tf). **No está aplicada**:
`generator_enabled = false` por omisión (no existe ni su cuenta de servicio), y encenderla es un `apply` con los `-var` del
entorno encendido (nota 38) más `-var generator_enabled=true`, ~0,13 USD/h.

| | |
| :--- | :--- |
| Ubicación | `us-east1-c`: misma región que el entorno, **otra zona** que las VM de aplicación |
| Tamaño | `e2-standard-4` (4 vCPU, 16 GiB) |
| Red | sin IP pública; sale por la NAT de B2, se entra por IAP; no abre puertos |
| Identidad | `sa-load-generator`: solo `logging.logWriter` y `monitoring.metricWriter` |
| Herramienta | JMeter **5.5** por digest `sha256:088ac52b…`, el mismo que resolvía `justb4/jmeter:latest` en la línea base |

`run_escenario1.sh` acepta `JMETER_IMAGE` y deja en `destino.json` la imagen, los núcleos y la memoria del generador.
**Diferencia declarada:** la línea base salió de un portátil y G3/G4 salen de una VM; hay que decirlo en G7.

## 3. «No limita»: tres señales cruzadas

`scripts/capacity_contraste_generador.py` rechaza una corrida por cualquiera de ellas (umbrales fechados en
[`criterios_de_medicion.md`](../../../../capacity-planning/criterios_de_medicion.md)):

1. CPU y memoria del generador (`generador.csv`).
2. La **brecha de latencia** entre lo que ve JMeter y lo que ve el balanceador.
3. Las **solicitudes contadas** por el balanceador frente a las enviadas.

Sin datos, no da la corrida por buena. [`autopruebas.txt`](./autopruebas.txt): 8 + 6 reglas pasan (generador holgado no limita;
CPU al 97 % limita; brecha de 860 ms limita; 60 % de solicitudes contadas limita; sin muestras no se acepta).
Sobre la línea base de A7 el comprobante responde **LIMITADO** por «no registra los núcleos del generador»: esas corridas son
anteriores a este registro y salieron de un portátil; no se reescribe su historia.

## 4. Después de cada campaña

```bash
bash scripts/capacity_postproceso.sh <directorio_de_resultados>
```

Reúne las métricas de cada corrida y comprueba el generador. Una corrida `LIMITADO` **no se publica**. Las reglas de A6
(percentiles monótonos, destino declarado, métricas no derivadas de una constante) ya corren dentro de
`capacity_resumen_jtl.py`.

## 5. Primera prueba real del comprobante, y un hallazgo abierto (2026-11-01)

Una corrida de humo del plan de calibración (`capacity-planning/calibracion_generador.jmx`, 5 usuarios, 60 s, solo lecturas)
**desde un portátil** contra el balanceador, y después el posproceso: [`calibracion_humo_desde_portatil/`](./calibracion_humo_desde_portatil/).

| Señal | Resultado |
| :--- | :--- |
| CPU y memoria del generador | holgadas: p95 6 % de un núcleo, 5,6 % de la memoria |
| Brecha de latencia, p50 | generador 87 ms, balanceador 6 ms: **81 ms** (red Colombia → `us-east1`), dentro del umbral de 150 ms |
| **Solicitudes contadas** | JMeter 170 en total (136 tras el calentamiento); el balanceador contó **136**, 80 % de las 170 → **LIMITADO** |

El comprobante hizo lo que debe: **no dio la corrida por buena**. Pero la causa de las 34 solicitudes que el balanceador no cuenta
**no está explicada**, y no se va a esconder bajando el umbral:

- El conteo del balanceador no cambia al volver a leerlo 9 minutos después, ni con otra alineación (`ALIGN_SUM`/`ALIGN_DELTA`).
- Todas las 170 respuestas fueron `200`; ninguna falló en JMeter.
- Los minutos no coinciden: JMeter 78 y 92 por minuto, el balanceador 51 y 85. Faltan más en el primer minuto (la rampa y la apertura de conexiones).
- El entorno es **compartido**: en el mismo rato el balanceador contó 128 solicitudes desde Colombia cuando este equipo envió 40 (un experimento
  aparte con `curl`), o sea que alguien más estaba generando tráfico. Con tráfico ajeno no se puede cerrar la igualdad, solo el *mínimo*.

Hipótesis, por orden de lo que cuesta descartarla: (a) algo entre el portátil y el balanceador atiende parte de las lecturas (un proxy o
antivirus con inspección de HTTPS); (b) retrasos de ingesta de Monitoring que no vuelven a llenarse; (c) un efecto de la rampa. **La
corrida de calibración desde la VM del generador (misma región, sin intermediarios) las separa**: si ahí cuenta ≥ 95 %, la causa es el
portátil, y es un argumento más para que G3 y G4 salgan de la VM.

## Pendiente

1. **Encender el generador** (`apply` con `generator_enabled=true`) y hacer una **corrida de calibración** a baja carga para
   comprobar en vivo que el generador no limita; guardar su `contraste_generador.json` aquí. Lo aplica quien lleve el entorno.
2. ~~Captura del tablero~~ **Hecho** como gráfica exportada del CSV (arriba). La del tablero de la consola de Google queda opcional.
3. Confirmar con Diego (responsable del issue) que no tenía trabajo adelantado.

## Nunca

Credenciales, llaves ni secretos.
