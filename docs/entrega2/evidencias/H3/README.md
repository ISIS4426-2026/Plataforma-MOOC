# Evidencia H3 — Escenario 1: ejecución, análisis y cuello de botella (issue #133)

Ejecución del plan acordado en [`capacity-planning/escenario1.md`](../../../../capacity-planning/escenario1.md) contra el entorno desplegado en GCP (`https://34.24.52.111.sslip.io`), el **2026-09-27 entre las 22:04 y las 23:07 (hora de Colombia)**. Configuración fija durante toda la campaña (ver §1); ninguna VM se redimensionó ni se reinició.

## Resultado en tres frases

1. **Hasta 200 estudiantes concurrentes (≈ 32 peticiones/s) la plataforma no se saturó:** 0 errores reales, 0 timeouts y 0 fallos de validación funcional en las 8 corridas, con la integridad de intentos, calificación y progreso confirmada directamente en Cloud SQL tras cada una.
2. **Por eso 200 usuarios es el máximo probado y NO la capacidad máxima:** no se encontró el punto de degradación. La serie de presión que lo buscaría (acortar las pausas hasta saturar) **no se ejecutó**.
3. **Lo que sí degrada la latencia de cola es el establecimiento de conexiones nuevas, no el procesamiento:** sobre conexiones ya abiertas, el p95 del servidor es ≈ 117 ms; el p95 global (≈ 380 ms) sube por el ~9,5 % de peticiones que abren conexión TCP/TLS, y de ellas una parte espera ≈ 1 s de más, un patrón consistente con la retransmisión del primer paquete de la conexión (origen aún sin confirmar).

## Estado frente a los criterios del issue

| Tarea / criterio | Evidencia | |
| :--- | :--- | :---: |
| Línea base con carga baja | `A-nivel1_*` (1 usuario) | ✅ (con la salvedad del §5) |
| Al menos tres niveles crecientes, infraestructura fija | 10 → 25 → 50 → 100 → 200 usuarios | ✅ |
| Repetir la medición cerca del límite | Nivel de 200 (el máximo probado) tres veces | ✅ (no hubo límite que repetir: se repitió el máximo) |
| p50, p95, p99, throughput, errores y timeouts por corrida | [`tabla_escalera.txt`](./resultados/tabla_escalera.txt) y `resumen.txt`/`resumen.json` de cada corrida | ✅ |
| Métricas de VMs, base y cola en cada corrida | `metricas_gcp.json` de cada corrida | ⚠️ VMs y base sí; **la API no reporta métricas propias** (ver §6) y la cola no interviene en este escenario |
| Integridad de intentos, calificación y progreso bajo concurrencia | `verificacion_bd.txt` y `estado.txt` de cada corrida | ✅ |
| Identificar el punto de degradación, o el máximo probado | Máximo probado: 200 usuarios / 32,8 peticiones/s | ✅ (máximo probado, no capacidad máxima) |
| Relacionar la latencia con API, cola, pool de conexiones y PostgreSQL | §4 y §6 | ⚠️ parcial (API y cola sin métrica propia) |
| Proponer un cambio y la medición que lo respalda | §7 | ✅ |
| Resultados numéricos por corrida con su variación | §3 | ✅ |

## 1. Configuración durante la campaña

| Componente | Configuración |
| :--- | :--- |
| Web Server | `mooc-web-server`, `e2-highcpu-2` (2 vCPU, 2 GiB), nginx 1.27 + API Go, `DB_MAX_OPEN_CONNS=25` |
| Worker Server | `mooc-worker-server`, `e2-highcpu-2`; aloja Redis (sesiones, límites, idempotencia); no procesa trabajos en este escenario |
| Base de datos | Cloud SQL PostgreSQL 16, `db-custom-1-3840` (1 vCPU dedicada, 3,75 GiB), zona única |
| Generador | JMeter 5.6.3 (`justb4/jmeter`) en Docker, en un portátil Windows 11 con Ryzen 5 5500U **fuera de la VPC**; RTT ≈ 74–90 ms hasta `us-east1` |
| Recorrido | 14 pasos por sesión (9 lecturas, 5 escrituras), 3 sesiones por usuario, pausa 4–6 s, rampa de 30 s (60 s para 100 y 200) |
| Orquestación | `bash scripts/h3_nube.sh escalera` y `… repetir 200 4000 2000 2` (reinicio de cuentas, corrida, verificación en la base y pausa de 2 min entre corridas) |

