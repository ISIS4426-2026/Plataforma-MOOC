# Evidencia B3 — Inicialización automática y reproducible de cada instancia nueva ([#202](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/202))

| Criterio de aceptación | |
| :--- | :---: |
| Una instancia creada por el grupo atiende peticiones o consume trabajos sin acceso manual | ⏳ §6 — requiere C1 y C5 |
| Dos reinicios seguidos no duplican procesos, derivados ni recursos | ✅ §3 — por construcción; comprobación en §6 |
| Todas las instancias de un grupo y una corrida ejecutan la misma versión de código y configuración | ✅ §2 — estructural |
| Tiempo de arranque registrado para los dos grupos | ⏳ §4 — el mecanismo mide; el número necesita el despliegue |
| **Bloquea el cierre:** captura de la consola del grupo creando una instancia y del log hasta lista | ⏳ §6 — **requiere C1 y C5** |
| Anotado en `NOTAS_TECNICAS.md` | ✅ nota 32 |

**Estado: mecanismo escrito y validado; sin desplegar.** `terraform validate`
pasa, `fmt` no propone cambios, el guion pasa `bash -n`, y **B3 no añade ni
cambia ningún recurso**: el plan es idéntico al de B2
([`plan_b3.txt`](./plan_b3.txt)).

---

## 1. Qué sustituye

Hasta B3, el arranque dejaba la máquina a medias: instalaba Docker y nada más. El
resto estaba descrito en `ADMINISTRACION.md` como pasos manuales por SSH.

| Paso | Antes | Ahora |
| :--- | :--- | :--- |
| Dependencias del anfitrión | guion de arranque | guion de arranque |
| Autenticar contra Artifact Registry | a mano | `gcloud auth configure-docker` con la identidad de la instancia |
| `/etc/mooc/<rol>.conf` | **`sudo tee` a mano** | generado de la metadata |
| Código de despliegue | clonado en `/home/dfortizr1/mooc` | clonado en `/opt/mooc` **en el commit de `IMAGE_TAG`** |
| Unidad de systemd | **`sudo tee` a mano**, con esa ruta personal dentro | versionada en `infra/bootstrap/`, instalada desde el repositorio |
| Aplicar el esquema | `bash scripts/migrate.sh` a mano | `ExecStartPre`, con el lock de A3 |
| Levantar los contenedores | `docker compose up -d` a mano | `systemctl enable --now` |
| Saber si quedó lista | mirar | sonda `/api/v1/ready` y medición en `/var/lib/mooc/arranque.json` |

Dos cosas de la forma anterior **no sobreviven a un grupo administrado**, y son
la razón del issue:

- `/home/dfortizr1/mooc` **no existe** en una instancia nueva, y la unidad de
  systemd llevaba esa ruta escrita dentro.
- `/etc/mooc/<rol>.conf` escrito a mano es configuración que solo existe en un
  disco, y una recreación la borra. Es la **nota técnica 19**, y el propio
  `prepare_web_env.sh` ya lo advertía en un comentario.

## 2. La decisión de diseño: metadata en tiempo de ejecución, no `templatefile`

`mooc-bootstrap.sh` **no se parametriza desde Terraform**. Lo lee todo del
servidor de metadatos cuando arranca.

Consecuencia: **los bytes del guion son idénticos para los dos roles y para todas
las instancias**. Lo único que distingue una de otra es su metadata. Eso convierte
el criterio «todas las instancias de un grupo y una corrida ejecutan la misma
versión de código y configuración» en una propiedad estructural, en vez de algo
que haya que vigilar en cada plantilla.

### Un solo valor pin las dos cosas

`mooc-image-tag` es el SHA completo de 40 hexadecimales, y de él salen **las
dos** mitades del despliegue:

| | De dónde |
| :--- | :--- |
| La imagen del contenedor | `IMAGE_TAG` en el `.env` que leen los Compose |
| El `docker-compose.*.yml`, los guiones de entorno y las unidades | `git checkout --detach <IMAGE_TAG>` en `/opt/mooc` |

Clonar `main` y correr la imagen de otro commit es cómo se acaba depurando una
diferencia que no está en el código que se está leyendo. Con un solo valor eso no
puede pasar.

