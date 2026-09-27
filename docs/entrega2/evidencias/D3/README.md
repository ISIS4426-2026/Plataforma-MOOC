# Evidencia D3 — Web Server: Acceso HTTPS, Certificado TLS, Cookies Seguras y CSRF detrás del Proxy (Issue #124)

Este directorio documenta la configuración del acceso cifrado de usuarios por **HTTPS**, garantizando la preservación de cookies seguras (`__Host-`), la propagación controlada de cabeceras de proxy inverso y la defensa contra ataques de falsificación de peticiones en sitios cruzados (**CSRF**), cumpliendo con la rúbrica de **Funcionamiento de la plataforma y configuración de red (10%)** de la Entrega 2.

Todas las evidencias corresponden a ejecuciones reales sobre la infraestructura en Google Cloud Platform (`mooc-web-server`) y han sido sanitizadas para excluir cualquier tipo de contraseña, token de sesión, llave privada o secreto.

---

## Criterios de Aceptación y Archivos de Evidencia

| Archivo | Criterio del Enunciado | Estado |
| :--- | :--- | :---: |
| [`redireccion_http_a_https.txt`](./redireccion_http_a_https.txt) | **HTTP redirige a HTTPS**: Redirección permanente HTTP 301 desde el puerto 80 hacia el puerto 443 conservando la ruta y parámetros de la solicitud. | ✅ Cumplido |
| [`certificado_valido.txt`](./certificado_valido.txt) | **Certificado válido, sin advertencias del navegador**: Certificado público emitido por Let's Encrypt CA para `34.24.52.111.sslip.io`, verificado por CA de confianza pública (TLSv1.3), y validación de simulación de renovación (`certbot renew --dry-run`). | ✅ Cumplido |
| [`login_https_cookie_secure.txt`](./login_https_cookie_secure.txt) | **Login completo por HTTPS con cookie Secure**: Emisión de la cabecera `Set-Cookie` con prefijo `__Host-`, banderas `Secure`, `HttpOnly`, `SameSite=Lax` y `Path=/`, permitiendo el consumo autenticado posterior. | ✅ Cumplido |
| [`rechazo_csrf_origen_no_permitido.txt`](./rechazo_csrf_origen_no_permitido.txt) | **Origen no permitido recibe 403 de CSRF**: Petición de mutación con cookie de sesión y encabezado `Origin` no permitido rechazada con `HTTP 403 Forbidden` (`csrf_origin_rejected`), así como peticiones con cookie sin encabezado `Origin`/`Referer`. Petición con origen legítimo procesada con `HTTP 204 No Content`. | ✅ Cumplido |
| [`supervivencia_reinicio.txt`](./supervivencia_reinicio.txt) | **El contenedor sobrevive a un reinicio de la VM**: Recuperación autónoma del stack HTTPS completo tras `sudo reboot` de la máquina virtual en menos de 35 segundos. | ✅ Cumplido |

---

## 1. Topología de Red y Terminación TLS

```
                          INTERNET (Navegador / Cliente)
                                     │
                     ┌───────────────┴───────────────┐
                     │ Port 80 (HTTP)                │ Port 443 (HTTPS)
                     ▼                               ▼
       ┌───────────────────────────┐   ┌───────────────────────────┐
       │ Nginx: Redirección 301    │   │ Nginx: Terminación TLS    │
       │ (excepto /.well-known/)   │   │ Certificado Let's Encrypt │
       └─────────────┬─────────────┘   └─────────────┬─────────────┘
                     │                               │
                     │  301 Moved Permanently        │  Inyección de cabeceras:
                     └──────────────────────────────>│  - X-Real-IP: $remote_addr
                                                     │  - X-Forwarded-For: $remote_addr
                                                     │  - X-Forwarded-Proto: https
                                                     │  - X-Forwarded-Host: $host
                                                     ▼
                                       ┌───────────────────────────┐
                                       │ api:8080 (Red Docker)     │
                                       │ IP Proxy: 172.30.0.2      │
                                       │ - ForwardedHeaders MW     │
                                       │ - CSRF Middleware         │
                                       │ - Auth Handler (Cookie)   │
                                       └───────────────────────────┘
```

