# Rastro de 12 tareas, de la publicación a la confirmación (corrida 2, `max_delivery_attempts=20`)

Fase *workers* de la prueba de escalado (2026-11-04, 05:51:34 UTC): 12 videos de 120 s confirmados a la vez. Cada fila es un `message_id` de Pub/Sub; los tiempos salen de Cloud Logging (`media job published` en `mooc-backend`, `media job received` y `media job acknowledged` en `mooc-workers`).

| message_id | Publicado (UTC) | Espera hasta que un worker lo recibe | Procesamiento hasta confirmar | Total | Intento con que se confirmó |
|---|---|---:|---:|---:|---:|
| 22267856139182956 | 05:51:34.638 | 52.5 s | 52.4 s | 104.9 s | 1 |
| 22267856139182944 | 05:51:34.638 | 4.3 s | 48.2 s | 52.5 s | 1 |
| 22267856139182952 | 05:51:34.640 | 5.0 s | 50.6 s | 55.6 s | 1 |
| 22267856139182958 | 05:51:34.640 | 5.0 s | 32.0 s | 36.9 s | 1 |
| 22267856139182955 | 05:51:34.641 | 106.9 s | 35.2 s | 142.1 s | 1 |
| 22267856139182967 | 05:51:34.641 | 177.6 s | 34.9 s | 212.4 s | 1 |
| 22267856139182957 | 05:51:34.641 | 192.6 s | 52.1 s | 244.7 s | 2 |
| 22267856139182951 | 05:51:34.642 | 8.5 s | 49.3 s | 57.7 s | 1 |
| 22267856139182966 | 05:51:34.647 | 71.7 s | 35.2 s | 106.9 s | 1 |
| 22267856139182953 | 05:51:34.647 | 5.0 s | 50.3 s | 55.3 s | 1 |
| 22267856139182971 | 05:51:34.649 | 36.9 s | 34.8 s | 71.7 s | 1 |
| 22267856139182980 | 05:51:34.656 | 142.1 s | 35.5 s | 177.6 s | 1 |

- Espera entre 4.3 s y 192.6 s; procesamiento entre 32.0 s y 52.4 s por video.
- Intentos de entrega: 11 mensajes se confirmaron en el 1.º intento y 1 en el 2.º (máximo permitido: 20). Cloud Run respondió 12 veces `204` y 1 vez `429` (sin instancia libre) en toda la corrida; nada llegó a la cola de mensajes muertos.
- Cada video se procesó una sola vez: 12 `media job completed` para 12 mensajes.
