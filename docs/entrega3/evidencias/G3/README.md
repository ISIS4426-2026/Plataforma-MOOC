# Evidencia G3 — Escenario 1 sobre la arquitectura elástica: actividad académica concurrente ([#223](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/223))

| | |
| :--- | :--- |
| **Bloque** | G — Pruebas de carga comparativas |
| **Responsable** | `@tmichelldiaz` |
| **Depende de** | A7, E1, E2, G1 (todos cerrados) |
| **Desbloquea** | G7 |
| **Plan y razones** | [`capacity-planning/entrega3_g3_g4.md`](../../../../capacity-planning/entrega3_g3_g4.md) · [`RUNBOOK.md`](./RUNBOOK.md) |
| **Fecha de la campaña** | 2026-11-01, 19:37–21:54 UTC, desde la VM `mooc-load-generator` (`us-east1-c`), contra el balanceador, con la imagen `97120ca` |

---

## 0. Resultado en una página

**32 corridas, 153 113 peticiones medidas, 0 timeouts, 0 fallos de validación funcional, 1 fallo real (un 503 de 3 ms en `E-C200-r1`), y el estado en la base cuadra en las 32** (3 envíos por estudiante, ningún intento duplicado). El generador **no limitó** ninguna de las corridas publicadas (`capacity_contraste_generador.py`).

| Criterio de aceptación | |
| :--- | :---: |
| 1. p50, p95, p99, rendimiento, errores y timeouts por corrida | ✅ [`tablas.md`](./tablas.md) §1 |
| 2. Integridad de negocio verificada, no solo códigos HTTP | ✅ base directa tras cada corrida (`verificacion_bd.txt`, `estado.txt`) |
| 3. Análisis causal del desplazamiento del cuello de botella, con datos | ⚠️ **no hubo desplazamiento que explicar**: ninguna dependencia llegó a saturarse (§3) |
| 4. Nada atribuido a la arquitectura sin descontar el coste de A7 | ✅ no hay nada que descontar: los dos lados llevan el mismo código corregido (§1.3) |
| 5. Variación entre corridas | ✅ [`tablas.md`](./tablas.md) §2, media ± desviación estándar [mín–máx] |
| 6. Gráficas y salidas originales de cada corrida | ✅ `resultados/<corrida>/` (`.jtl` original, resúmenes, métricas, series) y [la gráfica de la campaña](./capturas/2026-11-01_campana-completa-instancias-cpu-balanceador.png) |
| 7. Hallazgos en `NOTAS_TECNICAS.md` | ✅ notas 57 y 58 |

**La respuesta corta a la pregunta del issue:** con la mezcla del escenario 1 y 200 cuentas de carga, **la arquitectura elástica y la de una sola instancia no se distinguen** hasta unas 110 peticiones por segundo; a unas 255 por segundo la elástica **no mejora** el rendimiento y su cola de latencia es peor mientras suma instancias. **El límite de la plataforma no se alcanzó**: la API no pasó de 51 % de CPU en un instante y PostgreSQL no pasó de 52 %. Lo que sí queda medido es que **escala sola** (1 → 2 → 3 instancias, §3.3) y que **con 3 instancias la base trabaja más para la misma carga**.

---

## 1. Qué se midió, y cómo

### 1.1 Los dos operandos, desde el mismo generador

La línea base de A7 salió de un portátil. Compararla directo con la nube mezcla arquitectura, generador y código, así que se añadió un operando de control medido **desde la misma VM, por el mismo balanceador y con el mismo código**:

| Operando | Qué es | Cómo se fijó |
| :--- | :--- | :--- |
| **R** | la arquitectura de hoy con la API **fija en 1 instancia** | `update-autoscaling --min-num-replicas=1 --max-num-replicas=1`, restaurado al terminar |
| **E** | la elástica tal como está desplegada, API de 1 a 3 | `--min-num-replicas=1 --max-num-replicas=3` (el valor de Terraform) |

**R frente a E es el efecto de la elasticidad.** A7 frente a R mezcla generador, camino de red (el NAT) y código, y solo se compara en las magnitudes que A7 §1.4 dice que sí se pueden comparar.

### 1.2 Lo que se mantuvo constante

El recorrido de 14 pasos y su mezcla 64 % / 36 %, 3 sesiones por usuario con cuentas independientes, la pausa de 4–6 s (serie A) y de 1–2 s (serie B), las rampas de A7 (60 s en la A de 100 y 200 usuarios, 15 s en la B), JMeter 5.5 por digest (`justb4/jmeter@sha256:088ac52b…`), `us-east1`, Cloud SQL `db-custom-1-3840`. Cada corrida deja su `destino.json` con la URL efectiva (`https://136.68.55.254.sslip.io`), el commit, el generador y la ventana UTC.

