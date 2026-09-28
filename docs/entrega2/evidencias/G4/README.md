# G4 — Verificación de red y seguridad del despliegue (issue #130)

Fecha de la verificación: 2026-09-28 (UTC). Entorno: proyecto `plataforma-mooc-entrega2`,
`https://34.24.52.111.sslip.io`.

Todo se reproduce con un solo comando, de **solo lectura** (no crea, cambia ni borra nada):

```bash
bash ./scripts/verify_g4.sh
```

Requiere `gcloud` con sesión iniciada, `curl` y `python`. Regenera los archivos `a_` a `f_` de esta
carpeta. Los correos personales del equipo no se escriben en la evidencia: solo aparece cuántas
personas tienen cada rol.

## Resultado en una línea

**Ningún hallazgo crítico abierto.** Hay 4 hallazgos menores y 2 observaciones, todos con su
recomendación más abajo. Los controles pedidos por el enunciado (a–f) pasan.

## Controles

| # | Control | Resultado | Evidencia |
|---|---------|-----------|-----------|
| a | Desde internet solo se ven los puertos 80 y 443 del Web Server | ✅ Web: abiertos solo 80 y 443. Worker: ningún puerto abierto (22, 80, 443, 5432, 6379, 8080, 9090, 9100 probados) | [`a_escaneo_puertos_externo.txt`](./a_escaneo_puertos_externo.txt) |
| b | Base de datos, cola y Worker no alcanzables desde internet | ✅ Cloud SQL solo con IP privada `10.171.240.3` (sin IPv4 pública, sin redes autorizadas, conexión cifrada obligatoria). Redis no publica puertos y el 6379 está cerrado en ambas VMs. Ninguna regla de entrada abre el worker a `0.0.0.0/0` | [`b_base_de_datos_cola_worker.txt`](./b_base_de_datos_cola_worker.txt) |
| c | IAM diferenciado por componente | ✅ Cada VM usa su propia cuenta de servicio (`sa-web-server`, `sa-worker-server`), ninguna usa la cuenta por defecto de Compute. Cero llaves de cuenta de servicio creadas por personas. La API solo puede crear objetos fuera de los derivados y no escribe en el bucket HLS; el worker solo escribe en el bucket HLS | [`c_iam_por_componente.txt`](./c_iam_por_componente.txt) |
| d | Sin secretos en repositorio, imágenes ni evidencias | ✅ con una observación menor (hallazgo 4). Sin llaves privadas ni tokens de proveedores en 381 archivos ni en el historial de git; `.env` nunca se subió; `.env.example` lleva los secretos vacíos; `.dockerignore` excluye `.env`, `.git`, `infra/`, `docs/` y las imágenes finales solo copian los binarios | [`d_secretos.txt`](./d_secretos.txt) |
| e | HTTPS, cookies seguras y CSRF en el entorno desplegado | ✅ El puerto 80 redirige (301) a HTTPS; TLS 1.1 rechazado. Una petición que cambia datos con cookie de sesión y `Origin` ajeno o ausente recibe 403 `csrf_origin_rejected`; con el origen propio pasa el filtro. Cookie `__Host-mooc_session` con `Secure; HttpOnly; SameSite=Lax` (captura real en D3) | [`e_https_cookies_csrf.txt`](./e_https_cookies_csrf.txt), [`../D3/login_https_cookie_secure.txt`](../D3/login_https_cookie_secure.txt) |
| f | Bucket de derivados público vs. originales privados, verificado en las dos direcciones | ✅ Manifiesto HLS sin firmar → 200. Original sin firmar → 403. Escritura anónima en cualquiera de los dos buckets → 403. El bucket de originales tiene el bloqueo de acceso público en `enforced` | [`f_buckets_publico_privado.txt`](./f_buckets_publico_privado.txt) |

### Los cinco chequeos que pidió el docente en el hilo del issue

1. **Red `default` con 42 subredes y regla SSH abierta**: la regla SSH y la de RDP ya no existen y **ninguna VM
   usa esa red** (0 VMs), pero la red `default` y sus 42 subredes siguen ahí, con dos reglas restantes:
   ICMP desde `0.0.0.0/0` y tráfico interno. → **Hallazgo 2, abierto.**
