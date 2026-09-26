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
| *(C1)* Cloud SQL · *(C3)* bucket · *(D2, E1)* VMs | |

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

```bash
export TF_VAR_db_password='...'   # la misma para todo el equipo; pedírsela a quien administra el proyecto
```

Va por entorno y nunca por el repositorio. En Windows con PowerShell:
`$env:TF_VAR_db_password = '...'`.

> **Tiene que ser exactamente la misma para los cuatro.** No es una
> recomendación de orden: si cada uno exporta una distinta, a partir de C1 cada
> `apply` cambiará la contraseña de la base por la de quien lo ejecutó y
> **romperá las conexiones de la aplicación**. El vaivén sería difícil de
> diagnosticar, porque el código es idéntico para todos y lo que difiere es el
> entorno. Se comparte por un gestor de contraseñas; la reparte quien administra
> el proyecto.

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

### Las cinco reglas

1. **Nunca editar el estado a mano** ni descargarlo al repositorio. Contiene la
   contraseña de la base en claro.
2. **Nunca crear recursos desde la consola web.** Terraform no los conocería, y
   el siguiente `plan` intentaría crearlos otra vez o los ignoraría hasta
   chocar. Si ya ocurrió: `terraform import`, o borrarlo y dejar que Terraform
   lo cree.
3. **Un `apply` a la vez.** El bloqueo lo impone, pero avisar en el chat evita
   la espera.
4. **`git pull` antes de planificar.** Siempre.
5. **La misma `TF_VAR_db_password` para los cuatro.** Si un `plan` propone
   cambiar la contraseña de la base sin que nadie haya tocado el código, es esto:
   alguien tiene exportada otra. No apliques — pregunta en el chat del equipo.

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

---

## Dónde está cada cosa

| Archivo | |
| :--- | :--- |
| `versions.tf` | Versiones fijadas y backend del estado remoto |
| `variables.tf` | Proyecto, región, zona y la contraseña de la base |
| `main.tf` | Proveedor y APIs habilitadas |
| `service_accounts.tf` | Las dos cuentas de servicio y su IAM diferenciado |
| `outputs.tf` | Lo que consumen los issues siguientes |
| `ADMINISTRACION.md` | Tareas de una sola vez: bootstrap del estado y accesos del equipo |

---

## Las cuentas de servicio

Una por componente, no una compartida, para que un fallo en uno no herede los
permisos del otro.

| | Web Server | Worker Server |
| :--- | :--- | :--- |
| Conectarse a Cloud SQL | ✅ | ✅ |
| Escribir logs y métricas | ✅ | ✅ |
| **Firmar URLs** | ✅ | ❌ |
| **Escribir derivados en el bucket** | ❌ *(C3)* | ✅ *(C3)* |

El permiso de firma merece una nota. `internal/storage/gcs.go` firma con las
credenciales por defecto y **sin archivo de llave**; desde una VM no hay clave
privada disponible, así que la librería delega la firma en `signBlob` de IAM
Credentials. Eso exige `roles/iam.serviceAccountTokenCreator`, y se concede
**sobre la propia cuenta**, no sobre el proyecto: la API puede firmar como ella
misma y como nadie más. A nivel de proyecto podría suplantar también a la del
worker, que es justo la separación que estas dos cuentas existen para mantener.

Los permisos sobre el bucket se conceden en **C3**, cuando el bucket exista.
