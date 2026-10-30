# G3 / E1 — Resultados de la corrida E2E en la nube

Generado por `scripts/e2e_cloud` el 2026-10-30 15:28:41 UTC contra `https://136.68.55.254.sslip.io`.
No editar a mano: se reescribe en cada corrida.

Pasos verificados: 108 · fallos: 0 · fuera de alcance: 1 · duración: 43s

## Instancias de API observadas (`X-Instance-ID`)

| Instancia (`X-Instance-ID`) | Peticiones atendidas |
| :--- | ---: |
| `8f34dc8a959f` | 51 |
| `d2c891965b86` | 44 |


## 0. Entorno desplegado

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | La API responde y alcanza la base administrada | `GET /api/v1/health` | 200 | `d2c891965b86` | `17b2c6f6-1b4a-4602-89da-c9ce891e26c6` | status="pass" database="up" |
| ✅ | La sonda de liveness responde sin consultar dependencias | `GET /api/v1/live` | 200 | `8f34dc8a959f` | `afdc38c5-ff8f-4c7c-ae1b-55334f8dd765` | 200 mientras el proceso sirva, para que un parpadeo de la base no recree las réplicas |
| ✅ | La sonda de readiness declara que la réplica puede atender | `GET /api/v1/ready` | 200 | `8f34dc8a959f` | `392b483b-7b4b-4e97-8cf0-fae965ebe727` | 200 con la base alcanzable |
| ✅ | La sonda pública no revela dependencias ni topología | `GET /api/v1/ready` | 200 | `8f34dc8a959f` | `392b483b-7b4b-4e97-8cf0-fae965ebe727` | cuerpo={"status":"pass"} |
| ✅ | Cada respuesta dice qué réplica la atendió | `GET /api/v1/ready` | 200 | `8f34dc8a959f` | `392b483b-7b4b-4e97-8cf0-fae965ebe727` | X-Instance-ID="8f34dc8a959f" |

## 1. Registro, verificación de correo y login

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Registro de una cuenta nueva (el 201 implica que el relevo SMTP aceptó el correo) | `POST /api/v1/auth/register` | 201 | `d2c891965b86` | `7354b3d7-79af-4594-83a5-cea15fc285e3` | role="estudiante" status="pending_verification" |
| ✅ | La cuenta sin verificar no puede iniciar sesión | `POST /api/v1/auth/login` | 403 | `8f34dc8a959f` | `ce33c3d1-199b-4598-b7cf-9f8c734a740d` | code="email_not_verified", sin token emitido |
| ✅ | Un enlace de verificación inválido se rechaza sin distinguir la causa | `GET /api/v1/auth/verify?token=token-inexistente-000000` | 400 | `8f34dc8a959f` | `98f2b573-0275-416c-bb45-cfede9f5ab15` | code="invalid_token" |
| ✅ | Reenvío del enlace de verificación (segunda entrega real por SMTP) | `POST /api/v1/auth/verify/resend` | 202 | `8f34dc8a959f` | `53b3f1d3-6391-418c-a94e-2663a9d42149` | el código de estado es la verificación |
| ℹ️ | Activación con el token recibido por correo | `GET /api/v1/auth/verify?token=…` | — | — | — | el token solo viaja por correo, así que esta pata no se conduce desde aquí; ciclo completo verificado a mano en la evidencia de F1 |
| ✅ | Inicia sesión administrador (admin) | `POST /api/v1/auth/login` | 200 | `8f34dc8a959f` | `b3096b4a-d504-4ccf-81c2-aeaf934cf3ad` | role="administrador" status="active", sesión emitida |
| ✅ | Inicia sesión profesor (profesor1) | `POST /api/v1/auth/login` | 200 | `d2c891965b86` | `75caa10b-51ef-4215-8538-ff857deae9d0` | role="profesor" status="active", sesión emitida |
| ✅ | Inicia sesión profesor (profesor2) | `POST /api/v1/auth/login` | 200 | `d2c891965b86` | `eed49f3b-91f2-4522-995d-f79a0475568c` | role="profesor" status="active", sesión emitida |
| ✅ | Inicia sesión estudiante (estudiante1) | `POST /api/v1/auth/login` | 200 | `8f34dc8a959f` | `bb0f5691-41ea-4509-8fa5-cad8d667e618` | role="estudiante" status="active", sesión emitida |

