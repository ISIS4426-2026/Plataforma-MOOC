# Fase api: usuarios concurrentes contra el backend

Inicio (UTC): 2026-11-04T01:17:36Z

| Escalon | Usuarios | Peticiones | Errores | Req/s | p50 | p95 | p99 | Instancias distintas vistas |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | 50 | 56320 | 0 | 563 | 87ms | 116ms | 133ms | 1 |
| 2 | 200 | 120072 | 0 | 1201 | 167ms | 230ms | 260ms | 1 |
| 3 | 500 | 137877 | 0 | 1379 | 149ms | 1.304s | 1.419s | 1 |

Fin (UTC): 2026-11-04T01:22:38Z · instancias distintas en toda la fase: 1
