# Runbook — I6: destruir y reconstruir el entorno en una sola sesión ([#236](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/236))

**Esta sesión es irrepetible y es la última de la entrega.** La captura de la
destrucción y de la recreación es lo único que falta para que
`scripts/check_evidencias.sh` salga con 0, y sin ese 0 no se puede crear el tag
de I5. Si algo se graba mal, rehacerlo cuesta otra sesión y otro dinero.

| | |
| :--- | :--- |
| Duración | ~2 h 30, de las que **hasta 1 h es esperar al certificado** |
| Coste | ~0,25 USD/h con el entorno arriba. Una sesión de 3 h, **menos de 1 USD** |
| Quién | el procedimiento lo ejecuta **alguien que no lo escribió** — es criterio de aceptación de #236 |
| Antes de empezar | leer `infra/terraform/ADMINISTRACION.md` §«Cierre de la entrega» y la nota de `infra/terraform/database.tf:103` |

> **El orden correcto es I6 y después I5**, aunque el cuerpo de #236 diga
> «Depende de: I5». El enunciado (p. 7) solo ordena «conservar las evidencias y
> respaldos **necesarios**» antes de eliminar, y pide (p. 13) que el tag
> registre «el commit evaluado»: si I6 produce commits después del tag, el tag
> deja de ser el commit final. Además I5 exige `check_evidencias.sh` en 0, y ese
> script falla hoy exactamente por I6.

---

## 0. Las cuatro trampas, antes de tocar nada

### `deletion_protection` se lee del **estado**, no del fichero

Ponerlo a `false` en `database.tf` no basta. Hace falta un `apply` que lo
persista **antes** del `destroy`, o el destroy falla con
`failed to delete instance because deletion_protection is set to true` aunque el
fichero diga `false`. Está documentado en `database.tf:103-114`.

Hoy: el fichero dice `true`; el flag de GCP `deletionProtectionEnabled` dice
`False`, **que es otro distinto y no ayuda**. El que bloquea es el de Terraform.

### El nombre de la base queda reservado **siete días**

Un `apply` posterior falla con un conflicto de nombre que no se puede forzar. La
salida está prevista: subir `db_instance_generation` en `database.tf`, o pasarlo
en línea de comandos.

### Las copias de seguridad se van con la instancia

Si hace falta conservar datos, **exportar antes**. Después no hay vuelta.

### Si se destruye la IP global, **cambia la URL de toda la entrega**

`136.68.55.254.sslip.io` está citada en el `README.md`, en `arquitectura.md`, en
`VERIFICACION_ENDPOINTS.md`, en el entorno de Postman y en el vídeo ya grabado.
Un `terraform destroy` completo se lleva la dirección reservada y el certificado,
y al recrear **salen otros**: habría que corregir todos esos documentos y el
vídeo ya no se podría corregir.

**Decisión recomendada:** destruir todo **menos** `mooc-lb-ip` y `mooc-lb-cert`.
Cuestan ~0 USD aparcados, mantienen la URL del vídeo válida y evitan la hora de
aprovisionamiento del certificado en la sustentación. La recreación sigue
cubriendo lo que pide #236 —grupos elásticos, balanceador, cola y frontend—.
Si el equipo prefiere el destroy total, hay que asumir el cambio de URL y
documentarlo; esa decisión **se anota en el README de esta evidencia**.

---

## 1. Pre-vuelo · *tu máquina* · sin coste

### 1.1 Publicar las imágenes del commit final — **no es opcional**

Comprobado el 2026-11-02: **no hay imágenes publicadas para `HEAD`**, y el
entorno que está corriendo usa `97120ca1`, que lleva ~70 commits de retraso e
**incluye cambios de código**, no solo documentación: `internal/auth` y 10
ficheros del frontend, que son las correcciones de H2.

Si la recreación del §6 se lanza sin publicar, las instancias nuevas intentan
descargar una etiqueta que no existe y el grupo nunca llega a sano.

```bash
cd "<raíz del repo>"
git checkout main && git pull
git status --short                       # publish_images.sh exige árbol limpio

bash ./scripts/publish_images.sh         # api, worker y web con el SHA de HEAD
```

Y se comprueba que las tres están arriba antes de seguir:

```bash
HEAD_SHA="$(git rev-parse HEAD)"
for img in api worker web; do
  gcloud artifacts docker tags list \
    "us-east1-docker.pkg.dev/plataforma-mooc-entrega2/mooc/$img" \
    --project=plataforma-mooc-entrega2 --format="value(tag)" \
    | grep -qx "$HEAD_SHA" && echo "OK    $img" || echo "FALTA $img"
done
```