## 2. Resultados por nivel

Ventana medida: se omiten los primeros 45 s (75 s para 100 y 200 usuarios) de cada corrida. Utilización: Cloud Monitoring, muestras de 60 s (media / máxima). `tps` = transacciones por segundo de PostgreSQL.

| Corrida | Usuarios | Peticiones medidas | Pet./s | p50 ms | p95 ms | p99 ms | Errores reales | Timeouts | Fallos de validación | CPU web % | CPU SQL % | Conexiones a la base (máx.) | tps (máx.) | Estado en la base |
| :--- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| Línea base | 1 | 33 | 0,2 | 113 | 1 576 | 1 711 | 0 | 0 | 0 | 5,5 / 12,7 | 10,8 / 11,2 | 4 | 4 | OK |
| Nivel 1 | 10 | 352 | 1,8 | 92 | 328 | 1 298 | 0 | 0 | 0 | 5,8 / 8,1 | 10,6 / 11,0 | 4 | 19 | OK |
| Nivel 2 | 25 | 882 | 4,5 | 96 | 375 | 1 378 | 0 | 0 | 0 | 5,5 / 6,2 | 11,7 / 13,3 | 6 | 47 | OK |
| Nivel 3 | 50 | 1 770 | 8,9 | 93 | 338 | 1 289 | 0 | 0 | 0 | 7,2 / 10,4 | 12,8 / 15,6 | 6 | 93 | OK |
| Nivel 4 | 100 | 3 234 | 16,5 | 93 | 372 | 1 373 | 0 | 0 | 0 | 8,8 / 13,3 | 14,3 / 18,1 | 7 | 174 | OK |
| Nivel 5 (corrida 1) | 200 | 6 473 | 32,8 | 94 | 380 | 1 365 | 0 | 0 | 0 | 12,4 / 15,0 | 20,2 / 27,1 | 15 | 348 | OK |
| Nivel 5 (repetición 1) ‡ | 200 | 6 529 | 32,0 | 96 | 789 | 1 901 | 0 | 0 | 0 | 11,5 / 18,5 | 17,9 / 26,7 | 27 | 342 | OK |
| Nivel 5 (repetición 2) | 200 | 6 482 | 32,3 | 94 | 377 | 1 364 | 0 | 0 | 0 | 19,4 / 33,9 | 20,3 / 26,0 | 17 | 356 | OK |

**Integridad tras cada corrida**, consultada directamente en Cloud SQL (`capacity_verificar_estado.sql`): con N usuarios, exactamente N estudiantes con exactamente 3 envíos (3 × N en total), N inscripciones activas y **0 envíos duplicados del mismo intento**, incluso con los reenvíos con la misma `Idempotency-Key` del paso 11. Las 8 corridas dan `OK` (600 envíos únicos en cada una de las tres de 200 usuarios).

### Variación en el nivel de 200 usuarios (3 corridas)

| Métrica | Media | Desv. est. | Mín. | Máx. | CV |
| :--- | :-: | :-: | :-: | :-: | :-: |
| Peticiones/s | 32,4 | 0,4 | 32,0 | 32,8 | 1,1 % |
| p50 (ms) | 94,7 | 0,9 | 94 | 96 | 1,0 % |
| **p95 (ms)** | 515 | 194 | 377 | **789** | 37,6 % |
| **p99 (ms)** | 1 543 | 253 | 1 364 | 1 901 | 16,4 % |
| CPU máx. Cloud SQL (%) | 26,6 | 0,5 | 26,0 | 27,1 | 1,7 % |
| CPU máx. web (%) | 22,5 | 8,2 | 15,0 | 33,9 | 36,6 % |
| Conexiones a la base (máx.) | 19,7 | 5,2 | 15 | 27 | 26,7 % |

