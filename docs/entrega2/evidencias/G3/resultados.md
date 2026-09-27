# G3 — Resultados de la corrida E2E en la nube

Generado por `scripts/e2e_cloud` el 2026-09-27 16:09:31 -05 contra `https://34.24.52.111.sslip.io`.
No editar a mano: se reescribe en cada corrida.

Pasos verificados: 61 · fallos: 3 · fuera de alcance: 1 · duración: 4m9s


## 0. Entorno desplegado

| | Paso | Petición | HTTP | Estado verificado |
| :---: | :--- | :--- | :---: | :--- |
| ✅ | La API responde y alcanza la base administrada | `GET /api/v1/health` | 200 | status="pass" database="up" |

## 1. Registro, verificación de correo y login

| | Paso | Petición | HTTP | Estado verificado |
| :---: | :--- | :--- | :---: | :--- |
| ✅ | Registro de una cuenta nueva (el 201 implica que el relevo SMTP aceptó el correo) | `POST /api/v1/auth/register` | 201 | role="estudiante" status="pending_verification" |
| ✅ | La cuenta sin verificar no puede iniciar sesión | `POST /api/v1/auth/login` | 403 | code="email_not_verified", sin token emitido |
| ✅ | Un enlace de verificación inválido se rechaza sin distinguir la causa | `GET /api/v1/auth/verify?token=token-inexistente-000000` | 400 | code="invalid_token" |
| ✅ | Reenvío del enlace de verificación (segunda entrega real por SMTP) | `POST /api/v1/auth/verify/resend` | 202 | el código de estado es la verificación |
| ℹ️ | Activación con el token recibido por correo | `GET /api/v1/auth/verify?token=…` | — | el token solo viaja por correo, así que esta pata no se conduce desde aquí; ciclo completo verificado a mano en la evidencia de F1 |
| ✅ | Inicia sesión administrador (admin) | `POST /api/v1/auth/login` | 200 | role="administrador" status="active", sesión emitida |
| ✅ | Inicia sesión profesor (profesor1) | `POST /api/v1/auth/login` | 200 | role="profesor" status="active", sesión emitida |
| ✅ | Inicia sesión profesor (profesor2) | `POST /api/v1/auth/login` | 200 | role="profesor" status="active", sesión emitida |
| ✅ | Inicia sesión estudiante (estudiante1) | `POST /api/v1/auth/login` | 200 | role="estudiante" status="active", sesión emitida |

## 2. Roles, propiedad y accesos denegados

| | Paso | Petición | HTTP | Estado verificado |
| :---: | :--- | :--- | :---: | :--- |
| ✅ | Un estudiante no puede crear cursos | `POST /api/v1/courses` | 403 | code="forbidden" |
| ✅ | El profesor crea el curso en borrador | `POST /api/v1/courses` | 201 | status="draft" version=1 stable_id emitido |
| ✅ | El borrador no aparece en el catálogo público | `GET /api/v1/courses?limit=50&search=G3+%C2%B7+Recorrido+E2E+en+la+nube+1790543373` | 200 | el catálogo responde sin sesión y no lista el borrador |
| ✅ | Sin sesión, el contenido del curso responde 401 | `GET /api/v1/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb/modules` | 401 | code="unauthorized" |
| ✅ | Con sesión pero sin inscripción, responde 403 | `GET /api/v1/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb/modules` | 403 | code="forbidden" |
| ✅ | El autor lee su propio borrador sin inscribirse | `GET /api/v1/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb/modules` | 200 | la lista responde al autor |
| ✅ | El administrador lee el borrador ajeno | `GET /api/v1/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb/modules` | 200 | la lista responde al administrador |
| ✅ | Otro profesor no puede editar un curso que no es suyo | `POST /api/v1/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb/modules` | 403 | code="forbidden", la propiedad manda sobre el rol |

## 3. Autoría y publicación

