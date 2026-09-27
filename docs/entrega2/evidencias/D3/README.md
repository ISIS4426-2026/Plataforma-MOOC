# Evidencia D3: acceso HTTPS

El hostname público todavía debe ser asignado en D2 y apuntar a la IP estática
del Web Server antes de emitir el certificado. No incluir aquí contraseñas,
tokens, cabeceras `Cookie`/`Set-Cookie`, archivos de certificado ni llaves.

## Puesta en marcha

Configurar en el entorno de despliegue, sin subir el archivo de variables:

- `APP_DOMAIN`: hostname DNS público, sin esquema ni ruta.
- `APP_BASE_URL`: `https://` seguido del hostname.
- `CSRF_ALLOWED_ORIGINS`: el mismo origen HTTPS exacto.

Crear el webroot ACME y levantar primero el proxy HTTP temporal:

```sh
mkdir -p certbot/www
NGINX_CONFIG_FILE=./nginx.bootstrap.conf docker compose -f docker-compose.prod.yml up -d
```

Instalar Certbot en la VM e emitir el certificado mediante HTTP-01. Sustituir
los marcadores por el hostname y el correo operativo reales:

```sh
sudo certbot certonly --webroot -w "$PWD/certbot/www" \
  -d "$APP_DOMAIN" --agree-tos --non-interactive -m "$ACME_EMAIL"
```

Una vez emitido el certificado, recrear el proxy con la configuración final:

```sh
docker compose -f docker-compose.prod.yml up -d --force-recreate proxy
```

Configurar el timer de renovación de Certbot con un deploy hook que ejecute
`docker compose -f <ruta-de-despliegue>/docker-compose.prod.yml exec -T proxy nginx -s reload`.
La carpeta `certbot/www` solo contiene desafíos temporales; los certificados
permanecen en `/etc/letsencrypt` en la VM y se montan como solo lectura.

## Comprobaciones que deben anexarse

- Resolución DNS del hostname hacia la IP pública del Web Server.
- HTTP redirige a HTTPS y el certificado presentado valida para ese hostname.
- Login completado por HTTPS; registrar solo nombres de atributos de cookie:
  `Secure`, `HttpOnly`, `SameSite=Lax` y `Path=/`. No guardar su valor.
- Una mutación con cookie y `Origin` ajeno responde `403` con código
  `csrf_origin_rejected`; una mutación con cookie sin `Origin` ni `Referer`
  también se rechaza.
- Renovación del certificado y recarga de Nginx comprobadas.

La evidencia de ejecución queda pendiente hasta disponer del DNS y completar el
despliegue. Guardar salidas sanitizadas en este directorio.