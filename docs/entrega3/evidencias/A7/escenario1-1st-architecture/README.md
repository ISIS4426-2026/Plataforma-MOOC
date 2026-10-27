# Escenario 1 con el código corregido

Segunda corrida, **sobre la misma infraestructura** que midió el tag. Imagen
`3affa11394ea81c569b6eb111ec3e12b1df1bca8` (`main`), esquema de 8 migraciones
—incluida `000008_quiz_attempt_drafts`— y PITR activado.

**Esta es la línea base de G3.** La Entrega 3 elástica corre este mismo código,
así que su diferencia con esta corrida es puramente arquitectónica: no hay que
descontar nada.

Hubo una segunda corrida, con el código del tag `entrega-2` sobre esta misma
infraestructura, que servía para dos cosas: avalar el método —reproducía las
cifras publicadas de la Entrega 2— y medir el coste de las correcciones de la
Entrega 1. **Sus datos crudos no se conservaron en el repositorio**; sus
resultados están reportados en [`../COMPARACION.md`](../COMPARACION.md) §1 y §2,
pero no se pueden rederivar desde aquí.

**No empieces por aquí.** El resultado está en
[`../COMPARACION.md`](../COMPARACION.md) §2:

| | Tag | Corregido | Coste |
| :--- | ---: | ---: | ---: |
| p50 del servidor, conexión caliente | 76 ms | 84 ms | **+8 ms** |
| p95 | 111 ms | 128 ms | **+17 ms** |
| CPU de Cloud SQL, media | 18,4 % | 24,7 % | **+34 %** al mismo caudal |

## Qué leer, en orden

| | |
| :--- | :--- |
| [`analisis.txt`](./analisis.txt) | Tabla por nivel, utilización e, importante, la §4: latencia del servidor **sin** el coste de conectar, que es la única comparable |
| `A-nivel*/resumen.txt` | Un nivel concreto, en legible |

## Qué falta, y por qué

La **serie B** (pausas de 1–2 s). El proceso generador fue detenido por falta de
memoria en la máquina justo al empezarla. La serie A está completa en sus seis
niveles en los dos lados, que es lo que la comparación necesita: restar exige
niveles equivalentes, no todos los niveles posibles.

## Qué hay en cada directorio de corrida

| | |
| :--- | :--- |
| `destino.json` | Contra qué se midió: URL efectiva, entorno, **commit**, generador y ventana. **Sin esto una corrida no es publicable** (A6) |
| `resumen.json` | El resumen que leen los guiones. `ms` son los percentiles en crudo; `ms_publicables` los que pasan el mínimo de observaciones |
| `resumen.txt` | El mismo, en legible |
| `resultados.jtl` | **Las muestras originales, una fila por petición.** De aquí sale la columna `Connect`, que es lo que separa el coste de abrir conexión de la latencia del servidor. El entorno está destruido: esto no se puede regenerar |
| `metricas_gcp.json` | CPU, memoria, conexiones y transacciones de Cloud Monitoring en la ventana. Las métricas caducan, así que se recogieron el mismo día |
| `generador.csv` | CPU del contenedor de JMeter. Si el generador se satura, la medición no es de la plataforma |
| `verificacion_bd.txt` y `estado.txt` | Idempotencia del envío comprobada en la base: 3 envíos por estudiante, 0 duplicados |
| `ventana.txt` | Inicio y fin UTC. Lo leen `capacity_metricas_gcp.py --todas` y `capacity_analisis_h3.py` |

## Cómo reproducir el análisis

```bash
python scripts/capacity_analisis_h3.py docs/entrega3/evidencias/A7/escenario1-1st-architecture
```

---

Este archivo no contiene credenciales, llaves ni secretos.