## 2. Roles, propiedad y accesos denegados

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Un estudiante no puede crear cursos | `POST /api/v1/courses` | 403 | `d2c891965b86` | `ea7405bf-e390-4624-8f42-07a791ae85bb` | code="forbidden" |
| ✅ | El profesor crea el curso en borrador | `POST /api/v1/courses` | 201 | `d2c891965b86` | `e2765c05-f973-4a70-b5d9-3632178209a3` | status="draft" version=1 stable_id emitido |
| ✅ | El borrador no aparece en el catálogo público | `GET /api/v1/courses?limit=50&search=G3+%C2%B7+Recorrido+E2E+en+la+nube+1793374123` | 200 | `8f34dc8a959f` | `ff80e1fc-d53c-4002-8331-343073192c7e` | el catálogo responde sin sesión y no lista el borrador |
| ✅ | Sin sesión, el contenido del curso responde 401 | `GET /api/v1/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0/modules` | 401 | `8f34dc8a959f` | `e8c72be9-626b-445b-a73a-2cf237c63035` | code="unauthorized" |
| ✅ | Con sesión pero sin inscripción, responde 403 | `GET /api/v1/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0/modules` | 403 | `8f34dc8a959f` | `564e2ae7-4e23-41c3-a73f-4a3d094622ab` | code="forbidden" |
| ✅ | El autor lee su propio borrador sin inscribirse | `GET /api/v1/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0/modules` | 200 | `d2c891965b86` | `26803262-9a1e-4a1e-9c9f-638bdfead1eb` | la lista responde al autor |
| ✅ | El administrador lee el borrador ajeno | `GET /api/v1/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0/modules` | 200 | `8f34dc8a959f` | `b31347d8-5bb4-4bf1-a997-599acafe4687` | la lista responde al administrador |
| ✅ | Otro profesor no puede editar un curso que no es suyo | `POST /api/v1/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0/modules` | 403 | `d2c891965b86` | `7ce9c2a7-52a2-40c4-88b6-e86a4365914d` | code="forbidden", la propiedad manda sobre el rol |

## 3. Autoría y publicación

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Publicar un curso vacío acumula los errores de validación | `POST /api/v1/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0/publish` | 422 | `d2c891965b86` | `72b2733c-5a72-42aa-8043-a87d0822c33d` | code="validation_failed" campos=[structure approval_criteria] |
| ✅ | El intento fallido no publicó nada | `GET /api/v1/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0` | 200 | `d2c891965b86` | `c0a5d03e-d1bf-4170-9834-2e8fc1232e78` | status="draft" |
| ✅ | El profesor crea el módulo | `POST /api/v1/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0/modules` | 201 | `8f34dc8a959f` | `8005b3c9-6e09-4a31-a4e2-baf6546ea433` | position=0 (base 0), stable_id emitido |
| ✅ | El profesor crea la unidad | `POST /api/v1/modules/65052547-d2b0-4449-8ec6-cbdc93e51df4/units` | 201 | `d2c891965b86` | `de547e8a-7180-4184-b70e-9e6d83e24fac` | position=0 (base 0), stable_id emitido |
| ✅ | Recurso de video, obligatorio y visible | `POST /api/v1/units/c4277bb2-76bd-4000-8981-cba08c44e608/resources` | 201 | `8f34dc8a959f` | `40722b4a-e4be-4e51-b86f-c4ce069667e5` | position=0 (base 0), stable_id emitido |
| ✅ | Recurso de texto, obligatorio y visible | `POST /api/v1/units/c4277bb2-76bd-4000-8981-cba08c44e608/resources` | 201 | `8f34dc8a959f` | `c6f63b83-a4bc-49ca-b470-dae0ce2ec964` | position=1 (base 0), stable_id emitido |
| ✅ | Recurso de quiz, obligatorio y visible | `POST /api/v1/units/c4277bb2-76bd-4000-8981-cba08c44e608/resources` | 201 | `d2c891965b86` | `50a3ab84-6072-4cce-9656-2d952fa3bcd8` | position=2 (base 0), stable_id emitido |
| ✅ | El profesor define el cuestionario | `POST /api/v1/resources/ccc6efd5-f257-42cd-a862-2dfbdb127bde/quiz` | 201 | `d2c891965b86` | `05e573e2-d13c-4a5d-998c-068e1f0151ba` | quiz creado sobre el recurso ccc6efd5-f257-42cd-a862-2... |
| ✅ | Pregunta 1 con su clave de respuesta | `POST /api/v1/quizzes/c1ccf8a7-719b-4da4-983d-4a2cb0c62108/questions` | 201 | `8f34dc8a959f` | `477c6b15-d671-4f1e-8219-fb4145297bbc` | position=0 |
| ✅ | Pregunta 2 con su clave de respuesta | `POST /api/v1/quizzes/c1ccf8a7-719b-4da4-983d-4a2cb0c62108/questions` | 201 | `d2c891965b86` | `4b9c3809-fd7a-4c00-8be9-bfa902648f43` | position=1 |
| ✅ | El autor relee el cuestionario con su clave de respuestas | `GET /api/v1/resources/ccc6efd5-f257-42cd-a862-2dfbdb127bde/quiz` | 200 | `d2c891965b86` | `0d137307-47f4-4e4c-adcc-3d375162e899` | 2 pregunta(s) con clave, passing_score=70 |

