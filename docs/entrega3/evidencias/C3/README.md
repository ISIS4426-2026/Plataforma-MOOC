# Evidencia C3 — Balanceador externo HTTPS, certificado y reglas de reenvío ([#206](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/206))

| Criterio de aceptación | |
| :--- | :---: |
| La API solo es alcanzable públicamente a través del balanceador y por HTTPS | ✅ §4 — la VM fija se retira; **ya no hay ninguna regla con origen `0.0.0.0/0`** |
| Certificado válido, sin advertencias del navegador | ⏳ §6 — requiere `apply` y hasta 1 h de aprovisionamiento |
| La URL HTTPS del balanceador queda registrada y enlazada desde el README | ⏳ §6 — la URL se conoce al reservar la IP |
| El manejo de las peticiones en curso al retirar una instancia está documentado | ✅ §3 |
| **Bloquea el cierre:** el navegador en la URL del balanceador, certificado válido sin advertencias | ⏳ §6 |
| Anotado en `NOTAS_TECNICAS.md` | ✅ nota 38 |

**Estado: escrito y validado; sin desplegar.** `terraform validate` pasa y `fmt`
no propone cambios.

---

## 1. El certificado: administrado por Google, y los riesgos que eso acepta

El issue pide elegir entre `<IP>.sslip.io` con certificado administrado o uno
propio subido al balanceador, **y registrar el riesgo**.

**Se elige el administrado**, y antes de decidirlo se comprobaron las tres cosas
que podían romperlo en silencio:

| Comprobación | Resultado |
| :--- | :--- |
| `34.24.52.111.sslip.io` resuelve a la IP que lleva dentro | **A = 34.24.52.111** ✅ |
| Registro AAAA parásito — la causa más frecuente de `FAILED_NOT_VISIBLE` | **no hay AAAA** ✅ |
| Registros CAA que limiten quién puede emitir | **no hay CAA**, ni en `sslip.io` ni en el subdominio ✅ |

El tercero importaba más de lo que parece: un CAA que solo autorizara a Let's
Encrypt habría impedido a Google Trust Services emitir, y el certificado se
habría quedado en provisión **sin un error que lo explicara**.

### Los riesgos, declarados

1. **Aprovisionar tarda, hasta una hora.** El `apply` termina antes que el
   certificado, y hasta que está `ACTIVE` el 443 no sirve. No es un fallo, es
   espera, y conviene saberlo antes de empezar a depurar:
   ```
   gcloud compute ssl-certificates describe mooc-lb-cert --global \
     --format="value(managed.status,managed.domainStatus)"
   ```
2. **`sslip.io` es un servicio de terceros.** Si deja de resolver, el nombre
   muere y el certificado **no puede renovarse**, porque la renovación revalida
   el DNS. Una caída suya coincidiendo con la renovación tumbaría HTTPS. Para una
   entrega de curso es aceptable; para producción no, y la salida es declarar un
   dominio propio en `var.app_domain`.
3. **La IP no puede cambiar.** El dominio lleva la IP dentro, así que liberar la
   dirección invalida el certificado. Es estática y reservada por eso.

### La alternativa, y por qué no

Subir el certificado de Let's Encrypt que ya tenía la VM fija. Está emitido para
**la IP de esa VM**, no la del balanceador; caduca cada 90 días; y renovarlo
exigiría mantener certbot con alcance HTTP-01 desde internet — que es exactamente
lo que este balanceador elimina.

## 2. La topología

| Recurso | Qué hace |
| :--- | :--- |
| `google_compute_global_address.lb` | IP estática global. **Reservarla es elegir el dominio**, porque `sslip.io` lo deriva de ella |
| `google_compute_managed_ssl_certificate.lb` | certificado para `<ip>.sslip.io` |
| `google_compute_backend_service.api` | el grupo de C1 por su named port `http`, con `balancing_mode = "RATE"` |
| `google_compute_url_map.api` | todo al backend |
| `google_compute_url_map.redirect_to_https` | el 80 redirige, no sirve |
| `google_compute_target_https_proxy.api` + `target_http_proxy.redirect` | los dos puntos de entrada |
| `google_compute_global_forwarding_rule.https` y `.http_redirect` | 443 y 80 |

