# Runbook — I6: destruir y reconstruir el entorno en una sola sesión ([#236](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/236))

> **Ejecutado el 2026-11-02.** Este runbook ya no es una propuesta: se corrió
> entero y las nueve cosas que falló están corregidas aquí dentro. El resultado
> y las correcciones, en [`README.md`](./README.md).

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

### 2a. Dar permiso a Cloud SQL sobre el bucket

El `export` **no** escribe con tus credenciales, sino con la cuenta de servicio
de la instancia, y esa cuenta no tiene ningún rol sobre el bucket. Sin esto el
export falla con permiso denegado.

```bash
SA="$(gcloud sql instances describe mooc-db-1 \
  --project=plataforma-mooc-entrega2 \
  --format='value(serviceAccountEmailAddress)')"

gcloud storage buckets add-iam-policy-binding gs://plataforma-mooc-entrega2-media \
  --member="serviceAccount:${SA}" \
  --role=roles/storage.objectCreator \
  --project=plataforma-mooc-entrega2
```

> **Preguntará por una condición**, porque el bucket tiene *bindings*
> condicionales de F2. **Responder `2` (None).** La opción 1 ofrece reutilizar
> `api_deny_derivatives_write`, que pertenece a `sa-web-server` y a otro
> propósito.

El permiso es temporal: muere con la instancia en el §4.

### 2b. El respaldo

```bash
gcloud sql export sql mooc-db-1 \
  "gs://plataforma-mooc-entrega2-media/respaldo-final-$(date +%Y%m%d).sql" \
  --database=moocdb --project=plataforma-mooc-entrega2

gcloud storage ls -l "gs://plataforma-mooc-entrega2-media/respaldo-final-*.sql"
```

**El tamaño tiene que ser mayor que cero.** Un export que «termina» y deja un
archivo vacío es el fallo silencioso que hace inútil el respaldo. En la sesión
del 2026-11-02 fueron 8,50 MiB.

### 2c. La copia del estado

```bash
gcloud storage cp \
  gs://plataforma-mooc-entrega2-tfstate/entrega2/default.tfstate \
  ./scratch/estado-antes-de-i6.tfstate
```

> **El prefijo del estado es `entrega2`, no `entrega3`**: la Entrega 3 reusa el
> mismo proyecto y el mismo estado (`infra/terraform/versions.tf:29-32`).
> `scratch/` está en `.gitignore`, así que la copia no se cuela en un commit.

> 📸 **Captura 2** → `capturas/AAAA-MM-DD_respaldo-conservado.png`
## 3. Quitar la protección · *tu máquina* · **no borra nada**

En `infra/terraform/database.tf`, línea 115:

```diff
-  deletion_protection = true
+  deletion_protection = false
```

```bash
cd infra/terraform
terraform plan -out=soltar.tfplan \
  -var "image_tag=${IMAGE_TAG}" \
  -var db_activation_policy=ALWAYS \
  -var lb_enabled=true \
  -var api_autoscaling_enabled=true \
  -var worker_autoscaling_enabled=true
```

> ⚠️ **Los cinco `-var` son obligatorios.** `lb_enabled`,
> `api_autoscaling_enabled` y `worker_autoscaling_enabled` valen `false` por
> omisión, así que un `plan` sin ellos no entiende «suelta el candado»:
> entiende «aparca el entorno», y propone `6 to destroy`. Y el `image_tag` es
> **el que está desplegado**, no el nuevo: con el nuevo reemplaza las dos
> plantillas justo antes de destruirlas.
>
> ```bash
> terraform state show google_compute_instance_template.api | grep mooc-image-tag
> ```

**El plan debe decir `1 to change, 0 to destroy`.** Cualquier otra cosa, para.

```bash
terraform apply soltar.tfplan
```

Este `apply` es el que persiste el `false` en el estado, y **el estado es lo que
el proveedor lee** (nota 67). Compruébalo, porque el fichero no sirve de prueba:

```bash
terraform state show google_sql_database_instance.main | grep -E "^\s+deletion_protection\b"
```

Tiene que decir `false`. Si dice `true`, el `apply` no llegó a correr y el §4
fallará después de haberse llevado 13 recursos por delante.
## 4. Destruir · *tu máquina* · **irreversible**

**No basta con apuntar a los cuatro recursos y lanzarlo.** GCP rechaza borrar un
grupo de instancias que un backend service está usando
(`resourceInUseByAnotherResource`), así que la cadena del balanceador tiene que
caer también — y #236 pide ensayar su reconstrucción, de modo que entra en el
ensayo de todas formas.

No hace falta alargar la lista: en un `destroy`, Terraform arrastra **todo lo que
depende** de lo que apuntas. Los cuatro objetivos tiran hacia arriba del backend
service, los url maps, los proxies y las reglas de reenvío, mientras que
`mooc-lb-ip` y `mooc-lb-cert` no dependen de nada de eso y sobreviven.

**Planifica y lee la lista antes de aplicar:**