El rendimiento, el p50 y la carga de la base son **muy estables** (CV ≈ 1–2 %). Lo que varía es la cola de la distribución (p95, p99). Sin la repetición 1, el p95 es de 378,5 ms (377 y 380 ms en las dos corridas).

‡ **La repetición 1 quedó afectada por el propio equipo generador**, y hay que decirlo con claridad: coincidió con actividad adicional en el mismo portátil (una sesión de trabajo de asistencia que ejecutaba comandos y scripts de lectura en paralelo, en el equipo con solo ~600 MiB libres). En esa corrida el contenedor de JMeter tuvo un pico de CPU de 688 % frente a 193–233 % en las otras dos, y hubo una ventana de 15 s (a los 165 s) con 116 peticiones de más de 1 s repartidas en **todos** los pasos por igual. En el servidor no hubo nada anormal (CPU web máx. 18,5 %, CPU SQL 26,7 %). Se conserva en los datos, no se descarta, pero se reporta como corrida con generador perturbado.

## 3. Análisis: ¿dónde está el límite?

**No se alcanzó.** Ningún nivel activó un criterio de saturación del plan (fallos reales > 1 %, p95 > 1 s sostenido, plateau del rendimiento, fallos de validación) ni de parada. El rendimiento creció linealmente con la carga (0,2 → 1,8 → 4,5 → 8,9 → 16,5 → 32,8 peticiones/s) sin aplanarse: la curva no dobla en ningún punto probado. **Por eso 200 usuarios / 32,8 peticiones/s es el máximo probado, no la capacidad máxima.** No hay evidencia de que exista un cuello de botella *en el rango probado*; lo que sigue son las razones por las que tampoco puede afirmarse que la capacidad sea mucho mayor, y qué indica la evidencia sobre qué se agotaría primero.

### La cola de latencia viene de conectar, no de procesar

En el nivel de 200 usuarios, separando las peticiones que abrieron una conexión nueva (TCP + TLS) de las que reutilizaron una:

| | Con conexión reutilizada (≈ 90 % de las peticiones) | Con conexión nueva (≈ 9,5 %) |
| :--- | :--- | :--- |
| Latencia | p50 92–94 ms · **p95 117–119 ms** · p99 319–416 ms | Tiempo de conexión p50 ≈ 260–284 ms · **p95 ≈ 1 285 ms** |

Las peticiones con latencia entre 1,2 y 1,7 s (la "segunda joroba" de la distribución) son **casi todas peticiones que abrieron conexión y tardaron más de 900 ms en establecerla**: 161 de 161 en la primera corrida, 151 de 152 en la repetición 2. Un tiempo de conexión de ≈ 1 s adicional es la huella de una retransmisión del primer paquete de la conexión (el temporizador inicial es de 1 s). Es una observación **del lado del cliente**: con los datos de este experimento no se puede distinguir si el paquete se pierde en la red entre el generador y GCP o en la cola de conexiones del servidor; hace falta el contador de descartes de SYN del servidor (`nstat`/`netstat -s`) antes y después de una corrida, que requiere acceso a la VM.

### Qué recurso tiene menos margen, y qué se puede y no se puede afirmar

| Recurso | A 32 peticiones/s | Tendencia (media, 0,2 → 32,8 peticiones/s) | Extrapolación **lineal** a 80–85 % |
| :--- | :--- | :--- | :--- |
| CPU de Cloud SQL (1 vCPU dedicada) | media 20 %, máx. 27 % | +0,29 puntos por petición/s | ≈ 240 peticiones/s |
| CPU del Web Server (2 vCPU) | media 12 %, máx. 15–34 % | +0,21 puntos por petición/s | ≈ 380 peticiones/s |
| Pool de conexiones de la API (`DB_MAX_OPEN_CONNS=25`) | 15–27 conexiones abiertas en la base | no lineal (depende de la simultaneidad) | **27 en un pico**: cerca del tope de ≈ 29 (25 de la API + 4 de base) |
| Memoria del Web Server | 31 % | plana | — |
| Red del generador | ≈ 1 KB por petición → ≈ 30 KB/s | — | 5,8 Mbps admiten > 700 peticiones/s |

