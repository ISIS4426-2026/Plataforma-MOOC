# Evidencia I4 — README, tag `entrega-2` y verificación del repositorio (issue #139)

| Tarea | |
| :--- | :---: |
| README con documentación, instrucciones, URL de la aplicación y evidencias | ✅ |
| Enlazar `docs/entrega2/` y `capacity-planning/` | ✅ |
| Sin credenciales en ningún archivo ni en el historial | ✅ · ver §1 |
| Tag `entrega-2` con el commit registrado | ⏳ se crea al cerrar · §4 |
| Un clon desde cero permite reconstruir el entorno | ✅ · ver §3 |

---

## 1. Verificación de credenciales

### En los archivos y en todo el historial

Se buscaron los patrones que delatan un secreto real —llaves privadas, claves de
proveedor, tokens de Google, credenciales de cuenta de servicio— **en el árbol
rastreado y en todos los commits de todas las ramas**:

```bash
git grep -nIE "BEGIN (RSA|EC|OPENSSH|PRIVATE) |xsmtpsib-|AKIA[0-9A-Z]{16}|ya29\.|AIza[0-9A-Za-z_-]{35}|\"private_key\""
git log --all -p | grep -aE "BEGIN .* KEY|xsmtpsib-|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_-]{35}|\"private_key\":"
```

**Cero coincidencias en ambos casos.**

### Archivos que nunca llegaron a rastrearse

```bash
git log --all --pretty=format: --name-only | sort -u | grep -E '(^|/)\.env'
# → .env.example   (y nada más)
```

Ningún `.env` real, ninguna llave `.pem`/`.key`, ningún `terraform.tfstate` y
ningún JSON de cuenta de servicio aparecen en la historia. `.gitignore` los
excluye explícitamente (líneas 20-22, 48-49, 63-65).

`.env.example` es el ejemplo sin valores que pide el enunciado: las variables de
contraseña están presentes y **vacías**.

### Lo que sí está publicado, y por qué

Las cuentas sembradas usan la contraseña `Password123!` y la base local
`moocpassword`. **Es deliberado y necesario:** el enunciado exige datos
sintéticos y scripts reproducibles, y el equipo docente necesita esas
credenciales para recorrer la plataforma. Están en `docs/DATOS_SINTETICOS.md`,
en los entornos de Postman y en la semilla.

Conviene decirlo explícito para que no se lea como un descuido: **esas cuentas
existen también en la base administrada**, porque la semilla se cargó allí. No
dan acceso a la infraestructura —ni a GCP, ni a la base, ni al bucket—, solo a la
aplicación con los roles sembrados. Si tras la sustentación el entorno se
conservara, lo prudente es rotarlas o eliminar la semilla.

Los secretos de verdad —contraseña de la base y clave del proveedor SMTP— viven
en Secret Manager y los leen las VMs con su propia identidad; nunca pasan por el
repositorio ni por el estado de Terraform.

## 2. Enlaces del README y del repositorio

Todos los enlaces relativos del repositorio resuelven: **cero rotos**,
comprobados con un recorrido de los `.md` de `docs/`, `capacity-planning/`,
`infra/` y la raíz.

**No lo estaban al empezar.** La verificación encontró y corrigió:

| Dónde | Qué pasaba |
| :--- | :--- |
| `evidencias/I5/GUION_VIDEO.md` | **55 enlaces absolutos** del tipo `file:///mnt/c/Users/User/Desktop/…`, rutas del portátil de quien lo escribió. Rotas para todos los demás |
| `evidencias/I5/README.md` | La referencia al issue #139 apuntaba a una ruta relativa del disco (`../../../../issues/139`) en lugar de a la URL de GitHub |
| `README.md` | No enlazaba el entregable de I2, `OPERACION_Y_CAPACIDAD.md`, ni desde la tabla de la Entrega 2 ni desde el índice ni desde el árbol del monorepo |

Es la segunda vez que aparecen rutas `file:///mnt/c/…` en el repositorio —ya se
limpiaron seis en `GUIA_DE_DESPLIEGUE.md` y cinco en
`REPORTE_BUGS_Y_CALIDAD_ETAPA.md`—, así que conviene revisarlo antes de crear el
tag: un editor que inserta enlaces por ruta absoluta los reintroduce sin avisar.

El README enlaza lo que el enunciado pide, y lo hace desde la tabla de la
**Entrega 2**, arriba del todo: la URL de la aplicación, la documentación de la
API, cómo desplegar y operar, los procedimientos de las VMs, el documento de
arquitectura, el informe de capacidad, las evidencias y los costos.

## 3. Reconstrucción desde un clon limpio

```
git clone <repo> /c/tmp-mooc
git checkout main            # commit e47925c
git status --porcelain       # 0 archivos con problemas
go build ./...               # sin errores
go test ./internal/config/ ./internal/domain/ ./internal/transcode/   # ok, ok, ok
```

Todos los archivos que el despliegue necesita están presentes en el clon:
`.env.example`, los tres Compose, `nginx.conf`, el `Makefile`, la configuración
de Terraform y la del Ops Agent, los cuatro scripts de arranque y publicación,
las migraciones y la semilla.

### Una salvedad de Windows que conviene anotar en el README

Un primer intento de clon **falló** con `Filename too long`. No es un defecto del
repositorio: la ruta rastreada más larga son **100 caracteres**, y el fallo
apareció solo al clonar dentro de un directorio de pruebas muy profundo, donde la
suma superaba el límite de 260 caracteres de Windows. Clonando en una ruta corta
el resultado fue **0 archivos con problemas**.

La salida para quien clone en una ruta profunda:

```bash
git config --global core.longpaths true
```

## 4. Tag `entrega-2`

Se crea al cerrar la entrega, sobre el commit de `main` que se evalúa, y **después**
de que I5 grabe el video —el README debe enlazarlo antes de congelar.

```bash
git checkout main && git pull --ff-only
git tag -a entrega-2 -m "Entrega 2 — Despliegue basico en la nube"
git push origin entrega-2
git rev-list -n 1 entrega-2  # commit evaluado; registrar este SHA aquí
```

| | |
| :--- | :--- |
| Commit evaluado | *(pendiente)* |
| Fecha del tag | *(pendiente)* |

---

Este archivo no contiene credenciales, llaves ni secretos.