```bash
terraform plan -destroy -out=destruir.tfplan \
  -var "image_tag=${IMAGE_TAG}" \
  -var db_activation_policy=ALWAYS \
  -var lb_enabled=true \
  -var api_autoscaling_enabled=true \
  -var worker_autoscaling_enabled=true \
  -target=google_compute_instance_group_manager.api \
  -target=google_compute_instance_group_manager.worker \
  -target=google_compute_instance.queue_server \
  -target=google_sql_database_instance.main
```

| Tiene que **aparecer** | Tiene que **NO aparecer** |
| :--- | :--- |
| los 2 MIG, sus 2 plantillas y los 2 autoescaladores | `google_compute_global_address.lb` |
| `backend_service.api`, 2 `url_map`, 2 proxies, la regla de reenvío HTTPS | `google_compute_managed_ssl_certificate.lb` |
| `instance.queue_server` | |
| `sql_database_instance.main` + `sql_database.mooc` + `sql_user.app` | |

Son **14**. Y en el bloque de la base tiene que leerse `deletion_protection =
false -> null`; si dice `true`, vuelve al §3.

```bash
terraform apply destruir.tfplan
```

> **La regla de redirección del 80 NO entra en los 14.** Cuelga de su propio url
> map y proxy, no de los grupos, así que sobrevive. No la borres a mano: el §6
> la reconcilia, y sin ella la dirección global pasa a `RESERVED` sin uso y
> empieza a facturar 0,24 USD/día.

> 📸 **Captura 3** → `capturas/AAAA-MM-DD_destruccion-en-curso.png`

### Si falla en `google_sql_user.app`

Síntoma: `role "moocuser" cannot be dropped because some objects depend on it`.
`database.tf` ya lleva `deletion_policy = "ABANDON"` en el usuario para que no
vuelva a pasar, **pero el proveedor lo lee del estado** (nota 67), así que en un
estado antiguo sigue mordiendo. Salida en caliente:

```bash
terraform state rm google_sql_user.app
```

No toca GCP: el usuario se va con la instancia, y el §6 lo recrea.

Y la comprobación de que no queda nada vivo:

```bash
gcloud compute instances list --project=plataforma-mooc-entrega2
gcloud sql instances list --project=plataforma-mooc-entrega2
```

> 📸 **Captura 4** → `capturas/AAAA-MM-DD_entorno-destruido.png`
> Las dos listas vacías, en la misma toma. **Es la mitad bloqueante de #236.**
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

Aquí entra la imagen **nueva**, la del commit que se entrega. Si la base se
recrea dentro de los siete días del nombre reservado, añade
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

Sin `-target` esta vez: queremos que Terraform reconcilie todo, incluida la
cadena del redirect HTTP que sobrevivió al §4. Son del orden de **14 recursos a
crear y ninguno a destruir**; si propone destruir algo, para.

Comprueba que los grupos levantaron y que el balanceador los da por sanos:

```bash
cd ../.. && gcloud compute instances list --project=plataforma-mooc-entrega2
gcloud compute backend-services get-health mooc-api-backend \
  --global --project=plataforma-mooc-entrega2
```

> 📸 **Captura 6** → `capturas/AAAA-MM-DD_recreacion-completa.png`

### La base viene vacía, y el esquema se aplica solo

Terraform crea la instancia, nunca el esquema. Pero **las migraciones no hay que
lanzarlas**: cada instancia las aplica al arrancar, con el *advisory lock* de A3.
Medido en la sesión del 2026-11-02, el guion de siembra encontró las **8
migraciones ya aplicadas**.

Lo que sí falta son los datos: `verificar_endpoints_nube.sh` **hace login** con
`admin@plataforma-mooc.test`, no lo registra, así que sin semilla el §7 falla.

```bash
bash ./scripts/recrear-entorno.sh --solo-base
```

> ⚠️ **`--solo-base` no es opcional.** Sin la bandera, el guion corre su fase 1,
> que es `terraform apply -auto-approve` **sin ninguna `-var`**: los tres
> interruptores caerían a `false` y desmontaría los autoescaladores y el
> balanceador que el paso anterior acaba de crear. Pide un `si` antes, pero es
> el mismo error del §3 con la red quitada.

Crea una VM temporal dentro de la VPC, migra, siembra y la borra. **120 s**
medidos. El guion debe informar **«Leída (32 caracteres)»**; si dice 33, el
recorte del retorno de carro no está aplicado (nota 68).
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
terraform plan -out=proteger.tfplan \
  -var "image_tag=${IMAGE_TAG}" \
  -var db_activation_policy=ALWAYS \
  -var lb_enabled=true \
  -var api_autoscaling_enabled=true \
  -var worker_autoscaling_enabled=true

terraform apply proteger.tfplan
```

`1 to change, 0 to destroy`. Y **se comprueba en el estado**, igual que en el §3:

```bash
terraform state show google_sql_database_instance.main | grep -E "^\s+deletion_protection\b"
```

Tiene que decir `true`.
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
