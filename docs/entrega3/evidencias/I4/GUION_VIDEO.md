# I4 — Guion del vídeo de sustentación, Entrega 3 ([#234](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/234))

**Versión 1.0, 2026-11-02, lista para montar.** El vídeo se arma sobre las capturas que cada issue guardó mientras
ocurría ([`INVENTARIO_CAPTURAS.md`](./INVENTARIO_CAPTURAS.md)), sin repetir los ciclos. Los procesos largos se
muestran desde sus ejecuciones previas, cuyos registros completos están en el repositorio, como permite el enunciado.

| | |
| :--- | :--- |
| Duración | **18 min** (bloques de abajo). El enunciado de la Entrega 3 no fija tope; se toma el de 20 min de la Entrega 2 con margen |
| Quién habla | **los cuatro**, 4:30 cada uno (tabla «Tiempo por presentador») |
| Formato | captura o documento a pantalla completa y la voz de quien presenta. Una diapositiva de título por bloque (número, nombre, presentador) de 2–3 s |
| Publicación | enlace con acceso para el equipo docente, en la fila 8 del README (ver «Después de subir») |

## Bloques

Las rutas de las capturas son relativas a `docs/entrega3/evidencias/`. «Qué decir» son las cifras que no pueden
faltar; el resto, con las palabras de cada uno.

| # | Bloque | Min. | Presenta | Qué se muestra, en este orden | Qué decir |
| ---: | :--- | ---: | :--- | :--- | :--- |
| 0 | Apertura | 0:40 | @CrispisCas9 | `README.md` §Entrega 3: las dos URL y la tabla de los ocho entregables · `I3/capturas/2026-11-02_urls-https-responden.png` | Qué se entrega y dónde está; frontend y API por el mismo balanceador HTTPS |
| 1 | Arquitectura desplegada | 2:00 | @fredyxander | `arquitectura.md` §2.1 y §3.1 (diagramas) · `C3/capturas/2026-10-27_balanceador-cadena-completa.png` · `C3/capturas/2026-10-27_certificado-valido.png` | Dos grupos de 1 a 3 instancias detrás de un balanceador HTTPS; cola, PostgreSQL administrado y almacenamiento fuera de las réplicas; nada público salvo el balanceador |
| 2 | Arranque autónomo y balanceo | 1:20 | @fredyxander | `B3/capturas/2026-10-27_arranque-hasta-listo.png` · `C4/capturas/2026-10-28_dos-replicas-sanas.png` · `C4/capturas/2026-10-28_replica-excluida.png` · `C4/capturas/2026-10-28_replica-reincorporada.png` | Una instancia nueva queda lista sola desde la metadata (62–66 s); el balanceador saca a la que falla (43 s) y la reincorpora |
| 3 | Recorrido del profesor | 1:15 | @tmichelldiaz | `D3/capturas/2026-10-31_nube-profesor-inicio-mis-cursos.png` · `D3/capturas/2026-10-31_nube-profesor-video-subido-disponible.png` · `D3/capturas/2026-10-31_nube-profesor-curso-publicado.png` | Crea el curso, sube un video (en cola a los 2 s, disponible a los 6 s) y lo publica, desde el navegador y en la nube |
| 4 | Recorrido del estudiante y recorrido conectado | 1:45 | @tmichelldiaz | `D2/capturas/2026-10-29_nube-estudiante-catalogo.png` · `D2/capturas/2026-10-31_nube-estudiante-aula-video-hls.png` · `D2/capturas/2026-10-31_nube-estudiante-documento-pdf-descarga-ok.png` · **`D5/capturas/2026-10-31_nube-recorrido-conectado-profesor-estudiante.png`** | El estudiante encuentra el curso que el profesor acaba de publicar, lo cursa (texto, HLS, cuestionario con nota 100) y queda «Curso aprobado», sin llamadas manuales a la API |
| 5 | Integridad entre réplicas | 1:20 | @CrispisCas9 | `E1/capturas/2026-11-01_sesion-atendida-por-dos-instancias.png` · `E1/capturas/2026-11-01_retiro-de-una-api-bajo-trafico-85-de-85.png` · `E2/capturas/2026-11-01_envio-duplicado-calificado-una-vez-y-repetido-en-otra-replica.png` · `E2/capturas/2026-11-01_inscripcion-simultanea-una-fila-y-un-solo-evento.png` | Cualquier réplica atiende cualquier petición; al retirar una API bajo tráfico, 85 de 85 respuestas correctas; un envío duplicado se califica una sola vez aunque llegue a otra réplica |
| 6 | Escenario 3A: ciclo de la **API** | 2:15 | @DiegoOrtizRuiz | `G5/capturas/2026-11-01_g5-partida-1-instancia.png` → `…autoescalador-amplia-a-2.png` → `…instancia-nueva-sana-y-reparto.png` → `…reduccion-a-1.png` → `…auditoria-sin-intervencion-manual.png` · `…tablero-del-ciclo.png` | Carga escalonada 10 → 130 pet/s; umbral 80 pet/s por instancia. **Reacción 48 s, incorporación 157 s** (reparto 5/5), reducción a 1 a los ~11 min; **0 errores en 90 080 peticiones**; la auditoría solo tiene al agente del grupo: nadie tocó nada |
| 7 | Escenario 3B: ciclo de **workers** | 2:15 | @DiegoOrtizRuiz | `G6/capturas/2026-11-01_ciclo_autoescalado_workers_y_cola.png` · `…consumo_efectivo_workers_nuevos_terminal.png` · `…reconciliacion_final_y_auditoria_terminal.png` · `C7/capturas/2026-10-29_drenaje-worker-con-trabajo-activo.png` | Ciclo 1 → 2 → 3 → 2 → 1 por la cola; **reacción 26 s, incorporación 99 s**; 8 de 8 trabajos sin pérdidas ni duplicados; un worker que se retira devuelve su trabajo a la cola |
| 8 | Seguridad y costos | 1:10 | @fredyxander | `F1/capturas/2026-11-01_barrido-externo.png` · `F1/capturas/2026-11-01_reglas-firewall.png` · `F2/capturas/2026-11-01_verificacion-identidades-y-ausencia-secretos.png` · `F3/capturas/2026-11-01_presupuesto-activo.png` | Solo 80 y 443 abiertos, TLS 1.2 mínimo, ninguna regla abierta a internet, identidad por componente. 24×7: **168,69 USD/mes** con 1 instancia por grupo; lo caro es la entrada y la cola, no escalar (~0,055 USD/h por instancia extra) |
| 9 | Resultados comparativos | 1:50 | @CrispisCas9 | [Informe de capacidad](../../../../capacity-planning/pruebas_de_carga_entrega3.md) §2.1 y §3.1 (tablas) · `G3/capturas/2026-11-01_campana-completa-instancias-cpu-balanceador.png` · `G4/capturas/2026-11-02_g4-campana-instancias-cola-cpu.png` · informe §6 | **Escenario 1:** igual hasta ~110 pet/s; a 255 pet/s escalar no mejora y la cola de latencia empeora mientras entran instancias; candidato a cuello de botella, PostgreSQL. **Escenario 2:** 8 de 8 videos frente a 4 de 8 y espera máxima en cola **305 s frente a 1 184 s**; el procesamiento de cada video no cambia; primer cuadro **378 ms** (p50). Hallazgo: un retiro con videos en curso los retrasa |
| 10 | Calidad y deuda técnica | 1:30 | @tmichelldiaz | `H1/capturas/2026-11-01_h1_resumen_verificacion.png` · `H2/capturas/2026-11-01_h2_umbrales_y_deuda.png` · `H3/capturas/2026-11-01_h3_estructura_e_inventario_8_campos.png` · `deuda-tecnica.md` | Línea base de calidad del commit final, umbrales, inventario priorizado y plan de tratamiento |
| 11 | Cierre | 0:40 | @CrispisCas9 | `README.md`, tabla de entregables · informe §9 (limitaciones) | Qué quedó medido, qué no (nivel 4 del escenario 2, saturación del escenario 1) y dónde está cada cosa |
| | **Total** | **18:00** | | | |

