# E2 — Consistencia de operaciones académicas entre réplicas

Generado por `scripts/e2e_cloud` el 2026-11-01 12:37:45 -05 contra `https://136.68.55.254.sslip.io`.
No editar a mano: se reescribe en cada corrida.

Pasos verificados: 44 · fallos: 0 · fuera de alcance: 0 · duración: 15s

## Instancias de API observadas (`X-Instance-ID`)

| Instancia (`X-Instance-ID`) | Peticiones atendidas |
| :--- | ---: |
| `7b298f27807b` | 23 |
| `fe5bcec9c229` | 21 |


## 0. Réplicas disponibles

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | La API responde y alcanza la base administrada | `GET /api/v1/health` | 200 | `7b298f27807b` | `b3bcb537-6fcb-4033-a0ca-f6f71b45d466` | status="pass" database="up" |
| ✅ | El balanceador reparte entre las réplicas exigidas | `GET /api/v1/live` | 200 | `7b298f27807b` | `70f610d5-2ca2-4427-88c1-2a4cc17d4c37` | 2 réplica(s) distintas respondieron [7b298f27807b=7, fe5bcec9c229=5]; se exigen 2 |
| ✅ | Inicia sesión administrador (admin) | `POST /api/v1/auth/login` | 200 | `fe5bcec9c229` | `aa563097-c734-4319-bc11-e3ab4c21cfce` | role="administrador" status="active", sesión emitida |
| ✅ | Inicia sesión profesor (profesor1) | `POST /api/v1/auth/login` | 200 | `fe5bcec9c229` | `f1334156-6f1f-4e72-aa32-cfb586709f45` | role="profesor" status="active", sesión emitida |
| ✅ | Inicia sesión estudiante (estudiante1) | `POST /api/v1/auth/login` | 200 | `7b298f27807b` | `89333fc9-8219-4fe4-8035-a4e1f4202887` | role="estudiante" status="active", sesión emitida |
| ✅ | Inicia sesión estudiante (estudiante2) | `POST /api/v1/auth/login` | 200 | `7b298f27807b` | `9147c218-8896-47dc-89ae-e9b97d563c7f` | role="estudiante" status="active", sesión emitida |

## 1. Curso de prueba con un cuestionario y dos lecturas obligatorias

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | El profesor crea el curso en borrador | `POST /api/v1/courses` | 201 | `fe5bcec9c229` | `25693b1d-2103-4d86-ad99-4d79f1c30751` | status="draft" |
| ✅ | El profesor crea el módulo | `POST /api/v1/courses/f847abe8-809c-4837-b904-4a9035b5cf01/modules` | 201 | `7b298f27807b` | `46f46e4f-0603-4614-aa73-860d7565024b` | position=0 (base 0), stable_id emitido |
| ✅ | El profesor crea la unidad | `POST /api/v1/modules/76f70442-3de6-473e-8b6e-ddf0c087282b/units` | 201 | `fe5bcec9c229` | `aff3ca28-f0e2-4d61-9f01-3def462c904d` | position=0 (base 0), stable_id emitido |
| ✅ | Recurso de texto 1, obligatorio y visible | `POST /api/v1/units/1cb6ab22-26ea-4102-a341-1c5d8b7d83f4/resources` | 201 | `7b298f27807b` | `ad5d2d00-396b-48b3-8331-d7805c199e81` | position=0 (base 0), stable_id emitido |
| ✅ | Recurso de texto 2, obligatorio y visible | `POST /api/v1/units/1cb6ab22-26ea-4102-a341-1c5d8b7d83f4/resources` | 201 | `7b298f27807b` | `6016b65a-35e5-4579-addc-cc8c08076894` | position=1 (base 0), stable_id emitido |
| ✅ | Recurso de quiz, obligatorio y visible | `POST /api/v1/units/1cb6ab22-26ea-4102-a341-1c5d8b7d83f4/resources` | 201 | `7b298f27807b` | `7784c647-7486-499d-b9f8-e28e515813eb` | position=2 (base 0), stable_id emitido |
| ✅ | El profesor define el cuestionario con 3 intentos | `POST /api/v1/resources/99712369-5b0c-40b1-b556-9132323b6e3d/quiz` | 201 | `7b298f27807b` | `7155834e-2073-469e-9588-95478205ac8b` | quiz creado sobre el recurso 99712369-5b0c-40b1-b556-9... |
| ✅ | Pregunta 1 con su clave de respuesta | `POST /api/v1/quizzes/42f518c1-279b-4135-8831-7ad93289d872/questions` | 201 | `7b298f27807b` | `86a24692-271b-4cb8-9093-0437cb759478` | el código de estado es la verificación |
| ✅ | Pregunta 2 con su clave de respuesta | `POST /api/v1/quizzes/42f518c1-279b-4135-8831-7ad93289d872/questions` | 201 | `7b298f27807b` | `74aa75db-8477-4491-ab39-219a07f1a853` | el código de estado es la verificación |
| ✅ | El autor relee el cuestionario con su clave | `GET /api/v1/resources/99712369-5b0c-40b1-b556-9132323b6e3d/quiz` | 200 | `fe5bcec9c229` | `c305352e-dc4e-45e1-8088-30ee18200fe2` | 2 pregunta(s) con clave |
| ✅ | El curso se publica | `POST /api/v1/courses/f847abe8-809c-4837-b904-4a9035b5cf01/publish` | 200 | `fe5bcec9c229` | `8d8405ce-3db4-4c06-88fb-9de8cba59e7e` | status="published" |

