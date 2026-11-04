# G3 / E1 — Resultados de la corrida E2E en la nube

Generado por `scripts/e2e_cloud` el 2026-11-04 11:57:26 -05 contra `https://mooc-frontend-f2nof4l7pq-ue.a.run.app`.
No editar a mano: se reescribe en cada corrida.

Pasos verificados: 103 · fallos: 0 · fuera de alcance: 1 · duración: 55s

## Instancias de API observadas (`X-Instance-ID`)

| Instancia (`X-Instance-ID`) | Peticiones atendidas |
| :--- | ---: |
| `49960de5880e` | 90 |


## 0. Entorno desplegado

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | La API responde y alcanza la base administrada | `GET /api/v1/health` | 200 | `49960de5880e` | `024a19b4-7f20-4944-bf62-897f608ef198` | status="pass" database="up" |
| ✅ | La sonda de liveness responde sin consultar dependencias | `GET /api/v1/live` | 200 | `49960de5880e` | `6cf05e82-e772-47c7-a016-f342eee48bdd` | 200 mientras el proceso sirva, para que un parpadeo de la base no recree las réplicas |
| ✅ | La sonda de readiness declara que la réplica puede atender | `GET /api/v1/ready` | 200 | `49960de5880e` | `4ed77d1d-5493-4394-85e5-f10855096466` | 200 con la base alcanzable |
| ✅ | La sonda pública no revela dependencias ni topología | `GET /api/v1/ready` | 200 | `49960de5880e` | `4ed77d1d-5493-4394-85e5-f10855096466` | cuerpo={"status":"pass"} |
| ✅ | Cada respuesta dice qué réplica la atendió | `GET /api/v1/ready` | 200 | `49960de5880e` | `4ed77d1d-5493-4394-85e5-f10855096466` | X-Instance-ID="49960de5880e" |

## 1. Registro, verificación de correo y login

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Registro de una cuenta nueva (el 201 implica que el relevo SMTP aceptó el correo) | `POST /api/v1/auth/register` | 201 | `49960de5880e` | `baed3a97-5ef6-44ed-a946-dc6d366eea8b` | role="estudiante" status="pending_verification" |
| ✅ | La cuenta sin verificar no puede iniciar sesión | `POST /api/v1/auth/login` | 403 | `49960de5880e` | `0de5058e-c2ad-49a7-bc1b-9ef9cf46585a` | code="email_not_verified", sin token emitido |
| ✅ | Un enlace de verificación inválido se rechaza sin distinguir la causa | `GET /api/v1/auth/verify?token=token-inexistente-000000` | 400 | `49960de5880e` | `4d8ffbbe-9b06-4c64-9578-041a4fba2771` | code="invalid_token" |
| ✅ | Reenvío del enlace de verificación (segunda entrega real por SMTP) | `POST /api/v1/auth/verify/resend` | 202 | `49960de5880e` | `8d03f464-8594-45b6-9f47-cad9e5e978e0` | el código de estado es la verificación |
| ℹ️ | Activación con el token recibido por correo | `GET /api/v1/auth/verify?token=…` | — | — | — | el token solo viaja por correo, así que esta pata no se conduce desde aquí; ciclo completo verificado a mano en la evidencia de F1 |
| ✅ | Inicia sesión administrador (admin) | `POST /api/v1/auth/login` | 200 | `49960de5880e` | `1a9b2cca-d6d5-4c9f-9cfd-32aac6009e31` | role="administrador" status="active", sesión emitida |
| ✅ | Inicia sesión profesor (profesor1) | `POST /api/v1/auth/login` | 200 | `49960de5880e` | `1de44ec4-3bd4-4692-aae8-443df2f6d713` | role="profesor" status="active", sesión emitida |
| ✅ | Inicia sesión profesor (profesor2) | `POST /api/v1/auth/login` | 200 | `49960de5880e` | `b1de0fa9-4e3d-4323-8891-dee2096111e8` | role="profesor" status="active", sesión emitida |
| ✅ | Inicia sesión estudiante (estudiante1) | `POST /api/v1/auth/login` | 200 | `49960de5880e` | `972bd59c-917b-4cf3-a267-f4575291084b` | role="estudiante" status="active", sesión emitida |

