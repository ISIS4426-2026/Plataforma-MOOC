# Evidencia H2 — Escenario 1: plan y script con validaciones funcionales (issue #132)

El plan está en [`capacity-planning/escenario1.md`](../../../../capacity-planning/escenario1.md). Aquí está el script, sus corridas de prueba y lo que demuestran.

## Estado

| Criterio de aceptación / tarea | Evidencia | |
| :--- | :--- | :---: |
| `capacity-planning/escenario1.md` acordado por el equipo antes de la primera corrida | Documento escrito y listo para revisión; el acuerdo queda como casillas al final (§13) para aprobar en el pull request | ⚠️ falta la aprobación del equipo |
| Recorrido: catálogo, curso, inscripción, contenido, progreso válido y quiz | [`escenario1.jmx`](./escenario1.jmx), 14 pasos por sesión | ✅ |
| Mezcla de lecturas y escrituras constante entre niveles | 9 lecturas / 5 escrituras por sesión, el mismo recorrido en todos los niveles | ✅ |
| Cuentas e intentos distintos por usuario virtual | Un hilo = una cuenta; 3 sesiones = los 3 intentos del quiz; claves de idempotencia únicas | ✅ |
| Decidir si la autenticación entra al recorrido; ráfaga de login como variante | No entra (límite de 10 logins/min por IP); [`escenario1_login_rafaga.jmx`](./escenario1_login_rafaga.jmx) | ✅ |
| Línea base, tres o más niveles crecientes, repetición cerca del límite | 1 → 10 → 25 → 50 → 100 → 200, repetición ×3 (plan §6) | ✅ |
| Patrón de inyección, calentamiento, duración, incrementos, pausas y parada | Plan §7 y §8.3 | ✅ |
| Criterios de éxito y saturación; rechazos de negocio frente a fallos reales | Plan §8 y §9; [`scripts/capacity_resumen_jtl.py`](../../../../scripts/capacity_resumen_jtl.py) los clasifica | ✅ |
| Script que valide el estado resultante, no solo el código HTTP | Cada paso trae aserciones sobre el cuerpo; más una verificación independiente en la base | ✅ |
| Comprobación de envío duplicado de quiz sin doble calificación | Paso 11 (misma `Idempotency-Key`) y paso 13 (cuenta de envíos); 0 duplicados en la base | ✅ |
| Exportar resultados originales por corrida | `resultados/<corrida>/resultados.jtl` (una fila por petición) | ✅ |
| **Piloto corto sin errores inesperados y con las validaciones activas** | ✅ en local, ver abajo. ⏳ **Falta correrlo contra la nube** (requiere las cuentas de carga en Cloud SQL) | ⚠️ |

## Qué se probó y con qué resultado

Todo contra el ambiente local (`docker compose`, semilla funcional y de capacidad) con el **mismo script y las mismas validaciones** que correrán en la nube.

| Corrida | Qué demuestra | Resultado |
| :--- | :--- | :--- |
| [`local-piloto3_…`](./resultados/local-piloto3_20260927_202208/resumen.txt) | Piloto de 3 usuarios × 3 sesiones, ritmo rápido | 126 peticiones, **0 fallos**, 0 fallos de validación |
| [`local-piloto-10-usuarios_…`](./resultados/local-piloto-10-usuarios_20260927_202648/resumen.txt) | 10 usuarios × 3 sesiones, todas las validaciones activas | 420 peticiones, **0 fallos reales, 0 de validación** |
| [`verificacion_estado_bd_piloto_local.txt`](./verificacion_estado_bd_piloto_local.txt) | Consulta directa a la base tras la corrida de 10 usuarios | 10 estudiantes con **exactamente 3 envíos** (30 en total), 10 inscripciones, **0 envíos duplicados del mismo intento**: los 30 reenvíos con la misma clave no calificaron dos veces |
| [`local-sin-reinicio_…`](./resultados/local-sin-reinicio_20260927_202240/resumen.txt) | **Control negativo**: la misma corrida sin reiniciar las cuentas | Las validaciones **sí muerden**: 18 respuestas 409 «sin intentos» clasificadas como *rechazo de negocio* (no como falla de capacidad) y 9 fallos de validación detectados en el avance y en los envíos |
| [`local-login-rafaga_…`](./resultados/local-login-rafaga_20260927_202603/resumen.txt) | Variante de ráfaga de login, 15 cuentas a la vez | 10 inician sesión y 5 reciben 429: el limitador funciona como debe, clasificado como rechazo de negocio |