> **Las tres tienen que decir `OK`.** Si alguna falta, no sigas: el §6 fallará
> cuando ya no haya entorno al que volver.

### 1.2 Variables de la sesión

```bash
export IMAGE_TAG="$(git rev-parse HEAD)"
export TF_VAR_db_password='<de Secret Manager, no inventado>'
echo "$IMAGE_TAG"
```

### 1.3 Estado de partida

Comprueba que el entorno está arriba y anota el punto de partida:

```bash
gcloud compute instances list --project=plataforma-mooc-entrega2
gcloud compute forwarding-rules list --global --project=plataforma-mooc-entrega2
gcloud sql instances list --project=plataforma-mooc-entrega2
```

> 📸 **Captura 1** → `capturas/AAAA-MM-DD_estado-inicial.png`
> Las tres salidas juntas: el entorno completo antes de tocarlo.

---

## 2. Conservar · *tu máquina* · **antes de destruir nada**

El enunciado lo pone como condición previa: «después de conservar las evidencias
y respaldos necesarios».

```bash
cd infra/terraform
BUCKET="$(terraform output -raw storage_bucket_name)"
DB="$(terraform output -raw db_instance_name)"
cd ../..

# Respaldo lógico de la base, al bucket que sobrevive al destroy
gcloud sql export sql "$DB" "gs://${BUCKET}/respaldo-final-$(date +%Y%m%d).sql" \
  --database=moocdb --project=plataforma-mooc-entrega2

# Copia del estado de Terraform, fuera del repositorio
gcloud storage cp \
  gs://plataforma-mooc-entrega2-tfstate/entrega2/default.tfstate \
  ./scratch/estado-antes-de-i6.tfstate
```

> **El prefijo del estado es `entrega2`, no `entrega3`**: la Entrega 3 reusa el
> mismo proyecto y el mismo estado. Está en `infra/terraform/versions.tf:29-32`,
> comprobado contra el bucket.

> 📸 **Captura 2** → `capturas/AAAA-MM-DD_respaldo-conservado.png`
> El `export` terminado y el objeto listado en el bucket.

---

## 3. Quitar la protección · *tu máquina* · **no borra nada**

En `infra/terraform/database.tf`, línea 115:

```diff
-  deletion_protection = true
+  deletion_protection = false
```

```bash
cd infra/terraform
terraform plan -out=soltar.tfplan -var "image_tag=${IMAGE_TAG}"
```

**El plan debe decir `1 to change` y `0 to destroy`.** Si propone destruir algo,
para: alguien aplicó desde una rama.

```bash
terraform apply soltar.tfplan
```

Este `apply` es lo que persiste el `false` en el estado. Sin él, el paso 4 falla.

---

## 4. Destruir · *tu máquina* · **irreversible**

Con la decisión recomendada del §0, la dirección y el certificado se conservan:

```bash
terraform destroy \
  -var "image_tag=${IMAGE_TAG}" \
  -target=google_compute_instance_group_manager.api \
  -target=google_compute_instance_group_manager.worker \
  -target=google_compute_instance.queue_server \
  -target=google_sql_database_instance.main
```

> Confirma los nombres de los recursos con `terraform state list` antes de
> lanzarlo: si un `-target` no existe, Terraform avisa y no destruye nada.

Para el **destroy total** (si el equipo lo decide), es `terraform destroy` a
secas, y entonces también se van `mooc-lb-ip` y `mooc-lb-cert`.

> 📸 **Captura 3** → `capturas/AAAA-MM-DD_destruccion-en-curso.png`
> La salida del `destroy` con el recuento de recursos destruidos.

Y la comprobación de que no queda nada vivo:

```bash
gcloud compute instances list --project=plataforma-mooc-entrega2
gcloud sql instances list --project=plataforma-mooc-entrega2
```

> 📸 **Captura 4** → `capturas/AAAA-MM-DD_entorno-destruido.png`
> Las dos listas vacías. **Esta es la mitad de la captura que exige #236.**

---

## 5. Medir el coste residual · *tu máquina*

Es criterio de aceptación: «coste residual medido y comparado con el coste del
entorno completo».

