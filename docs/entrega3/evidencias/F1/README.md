# Evidencia F1 — Superficie de red y accesos mínimos del despliegue elástico ([#219](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/219))

| | |
| :--- | :--- |
| **Bloque** | F — Seguridad, operación y costos |
| **Rúbrica** | Integridad y operación |
| **Responsable** | `@CrispisCas9` |
| **Depende de** | B2 (#201), C3 (#206) — cerrados |
| **Desbloquea** | I2 (#232) |
| **Cómo se corre** | §4 del [runbook de G5](../G5/RUNBOOK.md): misma sesión, el entorno tiene que estar encendido |

**Estado: verificador listo y probado en seco; falta correrlo contra el entorno
encendido.**

---

## Criterios de aceptación

| Criterio | Cómo se demuestra | Estado |
| :--- | :--- | :---: |
| Evidencia de barrido externo sin puertos de aplicación ni de datos abiertos | `a_barrido_externo.txt`: todas las direcciones públicas del proyecto (instancias, reservadas, reglas de reenvío, Cloud SQL) y un barrido de 19 puertos desde fuera de la VPC. Solo puede tener IP pública el balanceador, y solo con 80 y 443 | ⏳ |
| Las reglas de cortafuegos efectivas están inventariadas y cada una tiene una razón | `b_reglas_firewall.txt`: **todas** las reglas del proyecto, en todas las redes, comparadas **por contenido** con las cinco del código (no por nombre). Las que sobran o difieren fallan | ⏳ |
| Los controles de la Entrega 2 siguen vigentes, comprobados sobre el despliegue nuevo | `c_acceso_minimo.txt` (qué reglas alcanzan de verdad a cada instancia, Cloud SQL privada y cifrada, IAP) y `d_controles_entrega2.txt` (HTTP → HTTPS, TLS 1.1 rechazado, CSRF por origen, buckets en los dos sentidos) | ⏳ |
| **Bloquea el cierre:** captura del barrido externo, con la fecha | `capturas/AAAA-MM-DD_barrido-externo.png` y dos más (§4 del runbook) | ⏳ |
| Lo que el equipo debe saber, en `NOTAS_TECNICAS.md` | tras la corrida: las reglas retiradas (para F2 e I6) | ⏳ |

## Qué cambia respecto a la verificación de la Entrega 2

[`verify_g4.sh`](../../../../scripts/verify_g4.sh) **imprimía** y nadie
comprobaba lo impreso: es el patrón de la nota 36. El de esta entrega,
[`f1_superficie_red.sh`](../../../../scripts/f1_superficie_red.sh), **afirma**:
cada control termina en `OK`, `FALLA` u `OBS` (observación que no es falla, como
la ausencia de HSTS), y sale con 1 si hay alguna `FALLA`. Además:

| | Entrega 2 | Entrega 3 |
| :--- | :--- | :--- |
| Qué se barre | las dos IP fijas, escritas en el guion | todas las direcciones públicas que **el proyecto** tiene en ese momento, y falla si hay alguna que no sea el balanceador |
| Reglas | listadas | comparadas con el código por puertos, orígenes y destinos; una regla con el nombre correcto pero ampliada **falla** |
| Acceso a la API | — | qué reglas alcanzan de verdad a cada instancia (por etiqueta, por cuenta de servicio o por toda la red), por componente |
| Entorno aparcado | el barrido daba «cerrado» y parecía bien | falla si el balanceador no tiene reglas de reenvío: un barrido sin nada escuchando no prueba nada |

Retirar sobrantes es otro guion,
[`f1_retirar_reglas_sobrantes.sh`](../../../../scripts/f1_retirar_reglas_sobrantes.sh),
porque borra: pregunta regla por regla, no toca nunca las cinco del código (esas
se corrigen con `terraform apply`) y deja `retiro_reglas.txt`.

### Cómo se probó sin gastar

Contra un proyecto simulado (`gcloud` y `curl` falsos, y dos puertos locales
escuchando como balanceador):

- **limpio** —las cinco reglas del código, sin IPs externas, Cloud SQL privada—:
  sale con 0, una observación (HSTS).
- **con las sobras típicas** —la regla `mooc-allow-web-ingress` de la Entrega 2,
  la red `default` con `default-allow-icmp`, `mooc-allow-lb-health` ampliada a
  `0.0.0.0/0` y un worker con IP externa—: sale con 1 y señala las seis.

## Resultado

*Pendiente de la corrida.*

---

Este archivo no contiene credenciales, llaves ni secretos.