## Tiempo por presentador

| Presenta | Bloques | Tiempo |
| :--- | :--- | ---: |
| @CrispisCas9 | 0, 5, 9, 11 | 4:30 |
| @tmichelldiaz | 3, 4, 10 | 4:30 |
| @fredyxander | 1, 2, 8 | 4:30 |
| @DiegoOrtizRuiz | 6, 7 | 4:30 |

## Cómo montarlo

1. Grabar cada bloque por separado (pantalla + voz), con las capturas abiertas en el orden de la tabla.
2. Unir en orden, con la diapositiva de título de cada bloque. Sin música; si se pasa de 18 min, cada presentador
   recorta dentro de sus bloques para conservar el reparto; los bloques 6 y 7 no se recortan (son los criterios de elasticidad).
3. Revisar que ninguna toma muestre credenciales, tokens, IP privadas ni el correo personal de nadie.

## Después de subir

- Enlace en la fila 8 de la tabla de entregables de `README.md` (sustituye «*(I4, pendiente…)*»).
- El mismo enlace en `D5/README.md` §Pendiente: el recorrido conectado se muestra en el bloque 4.
- Comprobar que el enlace abre sin iniciar sesión con una cuenta del equipo (o con acceso dado al equipo docente).
- Cerrar #234 con el enlace.

## Antes de grabar

- [x] Capturas de todos los bloques en el repositorio, incluidas G4, G5, F1 y el recorrido conectado (D5).
- [x] G4 y G7 cerrados: el bloque 9 usa el informe definitivo.
- [ ] Cada presentador repasa su bloque con su issue abierto: la sustentación síncrona puede pedir explicar una
      configuración o repetir una prueba.

---

Este archivo no contiene credenciales, llaves ni secretos.
