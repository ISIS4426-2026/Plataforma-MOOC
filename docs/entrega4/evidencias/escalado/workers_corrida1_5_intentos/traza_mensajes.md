# Rastro de 9 tareas, de la publicación a la confirmación

Fase *workers* de la prueba de escalado (2026-11-04, 01:23:54 UTC): 9 videos de 120 s confirmados a la vez. Cada fila es un `message_id` de Pub/Sub; los tiempos salen de los registros de Cloud Logging de `mooc-backend` (`media job published`) y `mooc-workers` (`media job received` y `media job acknowledged`).

| message_id | Publicado (UTC) | Espera hasta que un worker lo recibe | Procesamiento hasta confirmar (2xx) | Total |
|---|---|---:|---:|---:|
| 22266651096961738 | 01:23:54.887 | 7.6 s | 47.4 s | 54.9 s |
| 22266651096961751 | 01:23:54.888 | 4.2 s | 33.2 s | 37.4 s |
| 22266651096961740 | 01:23:54.888 | 4.5 s | 48.1 s | 52.7 s |
| 22266651096961755 | 01:23:54.889 | 109.7 s | 50.6 s | 160.3 s |
| 22266651096961750 | 01:23:54.890 | 116.7 s | 52.1 s | 168.8 s |
| 22266651096961711 | 01:23:54.891 | 52.7 s | 53.3 s | 106.0 s |
| 22266651096961748 | 01:23:54.892 | 83.1 s | 35.9 s | 119.0 s |
| 22266651096961763 | 01:23:54.892 | 54.9 s | 51.6 s | 106.5 s |
| 22266651096961749 | 01:23:54.893 | 46.5 s | 36.1 s | 82.5 s |

- Espera mínima 4.2 s y máxima 116.7 s: los tres primeros mensajes los toman los workers que arrancan (3 instancias); los demás esperan, rechazados y reintentados, hasta que una instancia queda libre, porque cada instancia atiende un video a la vez.
- Procesamiento: entre 33.2 s y 53.3 s por video de 120 s en 2 vCPU.
- **Hubo reentregas.** El intento con el que se confirmó cada mensaje fue: 3 mensajes en el 1.º, 3 en el 3.º, 1 en el 4.º y 2 en el 5.º (el último que permitía la suscripción, `max_delivery_attempts=5`). Los intentos anteriores no llegaron al código: Cloud Run los rechazó por no tener instancia libre (ver `respuestas_workers.md`). El trabajo se hizo una sola vez por mensaje.
- Por eso el límite de intentos se subió a 20 (`cloudrun.tf`) y la prueba se repitió: ver `../`.

## Los registros de una tarea, tal cual

`message_id` 22266651096961751:

```text
2026-11-04T01:23:54.888Z  mooc-backend  media job published 22266651096961751
2026-11-04T01:23:59.058Z  mooc-workers  2026/11/04 01:23:59 INFO media job received message_id=22266651096961751 delivery_attempt=1 resource_id=96cad2dd-33e8-44ad-900c-74cba5901f12 object_key=originals/35509402-8b48-47f1-ae57-32bb1cb4c5d3/008bf644-2e4e-419c-be8a-de879d1
2026-11-04T01:24:32.301Z  mooc-workers  2026/11/04 01:24:32 INFO media job acknowledged message_id=22266651096961751 delivery_attempt=1 resource_id=96cad2dd-33e8-44ad-900c-74cba5901f12 object_key=originals/35509402-8b48-47f1-ae57-32bb1cb4c5d3/008bf644-2e4e-419c-be8a-de8
```