**Lectura honesta:** la CPU de la base es el recurso con menos margen entre los medidos, y el pool de conexiones ya rozó su tope en un pico (27 de ≈ 29 posibles) con la carga desordenada de la repetición 1. Ninguno se agotó. La extrapolación lineal (≈ 240 peticiones/s, unas 7 veces lo probado, ≈ 1 200 estudiantes con pausas de 4–6 s) **es una estimación con seis puntos, no una medición**: solo la serie de presión, no ejecutada, puede confirmarla o desmentirla.

## 4. Relación de la latencia con cada componente

| Componente | Qué se observó | Conclusión |
| :--- | :--- | :--- |
| **API** | Sin métricas propias en Cloud Monitoring (`up = 0` hasta el redespliegue de H1). Se infiere de la latencia sobre conexión reutilizada: p50 92 ms frente a un piso de red de ≈ 74–90 ms, es decir ≈ 5–20 ms de proceso por petición | La API no es el factor de la cola de latencia |
| **Cola (asynq)** | Este escenario no encola trabajos; el worker estuvo a 3–8 % de CPU (solo aloja Redis de sesiones) | No interviene |
| **Pool de conexiones** | 15 / 27 / 17 conexiones abiertas máximas en las tres corridas de 200; tope teórico ≈ 29 | Sin agotarse, pero es lo más cercano a un límite duro |
| **PostgreSQL / Cloud SQL** | CPU 20 % media, máx. 27 %, 348 tx/s; memoria 26 %; ≈ 10 transacciones de PostgreSQL por petición (348 tx/s ÷ 32,8 peticiones/s) | Holgada; el recurso con menor margen de CPU |
| **Conexión TCP/TLS** | El 100 % de la cola de 1,2–1,7 s se explica por establecer conexión | Es el origen de la variación del p95 entre corridas |

## 5. Salvedades que hay que leer con los números

- **Línea base (1 usuario):** su p95 de 1 576 ms sale de solo ≈ 30 muestras y de una conexión nueva lenta; con 10 usuarios el p95 es 328 ms. **No es un resultado, es un artefacto de la muestra.** Por eso el orquestador no cuenta la saturación por debajo de 10 usuarios.
- **Repetición 1 del nivel de 200** con el generador perturbado (‡ arriba).
- **El generador es un solo portátil con una sola IP:** por eso el login quedó fuera del recorrido medido (límite de 10 por minuto por IP). Las corridas con carga mayor a 33 peticiones/s no se probaron y el generador podría convertirse en el límite antes que la plataforma (su CPU llegó a 190–690 % de un núcleo con 12 hilos disponibles y ≈ 850 MiB de memoria).
- **El catálogo de la nube crece** con cada corrida de otros escenarios (ya supera las 20 entradas de la primera página): el paso 1 ya no exige que el curso de la prueba salga en ella.
- **Las métricas de Cloud Monitoring son de 60 s**, así que un pico de segundos queda promediado.

## 6. Lo que no se hizo, y por qué importa

| Pendiente | Por qué |
| :--- | :--- |
| **Serie de presión** (200 usuarios con pausas de 1–2 s, 0,5–1 s…) para buscar el punto real de degradación | El tiempo disponible antes de la entrega; es lo único que convertiría el «máximo probado» en un límite medido |
| **Métricas de la API** en Cloud Monitoring | Requiere redesplegar `docker-compose.prod.yml` de H1 (puerto `127.0.0.1:8080` del contenedor `api`) |
| **Contadores de red del servidor** (descartes de SYN) | Requiere acceso SSH a la VM web |
| **Confirmar con una segunda repetición del nivel de 200 sin actividad en el generador** | La repetición 1 quedó perturbada; conviene una tercera limpia |

## 7. Propuesta de cambio y medición que la respalda

**Propuesta principal: primero confirmar de dónde viene la espera de ≈ 1 s al conectar y comprobar el pool, antes de pedir más capacidad.**