## 4. Carga multimedia, procesamiento asíncrono y consumo

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | La API firma la URL de carga directa | `POST /api/v1/media/presigned-url` | 200 | `d2c891965b86` | `224b6c67-8d6b-486a-be2c-fb0d5712392d` | method="PUT" content_type="video/mp4" object_key bajo originals/5822d55a-4e50-40c5-b3e4-4afd5aff8ddc/ |
| ✅ | Una extensión no permitida no obtiene firma | `POST /api/v1/media/presigned-url` | 400 | `8f34dc8a959f` | `f4108f68-7759-464d-a5a9-def09236f04f` | code="invalid_input" |
| ✅ | Un estudiante no obtiene URL de carga | `POST /api/v1/media/presigned-url` | 403 | `8f34dc8a959f` | `545cc0fe-83c4-4dd7-8698-de6f4a0cf98d` | code="forbidden" |
| ✅ | El navegador sube el original directo al bucket con la URL firmada | `PUT https://storage.googleapis.com/plataforma-mooc-entrega2-media/originals/582...` | 200 | — | — | 179691 bytes aceptados por Cloud Storage sin pasar por la API |
| ✅ | Confirmar la carga registra el objeto y encola el procesamiento | `POST /api/v1/media/uploads/8e00400f-9eb0-42b6-b396-568d45cf53e1/complete` | 202 | `8f34dc8a959f` | `cf6536cc-9875-440c-bbe3-36c548a8388e` | processing_status="pending" object_key registrado |
| ✅ | Confirmar dos veces la misma carga no cambia el estado | `POST /api/v1/media/uploads/8e00400f-9eb0-42b6-b396-568d45cf53e1/complete` | 202 | `8f34dc8a959f` | `8f9b026d-2b54-4706-8c63-60acfcad7aeb` | object_key intacto, processing_status="pending" |
| ✅ | El autor obtiene una URL firmada de lectura del original | `GET /api/v1/media/resources/8e00400f-9eb0-42b6-b396-568d45cf53e1/download-url` | 200 | `d2c891965b86` | `6f512fb2-e576-443e-b44b-1cf864208fba` | la URL lleva firma V4 |
| ✅ | El original se lee con la firma y devuelve los bytes subidos | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/originals/582...` | 200 | — | — | Content-Length=179691 frente a 179691 subidos |
| ✅ | El mismo original sin firma es inaccesible | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/originals/582...` | 403 | — | — | el prefijo originals/ no es legible de forma anónima |
| ✅ | El worker de la otra VM transcodifica el original a HLS | `GET /api/v1/units/{unitID}/resources (sondeo)` | — | — | — | processing_status="completed" tras 10s de espera |
| ✅ | El bucket de derivados admite lectura sin firma | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-hls/hls/no-existe-a...` | 404 | — | — | un objeto inexistente responde 404 (bucket público) y no 403 (bucket privado) |
| ✅ | Un reproductor consume el manifiesto HLS sin firma | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-hls/hls/5822d55a-4e...` | 200 | — | — | el maestro declara sus variantes por ruta relativa |