## 2. Roles, propiedad y accesos denegados

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Un estudiante no puede crear cursos | `POST /api/v1/courses` | 403 | `49960de5880e` | `172691da-cf4c-4e95-aa20-be3f1f33af2e` | code="forbidden" |
| ✅ | El profesor crea el curso en borrador | `POST /api/v1/courses` | 201 | `49960de5880e` | `7350dc10-0df4-4c9f-898f-da3bb6668a18` | status="draft" version=1 stable_id emitido |
| ✅ | El borrador no aparece en el catálogo público | `GET /api/v1/courses?limit=50&search=G3+%C2%B7+Recorrido+E2E+en+la+nube+1793811449` | 200 | `49960de5880e` | `a66801d3-d059-4e6f-8d34-c4beef4d35ca` | el catálogo responde sin sesión y no lista el borrador |
| ✅ | Sin sesión, el contenido del curso responde 401 | `GET /api/v1/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961/modules` | 401 | `49960de5880e` | `0a92dbff-0aea-489f-a301-c02e0e14399c` | code="unauthorized" |
| ✅ | Con sesión pero sin inscripción, responde 403 | `GET /api/v1/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961/modules` | 403 | `49960de5880e` | `0bcb4c85-69ba-4694-be01-555456ce4a4a` | code="forbidden" |
| ✅ | El autor lee su propio borrador sin inscribirse | `GET /api/v1/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961/modules` | 200 | `49960de5880e` | `017d0fc0-0594-4aa9-a0bb-12863e47e421` | la lista responde al autor |
| ✅ | El administrador lee el borrador ajeno | `GET /api/v1/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961/modules` | 200 | `49960de5880e` | `0f02fab2-700a-45eb-87f3-21b8b30f6388` | la lista responde al administrador |
| ✅ | Otro profesor no puede editar un curso que no es suyo | `POST /api/v1/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961/modules` | 403 | `49960de5880e` | `20db6479-3bbb-467c-bf23-fb999f8e258c` | code="forbidden", la propiedad manda sobre el rol |

## 3. Autoría y publicación

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Publicar un curso vacío acumula los errores de validación | `POST /api/v1/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961/publish` | 422 | `49960de5880e` | `91d7fbec-1098-4fd3-9bbf-e089c0f9c5b2` | code="validation_failed" campos=[structure approval_criteria] |
| ✅ | El intento fallido no publicó nada | `GET /api/v1/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961` | 200 | `49960de5880e` | `dde43048-5eba-4e68-ba97-ad3239046916` | status="draft" |
| ✅ | El profesor crea el módulo | `POST /api/v1/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961/modules` | 201 | `49960de5880e` | `108d58ca-7bac-4518-b516-5cba4f84ce23` | position=0 (base 0), stable_id emitido |
| ✅ | El profesor crea la unidad | `POST /api/v1/modules/626ca2e9-9169-4cc7-ae11-52372132d314/units` | 201 | `49960de5880e` | `7d4e7915-c3da-44b8-b500-d767b0b93a0e` | position=0 (base 0), stable_id emitido |
| ✅ | Recurso de video, obligatorio y visible | `POST /api/v1/units/b25cf01a-a568-4ccd-acd8-e580cc87fdd5/resources` | 201 | `49960de5880e` | `f2fe65e3-981d-4ac1-aeb8-10c96462779e` | position=0 (base 0), stable_id emitido |
| ✅ | Recurso de texto, obligatorio y visible | `POST /api/v1/units/b25cf01a-a568-4ccd-acd8-e580cc87fdd5/resources` | 201 | `49960de5880e` | `4f826a5f-7565-483f-a166-6af3bade3bb5` | position=1 (base 0), stable_id emitido |
| ✅ | Recurso de quiz, obligatorio y visible | `POST /api/v1/units/b25cf01a-a568-4ccd-acd8-e580cc87fdd5/resources` | 201 | `49960de5880e` | `900dd092-2831-41cd-9ea2-8bb8278161f1` | position=2 (base 0), stable_id emitido |
| ✅ | El profesor define el cuestionario | `POST /api/v1/resources/d822f128-92d7-4597-8117-12a06c157b8e/quiz` | 201 | `49960de5880e` | `f37a57ee-c063-4a4d-a256-80649aed47a3` | quiz creado sobre el recurso d822f128-92d7-4597-8117-1... |
| ✅ | Pregunta 1 con su clave de respuesta | `POST /api/v1/quizzes/a607e55b-5512-443b-89e8-18cc9b7b5ad7/questions` | 201 | `49960de5880e` | `0d0bf95e-e028-47ee-8f8d-292b4cdaa29d` | position=0 |
| ✅ | Pregunta 2 con su clave de respuesta | `POST /api/v1/quizzes/a607e55b-5512-443b-89e8-18cc9b7b5ad7/questions` | 201 | `49960de5880e` | `eeae821d-883a-45d5-b0f9-5595897203a3` | position=1 |
| ✅ | El autor relee el cuestionario con su clave de respuestas | `GET /api/v1/resources/d822f128-92d7-4597-8117-12a06c157b8e/quiz` | 200 | `49960de5880e` | `b6f34d4a-ca09-451a-a445-5bc932fb0320` | 2 pregunta(s) con clave, passing_score=70 |

