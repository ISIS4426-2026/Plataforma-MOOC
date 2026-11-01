# E2 — Consistencia de operaciones académicas entre réplicas

Generado por `scripts/e2e_cloud` el 2026-11-01 04:12:02 -05 contra `https://136.68.55.254.sslip.io`.
No editar a mano: se reescribe en cada corrida.

Pasos verificados: 44 · fallos: 2 · fuera de alcance: 0 · duración: 17s

## Instancias de API observadas (`X-Instance-ID`)

| Instancia (`X-Instance-ID`) | Peticiones atendidas |
| :--- | ---: |
| `b3677a6a89f1` | 25 |
| `62e344a1fb55` | 19 |


## 0. Réplicas disponibles

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | La API responde y alcanza la base administrada | `GET /api/v1/health` | 200 | `b3677a6a89f1` | `639e4384-5c36-4457-b977-1a4eb6d4c80c` | status="pass" database="up" |
| ✅ | El balanceador reparte entre las réplicas exigidas | `GET /api/v1/live` | 200 | `b3677a6a89f1` | `236dfc60-60a9-4d40-82d9-7f5881b30106` | 2 réplica(s) distintas respondieron [62e344a1fb55=6, b3677a6a89f1=6]; se exigen 2 |
| ✅ | Inicia sesión administrador (admin) | `POST /api/v1/auth/login` | 200 | `62e344a1fb55` | `ec029935-1fca-4d92-8021-7e25d01c33ea` | role="administrador" status="active", sesión emitida |
| ✅ | Inicia sesión profesor (profesor1) | `POST /api/v1/auth/login` | 200 | `62e344a1fb55` | `0ee1b1ad-d45c-4109-8907-cbe8ecccaab8` | role="profesor" status="active", sesión emitida |
| ✅ | Inicia sesión estudiante (estudiante1) | `POST /api/v1/auth/login` | 200 | `b3677a6a89f1` | `293fc74b-155f-44f7-b41f-9d0a6ac9c995` | role="estudiante" status="active", sesión emitida |
| ✅ | Inicia sesión estudiante (estudiante2) | `POST /api/v1/auth/login` | 200 | `62e344a1fb55` | `c5b2764d-f6be-4e55-9a01-eb62213567de` | role="estudiante" status="active", sesión emitida |

## 1. Curso de prueba con un cuestionario y dos lecturas obligatorias

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | El profesor crea el curso en borrador | `POST /api/v1/courses` | 201 | `b3677a6a89f1` | `faf9a09c-8107-4a04-ba99-9d15ea9eb9db` | status="draft" |
| ✅ | El profesor crea el módulo | `POST /api/v1/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7/modules` | 201 | `62e344a1fb55` | `78aa6b52-54da-4ba0-9119-3d36ad804b6b` | position=0 (base 0), stable_id emitido |
| ✅ | El profesor crea la unidad | `POST /api/v1/modules/de52802c-a890-4a21-9a4e-b3c8821d5a3c/units` | 201 | `62e344a1fb55` | `484166dc-d4e6-4017-8492-df79087c3bbc` | position=0 (base 0), stable_id emitido |
| ✅ | Recurso de texto 1, obligatorio y visible | `POST /api/v1/units/a4ff54a4-da28-465c-926e-471484362526/resources` | 201 | `62e344a1fb55` | `21f297bc-ecc8-43bb-8205-e2f99e6aa3aa` | position=0 (base 0), stable_id emitido |
| ✅ | Recurso de texto 2, obligatorio y visible | `POST /api/v1/units/a4ff54a4-da28-465c-926e-471484362526/resources` | 201 | `62e344a1fb55` | `17fc6a3f-424f-45a2-b1dc-6fbeb65e8ee5` | position=1 (base 0), stable_id emitido |
| ✅ | Recurso de quiz, obligatorio y visible | `POST /api/v1/units/a4ff54a4-da28-465c-926e-471484362526/resources` | 201 | `b3677a6a89f1` | `4e192bd7-cf75-4b46-afed-bf4753bf4a27` | position=2 (base 0), stable_id emitido |
| ✅ | El profesor define el cuestionario con 3 intentos | `POST /api/v1/resources/de66e5fa-6efa-4e98-8fa6-4a8431f6db45/quiz` | 201 | `b3677a6a89f1` | `5a46d1d9-cb00-4110-926c-21e6fb907fc8` | quiz creado sobre el recurso de66e5fa-6efa-4e98-8fa6-4... |
| ✅ | Pregunta 1 con su clave de respuesta | `POST /api/v1/quizzes/8712f788-2e86-45c9-aec9-8dff4840fc3c/questions` | 201 | `b3677a6a89f1` | `ae120f57-0d1e-4600-aa12-8b4fd6478cbd` | el código de estado es la verificación |
| ✅ | Pregunta 2 con su clave de respuesta | `POST /api/v1/quizzes/8712f788-2e86-45c9-aec9-8dff4840fc3c/questions` | 201 | `62e344a1fb55` | `e63ec84c-7b07-4111-9339-c7ed9b7c3e04` | el código de estado es la verificación |
| ✅ | El autor relee el cuestionario con su clave | `GET /api/v1/resources/de66e5fa-6efa-4e98-8fa6-4a8431f6db45/quiz` | 200 | `62e344a1fb55` | `4916d331-56d4-46bb-9a0f-62a29d125f39` | 2 pregunta(s) con clave |
| ✅ | El curso se publica | `POST /api/v1/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7/publish` | 200 | `b3677a6a89f1` | `19771d9f-54ba-41fd-9e90-daebd67ab389` | status="published" |