## 4b. Documentos e imágenes sin transcodificación

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Recurso de PDF, visible y no obligatorio | `POST /api/v1/units/c4277bb2-76bd-4000-8981-cba08c44e608/resources` | 201 | `8f34dc8a959f` | `08c85ae8-c3e3-4546-98eb-b46e05ccd45f` | position=3 (base 0), stable_id emitido |
| ✅ | La API firma la carga del PDF bajo documents/ | `POST /api/v1/media/presigned-url` | 200 | `d2c891965b86` | `7863c5f9-0462-4628-8ca2-a70480dec2dd` | content_type="application/pdf" object_key bajo documents/fe10a3ec-1284-4bf7-af84-5d52be5d85db/ |
| ✅ | El PDF sube directo al bucket con la URL firmada | `PUT https://storage.googleapis.com/plataforma-mooc-entrega2-media/documents/fe1...` | 200 | — | — | 613 bytes aceptados por Cloud Storage |
| ✅ | Confirmar la carga del PDF encola su procesamiento | `POST /api/v1/media/uploads/ab4a246f-0cbd-47c4-b43b-aee7b75e634d/complete` | 202 | `8f34dc8a959f` | `c57b6481-6b80-4fb4-862d-8f48a17e31ee` | processing_status="pending" |
| ✅ | El PDF queda disponible sin pasar por el transcodificador | `POST /api/v1/media/uploads/ab4a246f-0cbd-47c4-b43b-aee7b75e634d/complete` | 202 | `8f34dc8a959f` | `c57b6481-6b80-4fb4-862d-8f48a17e31ee` | processing_status="completed" tras 10s |
| ✅ | El autor obtiene URL firmada de lectura del PDF | `GET /api/v1/media/resources/ab4a246f-0cbd-47c4-b43b-aee7b75e634d/download-url` | 200 | `d2c891965b86` | `00218a0d-02ab-458d-8509-410a81d504ee` | la URL lleva firma V4 |
| ✅ | El PDF se lee con la firma y devuelve los bytes subidos | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/documents/fe1...` | 200 | — | — | Content-Length=613 frente a 613 subidos |
| ✅ | El mismo PDF sin firma es inaccesible | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/documents/fe1...` | 403 | — | — | el prefijo documents/ no es legible de forma anónima |
| ✅ | Recurso de imagen, visible y no obligatorio | `POST /api/v1/units/c4277bb2-76bd-4000-8981-cba08c44e608/resources` | 201 | `8f34dc8a959f` | `2fd6431f-7873-4764-a53f-d66dc4965342` | position=4 (base 0), stable_id emitido |
| ✅ | La API firma la carga del imagen bajo thumbnails/ | `POST /api/v1/media/presigned-url` | 200 | `d2c891965b86` | `a1ef5da4-dce9-4c60-abea-37cfa9debd91` | content_type="image/png" object_key bajo thumbnails/ce651875-375e-4347-8a2b-2baabd6e8d87/ |
| ✅ | El imagen sube directo al bucket con la URL firmada | `PUT https://storage.googleapis.com/plataforma-mooc-entrega2-media/thumbnails/ce...` | 200 | — | — | 463 bytes aceptados por Cloud Storage |
| ✅ | Confirmar la carga del imagen encola su procesamiento | `POST /api/v1/media/uploads/ae765358-2197-483a-92b2-128e2f7e9858/complete` | 202 | `8f34dc8a959f` | `0b8a6761-8f1d-4a5f-89c4-ddfd0cd8bc0e` | processing_status="pending" |
| ✅ | El imagen queda disponible sin pasar por el transcodificador | `POST /api/v1/media/uploads/ae765358-2197-483a-92b2-128e2f7e9858/complete` | 202 | `8f34dc8a959f` | `0b8a6761-8f1d-4a5f-89c4-ddfd0cd8bc0e` | processing_status="completed" tras 10s |
| ✅ | El autor obtiene URL firmada de lectura del imagen | `GET /api/v1/media/resources/ae765358-2197-483a-92b2-128e2f7e9858/download-url` | 200 | `8f34dc8a959f` | `e2617f7a-2720-42b0-b80f-6e4a30f00741` | la URL lleva firma V4 |
| ✅ | El imagen se lee con la firma y devuelve los bytes subidos | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/thumbnails/ce...` | 200 | — | — | Content-Length=463 frente a 463 subidos |
| ✅ | El mismo imagen sin firma es inaccesible | `GET https://storage.googleapis.com/plataforma-mooc-entrega2-media/thumbnails/ce...` | 403 | — | — | el prefijo thumbnails/ no es legible de forma anónima |

