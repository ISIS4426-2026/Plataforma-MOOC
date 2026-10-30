# Evidencia C2 — Política de autoscaling de la API: señal, objetivo y estabilización ([#205](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/205))

| Criterio de aceptación | |
| :--- | :---: |
| Tabla completa de la política: señal, objetivo, unidad, justificación, inicialización, estabilización y límites | ✅ §1 |
| La política vive en el código de infraestructura, no aplicada desde la consola | ✅ [`api_autoscaler.tf`](../../../../infra/terraform/api_autoscaler.tf) |
| Queda explícito qué pasa con la demanda que excede la capacidad de 3 instancias | ✅ §4 |
| Anotado en `NOTAS_TECNICAS.md` | ✅ nota 37 |

**Estado: escrito y validado.** `terraform validate` pasa y `fmt` no propone
cambios. No necesitó entorno: la política se justifica con las mediciones de A7 y
del escenario 1 de la Entrega 2.

---

## 1. La política, completa

| | |
| :--- | :--- |
| **Señal** | peticiones por segundo y por instancia, vía el balanceador |
| **Parámetro del proveedor** | `autoscaling_policy.load_balancing_utilization.target` |
| **Objetivo** | `0.8` — fracción de la capacidad declarada por instancia |
| **Unidad** | adimensional; el denominador es `max_rate_per_instance` del backend, **100 pet/s** (lo declara C3) |
| **Umbral efectivo** | se añade instancia al pasar de **80 pet/s por instancia** |
| **Justificación** | a 103,5 pet/s una sola instancia daba p95 de 122 ms **sin saturarse**; 80 deja margen para que la nueva arranque antes de llegar a lo medido |
| **Inicialización** | `cooldown_period = 300` s |
| **Justificación** | B3 midió **62 s** de arranque y **160 s** del borrado a `HEALTHY`. Mismo valor que `initial_delay_sec` de C1 |
| **Estabilización de bajada** | `scale_in_control`: `max_scaled_in_replicas.fixed = 1`, `time_window_sec = 600` |
| **Justificación** | una instancia cada diez minutos como máximo; de 3 a 1 tarda veinte. Retirar es barato, equivocarse al retirar cuesta 160 s de arranque |
| **Límites** | `min_replicas = 1`, `max_replicas = 3` |
| **Modo** | `mode = "ON"` — escala en los dos sentidos |

### Qué significa cada nombre del proveedor

El criterio pide el nombre real y su significado, no una paráfrasis.

| Nombre real | Qué es, exactamente |
| :--- | :--- |
| `cooldown_period` | **no** es un tiempo entre decisiones: es cuánto ignora el autoescalador las métricas de una instancia **recién creada**. Es el «periodo de inicialización» del enunciado |
| `load_balancing_utilization.target` | fracción de la capacidad por instancia que el balanceador declara. **No es un número de peticiones**: es un cociente |
| `max_rate_per_instance` | el denominador de ese cociente, y **no está en este fichero**: va en el backend service de C3, con `balancing_mode = "RATE"` |
| `scale_in_control.max_scaled_in_replicas` | cuántas réplicas puede retirar **dentro** de la ventana, no en total |
| `scale_in_control.time_window_sec` | la ventana de esa limitación |
| `mode` | `ON`, `ONLY_SCALE_OUT` o `OFF`. Se declara explícito porque G5 mide el ciclo completo |

## 2. Por qué la señal NO es la CPU, y está medido

Era la elección obvia. Los datos la descartan.

| Corrida | pet/s | CPU web media/máx | CPU SQL media/máx | tps |
| :--- | ---: | ---: | ---: | ---: |
| A-nivel1 | 0,2 | 3,74 / 8,37 % | 12,28 / 12,7 % | 4,7 |
| A-nivel10 | 1,8 | 3,27 / 4,34 % | 16,31 / 25,85 % | 18,2 |
| A-nivel100 | 16,5 | 7,13 / 8,58 % | 16,67 / 19,89 % | 176,2 |
| A-nivel200 | 32,5 | 12,00 / 15,55 % | 24,67 / 30,69 % | 350,9 |
| B-nivel100 | 54,0 | 10,07 / 15,81 % | 18,97 / 25,21 % | — |
| **B-nivel200** | **103,5** | **22,42 / 37,33 %** | 24,87 / 35,20 % | **1 039,7** |

A **500 veces** el caudal, la CPU de la API pasa del 3,7 % al 22,4 %. La pendiente
marginal es de unos **0,18 puntos de CPU por petición/s**, así que un objetivo del
60 % —el valor habitual— se alcanzaría alrededor de **310 peticiones/s por
instancia**.

Y ese caudal no se puede sostener: a 103,5 pet/s, Cloud SQL ya iba a **1 039,7
tps**. Tres instancias a 310 pet/s pedirían del orden de **9 000 tps a una base de
1 vCPU**.