## 4. Carga multimedia, procesamiento asíncrono y consumo

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | La API firma la URL de carga directa | `POST /api/v1/media/presigned-url` | 200 | `49960de5880e` | `11cc52fc-34c4-47e3-80ba-04e8ef8578df` | method="PUT" content_type="video/mp4" object_key bajo originals/00559ca2-46e1-41f0-b59f-eedf27c6a017/ |
| ✅ | Una extensión no permitida no obtiene firma | `POST /api/v1/media/presigned-url` | 400 | `49960de5880e` | `5b77964c-1f06-43ff-8546-adee3adf56c7` | code="invalid_input" |
| ✅ | Un estudiante no obtiene URL de carga | `POST /api/v1/media/presigned-url` | 403 | `49960de5880e` | `724e4f11-302e-4c39-b850-1b59b4786202` | code="forbidden" |
| ✅ | El navegador sube el original directo al bucket con la URL firmada | `PUT https://storage.googleapis.com/plataforma-mooc-entrega2-media/originals/005...` | 200 | — | — | 431998 bytes aceptados por Cloud Storage sin pasar por la API |
| ✅ | Confirmar la carga registra el objeto y encola el procesamiento | `POST /api/v1/media/uploads/d6a54ae4-608a-4025-983d-77645d33c032/complete` | 202 | `49960de5880e` | `cd39524f-7060-4f37-b5d6-226f05c15b23` | processing_status="pending" object_key registrado |
| ✅ | Confirmar dos veces la misma carga no cambia el estado | `POST /api/v1/media/uploads/d6a54ae4-608a-4025-983d-77645d33c032/complete` | 202 | `49960de5880e` | `4a61e1b1-f6a0-4e82-a9b8-adba8de879ea` | object_key intacto, processing_status="pending" |
| ✅ | El autor obtiene una URL firmada de lectura del original | `GET /api/v1/media/resources/d6a54ae4-608a-4025-983d-77645d33c032/download-url` | 200 | `49960de5880e` | `750b8c1a-c0f3-4a32-8b33-d8c218b9c874` | la URL lleva firma V4 |
| ✅ | El original se lee con la firma y devuelve los bytes subidos | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/originals/005...` | 200 | — | — | Content-Length=431998 frente a 431998 subidos |
| ✅ | El mismo original sin firma es inaccesible | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/originals/005...` | 403 | — | — | el prefijo originals/ no es legible de forma anónima |
| ✅ | El worker de la otra VM transcodifica el original a HLS | `GET /api/v1/units/{unitID}/resources (sondeo)` | — | — | — | processing_status="completed" tras 20s de espera |
| ✅ | El bucket de derivados admite lectura sin firma | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-hls/hls/no-existe-a...` | 404 | — | — | un objeto inexistente responde 404 (bucket público) y no 403 (bucket privado) |
| ✅ | Un reproductor consume el manifiesto HLS sin firma | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-hls/hls/00559ca2-46...` | 200 | — | — | el maestro declara sus variantes por ruta relativa |