## 3. Autoría y publicación (cierre)

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Con estructura y criterio de aprobación, el curso se publica | `POST /api/v1/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0/publish` | 200 | `d2c891965b86` | `93e16e90-7714-42f5-b4ce-f2524707fcf4` | status="published" version=1 |
| ✅ | Un curso publicado es inmutable | `PUT /api/v1/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0` | 409 | `8f34dc8a959f` | `1dd34cb4-be92-45b1-918d-24d22e15901e` | code="course_immutable" |
| ✅ | El rechazo dejó el título intacto | `GET /api/v1/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0` | 200 | `d2c891965b86` | `dd4af2f6-49cd-4e04-8504-bdd3078206c3` | title sigue siendo el publicado ("G3 · Recorrido E2E en la...") |
| ✅ | El curso publicado ya figura en el catálogo público | `GET /api/v1/courses?limit=50&search=G3+%C2%B7+Recorrido+E2E+en+la+nube+1793374123` | 200 | `d2c891965b86` | `ceed6540-0f57-405e-a595-7ea5d3d28a44` | el catálogo anónimo lo lista |

## 5. Inscripción, quiz idempotente y progreso

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | El estudiante se inscribe en el curso publicado | `POST /api/v1/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0/enrollments` | 200 | `8f34dc8a959f` | `5a8da391-5728-44e0-a4b0-e487f87ea6f5` | status="active" sobre el stable_id del curso |
| ✅ | Con la inscripción activa ya lee el contenido | `GET /api/v1/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0/modules` | 200 | `d2c891965b86` | `4c6deed7-4343-4c9f-baff-af073cf79dcd` | 1 módulo(s) visibles donde antes había 403 |
| ✅ | El estudiante ve el cuestionario sin la clave de respuestas | `GET /api/v1/resources/ccc6efd5-f257-42cd-a862-2dfbdb127bde/quiz` | 200 | `8f34dc8a959f` | `d61cd70b-4143-4d9f-8ce6-625b9c436ece` | la respuesta no trae questions_with_key ni is_correct |
| ✅ | El progreso arranca en cero sobre los tres recursos obligatorios | `GET /api/v1/progress/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0` | 200 | `8f34dc8a959f` | `fce45441-222e-4791-891a-4a33bacafdb1` | 0/3 · 0.00% · aprobado=false |
| ✅ | El estudiante abre el intento y recibe el cuestionario congelado | `POST /api/v1/quizzes/c1ccf8a7-719b-4da4-983d-4a2cb0c62108/attempts` | 200 | `d2c891965b86` | `8053016e-3777-4b52-906d-07a8031c599f` | attempt=1 con 2 pregunta(s), sin respuestas previas |
| ✅ | El intento congelado no lleva la clave de respuestas | `POST /api/v1/quizzes/c1ccf8a7-719b-4da4-983d-4a2cb0c62108/attempts` | 200 | `d2c891965b86` | `8053016e-3777-4b52-906d-07a8031c599f` | ninguna opción viene marcada como correcta |
| ✅ | Abrir el intento dos veces no consume dos intentos | `POST /api/v1/quizzes/c1ccf8a7-719b-4da4-983d-4a2cb0c62108/attempts` | 200 | `8f34dc8a959f` | `42bb18e4-e964-45ed-9e16-0845795de85f` | attempt=1 en ambas llamadas |
| ✅ | El estudiante guarda una respuesta parcial sin calificar nada | `PUT /api/v1/quizzes/c1ccf8a7-719b-4da4-983d-4a2cb0c62108/attempts/current` | 200 | `8f34dc8a959f` | `a4f240e1-8f2a-4bb6-bb97-30f2b65c5d52` | 1 de 2 respuestas guardadas |
| ✅ | El avance sobrevive a cerrar el navegador | `GET /api/v1/quizzes/c1ccf8a7-719b-4da4-983d-4a2cb0c62108/attempts/current` | 200 | `d2c891965b86` | `6570d480-01c0-44e1-aa4f-63a1668574de` | 1 respuesta(s) recuperadas del intento en curso |
| ✅ | El estudiante envía el quiz y queda calificado | `POST /api/v1/quizzes/c1ccf8a7-719b-4da4-983d-4a2cb0c62108/submissions` | 200 | `8f34dc8a959f` | `74c9129d-fbe0-4c46-b629-38ecf08623b1` | score=100 passed=true attempt=1 |
| ✅ | Reenviar con la misma Idempotency-Key no vuelve a calificar | `POST /api/v1/quizzes/c1ccf8a7-719b-4da4-983d-4a2cb0c62108/submissions` | 200 | `d2c891965b86` | `999e21df-6ccd-49f0-b373-2c6d92970d15` | submission_id idéntico y attempt sigue en 1 |
| ✅ | Solo quedó un envío registrado, no dos | `GET /api/v1/quizzes/c1ccf8a7-719b-4da4-983d-4a2cb0c62108/submissions/me` | 200 | `d2c891965b86` | `3fc576f6-7be7-4305-9273-4f292d7580d9` | 1 envío(s) en el historial del estudiante |
| ✅ | Aprobar el quiz completó su recurso sin latido del cliente | `GET /api/v1/progress/courses/8fba94bd-7aaa-4e78-a3ef-45f2428c69c0` | 200 | `8f34dc8a959f` | `bc830f81-898f-46ee-b940-ef3a258c3167` | 1/3 · 33.33% |
| ✅ | Un latido sobre la lectura avanza el progreso | `POST /api/v1/progress/heartbeat` | 200 | `d2c891965b86` | `748e9718-69ee-4902-8312-6c58a485a8ca` | 2/3 · 66.67% |
| ✅ | El último recurso obligatorio completa el curso y emite la insignia | `POST /api/v1/progress/heartbeat` | 200 | `8f34dc8a959f` | `58ab2522-43eb-4209-8934-61fd527ff910` | 3/3 · 100.00% · aprobado=true · insignia=true |
| ✅ | Repetir el latido no infla el avance | `POST /api/v1/progress/heartbeat` | 200 | `8f34dc8a959f` | `24ef4687-6c5f-400a-8407-db97822b154e` | sigue en 3/3 |
| ✅ | El autor no acumula progreso en su propio curso | `POST /api/v1/progress/heartbeat` | 403 | `8f34dc8a959f` | `653f08c3-5bba-494a-a22e-e0671a68a714` | code="forbidden": sin inscripción no hay avance |

