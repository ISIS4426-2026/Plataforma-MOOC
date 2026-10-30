# Evidencia D5 — Matriz de flujos y recorrido conectado (issue #215)

## Matriz: flujo ↔ ruta de la interfaz ↔ endpoint ↔ evidencia

| Rol | Flujo | Ruta | Endpoint principal | Local | Cloud |
| :--- | :--- | :--- | :--- | :---: | :---: |
| Ambos | Iniciar / cerrar sesión | `/login` | `POST /auth/login`, `POST /auth/logout` | ✅ [D1](../D1/README.md) | ⏳ |
| Estudiante | Catálogo | `/catalogo` | `GET /courses` | ✅ [D2](../D2/README.md) | ⏳ |
| Estudiante | Inscribirse | `/cursos/:id` | `POST /courses/{id}/enrollments` | ✅ | ⏳ |
| Estudiante | Mis cursos y progreso | `/`, `/mis-cursos` | `GET /me/enrollments`, `GET /progress/courses/{id}` | ✅ | ⏳ |
| Estudiante | Aula, texto completado | `/cursos/:id/:recurso` | `POST /progress/heartbeat` | ✅ | ⏳ |
| Estudiante | Video HLS | aula | `master.m3u8` del almacenamiento | ✅ | ⏳ |
| Estudiante | Cuestionario | aula | `POST /quizzes/{id}/attempts`, `PUT …/attempts/current`, envío | ✅ | ⏳ |
| Profesor | Mis cursos | `/profesor/cursos` | `GET /me/courses` (nuevo) | ✅ [D3](../D3/README.md) | ⏳ |
| Profesor | Crear curso y estructura | `/profesor/cursos/:id` | `POST /courses`, módulos, lecciones, recursos | ✅ | ⏳ |
| Profesor | Subir archivo | editor | `POST /media/presigned-url` → PUT → `POST /media/uploads/{id}/complete` | ✅ | ⏳ |
| Profesor | Cuestionario | editor | quiz y preguntas | ✅ | ⏳ |
| Profesor | Publicar / despublicar | editor | `POST /courses/{id}/publish`, `/unpublish` | ✅ | ⏳ |
| Ambos | Autorización negativa | — | 401 / 403 | ✅ [D4](../D4/README.md) | ⏳ |

## Recorrido conectado (profesor → estudiante)

Verificado en local, en el navegador, de punta a punta: el profesor crea el curso, le sube un video y crea un
cuestionario; el worker procesa el video; el profesor publica; un estudiante lo encuentra en el catálogo, se inscribe,
reproduce el video, resuelve el cuestionario y ve su progreso. Falta repetirlo **en la nube** y grabar el video corto
que pide el issue.

## Pendiente (todo depende de tener la nube arriba)

1. C3 (balanceador) y el frontend desplegado (D1).
2. Corrida del recorrido en la nube y video corto, mostrando la URL del balanceador y el `X-Instance-ID`.
3. Pasar la columna «Cloud» a ✅ con enlaces a las capturas.

## Nunca

Credenciales, llaves ni secretos.