## 4b. Documentos e imágenes sin transcodificación

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Recurso de PDF, visible y no obligatorio | `POST /api/v1/units/b25cf01a-a568-4ccd-acd8-e580cc87fdd5/resources` | 201 | `49960de5880e` | `a1a00eb0-a542-4595-b89f-95cfb4202be8` | position=3 (base 0), stable_id emitido |
| ✅ | La API firma la carga del PDF bajo documents/ | `POST /api/v1/media/presigned-url` | 200 | `49960de5880e` | `4a11c970-23d4-4a4e-8ca6-6c1c67ca4a20` | content_type="application/pdf" object_key bajo documents/194792a5-8a96-4397-8026-2686f063258d/ |
| ✅ | El PDF sube directo al bucket con la URL firmada | `PUT https://storage.googleapis.com/plataforma-mooc-entrega2-media/documents/194...` | 200 | — | — | 613 bytes aceptados por Cloud Storage |
| ✅ | Confirmar la carga del PDF encola su procesamiento | `POST /api/v1/media/uploads/89b467f9-9cc8-4003-b30f-e65ea51f6430/complete` | 202 | `49960de5880e` | `b236211a-3916-49a6-b084-4ea68c1f1ecc` | processing_status="pending" |
| ✅ | El PDF queda disponible sin pasar por el transcodificador | `POST /api/v1/media/uploads/89b467f9-9cc8-4003-b30f-e65ea51f6430/complete` | 202 | `49960de5880e` | `b236211a-3916-49a6-b084-4ea68c1f1ecc` | processing_status="completed" tras 10s |
| ✅ | El autor obtiene URL firmada de lectura del PDF | `GET /api/v1/media/resources/89b467f9-9cc8-4003-b30f-e65ea51f6430/download-url` | 200 | `49960de5880e` | `8ce457e2-bfae-46fb-9e2c-1aef09001a20` | la URL lleva firma V4 |
| ✅ | El PDF se lee con la firma y devuelve los bytes subidos | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/documents/194...` | 200 | — | — | Content-Length=613 frente a 613 subidos |
| ✅ | El mismo PDF sin firma es inaccesible | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/documents/194...` | 403 | — | — | el prefijo documents/ no es legible de forma anónima |
| ✅ | Recurso de imagen, visible y no obligatorio | `POST /api/v1/units/b25cf01a-a568-4ccd-acd8-e580cc87fdd5/resources` | 201 | `49960de5880e` | `fa58ce49-1b55-4ee9-b71c-6ad8f7c8cc07` | position=4 (base 0), stable_id emitido |
| ✅ | La API firma la carga del imagen bajo thumbnails/ | `POST /api/v1/media/presigned-url` | 200 | `49960de5880e` | `13aae2f1-8438-4668-b60e-75e384fc3b48` | content_type="image/png" object_key bajo thumbnails/4e2f81c4-3c89-42f0-b3d2-6493d0b69e8e/ |
| ✅ | El imagen sube directo al bucket con la URL firmada | `PUT https://storage.googleapis.com/plataforma-mooc-entrega2-media/thumbnails/4e...` | 200 | — | — | 463 bytes aceptados por Cloud Storage |
| ✅ | Confirmar la carga del imagen encola su procesamiento | `POST /api/v1/media/uploads/8a11b401-1c38-4ee4-b6c8-fb2e671b24e9/complete` | 202 | `49960de5880e` | `e992f9c0-ecb5-4abc-9c38-a3b302cebdb5` | processing_status="pending" |
| ✅ | El imagen queda disponible sin pasar por el transcodificador | `POST /api/v1/media/uploads/8a11b401-1c38-4ee4-b6c8-fb2e671b24e9/complete` | 202 | `49960de5880e` | `e992f9c0-ecb5-4abc-9c38-a3b302cebdb5` | processing_status="completed" tras 10s |
| ✅ | El autor obtiene URL firmada de lectura del imagen | `GET /api/v1/media/resources/8a11b401-1c38-4ee4-b6c8-fb2e671b24e9/download-url` | 200 | `49960de5880e` | `80d0577b-d3ca-429c-a8da-8867baba8372` | la URL lleva firma V4 |
| ✅ | El imagen se lee con la firma y devuelve los bytes subidos | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/thumbnails/4e...` | 200 | — | — | Content-Length=463 frente a 463 subidos |
| ✅ | El mismo imagen sin firma es inaccesible | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/thumbnails/4e...` | 403 | — | — | el prefijo thumbnails/ no es legible de forma anónima |