```bash
gcloud storage ls --long gs://plataforma-mooc-entrega2-media
gcloud storage ls --long gs://plataforma-mooc-entrega2-hls
gcloud artifacts repositories describe mooc \
  --location=us-east1 --project=plataforma-mooc-entrega2 \
  --format="value(name,sizeBytes)"
gcloud compute addresses list --global --project=plataforma-mooc-entrega2
```

La referencia con la que comparar está en `F3/README.md` §4: el entorno completo
son **0,2311 USD/h**, unos 168,69 al mes.

> 📸 **Captura 5** → `capturas/AAAA-MM-DD_coste-residual.png`

---

## 6. Reconstruir · *tu máquina* · **empieza el gasto otra vez**

Si la base se recreó dentro de los siete días del nombre reservado, añade
`-var 'db_instance_generation=2'`.

```bash
terraform plan -out=recrear.tfplan \
  -var "image_tag=${IMAGE_TAG}" \
  -var db_activation_policy=ALWAYS \
  -var lb_enabled=true \
  -var api_autoscaling_enabled=true \
  -var worker_autoscaling_enabled=true

terraform apply recrear.tfplan
```

> Los cinco interruptores van juntos. Si falta `lb_enabled=true`, el entorno
> queda sin reglas de reenvío y `terraform output -raw lb_url` devuelve vacío.

La base viene **vacía**: Terraform nunca crea el esquema. Hay que migrar y
sembrar desde dentro de la VPC, con `scripts/recrear-entorno.sh` — sus tres
fases y por qué no se pueden juntar están en la cabecera del propio guion.

```bash
cd ../.. && bash ./scripts/recrear-entorno.sh
```

> 📸 **Captura 6** → `capturas/AAAA-MM-DD_recreacion-completa.png`
> El `apply` terminado y las instancias de los dos grupos en `RUNNING`.

---

## 7. Verificar sobre el entorno recreado · *tu máquina*

Es criterio de aceptación: «la verificación funcional pasa sobre el entorno
recreado, con su salida registrada».

```bash
cd infra/terraform && BASE_URL="$(terraform output -raw lb_url)" && cd ../..
echo "$BASE_URL"

BASE_URL="$BASE_URL" bash ./scripts/verificar_endpoints_nube.sh | tee \
  docs/entrega3/evidencias/I6/verificacion_entorno_recreado.txt

bash ./scripts/test_postman_cloud.sh
```

> Los dos no se lanzan pegados: el limitador de login son **10 intentos por
> minuto y por IP**.

> 📸 **Captura 7** → `capturas/AAAA-MM-DD_verificacion-sobre-entorno-recreado.png`
> **Es la otra mitad de la captura que exige #236.** Sin ella el issue no cierra.

---

## 8. Dejar la protección en su valor seguro · *tu máquina*

Criterio de aceptación de #236. Revertir el cambio del §3:

```diff
-  deletion_protection = false
+  deletion_protection = true
```

```bash
cd infra/terraform
terraform apply -var "image_tag=${IMAGE_TAG}"    # 1 to change, 0 to destroy
```

---

## 9. Aparcar, y solo entonces ir a I5

```bash
terraform apply -var "image_tag=${IMAGE_TAG}"    # los interruptores a false

for G in mooc-api-group mooc-worker-group; do
  gcloud compute instance-groups managed resize "$G" --size=0 \
    --zone=us-east1-b --project=plataforma-mooc-entrega2 --quiet
done

gcloud sql instances patch "$(terraform output -raw db_instance_name)" \
  --activation-policy=NEVER --project=plataforma-mooc-entrega2
```

Y la cola, que no pertenece a ningún grupo y un `resize` no la toca:

```bash
gcloud compute instances stop mooc-queue-server \
  --zone=us-east1-b --project=plataforma-mooc-entrega2
```

---

## Cierre de la sesión

- [ ] Las **siete capturas** en `capturas/`, con nombre `AAAA-MM-DD_descripcion.png`
- [ ] `verificacion_entorno_recreado.txt` guardado
- [ ] `README.md` de esta evidencia relleno, con el coste residual medido y la
      decisión sobre la IP global
- [ ] Anotar en `docs/entrega3/NOTAS_TECNICAS.md` lo que esta sesión descubra; si
      no hubo nada, decirlo explícitamente al cerrar
- [ ] `bash ./scripts/check_evidencias.sh` → **debe salir con 0**
- [ ] Commit y merge a `main`
- [ ] **Y entonces I5**: el tag sobre ese commit

---

Este archivo no contiene credenciales, llaves ni secretos.