## 2. Lectura tras escritura: la publicación se ve igual en todas las réplicas

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | El detalle del curso figura como publicado en cada réplica | `GET /api/v1/courses/f847abe8-809c-4837-b904-4a9035b5cf01` | 200 | `7b298f27807b` | `3615b772-1284-462c-a71a-5bb07bea56a7` | 12 lectura(s) en 2 réplica(s) [7b298f27807b=5, fe5bcec9c229=7], 0 obsoleta(s) o fallida(s) — ninguna réplica sirve el borrador anterior |
| ✅ | El catálogo público lo lista en cada réplica | `GET /api/v1/courses?limit=50&search=E2+%C2%B7+Consistencia+entre+r%C3%A9plicas+1793554668` | 200 | `7b298f27807b` | `9190c4d6-38fc-4bf9-a84d-32c84879d92f` | 12 lectura(s) en 2 réplica(s) [7b298f27807b=7, fe5bcec9c229=5], 0 obsoleta(s) o fallida(s) — el catálogo anónimo ya incluye el curso |

## 3. Inscripción repetida y simultánea: una fila y un evento

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Ocho inscripciones simultáneas devuelven la misma inscripción activa | `POST /api/v1/courses/f847abe8-809c-4837-b904-4a9035b5cf01/enrollments` | 200 | `7b298f27807b` | `e7e4d73b-85bc-4e14-ba46-8184b69dfe72` | las 8 respondieron 200 active con 1 identificador(es) distinto(s); réplicas [7b298f27807b=4, fe5bcec9c229=4] |
| ✅ | El censo del curso tiene una sola fila de estudiante1 | `GET /api/v1/courses/f847abe8-809c-4837-b904-4a9035b5cf01/enrollments` | 200 | `fe5bcec9c229` | `ab30037c-28df-4982-afa9-ea604fb838ba` | 1 fila(s) de estudiante1 en un censo de 1 |
| ✅ | La auditoría registra una sola inscripción, no ocho | `GET /api/v1/admin/audit-logs?action_prefix=enrollment.&actor_id=c0000000-0000-0000-0000-000000000001&limit=100&target_resource=course%3A22525ff1-f090-4fbf-b309-07d989f6e7f5` | 200 | `fe5bcec9c229` | `0baa46fb-f6ea-4c2b-86e4-c2d30f707cee` | 1 entrada(s) enrollment.* de estudiante1 sobre el curso |
| ✅ | Inscribirse otra vez, ya inscrito, no agrega ningún evento | `POST /api/v1/courses/f847abe8-809c-4837-b904-4a9035b5cf01/enrollments` | 200 | `fe5bcec9c229` | `acca892d-cebe-436b-98d7-ebac87933f5c` | 1 entrada(s) tras el reintento secuencial |
| ✅ | La inscripción activa se lee igual en cada réplica | `GET /api/v1/courses/f847abe8-809c-4837-b904-4a9035b5cf01/enrollments/me` | 200 | `7b298f27807b` | `14689019-9b6c-4b80-892e-c9fe815b64d7` | 12 lectura(s) en 2 réplica(s) [7b298f27807b=7, fe5bcec9c229=5], 0 obsoleta(s) o fallida(s) — status=active en todas |
| ✅ | Con la misma clave, ocho solicitudes simultáneas dejan una inscripción | `POST /api/v1/courses/f847abe8-809c-4837-b904-4a9035b5cf01/enrollments` | 200 | `7b298f27807b` | `8df89c2a-cfc2-481f-9a74-fe19512311bd` | 6 llegaron mientras la original corría (409 idempotency_in_progress) y al reintentar recibieron la grabada; todas terminaron en 200 active con 1 identificador(es) |
| ✅ | El censo suma una fila por estudiante, dos en total | `GET /api/v1/courses/f847abe8-809c-4837-b904-4a9035b5cf01/enrollments` | 200 | `fe5bcec9c229` | `17841424-2802-4c58-9e07-2cbb393ff53d` | 1 fila(s) de estudiante2 en un censo de 2 |
| ✅ | La auditoría registra una sola inscripción de estudiante2 | `GET /api/v1/admin/audit-logs?action_prefix=enrollment.&actor_id=c0000000-0000-0000-0000-000000000002&limit=100&target_resource=course%3A22525ff1-f090-4fbf-b309-07d989f6e7f5` | 200 | `7b298f27807b` | `a19ee2e6-38fd-437e-ae1b-936583b16907` | 1 entrada(s) enrollment.* |

