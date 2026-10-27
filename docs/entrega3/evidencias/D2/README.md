# Evidencia D2 — Interfaz del estudiante (issue #212)

Código en [`frontend/src/student/`](../../../../frontend/src/student/). Probado en el navegador contra la API
local (`docker compose --profile frontend up`, `http://localhost:8081`).

## Estado frente al issue

| Flujo obligatorio | Ruta | Estado local | Cloud |
| :--- | :--- | :---: | :---: |
| Iniciar sesión | `/login` | ✅ | ⏳ |
| Ver el catálogo de cursos publicados | `/catalogo` | ✅ | ⏳ |
| Inscribirse (con protección contra doble clic) | `/catalogo`, `/cursos/:id` | ✅ | ⏳ |
| Ver «mis cursos» y progreso | `/mis-cursos`, `/` | ✅ | ⏳ |
| Entrar al aula: módulos, lecciones, recursos | `/cursos/:id/:recurso?` | ✅ | ⏳ |
| Reproducir un video procesado (HLS) | aula | ✅ ([captura](./capturas/2026-10-26_estudiante-aula-video-hls.png)) | ⏳ |
| Marcar un recurso de texto como completado | aula | ✅ ([captura](./capturas/2026-10-26_estudiante-aula-texto-completado.png)) | ⏳ |
| Resolver un cuestionario: abrir intento, guardar parcial, enviar | aula | ✅ ([captura](./capturas/2026-10-26_estudiante-quiz-intentos-anteriores.png)) | ⏳ |
| Progreso visible en el inicio | `/` | ✅ ([captura](./capturas/2026-10-26_estudiante-inicio-con-progreso.png)) | ⏳ |

**Para cerrar el issue falta la corrida en la nube** (los criterios piden el flujo contra el despliegue real): la
dependencia es C3 (balanceador) y el frontend desplegado (D1). Con eso: repetir el recorrido, tomar las capturas
con la URL del balanceador visible y registrar el `X-Instance-ID` de las respuestas.

## Lo que se verificó y cómo

- **Video:** archivo mp4 de prueba subido por el flujo del profesor → transcodificado por el worker → reproducido
  en el aula con `hls.js` desde `<HLS_BASE_URL>/hls/<id del recurso>/master.m3u8`. El error de reproducción (HLS o
  nativo) se muestra al estudiante, no se queda en blanco.
- **Cuestionario:** el estudiante nunca recibe la clave de respuestas. Doble clic en «Enviar» no genera un segundo
  envío (guardia local; sin ella el servidor responde 409 al duplicado concurrente).
- **Recurso sin archivo:** un recurso de tipo archivo sin `object_key` se muestra «sin archivo», no «procesando»
  ni «disponible» (ver nota 37).
- **Estados de interfaz** (cargando, vacío, error con reintento, sin permiso): ver [D4](../D4/README.md).
- **Pruebas unitarias:** 30 pruebas de la capa de cliente y utilidades pasan (`npm test` en `frontend/`).

## Pendiente

1. Corrida en la nube con capturas (arriba).
2. Capturas locales de más estados (catálogo, mis cursos, error de red) si el revisor las pide; las existentes cubren
   el recorrido principal.

## Nunca

Credenciales, llaves ni secretos. Las cuentas de prueba usan la contraseña de la semilla, pasada por entorno.