Y es el SHA y no una etiqueta móvil: con `latest`, dos instancias del mismo grupo
pueden quedarse en imágenes distintas según cuándo arrancaron. `variables.tf` lo
valida con una expresión regular, así que un SHA corto o una etiqueta detienen el
`apply`.

### El contrato de metadata, para C1 y C5

| Clave | Rol | Obligatoria | Qué es |
| :--- | :--- | :---: | :--- |
| `mooc-role` | ambos | sí | `web` o `worker` |
| `mooc-image-tag` | ambos | sí | SHA de 40 hex |
| `mooc-db-ip` | ambos | sí | IP privada de Cloud SQL |
| `mooc-queue-ip` | ambos | sí | IP privada **estática** de la cola (B1) |
| `mooc-project-id` | ambos | no | por omisión, el proyecto de la Entrega 2 |
| `mooc-region` | ambos | no | por omisión `us-east1` |
| `mooc-repo-url` | ambos | no | por omisión el repositorio público |
| `mooc-app-domain` | web | sí | dominio con el que responde |
| `mooc-smtp-from` | web | sí | remitente verificado |
| `mooc-smtp-username` | web | sí | login SMTP |
| `mooc-hls-bucket` | worker | sí | bucket de derivados |
| `mooc-worker-concurrency` | worker | no | por omisión 2 |

`infra/terraform/bootstrap.tf` las expone ya resueltas en
`local.mooc_metadata_comun`, `local.mooc_metadata_web` y
`local.mooc_metadata_worker`, así que una plantilla se escribe así:

```hcl
metadata = merge(local.mooc_metadata_comun, local.mooc_metadata_worker)
```

El guion **valida cada valor y se para** si falta o tiene mala forma. Con un
grupo administrado eso es importante en los dos sentidos: una instancia que
arranca mal y no lo dice es peor, pero una que muere en bucle también lo es, así
que el mensaje va al registro serie, que es donde la consola del grupo lo muestra.

## 3. Idempotencia

El agente invitado ejecuta el guion **en cada arranque**, así que la idempotencia
no es una propiedad deseable: es un requisito.

| Paso | Por qué no duplica |
| :--- | :--- |
| Dependencias | comprueba `command -v docker` y `git` antes de instalar |
| Clonado | si `/opt/mooc/.git` existe, hace `fetch` y `checkout`, no otro clon |
| `/etc/mooc/<rol>.conf` | se reescribe entero; no se acumula |
| Unidad de systemd | `install` sobre la misma ruta, no otra unidad |
| Contenedores | `docker compose -p mooc up -d` reutiliza los que ya existen |

### `-p mooc` no es decorativo

Sin nombre de proyecto explícito, Compose lo deriva **del directorio**. Si la
ruta cambiara, los contenedores de la ruta anterior quedarían como un proyecto
distinto en lugar de reutilizarse — que es exactamente la duplicación que el
criterio prohíbe. Con `-p mooc` el proyecto no depende de dónde esté el fichero.

### Las unidades heredadas se retiran

Si el disco viene de la época anterior, en `/etc/systemd/system` hay una unidad
que apunta a la ruta personal. El guion la detecta —no contiene `/opt/mooc`—, la
desactiva y la aparta con sufijo `.b3-respaldo`. Sin eso habría dos unidades
levantando Compose desde rutas distintas; como los nombres de contenedor son
fijos (`mooc-api`, `mooc-worker`), no se duplicarían: **fallaría una de las dos**,
que es peor porque el fallo no se parece a su causa.

### Derivados

Un reinicio a mitad de una tarea no duplica derivados HLS: el worker es
idempotente y asynq reencola la tarea
(`internal/worker/idempotency_test.go`). Lo que sí deja es basura en el
directorio de trabajo, y de eso se encarga la unidad.

### El esquema sí se aplica al arrancar, y eso lo decidió A3

