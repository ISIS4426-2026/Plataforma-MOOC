# Evidencia F1 — Correo transaccional en cloud (issue #126)

Prueba ejecutada el **2026-09-27** contra
`https://34.24.52.111.sslip.io`, con Brevo SMTP mediante STARTTLS en el puerto
587. La API desplegada corresponde al commit
`694f8386e94b8411330a05ea9bbfa47465dc9b77`.

| Criterio | Resultado |
| :--- | :---: |
| Un registro nuevo recibe el correo y el enlace activa la cuenta | Cumplido |
| La recuperación de contraseña completa el ciclo | Cumplido |
| Las credenciales SMTP permanecen fuera del repositorio | Cumplido |

## Resultado funcional

La prueba usó un alias temporal del buzón verificado; el alias, los tokens, las
contraseñas y el identificador del usuario se omiten deliberadamente.

```text
GET  /api/v1/health                 -> 200
POST /api/v1/auth/register          -> 201 pending_verification
correo de activación recibido       -> sí
GET  /api/v1/auth/verify            -> 200 active
POST /api/v1/auth/password/forgot   -> 202
correo de recuperación recibido     -> sí
POST /api/v1/auth/password/reset    -> 204
login con contraseña anterior       -> 401
login con contraseña nueva          -> 200 active; sesión emitida
```

El ciclo demuestra entrega real en ambos casos: la activación necesitó el token
recibido por correo y la recuperación terminó cambiando la credencial. El
rechazo posterior de la contraseña anterior comprueba que el `204` no fue solo
una respuesta superficial.

## Configuración comprobada

- `smtp-relay.brevo.com:587`, con STARTTLS obligatorio.
- Remitente individual verificado porque el equipo no dispone de dominio.
- `smtp-password` almacenado en Secret Manager y accesible únicamente por
  `sa-web-server`.
- La VM obtiene los secretos durante el arranque y genera un `.env` con modo
  `600`; ninguna credencial forma parte de la imagen o del repositorio.
- Mailpit se conserva únicamente en el Compose local.

Los procedimientos de configuración, rotación y contingencia están en
[`infra/terraform/ADMINISTRACION.md`](../../../../infra/terraform/ADMINISTRACION.md).

Este archivo no contiene claves SMTP, contraseñas, tokens de verificación,
tokens de recuperación ni tokens de sesión.