| | Paso | Petición | HTTP | Estado verificado |
| :---: | :--- | :--- | :---: | :--- |
| ✅ | Publicar un curso vacío acumula los errores de validación | `POST /api/v1/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb/publish` | 422 | code="validation_failed" campos=[structure approval_criteria] |
| ✅ | El intento fallido no publicó nada | `GET /api/v1/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb` | 200 | status="draft" |
| ✅ | El profesor crea el módulo | `POST /api/v1/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb/modules` | 201 | position=0 (base 0), stable_id emitido |
| ✅ | El profesor crea la unidad | `POST /api/v1/modules/2462573d-9eb7-45e1-9077-58c4a1b0c625/units` | 201 | position=0 (base 0), stable_id emitido |
| ✅ | Recurso de video, obligatorio y visible | `POST /api/v1/units/50ecf1d9-bd47-4280-bf2b-888fab961d62/resources` | 201 | position=0 (base 0), stable_id emitido |
| ✅ | Recurso de texto, obligatorio y visible | `POST /api/v1/units/50ecf1d9-bd47-4280-bf2b-888fab961d62/resources` | 201 | position=1 (base 0), stable_id emitido |
| ✅ | Recurso de quiz, obligatorio y visible | `POST /api/v1/units/50ecf1d9-bd47-4280-bf2b-888fab961d62/resources` | 201 | position=2 (base 0), stable_id emitido |
| ✅ | El profesor define el cuestionario | `POST /api/v1/resources/2844a8c0-7a95-4643-ab48-bf875696924d/quiz` | 201 | quiz creado sobre el recurso 2844a8c0-7a95-4643-ab48-b... |
| ✅ | Pregunta 1 con su clave de respuesta | `POST /api/v1/quizzes/b510cff0-b91c-4c0a-b413-e46d98d80ccf/questions` | 201 | position=0 |
| ✅ | Pregunta 2 con su clave de respuesta | `POST /api/v1/quizzes/b510cff0-b91c-4c0a-b413-e46d98d80ccf/questions` | 201 | position=1 |
| ✅ | El autor relee el cuestionario con su clave de respuestas | `GET /api/v1/resources/2844a8c0-7a95-4643-ab48-bf875696924d/quiz` | 200 | 2 pregunta(s) con clave, passing_score=70 |

## 4. Carga multimedia, procesamiento asíncrono y consumo

| | Paso | Petición | HTTP | Estado verificado |
| :---: | :--- | :--- | :---: | :--- |
| ✅ | La API firma la URL de carga directa | `POST /api/v1/media/presigned-url` | 200 | method="PUT" content_type="video/mp4" object_key bajo originals/11ccfd7a-5715-4bdb-b527-011129ecccde/ |
| ✅ | Una extensión no permitida no obtiene firma | `POST /api/v1/media/presigned-url` | 400 | code="invalid_input" |
| ✅ | Un estudiante no obtiene URL de carga | `POST /api/v1/media/presigned-url` | 403 | code="forbidden" |
| ✅ | El navegador sube el original directo al bucket con la URL firmada | `PUT https://storage.googleapis.com/plataforma-mooc-entrega2-media/originals/11c...` | 200 | 174692 bytes aceptados por Cloud Storage sin pasar por la API |
| ✅ | Confirmar la carga registra el objeto y encola el procesamiento | `POST /api/v1/media/uploads/3f9db22b-0061-4a0a-b2a4-f74fb9d23e71/complete` | 202 | processing_status="pending" object_key registrado |
| ✅ | Confirmar dos veces la misma carga no cambia el estado | `POST /api/v1/media/uploads/3f9db22b-0061-4a0a-b2a4-f74fb9d23e71/complete` | 202 | object_key intacto, processing_status="pending" |
| ✅ | El autor obtiene una URL firmada de lectura del original | `GET /api/v1/media/resources/3f9db22b-0061-4a0a-b2a4-f74fb9d23e71/download-url` | 200 | la URL lleva firma V4 |
| ✅ | El original se lee con la firma y devuelve los bytes subidos | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/originals/11c...` | 200 | Content-Length=174692 frente a 174692 subidos |
| ✅ | El mismo original sin firma es inaccesible | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/originals/11c...` | 403 | el prefijo originals/ no es legible de forma anónima |
| ❌ | El worker de la otra VM transcodifica el original a HLS | `GET /api/v1/units/{unitID}/resources (sondeo)` | — | processing_status="pending" tras 4m2s de espera — **el recurso no alcanzó "completed" en 4m0s; la tarea no llegó al worker o falló** |
| ❌ | El prefijo hls/ admite lectura sin firma | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/hls/no-existe...` | 403 | un objeto inexistente responde 404 (prefijo público) y no 403 (prefijo privado) — **se esperaba 404 y respondió 403; code=""** |
| ❌ | Un reproductor consume el manifiesto HLS sin firma | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/hls/11ccfd7a-...` | 403 | el maestro declara sus variantes por ruta relativa — **se esperaba 200 y respondió 403; code=""** |

## 3. Autoría y publicación (cierre)

