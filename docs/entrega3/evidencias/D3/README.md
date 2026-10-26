# Evidencia D3 — Interfaz del profesor (issue #213)

Código en [`frontend/src/professor/`](../../../../frontend/src/professor/). Depende de un endpoint nuevo de la
API: `GET /api/v1/me/courses` (rama `feat/213-api-mis-cursos-profesor`), porque `GET /courses` solo lista
publicados y el profesor necesita ver también sus borradores (nota 27).

## Estado frente al issue

| Flujo obligatorio | Ruta | Estado local | Cloud |
| :--- | :--- | :---: | :---: |
| Iniciar sesión como profesor | `/login` | ✅ | ⏳ |
| Consultar sus cursos (borradores incluidos) | `/profesor`, `/profesor/cursos` | ✅ | ⏳ |
| Crear un curso | `/profesor/cursos/nuevo` | ✅ | ⏳ |
| Agregar módulos, lecciones y recursos | `/profesor/cursos/:id` | ✅ | ⏳ |
| Cargar un archivo: URL firmada → subida directa → confirmación, con progreso | editor | ✅ | ⏳ |
| Ver el recurso pasar de «Procesando» a «Disponible» | editor (se actualiza sola) | ⚠️ ver abajo | ⏳ |
| Crear un cuestionario con preguntas y respuesta correcta | editor | ✅ | ⏳ |
| Publicar / despublicar, con las razones si el servidor lo rechaza | editor | ✅ | ⏳ |

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

### Limitación honesta: «Procesando → Disponible»

El estado «Procesando» se vio en pantalla y el recurso terminó «Disponible», pero **no se capturó el cambio de
estado en vivo**: la transcodificación de 90 s a 720p tardó 2 m 49 s en esta máquina y el video terminó mientras se
cambiaba de sesión. Hay que registrarlo en la corrida de la nube (los videos de la semilla son cortos).

## Pendiente

1. Corrida en la nube con capturas, incluida la del cambio «Procesando → Disponible».
2. **Fredy debe aprobar `GET /me/courses`**: es un cambio aditivo en el contrato (`api/openapi.yaml`) y en el
   servidor; va en un PR separado de la interfaz.
3. Capturas locales del editor (no se guardaron al disco en esta sesión).

## Nunca

Credenciales, llaves ni secretos.
