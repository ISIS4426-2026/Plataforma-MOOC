# Evidencia B4 — Secretos y configuración de las plantillas sin valores sensibles ([#203](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/203))

| Criterio de aceptación | |
| :--- | :---: |
| Búsqueda de patrones de secreto en el árbol y en todo el historial sin coincidencias | ✅ §3 — **0 hallazgos**, reproducible |
| Las plantillas y la metadata se pueden publicar tal cual | ⚠️ §2 — las plantillas sí; la metadata, **salvo dos valores**, y está razonado |
| La evidencia explica las dos capas de identidad sin confundirlas | ✅ §4 |
| Anotado en `NOTAS_TECNICAS.md` | ✅ nota 33 |

---

## 1. Inventario: tres niveles, no dos

El issue pide separar «lo público de lo secreto». Dos cajones no alcanzan, y
forzarlos es lo que lleva a tratar como secreto algo que no lo es —o a publicar
algo que no se debe—.

### Nivel 1 — Secreto

Vive en **Secret Manager**, lo lee la identidad de la instancia en el arranque,
y no está en el repositorio, ni en la metadata, ni en una imagen.

| Secreto | Quién lo lee | Alcance del permiso |
| :--- | :--- | :--- |
| `db-password` | `sa-web-server`, `sa-worker-server` | sobre el secreto, no sobre el proyecto |
| `smtp-password` | `sa-web-server` **solo** | sobre el secreto |

El worker no aparece en la segunda fila a propósito: procesa vídeo y no manda
mensajes. Darle acceso ampliaría lo que alcanza una credencial filtrada sin que
nadie gane nada.

**Una salvedad que hay que decir:** `db_password` pasa por Terraform para crear
el usuario de la base, así que **queda en claro en el estado remoto**. Por eso el
estado vive en un bucket privado y versionado y nunca se versiona en git. No es
un descuido, es la consecuencia de que Terraform cree el usuario; evitarlo
exigiría crear el usuario fuera de Terraform y perder la reproducibilidad.

### Nivel 2 — No secreto, pero no para publicar

| Valor | Dónde vive | Por qué no es un secreto | Por qué no se publica |
| :--- | :--- | :--- | :--- |
| `smtp_from` | metadata de la instancia, `/etc/mooc/web.conf` | es una dirección de correo | identifica la cuenta del equipo |
| `smtp_username` | igual | **en Brevo el usuario ES el correo de la cuenta** | igual |

El proyecto ya había decidido esto, y con argumento, en `mail.tf`: tratarlos como
secretos «sería teatro», porque en SendGrid el usuario es la cadena literal
`apikey` y en Brevo el correo de la cuenta. **Lo que protege la cuenta es la
contraseña, y esa sí está en el nivel 1.**

### Nivel 3 — Público

Todo lo demás, y es la mayor parte: la configuración de Terraform, los Compose,
los guiones, el guion de arranque, los nombres de bucket, los rangos de red, las
etiquetas de cortafuegos. Está en un repositorio público y así debe estar.

### Lo que no existe, y es el resultado que más vale

**Ninguna llave de cuenta de servicio.** Ni para Terraform ni para las VMs: las
tres identidades van adjuntas a las máquinas y obtienen credenciales del servidor
de metadatos. Un secreto que no existe no se filtra, no caduca y no hay que
rotarlo.

## 2. ¿Se pueden publicar las plantillas y la metadata?

**Las plantillas, sí, tal cual.** No contienen ningún valor de los niveles 1 ni 2:
los secretos se leen en ejecución y los dos valores del nivel 2 llegan por
variables de entorno de Terraform, no escritos en el código.

**La metadata, no del todo**, y conviene decirlo en vez de redefinir el criterio
para que pase. `mooc-smtp-from` y `mooc-smtp-username` viajan en la metadata de la
instancia, porque el arranque tiene que leerlos de algún sitio y las dos únicas
opciones son metadata o Secret Manager — y Secret Manager para estos dos es el
teatro que `mail.tf` descartó.

Cómo se resuelve en la práctica:

- La metadata es visible para quien pueda describir la instancia, es decir
  miembros del proyecto. **Es el mismo nivel de exposición que
  `/etc/mooc/web.conf`**, que es donde vivían antes y donde el proyecto ya los
  consideraba aceptables.
- **En la evidencia van redactados**, siempre: la tabla del contrato de metadata
  de [B3 §2](../B3/README.md) los lista como claves obligatorias sin mostrar sus
  valores, y cualquier captura de la consola tiene que taparlos.

> Si se quisiera que la metadata fuera publicable **literalmente**, el arreglo es
> un secreto más en Secret Manager y unas diez líneas. No se hizo porque
> contradiría una decisión documentada del proyecto, y porque el riesgo real
> —perder la cuenta de correo— lo cubre la contraseña, que sí es secreta.

## 3. El barrido, y por qué afirma algo

`bash ./scripts/b4_barrido_secretos.sh` →
[`barrido_secretos.txt`](./barrido_secretos.txt). **0 hallazgos**, salida 0.

Cubre 288 commits y 2 995 objetos en todas las ramas, con seis comprobaciones.
Lo que importa del diseño es la tercera.

### Un barrido que lista coincidencias no sirve

La primera versión imprimía 22 líneas de URIs con contraseña. Todas eran
credenciales de desarrollo local, y eso es **peor que no comprobar nada**: un
informe con ruido no distingue «no hay secretos» de «nadie lee este informe».

Así que la comprobación **afirma**: toda contraseña que aparezca en una URI de
conexión tiene que ser una de las cuatro declaradas como de desarrollo.

