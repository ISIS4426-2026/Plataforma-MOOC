# Evidencia C1 — Cloud SQL: instancia, conexión privada, SSL, pool y migraciones (issue #118)

| Archivo | Criterio que demuestra |
| :--- | :--- |
| [`terraform_plan.txt`](./terraform_plan.txt) | Instancia en una zona sin réplicas · IP privada · SSL obligatorio, todo declarado en Terraform |
| [`migraciones_ejecutor.txt`](./migraciones_ejecutor.txt) | El ejecutor de migraciones, contra una base vacía igual a la que nace en Cloud SQL |
| [`plan_deriva_de_estado.txt`](./plan_deriva_de_estado.txt) | No es un criterio: es el hallazgo de la [nota 16](../../NOTAS_TECNICAS.md), capturado |

**Estado: la infraestructura está declarada y planificada, no aplicada.** Los dos
criterios de la ola 1 —`psql` por IP privada desde la VM, y las migraciones
aplicadas contra la instancia— necesitan un `terraform apply` y un host dentro de
la VPC.

La configuración está verificada contra el estado real: el plan del estado
posterior a la mezcla sale en `5 to add, 0 to change, 0 to destroy`, así que lo que
falta es ejecutarlo, no escribirlo.

---

## Lo que dice el plan

```
Plan: 5 to add, 0 to change, 0 to destroy.
```

**Los 5 son C1:** la instancia, la base, el usuario y los dos permisos de lectura
del secreto. Y el `0 to destroy` es la otra mitad de la verificación: **C1 no toca
ni un recurso de B2, B3 ni C3.** Es la convención de un archivo por componente
funcionando — `database.tf` se escribió en paralelo a la red y al bucket de Diego
sin tocar nada suyo.

Ese cero costó un hallazgo. En la primera pasada el plan decía `9 to destroy`, y
los nueve eran el bucket de C3 con su IAM y sus prefijos: estaban vivos en GCP y
en el estado compartido, pero su `storage.tf` no estaba en `main` ni en ninguna
rama remota. Aplicar habría borrado el trabajo de otra persona **y habría
terminado en verde**. Está capturado en
[`plan_deriva_de_estado.txt`](./plan_deriva_de_estado.txt) y explicado en la
[nota 16](../../NOTAS_TECNICAS.md); de ahí salió la sexta regla de
[`infra/terraform/README.md`](../../../../infra/terraform/README.md), porque no hay
salvaguarda técnica: la única barrera es leer la última línea del plan.

Lo que el plan confirma de C1, línea por línea:

| Criterio del issue | En el plan |
| :--- | :--- |
| Una sola zona, sin réplicas | `availability_type = "ZONAL"`, `location_preference.zone = "us-east1-b"` |
| IP privada | `private_network = ".../networks/mooc-vpc"` |
| Sin IP pública | `ipv4_enabled = false` |
| SSL obligatorio | `ssl_mode = "ENCRYPTED_ONLY"` |
| Límite de conexiones conocido | `database_flags { max_connections = 100 }` |
| Declarado en Terraform | el plan mismo |

`password = (sensitive value)`: Terraform no la imprime. El archivo se comprobó
contra el valor real de Secret Manager y da **cero coincidencias**.

### La dependencia que no se ve

```hcl
depends_on = [google_service_networking_connection.private_vpc_connection]
```

Sin esa línea, la creación de la instancia compite con el peering de B3 y falla
con un error que no menciona la red. Es la trampa menos evidente del archivo, y
la razón por la que C1 dependía de B3.

---

## El presupuesto de conexiones

`max_connections` se **declara** en lugar de heredar el valor por defecto de
Google, que depende de la memoria del perfil y cambiaría solo si alguien cambia
el perfil. Declarado, el pool se dimensiona contra un número conocido:

| | Conexiones |
| :--- | ---: |
| API (VM de D2), `DB_MAX_OPEN_CONNS` | 25 |
| Worker (VM de E1), `DB_MAX_OPEN_CONNS` | 25 |
| Reserva de superusuario | 3 |
| Agentes de Google | ~5 |
| **Comprometido** | **~58** de 100 |

