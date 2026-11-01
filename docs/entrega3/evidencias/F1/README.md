# Evidencia F1 — Superficie de red y accesos mínimos del despliegue elástico ([#219](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/219))

| | |
| :--- | :--- |
| **Bloque** | F — Seguridad, operación y costos |
| **Rúbrica** | Integridad y operación |
| **Responsable** | `@CrispisCas9` |
| **Depende de** | B2 (#201), C3 (#206) — cerrados |
| **Desbloquea** | I2 (#232) |
| **Cómo se corre** | §4 del [runbook de G5](../G5/RUNBOOK.md): misma sesión, el entorno tiene que estar encendido |

**Estado: ejecutado el 2026-11-01 contra el entorno encendido: 0 FALLA, 23 controles OK y 1 observación (HSTS, heredada).**

---

## Criterios de aceptación

| Criterio | Cómo se demuestra | Estado |
| :--- | :--- | :---: |
| Evidencia de barrido externo sin puertos de aplicación ni de datos abiertos | `a_barrido_externo.txt`: todas las direcciones públicas del proyecto (instancias, reservadas, reglas de reenvío, Cloud SQL) y un barrido de 19 puertos desde fuera de la VPC. Solo puede tener IP pública el balanceador, y solo con 80 y 443 | ⏳ |
| Las reglas de cortafuegos efectivas están inventariadas y cada una tiene una razón | `b_reglas_firewall.txt`: **todas** las reglas del proyecto, en todas las redes, comparadas **por contenido** con las cinco del código (no por nombre). Las que sobran o difieren fallan | ⏳ |
| Los controles de la Entrega 2 siguen vigentes, comprobados sobre el despliegue nuevo | `c_acceso_minimo.txt` (qué reglas alcanzan de verdad a cada instancia, Cloud SQL privada y cifrada, IAP) y `d_controles_entrega2.txt` (HTTP → HTTPS, TLS 1.1 rechazado, CSRF por origen, buckets en los dos sentidos) | ⏳ |
| **Bloquea el cierre:** captura del barrido externo, con la fecha | `capturas/AAAA-MM-DD_barrido-externo.png` y dos más (§4 del runbook) | ✅ |
| Lo que el equipo debe saber, en `NOTAS_TECNICAS.md` | la nota 63: la política TLS, la red `default` retirada y las trampas del guion en Windows | ✅ |

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

Corrida del **2026-11-01, 23:05–23:09 UTC**, desde el portátil (fuera de la VPC), con el entorno encendido y la imagen `97120ca`: [`resumen.txt`](./resumen.txt) — **0 FALLA, 23 OK, 1 OBS**.

| Control | Resultado |
| :--- | :--- |
| Direcciones públicas | solo el balanceador (`136.68.55.254`) y la dirección de salida del NAT, sin ningún puerto abierto en esta última |
| Barrido de 19 puertos | **solo el 80 (redirige a HTTPS) y el 443** responden en el balanceador |
| Reglas de cortafuegos | las 5 del código, iguales a lo declarado y con su razón; **ninguna con origen `0.0.0.0/0`** |
| Instancias | ninguna con IP externa; cada una con la cuenta y las etiquetas de su componente; solo las alcanzan balanceador/salud, IAP e ICMP interno |
| Cloud SQL | sin IPv4 pública, solo por la red privada, conexión cifrada obligatoria |
| Controles de la Entrega 2 | HTTP → HTTPS, certificado válido, **TLS 1.0/1.1 rechazados**, CSRF por origen (403 el ajeno y el ausente), buckets público/privado en los dos sentidos |
| Observación | sin `Strict-Transport-Security` (heredada, no es falla) |

**Lo que la primera pasada encontró y se corrigió en esta sesión** (detalle en la nota 63):

1. **El balanceador aceptaba TLS 1.0/1.1** (real). Se añadió una política SSL con mínimo TLS 1.2 (`load_balancer.tf`) y se aplicó; el control sale OK.
2. **La red `default` y sus dos reglas heredadas de la Entrega 2** (real): retiradas con confirmación; registro en [`retiro_reglas.txt`](./retiro_reglas.txt).
3. **Cinco falsas FALLA del guion** (Windows y la IP del NAT): corregidas en `f1_superficie_red.sh`.

Las tres capturas son la salida real de la consola dibujada con [`scripts/render_consola.py`](../../../../scripts/render_consola.py) (el pie de cada una nombra el archivo y las líneas de origen). La de reglas omite la de ICMP interno por llevar el rango privado de la subred:

| Captura | Qué muestra |
| :--- | :--- |
| [`2026-11-01_barrido-externo.png`](./capturas/2026-11-01_barrido-externo.png) | el barrido con la fecha: solo 80 y 443 abiertos en el balanceador; la IP del NAT, ninguno |
| [`2026-11-01_reglas-firewall.png`](./capturas/2026-11-01_reglas-firewall.png) | las reglas que quedan y «ninguna con origen 0.0.0.0/0» |
| [`2026-11-01_instancias-sin-ip-externa.png`](./capturas/2026-11-01_instancias-sin-ip-externa.png) | instancias sin IP externa, y Cloud SQL privada y cifrada |

---

Este archivo no contiene credenciales, llaves ni secretos.
