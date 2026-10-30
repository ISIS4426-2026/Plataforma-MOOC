# Evidencia C4 — Verificaciones de salud y distribución de tráfico entre réplicas ([#207](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/207))

| Criterio de aceptación | |
| :--- | :---: |
| Evidencia de reparto con dos o más instancias identificadas | ✅ §2 — **25/15 de 40 peticiones** |
| Evidencia de exclusión de una instancia que no supera las verificaciones y de su reincorporación | ✅ §3 — **43 s** fuera, **278 s** de vuelta |
| Los dos mecanismos quedan explicados por separado y no se confunden | ✅ §4 |
| **Bloquea el cierre:** peticiones repartidas entre dos instancias, y la degradada saliendo y volviendo | ✅ seis capturas más [`reparto_y_salud.txt`](./reparto_y_salud.txt) |
| Anotado en `NOTAS_TECNICAS.md` | ✅ nota 40 |

Medido el 2026-10-28 contra el entorno desplegado, con
`bash ./scripts/c4_reparto_y_salud.sh`.

---

## 1. La comprobación de salud, con sus parámetros

Una sola, declarada en `infra/terraform/api_group.tf` y usada por **dos**
consumidores: el backend service del balanceador y el autohealing del grupo.

| Parámetro | Valor | Qué significa |
| :--- | :--- | :--- |
| Endpoint | **`/api/v1/ready`** | la sonda de A2: prueba la base **y** la bandera de arranque |
| Protocolo y puerto | HTTP, **80** | el balanceador termina TLS; dentro de la VPC va en claro (C3) |
| `check_interval_sec` | 10 | un sondeo cada diez segundos |
| `timeout_sec` | 5 | más de cinco segundos cuenta como fallo |
| `unhealthy_threshold` | 3 | **tres fallos seguidos** para declararla enferma: ~30 s |
| `healthy_threshold` | 2 | dos aciertos para volver: ~20 s |

**No es `/api/v1/health`,** y la diferencia importa: `health` dice que el proceso
vive y **aguanta la caída de la base**; `ready` prueba la dependencia y responde
503 mientras la instancia se monta. Lo que decide si una réplica recibe tráfico
tiene que ser lo segundo.

El umbral de tres no es casual: con uno, un sondeo perdido por un hipo de red
sacaría a una réplica sana de la rotación.

## 2. El reparto, con las réplicas identificadas

La API pone en cada respuesta la cabecera **`X-Instance-ID`**
(`internal/http/middleware/instance.go`, de A2), un identificador estable de 12
hexadecimales por réplica. No hace falta correlacionar registros: el destino
viene en la propia respuesta.

Correspondencia establecida preguntando a cada instancia por dentro:

| Instancia | `X-Instance-ID` |
| :--- | :--- |
| `mooc-api-l9rf` | `4a04e2ede9e8` |
| `mooc-api-sj3d` | `9cb53ef3626d` |

Sobre **40 peticiones** a `/api/v1/ready` a través del balanceador:

```
4a04e2ede9e8: 25      9cb53ef3626d: 15
```

Las dos reciben tráfico. El reparto no es exactamente mitad y mitad, y **no
tiene por qué serlo**: con `balancing_mode = "RATE"` el balanceador reparte hasta
la capacidad declarada por instancia (`max_rate_per_instance = 100`, C3), no a
turnos. Con 40 peticiones en unos segundos, las dos están muy por debajo de su
capacidad y el reparto depende de la proximidad del proxy de Google que atiende
cada conexión.

> Para una medición de reparto uniforme habría que sostener carga por encima de
> la capacidad de una réplica. Eso es **G5**, y ahí el reparto es una consecuencia
> del escalado, no el objeto de la prueba.

## 3. Degradar una réplica, y verla salir y volver

**Cómo se degrada:** se para el contenedor `api` de `mooc-api-l9rf`, no el proxy.
Así nginx sigue vivo y devuelve **502** a `/api/v1/ready`: la comprobación ve un
código incorrecto, que es un fallo de **aplicación** — lo que una sonda de
readiness existe para detectar. Parar el proxy daría conexión rechazada, que es
un fallo de máquina y prueba otra cosa.

| T+ | Salud de `l9rf` | Acción del grupo | Tráfico que recibe |
| ---: | :--- | :--- | :--- |
| 45 s | `HEALTHY` | `NONE` | 25 de 40 |
| **98 s** | `TIMEOUT` | **`RECREATING`** | **0** |
| 292 s | `TIMEOUT` | `VERIFYING` | 0 |
| **387 s** | `HEALTHY` | `NONE` | vuelve al reparto |