## 3. Autoría y publicación (cierre)

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Con estructura y criterio de aprobación, el curso se publica | `POST /api/v1/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961/publish` | 200 | `49960de5880e` | `25716b5e-9fb5-4cc9-bccf-f4e5e1c32f8c` | status="published" version=1 |
| ✅ | Un curso publicado es inmutable | `PUT /api/v1/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961` | 409 | `49960de5880e` | `18b7f953-aa8d-433a-9b51-b69cc651891c` | code="course_immutable" |
| ✅ | El rechazo dejó el título intacto | `GET /api/v1/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961` | 200 | `49960de5880e` | `2d633884-999d-4b71-bc3f-2b6eeced4cd5` | title sigue siendo el publicado ("G3 · Recorrido E2E en la...") |
| ✅ | El curso publicado ya figura en el catálogo público | `GET /api/v1/courses?limit=50&search=G3+%C2%B7+Recorrido+E2E+en+la+nube+1793811449` | 200 | `49960de5880e` | `62288015-76db-4c37-b26f-a80a84c0004a` | el catálogo anónimo lo lista |

## 5. Inscripción, quiz idempotente y progreso

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | El estudiante se inscribe en el curso publicado | `POST /api/v1/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961/enrollments` | 200 | `49960de5880e` | `f1c11bf6-90ba-4a1d-8d5f-adb059f90dda` | status="active" sobre el stable_id del curso |
| ✅ | Con la inscripción activa ya lee el contenido | `GET /api/v1/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961/modules` | 200 | `49960de5880e` | `a4f89c64-cfa9-4441-b62d-c09952ebc82c` | 1 módulo(s) visibles donde antes había 403 |
| ✅ | El estudiante ve el cuestionario sin la clave de respuestas | `GET /api/v1/resources/d822f128-92d7-4597-8117-12a06c157b8e/quiz` | 200 | `49960de5880e` | `dc3642ed-5b56-419f-8b26-d17a0af8bc44` | la respuesta no trae questions_with_key ni is_correct |
| ✅ | El progreso arranca en cero sobre los tres recursos obligatorios | `GET /api/v1/progress/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961` | 200 | `49960de5880e` | `3bb62a05-fbe1-4765-8bde-3cec3b3471ed` | 0/3 · 0.00% · aprobado=false |
| ✅ | El estudiante abre el intento y recibe el cuestionario congelado | `POST /api/v1/quizzes/a607e55b-5512-443b-89e8-18cc9b7b5ad7/attempts` | 200 | `49960de5880e` | `d80f4622-7c0f-4666-926a-6f3d066628ec` | attempt=1 con 2 pregunta(s), sin respuestas previas |
| ✅ | El intento congelado no lleva la clave de respuestas | `POST /api/v1/quizzes/a607e55b-5512-443b-89e8-18cc9b7b5ad7/attempts` | 200 | `49960de5880e` | `d80f4622-7c0f-4666-926a-6f3d066628ec` | ninguna opción viene marcada como correcta |
| ✅ | Abrir el intento dos veces no consume dos intentos | `POST /api/v1/quizzes/a607e55b-5512-443b-89e8-18cc9b7b5ad7/attempts` | 200 | `49960de5880e` | `ff064e1e-39b7-49ab-a0c6-b68a062a2c22` | attempt=1 en ambas llamadas |
| ✅ | El estudiante guarda una respuesta parcial sin calificar nada | `PUT /api/v1/quizzes/a607e55b-5512-443b-89e8-18cc9b7b5ad7/attempts/current` | 200 | `49960de5880e` | `6b49c369-7909-4873-bf34-7e1a21220e2d` | 1 de 2 respuestas guardadas |
| ✅ | El avance sobrevive a cerrar el navegador | `GET /api/v1/quizzes/a607e55b-5512-443b-89e8-18cc9b7b5ad7/attempts/current` | 200 | `49960de5880e` | `121dcdd7-1a83-49b9-92f8-b72a395d4899` | 1 respuesta(s) recuperadas del intento en curso |
| ✅ | El estudiante envía el quiz y queda calificado | `POST /api/v1/quizzes/a607e55b-5512-443b-89e8-18cc9b7b5ad7/submissions` | 200 | `49960de5880e` | `294b4640-7628-46ac-a278-8b493fe6c3b1` | score=100 passed=true attempt=1 |
| ✅ | Reenviar con la misma Idempotency-Key no vuelve a calificar | `POST /api/v1/quizzes/a607e55b-5512-443b-89e8-18cc9b7b5ad7/submissions` | 200 | `49960de5880e` | `0d497ab6-273f-4408-8fe0-597acb784d1c` | submission_id idéntico y attempt sigue en 1 |
| ✅ | Solo quedó un envío registrado, no dos | `GET /api/v1/quizzes/a607e55b-5512-443b-89e8-18cc9b7b5ad7/submissions/me` | 200 | `49960de5880e` | `78fed074-b29e-4715-a83b-86b168ab34cf` | 1 envío(s) en el historial del estudiante |
| ✅ | Aprobar el quiz completó su recurso sin latido del cliente | `GET /api/v1/progress/courses/4a4df51e-3cbf-4037-ada0-8ee3b283e961` | 200 | `49960de5880e` | `18f89f1e-79c5-41df-ac50-3870531da140` | 1/3 · 33.33% |
| ✅ | Un latido sobre la lectura avanza el progreso | `POST /api/v1/progress/heartbeat` | 200 | `49960de5880e` | `d272a04f-5d9c-47c6-958a-079f7d4d4721` | 2/3 · 66.67% |
| ✅ | El último recurso obligatorio completa el curso y emite la insignia | `POST /api/v1/progress/heartbeat` | 200 | `49960de5880e` | `f928162e-caf7-44d6-b13a-6bde32fd0a30` | 3/3 · 100.00% · aprobado=true · insignia=true |
| ✅ | Repetir el latido no infla el avance | `POST /api/v1/progress/heartbeat` | 200 | `49960de5880e` | `5aa72436-c497-4886-a1f9-90568477f8b6` | sigue en 3/3 |
| ✅ | El autor no acumula progreso en su propio curso | `POST /api/v1/progress/heartbeat` | 403 | `49960de5880e` | `bb3c347a-7366-495a-a5d2-bc4bb45ccd4b` | code="forbidden": sin inscripción no hay avance |