**Global y no regional**, que es lo que B2 ya había fijado en la nota 31: los
backends solo aceptan entrada desde `130.211.0.0/22` y `35.191.0.0/16`, que son
los rangos de un balanceador **global**. Uno regional entrega desde una subred de
solo proxy que habría que crear, y entonces el origen cambiaría.

### El 80 redirige en el balanceador, no en una instancia

Es lo que hacía `nginx.conf` con `return 301 https://$host$request_uri`, movido a
donde no cuesta una instancia atenderlo. Con `strip_query = false`, porque un 301
que pierda la cadena de consulta rompe cualquier enlace con parámetros.

## 3. Lo que C3 le debía a C2, y el drenaje

### `RATE`, no `UTILIZATION`

```hcl
balancing_mode        = "RATE"
max_rate_per_instance = 100
```

No es una preferencia: C2 demostró con los datos de A7 que **la CPU de esta API
no se mueve con la carga** —a 103,5 peticiones/s seguía en el 22 %—, así que con
`UTILIZATION` el autoescalador mediría CPU por la puerta de atrás y el grupo se
quedaría en una instancia mientras la base se funde.

El `100` es el denominador que C2 necesita: 103,5 peticiones/s medidas sobre
**una** VM del mismo tipo, con p95 de 122 ms y sin saturarse, redondeado a la
baja. Con el `target = 0.8` del autoescalador, se añade instancia al pasar de
**80 peticiones/s por instancia**.

### El drenaje de conexiones

```hcl
connection_draining_timeout_sec = 60
```

Es el criterio «el manejo de las peticiones en curso al retirar una instancia
está documentado». Cuando una instancia sale del grupo —el autoescalador reduce,
el autohealer la recrea, una rotación de plantilla la sustituye— el balanceador
**deja de mandarle peticiones nuevas** y le da este plazo para terminar las que
ya tiene en vuelo.

60 s es holgura amplia y no un cálculo ajustado: las peticiones de esta API son
de milisegundos, y el peor caso medido en A7 fueron 908 ms **incluyendo el ida y
vuelta de red del generador**. Lo que importa es que el valor por omisión del
proveedor son **300 s**, que retrasaría cada reducción cinco minutos sin que nadie
lo pidiera.

`timeout_sec = 60` conserva el `proxy_read_timeout 60s` que tenía nginx, para no
cambiar el comportamiento al mover el proxy de sitio.

> **Esto no es el drenaje de los workers.** Un worker no recibe peticiones: saca
> tareas de la cola, y pararlo a mitad de un transcodificado es otro problema. Es
> **C7**, y la nota 32 ya dice que el `--timeout 120` de su unidad es un
> compromiso y no una política.

## 4. Lo que se retira, y por qué el criterio lo exige

El criterio es que **la API solo sea alcanzable públicamente a través del
balanceador y por HTTPS**. La VM web fija tenía IP pública con 80 y 443 abiertos
sirviendo la misma aplicación: un segundo camino público. No es opcional
retirarla.

| Se retira | Por qué |
| :--- | :--- |
| `google_compute_instance.web_server` | segundo camino público a la misma API |
| `google_compute_address.web_server_ip` | su dirección estática; reservada y sin usar se factura |
| `google_compute_firewall.allow_web_ingress` | era la única regla con origen `0.0.0.0/0` |
| `nginx.conf`, `nginx.bootstrap.conf` | terminaban TLS en la instancia; solo existían para esa VM |
| `certbot/` | el webroot del desafío ACME, que ya no se sirve |
| Los montajes de `/etc/letsencrypt` y `./certbot/www` | no hay certificado en la instancia |
| El puerto 443 del Compose | TLS se termina en el balanceador |

**Después de C3 no queda ninguna regla de cortafuegos con origen `0.0.0.0/0` en
la VPC.** La entrada pública es el balanceador, y de él a los backends solo por
los dos rangos de Google.

