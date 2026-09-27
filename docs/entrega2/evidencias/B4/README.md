# Evidencia B4 — Gestión de secretos y configuración externa (issue #117)

| Archivo | Criterio que demuestra |
| :--- | :--- |
| [`historial_sin_credenciales.txt`](./historial_sin_credenciales.txt) | Historial de git sin credenciales · `.env.example` completo y sin valores |
| [`imagen_sin_secretos.txt`](./imagen_sin_secretos.txt) | `docker history` sin secretos |
| [`arranque_rechaza_desarrollo.txt`](./arranque_rechaza_desarrollo.txt) | La aplicación arranca solo con configuración inyectada |

El inventario completo de configuración sensible y los procedimientos de
rotación están en
[`infra/terraform/ADMINISTRACION.md`](../../../../infra/terraform/ADMINISTRACION.md).

## Los tres criterios

**1. Historial de git sin credenciales.** `.env`, las llaves de cuenta de
servicio y el estado de Terraform **nunca fueron versionados**: cero
coincidencias en todo el historial. Lo único que se versiona es `.env.example`,
y sus siete variables sensibles van vacías.

**2. `docker history` sin secretos.** Cero coincidencias de
`password|secret|minioadmin|moocpassword` en las capas, y la imagen final
contiene **un solo archivo**: el binario. Ni `.env`, ni llaves, ni `.git`.

Eso ya era cierto antes de este issue por el diseño multietapa, pero lo era por
suerte: el `COPY . .` de la etapa constructora metía el repositorio entero en el
contexto de construcción. Ahora hay un **`.dockerignore`** que lo impide desde
antes, en lugar de confiar en que la etapa final no lo copie.

**3. La aplicación arranca solo con configuración inyectada.** Este criterio solo
significa algo si arrancar sin ella falla, así que ahora falla:

```
{"level":"ERROR","msg":"api server terminated","error":"la configuracion de
produccion conserva valores de desarrollo:
  - DATABASE_URL conserva la contrasena de desarrollo
  - STORAGE_BACKEND=minio en produccion
  - S3_SECRET_KEY conserva el valor de desarrollo
  - APP_BASE_URL apunta a localhost
  - SMTP_HOST apunta a mailpit
  - SMTP_PORT=25: Google bloquea ese puerto saliente"}
```

Código de salida 1. Los seis problemas en un solo intento, porque quien
despliega prefiere una lista que corregir a seis arranques fallidos.

Las comprobaciones viven en `internal/config/validate.go` y solo se aplican con
`APP_ENV=production`. Es una lista blanca al revés —se exige en producción, no
se relaja en desarrollo— para que quien clona el repositorio pueda levantar
Docker Compose sin configurar nada.

## Lo que más vale del inventario

De las cinco clases de credencial del sistema, **tres dejaron de existir en la
nube**:

| | En la nube |
| :--- | :--- |
| Credenciales de almacenamiento (`S3_*`, MinIO) | **No existen** — la VM usa su cuenta de servicio adjunta |
| Credenciales de Terraform | **No existen** — cada integrante usa su identidad personal |
| Credenciales de las VMs | **No existen** — cuenta adjunta, sin archivo de llave |
| Contraseña de la base | Secret Manager |
| Credenciales de SMTP | Secret Manager (issue #126) |

Un secreto que no existe no se filtra, no caduca y no hay que rotarlo. Se
consiguió eligiendo identidades adjuntas en lugar de archivos de llave.

## Por qué `docker-compose.yml` sí lleva contraseñas

`moocpassword` y `minioadmin` siguen en el repositorio, como valores por defecto
del Compose, y es deliberado: **no protegen nada**. Son las credenciales de unos
contenedores que solo existen en la máquina de quien los levanta, y tenerlas ahí
es lo que permite clonar y arrancar sin configurar nada — verificado: con
`.env.example` como `.env`, Compose resuelve sus valores por defecto y el stack
levanta igual.

Lo que las hace inofensivas no es una promesa, es el criterio 3: si alguna
sobrevive a un despliegue, el proceso no arranca.

## Nunca

Estos archivos son salidas de verificación y no contienen ninguna credencial.
Las búsquedas que aparecen en ellos son de *patrones*, no de valores.
