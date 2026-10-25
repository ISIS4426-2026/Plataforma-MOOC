# Evidencias de la Entrega 3

Un directorio por issue. Los 18 marcados con **captura** tienen además
`capturas/`, y **no se pueden cerrar sin ella**: el entorno se destruye en
I6 y la evidencia no se puede rehacer.

Convención de medios: PNG para pantallas, consola y gráficas; vídeo corto solo
cuando el movimiento es la prueba; nombre `AAAA-MM-DD_descripcion.png`. Sin
credenciales, tokens ni IPs privadas a la vista. Lo que pase de ~10 MB va a la
plataforma del vídeo y se enlaza desde el README del issue.

Antes de empezar cualquiera de estos issues, lee
[`../NOTAS_TECNICAS.md`](../NOTAS_TECNICAS.md); antes de cerrarlo, anota allí lo
que el resto del equipo deba saber.


## A - Preparacion de la aplicacion y de la linea base

| Issue | | Responsable | Captura |
| :--- | :--- | :--- | :--- |
| [`A1`](./A1/) | [#193](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/193) Rama base de la Entrega 3, CI verde y tablero del backlog | @fredyxander | — |
| [`A2`](./A2/) | [#194](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/194) Endpoint de readiness, identificador de instancia y tiempo de arranque | @fredyxander | — |
| [`A3`](./A3/) | [#195](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/195) Migraciones controladas: una sola ejecucion por version, no una por replica | @fredyxander | — |
| [`A4`](./A4/) | [#196](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/196) Sesiones, revocacion y CSRF correctos entre replicas de API | @fredyxander | — |
| [`A5`](./A5/) | [#197](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/197) Archivos multimedia de prueba con los tres perfiles reales declarados | @fredyxander | — |
| [`A6`](./A6/) | [#198](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/198) Criterios de medicion y validacion de metricas de toda la entrega | @fredyxander | — |
| [`A7`](./A7/) | [#199](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/199) Linea base retrospectiva de la Entrega 2 medida contra la nube | @fredyxander | — |

## B - Servicios compartidos fuera de los grupos elasticos

| Issue | | Responsable | Captura |
| :--- | :--- | :--- | :--- |
| [`B1`](./B1/) | [#200](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/200) Cola compartida en un servicio dedicado fuera de ambos grupos elasticos | @fredyxander | — |
| [`B2`](./B2/) | [#201](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/201) Red, salida a internet, firewall e identidades de servicio de los grupos | @fredyxander | — |
| [`B3`](./B3/) | [#202](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/202) Inicializacion automatica y reproducible de cada instancia nueva | @DiegoOrtizRuiz | **la consola del grupo creando una instancia y el log de arranque hasta que queda lista** |
| [`B4`](./B4/) | [#203](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/203) Secretos y configuracion de las plantillas sin valores sensibles | @DiegoOrtizRuiz | — |

## C - Grupos elasticos y balanceador

| Issue | | Responsable | Captura |
| :--- | :--- | :--- | :--- |
| [`C1`](./C1/) | [#204](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/204) Plantilla de instancia y grupo administrado de API (1-3) con autohealing | @fredyxander | **la instancia borrada a mano y el grupo recreandola** |
| [`C2`](./C2/) | [#205](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/205) Politica de autoscaling de la API: senal, objetivo y estabilizacion | @fredyxander | — |
| [`C3`](./C3/) | [#206](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/206) Balanceador externo HTTPS, certificado y reglas de reenvio | @fredyxander | **el navegador en la URL del balanceador** |
| [`C4`](./C4/) | [#207](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/207) Verificaciones de salud y distribucion de trafico entre replicas | @fredyxander | **peticiones repartidas entre dos instancias identificadas** |
| [`C5`](./C5/) | [#208](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/208) Plantilla de instancia y grupo administrado de workers (1-3) | @DiegoOrtizRuiz | **una instancia nueva del grupo consumiendo y terminando su primer trabajo de la cola** |
| [`C6`](./C6/) | [#209](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/209) Politica de autoscaling de workers con senal de cola | @DiegoOrtizRuiz | **la metrica de cola en Cloud Monitoring y la decision del autoscaler tomada sobre ella** |
| [`C7`](./C7/) | [#210](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/210) Retiro seguro de workers: drenaje, reintentos e idempotencia | @DiegoOrtizRuiz | **el drenaje de un worker que se retira con un trabajo activo** |

## D - Frontend minimo funcional

| Issue | | Responsable | Captura |
| :--- | :--- | :--- | :--- |
| [`D1`](./D1/) | [#211](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/211) Base del frontend: stack, build versionado, despliegue, sesion y CORS | @tmichelldiaz | — |
| [`D2`](./D2/) | [#212](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/212) Estudiante: recorrido completo desde el navegador | @tmichelldiaz | **el recorrido completo de estudiante en el navegador** |
| [`D3`](./D3/) | [#213](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/213) Profesor: recorrido completo de autoria y publicacion desde el navegador | @tmichelldiaz | **el recorrido completo de profesor** |
| [`D4`](./D4/) | [#214](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/214) Estados de interfaz y autorizacion negativa verificados en el navegador | @tmichelldiaz | **los estados de interfaz y los dos rechazos de autorizacion** |
| [`D5`](./D5/) | [#215](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/215) Matriz de recorridos, rutas de interfaz y recorrido conectado profesor-estudiante en cloud | @tmichelldiaz | **video corto del recorrido conectado profesor-estudiante** |

## E - Integridad y estado compartido

| Issue | | Responsable | Captura |
| :--- | :--- | :--- | :--- |
| [`E1`](./E1/) | [#216](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/216) Continuidad de sesion y retiro o reemplazo de instancias sin perdida | @DiegoOrtizRuiz | **la sesion atendida por dos instancias distintas** |
| [`E2`](./E2/) | [#217](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/217) Consistencia de operaciones academicas y envio duplicado sin doble calificacion | @CrispisCas9 | — |
| [`E3`](./E3/) | [#218](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/218) Estado externo: objetos y estado de trabajos accesibles desde cualquier instancia | @DiegoOrtizRuiz | — |

## F - Seguridad, operacion y costos

| Issue | | Responsable | Captura |
| :--- | :--- | :--- | :--- |
| [`F1`](./F1/) | [#219](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/219) Superficie de red y accesos minimos del despliegue elastico | @CrispisCas9 | **el barrido externo sin puertos de aplicacion ni de datos abiertos** |
| [`F2`](./F2/) | [#220](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/220) Identidades de servicio con permisos minimos y separacion de las dos autenticaciones | @DiegoOrtizRuiz | — |
| [`F3`](./F3/) | [#221](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/221) Costos: region, tamanos, estimacion con 1 y 3 instancias por grupo y consumo observado | @CrispisCas9 | — |

## G - Pruebas de carga comparativas

| Issue | | Responsable | Captura |
| :--- | :--- | :--- | :--- |
| [`G1`](./G1/) | [#222](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/222) Instrumentacion de las corridas, generador externo y criterios de parada | @DiegoOrtizRuiz | — |
| [`G3`](./G3/) | [#223](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/223) Escenario 1 sobre la arquitectura elastica: actividad academica concurrente | @tmichelldiaz | **las graficas y las salidas originales de cada corrida** |
| [`G4`](./G4/) | [#224](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/224) Escenario 2 sobre la arquitectura elastica: carga, procesamiento y consumo multimedia | @tmichelldiaz | **las graficas por etapa** |
| [`G5`](./G5/) | [#225](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/225) Escenario 3A: ciclo de escalamiento automatico de la API | @CrispisCas9 | **la linea de tiempo de instancias** |
| [`G6`](./G6/) | [#226](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/226) Escenario 3B: ciclo de escalamiento automatico de workers | @DiegoOrtizRuiz | **la linea de tiempo de workers** |
| [`G7`](./G7/) | [#227](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/227) Informe consolidado de capacidad de la Entrega 3 | @CrispisCas9 | — |

## H - Calidad del codigo y deuda tecnica

| Issue | | Responsable | Captura |
| :--- | :--- | :--- | :--- |
| [`H1`](./H1/) | [#228](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/228) SonarQube en GitHub Actions sobre API, workers y frontend | @DiegoOrtizRuiz | — |
| [`H2`](./H2/) | [#229](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/229) Verificacion de umbrales y atencion de hallazgos prioritarios | @DiegoOrtizRuiz | — |
| [`H3`](./H3/) | [#230](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/230) Primer documento de deuda tecnica | @DiegoOrtizRuiz | — |

## I - Documentacion y cierre

| Issue | | Responsable | Captura |
| :--- | :--- | :--- | :--- |
| [`I1`](./I1/) | [#231](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/231) docs/entrega3/arquitectura.md: modelo de componentes y modelo de despliegue | @CrispisCas9 | — |
| [`I2`](./I2/) | [#232](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/232) docs/entrega3/arquitectura.md: decisiones de escalamiento, operacion y capacidad | @DiegoOrtizRuiz | — |
| [`I3`](./I3/) | [#233](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/233) README: URLs HTTPS, documentacion, instrucciones y evidencias de la Entrega 3 | @CrispisCas9 | — |
| [`I4`](./I4/) | [#234](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/234) Video de sustentacion | @CrispisCas9 | — |
| [`I5`](./I5/) | [#235](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/235) Tag entrega-3 y registro del commit evaluado | @fredyxander | — |
| [`I6`](./I6/) | [#236](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/236) Cierre: eliminacion controlada de recursos y procedimiento de reconstruccion | @fredyxander | **la destruccion y la recreacion completa** |

---

Este archivo no contiene credenciales, llaves ni secretos.