### 1.3 Diferencias declaradas

| Diferencia | A7 | R y E |
| :--- | :--- | :--- |
| Generador | portátil, red doméstica | VM `e2-standard-4` en la región |
| Camino de red | directo a la VM | NAT de la VPC (2048 puertos por VM, nota 57) → balanceador |
| Cómputo | 1 VM fija | R: 1 instancia · E: 1 a 3 |
| Código | `3affa11` | `97120ca`: solo la corrección de E2 toca el recorrido (un candado más por inscripción) |

**Sobre el punto 7 del issue (descontar el coste que A7 midió):** la línea base de A7 que se usa **ya lleva** el snapshot de quiz y PITR, y R y E corren el mismo código, así que el coste está en los dos lados y se cancela. No se resta nada; se informa la cifra de A7 como contexto (+8 ms de p50 y +34 % de CPU de la base, `A7/COMPARACION.md` §2).

### 1.4 La matriz que se corrió, y lo que se recortó

Por pedido del equipo (hacerlo lo más rápido posible) se recortó la matriz del plan a lo esencial. **Se corrió:**

| Serie | Qué | E | R |
| :--- | :--- | :---: | :---: |
| A | pausa 4–6 s, 1·10·25·50·100·200 usuarios | escalera completa, y 100 × 3 y 200 × 3 | 200 × 3 |
| B | pausa 1–2 s | 100 × 1, 200 × 3 | 200 × 3 |
| C | pausa 0,25–0,75 s, 200 usuarios (más allá de A7) | 2 | 1 publicable (+1 descartada) |
| S | 8 corridas B-200 seguidas | 8 | — |

**No se corrió** (plan §2.3): A100 y B100 en R, B100 repetido y la serie S de 12 corridas (se hicieron 8). Consecuencia: **R no tiene curva de niveles** (solo 200 usuarios), y C en R tiene una sola corrida publicable. La variación de C en R no se puede informar.

### 1.5 Un defecto de la plataforma de pruebas, encontrado en la primera corrida

La primera corrida de R (100 usuarios) dio **2 012 timeouts de conexión de 4 200 peticiones**. Era el NAT de la VPC (64 puertos por VM): 4 351 paquetes descartados con motivo `OUT_OF_RESOURCES`. Se corrigió (`min_ports_per_vm = 2048`) y se repitió todo. Esa corrida **no se publica**; su diagnóstico está en [`hallazgo_nat/`](./hallazgo_nat/nat_agotado_diagnostico.txt). Es la nota 57.

---

## 2. Resultados

Tablas completas, una fila por corrida y con el contraste del generador: [`tablas.md`](./tablas.md). Las métricas de infraestructura de cada corrida son **una o dos muestras de Cloud Monitoring** (una por minuto, y una corrida dura de 1 a 5 minutos), así que son indicativas; la serie continua está en [`campana_completa/`](./campana_completa/serie_temporal.csv).

**R frente a E, 200 usuarios** (media ± desviación estándar [mínimo–máximo]; `caliente` = latencia del servidor sobre conexión ya abierta):

| Serie | Operando | Corridas | Pet./s | p95 caliente (ms) | CPU API máx. (%) | CPU base máx. (%) | Instancias de API |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: |
| A (4–6 s) | R | 3 | 33,0 ± 0,7 [32,3–33,6] | 40 ± 1 [39–41] | 17 ± 5 | 30 ± 1 | 1 |
| A (4–6 s) | E | 3 | 33,0 ± 0,3 [32,7–33,3] | 36 ± 2 [35–38] | 13 ± 2 | 32 ± 4 | 1 |
| B (1–2 s) | R | 3 | 108,2 ± 4,8 [102,8–111,7] | 45 ± 4 [41–49] | 23 ± 10 | 34 ± 3 | 1 |
| B (1–2 s) | E | 3 | 107,5 ± 3,3 [103,9–110,2] | 48 ± 14 [38–64] | 25 ± 23 | 34 ± 4 | 1 a 2 |
| C (0,25–0,75 s) | R | 1 | 250,1 | 99 | 7 | 18 | 1 |
| C (0,25–0,75 s) | E | 2 | 260,5 ± 10,4 [253,2–267,9] | 163 ± 45 [131–195] | 15 ± 2 | 27 ± 5 | 2 |