## 6. Insignia y verificación pública

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | El estudiante lee su propia insignia | `GET /api/v1/badges/dbc7fffa-b0b8-42e4-b542-6bf997a488ff` | 200 | `d2c891965b86` | `2740c92d-ab07-4f29-9d1b-ec945b2a3a1a` | course_stable_id correcto, revoked=false, ETag emitido |
| ✅ | Con el ETag vigente la lectura es condicional | `GET /api/v1/badges/dbc7fffa-b0b8-42e4-b542-6bf997a488ff` | 304 | `d2c891965b86` | `b918a58e-909e-4969-ac1f-ca4e5bc5b138` | 304 sin cuerpo |
| ✅ | La insignia de otra persona responde 404, no 403 | `GET /api/v1/badges/dbc7fffa-b0b8-42e4-b542-6bf997a488ff` | 404 | `8f34dc8a959f` | `86f4d95d-e853-4300-92c8-c30a813cea66` | code="not_found": el identificador no confirma que exista |
| ✅ | Cualquiera verifica la insignia con el código, sin sesión | `GET /api/v1/badges/verify/e958e6ba-55a1-4892-83c4-d187eb9df2e8` | 200 | `d2c891965b86` | `86c9808d-a97e-46d1-b84b-33719ef70af3` | valid=true revoked=false, nombra el curso y no identifica al estudiante |
| ✅ | Un código inexistente no confirma ni niega nada más que el 404 | `GET /api/v1/badges/verify/00000000-0000-0000-0000-000000000000` | 404 | `8f34dc8a959f` | `bc6f7a54-fef7-4b4b-af9b-f4d757362e21` | code="not_found" |

