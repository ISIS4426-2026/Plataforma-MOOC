# Evidencia D5 — Matriz de flujos y recorrido conectado (issue #215)

## Matriz: flujo ↔ ruta de la interfaz ↔ endpoint ↔ evidencia

| Rol | Flujo | Ruta | Endpoint principal | Local | Cloud |
| :--- | :--- | :--- | :--- | :---: | :---: |
| Ambos | Iniciar / cerrar sesión | `/login` | `POST /auth/login`, `POST /auth/logout` | ✅ [D1](../D1/README.md) | ✅ |
| Estudiante | Catálogo | `/catalogo` | `GET /courses` | ✅ [D2](../D2/README.md) | ✅ |
| Estudiante | Inscribirse | `/cursos/:id` | `POST /courses/{id}/enrollments` | ✅ | ✅ |
| Estudiante | Mis cursos y progreso | `/`, `/mis-cursos` | `GET /me/enrollments`, `GET /progress/courses/{id}` | ✅ | ✅ |
| Estudiante | Aula, texto completado | `/cursos/:id/:recurso` | `POST /progress/heartbeat` | ✅ | ✅ |
| Estudiante | Video HLS | aula | `master.m3u8` del almacenamiento | ✅ | ✅ |
| Estudiante | Cuestionario | aula | `POST /quizzes/{id}/attempts`, `PUT …/attempts/current`, envío | ✅ | ✅ |
| Profesor | Mis cursos | `/profesor/cursos` | `GET /me/courses` (nuevo) | ✅ [D3](../D3/README.md) | ✅ |
| Profesor | Crear curso y estructura | `/profesor/cursos/:id` | `POST /courses`, módulos, lecciones, recursos | ✅ | ✅ |
| Profesor | Subir archivo | editor | `POST /media/presigned-url` → PUT → `POST /media/uploads/{id}/complete` | ✅ | ✅ |
| Profesor | Cuestionario | editor | quiz y preguntas | ✅ | ✅ |
| Profesor | Publicar / despublicar | editor | `POST /courses/{id}/publish`, `/unpublish` | ✅ | ✅ |
| Ambos | Autorización negativa | — | 401 / 403 | ✅ [D4](../D4/README.md) | ✅ |

## Recorrido conectado (profesor → estudiante), en la nube

Ejecutado el **2026-10-31** en `https://136.68.55.254.sslip.io`, desde el navegador, con la versión `f6aa76e5988c`:
el profesor crea el curso, le sube un video (procesado en ~4 s), crea un cuestionario y publica; el estudiante lo
encuentra, se inscribe, lee, reproduce el video, resuelve el cuestionario (nota 100) y queda «Curso aprobado».
Detalle y tiempos: [`D3/recorrido_profesor_nube_2026-10-31.txt`](../D3/recorrido_profesor_nube_2026-10-31.txt) y
[`D2/recorrido_estudiante_nube_2026-10-31.txt`](../D2/recorrido_estudiante_nube_2026-10-31.txt).

## Pendiente

1. **El video corto de pantalla** del recorrido conectado, que pide el issue: lo debe grabar una persona (el movimiento es la prueba).
2. El rechazo de un profesor sobre el curso de otro profesor, y `verify_authorization.sh` contra el balanceador (ver D4).

## Nunca

Credenciales, llaves ni secretos.