**Línea base de A7** (portátil, 1 VM, `3affa11`; solo las magnitudes comparables, A7 §1.4): rendimiento 32,5 pet./s (A200) y 104,6 (B200); latencia caliente p95 128 y 123 ms. **No se concluye nada de la diferencia de latencia (128 ms frente a 36–48 ms):** cambió el generador y el camino de red, y es justo lo que A7 advirtió.

**Nota sobre las primeras corridas de E:** una sonda de solo lectura de 100 usuarios (19:30, para comprobar el NAT) ya había hecho escalar la API a 2 instancias, así que la escalera de 1 a 25 usuarios y `E-A100-r1/r2` corrieron con **2 instancias existentes** (columna «API inst.» de [`tablas.md`](./tablas.md)); desde `E-A-nivel50` hasta `E-A200-r3` y `E-B-nivel100` fue 1. No cambia nada de lo concluido, pero hay que decirlo.

**Serie sostenida (E, 8 corridas seguidas con la API ya en 3 instancias):** rendimiento 100,9–111,5 pet./s y latencia caliente p95 de 45 a 50 ms, **igual que con una sola instancia**; la CPU de la base sube (§3.4). Detalle en [`tablas.md`](./tablas.md) §4.

---

## 3. Análisis

### 3.1 R y E no se distinguen a 33 y a 108 pet./s

Mismo rendimiento (33,0 frente a 33,0; 108,2 frente a 107,5) y latencias dentro de una desviación estándar. **Es lo esperado y no es un resultado a favor ni en contra de la elasticidad:** una sola instancia de API trabaja con 13–25 % de CPU a 108 pet./s, muy lejos de saturarse, así que añadir instancias no tiene qué mejorar. La elástica **no penaliza** estos niveles: los percentiles y los errores son los mismos.

### 3.2 A 255 pet./s, E no mejora el rendimiento y su cola es peor

E y R rinden lo mismo (260 frente a 250 pet./s, y el balanceador y la API ni se acercan a su capacidad: la CPU de la API no pasa de 29 %). El **p95 de E es 163 ms frente a 99 ms de R**, con la salvedad de que R tiene una sola corrida publicable y E dos (131 y 195 ms). La explicación más probable —**no demostrada**— es que durante esta serie el autoescalador estaba sumando instancias (la API pasó de 1 a 2 en `E-C200-r1`, y el balanceador muestra un pico de p99 de unos 340 ms hacia las 21:00 UTC, durante la serie C y justo antes de que el grupo llegue a 3, [gráfica](./capturas/2026-11-01_campana-completa-instancias-cpu-balanceador.png)): una instancia que acaba de entrar atiende con la caché y los pools fríos, y su cola de latencia se paga mientras se calienta. Lo que sí está medido es la **única respuesta 503** de toda la campaña, en el momento de ese escalamiento (`E-C200-r1`, 3 ms, 1 de 4 153 peticiones).

### 3.3 La plataforma sí escala sola

Con carga, el grupo de la API subió **de 1 a 2 instancias** al empezar la primera serie, **a 3** en la sostenida (21:01) y bajó de nuevo a 2 y 1 al cesar la carga (la gráfica muestra los tres escalones). El escalamiento tarda **de 3 a 8 minutos** en aparecer tras el inicio de la carga (la gráfica: ~6–8 min tras la sonda de las 19:30, ~3–4 min tras las corridas B200 de las 20:49, ~2–5 min tras la serie C), que es más de lo que dura una corrida de las del plan: por eso la serie S existe, y por eso un solo nivel de 200 usuarios no la ve.

### 3.4 ¿Se desplazó el cuello de botella? No se llegó a ninguno

Ningún recurso se saturó: la API tuvo como máximo **51 %** de CPU en una de sus instancias en un minuto (entrada de una instancia nueva) y una corriente de 14–33 % en las demás, PostgreSQL **52 %** de CPU, **52 de 100** conexiones y 987 transacciones por segundo. **El punto de degradación no se encontró** y este escenario, con 200 cuentas y 3 intentos de quiz por cuenta, no puede empujar mucho más: la serie C ya rozaba lo que el guion permite (255 pet./s).

Sí hay un dato sobre la base: **con 3 instancias de API (serie S) la CPU de PostgreSQL fue de 47–52 % para 100–111 pet./s, y con 1 instancia (B200) fue de 29–36 % para el mismo rendimiento**, con 30–51 conexiones frente a 20–27. La misma carga cuesta más a la base cuando la sirven tres instancias. No se midió la causa; hay tres candidatas que quedan como hipótesis: pools de conexiones por instancia con más conexiones abiertas, más trabajo de mantenimiento tras los `DELETE` de cada reinicio de cuentas (la serie S reinicia cada ~100 s), o el crecimiento de `audit_logs` (solo se añade). Para G7: **si el límite se desplaza, será hacia PostgreSQL** (1 vCPU, `max_connections = 100`, hasta 25 conexiones por proceso de API), pero **esta campaña no lo demuestra**.

