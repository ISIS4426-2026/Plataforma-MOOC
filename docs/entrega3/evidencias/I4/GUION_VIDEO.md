# I4 — Guion del vídeo de sustentación, Entrega 3 ([#234](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/234))

**Borrador v0.1, 2026-11-01.** Montado sobre las capturas que cada issue guardó mientras ocurría
([`INVENTARIO_CAPTURAS.md`](./INVENTARIO_CAPTURAS.md)), sin volver a provocar los ciclos. Los procesos largos se
muestran desde sus ejecuciones previas, con sus registros completos en el repositorio, como permite el enunciado.

| | |
| :--- | :--- |
| Duración objetivo | **18 min**. El enunciado de la Entrega 3 no fija tope; se toma el de 20 min de la Entrega 2 con 2 de margen. **Confirmar con el curso** |
| Quién habla | **los cuatro**: el criterio de aceptación de I4 exige que todos participen y puedan explicar configuración, decisiones y resultados. Cada bloque lo presenta quien construyó esa parte |
| Formato | pantalla con la captura o el documento, y la voz del presentador. Una cámara por presentador en la esquina, si el curso lo pide |
| Publicación | enlace con acceso para el equipo docente, enlazado desde el README (fila 8 de la tabla de entregables) |

## Bloques

| # | Bloque | Min. | Presenta | Qué se muestra | La idea que tiene que quedar |
| ---: | :--- | ---: | :--- | :--- | :--- |
| 0 | Apertura | 0:45 | @CrispisCas9 | README de la Entrega 3: las dos URL y la tabla de entregables | Qué se entrega y dónde está cada cosa |
| 1 | Arquitectura desplegada | 2:30 | @fredyxander | [`arquitectura.md`](../../arquitectura.md) §2.1 y §3.1; [`C3`](../C3/capturas/): balanceador y certificado | Dos grupos de 1 a 3 detrás de un balanceador HTTPS; cola, base y objetos fuera de las réplicas; nada público salvo el balanceador |
| 2 | Arranque autónomo y balanceo | 1:30 | @fredyxander | [`B3`](../B3/capturas/) arranque desde la metadata; [`C4`](../C4/capturas/) dos réplicas sanas y exclusión de una enferma | Una instancia nueva queda lista sola (62–66 s); el balanceador saca a la que falla en 43 s |
| 3 | Recorrido del profesor | 1:30 | @tmichelldiaz | [`D3`](../D3/capturas/): mis cursos, vídeo subido, curso publicado | Autoría y publicación desde el navegador, en la nube |
| 4 | Recorrido del estudiante y recorrido conectado | 2:00 | @tmichelldiaz | [`D2`](../D2/capturas/): catálogo, aula con HLS, PDF, cuestionario, curso aprobado · **vídeo corto de D5** *(falta)* | El estudiante consume lo que el profesor acaba de publicar, sin llamadas manuales a la API |
| 5 | Integridad entre réplicas | 2:00 | @DiegoOrtizRuiz y @CrispisCas9 | [`E1`](../E1/capturas/): sesión en dos instancias, retiro de una API bajo tráfico (85 de 85) · [`E2`](../E2/capturas/): envío duplicado calificado una vez | Cualquier réplica atiende cualquier petición, y retirar una no pierde nada |
| 6 | Ciclo de escalamiento de la **API** | 2:00 | @CrispisCas9 | [`G5`](../G5/capturas/): partida en 1, el autoescalador pide 2, la nueva sana, reparto 5/5, reducción a 1 · `linea_de_tiempo.md` | Creación y retiro **automáticos**, con la auditoría que prueba que nadie tocó el grupo; reacción e incorporación por separado |
| 7 | Ciclo de escalamiento de **workers** y recuperación de trabajos | 2:00 | @DiegoOrtizRuiz | [`G6`](../G6/capturas/): ciclo 1→2→3→2→1, consumo por el worker nuevo, reconciliación 8/8 · [`C7`](../C7/capturas/): drenaje con trabajo activo | Los workers nuevos consumen de la cola compartida a los 99 s; un worker que se retira devuelve su trabajo sin perderlo |
| 8 | Seguridad | 1:00 | @tmichelldiaz | [`F1`](../F1/capturas/): barrido externo, reglas, instancias sin IP · [`F2`](../F2/capturas/) | Solo 80 y 443, TLS 1.2 mínimo, ninguna regla abierta a internet, identidades por componente |
| 9 | Resultados comparativos | 2:15 | @tmichelldiaz y @CrispisCas9 | [Informe de capacidad](../../../../capacity-planning/pruebas_de_carga_entrega3.md): tabla comparativa del escenario 1, escenario 2, 3A y 3B · [`G3`](../G3/capturas/) campaña completa | Qué mejoró, qué no y por qué; el cuello de botella probable es PostgreSQL; lo caro es la entrada, no escalar (F3) |
| 10 | Calidad y deuda técnica | 1:30 | @DiegoOrtizRuiz | [`H1`](../H1/capturas/), [`H2`](../H2/capturas/), [`H3`](../H3/capturas/) · [`deuda-tecnica.md`](../../deuda-tecnica.md) | Línea base del commit final, umbrales, inventario priorizado y plan de tratamiento |
| 11 | Cierre | 0:30 | @CrispisCas9 | tabla de entregables del README | Qué queda declarado como limitación y dónde está |
| | **Total** | **19:30** | | | **Recortar 1:30** al grabar: candidatos, los bloques 1 y 9 |

## Antes de grabar

- [x] Capturas de **G5** y **F1**: hechas el 2026-11-01.
- [ ] El **vídeo corto de D5**, que solo se puede grabar con el entorno encendido.
- [ ] G4 y G7 cerrados: el bloque 9 se monta sobre el informe definitivo, no sobre el borrador.
- [ ] Cada presentador repasa su bloque con su issue abierto: el enunciado pide que todos puedan explicar
      configuración, decisiones y resultados, y la sustentación síncrona puede pedir repetir una prueba.
- [ ] Ninguna captura con credenciales, tokens ni IP privadas a la vista (las de las evidencias ya cumplen).

---

Este archivo no contiene credenciales, llaves ni secretos.