## 4. Envío duplicado del cuestionario: una calificación, un intento, un evento

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Seis envíos simultáneos con la misma clave se califican una sola vez | `POST /api/v1/quizzes/42f518c1-279b-4135-8831-7ad93289d872/submissions` | 200 | `fe5bcec9c229` | `787c044e-529d-43cd-923a-6e77776d4b08` | 1 original y 5 repetición(es) con Idempotent-Replay; 1 submission_id distinto(s); intento 1 en todas |
| ✅ | El reenvío en otra réplica devuelve la misma calificación | `POST /api/v1/quizzes/42f518c1-279b-4135-8831-7ad93289d872/submissions` | 200 | `7b298f27807b` | `720b0d6c-2cc5-4d90-a060-0bc488bd5cb3` | original en fe5bcec9c229 y repetición en 7b298f27807b: mismo submission_id (fd912153-ce14-4644-a0e2-e...), mismo intento (1), misma nota (100) |
| ✅ | El historial del estudiante tiene un solo envío | `GET /api/v1/quizzes/42f518c1-279b-4135-8831-7ad93289d872/submissions/me` | 200 | `7b298f27807b` | `bb8e53d8-8f2d-40c8-aef8-77aefac4d44e` | 1 envío(s) registrado(s) |
| ✅ | La auditoría registra un solo quiz.submitted | `GET /api/v1/admin/audit-logs?action_prefix=quiz.submitted&actor_id=c0000000-0000-0000-0000-000000000001&limit=100&target_resource=quiz%3A42f518c1-279b-4135-8831-7ad93289d872` | 200 | `fe5bcec9c229` | `04acbb9b-2cea-44a1-a5a1-ea293ee77b0c` | 1 entrada(s) quiz.submitted de estudiante1 |
| ✅ | El historial se lee igual en cada réplica | `GET /api/v1/quizzes/42f518c1-279b-4135-8831-7ad93289d872/submissions/me` | 200 | `fe5bcec9c229` | `72c6b1bd-7efd-4465-978d-0dfd637712e3` | 12 lectura(s) en 2 réplica(s) [7b298f27807b=7, fe5bcec9c229=5], 0 obsoleta(s) o fallida(s) — un envío en todas |
| ✅ | La misma clave con otro cuerpo se rechaza, no se reinterpreta | `POST /api/v1/quizzes/42f518c1-279b-4135-8831-7ad93289d872/submissions` | 422 | `fe5bcec9c229` | `28cc6756-a5a4-43e1-9c1f-a76b63339fd8` | code="idempotency_key_reused" |
| ✅ | Sin clave, los envíos simultáneos nunca repiten un número de intento | `POST /api/v1/quizzes/42f518c1-279b-4135-8831-7ad93289d872/submissions` | 200 | `fe5bcec9c229` | `049381d5-ef77-4f82-a285-fbcfb4175c4d` | 2 aceptado(s) con intentos distintos [1 2], el resto 409; 0 respuesta(s) inesperada(s); réplicas [7b298f27807b=4, fe5bcec9c229=2] |
| ✅ | Lo registrado coincide con lo aceptado: ni más ni menos | `GET /api/v1/quizzes/42f518c1-279b-4135-8831-7ad93289d872/submissions/me` | 200 | `7b298f27807b` | `fd144a74-bde9-4d93-ad5b-88faaae7e686` | 2 aceptado(s) → 2 envío(s) en el historial y 2 en la auditoría |
| ✅ | Agotado el cupo, el envío siguiente se rechaza | `POST /api/v1/quizzes/42f518c1-279b-4135-8831-7ad93289d872/submissions` | 409 | `7b298f27807b` | `ad93130d-9b43-43cb-b1cc-1133b2b4eddc` | code="quiz_conflict"; el historial se queda en 3 de 3 intentos |