### 3.5 Estimación (no medida) del margen

A 250 pet./s una instancia de API usó 7–29 % de CPU y PostgreSQL 18–30 %; extrapolando linealmente (algo que A7 §1.5 advierte que hay que tomar con cuidado) la API aguantaría varios cientos de pet./s más y la base sería lo primero en limitar. **Es una estimación, no un resultado.**

---

## 4. Integridad

- **32 de 32** corridas con el estado de la base correcto: cada estudiante con exactamente 3 envíos, una inscripción activa y **cero** envíos duplicados del mismo intento (`verificacion_bd.txt` y `estado.txt` de cada corrida, directo en PostgreSQL desde la VM).
- El paso 11 del recorrido (envío duplicado con la misma `Idempotency-Key`) valida el mismo `submission_id` e intento en cada sesión: **0 fallos de validación en 153 113 peticiones.**
- No se hizo el cruce adicional con la auditoría que proponía el plan (§2.6); la consistencia entre réplicas del envío duplicado ya está probada, con ese cruce, en E2.

---

## 5. Lo que no quedó medido

- El punto de degradación y el desplazamiento del cuello de botella (§3.4).
- R con 100 usuarios y la curva de niveles de R.
- La variación de la serie C en R (una sola corrida publicable; `R-C200-r2` se descartó porque el contraste del generador no pudo darla por buena: su ventana de 37 s no alcanzó a tener la latencia del balanceador).
- La variante «ráfaga de login» de la escalera (necesita la contraseña de las cuentas y no se pasó a la VM; `login_rafaga.log`).
- La causa del aumento de CPU de la base con 3 instancias.
- El tiempo de escalamiento exacto: con una muestra de Monitoring por minuto, solo se sabe que ocurre «de 3 a 8 minutos» después de que empieza la carga. G5 (ciclo de escalamiento de la API) es quien lo mide.

---

## 6. Para G7 (y para quien compare con la Entrega 2)

1. Dos generadores y dos caminos de red: **declarar la diferencia** (nota 57 y §1.3).
2. La latencia caliente de la Entrega 3 (35–48 ms) y la de A7 (123–128 ms) **no se comparan**.
3. Con este escenario, **la elasticidad no mejora lo que no está saturado**. Lo que muestra el escenario es que escala, no que rinda más.
4. El 6,67 de la Entrega 2 se explica en `A7/COMPARACION.md` §5; G3 no lo cambia.

---

## 7. Coste y estado del entorno

| | |
| :--- | :--- |
| VM generadora | creada a las 18:19 UTC, apagada tras bajar los resultados: ~4 h × ~0,13 USD/h ≈ **0,5 USD** |
| Instancias extra de API | 1 o 2 más durante ~1,5 h en total, por el autoescalador; el importe facturado se consulta en la consola de facturación (esta cifra es una estimación de horas, no un importe) |
| Estado al cerrar | API en 1–3 (valor de Terraform), generador apagado (`terraform apply` sin `generator_enabled`), `/run/mooc-db-pw` desaparece con la VM |

---

## 8. Archivos

| Qué | Dónde |
| :--- | :--- |
| Una carpeta por corrida | [`resultados/`](./resultados/) · `resultados.jtl` (muestras originales), `resumen.json`, `destino.json`, `generador.csv`, `verificacion_bd.txt`, `metricas_gcp.json`, `serie_temporal.csv`, `contraste_generador.json` |
| Tablas | [`tablas.md`](./tablas.md) (las genera `scripts/g3_informe.py`) |
| Serie continua de toda la campaña | [`campana_completa/`](./campana_completa/) |
| Gráfica de la campaña | [`capturas/2026-11-01_campana-completa-instancias-cpu-balanceador.png`](./capturas/2026-11-01_campana-completa-instancias-cpu-balanceador.png) |
| Registros de la campaña | [`registros/`](./registros/) |
| La corrida descartada por el NAT | [`hallazgo_nat/`](./hallazgo_nat/nat_agotado_diagnostico.txt) |
| Procedimiento | [`RUNBOOK.md`](./RUNBOOK.md) · `scripts/g3_campana.sh`, `scripts/h3_nube.sh`, `scripts/sql_nube_vm.sh` |

Este archivo no contiene credenciales, llaves ni secretos.