Nota sobre las latencias de estas corridas: son de un ambiente local con 3 GiB de memoria y no dicen nada sobre la nube. Sirven solo para comprobar que el script funciona.

## Hallazgo que condicionó el diseño

Al recorrer los endpoints se confirmó que **aprobar el quiz completa su recurso** en el avance del curso: con dos latidos el avance queda en 2 de 3 y solo pasa a 3 de 3 (con insignia) cuando se aprueba el quiz. El guion usa eso para que el «progreso válido» dependa de algo que la plataforma verificó, no de un latido que dice «terminé» sobre el propio quiz. Las calificaciones esperadas por sesión (50 → 100 → 0) están en el plan §5.

## Piloto contra la nube (pendiente)

El criterio pide un piloto corto. La base de la nube necesita las cuentas de carga (`scripts/seeds/capacity_data.sql`, cargado hoy solo en local), que exige acceso desde dentro de la red privada (SSH por IAP a una de las VMs, o el cliente de Cloud SQL). Con eso cargado:

```bash
# 1. ¿Existen las cuentas? Si da 401, faltan (se ve rápido y sirve de prueba).
BASE_URL=https://34.24.52.111.sslip.io CAPACITY_PASSWORD=<contraseña de prueba de la semilla> \
  bash scripts/capacity_login_tokens.sh 3 1

# 2. Piloto de 3 usuarios contra la nube
BASE_URL=https://34.24.52.111.sslip.io RAMP_SECONDS=3 THINK_MS=800 THINK_RANGE_MS=800 \
  WARMUP_SKIP_SECONDS=0 bash scripts/run_escenario1.sh 3 nube-piloto
```

Debe dar 0 fallos reales y 0 de validación. Después, `scripts/seeds/capacity_verificar_estado.sql` confirma el estado en la base.

## Archivos

| Archivo | Qué es |
| :--- | :--- |
| [`escenario1.jmx`](./escenario1.jmx) | Plan de JMeter del recorrido |
| [`escenario1_login_rafaga.jmx`](./escenario1_login_rafaga.jmx) | Variante: ráfaga de login |
| [`scripts/run_escenario1.sh`](../../../../scripts/run_escenario1.sh) | Corre un nivel en Docker y resume |
| [`scripts/capacity_login_tokens.sh`](../../../../scripts/capacity_login_tokens.sh) | Paso previo: inicia sesión con ritmo y guarda los tokens localmente |
| [`scripts/capacity_resumen_jtl.py`](../../../../scripts/capacity_resumen_jtl.py) | Resumen clasificado (éxito / negocio / validación / real) |
| [`scripts/seeds/capacity_reset.sql`](../../../../scripts/seeds/capacity_reset.sql) | Deja las cuentas como recién sembradas entre corridas |
| [`scripts/seeds/capacity_verificar_estado.sql`](../../../../scripts/seeds/capacity_verificar_estado.sql) | Verificación independiente del estado, directo en la base |

## Nunca

Credenciales, llaves ni secretos. Los tokens de sesión y la contraseña de prueba viven solo en `capacity-planning/datos/` (en `.gitignore`); los `.jtl` no guardan cabeceras ni cuerpos, y se comprobó que ni el `.jtl` ni el `jmeter.log` contienen la contraseña.