## 2. Lectura tras escritura: la publicación se ve igual en todas las réplicas

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | El detalle del curso figura como publicado en cada réplica | `GET /api/v1/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7` | 200 | `b3677a6a89f1` | `053b3bbf-4fe5-465e-88a2-d5474b9220f4` | 12 lectura(s) en 2 réplica(s) [62e344a1fb55=5, b3677a6a89f1=7], 0 obsoleta(s) o fallida(s) — ninguna réplica sirve el borrador anterior |
| ✅ | El catálogo público lo lista en cada réplica | `GET /api/v1/courses?limit=50&search=E2+%C2%B7+Consistencia+entre+r%C3%A9plicas+1793524325` | 200 | `b3677a6a89f1` | `d9f7d957-0ae9-4ad5-8661-c2d68e5d30fa` | 12 lectura(s) en 2 réplica(s) [62e344a1fb55=6, b3677a6a89f1=6], 0 obsoleta(s) o fallida(s) — el catálogo anónimo ya incluye el curso |

## 3. Inscripción repetida y simultánea: una fila y un evento

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Ocho inscripciones simultáneas devuelven la misma inscripción activa | `POST /api/v1/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7/enrollments` | 200 | `62e344a1fb55` | `22ad67c6-a0ee-445f-82aa-333723236cc3` | las 8 respondieron 200 active con 1 identificador(es) distinto(s); réplicas [62e344a1fb55=5, b3677a6a89f1=3] |
| ✅ | El censo del curso tiene una sola fila de estudiante1 | `GET /api/v1/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7/enrollments` | 200 | `b3677a6a89f1` | `fe11d396-883b-4500-a068-11ff4f6b7c63` | 1 fila(s) de estudiante1 en un censo de 1 |
| ❌ | La auditoría registra una sola inscripción, no ocho | `GET /api/v1/admin/audit-logs?action_prefix=enrollment.&actor_id=c0000000-0000-0000-0000-000000000001&limit=100&target_resource=course%3A2d952ef0-8fd3-4716-8324-20fc9b75a674` | 200 | `b3677a6a89f1` | `09b6b5fb-1b3b-43f9-824e-3d87e674b591` | 8 entrada(s) enrollment.* de estudiante1 sobre el curso — **el estado observado no coincide con lo esperado** |
| ❌ | Inscribirse otra vez, ya inscrito, no agrega ningún evento | `POST /api/v1/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7/enrollments` | 200 | `b3677a6a89f1` | `c773f0b5-d463-4eff-b53b-51cf7c645030` | 8 entrada(s) tras el reintento secuencial — **el estado observado no coincide con lo esperado** |
| ✅ | La inscripción activa se lee igual en cada réplica | `GET /api/v1/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7/enrollments/me` | 200 | `b3677a6a89f1` | `f30a5dea-9ef0-4305-b090-6f8d093d381b` | 12 lectura(s) en 2 réplica(s) [62e344a1fb55=6, b3677a6a89f1=6], 0 obsoleta(s) o fallida(s) — status=active en todas |
| ✅ | Con la misma clave, ocho solicitudes simultáneas dejan una inscripción | `POST /api/v1/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7/enrollments` | 200 | `b3677a6a89f1` | `e8dd007f-82b8-4832-9986-f10328214ff5` | 7 llegaron mientras la original corría (409 idempotency_in_progress) y al reintentar recibieron la grabada; todas terminaron en 200 active con 1 identificador(es) |
| ✅ | El censo suma una fila por estudiante, dos en total | `GET /api/v1/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7/enrollments` | 200 | `b3677a6a89f1` | `16480ffd-c664-49af-90ec-c6b7db9a680d` | 1 fila(s) de estudiante2 en un censo de 2 |
| ✅ | La auditoría registra una sola inscripción de estudiante2 | `GET /api/v1/admin/audit-logs?action_prefix=enrollment.&actor_id=c0000000-0000-0000-0000-000000000002&limit=100&target_resource=course%3A2d952ef0-8fd3-4716-8324-20fc9b75a674` | 200 | `62e344a1fb55` | `492d9ea7-b56d-488a-978f-6c5687a046db` | 1 entrada(s) enrollment.* |