| Declarada | Dónde y por qué |
| :--- | :--- |
| `moocpassword` | `docker-compose.yml`; es la constante `localDBPassword` de `validate.go` |
| `postgres` | valor por omisión de `config.go` para arrancar en local |
| `UnaContrasena*` | fixtures de `validate_test.go` |
| `a?b&c=d` | fixture de `TestValidateNoSeConfundeConUnaContrasenaConSimbolos` |

Cualquier otra es un hallazgo y el guion sale con 1. Los marcadores en mayúsculas
(`CONTRASENA`, `<PASSWORD>`) y las referencias a variable (`$VAR`, `${VAR}`) se
excluyen: un hueco a rellenar no es una contraseña, y confundirlo convierte el
barrido en ruido otra vez.

### La garantía real no es el barrido

`moocpassword` y `minioadmin` **están en el repositorio a propósito**: son las
credenciales de unos contenedores que solo existen en la máquina de quien los
levanta, y tenerlas ahí es lo que permite clonar y arrancar sin configurar nada.

Lo que las hace inofensivas no es una promesa, es que **`internal/config/validate.go`
rechaza el arranque** con `APP_ENV=production` si sobreviven a un despliegue. El
barrido lo comprueba en su sección 6: que las tres constantes sigan declaradas y
que `TestValidateRechazaValoresDeDesarrolloEnProduccion` pase.

Esa es la diferencia entre una convención, que se olvida, y una comprobación, que
no.

## 4. Las dos capas de identidad, sin confundirlas

Se confunden a menudo porque las dos se llaman «autenticación» y las dos acaban
en la base de datos. Son independientes y responden preguntas distintas.

### Capa A — Autorización de acceso al servicio cloud (IAM)

**Pregunta: ¿puede esta máquina hablar con este servicio de Google?**

| | |
| :--- | :--- |
| Quién es | la VM, por su cuenta de servicio adjunta |
| Cómo se prueba | token del servidor de metadatos, sin archivo de llave |
| Qué concede | leer un secreto, leer o escribir objetos de un bucket, bajar una imagen, escribir registros y métricas |
| Dónde se declara | `service_accounts.tf`, `storage.tf`, `mail.tf`, `database.tf`, `artifact_registry.tf` |
| Cómo se revoca | quitando el enlace de IAM; es inmediato y no hay que rotar nada |

### Capa B — Autenticación de la aplicación ante PostgreSQL

**Pregunta: ¿qué rol de base de datos es esta sesión, y qué puede hacer dentro?**

| | |
| :--- | :--- |
| Quién es | el rol `moocuser`, no la VM |
| Cómo se prueba | contraseña, leída de Secret Manager en el arranque, sobre `sslmode=require` |
| Qué concede | lo que ese rol tenga dentro de la base; las dos VMs comparten rol |
| Dónde se declara | `google_sql_user.app` en `database.tf` |
| Cómo se revoca | cambiando la contraseña y recargando la configuración; **no** desde IAM |

### Donde la confusión cuesta dinero, y un hallazgo

El error habitual es creer que `roles/cloudsql.client` «deja entrar» a la base.
No: ese rol concede `cloudsql.instances.connect`, que es lo que consumen **el
proxy de autenticación y los conectores** de Cloud SQL.

**Esta plataforma no usa ninguno de los dos.** Se conecta por TCP directo a la IP
privada de la instancia, y en ese camino **IAM no se consulta**: lo que permite la
conexión es la ruta de la VPC más la contraseña de PostgreSQL.

Consecuencias, las dos:

1. **Quitar `cloudsql.client` no cerraría la base**, y tener la contraseña sin él
   tampoco la abriría si faltara la ruta de red. Las dos capas son ortogonales, y
   es la ruta privada —no IAM— la que hace de primera barrera.
2. **Ese rol hoy no se usa.** Es un hallazgo de este inventario que **corrige la
   auditoría de B2**, donde se declaró que solo había un permiso sin uso.
   Retirarlo es de **F2**, con una comprobación contra un despliegue vivo: si
   alguna ruta lo necesitara, el fallo sería «permiso denegado» y no «conexión
   rechazada», y a ciegas esos dos se distinguen mal.

## 5. Los ejemplos de configuración

| Archivo | Estado |
| :--- | :--- |
| `.env.example` | ✅ nueve variables marcadas `[SECRETO]`, todas **sin valor** |
| `infra/terraform/terraform.tfvars.example` | **actualizado en B4** |

El ejemplo de Terraform estaba desfasado: no mencionaba las cuatro variables que
B3 añadió, y **tres de ellas son obligatorias**. Alguien que siguiera ese archivo
habría visto el `apply` pararse a preguntar sin entender por qué. Ahora lista las
tres obligatorias con su forma de pasarlas por entorno, las opcionales con su
valor por defecto, y una tercera sección con lo que no está ahí y no es un
olvido.

## 6. Qué falta

Nada que dependa de un despliegue. Dos cosas quedan **asignadas a otros issues**,
con el hallazgo ya escrito:

| Qué | Quién |
| :--- | :--- |
| Retirar `roles/cloudsql.client` tras comprobarlo contra un despliegue vivo | **F2** |
| Decidir si la metadata debe ser publicable literalmente, y pagar un secreto más | **F2** |

`capturas/` está creado y vacío: las comprobaciones de B4 son de árbol e
historial, y su salida es un fichero de texto reproducible, que es mejor evidencia
que una captura de pantalla de lo mismo.

---

Este archivo no contiene credenciales, llaves ni secretos.