**A3 ([#195](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/195)) eligió
«arranque protegido con el lock, no paso explícito de despliegue»**, y su criterio
de aceptación lo dice: «el procedimiento vive en el script de arranque, no en
pasos manuales». Así que las unidades ejecutan, **antes de levantar los
contenedores**, exactamente lo que A3 §4 especifica:

```bash
bash ./scripts/migrate.sh            # aplica lo que falte, protegido por el lock
bash ./scripts/migrate.sh --require  # y se niega a seguir si aun falta algo
```

Va en `infra/bootstrap/mooc-migrate.sh`, que lee `DATABASE_URL` de
`/opt/mooc/.env` en vez de recibirla en la línea de órdenes: así la contraseña
no aparece ni en el `ExecStartPre` ni en el registro de systemd.

La segunda línea parece redundante y no lo es: cubre el caso en que la primera
no llegue a ejecutarse, y convierte un esquema desfasado en un arranque que falla
en vez de en una instancia que atiende contra una base que no corresponde.

**Por qué es seguro que tres instancias lo hagan a la vez.** Porque A3 lo
arregló: cada migración toma un advisory lock **de transacción** antes de
comprobar si ya está aplicada, y la comprobación ocurre dentro del lock. A3
reprodujo el defecto anterior --tres instancias simultáneas daban **dos
arranques fallidos de tres**, y el grupo administrado las recreaba para que
fallaran igual, en bucle-- y lo volvió a medir con el aplicador nuevo: tres
códigos de salida 0. La **nota técnica 5** describe el defecto; A3 es su arreglo.

Lo ejecutan **los dos roles**. Con el lock da igual quién llegue primero, y que
el worker también exija el esquema con `--require` evita que procese contra una
base que no corresponde.

Va en la unidad y no en el guión de arranque para que también corra en un
`systemctl restart`: un despliegue que solo cambia `IMAGE_TAG` pasa por la
unidad, no por el arranque de la instancia, y el esquema nuevo tiene que entrar
igual.

> **Esto corrigió un error de la primera versión de B3**, que no migraba al
> arrancar «dejándolo a A3». A3 ya había decidido lo contrario, así que un
> entorno nuevo habría arrancado sin esquema. De paso aparecio el segundo fallo:
> el guión no instalaba `postgresql-client`, que `migrate.sh` necesita, y el
> arranque anterior solo lo ponía en la VM web.

### Lo que el arranque deliberadamente NO hace

- **No instala FFmpeg en el anfitrión.** Va dentro de la imagen del worker
  (`Dockerfile.worker`, `apk add ffmpeg`), así que la tarea del issue
  «dependencias multimedia disponibles en el arranque de los workers» se cumple
  por la imagen. Instalarlo también aquí sería una dependencia duplicada que
  nadie usa.
- **No siembra datos.** `seed.sh` es una herramienta de desarrollo, y desde B1
  además se niega a vaciar un Redis que no sea local.

## 4. Limpieza del temporal, interrupciones y tiempo de arranque

### El plazo de parada por omisión mata un transcodificado

`docker compose down` manda SIGTERM y **mata a los 10 segundos**. El worker
atiende SIGTERM con una parada ordenada de asynq, que espera a que las tareas en
curso terminen, pero una tarea de verdad tarda minutos: el perfil largo midió
**1 711 s** de transcodificado en A7. Con el plazo por omisión, parar el
servicio mata un transcodificado a los diez segundos.

Que eso no sea pérdida de datos lo garantiza la idempotencia —asynq la reencola y
el worker la repite sin duplicar derivados— pero es trabajo tirado. La unidad usa
`--timeout 120` y `TimeoutStopSec=180`.

**120 segundos es un compromiso para el arranque y la operación manual, no una
política de drenaje.** La de verdad la decide **C7**, que tiene que apagar el
consumo de tareas nuevas *antes* de parar el contenedor, para que la espera no
dependa de cuánto quede de la tarea más larga.

### La limpieza va al arrancar, no al parar

El worker escribe originales y segmentos intermedios en el volumen
`worker_scratch`. Cada tarea limpia lo suyo, pero una tarea **interrumpida** deja
ahí un original de hasta 2 GiB, y varios reinicios seguidos llenarían el disco.

Se limpia en `ExecStartPre` y no al parar, a propósito: **al arrancar no hay
ninguna tarea en curso**, así que no se puede borrar nada que se esté usando. Al
parar, sí.

### Cómo se determina el tiempo de arranque

| | |
| :--- | :--- |
| Se mide desde | el inicio de `mooc-bootstrap.sh` |
| Se mide hasta | la sonda de listo responde |
| Sonda, rol web | `GET http://127.0.0.1:8080/api/v1/ready` |
| Sonda, rol worker | contenedor en marcha **y** `GET :9090/metrics` |
| Se escribe en | `/var/lib/mooc/arranque.json` y en el registro serie |

**No se mide desde la creación de la instancia**, y la diferencia importa: el
arranque del sistema operativo y del agente invitado van antes y no son nuestros.
Quien compare este número con `creation_timestamp` está sumando dos cosas
distintas. Los dos son útiles —el del grupo para dimensionar el periodo de
inicialización del autoescalado, este para saber cuánto tarda *nuestro* arranque—
pero no son el mismo.

La sonda del rol web es `/api/v1/ready` de **A2**, que prueba la base y la
bandera de arranque, no solo que el proceso respire. Es **la misma que usará el
balanceador en C3**, así que lo que aquí se mide es lo que allí decide si la
instancia recibe tráfico. A2 midió el arranque del *proceso* (93 ms); esto mide
el de la *instancia*, que es otra magnitud.

## 5. Por qué B3 no toca las dos VMs fijas

Se intentó, y el proveedor no lo permite sin destruir.

`metadata_startup_script` es **ForceNew en los dos sentidos**: cambiar su
contenido recrea la VM, y quitarlo para mover el guion a
`metadata["startup-script"]` —que sí se actualiza en sitio— también. El plan lo
dice con `# forces replacement` en las dos máquinas.

```
# google_compute_instance.web_server must be replaced
# google_compute_instance.worker_server must be replaced
      - metadata_startup_script = <<-EOT # forces replacement
```

Para una plantilla de grupo es irrelevante —una plantilla es inmutable y
cambiarla *es* crear otra—, así que el mecanismo entra limpio en C1 y C5. Pero
migrar las VMs heredadas sería **destruir infraestructura existente** que B3 no
necesita: su comprobación bloqueante es una captura de la consola **del grupo**
creando una instancia, que exige C1 y C5 de todos modos.

Las dos VMs fijas se van cuando los grupos las sustituyan, y entonces la
recreación la hace C1.

> Si se quisiera ejercitar el arranque antes de C1 —como ensayo—, se puede, y la
> consecuencia es recrear esas dos VMs. No se hizo por iniciativa propia porque
> es una destrucción, y los discos actuales llevan el montaje manual de la
> Entrega 2.

## 6. Qué falta para cerrar

| Criterio | Qué necesita |
| :--- | :--- |
| Una instancia del grupo atiende o consume sin acceso manual | **C1 y C5** |
| **La captura mientras ocurre** (bloquea el cierre) | **C1 y C5** |
| Tiempo de arranque de los dos grupos | el `apply`; el mecanismo ya lo mide y lo escribe |
| Dos reinicios seguidos sin duplicar | el `apply`; dos `systemctl restart` y comparar `docker ps` |

La captura **no se puede recrear después**: el entorno se destruye en I6, las
métricas de Cloud Monitoring caducan y relanzar cuesta dinero. Lo que hay que
capturar, con la marca de tiempo visible:

1. la consola del grupo creando la instancia;
2. el registro serie con las líneas `B3: ===== inicio del arranque =====` y
   `B3: ===== LISTO en Ns =====`;
3. `/var/lib/mooc/arranque.json` de una instancia de cada rol.

`capturas/` está creado y vacío a propósito.

### Tres variables nuevas que el `apply` necesita

| Variable | Cómo se pasa |
| :--- | :--- |
| `image_tag` | `TF_VAR_image_tag`, el SHA de 40 hex ya publicado con `publish_images.sh` |
| `smtp_from` | `TF_VAR_smtp_from` |
| `smtp_username` | `TF_VAR_smtp_username` |

Las dos de SMTP no son secretos —la contraseña vive en Secret Manager— pero
identifican la cuenta del equipo y el repositorio es público, así que no tienen
valor por omisión: se pasan como `TF_VAR_db_password`. Un valor ausente detiene
el `apply`, que es mejor que una instancia que arranca sin poder enviar correo y
entra en bucle de recreación.

`app_domain` sí tiene por omisión: vacío se calcula como
`<ip-de-la-vm-web>.sslip.io`, que es el valor que la Entrega 2 escribía a mano.
Cuando C3 ponga el balanceador delante hay que declararlo.

---

Este archivo no contiene credenciales, llaves ni secretos.