| | Paso | Petición | HTTP | Estado verificado |
| :---: | :--- | :--- | :---: | :--- |
| ✅ | Con estructura y criterio de aprobación, el curso se publica | `POST /api/v1/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb/publish` | 200 | status="published" version=1 |
| ✅ | Un curso publicado es inmutable | `PUT /api/v1/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb` | 409 | code="course_immutable" |
| ✅ | El rechazo dejó el título intacto | `GET /api/v1/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb` | 200 | title sigue siendo el publicado ("G3 · Recorrido E2E en la...") |
| ✅ | El curso publicado ya figura en el catálogo público | `GET /api/v1/courses?limit=50&search=G3+%C2%B7+Recorrido+E2E+en+la+nube+1790543373` | 200 | el catálogo anónimo lo lista |

## 5. Inscripción, quiz idempotente y progreso

| | Paso | Petición | HTTP | Estado verificado |
| :---: | :--- | :--- | :---: | :--- |
| ✅ | El estudiante se inscribe en el curso publicado | `POST /api/v1/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb/enrollments` | 200 | status="active" sobre el stable_id del curso |
| ✅ | Con la inscripción activa ya lee el contenido | `GET /api/v1/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb/modules` | 200 | 1 módulo(s) visibles donde antes había 403 |
| ✅ | El estudiante ve el cuestionario sin la clave de respuestas | `GET /api/v1/resources/2844a8c0-7a95-4643-ab48-bf875696924d/quiz` | 200 | la respuesta no trae questions_with_key ni is_correct |
| ✅ | El progreso arranca en cero sobre los tres recursos obligatorios | `GET /api/v1/progress/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb` | 200 | 0/3 · 0.00% · aprobado=false |
| ✅ | El estudiante envía el quiz y queda calificado | `POST /api/v1/quizzes/b510cff0-b91c-4c0a-b413-e46d98d80ccf/submissions` | 200 | score=100 passed=true attempt=1 |
| ✅ | Reenviar con la misma Idempotency-Key no vuelve a calificar | `POST /api/v1/quizzes/b510cff0-b91c-4c0a-b413-e46d98d80ccf/submissions` | 200 | submission_id idéntico y attempt sigue en 1 |
| ✅ | Solo quedó un envío registrado, no dos | `GET /api/v1/quizzes/b510cff0-b91c-4c0a-b413-e46d98d80ccf/submissions/me` | 200 | 1 envío(s) en el historial del estudiante |
| ✅ | Aprobar el quiz completó su recurso sin latido del cliente | `GET /api/v1/progress/courses/d336462c-58f4-4c47-8f1c-7beb4cc362eb` | 200 | 1/3 · 33.33% |
| ✅ | Un latido sobre la lectura avanza el progreso | `POST /api/v1/progress/heartbeat` | 200 | 2/3 · 66.67% |
| ✅ | El último recurso obligatorio completa el curso y emite la insignia | `POST /api/v1/progress/heartbeat` | 200 | 3/3 · 100.00% · aprobado=true · insignia=true |
| ✅ | Repetir el latido no infla el avance | `POST /api/v1/progress/heartbeat` | 200 | sigue en 3/3 |
| ✅ | El autor no acumula progreso en su propio curso | `POST /api/v1/progress/heartbeat` | 403 | code="forbidden": sin inscripción no hay avance |

## 6. Insignia y verificación pública

| | Paso | Petición | HTTP | Estado verificado |
| :---: | :--- | :--- | :---: | :--- |
| ✅ | El estudiante lee su propia insignia | `GET /api/v1/badges/fc886a46-00f5-4b49-879b-8b220547da40` | 200 | course_stable_id correcto, revoked=false, ETag emitido |
| ✅ | Con el ETag vigente la lectura es condicional | `GET /api/v1/badges/fc886a46-00f5-4b49-879b-8b220547da40` | 304 | 304 sin cuerpo |
| ✅ | La insignia de otra persona responde 404, no 403 | `GET /api/v1/badges/fc886a46-00f5-4b49-879b-8b220547da40` | 404 | code="not_found": el identificador no confirma que exista |
| ✅ | Cualquiera verifica la insignia con el código, sin sesión | `GET /api/v1/badges/verify/e7af08f9-e31d-4463-b4ed-fb36b31a058b` | 200 | valid=true revoked=false, nombra el curso y no identifica al estudiante |
| ✅ | Un código inexistente no confirma ni niega nada más que el 404 | `GET /api/v1/badges/verify/00000000-0000-0000-0000-000000000000` | 404 | code="not_found" |
