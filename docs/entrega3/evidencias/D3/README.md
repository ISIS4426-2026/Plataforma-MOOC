# Evidencia D3 — Interfaz del profesor (issue #213)

Código en [`frontend/src/professor/`](../../../../frontend/src/professor/). Depende de un endpoint nuevo de la
API: `GET /api/v1/me/courses` (rama `feat/213-api-mis-cursos-profesor`), porque `GET /courses` solo lista
publicados y el profesor necesita ver también sus borradores (nota 27).

## Estado frente al issue

| Flujo obligatorio | Ruta | Estado local | Cloud |
| :--- | :--- | :---: | :---: |
| Iniciar sesión como profesor | `/login` | ✅ | ✅ |
| Consultar sus cursos (borradores incluidos) | `/profesor`, `/profesor/cursos` | ✅ | ✅ |
| Crear un curso | `/profesor/cursos/nuevo` | ✅ | ✅ |
| Agregar módulos, lecciones y recursos | `/profesor/cursos/:id` | ✅ | ✅ |
| Cargar un archivo: URL firmada → subida directa → confirmación, con progreso | editor | ✅ | ✅ |
| Ver el recurso pasar de «Procesando» a «Disponible» | editor (se actualiza sola) | ⚠️ | ✅ |
| Crear un cuestionario con preguntas y respuesta correcta | editor | ✅ | ✅ |
| Publicar / despublicar, con las razones si el servidor lo rechaza | editor | ✅ | ✅ |

## Lo que se verificó y cómo

- **Recorrido conectado:** curso creado por el profesor → recurso de video cargado → worker lo procesó → curso
  publicado → un estudiante lo ve en el catálogo, se inscribe, reproduce el video y resuelve el cuestionario.
- **Publicación rechazada:** si el curso no cumple (p. ej. sin contenido o con un recurso sin archivo) el servidor
  responde `422 validation_failed` con `details:[{field,message}]` y la interfaz lista cada razón.
- **Doble clic:** crear curso / módulo / recurso no duplica (guardia local; el servidor además usa `Idempotency-Key`).
- **Curso ajeno:** si el profesor abre el curso de otro, la interfaz muestra «No tienes permiso para editar este
  curso» y el servidor responde 403 a toda escritura (comprobado en [D4](../D4/README.md)).
- **Subida:** el archivo viaja directo al almacenamiento de objetos; la API solo firma y confirma. El servidor exige
  el MIME canónico de la extensión y un máximo de 1 GiB; la interfaz valida extensión, tamaño y vacío antes de subir.

### En la nube (2026-10-31)

[`recorrido_profesor_nube_2026-10-31.txt`](./recorrido_profesor_nube_2026-10-31.txt): curso, módulo, unidad, texto, video, cuestionario con 1 pregunta y publicación, todo desde la interfaz. **El cambio de estado se capturó en vivo**: Sin archivo → «Pidiendo permiso…» → «Subiendo… 100 %» → «Archivo recibido» → En cola (2,1 s) → **Disponible (6,2 s)**.

## Pendiente

Nada de este issue. (Las capturas del editor locales no se guardaron; las de la nube sí: `capturas/2026-10-31_nube-profesor-*`.)

## Nunca

Credenciales, llaves ni secretos.