1. **Averiguar el origen de la espera al conectar y actuar según el resultado.** *Medición que la respalda:* el p95 de una petición sobre conexión reutilizada es ≈ 117 ms frente a ≈ 1 285 ms de tiempo de conexión en las que conectan, y el 100 % de las peticiones de 1,2–1,7 s son de conexión nueva (§3). *Qué hacer:* leer en la VM web los contadores de descartes de SYN antes y después de una corrida (`nstat -az TcpExtListenDrops TcpExtListenOverflows`). Si crecen, es la cola de conexiones del servidor y la corrección es subir el `backlog` de nginx (`listen 443 ssl backlog=…`) y `net.core.somaxconn`; si no crecen, la pérdida está en la red del generador y no hay cambio de servidor que la arregle. En ambos casos, **habilitar `ssl_session_cache` y HTTP/2 en nginx** reduce el número de handshakes completos que un cliente real necesita. *Qué mediría el resultado:* la proporción de peticiones con tiempo de conexión > 900 ms (hoy ≈ 2 % del total) y el CV del p95 entre repeticiones (hoy 37 %).
2. **Antes de aumentar `DB_MAX_OPEN_CONNS`, medirlo:** el pool llegó a 27 conexiones abiertas de un tope de ≈ 29, con Cloud SQL al 27 % de CPU. *Medición que respaldaría subirlo:* que en la serie de presión el p95 crezca mientras la CPU de la base se mantenga por debajo de 50 % y las conexiones queden clavadas en el tope. El presupuesto de conexiones (`max_connections = 100`, ≈ 58 comprometidas, nota 11) deja margen para subirlo un poco, no para duplicarlo.
3. **Redesplegar H1 y correr la serie de presión** para convertir el máximo probado en un límite: sin las métricas de la API no se puede separar el tiempo de proceso del tiempo de espera de una conexión en el pool.

**Lo que esta evidencia no respalda:** cambiar el tipo de VM, agregar réplicas de lectura o un balanceador. Con la CPU del web al 12–34 % y la de la base al 27 %, ninguna de esas medidas aumentaría la capacidad medida; solo se justificaría con datos de la serie de presión que hoy no existen.

## 8. Reproducir

```bash
# Escalera completa (~1 h; pide la contraseña de prueba una vez)
bash scripts/h3_nube.sh escalera
# Repetir un nivel
bash scripts/h3_nube.sh repetir 200 4000 2000 2 A-nivel200-rep
# Serie de presión (no ejecutada en esta entrega)
bash scripts/h3_nube.sh presion 200 1000:1000 500:500 300:300 200:200
# Métricas de Cloud Monitoring por corrida, y el análisis de este README
python scripts/capacity_metricas_gcp.py --todas docs/entrega2/evidencias/H3/resultados
python scripts/capacity_analisis_h3.py
```

## Archivos

| Archivo | Qué es |
| :--- | :--- |
| `resultados/<corrida>/resultados.jtl` | Muestras originales, una fila por petición |
| `resultados/<corrida>/resumen.txt` · `resumen.json` | Resumen clasificado (éxito / negocio / validación / real) |
| `resultados/<corrida>/metricas_gcp.json` | VMs y Cloud SQL durante la ventana de la corrida |
| `resultados/<corrida>/verificacion_bd.txt` · `estado.txt` | Integridad en Cloud SQL tras la corrida |
| `resultados/<corrida>/generador.csv` | CPU y memoria del contenedor de JMeter cada 10 s |
| `resultados/tabla_escalera.txt` | Tabla final que imprime el orquestador |
| [`analisis_h3.txt`](./analisis_h3.txt) | Salida de `scripts/capacity_analisis_h3.py` (tablas de este README) |
| [`seccion_escenario1_para_informe.md`](./seccion_escenario1_para_informe.md) | Redacción con los números reales para el informe consolidado (I3) |

## Nunca

Credenciales, llaves ni secretos. Los `.jtl` no guardan cabeceras ni cuerpos; los tokens de sesión viven solo en `capacity-planning/datos/` (en `.gitignore`).