## 7. Continuidad de sesión, revocación y suspensión entre réplicas (E1)

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Sesión activa atendida por instancia d2c891965b86 (intento #1) | `GET /api/v1/auth/sessions` | 200 | `d2c891965b86` | `eb2510cd-29d4-409b-9d38-65880023bb5b` | X-Instance-ID="d2c891965b86" X-Request-ID="eb2510cd-29d4-409b-9d38-65880023bb5b" |
| ✅ | Sesión activa atendida por instancia 8f34dc8a959f (intento #2) | `GET /api/v1/auth/sessions` | 200 | `8f34dc8a959f` | `89220441-3214-4d7a-8cc9-abcfca135fbe` | X-Instance-ID="8f34dc8a959f" X-Request-ID="89220441-3214-4d7a-8cc9-abcfca135fbe" |
| ✅ | Peticiones sucesivas de la misma sesión atendidas por instancias distintas sin reautenticar | `GET /api/v1/auth/sessions` | 200 | `d2c891965b86` | `2da98cb7-eeac-423b-861c-656e34e779e5` | 2 instancia(s) distinta(s) atendieron la sesión: 8f34dc8a959f=2, d2c891965b86=2 (mín. exigido=2) |
| ✅ | Sesión activa atendida por instancia 8f34dc8a959f (intento #1) | `GET /api/v1/auth/sessions` | 200 | `8f34dc8a959f` | `4500a692-7aae-41af-a535-70842307dbe5` | X-Instance-ID="8f34dc8a959f" X-Request-ID="4500a692-7aae-41af-a535-70842307dbe5" |
| ✅ | Sesión activa atendida por instancia d2c891965b86 (intento #2) | `GET /api/v1/auth/sessions` | 200 | `d2c891965b86` | `f33918e4-520f-48e0-a72b-dbab48af7c51` | X-Instance-ID="d2c891965b86" X-Request-ID="f33918e4-520f-48e0-a72b-dbab48af7c51" |
| ✅ | La sesión de profesor2 opera y calienta caché en las réplicas antes del logout | `GET /api/v1/auth/sessions` | 200 | `8f34dc8a959f` | `770ad460-bbe3-426e-ae8a-acbc2d46ce66` | 200 en 2 instancia(s): 8f34dc8a959f=3, d2c891965b86=1 |
| ✅ | Logout revoca la sesión en la instancia que lo recibe y en la caché compartida | `POST /api/v1/auth/logout` | 204 | `d2c891965b86` | `d0e801cc-d4a8-405b-aef0-c22499f41000` | 204 procesado por X-Instance-ID="d2c891965b86" |
| ✅ | Rechazo inmediato (401) en instancia d2c891965b86 (intento #1) | `GET /api/v1/auth/sessions` | 401 | `d2c891965b86` | `1ca64ff1-98bb-4095-bd66-29d9cb9313b1` | code="unauthorized" X-Instance-ID="d2c891965b86" |
| ✅ | Rechazo inmediato (401) en instancia 8f34dc8a959f (intento #3) | `GET /api/v1/auth/sessions` | 401 | `8f34dc8a959f` | `4c0d41ce-d54a-421b-844c-cde9b6174320` | code="unauthorized" X-Instance-ID="8f34dc8a959f" |
| ✅ | El logout invalida la sesión de inmediato en todas las instancias en la siguiente petición | `GET /api/v1/auth/sessions` | 401 | `8f34dc8a959f` | `322a5d95-1636-4518-936d-ac59b85709d3` | 401 (code="unauthorized") en el 100% de 4 peticiones repartidas en: 8f34dc8a959f=2, d2c891965b86=2 |
| ✅ | El administrador asegura que estudiante2 está activo antes de la prueba | `PATCH /api/v1/admin/users/c0000000-0000-0000-0000-000000000002/status` | 200 | `8f34dc8a959f` | `a06a23fb-a54b-4df3-bca7-ffb6bf21eb1c` | status="active" |
| ✅ | Inicia sesión estudiante (estudiante2) | `POST /api/v1/auth/login` | 200 | `d2c891965b86` | `a8bdc8f2-1459-4672-ba96-09ce14f738c4` | role="estudiante" status="active", sesión emitida |
| ✅ | Sesión activa atendida por instancia d2c891965b86 (intento #1) | `GET /api/v1/auth/sessions` | 200 | `d2c891965b86` | `d54a4067-fa84-4be7-8940-59f57d483b6a` | X-Instance-ID="d2c891965b86" X-Request-ID="d54a4067-fa84-4be7-8940-59f57d483b6a" |
| ✅ | Sesión activa atendida por instancia 8f34dc8a959f (intento #4) | `GET /api/v1/auth/sessions` | 200 | `8f34dc8a959f` | `e3bb680f-0d5d-4527-b295-540f38db567b` | X-Instance-ID="8f34dc8a959f" X-Request-ID="e3bb680f-0d5d-4527-b295-540f38db567b" |
| ✅ | La sesión de estudiante2 es atendida por las réplicas antes de suspender | `GET /api/v1/auth/sessions` | 200 | `8f34dc8a959f` | `e3bb680f-0d5d-4527-b295-540f38db567b` | 200 en 2 instancia(s): 8f34dc8a959f=1, d2c891965b86=3 |
| ✅ | El administrador suspende la cuenta de estudiante2 | `PATCH /api/v1/admin/users/c0000000-0000-0000-0000-000000000002/status` | 200 | `d2c891965b86` | `dffdb19c-4892-4989-a773-cd8dd343af70` | status="suspended" procesado por X-Instance-ID="d2c891965b86" |
| ✅ | Rechazo inmediato (401) en instancia d2c891965b86 (intento #1) | `GET /api/v1/auth/sessions` | 401 | `d2c891965b86` | `d49061b3-faf6-40bb-bf08-f2a329279cd3` | code="unauthorized" X-Instance-ID="d2c891965b86" |
| ✅ | Rechazo inmediato (401) en instancia 8f34dc8a959f (intento #2) | `GET /api/v1/auth/sessions` | 401 | `8f34dc8a959f` | `8a6e5275-5ac5-4c50-885c-8c9d6779e5a2` | code="unauthorized" X-Instance-ID="8f34dc8a959f" |
| ✅ | Suspender al usuario corta sus sesiones en todas las instancias en la siguiente petición | `GET /api/v1/auth/sessions` | 401 | `d2c891965b86` | `70d9b4b8-53d9-4f3e-90e2-8c8bcd66a864` | 401 (code="unauthorized") en el 100% de 4 peticiones repartidas en: 8f34dc8a959f=2, d2c891965b86=2 |
| ✅ | El administrador reactiva estudiante2 para dejar la base idempotente | `PATCH /api/v1/admin/users/c0000000-0000-0000-0000-000000000002/status` | 200 | `8f34dc8a959f` | `2d97182e-6a8b-48cd-b0d6-5ba443e23cfd` | status="active" restaurado |
| ✅ | La sesión previa a la suspensión sigue revocada incluso tras reactivar la cuenta | `GET /api/v1/auth/sessions` | 401 | `d2c891965b86` | `92d6cfe1-1a2f-42fb-96a9-f45d1dda76a4` | code="unauthorized" en X-Instance-ID="d2c891965b86" |

## 8. Reconciliación de trabajos multimedia aceptados (E1)

| | Paso | Petición | HTTP | Instancia (`X-Instance-ID`) | `X-Request-ID` | Estado verificado |
| :---: | :--- | :--- | :---: | :---: | :---: | :--- |
| ✅ | Todos los trabajos aceptados alcanzaron su estado final sin pérdidas ni duplicados | `reconciliación POST /complete (202) -> worker -> PostgreSQL + GCS` | — | — | — | 3/3 trabajos reconciliados en completed [video:completed(8e00400f-9eb0-42b6-b396-5...), pdf:completed(ab4a246f-0cbd-47c4-b43b-a...), image:completed(ae765358-2197-483a-92b2-1...)] |

## Reconciliación de trabajos multimedia aceptados

| | Recurso (`id` / `stable_id`) | Tipo | `object_key` | Instancia `/complete` | `X-Request-ID` | Estado final | Duración | Derivados y verificación |
| :---: | :--- | :---: | :--- | :---: | :---: | :---: | ---: | :--- |
| ✅ | `8e00400f-9eb0-42b6-b396-5...` (`5822d55a-4e50-40c5-b3e4-4...`) | `video` | `originals/5822d55a-4e50-40c5-b3e4-4afd5aff8ddc/7467d46b-0ac6-48b8-868c-344474d7f1dc.mp4` | `8f34dc8a959f` | `cf6536cc-9875-440c-bbe3-36c548a8388e` | `completed` | 10s | HLS hls/5822d55a-4e50-40c5-b3e4-4afd5aff8ddc/master.m3u8 (200=true) |
| ✅ | `ab4a246f-0cbd-47c4-b43b-a...` (`fe10a3ec-1284-4bf7-af84-5...`) | `pdf` | `documents/fe10a3ec-1284-4bf7-af84-5d52be5d85db/26ae93de-d4e8-4cda-88bf-bec213c82b48.pdf` | `8f34dc8a959f` | `c57b6481-6b80-4fb4-862d-8f48a17e31ee` | `completed` | 10s | objeto verificado (613 B, lectura=true) |
| ✅ | `ae765358-2197-483a-92b2-1...` (`ce651875-375e-4347-8a2b-2...`) | `image` | `thumbnails/ce651875-375e-4347-8a2b-2baabd6e8d87/cbe8a4c0-173b-4b5d-aab8-d0b5e90d2646.png` | `8f34dc8a959f` | `0b8a6761-8f1d-4a5f-89c4-ddfd0cd8bc0e` | `completed` | 10s | objeto verificado (463 B, lectura=true) |