Lo que importa es que **son dos VMs sumando contra el mismo tope**, que es
exactamente el riesgo que anticipó la nota 11 de
[`NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md). Los ~42 restantes son el margen
para las migraciones, un `psql` de diagnóstico y el solapamiento de un reinicio
—durante el cual conviven un momento el pool viejo y el nuevo—.

Las inactivas se dejan iguales a las abiertas a propósito: contra Cloud SQL cada
conexión nueva paga un saludo TLS, y en el escenario 1 interesa medir la
plataforma, no los saludos.

---

## El SSL, exigido por los dos lados

La instancia está en `ENCRYPTED_ONLY`, así que **rechaza en el servidor** cualquier
conexión en claro. Eso ya cumple el criterio, pero el arranque de la aplicación
lo exige también (`internal/config/validate.go`, ampliado en este issue):

```
- DATABASE_URL usa sslmode=prefer, que deja el cifrado a criterio del servidor;
  usar require, verify-ca o verify-full
```

Puede parecer redundante. No lo es, y el caso que lo justifica es `prefer`:
contra esta instancia acabaría cifrando igual, así que funcionaría, así que nadie
lo corregiría — y el día que la cadena apunte a otro servidor viajaría en claro
sin que cambie una línea de código.

La comprobación entiende las dos formas de cadena que admite libpq (la URL y la
de pares `clave=valor`), porque si solo entendiera una, cambiar de forma la
desactivaría en silencio. Hay pruebas para ambas, y una más para que una
contraseña con `&` o `?` dentro no confunda la lectura.

**Se comprobó que la prueba tiene dientes:** al añadir `prefer`, `disable` y la
cadena vacía a la lista de modos aceptados, cuatro pruebas fallan. Restaurada la
lista, vuelven a verde.

No se exige certificado de cliente (`TRUSTED_CLIENT_CERTIFICATE_REQUIRED`):
obligaría a distribuir y rotar certificados por VM, y lo que aporta —autenticar
al cliente— ya lo da el acceso privado.

---

## Las migraciones: el hueco que C1 tuvo que cerrar

En local el esquema lo aplica el hook de inicialización de Postgres
(`scripts/init-db.sh` montado en `/docker-entrypoint-initdb.d`), que corre una vez
cuando el volumen está vacío. **Cloud SQL no tiene ese hook**: la instancia nace
con `moocdb` creada y sin una sola tabla.

Sin nada más, migrar la instancia administrada sería pegar siete archivos a mano
y recordar cuáles ya se aplicaron. Eso funciona una vez. De ahí
`scripts/migrate.sh`, verificado en `migraciones_ejecutor.txt` en cinco
escenarios:

| | Resultado |
| :--- | :--- |
| Base vacía | Aplica las 7 en orden |
| Se repite | «Nada que aplicar»; sirve para aplicar solo la migración nueva |
| Una migración falla | Código 3, **ni registro ni esquema a medias** |
| `--verify` | 17 tablas, las 5 restricciones, los 2 disparadores de auditoría, el límite de conexiones |
| `--baseline` sobre base vacía | Se niega (código 1) |

**El tercero es el que importa.** La migración y su registro en
`schema_migrations` van en la misma transacción, así que el estado que arruina una
base —aplicada sin registrar, o registrada sin aplicar— no puede ocurrir. Se
demostró introduciendo a propósito una migración con un `CREATE TABLE` válido
seguido de SQL inválido: el `CREATE` se ejecuta, luego falla, y después no queda
ni la tabla ni el registro.

El quinto guarda contra el peor uso de `--baseline`: sobre una base vacía dejaría
la tabla de control diciendo que todo se aplicó y una base sin nada, y el script
no volvería a intentarlo.

### Las migraciones ya eran portables, y no por casualidad

Se revisaron contra las restricciones de Cloud SQL: **cero `CREATE EXTENSION`,
nada no transaccional** (ningún `CONCURRENTLY`, ningún `VACUUM`), **ningún rol**.
El único `BEGIN` del árbol es el cuerpo de una función PL/pgSQL, no control de
transacción, así que `--single-transaction` es seguro.

Que no haga falta ninguna extensión pese a usar `gen_random_uuid()` en 22 columnas
es porque esa función es parte del núcleo desde PostgreSQL 13. Es una razón más
para que `database_version` esté fijado a `POSTGRES_16` y no a algo anterior.

---

## Lo que falta y por qué

Los dos criterios de la ola 1 no se pueden cerrar con lo que existe hoy:

**1. `psql` por IP privada desde la VM.** No hay VM todavía —es el issue #123
(D2)—, y la instancia no tiene IP pública, así que desde un portátil no hay a
dónde conectarse. Es el diseño funcionando, no un obstáculo: si se pudiera
comprobar desde fuera, el criterio «desde fuera no» estaría incumplido.

**2. Las migraciones aplicadas contra la instancia.** Necesitan un host dentro de
la VPC, es decir lo mismo.

El ejecutor ya está verificado contra una base vacía de PostgreSQL 16, que es
exactamente el estado en que nace la instancia, así que lo que falta es correrlo,
no escribirlo.

Hay una salida sin esperar a D2: una VM pequeña y temporal en `mooc-subnet`, con
SSH por IAP —la regla de firewall `mooc-allow-ssh-iap` que B3 ya creó existe para
esto—, migrar, capturar la evidencia y borrarla. El procedimiento completo está en
[`infra/terraform/README.md`](../../../../infra/terraform/README.md#sin-vm-todavía-el-host-temporal).

### El orden, que no es negociable

1. ~~El `storage.tf` de C3 tiene que estar en `main`.~~ **Hecho** (PR #153). Es lo
   que hizo que el plan pasara de `9 to destroy` a `0`.
2. Mezclar C1 a `main`.
3. `git pull` en `main` y **aplicar desde ahí** (sexta regla del README).
4. La VM temporal, migrar, `--verify`, comprobar que desde fuera no se llega, y
   borrarla.
5. Detener la instancia si no va a usarse de inmediato:
   `db_activation_policy = "NEVER"`.

Invertir 1 y 3 es exactamente la avería de la nota 16.

---

## Sobre el coste, que aquí no es un detalle

La instancia son **49,31 USD/mes de cómputo más 1,70 de almacenamiento** según la
estimación de B1. El crédito por integrante es de 50 USD, de modo que **esta sola
instancia funcionando 24×7 se lleva un cupón completo en un mes**.

No cambia la decisión de dimensionamiento —está justificada en
[`CONFIGURACION_Y_COSTOS.md`](../../CONFIGURACION_Y_COSTOS.md) y bajar de ahí
significa volver al núcleo compartido y perder la reproducibilidad de la
medición— pero sí cambia cuándo conviene aplicar: la instancia debería existir
para las pruebas y no entre ellas.

Para eso `database.tf` expone `db_activation_policy`, con `ALWAYS` y `NEVER`.
Detenerla es un cambio de código y un `apply`, no un clic, y **es deliberado que
sea código y no variable de entorno**: si viviera en el entorno de cada uno, el
`apply` de quien no la exportara volvería a encender la instancia en silencio y el
aviso llegaría en la factura. Cuánto se ahorra exactamente hay que contrastarlo
contra el informe de facturación tras la primera parada — es lo que pide la regla 5
de B1 y lo único de este apartado que no se puede afirmar sin medirlo.

Las copias de seguridad diarias (3 días de retención) se facturan por tamaño
usado; con una base de megabytes, quedan por debajo del céntimo al mes y no
alteran la estimación.

---

## Nunca

Los dos archivos son salidas de verificación y **no contienen ninguna
credencial**, comprobado y no supuesto:

* En `terraform_plan.txt` la contraseña sale como `(sensitive value)`. El archivo
  se contrastó contra el valor real de Secret Manager: **cero coincidencias**.
* En `migraciones_ejecutor.txt` no aparece ninguna cadena de conexión.
  `migrate.sh` recibe `DATABASE_URL` por entorno y nunca la imprime —ni en los
  mensajes de error—, así que ni siquiera la contraseña del Compose local llegó
  al archivo.