La VM del **worker** sigue: no tiene superficie pública desde B2 y la sustituye
el grupo de C5.

> **Antes de aplicar:** el disco de la VM web era la única copia legible de
> `SMTP_FROM` y `SMTP_USERNAME` sin conocerlos de memoria. Siguen en la metadata
> de la plantilla de instancia, que persiste, pero conviene tenerlos apuntados
> donde el equipo controle.

## 5. La nota 35, resuelta

`local.mooc_app_domain` se calculaba de la IP de la VM fija, y de ese valor salen
**dos** cosas en `prepare_web_env.sh`:

```
APP_BASE_URL='https://${APP_DOMAIN}'
CSRF_ALLOWED_ORIGINS='https://${APP_DOMAIN}'
```

Con el dominio de la VM vieja, **toda petición que mutara estado entrando por el
balanceador habría dado 403** como origen ajeno — el mismo
`csrf_origin_rejected` que costó el primer intento del escenario 2 en A7, y que
se parece a un fallo de la aplicación y no a un valor de configuración.

Ahora `mooc_app_domain = local.lb_domain`. Y queda en pie lo que A4 dejó dicho:
con el frontend **detrás del mismo balanceador** no hace falta ni CORS ni
`SESSION_COOKIE_SAMESITE=none`; en otro origen hacen falta las tres cosas a la
vez. Es una decisión de **D1**.

## 6. Qué falta, y cuánto cuesta esperar

| Criterio | Qué necesita |
| :--- | :--- |
| Certificado válido sin advertencias | el `apply` **y hasta una hora** de aprovisionamiento |
| URL en el README | la URL se conoce al reservar la IP; se enlaza tras el `apply` |
| **La captura del navegador** (bloquea el cierre) | el `apply` y el certificado `ACTIVE` |

### El entorno nace aparcado, y eso es nuevo

Las reglas de reenvío se facturan **por hora, sirvan o no tráfico**: 0,025 USD/h
cada una, y hay dos. Son **1,20 USD/día** con el entorno apagado — veinte veces
lo que cuesta la base detenida.

Pero destruir el balanceador entre sesiones es peor: el certificado tarda hasta
una hora en reaprovisionarse, así que cada sesión empezaría esperando.

**El compromiso:** se conservan la dirección, el certificado, el backend, los
mapas y los proxies —que **no tienen coste por hora**— y se apagan solo las dos
reglas de reenvío con `var.lb_enabled`. El certificado sigue `ACTIVE` entre
sesiones y el dominio no cambia.

| Estado | Coste/día |
| :--- | ---: |
| Aparcado: base detenida, grupo a 0, sin reglas de reenvío | **~0,30 USD** |
| Encendido para medir | ~3,5 USD |

### Encender es ahora el acto explícito

Los tres interruptores tienen por omisión el valor seguro, **salvo uno**:

| Variable | Omisión | Nota |
| :--- | :--- | :--- |
| `lb_enabled` | `false` | sin entrada pública |
| `api_autoscaling_enabled` | `false` | sin autoescalador, el grupo se queda donde esté |
| `db_activation_policy` | **`ALWAYS`** | **no se pudo invertir**: la instancia tiene que existir para poder detenerse, y no se comprobó que el proveedor acepte crearla ya en `NEVER` |

Esa última es la que cuesta dinero si se olvida: **un `apply` sin
`-var db_activation_policy=NEVER` vuelve a encender la base.** El plan lo dice
con `google_sql_database_instance.main will be updated in-place`, y hay que
leerlo.

Para una sesión de medición:

```bash
terraform apply \
  -var lb_enabled=true \
  -var api_autoscaling_enabled=true \
  -var db_activation_policy=ALWAYS
```

Y antes, **publicar la imagen del commit** que se vaya a aplicar: desde la
corrección de la nota 34 el plan se niega si no existe en Artifact Registry.

---

Este archivo no contiene credenciales, llaves ni secretos.