## 4. Envío duplicado del cuestionario: una calificación, un intento, un evento

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Seis envíos simultáneos con la misma clave se califican una sola vez | `POST /api/v1/quizzes/8712f788-2e86-45c9-aec9-8dff4840fc3c/submissions` | 200 | `62e344a1fb55` | `4b60c921-0862-4e82-b4b5-b0501206b3b9` | 1 original y 5 repetición(es) con Idempotent-Replay; 1 submission_id distinto(s); intento 1 en todas |
| ✅ | El reenvío en otra réplica devuelve la misma calificación | `POST /api/v1/quizzes/8712f788-2e86-45c9-aec9-8dff4840fc3c/submissions` | 200 | `b3677a6a89f1` | `4551a9b0-482c-46c3-9d59-1d42affdbd05` | original en 62e344a1fb55 y repetición en b3677a6a89f1: mismo submission_id (6ead3d57-a2d8-42fe-a2a1-b...), mismo intento (1), misma nota (100) |
| ✅ | El historial del estudiante tiene un solo envío | `GET /api/v1/quizzes/8712f788-2e86-45c9-aec9-8dff4840fc3c/submissions/me` | 200 | `62e344a1fb55` | `a0461199-d048-4d28-b688-248c35cf70d7` | 1 envío(s) registrado(s) |
| ✅ | La auditoría registra un solo quiz.submitted | `GET /api/v1/admin/audit-logs?action_prefix=quiz.submitted&actor_id=c0000000-0000-0000-0000-000000000001&limit=100&target_resource=quiz%3A8712f788-2e86-45c9-aec9-8dff4840fc3c` | 200 | `b3677a6a89f1` | `4d476631-d258-4c8a-88a5-27030949cf5b` | 1 entrada(s) quiz.submitted de estudiante1 |
| ✅ | El historial se lee igual en cada réplica | `GET /api/v1/quizzes/8712f788-2e86-45c9-aec9-8dff4840fc3c/submissions/me` | 200 | `b3677a6a89f1` | `49496b10-7896-437e-9b43-a3a17b6b6830` | 12 lectura(s) en 2 réplica(s) [62e344a1fb55=3, b3677a6a89f1=9], 0 obsoleta(s) o fallida(s) — un envío en todas |
| ✅ | La misma clave con otro cuerpo se rechaza, no se reinterpreta | `POST /api/v1/quizzes/8712f788-2e86-45c9-aec9-8dff4840fc3c/submissions` | 422 | `62e344a1fb55` | `795cf466-2689-40a4-9d8b-e05fd1afdc32` | code="idempotency_key_reused" |
| ✅ | Sin clave, los envíos simultáneos nunca repiten un número de intento | `POST /api/v1/quizzes/8712f788-2e86-45c9-aec9-8dff4840fc3c/submissions` | 200 | `62e344a1fb55` | `50c5ce72-6eb7-4b22-9b1d-dedc260a4a05` | 1 aceptado(s) con intentos distintos [1], el resto 409; 0 respuesta(s) inesperada(s); réplicas [62e344a1fb55=4, b3677a6a89f1=2] |
| ✅ | Lo registrado coincide con lo aceptado: ni más ni menos | `GET /api/v1/quizzes/8712f788-2e86-45c9-aec9-8dff4840fc3c/submissions/me` | 200 | `62e344a1fb55` | `c1cebd74-680d-407d-959d-46da0dd21dbc` | 1 aceptado(s) → 1 envío(s) en el historial y 1 en la auditoría |
| ✅ | Agotado el cupo, el envío siguiente se rechaza | `POST /api/v1/quizzes/8712f788-2e86-45c9-aec9-8dff4840fc3c/submissions` | 409 | `b3677a6a89f1` | `81c95365-7514-4693-9268-735c7b770863` | code="quiz_conflict"; el historial se queda en 3 de 3 intentos |