## 6. Insignia y verificación pública

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | El estudiante lee su propia insignia | `GET /api/v1/badges/f47befa7-703b-4894-8f29-ede0c138be7e` | 200 | `49960de5880e` | `e14a2877-5723-43d8-be86-5db1e5f248b3` | course_stable_id correcto, revoked=false, ETag emitido |
| ✅ | Con el ETag vigente la lectura es condicional | `GET /api/v1/badges/f47befa7-703b-4894-8f29-ede0c138be7e` | 304 | `49960de5880e` | `7dcd4b49-469f-4b2f-bbce-3416561e65cf` | 304 sin cuerpo |
| ✅ | La insignia de otra persona responde 404, no 403 | `GET /api/v1/badges/f47befa7-703b-4894-8f29-ede0c138be7e` | 404 | `49960de5880e` | `ce4a8c9d-8bac-4864-b0d4-d6ba4d190ecb` | code="not_found": el identificador no confirma que exista |
| ✅ | Cualquiera verifica la insignia con el código, sin sesión | `GET /api/v1/badges/verify/fafd74ac-1bfc-4424-bee7-216f8e08832f` | 200 | `49960de5880e` | `fb6a3018-47c3-48c3-92cd-aa6a606fb5e6` | valid=true revoked=false, nombra el curso y no identifica al estudiante |
| ✅ | Un código inexistente no confirma ni niega nada más que el 404 | `GET /api/v1/badges/verify/00000000-0000-0000-0000-000000000000` | 404 | `49960de5880e` | `3b20e3f5-16d0-4437-b8ef-6c06e1656e3e` | code="not_found" |