2. **Cuenta de servicio de cada VM**: `mooc-web-server` → `sa-web-server`, `mooc-worker-server` →
   `sa-worker-server`. ✅ (La cuenta por defecto conserva el rol Editor pero nadie la usa: hallazgo 1.)
3. **Asimetría `originals/` privado y `hls/` público, en ambos sentidos**: ✅ ver control f.
4. **Sin llaves de cuenta de servicio**: ✅ 0 llaves de usuario en las tres cuentas.
5. **Worker con entrada cerrada** (tiene IPv4 externa solo por costo): ✅ ninguna regla lo abre a internet y
   ningún puerto responde desde fuera.

## Acceso administrativo y conectividad de salida (tarea f del issue)

**Cómo se administra**
- **SSH solo por IAP**: el puerto 22 únicamente acepta el rango de Google `35.235.240.0/20`
  (regla `mooc-allow-ssh-iap`, etiqueta `allow-iap-ssh`). Desde internet el 22 está cerrado en las dos VMs.
  Entrar exige además el rol `iap.tunnelResourceAccessor`, que hoy tienen 2 personas.
- **Nube**: 1 persona con rol Propietario y 3 con Editor más roles de administración (red, IAM, storage,
  Artifact Registry). Terraform corre con las credenciales personales de quien lo ejecuta
  (`gcloud auth application-default login`); no hay archivos de llave compartidos.
- **Secretos**: la contraseña de la base vive en Secret Manager.

**Conectividad de salida**
- No hay reglas de firewall de salida propias: rige el comportamiento por defecto de GCP (toda salida permitida).
- Las dos VMs tienen IP externa, así que salen a internet por ella. El Cloud NAT `mooc-nat` cubre cualquier VM
  de la subred que no tenga IP externa (por ejemplo, si más adelante se le quita la IP al worker).
- Cloud SQL solo es alcanzable desde la red privada `mooc-vpc`.

## Hallazgos

Ninguno es crítico: no expone datos ni permite acceso no autorizado hoy.

| # | Severidad | Hallazgo | Recomendación | Estado |
|---|-----------|----------|---------------|--------|
| 1 | Media | La cuenta de Compute por defecto (`921648860819-compute@…`) conserva `roles/editor` a nivel proyecto. Ninguna VM la usa, así que hoy no se puede abusar, pero si alguien crea otra VM sin indicar cuenta heredaría Editor | Quitarle el rol (el proyecto no depende de ella). Cambio de IAM: requiere aprobación del equipo | Abierto |
| 2 | Baja | La red `default` (42 subredes) sigue existiendo con ICMP abierto a `0.0.0.0/0`. No tiene VMs | Borrarla (`gcloud compute networks delete default`) tras borrar sus dos reglas restantes | Abierto |
| 3 | Baja | El bucket HLS es legible públicamente **por diseño**, pero `objectViewer` también permite **listar**, así que cualquiera puede enumerar los nombres (`hls/<id>/…`) de todos los videos | Cambiar `allUsers` a un rol personalizado con solo `storage.objects.get`. Los archivos ya son públicos, solo se cerraría la enumeración | Abierto |
| 4 | Baja | `docs/e2e/evidencia/test_{admin,authoring,identity}_raw.json` (Entrega 1) contienen 8 tokens Bearer de sesión sin redactar. Eran de un entorno local y **no sirven en la nube** (probado: 401) | Reemplazarlos por `[REDACTED_SESSION_TOKEN]`, como hace la evidencia de D3 | Abierto |

Observaciones (no son fallas):
- **HSTS ausente**: el servidor no envía `Strict-Transport-Security`. La redirección 301 y la cookie `Secure`
  cubren el caso normal; HSTS evitaría que un navegador intente HTTP la primera vez.
- **Usuarios de prueba**: el seed documenta usuarios con contraseña conocida (`Password123!` en la
  documentación E2E). Si esos usuarios existen en la base de la nube, conviene borrarlos o cambiarles la
  contraseña cuando terminen las pruebas de capacidad.

## Lo que no se pudo verificar

- **Desde dentro de la VM** (`ss -tlnp`, salida real a internet, contenido de las imágenes ya construidas):
  el equipo no automatiza el acceso SSH a producción; la verificación se hizo desde fuera y con la
  configuración de GCP. La ausencia de secretos en las imágenes se apoya en los Dockerfiles multietapa y en
  `.dockerignore`, no en abrir la imagen publicada.