## 5. Progreso repetido y simultáneo: sin inflar el avance y con una sola insignia

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Aprobar el quiz completó un recurso, uno solo | `GET /api/v1/progress/courses/f847abe8-809c-4837-b904-4a9035b5cf01` | 200 | `fe5bcec9c229` | `4127afac-1062-42bd-811b-d627823913fc` | 1/3 · 33.33% |
| ✅ | Dieciséis latidos simultáneos se aceptan sin error | `POST /api/v1/progress/heartbeat` | 200 | `7b298f27807b` | `5e32d4d9-c77a-4378-9924-91f165f6f084` | estados: 16×200 |
| ✅ | El avance quedó en 3 de 3, no más | `GET /api/v1/progress/courses/f847abe8-809c-4837-b904-4a9035b5cf01` | 200 | `fe5bcec9c229` | `23bf7b91-3a41-4ffb-9bb8-6e6ab356abd2` | 3/3 · 100.00% · aprobado=true |
| ✅ | Se emitió una sola insignia | `GET /api/v1/admin/audit-logs?action_prefix=badge.&actor_id=c0000000-0000-0000-0000-000000000001&from=2026-11-01T17%3A37%3A45Z&limit=100` | 200 | `fe5bcec9c229` | `918fb29e-03a5-4cb7-a837-7d05ae454505` | 1 entrada(s) badge.issued de estudiante1 desde el inicio de la corrida |
| ✅ | El curso se completó una sola vez | `GET /api/v1/admin/audit-logs?action_prefix=progress.&actor_id=c0000000-0000-0000-0000-000000000001&limit=100&target_resource=course%3A22525ff1-f090-4fbf-b309-07d989f6e7f5` | 200 | `fe5bcec9c229` | `cccd43bb-6c9e-46a8-82bb-2f4263f994bb` | 1 entrada(s) progress.course_completed |
| ✅ | El avance final se lee igual en cada réplica | `GET /api/v1/progress/courses/f847abe8-809c-4837-b904-4a9035b5cf01` | 200 | `fe5bcec9c229` | `fd184e63-3d86-4497-af93-d5345a2e024f` | 12 lectura(s) en 2 réplica(s) [7b298f27807b=6, fe5bcec9c229=6], 0 obsoleta(s) o fallida(s) — 3/3 y la insignia en todas |

## 6. Cachés locales: el validador del curso es el mismo en toda réplica

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | El detalle del curso responde con un ETag | `GET /api/v1/courses/f847abe8-809c-4837-b904-4a9035b5cf01` | 200 | `fe5bcec9c229` | `0ee256c9-2aa2-4927-b57c-64270ce0e813` | ETag presente: true |
| ✅ | Cualquier réplica reconoce el ETag y responde 304 | `GET /api/v1/courses/f847abe8-809c-4837-b904-4a9035b5cf01` | 304 | `7b298f27807b` | `80cb7642-da55-4cd3-9a0f-c80689c41865` | 2 réplica(s) [7b298f27807b=8, fe5bcec9c229=4]; 0 respuesta(s) distinta(s) de 304 |