## 5. Progreso repetido y simultáneo: sin inflar el avance y con una sola insignia

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Aprobar el quiz completó un recurso, uno solo | `GET /api/v1/progress/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7` | 200 | `62e344a1fb55` | `05631eba-075a-464b-97a1-c4e7adeb5cd1` | 1/3 · 33.33% |
| ✅ | Dieciséis latidos simultáneos se aceptan sin error | `POST /api/v1/progress/heartbeat` | 200 | `b3677a6a89f1` | `ce1fe25f-2f1e-4cb5-9eea-09dd77f379dc` | estados: 16×200 |
| ✅ | El avance quedó en 3 de 3, no más | `GET /api/v1/progress/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7` | 200 | `62e344a1fb55` | `a770c91d-4545-4b1e-8053-4cf910fbf976` | 3/3 · 100.00% · aprobado=true |
| ✅ | Se emitió una sola insignia | `GET /api/v1/admin/audit-logs?action_prefix=badge.&actor_id=c0000000-0000-0000-0000-000000000001&from=2026-11-01T09%3A12%3A02Z&limit=100` | 200 | `b3677a6a89f1` | `5b9c38ec-749c-4346-93cf-95c6c8cd7df9` | 1 entrada(s) badge.issued de estudiante1 desde el inicio de la corrida |
| ✅ | El curso se completó una sola vez | `GET /api/v1/admin/audit-logs?action_prefix=progress.&actor_id=c0000000-0000-0000-0000-000000000001&limit=100&target_resource=course%3A2d952ef0-8fd3-4716-8324-20fc9b75a674` | 200 | `62e344a1fb55` | `8dee1f5b-cb78-4386-923c-de01ef393112` | 1 entrada(s) progress.course_completed |
| ✅ | El avance final se lee igual en cada réplica | `GET /api/v1/progress/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7` | 200 | `b3677a6a89f1` | `d10e30d4-c1b9-4cc7-84b4-7a82185486a5` | 12 lectura(s) en 2 réplica(s) [62e344a1fb55=7, b3677a6a89f1=5], 0 obsoleta(s) o fallida(s) — 3/3 y la insignia en todas |

## 6. Cachés locales: el validador del curso es el mismo en toda réplica

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | El detalle del curso responde con un ETag | `GET /api/v1/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7` | 200 | `b3677a6a89f1` | `d8c9b69c-cd0a-4943-ae46-49bf377efa8b` | ETag presente: true |
| ✅ | Cualquier réplica reconoce el ETag y responde 304 | `GET /api/v1/courses/77929666-eebd-44a0-91a9-8ad53f4e0bd7` | 304 | `b3677a6a89f1` | `2cfe77b8-aa7c-4b6b-bb21-48f14639f7ea` | 2 réplica(s) [62e344a1fb55=6, b3677a6a89f1=6]; 0 respuesta(s) distinta(s) de 304 |
