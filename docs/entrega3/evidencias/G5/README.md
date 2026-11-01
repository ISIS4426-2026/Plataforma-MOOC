# Evidencia G5 — Escenario 3A: ciclo de escalamiento automático de la API ([#225](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/225))

| | |
| :--- | :--- |
| **Bloque** | G — Pruebas de carga comparativas |
| **Rúbrica** | Pruebas comparativas |
| **Responsable** | `@CrispisCas9` |
| **Depende de** | C2 (#205), C4 (#207), E1 (#216), G1 (#222) — todos cerrados |
| **Desbloquea** | G7 (#227) |
| **Cómo se corre** | [`RUNBOOK.md`](./RUNBOOK.md), en la misma sesión que F1 |

**Estado: herramientas listas y probadas en seco; falta la corrida con el
entorno encendido.** Esta página se completa con los números de
`linea_de_tiempo.md` en cuanto exista.

---

## Criterios de aceptación

| Criterio | Cómo se demuestra | Estado |
| :--- | :--- | :---: |
| Al menos una creación automática y una reducción posterior, observadas y fechadas | `hitos.tsv` del observador: `DECISION_AMPLIAR`, `INSTANCIA_CREADA` (con el `creationTimestamp` de GCE), `DECISION_REDUCIR`, `INSTANCIA_RETIRADA` | ⏳ |
| Incorporación al balanceador demostrada con peticiones atendidas por la instancia nueva | el observador manda peticiones por el balanceador y agrupa por `X-Instance-ID`; como ese valor es `sha256(ID de la instancia)[:12]`, se sabe **qué** instancia respondió sin entrar por SSH | ⏳ |
| Los dos tiempos —reacción e incorporación— reportados por separado | `linea_de_tiempo.md`: reacción = cruce del umbral en el `.jtl` → decisión; incorporación = decisión → primera petición atendida, desglosada en creada / `RUNNING` / sana / atiende | ⏳ |
| La ampliación manual del grupo no aparece como evidencia | el observador se niega a empezar si el mínimo no es 1 o el grupo no está en 1; `g5_linea_de_tiempo.py` lee el **registro de auditoría** de la ventana y falla si una persona llamó a `resize`, `update-autoscaling` o similares | ⏳ |
| **Bloquea el cierre:** capturas de la línea de tiempo, la decisión del autoescalador y la consola durante el ciclo completo | seis capturas, cada una anunciada por el observador en su momento (§5 del runbook) | ⏳ |
| Lo que el equipo debe saber, en `NOTAS_TECNICAS.md` | tras la corrida: los dos tiempos (para G7 e I2) | ⏳ |

## Diseño de la corrida

**La señal no es la CPU** (C2, nota 37): son peticiones por segundo y por
instancia que cuenta el balanceador, con objetivo 0,8 sobre
`max_rate_per_instance = 100`. Se añade una instancia al pasar de **80 pet/s por
instancia**. Por eso la carga no se define por usuarios sino por **tasa**: cinco
etapas seguidas, cada una con un *Constant Throughput Timer* compartido por su
grupo de hilos ([`escenario3a_api.jmx`](../../../../capacity-planning/escenario3a_api.jmx)).

| Etapa | Tasa | Duración | Para qué |
| :--- | ---: | ---: | :--- |
| e1 base | 10 pet/s | 5 min | arrancar con baja carga y una instancia |
| e2 subida | 40 pet/s | 2 min | aumentar, por debajo del umbral |
| e3 subida | 70 pet/s | 2 min | aumentar, cerca del umbral |
| e4 alta | 130 pet/s | 15 min | por encima de 80 con una instancia: tiene que pedir 2. Con dos quedan ~65 cada una, así que se estabiliza en vez de oscilar, y sostiene la carga durante el arranque |
| e5 bajada | 10 pet/s | 20 min | reducir la demanda y observar la reducción (una instancia cada 600 s como máximo) |

Por qué **130 y no 200**: el criterio pide «al menos una» instancia adicional, y
C2 extrapola que a ~240 pet/s la base de 1 vCPU se acerca a su techo. Llegar a 3
es otra medición, con otro coste; se puede pedir con `RATE_ALTA=200`.

Las peticiones son lecturas (catálogo y `/ready`), como la calibración de G1: a
la señal le da igual qué petición cuenta, y así se puede repetir sin ensuciar la
base. **La integridad de las escrituras** durante el ciclo la cubre el arnés de
consistencia de E2, lanzado con el grupo ya en 2 por el autoescalador
(`consistencia_durante_el_ciclo/`).

## Herramientas

| Archivo | Dónde corre | Qué hace |
| :--- | :--- | :--- |
| [`g5_carga_escalonada.sh`](../../../../scripts/g5_carga_escalonada.sh) | VM del generador | JMeter por digest (nota 49), las cinco etapas, `etapas.tsv` con la hora UTC planificada de cada una, `destino.json` y salud del generador |
| [`g5_observar_escalamiento_api.sh`](../../../../scripts/g5_observar_escalamiento_api.sh) | portátil | comprueba el punto de partida, sondea cada 15 s grupo, autoescalador, instancias y reparto, anuncia hitos y capturas, y **sale con 1** si no hubo ampliación, atención por la nueva y reducción |
| [`g5_linea_de_tiempo.py`](../../../../scripts/g5_linea_de_tiempo.py) | portátil | cruza `.jtl`, observación y auditoría; escribe `linea_de_tiempo.md`, `tasa_10s.csv` y `auditoria.tsv`, y **sale con 1** si algo no cuadra |

### Cómo se probaron sin gastar

- `g5_linea_de_tiempo.py --autoprueba`: un ciclo sintético con tiempos conocidos
  (reacción 75 s, incorporación 135 s, un 502 durante el retiro) y el mismo ciclo
  con la etapa alta recortada al 67 % de su tasa, que **tiene que fallar**. Pasa.
- El observador, contra un `gcloud` y un `curl` simulados que recorren 1 → 2 → 1:
  sale con 0 y los hitos salen en orden. Con un grupo que no crece sale con 1
  (cuatro comprobaciones fallidas), y con un `X-Instance-ID` que no corresponde a
  ninguna instancia del grupo se para en la fase 1.
- La lectura de auditoría, con un registro simulado que incluye un `resize` de
  una persona: lo detecta, falla, y el correo no aparece en ninguna salida.
- `g5_carga_escalonada.sh`, con un `docker` simulado: las propiedades que llegan
  a JMeter, `etapas.tsv` y `destino.json` son los esperados; con `RATE_ALTA=70`
  se niega a empezar.

Lo que **no** se pudo probar aquí es el plan de JMeter contra JMeter real: el
XML es válido y sigue la forma de `calibracion_generador.jmx`, pero la primera
comprobación de verdad es la etapa e1. `g5_linea_de_tiempo.py` compara la tasa
lograda con la pedida en cada etapa y falla si la alta no llega al 90 %.

## Resultado

*Pendiente de la corrida.* Aquí van: los dos tiempos, la tabla por etapa, la
línea de tiempo resumida, el reparto con la instancia nueva y el enlace a las
seis capturas.

---

Este archivo no contiene credenciales, llaves ni secretos.