> **Con un objetivo de CPU, el grupo se quedaría en una instancia mientras la base
> se funde.** La CPU de esta API no mide su carga porque la API no computa:
> traduce HTTP a SQL y espera. Lo que se agota primero está en la base, no en el
> proceso.

### Las otras señales, y por qué tampoco

| Señal | Por qué no |
| :--- | :--- |
| CPU | medido arriba: no se mueve lo suficiente |
| Memoria | web mem se quedó en **30,1–30,8 %** en **todas** las corridas, de 0,2 a 103,5 pet/s. Es plana: no es una señal, es una constante |
| p95 de latencia, como métrica personalizada | es la consecuencia, no la causa, y llega tarde: cuando el p95 sube ya hay usuarios esperando. Sirve para **comprobar** la política (G5), no para dispararla |
| Profundidad de cola | es la señal de **C6**, para los workers. La API no encola su propio trabajo |

## 3. Lo que esta política necesita de C3

**Sin C3 esta política no escala.** `load_balancing_utilization` es un cociente y
su denominador —`max_rate_per_instance`— vive en el backend service, que crea C3
con `balancing_mode = "RATE"`.

| Lo que C3 tiene que declarar | Valor | De dónde sale |
| :--- | :--- | :--- |
| `balancing_mode` | `"RATE"` | sin él no hay tasa que medir |
| `max_rate_per_instance` | **100** pet/s | 103,5 medidos con p95 de 122 ms y sin saturación, redondeado a la baja |

Si C3 usara `balancing_mode = "UTILIZATION"`, el autoescalador volvería a medir
CPU por la puerta de atrás y la política de arriba quedaría sin efecto.

## 4. Qué pasa al llegar a 3 instancias

Esto es lo que el criterio pide explícito, y la respuesta no es «se degrada la
API».

Con el umbral de 80 pet/s por instancia, tres instancias cubren **240 pet/s**.
Extrapolando de la medida de A7 —103,5 pet/s produjeron 1 039,7 tps, es decir
unos **10 tps por petición/s**— eso son del orden de **2 400 tps** sobre
`db-custom-1-3840`, que es **1 vCPU**.

A 1 039,7 tps la CPU de Cloud SQL iba al 35,20 % como máximo. Extrapolando
linealmente, 2 400 tps la pondrían cerca del **80 %**.

> **El techo del grupo de la API cae aproximadamente donde cae el techo de la
> base.** Escalar la API no escala PostgreSQL: las tres réplicas comparten una
> instancia de 1 vCPU.

Así que la demanda que excede 3 instancias **no la absorbe nadie**: el
balanceador sigue repartiendo, y lo primero que se agota es la base. El síntoma
sería latencia creciente en todas las réplicas a la vez, con CPU de API baja —que
es exactamente el patrón que delata que el cuello no está donde se está mirando.

**Esto es una extrapolación lineal, no una medición**, y hay que decirlo: la
relación tps/petición y la curva de CPU de la base por encima de 1 040 tps no se
han medido. **G5 tiene que medirlo**, y si la base cede antes de las 3 instancias,
la conclusión de la Entrega 3 no es «el autoescalado no funciona» sino «el
siguiente cuello es la base», que es un hallazgo mejor.

### Lo que habría que cambiar para subir ese techo

En orden de coste, no de preferencia:

1. **`availability_type = "REGIONAL"` o más vCPU en Cloud SQL.** Es lo único que
   mueve el techo de verdad, y casi dobla el coste de la instancia de base.
2. **Réplicas de lectura.** El escenario 1 es mayoritariamente lectura —catálogo,
   progreso—, así que dirigir lecturas a una réplica quitaría presión sin tocar
   la escritura. Exige cambios en la aplicación, no solo en la infraestructura.
3. **Subir `max_replicas` por encima de 3.** **No sirve**: añade réplicas que
   esperan a la misma base. El enunciado fija 3 y la medición dice que el límite
   está detrás, no delante.

## 5. Qué falta

Nada que C2 pueda cerrar por sí mismo. La política está en el código y
justificada con mediciones.

Lo que queda es **comprobarla en marcha**, y es de otros:

| Issue | Qué tiene que demostrar |
| :--- | :--- |
| **C3** | declarar `balancing_mode = "RATE"` y `max_rate_per_instance = 100`, sin lo cual esta política no escala |
| **G5** | el ciclo completo de subida y bajada, y si la base cede antes de las 3 instancias |

`capturas/` está creado y vacío: las gráficas del ciclo de escalado son de G5, no
de C2, y tomarlas aquí sería material duplicado de una corrida que todavía no
existe.

---

Este archivo no contiene credenciales, llaves ni secretos.
