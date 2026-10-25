# Evidencia A4 — Sesiones, revocación y CSRF entre réplicas ([#196](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/196))

| Criterio de aceptación | |
| :--- | :---: |
| Un logout en una réplica invalida la sesión en las demás | ✅ §2 |
| Iniciar sesión en una réplica y operar en otra, sin reautenticar | ✅ §2 |
| Ninguna instancia conserva estado de sesión propio | ✅ §3 |
| El frontend puede hacer peticiones mutantes; un origen ajeno es rechazado | ✅ §4 |

---

## 1. El defecto

En la Entrega 2 cada VM levantaba su propio Redis (`docker-compose.prod.yml`),
así que **cada réplica tenía su propia caché de sesiones**.

Lo que *no* estaba roto: iniciar sesión en una réplica y usar otra.
`Authenticate` cae a PostgreSQL cuando la caché falla, así que la sesión
sobrevive al cambio de instancia.

Lo que sí: **el cierre de sesión**. `Logout` revoca en la base y borra **su**
entrada de caché. Otra réplica que tuviera la sesión cacheada la seguiría
sirviendo —su copia dice *usable*— hasta que expirara el TTL, que por omisión
son **24 horas**. Una sesión cerrada que sigue valiendo un día no es una
molestia: es un fallo de seguridad.

## 2. La corrección, y la prueba de que tiene dientes

La corrección es de despliegue, no de código: **una sola caché compartida fuera
de las instancias**. Las pruebas de `internal/auth/replicas_test.go` fijan el
comportamiento que esa decisión produce, montando dos servicios que comparten
base y caché — que es exactamente lo que son dos instancias detrás del
balanceador.

`TestElCierreDeSesionValeEnTodasLasReplicas` hace que la réplica B use la sesión
**antes** del logout, para que la tenga cacheada. Sin ese paso la prueba pasaría
por vacuidad: B iría a la base y vería la revocación de todos modos.

**Comprobado que falla con el defecto.** Dándole a cada réplica su propia caché,
como en la Entrega 2:

```
--- FAIL: TestElCierreDeSesionValeEnTodasLasReplicas
    replicas_test.go:132: la replica B siguio aceptando una sesion cerrada: err = <nil>
```

Con la caché compartida, las cuatro pasan:

| | |
| :--- | :--- |
| `LaSesionValeEnCualquierReplica` | iniciar en una y operar en otra |
| `ElCierreDeSesionValeEnTodasLasReplicas` | el defecto, cerrado |
| `RevocarTodasLasSesionesValeEnTodasLasReplicas` | lo mismo para la pantalla de sesiones |
| `SinCacheLasReplicasSiguenResolviendoContraLaBase` | ver §3 |
| `SuspenderLaCuentaCortaLaSesionEnTodasLasReplicas` | la suspensión, en §2.1 |
| `LaSuspensionNoDependeDeLaCache` | y sin caché tampoco |

### 2.1 La suspensión funciona por otra razón

Conviene no confundirla con lo anterior. `Authenticate` lee el usuario **de la
base en cada petición**; lo único que se cachea es la sesión. Así que suspender
una cuenta surte efecto en la siguiente petición de cualquier réplica, con caché
compartida o sin ella.

Es el reverso útil del diseño: si la suspensión dependiera de la caché, sacarla
de las instancias habría sido un cambio de riesgo y no una corrección.

## 3. Ninguna instancia conserva estado propio

`docker-compose.prod.yml` **ya no levanta Redis**. Quedan tres servicios —
`proxy`, `api`, `worker` — y `REDIS_URL` pasa a ser obligatoria:

```yaml
- REDIS_URL=${REDIS_URL:?REDIS_URL debe apuntar al servicio compartido, no a este host}
```

Sin valor por omisión a propósito: uno que apuntara aquí dentro devolvería en
silencio el defecto que esto arregla. El servicio lo aprovisiona **B1**.

**La contrapartida, y por qué es aceptable.** Sacar la caché de las instancias
podría haber cambiado un fallo de seguridad por un punto único de fallo. No lo
hace, porque `Authenticate` sigue cayendo a PostgreSQL: con la caché caída, las
dos réplicas siguen resolviendo sesiones. Lo fija
`TestSinCacheLasReplicasSiguenResolviendoContraLaBase`.

### Separación de la cola y las sesiones

Las sesiones van a la base lógica **1**; la cola de asynq se queda en la **0**
(`REDIS_SESSION_DB`).