| | |
| :--- | ---: |
| **Exclusión** tras degradar | **43 s** |
| **Reincorporación** tras reparar | **278 s** |

Los 43 s cuadran con los parámetros: tres sondeos fallidos a diez segundos, más
la propagación de la decisión a los proxies del balanceador.

### Los 278 s de vuelta NO son del balanceador

Y es lo más instructivo de la corrida. Con `healthy_threshold = 2`, el
balanceador reincorpora en unos **20 s**. Tardó **278**.

La razón está en la columna de en medio: cuando se restauró el contenedor, el
grupo **ya había empezado su propia reparación** (`RECREATING`). Una instancia
con una acción en curso no vuelve a la rotación aunque su salud se recupere; hay
que esperar a que la acción se despeje (`RECREATING` → `VERIFYING` → `NONE`).

**La instancia no llegó a sustituirse:** conserva su id `4402968417801131874`,
el mismo que tenía antes de la prueba. La reparación se inició y se resolvió sin
crear una máquina nueva.

## 4. Los dos mecanismos, que no son el mismo

Es el criterio que pide no confundirlos, y la corrida los separa con números.

| | Exclusión por el balanceador | Reparación por el grupo |
| :--- | :--- | :--- |
| **Quién decide** | el backend service | el autohealing del MIG |
| **Qué hace** | deja de enviarle peticiones nuevas | repara o reemplaza la instancia |
| **Qué conserva** | la instancia sigue viva y accesible por IAP | puede destruirla y crear otra |
| **Escala** | **decenas de segundos** | **minutos** (160–273 s en C1) |
| **Reversible** | sí, en ~20 s si la aplicación se recupera | no: una vez recreada, es otra máquina |
| **Para qué sirve** | que nadie reciba un error mientras se arregla | que el grupo no se quede sin capacidad |

### El hallazgo: comparten la misma comprobación, y eso tiene precio

`google_compute_health_check.api` alimenta **a los dos**. La consecuencia la
midió esta corrida:

> **Un parpadeo de 30 segundos de la aplicación no solo saca la réplica de la
> rotación: dispara una reparación de la máquina.** Y volver cuesta 278 s en vez
> de los ~20 s que costaría si solo hubiera habido exclusión.

Es un compromiso real, no un defecto. Una sola comprobación es más simple y
garantiza que lo que decide el tráfico y lo que decide la reparación sean lo
mismo —que es justo lo que C1 y C3 buscaban—. Pero significa que **el autohealing
reacciona a fallos de aplicación como si fueran fallos de máquina**.

La alternativa, para quien la necesite, es dos comprobaciones: una ágil para el
balanceador y otra más tolerante —intervalo mayor, umbral más alto— para el
autohealing, de modo que solo recree ante fallos persistentes. Queda anotado para
**C6** y **C7**, que tienen la misma decisión del lado de los workers.

## 5. Las capturas

| Captura | Qué demuestra |
| :--- | :--- |
| `dos-replicas-sanas` | las dos en `CORRECTO`: la línea base del reparto |
| `reparto-y-exclusion-terminal` | la salida del guion: el reparto por identificador y cómo cae a cero |
| `replica-excluida` | una sana y la otra no, en la consola del grupo |
| `grupo-recreando-replica` | el autohealing actuando: `RECREATING` |
| `grupo-verificando-replica` | `VERIFYING`, el paso previo a volver |
| `replica-reincorporada` | las dos de vuelta en `CORRECTO` |

Las tres del medio son las que separan los dos mecanismos: la exclusión se ve en
el reparto de la terminal, y la reparación en la columna de acción de la consola.

## 6. Cómo se consiguieron dos réplicas, dicho explícitamente

Con `-var api_min_replicas=2`. **No es la «ampliación manual» que C1 descarta como
evidencia**, y conviene distinguirlo: lo que C1 prohíbe es decidir el tamaño a
mano para justificar su propio criterio de tamaño. Aquí el tamaño no es la
afirmación, es la **condición previa**; la afirmación es cómo se reparte el
tráfico y cómo se excluye una réplica enferma.

Que el grupo llegue a dos **solo**, por carga, es la política de C2 y la mide
**G5**.

---

Este archivo no contiene credenciales, llaves ni secretos.
