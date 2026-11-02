# Evidencia I3 — README de la Entrega 3: URL HTTPS, documentación, instrucciones y evidencias ([#233](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/233))

| | |
| :--- | :--- |
| **Bloque** | I — Documentación y cierre |
| **Rúbrica** | Documentación y deuda |
| **Responsable** | `@CrispisCas9` |
| **Depende de** | D5, G7, H3, I1, I2 |
| **Desbloquea** | I4, I5 |
| **Fecha** | 2026-11-02 |

## Criterios de aceptación

| Criterio | Cómo se comprobó | Estado |
| :--- | :--- | :---: |
| Cero enlaces rotos y cero rutas absolutas en todo el repositorio | Recorrido de todos los `.md`: cada enlace relativo resuelve a un archivo que existe. `file:///` solo aparece como texto en [`entrega2/evidencias/I4`](../../../entrega2/evidencias/I4/README.md), donde se describe el defecto que se corrigió, no como enlace | ✅ |
| Las dos URL HTTPS están publicadas y responden | [`urls_https.txt`](./urls_https.txt) y [captura](./capturas/2026-11-02_urls-https-responden.png): frontend, `/api/v1/ready` y `/api/docs` en HTTP 200 desde fuera de la VPC. El certificado, la redirección HTTP → HTTPS y el mínimo TLS 1.2 se comprobaron contra el balanceador en [F1](../F1/README.md) | ✅ |
| Cada uno de los ocho entregables tiene su enlace en el README | Tabla «Los entregables del enunciado» del [README](../../../../README.md). El 2 (tag) y el 8 (video) quedan con su issue (I5, I4) hasta que existan | 🟡 I4, I5 |
| Ningún archivo ni commit del historial contiene credenciales | `bash scripts/b4_barrido_secretos.sh`: **0 hallazgos** en el árbol y en las 143 confirmaciones de todas las ramas; ningún fichero sensible versionado nunca | ✅ |
| `scripts/check_evidencias.sh` sale con código 0 | Faltan **D5** (video corto) e **I6** (destrucción y recreación), que solo se pueden hacer con el entorno; el resto de los issues con captura obligatoria la tienen | ⏳ D5, I6 |
| Un clon desde cero permite reconstruir el entorno | Clon nuevo de `main` (`fb7ca27`): `terraform init -backend=false && terraform validate` → *Success*; la compilación, las pruebas y la validación de Terraform pasan en CI sobre el mismo commit. La reconstrucción completa la demuestra I6 | ✅ · I6 |
| Notas técnicas | Nota de I3 abajo | ✅ |

## Lo que encontró la verificación

**El barrido de secretos daba 2 falsos hallazgos.** El patrón de JSON de cuenta de servicio (`"private_key":`)
coincidía con la documentación de F2, que nombra el campo, en el árbol y en el historial. Una llave real siempre lleva
el PEM como valor, así que el patrón ahora exige `"private_key": "-----BEGIN` y sigue detectando la forma real
(comprobado con una línea de ese formato).

**Los 4 enlaces rotos apuntaban al informe de capacidad**, que estaba en borrador; se resolvieron al fusionar G7.

---

Este archivo no contiene credenciales, llaves ni secretos.