No es aislamiento de memoria: **la política de expulsión es de la instancia
entera, no de la base**. Y ahí hay algo que B1 tiene que saber: asynq exige
`maxmemory-policy noeviction`, porque un trabajo expulsado es un trabajo
perdido. Con las dos cosas en el mismo servicio, **el control de memoria de las
sesiones es su TTL**, no la expulsión. Si las sesiones llenaran la memoria, lo
que rompe es la cola.

Lo que sí separa la base lógica es la operación: inspeccionar, medir o vaciar
una no toca la otra, y un `FLUSHDB` equivocado se lleva una sola cosa.

## 4. CSRF, CORS y la cookie

### La decisión que esto le pone delante a D1

La cookie de sesión era `SameSite=Lax` fija. **`Lax` solo la manda en
navegaciones de primer nivel**, así que un frontend servido desde *otro origen*
no podría autenticarse: su `fetch` saldría sin cookie. El login respondería 200,
dejaría su cookie, y la siguiente petición llegaría sin sesión — la clase de
fallo que se diagnostica mal.

| Dónde se aloje el frontend | Qué hace falta |
| :--- | :--- |
| **Detrás del mismo balanceador** ← recomendado | nada. Todo es del mismo sitio, `lax` basta y no hace falta CORS |
| En otro origen | las **tres** a la vez: CORS con credenciales, `SESSION_COOKIE_SAMESITE=none`, y el origen en `CSRF_ALLOWED_ORIGINS` |

Faltando cualquiera de las tres, el frontend no se autentica. Por eso están las
tres documentadas juntas en `internal/http/middleware/cors.go`.

### Lo que queda implementado

- **`SESSION_COOKIE_SAMESITE`**, por omisión `lax`. Un valor no reconocido cae
  en `lax` y el servidor **avisa**: fallar el arranque sería peor, pero hacerlo
  en silencio dejaría un frontend que no se autentica sin que nadie sepa por qué.
- **La cookie de borrado copia los atributos de la de sesión.** Con un
  `SameSite` distinto el navegador la trata como *otra* cookie y la de verdad
  sobreviviría al cierre de sesión. Lo fija `TestLaCookieDeBorradoCoincideConLaDeSesion`.
- **Sin `Domain`**, para que siga siendo una cookie `__Host-`: atada al host
  exacto. El dominio padre la compartiría con cualquier subdominio.
- **Middleware de CORS** que comparte lista con CSRF. Dos listas distintas se
  desincronizan, y las dos responden a la misma pregunta: quién puede hablar
  con esta API desde un navegador.

### Decisiones del CORS

| | |
| :--- | :--- |
| **Nunca comodín** | `Allow-Origin: *` es incompatible con `Allow-Credentials` según la especificación, y anularía la lista que sostiene CSRF |
| **Sin lista, no hace nada** | con el frontend en el mismo origen el middleware no añade ni una cabecera |
| **Un origen ajeno no recibe las cabeceras, ni un error** | bloquear la lectura es cosa del navegador; devolver 403 convertiría en fallo del servidor algo que es política del cliente |
| **El preflight no llega al enrutador** | no lleva cookie ni cabecera de traza que leer |
| **Las cabeceras pedidas se filtran** | devolver lo que pida el navegador sería anunciar que se acepta cualquiera |
| **Se exponen `X-Request-ID` y `X-Instance-ID`** | sin esto `fetch` no las ve, y son las que C4 y G5 necesitan para demostrar el reparto |

Ocho pruebas en `internal/http/middleware/cors_test.go`.

## 5. Afinidad de sesión

**No se usa, y no hace falta.** El estado de sesión vive en PostgreSQL y en la
caché compartida; ninguna réplica guarda nada propio. Las pruebas de §2
demuestran que cualquier réplica atiende cualquier sesión.

Activarla en el balanceador sería tapar el problema que esto resuelve: una
instalación con afinidad y caché local *parece* funcionar hasta que una réplica
se retira y sus sesiones se comportan de forma distinta a las demás. El
enunciado lo dice igual: *«la afinidad de sesión, si se utiliza, no reemplaza el
desacoplamiento del estado»*.

## 6. Lo que falta verificar

Todo lo de arriba está probado con pruebas unitarias que no necesitan nube. Lo
que **no** se ha ejercitado contra un despliegue real:

- Que `REDIS_URL` apunte al servicio de **B1**, que todavía no existe.
- El recorrido del navegador con el frontend de **D1**, que decidirá si hace
  falta `SameSite=none` y CORS.

**E1** ([#216](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/216)) es
quien cierra eso: comprueba sobre el entorno real que una sesión atendida por
dos instancias distintas funciona y que el logout se propaga.

---

Este archivo no contiene credenciales, llaves ni secretos.
