# Evidencia G1 — Instrumentación de las corridas, generador externo y criterios de parada ([#222](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/222))

| Criterio de aceptación | Estado | Dónde |
| :--- | :---: | :--- |
| Tablero o exportaciones que cubran todas las métricas exigidas por el enunciado | ✅ | §1 · [`ejemplo_ventana_del_despliegue/`](./ejemplo_ventana_del_despliegue/) · [gráfica del tablero](./capturas/2026-10-31_tablero-ventana-del-despliegue.png) |
| Generador externo verificado como no limitante | ✅ calibración desde la VM: **NO LIMITA** | §5 · [`calibracion_desde_la_vm/`](./calibracion_desde_la_vm/) |
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

## 5. Calibración real del generador (2026-11-01)

Plan de solo lectura (`capacity-planning/calibracion_generador.jmx`: el catálogo y la sonda de disponibilidad, sin cuentas ni escrituras),
**20 usuarios, 3 minutos**, entrando por el balanceador `https://136.68.55.254.sslip.io`. Dos corridas del mismo plan:

| | Desde la VM del generador | Desde un portátil (prueba de humo) |
| :--- | :--- | :--- |
| Carpeta | [`calibracion_desde_la_vm/`](./calibracion_desde_la_vm/) | [`calibracion_humo_desde_portatil/`](./calibracion_humo_desde_portatil/) |
| Usuarios · duración | 20 · 3 min | 5 · 1 min |
| Latencia p50 / p95 (generador) | **7 / 12 ms** | 87 / 183 ms |
| Latencia p50 / p95 (balanceador) | 5,9 / 10,4 ms | 6,2 / 9,7 ms |
| Brecha p50 | **1,1 ms** | 81 ms (la distancia Colombia → Virginia) |
| CPU del generador (p95) | 34 % de **un** núcleo de 4 (límite 280 %) | 6 % |
| Memoria del generador | 4,1 % | 5,6 % |
| Solicitudes: generador → balanceador | 2189 → 2223 (**102 %**) | 170 → 136 (**80 %**) |
| Errores | 0 | 0 |
| **Veredicto** | **NO LIMITA** | **LIMITADO** |

[Gráfica del tablero de la corrida desde la VM](./capturas/2026-11-01_tablero-calibracion-20-usuarios-desde-la-vm.png).

**Qué dice esto:**
- El generador de la VM no limita: usa un tercio de un núcleo de cuatro, su latencia coincide con la del balanceador salvo 1 ms, y el
  balanceador cuenta todo lo que envió (de más, porque cuenta también el tráfico de otros).
- **La causa del hallazgo anterior parece estar en el portátil.** La misma comprobación desde el portátil pierde el 20 % de las
  solicitudes (170 enviadas, 136 contadas) con todas las respuestas `200`: algo entre el portátil y el balanceador atiende parte de las
  lecturas (la hipótesis de un proxy o antivirus que inspecciona el HTTPS). No se ha demostrado qué es, pero **desde la VM no ocurre**, y es
  un argumento de peso para que G3 y G4 salgan de la VM. La línea base de A7 salió de un portátil: hay que declararlo en G7.

**Dos errores de medición que esta corrida destapó y están corregidos** (y con pruebas que los impiden):
1. El conteo del balanceador se leía **sin margen** en los bordes de la ventana: Cloud Monitoring entrega un punto por minuto y descarta los
   que cruzan el borde, así que se perdía el primer y el último minuto (1950 de 2223). Ahora se lee con 2 minutos de margen antes y 4 después.
2. El comprobante comparaba el conteo del balanceador con las solicitudes **de la tabla** (tras omitir el calentamiento) y no con las
   **enviadas**: el balanceador cuenta el calentamiento también. Ahora compara contra todas las del `.jtl`.

El balanceador cuenta el tráfico de **todos**, no solo el del generador: la comparación es una **cota inferior**. En el entorno compartido, quien
vaya a medir debe avisar al equipo para no mezclar el tráfico.

## Pendiente

Nada de este issue. (Para G3 y G4: `terraform apply -var generator_enabled=true` con los `-var` del entorno encendido; el generador se
apagó al terminar la calibración. Ver nota 49.)

## Nunca

Credenciales, llaves ni secretos.