1. **Resolución DNS y Dominio:** Se utilizó el FQDN `34.24.52.111.sslip.io`, el cual resuelve de forma nativa e inmediata a la IP estática pública `34.24.52.111` del Web Server. Al pertenecer a la Public Suffix List (PSL), califica plenamente para certificados públicos de Let's Encrypt sin restricciones compartidas.
2. **Reto ACME HTTP-01 y Renovación:**
   * Nginx expone la ubicación `^~ /.well-known/acme-challenge/` apuntando al volumen compartido `/var/www/certbot` montado en el host en `./certbot/www`.
   * El servicio systemd de renovación periódica de Certbot (`certbot.timer`) ejecuta el reto HTTP sin interrumpir la operación del servidor web.
   * Se configuró el hook de despliegue en `/etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh`, ejecutando `docker compose exec -T proxy nginx -s reload` tras cualquier actualización de llaves.

---

## 2. Parámetros de Configuración y Entorno de Producción

En cumplimiento con la validación en arranque (`internal/config/validate.go`), el despliegue en producción rechaza cualquier valor de desarrollo:

| Variable | Valor Efectivo | Propósito / Validación |
| :--- | :--- | :--- |
| `APP_ENV` | `production` | Activa comprobaciones estrictas de seguridad. |
| `APP_DOMAIN` | `34.24.52.111.sslip.io` | Nombre de dominio del servidor web para Nginx y certificados. |
| `APP_BASE_URL` | `https://34.24.52.111.sslip.io` | Origen público HTTPS canónico (valida que comience por `https://` y no contenga rutas). |
| `CSRF_ALLOWED_ORIGINS` | `https://34.24.52.111.sslip.io` | Lista de orígenes autorizados para peticiones con cookies de sesión. |
| `TRUSTED_PROXY_IP` | `172.30.0.2` | Dirección IPv4 estática interna del contenedor Nginx en la red Docker `backend`. |
| `DATABASE_URL` | `postgres://...` | Conexión privada con `sslmode=require` hacia Cloud SQL (`10.171.240.3`). |
| `STORAGE_BACKEND` | `gcs` | Backend de Google Cloud Storage con bucket regional. |

---

## 3. Modelo de Seguridad: Cookies y Mitigación de CSRF

### A. Cookies de Sesión (`__Host-mooc_session`)
El servidor implementa cookies bajo el estándar RFC 6265bis:
* **Prefijo `__Host-`:** Exige que la cookie sea servida exclusivamente sobre conexiones HTTPS, que tenga la directiva `Path=/` y que no posea la directiva `Domain` (impidiendo que otros subdominios del mismo dominio puedan leerla o sobreescribirla).
* **`HttpOnly`:** Impide que cualquier código JavaScript en el navegador acceda a la cookie, neutralizando el robo de credenciales mediante vulnerabilidades XSS.
* **`SameSite=Lax`:** El navegador restringe el envío de la cookie en peticiones cruzadas generadas por sitios de terceros.

### B. Defensa contra Cross-Site Request Forgery (CSRF)
* El middleware `internal/http/middleware/csrf.go` detecta automáticamente si la solicitud entrante utiliza autenticación basada en cookies (`__Host-mooc_session`).
* Para métodos mutables (`POST`, `PUT`, `PATCH`, `DELETE`), si la petición viaja con la cookie de sesión, el middleware comprueba que el encabezado `Origin` o `Referer` esté presente y coincida de manera idéntica con un origen registrado en `CSRF_ALLOWED_ORIGINS`.
* Si el origen no está permitido o está ausente, la petición es abortada inmediatamente con código `HTTP 403 Forbidden` (`csrf_origin_rejected`), impidiendo que un sitio malicioso induzca acciones no deseadas a través del navegador de la víctima.

### C. Confianza Controlada de Cabeceras `X-Forwarded-*`
* A través del middleware `internal/http/middleware/forwarded.go`, la API únicamente procesa las cabeceras `X-Forwarded-For`, `X-Forwarded-Proto` y `X-Forwarded-Host` si la dirección del par de red (`r.RemoteAddr`) coincide de manera exacta con `TRUSTED_PROXY_IP` (`172.30.0.2`).
* Cualquier cliente externo o contenedor no autorizado que intente inyectar cabeceras `X-Forwarded-*` directamente a la API es ignorado, asegurando la integridad de los registros de auditoría y limitadores de tasa.