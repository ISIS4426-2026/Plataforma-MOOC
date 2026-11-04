# Respuestas de mooc-workers durante la corrida 1

Cloud Run (registro de peticiones, 01:23:50 a 01:28:00 UTC). Cada fila es un intento de entrega de Pub/Sub.

| Respuesta | Intentos | Qué significa |
|---|---:|---|
| 204 | 9 | el worker terminó el video y confirmó (1 por mensaje) |
| 429 | 11 | Cloud Run no tenía instancia libre: Pub/Sub reintenta |
| 500 | 6 | «request aborted after waiting too long for an available instance» (también de Cloud Run, no del código) |

Total de intentos: 26 para 9 mensajes. Ningún rechazo llegó a la aplicación; el trabajo se hizo una sola vez por mensaje (9 `media job completed`).