## 7. Continuidad de sesión, revocación y suspensión entre réplicas (E1)

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Sesión activa atendida por instancia 49960de5880e (intento #1) | `GET /api/v1/auth/sessions` | 200 | `49960de5880e` | `c551fc44-648f-435c-bac5-2888dfc94d2d` | X-Instance-ID="49960de5880e" X-Request-ID="c551fc44-648f-435c-bac5-2888dfc94d2d" |
| ✅ | Peticiones sucesivas de la misma sesión atendidas por instancias distintas sin reautenticar | `GET /api/v1/auth/sessions` | 200 | `49960de5880e` | `7f75b7ae-6370-4dab-b15a-a683ed8187f6` | 1 instancia(s) distinta(s) atendieron la sesión: 49960de5880e=2 (mín. exigido=1) |
| ✅ | Sesión activa atendida por instancia 49960de5880e (intento #1) | `GET /api/v1/auth/sessions` | 200 | `49960de5880e` | `2f3bf8c6-82de-4897-9163-2da301c78998` | X-Instance-ID="49960de5880e" X-Request-ID="2f3bf8c6-82de-4897-9163-2da301c78998" |
| ✅ | La sesión de profesor2 opera y calienta caché en las réplicas antes del logout | `GET /api/v1/auth/sessions` | 200 | `49960de5880e` | `930dde7f-32a7-40fa-86ef-caaf101c1cf6` | 200 en 1 instancia(s): 49960de5880e=2 |
| ✅ | Logout revoca la sesión en la instancia que lo recibe y en la caché compartida | `POST /api/v1/auth/logout` | 204 | `49960de5880e` | `b7ccd53b-e142-46c4-8311-b92efa0274bf` | 204 procesado por X-Instance-ID="49960de5880e" |
| ✅ | Rechazo inmediato (401) en instancia 49960de5880e (intento #1) | `GET /api/v1/auth/sessions` | 401 | `49960de5880e` | `2786e0ef-2c99-4e35-b633-a98a52bc44c6` | code="unauthorized" X-Instance-ID="49960de5880e" |
| ✅ | El logout invalida la sesión de inmediato en todas las instancias en la siguiente petición | `GET /api/v1/auth/sessions` | 401 | `49960de5880e` | `1783e42d-9901-49e4-87da-683a1fe3d7a7` | 401 (code="unauthorized") en el 100% de 2 peticiones repartidas en: 49960de5880e=2 |
| ✅ | El administrador asegura que estudiante2 está activo antes de la prueba | `PATCH /api/v1/admin/users/c0000000-0000-0000-0000-000000000002/status` | 200 | `49960de5880e` | `03951592-ba33-4059-9683-837d76410095` | status="active" |
| ✅ | Inicia sesión estudiante (estudiante2) | `POST /api/v1/auth/login` | 200 | `49960de5880e` | `ca1d2e0a-02ab-48dd-bd79-9cb84166d758` | role="estudiante" status="active", sesión emitida |
| ✅ | Sesión activa atendida por instancia 49960de5880e (intento #1) | `GET /api/v1/auth/sessions` | 200 | `49960de5880e` | `37356771-d36d-402c-8fb0-dcac8d9c4eef` | X-Instance-ID="49960de5880e" X-Request-ID="37356771-d36d-402c-8fb0-dcac8d9c4eef" |
| ✅ | La sesión de estudiante2 es atendida por las réplicas antes de suspender | `GET /api/v1/auth/sessions` | 200 | `49960de5880e` | `89a08ed8-e0d3-4207-ab3c-82768e891ac5` | 200 en 1 instancia(s): 49960de5880e=2 |
| ✅ | El administrador suspende la cuenta de estudiante2 | `PATCH /api/v1/admin/users/c0000000-0000-0000-0000-000000000002/status` | 200 | `49960de5880e` | `4542111a-ecc6-4505-aa24-8607ac8bc1dd` | status="suspended" procesado por X-Instance-ID="49960de5880e" |
| ✅ | Rechazo inmediato (401) en instancia 49960de5880e (intento #1) | `GET /api/v1/auth/sessions` | 401 | `49960de5880e` | `e48fe870-e03d-415f-9cd2-3fb3d16f0f0c` | code="unauthorized" X-Instance-ID="49960de5880e" |
| ✅ | Suspender al usuario corta sus sesiones en todas las instancias en la siguiente petición | `GET /api/v1/auth/sessions` | 401 | `49960de5880e` | `47925f23-6dc3-4d7f-8c71-824163ddaaaf` | 401 (code="unauthorized") en el 100% de 2 peticiones repartidas en: 49960de5880e=2 |
| ✅ | El administrador reactiva estudiante2 para dejar la base idempotente | `PATCH /api/v1/admin/users/c0000000-0000-0000-0000-000000000002/status` | 200 | `49960de5880e` | `7c154adc-ee99-4d6a-9d13-b196abae5689` | status="active" restaurado |
| ✅ | La sesión previa a la suspensión sigue revocada incluso tras reactivar la cuenta | `GET /api/v1/auth/sessions` | 401 | `49960de5880e` | `7ae8dca9-151e-4b3e-8ed4-5939bcf6d7f7` | code="unauthorized" en X-Instance-ID="49960de5880e" |

## 8. Reconciliación de trabajos multimedia aceptados (E1)

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Todos los trabajos aceptados alcanzaron su estado final sin pérdidas ni duplicados | `reconciliación POST /complete (202) -> worker -> PostgreSQL + GCS` | — | — | — | 3/3 trabajos reconciliados en completed [video:completed(d6a54ae4-608a-4025-983d-7...), pdf:completed(89b467f9-9cc8-4003-b30f-e...), image:completed(8a11b401-1c38-4ee4-b6c8-f...)] |

## Reconciliación de trabajos multimedia aceptados

| | Recurso (`id` / `stable_id`) | Tipo | `object_key` | Instancia `/complete` | `X-Request-ID` | Estado final | Duración | Derivados y verificación |
| :---: | :--- | :---: | :--- | :---: | :---: | :---: | ---: | :--- |
| ✅ | `d6a54ae4-608a-4025-983d-7...` (`00559ca2-46e1-41f0-b59f-e...`) | `video` | `originals/00559ca2-46e1-41f0-b59f-eedf27c6a017/49ec3f74-3872-405d-84e0-661aa6ffc552.mp4` | `49960de5880e` | `cd39524f-7060-4f37-b5d6-226f05c15b23` | `completed` | 20s | HLS hls/00559ca2-46e1-41f0-b59f-eedf27c6a017/master.m3u8 (200=true) |
| ✅ | `89b467f9-9cc8-4003-b30f-e...` (`194792a5-8a96-4397-8026-2...`) | `pdf` | `documents/194792a5-8a96-4397-8026-2686f063258d/7dda4bcf-3fed-4adc-ba6f-ef75d09d19e3.pdf` | `49960de5880e` | `b236211a-3916-49a6-b084-4ea68c1f1ecc` | `completed` | 10s | objeto verificado (613 B, lectura=true) |
| ✅ | `8a11b401-1c38-4ee4-b6c8-f...` (`4e2f81c4-3c89-42f0-b3d2-6...`) | `image` | `thumbnails/4e2f81c4-3c89-42f0-b3d2-6493d0b69e8e/9a548127-01ad-47ed-8a7e-50afd120555c.png` | `49960de5880e` | `e992f9c0-ecb5-4abc-9c38-a3b302cebdb5` | `completed` | 10s | objeto verificado (463 B, lectura=true) |
