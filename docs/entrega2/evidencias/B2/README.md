# Evidencia B2 — Terraform: estructura, estado remoto e IAM (issue #115)

La configuración está en [`../../../../infra/terraform/`](../../../../infra/terraform/)
y su manual de uso para el equipo en
[`infra/terraform/README.md`](../../../../infra/terraform/README.md).

## Lo ya verificado, sin tocar la nube

| Comprobación | Resultado |
| :--- | :--- |
| `terraform fmt -check` | ✅ sin diferencias |
| `terraform init -backend=false` | ✅ provider `hashicorp/google 8.4.0` descargado |
| `terraform validate` | ✅ *Success! The configuration is valid* |
| Lock multiplataforma | ✅ `windows_amd64`, `linux_amd64`, `darwin_amd64`, `darwin_arm64` |
| CI (`.github/workflows/ci.yml`) | ✅ trabajo `terraform` que repite fmt + validate en cada push |
| Nada sensible versionado | ✅ estado, `.terraform/` y `terraform.tfvars` en `.gitignore` |

Ejecutado con Terraform 1.16.4 en contenedor, sin instalar nada y sin
credenciales: `validate` comprueba la sintaxis y el esquema real del proveedor,
que es cuanto se puede verificar sin crear recursos.

## Ejecutado contra GCP el 2026-09-26

Los tres criterios de aceptación del issue, verificados sobre el proyecto
`plataforma-mooc-entrega2`:

| Criterio | Resultado |
| :--- | :--- |
| `apply` sobre un proyecto limpio levanta la infraestructura sin pasos manuales | ✅ `Apply complete! Resources: 15 added, 0 changed, 0 destroyed` |
| `destroy` seguido de `apply` reconstruye el entorno | ✅ `15 destroyed` y después `15 added` |
| Ningún secreto en el repositorio | ✅ estado en el bucket privado; nada sensible en estas evidencias |

| Archivo | Qué contiene |
| :--- | :--- |
| `init_backend_remoto.txt` | `Successfully configured the backend "gcs"!` y el provider 8.4.0 instalado desde el lock |
| `plan_proyecto_limpio.txt` | `Plan: 15 to add, 0 to change, 0 to destroy` |
| `apply_proyecto_limpio.txt` | El apply completo sobre el proyecto vacío |
| `iam_cuentas.txt` | Las políticas de IAM, del proyecto y de la cuenta del web server |
| `destroy.txt` | `Destroy complete! Resources: 15 destroyed` |
| `apply_reconstruccion.txt` | La reconstrucción posterior al destroy |
| `consola/cuentas_de_servicio.png` | Las dos cuentas en la consola, ambas con **«No keys»** |
| `consola/iam_proyecto.png` | La política de IAM del proyecto, con los roles de las cuentas y los del equipo |
| `consola/bucket_estado.png` | El bucket del estado, región `us-east1` y **Not public** |

Los 15 recursos son 6 APIs habilitadas, 2 cuentas de servicio, 6 asignaciones de
rol a nivel de proyecto y 1 sobre la propia cuenta.

### El IAM diferenciado, probado por los dos lados

En la política **del proyecto**, las dos cuentas comparten exactamente tres
roles:

```
roles/cloudsql.client          sa-web-server / sa-worker-server
roles/logging.logWriter        sa-web-server / sa-worker-server
roles/monitoring.metricWriter  sa-web-server / sa-worker-server
```

`serviceAccountTokenCreator` **no aparece ahí**, y es lo importante: a nivel de
proyecto, la API podría suplantar también a la cuenta del worker. Aparece
únicamente en la política de su propia cuenta:

```
bindings:
- members:
  - serviceAccount:sa-web-server@plataforma-mooc-entrega2.iam.gserviceaccount.com
  role: roles/iam.serviceAccountTokenCreator
```

Y la prueba simétrica, la del worker, es una ausencia:

```
etag: ACAB
```

Sin `bindings`. El worker no tiene con qué suplantarse, así que no puede emitir
URLs firmadas ni aunque el código se lo pidiera. Que la API firme y el worker no
deja de ser una convención del código para ser algo que IAM impone.

### Nota sobre el destroy

Las APIs figuran como destruidas pero **siguen habilitadas** en GCP: es
deliberado, `disable_on_destroy = false`. Terraform las saca de su estado sin
apagarlas, porque desactivarlas es lento, afecta a lo que dependa de ellas y no
ahorra nada — habilitar una API no cuesta.

### Pendiente, opcional

`bloqueo_estado.png` — el error `Error acquiring the state lock` cuando dos
integrantes aplican a la vez. Es la captura que mejor demuestra el trabajo
concurrente sin conflictos, pero requiere a dos personas a la vez.

## Nunca

Llaves de cuentas de servicio, archivos `.env`, el estado de Terraform, ni la
contraseña de la base. **El estado contiene esa contraseña en claro**: vive en el
bucket privado y no se descarga aquí bajo ningún concepto.

Los identificadores de proyecto, de recursos y los correos de las cuentas de
servicio sí pueden aparecer: no son secretos.
