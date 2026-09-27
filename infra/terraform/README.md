# Infraestructura como código — Entrega 2

Issue **B2** (#115). Aprovisionamiento declarativo del entorno en GCP, pensado
para que **los cuatro integrantes puedan aplicar cambios sobre la misma
infraestructura sin pisarse**.

Las decisiones que esta configuración materializa —proveedor, región, zona,
perfiles y presupuesto— están justificadas en
[`../../docs/entrega2/CONFIGURACION_Y_COSTOS.md`](../../docs/entrega2/CONFIGURACION_Y_COSTOS.md)
(issue #114). Aquí solo se codifican.

---

## Qué aprovisiona, y qué no

| Sí | No |
| :--- | :--- |
| Habilitación de APIs del proyecto | **Despliegue de contenedores** — eso es Docker Compose sobre las VMs |
| Cuentas de servicio por componente, con IAM diferenciado | Imágenes, migraciones o datos sembrados |
| *(B3)* VPC, subredes y firewall | Configuración interna del sistema operativo |
| *(C3)* Bucket Cloud Storage, prefijos, IAM por componente y CORS | |
| *(C1)* Cloud SQL · *(D2, E1)* VMs | |

Terraform define **qué recursos existen**. Lo que corre dentro de ellos se
despliega aparte. Mezclar ambas cosas haría que un cambio de versión de la
aplicación pasara por `terraform apply`, y eso es exactamente lo que no
queremos.

---

## Puesta a punto, una vez por persona

**1. Instalar las herramientas**

Hacen falta dos: **Terraform** (cualquier versión 1.16.x) y el **Google Cloud
CLI**.

**Windows** — con `winget`, que ya viene en Windows 10 y 11 y no suele pedir
permisos de administrador:

```powershell
winget install --id Hashicorp.Terraform -e
winget install --id Google.CloudSDK -e
```

Con Chocolatey, si lo prefieres. Esta vía **sí necesita PowerShell como
administrador**, y a cambio permite fijar la versión exacta:

```powershell
choco install terraform --version=1.16.4 -y
choco install gcloudsdk -y
```

**macOS** — con Homebrew:

```bash
brew tap hashicorp/tap
brew install hashicorp/tap/terraform
brew install --cask google-cloud-sdk
```

**Linux (Debian/Ubuntu)**:

```bash
# Terraform
wget -O- https://apt.releases.hashicorp.com/gpg | sudo gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/hashicorp.list
sudo apt update && sudo apt install terraform

# Google Cloud CLI
curl https://sdk.cloud.google.com | bash
```

> **Después de instalar, reinicia la terminal — y si usas VS Code, reinicia VS
> Code entero.** El PATH no se refresca en una sesión ya abierta, y abrir una
> pestaña nueva dentro de VS Code **no basta**: hereda el entorno con el que
> VS Code arrancó. Es la causa número uno de «lo instalé pero no lo encuentra».

Comprueba que quedaron bien:

```bash
terraform version   # debe decir 1.16.x
gcloud version
```

**Sobre la versión de Terraform.** `versions.tf` la restringe a `~> 1.16.0`:
admite cualquier parche de la serie 1.16, pero no una versión menor distinta.
Así nadie migra el formato del estado sin querer y deja a los demás sin poder
leerlo. No hace falta que los cuatro tengan el mismo parche —los parches no
cambian ese formato—, pero sí que todos estén en 1.16.x.

Si `winget` instalara una 1.17 porque ya salió, Terraform se negará a ejecutar
con un mensaje sobre la restricción de versión. En ese caso, o se instala una
1.16.x, o se sube la restricción en `versions.tf` **y se avisa al equipo**,
porque afecta a todos.

Verificado con 1.16.4 (contenedor y CI) y 1.16.2 (winget).

**2. Autenticarse con tu propia identidad**

```bash
gcloud auth login
gcloud auth application-default login
gcloud config set project plataforma-mooc-entrega2
```

**No existen archivos de llave de cuenta de servicio, y no deben crearse.** Cada
persona actúa con su cuenta de Google, y Terraform toma esas credenciales por
defecto. No hay ningún secreto compartido que filtrar, rotar ni subir al
repositorio por accidente.

**3. Exportar la contraseña de la base**

La contraseña vive en **Secret Manager**, en el mismo proyecto. No hay que
pedírsela a nadie ni copiarla a mano: se lee al abrir la terminal.

```bash
export TF_VAR_db_password="$(gcloud secrets versions access latest \
  --secret=db-password --project=plataforma-mooc-entrega2 | tr -d '\r\n')"
```

En PowerShell:

```powershell
$env:TF_VAR_db_password = (gcloud secrets versions access latest `
  --secret=db-password --project=plataforma-mooc-entrega2)
```

Hay que repetirlo **en cada terminal nueva**: las variables de entorno no
sobreviven al cierre de la sesión.

> **Léela siempre así, no la copies a mano.** Si cada uno guardara su propia
> copia, bastaría una errata para que dos personas tuvieran valores distintos, y
> a partir de C1 cada `apply` cambiaría la contraseña de la base por la de quien
> lo ejecutó, **rompiendo las conexiones de la aplicación**. El vaivén sería
> difícil de diagnosticar, porque el código es idéntico para todos y lo que
> difiere es el entorno. Leyéndola de una única fuente, no puede ocurrir.
> El `tr` es necesario en Git Bash sobre Windows: elimina el `CR` de la
> salida CRLF de `gcloud`. Terraform también aplica `trimspace` como segunda
> barrera antes de configurar el usuario de Cloud SQL.

Si el comando falla por permisos, pide el rol `secretmanager.secretAccessor`
a quien administra el proyecto.

---

> **¿Eres quien administra el proyecto de GCP?** El bootstrap del bucket de
> estado y la concesión de accesos al equipo se hacen **una sola vez** y están
> en [`ADMINISTRACION.md`](ADMINISTRACION.md). Si alguien ya te dio acceso, no
> necesitas ese documento: sigue aquí.

## El día a día

```bash
cd infra/terraform

git pull                    # (1)
terraform init              # (2) solo la primera vez, o si cambian versiones
terraform plan              # (3) leerlo, no hojearlo
terraform apply             # (4)
```

### D1 — Registro y publicación de imágenes

Terraform crea el repositorio Docker `mooc` en Artifact Registry, en la misma
región `us-east1`, y activa tags inmutables. Las cuentas de servicio de las VMs
solo reciben `roles/artifactregistry.reader`; no se guardan llaves de servicio
en el repositorio.

Después de integrar A3, A5 y A6 en un commit, publicar las imágenes desde una
identidad personal con permisos de Artifact Registry:

```bash
gcloud auth login
gcloud config set project plataforma-mooc-entrega2
./scripts/publish_images.sh
```

El script exige un árbol Git limpio, usa el SHA completo de `HEAD`, construye
para `linux/amd64` y publica API y worker directamente con ese tag. Nunca se
deben pasar secretos mediante `docker build --build-arg`, `ENV` o archivos de
credenciales.

En cada VM, el despliegue consume el mismo tag inmutable:

```bash
export IMAGE_TAG="<SHA_COMPLETO_PUBLICADO>"
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

El valor de `IMAGE_TAG` debe corresponder al commit cuyas imágenes se
publicaron; no se debe usar `latest`.

**(1) Traer los cambios antes de planificar.** El estado es compartido pero el
código no: si aplicas desde una rama vieja, Terraform verá recursos en el estado
que tu código no declara y **propondrá destruirlos**. El `plan` lo mostraría,
por eso hay que leerlo.

**(3) Leer el plan de verdad.** Concretamente, revisar la línea del final: un
`Plan: 0 to add, 0 to change, 3 to destroy` inesperado casi siempre significa que
tu rama está desactualizada.

---

## Cómo se evitan los conflictos

**El estado vive en un bucket compartido, no en el disco de nadie.** Es la pieza
central: los cuatro leen y escriben el mismo estado, así que nadie tiene una
versión privada de la realidad.

**El backend de GCS toma un bloqueo.** Mientras alguien ejecuta `plan` o
`apply`, el estado queda bloqueado. Si otro lo intenta a la vez, ve:

```
Error: Error acquiring the state lock
Lock Info:
  ID:        1234...
  Operation: OperationTypeApply
  Who:       otro@gmail.com
  Created:   ...
```

Eso **no es un fallo, es el mecanismo funcionando**. Se espera a que termine y
se reintenta.

**Si un bloqueo se queda colgado** —porque a alguien se le cerró la terminal a
media operación— se libera con el identificador que aparece en el mensaje:

```bash
terraform force-unlock 1234...
```

Antes de hacerlo, **confirmar por el chat del equipo que nadie está aplicando**.
Forzar la liberación mientras otra persona escribe el estado es la única forma
realista de corromperlo.

**Si el estado se corrompiera**, el bucket tiene versionado: la versión anterior
sigue ahí y se puede restaurar.

### Un archivo por componente

Antes de las reglas, la convención que evita la mitad de los roces: **cada issue
escribe en su propio archivo**. Así tres personas tocan tres archivos distintos y
`git` no tiene nada que fusionar.

| Archivo | Issue |
| :--- | :--- |
| `network.tf` | B3 — VPC, subredes y firewall |
| `secrets.tf` | B4 — gestión de secretos |
| `storage.tf` | C3 — bucket y su IAM |
| `database.tf` | C1 — Cloud SQL |
| `compute.tf` | D2 y E1 — las dos VMs |
| `mail.tf` | F1 — el secreto del proveedor SMTP y quién lo lee |

`versions.tf`, `variables.tf`, `main.tf`, `outputs.tf` y `service_accounts.tf` ya
existen y son de todos: cambiarlos sí puede generar conflicto, así que conviene
avisar antes de tocarlos.

### Las seis reglas

1. **Nunca editar el estado a mano** ni descargarlo al repositorio. Contiene la
   contraseña de la base en claro.
2. **Nunca crear recursos desde la consola web.** Terraform no los conocería, y
   el siguiente `plan` intentaría crearlos otra vez o los ignoraría hasta
   chocar. Si ya ocurrió: `terraform import`, o borrarlo y dejar que Terraform
   lo cree.
3. **Un `apply` a la vez.** El bloqueo lo impone, pero avisar en el chat evita
   la espera.
4. **`git pull` antes de planificar.** Siempre.
5. **La contraseña se lee de Secret Manager, no se copia.** Si un `plan` propone
   cambiar la contraseña de la base sin que nadie haya tocado el código, es esto:
   alguien tiene exportado otro valor. No apliques — pregunta en el chat del
   equipo.
6. **El `apply` se hace desde `main`, después de mezclar. Nunca desde una rama de
   trabajo, y nunca con un `.tf` sin publicar.** Es la regla que más cuesta
   entender y la que más daño hace al romperse, así que va explicada aparte,
   justo debajo.

#### Por qué el `apply` va desde `main`, y qué pasa si no

El estado es compartido, el código no. De ahí salen dos averías distintas, y las
dos ocurrieron de verdad mientras se construía C1:

**Si aplicas desde una rama que no tiene el archivo de otro**, Terraform ve en el
estado recursos que tu código no declara y **propone destruirlos**. El plan de la
rama de C1 decía:

```
Plan: 5 to add, 0 to change, 9 to destroy.
```

Los 5 eran C1. Los 9 eran el bucket de C3 con su IAM y sus prefijos. Aplicar ahí
habría borrado el trabajo de otra persona, y el `apply` no habría fallado: habría
funcionado perfectamente.

**Si aplicas código que no has publicado**, los recursos quedan vivos en el
estado y en GCP mientras el archivo que los declara existe solo en tu portátil.
Entonces el problema es de todos: cualquiera que aplique después —incluso desde
`main`— propondrá destruirlos, y si tu copia se pierde nadie puede volver a
declararlos. Es el estado más incómodo posible, porque la infraestructura real va
por delante del repositorio.

La secuencia, entonces:

```
rama → plan (para revisar tu parte) → PR → merge a main → git pull en main → apply
```

El `plan` desde la rama sirve y conviene: valida tu configuración y te dice qué
vas a crear. **Lo que no se hace desde la rama es el `apply`.**

---

## Reconstruir el entorno

```bash
terraform destroy
terraform apply
```

Esto no es un ejercicio teórico: el enunciado **exige eliminar la instancia de
base de datos administrada al cerrar la entrega** y poder recrear el entorno
para la sustentación (issue I6). Que `destroy` seguido de `apply` funcione es un
criterio de aceptación de este issue, no una curiosidad.

Ojo con lo que `destroy` **no** borra: el bucket del estado, que se creó fuera de
Terraform. Es intencionado — si lo borrara, se llevaría por delante el registro
de lo que hay que reconstruir.

### Y con la base de datos, `destroy` a secas no basta

Desde C1 hay dos cosas en el camino, las dos a propósito:

**1. La instancia está protegida contra borrado.** `terraform destroy` falla con
`cannot destroy instance because deletion_protection is set to true`, y eso es lo
que se quiere el otro 99 % del tiempo: tres personas aplican sobre el mismo
estado y perder la base no se deshace. Para borrarla de verdad hay que decirlo
en el código primero:

```bash
# En database.tf: deletion_protection = false
terraform apply      # quita la protección, no borra nada todavía
terraform destroy    # ahora sí
```

**2. El nombre queda reservado una semana.** Google retiene el nombre de una
instancia borrada durante siete días, así que el `apply` siguiente falla con un
conflicto de nombre que no se puede forzar. La salida está prevista en
`database.tf`: subir `db_instance_generation`.

```bash
terraform apply -var='db_instance_generation=2'
```

Lo importante: **recrear la instancia da una base vacía.** Terraform crea la
base y el usuario, no el esquema. Después de cualquier recreación hay que
volver a migrar (ver abajo).

---

## Las migraciones no las hace Terraform

La instancia nace con `moocdb` creada y **sin una sola tabla**. En local el
esquema lo aplica el hook de inicialización de Postgres
(`scripts/init-db.sh`, montado en `/docker-entrypoint-initdb.d`), que corre una
vez cuando el volumen está vacío. **Cloud SQL no tiene ese hook.**

De eso se encarga `scripts/migrate.sh`, que hay que ejecutar **desde dentro de la
VPC** — la instancia no tiene IP pública, así que desde un portátil no hay a
dónde conectarse. Es decir, desde una de las VMs:

```bash
# En la VM, una vez:
sudo apt-get install -y postgresql-client

export DATABASE_URL="postgres://moocuser:$(gcloud secrets versions access latest \
  --secret=db-password)@$(terraform output -raw db_private_ip):5432/moocdb?sslmode=require"

bash ./scripts/migrate.sh            # aplica lo que falte
bash ./scripts/migrate.sh --verify   # comprueba el esquema resultante
```

Es repetible: una tabla `schema_migrations` registra lo aplicado, así que
ejecutarlo dos veces no hace nada la segunda, y tras añadir una migración nueva
solo aplica esa. Cada migración va **en una sola transacción junto con su
registro**, de modo que una que falla no deja ni esquema a medias ni constancia
de haberse aplicado.

Contra tu base de desarrollo local usa `--baseline` la primera vez: ahí el
esquema ya existe pero sin tabla de control, y sin eso el script intentaría
aplicar `000001` y fallaría.

### Sin VM todavía: el host temporal

Mientras D2 no exista hay un camino que no obliga a esperarlo: una VM mínima en
`mooc-subnet`, que se borra al terminar. Cuesta céntimos y **no necesita IP
pública** — se entra por IAP, que es para lo que B3 creó
`mooc-allow-ssh-iap`.

```bash
cd infra/terraform
IP="$(terraform output -raw db_private_ip)"
SA="$(terraform output -raw web_server_service_account)"
cd ../..

# 1. La VM. Sin dirección externa; la salida a internet la da el Cloud NAT de B3.
#    Lleva adjunta la cuenta de la API a propósito: así el acceso al secreto se
#    prueba por el mismo camino que usará el despliegue real, en lugar de pegar
#    la contraseña a mano.
gcloud compute instances create mooc-migrador-temporal \
  --project=plataforma-mooc-entrega2 --zone=us-east1-b \
  --machine-type=e2-micro --subnet=mooc-subnet --no-address \
  --tags=allow-iap-ssh \
  --service-account="${SA}" \
  --scopes=https://www.googleapis.com/auth/cloud-platform \
  --image-family=debian-12 --image-project=debian-cloud

# 2. Copiar solo lo que hace falta. Nada de clonar el repositorio: haría falta
#    autenticarse, y el script y las migraciones son todo lo necesario.
gcloud compute ssh mooc-migrador-temporal --zone=us-east1-b --tunnel-through-iap \
  --command='mkdir -p ~/mooc/scripts'
gcloud compute scp --recurse --tunnel-through-iap --zone=us-east1-b \
  migrations mooc-migrador-temporal:~/mooc/
gcloud compute scp --tunnel-through-iap --zone=us-east1-b \
  scripts/migrate.sh mooc-migrador-temporal:~/mooc/scripts/

# 3. Dentro de la VM
gcloud compute ssh mooc-migrador-temporal --zone=us-east1-b --tunnel-through-iap
```

```bash
# --- ya dentro de la VM ---
sudo apt-get update -qq && sudo apt-get install -y -qq postgresql-client
cd ~/mooc

# La contraseña la lee la propia VM con su cuenta de servicio. No se escribe,
# no se pega y no queda en el historial del shell.
export DATABASE_URL="postgres://moocuser:$(gcloud secrets versions access latest \
  --secret=db-password)@<IP_PRIVADA>:5432/moocdb?sslmode=require"

bash ./scripts/migrate.sh
bash ./scripts/migrate.sh --verify
exit
```

```bash
# 4. El otro lado del criterio: desde fuera NO se llega. Debe agotar el tiempo
#    de espera, no rechazar la conexión -- no hay nada escuchando en internet.
#    Desde tu máquina:
nc -vz -w 5 "${IP}" 5432        # o Test-NetConnection en PowerShell

# 5. Borrarla. La VM era para esto y ya no hace falta.
gcloud compute instances delete mooc-migrador-temporal \
  --project=plataforma-mooc-entrega2 --zone=us-east1-b --quiet
```

Se crea con `gcloud` y no con Terraform a propósito, y es la única excepción a la
regla 2: es un recurso efímero de una tarea de operación, no parte de la
infraestructura de la entrega. Si estuviera en Terraform, el siguiente `apply` de
cualquiera la recrearía.

**Si el paso 3 falla con un error de IAP,** falta el rol
`roles/iap.tunnelResourceAccessor` sobre el proyecto. Con Owner se tiene; si no,
hay que concederlo.

---

## Dónde está cada cosa

| Archivo | |
| :--- | :--- |
| `versions.tf` | Versiones fijadas y backend del estado remoto |
| `variables.tf` | Proyecto, región, zona y la contraseña de la base |
| `main.tf` | Proveedor y APIs habilitadas |
| `service_accounts.tf` | Las dos cuentas de servicio y su IAM base |
| `network.tf` | VPC, subred, rango privado reservado, NAT y firewall |
| `database.tf` | Cloud SQL: instancia privada, base y usuario |
| `storage.tf` | Bucket, prefijos, IAM por componente y CORS |
| `mail.tf` | Secreto del proveedor SMTP y permiso de lectura de la API |
| `outputs.tf` | Lo que consumen los issues siguientes |
| `ADMINISTRACION.md` | Tareas de una sola vez: bootstrap del estado, accesos del equipo y cierre de la entrega |

---

## Las cuentas de servicio

Una por componente, no una compartida, para que un fallo en uno no herede los
permisos del otro.

| | Web Server | Worker Server |
| :--- | :--- | :--- |
| Conectarse a Cloud SQL | ✅ | ✅ |
| Escribir logs y métricas | ✅ | ✅ |
| **Firmar URLs** | ✅ | ❌ |
| **Escribir derivados en el bucket (`hls/`)** | ❌ *(denegado por IAM)* | ✅ *(autorizado por IAM)* |
| **Escribir originales, docs y miniaturas** | ✅ *(autorizado por IAM)* | ❌ *(denegado por IAM)* |

El permiso de firma merece una nota. `internal/storage/gcs.go` firma con las
credenciales por defecto y **sin archivo de llave**; desde una VM no hay clave
privada disponible, así que la librería delega la firma en `signBlob` de IAM
Credentials. Eso exige `roles/iam.serviceAccountTokenCreator`, y se concede
**sobre la propia cuenta**, no sobre el proyecto: la API puede firmar como ella
misma y como nadie más. A nivel de proyecto podría suplantar también a la del
worker, que es justo la separación que estas dos cuentas existen para mantener.

Los permisos sobre el bucket se gestionan en `storage.tf` mediante condiciones
CEL que aseguran que el worker solo pueda escribir en `hls/` y la API solo en
originales, documentos y miniaturas.
