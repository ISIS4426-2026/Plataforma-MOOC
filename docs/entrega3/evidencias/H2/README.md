# Evidencia H2 — Verificación de umbrales y atención de hallazgos prioritarios ([#229](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/229))

**Bloque:** H — Calidad del código y deuda técnica  
**Rúbrica:** Calidad del código  
**Responsable:** @DiegoOrtizRuiz  
**Depende de:** H1 ([#228](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/228))  
**Desbloquea:** H3 ([#230](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/230))

| Tarea / Criterio de aceptación | Estado | Sección / Evidencia |
| :--- | :---: | :--- |
| Duplicación de código por debajo del `20%` (`0.8%` medido en SonarQube Cloud) | ✅ | §1, [`a_umbrales_y_deuda_referencia.txt`](./a_umbrales_y_deuda_referencia.txt) |
| Mantenibilidad con calificación `C` o mejor (`Calificación A` / `1.0`, `sqale_debt_ratio = 0.0%`) | ✅ | §1, [`a_umbrales_y_deuda_referencia.txt`](./a_umbrales_y_deuda_referencia.txt) |
| Registrar la deuda técnica total estimada por la herramienta, con su unidad y su interpretación como primera medición de referencia | ✅ | §2, [`a_umbrales_y_deuda_referencia.txt`](./a_umbrales_y_deuda_referencia.txt), [`reporte_h2_estado_hallazgos.json`](./reporte_h2_estado_hallazgos.json) |
| Resolver los hallazgos `Critical` y `Blocker` antes del cierre, o dejar un plan explícito con pasos, responsable y plazo | ✅ | §3, §4, [`b_resolucion_blockers_y_criticals.txt`](./b_resolucion_blockers_y_criticals.txt), [`reporte_h2_estado_hallazgos.json`](./reporte_h2_estado_hallazgos.json) |
| Cada corrección tiene su commit y su evidencia de verificación (conservando o añadiendo pruebas unitarias) | ✅ | §3, §5, [`c_pruebas_y_make_check.txt`](./c_pruebas_y_make_check.txt) |
| Documentar la equivalencia aplicada entre las clasificaciones de severidad de las herramientas y del proyecto | ✅ | §6 |
| Anotar hallazgos transversales en [`docs/entrega3/NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md) (Nota Técnica #56) | ✅ | §7 |

---

## 1. Cumplimiento reproducible de los dos umbrales obligatorios

A partir del análisis automatizado ejecutado en **SonarQube Cloud** ([`ISIS4426-2026_Plataforma-MOOC`](https://sonarcloud.io/project/overview?id=ISIS4426-2026_Plataforma-MOOC)) y exportado de forma verificable en [`docs/entrega3/evidencias/H1/reporte_sonarcloud.json`](../H1/reporte_sonarcloud.json), los dos umbrales exigidos en la Entrega 3 se cumplen con amplio margen tanto en el barrido global como en el alcance estricto del código de aplicación (`sonar.sources=api,cmd,internal,frontend/src`):

| Indicador obligatorio | Umbral exigido (Entrega 3) | Valor medido (SonarQube Cloud) | Unidad / Escala | Veredicto |
| :--- | :--- | :--- | :--- | :---: |
| **Duplicación de código** (`duplicated_lines_density`) | **Máximo `20.0%`** ($< 20\%$) | **`0.8%`** (`751` líneas duplicadas en `48` bloques / `21` archivos sobre `90 488` líneas totales; **`0.0%`** dentro de `api,cmd,internal,frontend/src`) | Porcentaje (`%`) de líneas duplicadas sobre el total de líneas | ✅ **CUMPLE** ($19.2\text{ pp}$ por debajo del límite) |
| **Mantenibilidad** (`sqale_rating`) | **Calificación `C` o mejor (`A`, `B` o `C`)** | **Calificación `A`** (`sqale_rating = 1.0`, `sqale_debt_ratio = 0.0%`) | Escala ordinal `A` (`1.0`, ratio $\le 5\%$), `B` (`2.0`, $6\text{–}10\%$), `C` (`3.0`, $11\text{–}20\%$), `D` (`4.0`, $21\text{–}50\%$), `E` (`5.0`, $> 50\%$) | ✅ **CUMPLE** (máxima calificación posible `A`) |

---

## 2. Registro e interpretación de la deuda técnica total estimada (Primera medición de referencia)

El enunciado exige registrar el valor de deuda técnica estimada por la herramienta, su unidad y su interpretación técnica como **primera línea base verificable** del proyecto:

### 2.1. Tabla de la primera medición de referencia

| Métrica en SonarQube Cloud | Clave de API (`metricKey`) | Valor inicial de referencia | Valor tras correcciones H2 | Unidad oficial | Equivalencia en horas y jornadas SonarQube (1 d = 8 h = 480 min) |
| :--- | :--- | ---: | ---: | :--- | :--- |
| **Deuda técnica de mantenibilidad (barrido global inicial `ceecb01`)** | `sqale_index` | **`664`** | **`0`** (tras `sonar.exclusions` de `scripts/` + corrección `typescript:S8786`) | **Minutos (`min`)** | **`11 h 04 min`** (`1 jornada de 8 h + 3 h 04 min`) $\rightarrow$ **`0 min`** |
| **Deuda técnica de mantenibilidad en código de aplicación (`api,cmd,internal,frontend/src`)** | `sqale_index` (`CODE_SMELL` en `sonar.sources`) | **`60`** | **`0`** | **Minutos (`min`)** | **`1 h 00 min`** (`3 × 20 min` en `typescript:S8786`) $\rightarrow$ **`0 min`** |
| **Ratio de deuda técnica** | `sqale_debt_ratio` | **`0.0%`** | **`0.0%`** | **Porcentaje (`%`)** | Costo de remediación dividido entre el costo estimado de desarrollo ($30\text{ min} \times \text{NCLOC}/1000$) |
| **Esfuerzo total de remediación en código de aplicación (todos los tipos: `CODE_SMELL` + `BUG` + `VULNERABILITY`)** | $\sum \text{effort}$ en `sonar.sources` | **`785`** | **`690`** | **Minutos (`min`)** | **`13 h 05 min`** (`1 d 5 h 05 min`) iniciales $\rightarrow$ **`11 h 30 min`** (`1 d 3 h 30 min`) tras resolver `95 min` de hallazgos prioritarios en H2 |

### 2.2. Interpretación técnica de la medición

1. **Qué mide exactamente `sqale_index` en SonarQube:**
   - En el modelo **SQALE** (*Software Quality Assessment based on Lifecycle Expectations*) de SonarQube, la métrica `sqale_index` expresa en **minutos de trabajo de desarrollador** el esfuerzo estimado para remediar todos los hallazgos de la dimensión de **Mantenibilidad (`CODE_SMELL`)**.
   - SonarQube convierte los minutos a horas (`60 min = 1 h`) y a **jornadas laborales de 8 horas (`480 min = 1 d`)**. Por ello, `sqale_index = 664` minutos equivale exactamente a **`11 h 04 min`** o **`1 d 3 h 04 min`**.
2. **Desglose entre el barrido global inicial (`ceecb01`) y el código de aplicación (`api,cmd,internal,frontend/src`):**
   - De los `664 minutos` (`291` code smells) del barrido inicial sobre `ceecb01`, **`604 minutos` (`91.0%`)** provenían de scripts operativos de shell en `scripts/` (`287` ocurrencias de `shell:S7688` a `2 min` cada una = `574 min`, más `1` ocurrencia de `go:S1313` de `30 min` en un script de prueba), que no forman parte de los binarios productivos ni del bundle web y quedaron excluidos justificadamente en [`sonar-project.properties`](../../../../sonar-project.properties) durante H1 ([Nota Técnica #55](../../NOTAS_TECNICAS.md#55)).
   - Los **`60 minutos` (`9.0%`)** restantes correspondían a los **únicos 3 `CODE_SMELL`** presentes en el código de aplicación (`api,cmd,internal,frontend/src`): tres expresiones regulares con riesgo de *backtracking* superlineal (`typescript:S8786`, `20 min` cada una) en `frontend/src/config.ts`, `frontend/src/runtime.ts` y `frontend/src/lib/markdown.tsx`. **Los tres fueron resueltos en el Commit 2 (`bde3686`)**, dejando la deuda de mantenibilidad del código de aplicación en **`0 minutos`**.
3. **Por qué el esfuerzo de remediación de Seguridad/Confiabilidad se contabiliza aparte de `sqale_index`:**
   - SonarQube separa las tres dimensiones de código (*Maintainability / Code Smells*, *Reliability / Bugs* y *Security / Vulnerabilities*). Mientras `sqale_index` suma únicamente `CODE_SMELL`, los hallazgos de tipo `BUG` y `VULNERABILITY` aportan esfuerzo de remediación en sus propias dimensiones (`remediation_effort`).
   - En el código de aplicación (`api,cmd,internal,frontend/src`), el esfuerzo total combinando las tres dimensiones era de **`785 minutos` (`13 h 05 min`)** distribuidos en `39` hallazgos:
     - `1 BLOCKER VULNERABILITY` (`secrets:S8215` en `internal/auth/service.go:23`, `30 min`) $\rightarrow$ **Resuelto en H2 (`868e43d`)**.
     - `1 MAJOR BUG` (`typescript:S9383` en `frontend/src/student/StudentData.tsx:35`, `5 min`) $\rightarrow$ **Resuelto en H2 (`bde3686`)**.
     - `3 MAJOR CODE_SMELL` (`typescript:S8786` en `frontend/src/`, `60 min`) $\rightarrow$ **Resueltos en H2 (`bde3686`)**.
     - `33 MAJOR VULNERABILITY` (`go:S2077` en `internal/postgres/*.go`, `660 min`) y `1 MINOR VULNERABILITY` (`go:S5332` en `internal/mailer/smtp.go:89`, `30 min`) $\rightarrow$ Auditados como seguros en producción (usan constantes de compilación inmutables + parámetros `$1..$n` y guardia obligatorio de STARTTLS con credenciales) y registrados con plan de tratamiento para H3.

---

## 3. Correcciones realizadas en H2 con commit dedicado y pruebas de respaldo

Cada corrección se implementó en un commit atómico independiente sobre la rama `h2-Verificacion-de-humbrales`, conservando y ampliando las pruebas automatizadas que verifican el comportamiento:

| ID | Regla / Herramienta | Severidad SonarQube | Severidad Proyecto | Ubicación afectada | Descripción de la corrección | Commit | Evidencia de prueba y verificación |
| :--- | :--- | :---: | :---: | :--- | :--- | :---: | :--- |
| **`H2-FIX-01`** | `secrets:S8215` (SonarQube) | **BLOCKER** (`VULNERABILITY`) | **CRÍTICA** (`Blocker / P0`) | [`internal/auth/service.go:23`](../../../../internal/auth/service.go#L17-L22), [`internal/auth/password.go:25-37`](../../../../internal/auth/password.go#L25-L37) | Se eliminó el hash bcrypt literal estático `const dummyHash = "$2a$10$..."` del árbol de código y se reemplazó por `mustGenerateDummyHash()`, que deriva el hash en el arranque del paquete usando `bcrypt.DefaultCost`. Elimina el falso positivo de secreto embebido y alinea exactamente el factor de trabajo del escudo contra *timing attacks* con el de las cuentas reales. | `868e43d` | `go test -v ./internal/auth/...` — Prueba nueva [`TestLoginUnknownEmailRunsTimingShieldWithoutHardcodedHash`](../../../../internal/auth/service_test.go#L383-L395) (`PASS`) + `golangci-lint run ./internal/auth/...` (`0 issues`). |
| **`H2-FIX-02`** | `typescript:S9383` (SonarQube) | **MAJOR** (`BUG`) | **ALTA** (`Major / P1`) | [`frontend/src/student/StudentData.tsx:35`](../../../../frontend/src/student/StudentData.tsx#L33-L37) | Se marcó explícitamente con el operador `void` la invocación de la IIFE asíncrona `void (async () => { ... })()` dentro de `useEffect`, cumpliendo la regla `no-floating-promises` mientras el bloque `try/catch` interno gestiona el estado `error`. | `bde3686` | `npm run test:coverage && npm run build` (`tsc --noEmit && vite build`: `0 errores`). |
| **`H2-FIX-03`** | `typescript:S8786` × 3 (SonarQube) | **MAJOR** (`CODE_SMELL`) | **ALTA** (`Major / P1`) | [`frontend/src/config.ts:6`](../../../../frontend/src/config.ts#L5-L15), [`frontend/src/runtime.ts:24`](../../../../frontend/src/runtime.ts#L23-L34), [`frontend/src/lib/markdown.tsx:24-98`](../../../../frontend/src/lib/markdown.tsx#L24-L98) | Se reemplazaron las expresiones regulares `/\/+$/` y `/^(#{1,3})\s+(.*)$/` (susceptibles a *backtracking* superlineal $O(n^2)$) por funciones deterministas de recorrido lineal $O(n)$: `stripTrailingSlashes`, `parseHeading` y `parseListItem`. Reduce a `0 min` la deuda `CODE_SMELL` del código de aplicación. | `bde3686` | `npm run test:coverage` — Pruebas nuevas en [`frontend/src/runtime.test.ts`](../../../../frontend/src/runtime.test.ts#L8-L12) y [`frontend/src/lib/lib.test.tsx`](../../../../frontend/src/lib/lib.test.tsx#L48-L55) (`4 suites / 32 tests PASS`). |
| **`H2-FIX-04`** | `githubactions:S8482` (SonarQube) | **BLOCKER** (`VULNERABILITY`) | **CRÍTICA** (`Blocker / P0`) | [`.github/workflows/ci.yml:85`](../../../../.github/workflows/ci.yml#L83-L86) | Se sustituyó la descarga y ejecución directa por tubería `curl -sSfL .../install.sh \| sh` por `go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8`, que verifica criptográficamente el módulo contra la base de sumas de Go (`sum.golang.org`). | `28967df` | `bash ./scripts/h1_verificar_y_exportar_sonar.sh` (`27 OK · 0 FALLA`). |
| **`H2-FIX-05`** | `python:S5443` (1 `CRITICAL`), `go:S4830` (2 `CRITICAL`), `go:S5527` (2 `CRITICAL`), `secrets:S6698` (1 `BLOCKER`) | **CRITICAL / BLOCKER** (`VULNERABILITY`) | **CRÍTICA** (`Blocker / P0`) | [`scripts/g6_linea_de_tiempo.py:41`](../../../../scripts/g6_linea_de_tiempo.py#L41), [`scripts/capacity_escenario2/main.go:333`](../../../../scripts/capacity_escenario2/main.go#L330-L342), [`scripts/pilot_escenario2/main.go:105`](../../../../scripts/pilot_escenario2/main.go#L102-L114), [`scripts/migrate.sh:48`](../../../../scripts/migrate.sh#L48) | 1) En `g6_linea_de_tiempo.py` se cambió el directorio fijo `/tmp/matplotlib-g6` por `tempfile.mkdtemp(prefix="matplotlib-g6-")`; 2) En los arneses de `capacity_escenario2` y `pilot_escenario2` se activó por omisión la validación de certificados TLS 1.2 (`InsecureSkipVerify: os.Getenv("MOOC_TLS_INSECURE") == "1"`); 3) En `scripts/migrate.sh` se cambió el marcador `:CONTRASENA@` por `<DB_PASSWORD>`. | `28967df` | `python3.10 scripts/g6_linea_de_tiempo.py --autoprueba` (`OK`) + `go vet ./scripts/capacity_escenario2 ./scripts/pilot_escenario2` (`0 errores`). |
| **`H2-FIX-06`** | `Spectral operation-description` (`BUG-DOC-008` / Nota Técnica #13) | **MINOR** (`Warning`) | **BAJA** (`Minor / P3`) | [`api/openapi.yaml:1092`](../../../../api/openapi.yaml#L1091-L1101), [`api/openapi.yaml:1231`](../../../../api/openapi.yaml#L1230-L1240) | Se añadieron las descripciones faltantes en `PUT /modules/{module_id}` (`updateModule`) y `PUT /units/{unit_id}` (`updateUnit`), cerrando las 2 advertencias históricas de Spectral arrastradas desde la Entrega 1 ([`BUG-DOC-008`](../../../e2e/REPORTE_BUGS_Y_CALIDAD_ETAPA.md#48-bug-doc-008--advertencias-operacionales-en-openapi-spectral-linter-operation-description) y [Nota Técnica #13](../../NOTAS_TECNICAS.md#13)). | `d469eb6` | Validación automatizada de `0` operaciones sin `description` sobre las rutas de `api/openapi.yaml` + `go test ./internal/http/...` (`PASS`). |

---

## 4. Plan explícito de tratamiento para hallazgos remanentes (Cero `Blocker` / `Critical` sin resolver ni sin plan)

De acuerdo con el criterio de aceptación *"Ningun Critical o Blocker queda sin resolver ni sin plan explicito"*, a continuación se documenta el estado de la totalidad de hallazgos restantes y su plan de acción con **pasos concretos, responsable e hito/plazo**:

| ID Plan | Regla y Clasificación | Ubicación y Alcance | Diagnóstico técnico (Por qué no se muta en caliente en H2) | Plan explícito de tratamiento (Pasos) | Responsable | Plazo / Hito |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **`TD-SEC-FP-01`** | `secrets:S6698` (`1 BLOCKER`), `secrets:S8215` (`11 BLOCKER`), `json:S6418` (`4 BLOCKER`) | **Fuera de `sonar.sources`** (`infra/terraform/outputs.tf:138`, `scripts/seeds/*.sql`, `docs/e2e/evidencia/segmento1_identidad_admin/*.json`) | 1) En `infra/terraform/outputs.tf:138`, la cadena `"postgres://%s:CONTRASENA@%s:5432/%s?sslmode=require"` es un marcador de plantilla documentado (no una contraseña real), y la [Nota Técnica #1](../../NOTAS_TECNICAS.md#1) prohíbe modificar archivos de Terraform en ramas de trabajo para no desincronizar el estado remoto.<br>2) En `scripts/seeds/*.sql`, los 11 hashes bcrypt corresponden a las contraseñas sintéticas públicas (`AdminPassword123!`, `StudentPassword123!`) requeridas por las 7 colecciones de Newman/Postman.<br>3) En `docs/e2e/evidencia/`, los 4 JSON son evidencias históricas congeladas de la Entrega 1 con tokens efímeros de un contenedor local descartado en septiembre. | **Paso 1 (Completado en H1):** Excluir `infra/**`, `scripts/**` y `docs/**` de `sonar.sources` en [`sonar-project.properties`](../../../../sonar-project.properties) para que el análisis mida exclusivamente el código de aplicación.<br>**Paso 2:** Durante el cierre de infraestructura desde `main` (I6, #236), sustituir `:CONTRASENA@` por `<DB_PASSWORD>` en `infra/terraform/outputs.tf` y marcar las entradas históricas como *False Positive* en SonarQube Cloud.<br>**Paso 3:** Documentar en `docs/entrega3/deuda-tecnica.md` (H3, #230) la migración de `scripts/seeds/*.sql` a generación dinámica de hashes en Go para la siguiente iteración. | @DiegoOrtizRuiz / @fredyxander | **2026-11-04** (Cierre de Entrega 3: H3 #230 e I6 #236) |
| **`TD-SQL-01`** | `go:S2077` (`33 MAJOR VULNERABILITY`) | **Código de aplicación:** `internal/postgres/{quiz,structure,course,admin,enrollment,session,verification_token,audit}_repository.go` | Las 33 alertas se disparan porque las cadenas SQL se arman concatenando constantes de paquete declaradas en tiempo de compilación (`SELECT ` + `userColumns` + ` FROM users WHERE id = $1`) o cláusulas `WHERE` con placeholders posicionales (`$1, $2...`). **El 100 % de los valores de usuario viaja parametrizado en `args...`**, por lo que **no existe vulnerabilidad real de inyección SQL**. Reescribir las 33 consultas en caliente introduciría riesgo innecesario sobre las pruebas de carga ya medidas en G3–G6. | **Paso 1 (Completado en H2):** Auditar las 33 líneas confirmando que ninguna interpola entrada de usuario en la cadena SQL.<br>**Paso 2:** Registrar el ítem consolidado `TD-COD-01` en `docs/entrega3/deuda-tecnica.md` (H3, #230) con análisis de impacto y criterio de verificación.<br>**Paso 3:** En la siguiente iteración, externalizar las consultas constantes o adoptar `sqlc` y marcar las consultas auditadas como *Safe / Reviewed* en SonarQube Cloud. | @DiegoOrtizRuiz | **2026-11-02** (Documentado en H3 #230; refactorización prevista para Entrega 4) |
| **`TD-SMTP-01`** | `go:S5332` (`1 MINOR VULNERABILITY`) | **Código de aplicación:** [`internal/mailer/smtp.go:89`](../../../../internal/mailer/smtp.go#L89-L118) | `smtp.NewClient(conn, m.host())` inicia la sesión SMTP antes de negociar `STARTTLS`. Cuando hay credenciales configuradas (`m.username != ""`, que es el caso en producción), el código **exige** TLS implícito (puerto 465) o `STARTTLS` con TLS $\ge 1.2$ (puerto 587, corregido en el issue #126). Solo permite texto claro sin credenciales para hablar con el contenedor local `mailpit` en pruebas E2E. | **Paso 1:** Conservar intacto el bloqueo que impide autenticar sin TLS/STARTTLS cuando `m.username != ""`.<br>**Paso 2:** Registrar en `docs/entrega3/deuda-tecnica.md` (H3, #230) como compromiso técnico deliberado y controlado para pruebas locales con Mailpit. | @DiegoOrtizRuiz | **2026-11-02** (Documentado en H3 #230) |

---

## 5. Conservación y ejecución reproducible de las pruebas (`make check` y Frontend)

Todas las correcciones fueron validadas ejecutando los linters, compilaciones y suites de pruebas unitarias del repositorio ([`c_pruebas_y_make_check.txt`](./c_pruebas_y_make_check.txt)):

```bash
# 1. Ejecutar el verificador integral de H2 (umbrales, deuda de referencia, commits de correccion y pruebas):
bash ./scripts/h2_verificar_umbrales_y_hallazgos.sh

# 2. Ejecutar las verificaciones individuales del pipeline:
make fmt vet build
golangci-lint run ./...
go test -v ./internal/auth/... ./internal/http/... ./migrations/...
cd frontend && npm run test:coverage && npm run build && npm run check:bundle
```

Resultado del verificador [`scripts/h2_verificar_umbrales_y_hallazgos.sh`](../../../../scripts/h2_verificar_umbrales_y_hallazgos.sh):
- **`23 OK · 0 FALLA · 0 OBS`** en [`resumen.txt`](./resumen.txt).

---

## 6. Equivalencia aplicada entre clasificaciones de severidad

Dado que en el proyecto intervienen **SonarQube Cloud** (que expone tanto el modelo estándar de severidades e indicadores SQALE como la taxonomía *Clean Code / Multi-Dimensional Quality Mode [MQR]*), los linters del pipeline (**`golangci-lint`** y **OpenAPI `Spectral`**) y la **Taxonomía de Severidad del Proyecto** definida desde la Entrega 1 en [`docs/e2e/REPORTE_BUGS_Y_CALIDAD_ETAPA.md` §2](../../../e2e/REPORTE_BUGS_Y_CALIDAD_ETAPA.md#2-taxonomía-de-severidad-y-criterios-de-clasificación), se establece y aplica la siguiente tabla de equivalencia:

| Severidad SonarQube (Modelo Estándar) | Severidad SonarQube (Clean Code / MQR) | Tipo en SonarQube (`type`) | Equivalencia en `golangci-lint` / `Spectral` | Equivalencia en Taxonomía del Proyecto (`REPORTE_BUGS_Y_CALIDAD_ETAPA.md`) | Regla de atención en Entrega 3 (H2 / H3) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`BLOCKER`** | **`Blocker` / `High`** | `BUG`, `VULNERABILITY`, `CODE_SMELL` | `error` (bloquea `make lint` con código $\ne 0$) | **CRÍTICA (`Blocker / P0`)** | **Obligatorio resolver antes del cierre** con commit y prueba, o dejar **plan explícito** (pasos, responsable y plazo). |
| **`CRITICAL`** | **`High`** | `BUG`, `VULNERABILITY`, `CODE_SMELL` | `error` (fallo de seguridad o compilación/vet) | **CRÍTICA (`Blocker / P0`)** | **Obligatorio resolver antes del cierre** con commit y prueba, o dejar **plan explícito** (pasos, responsable y plazo). |
| **`MAJOR`** | **`Medium`** | `BUG`, `VULNERABILITY`, `CODE_SMELL` | `warning` / hallazgo funcional o estructural relevante | **ALTA (`Major / P1`)** (si es `BUG` o `VULNERABILITY` real) / **MEDIA (`Moderate / P2`)** (si es `CODE_SMELL` o consulta parametrizada auditada) | Resolver durante el sprint (como se hizo con `typescript:S9383` y `typescript:S8786`) o **registrar obligatoriamente en el inventario de H3** con regla, ubicación, impacto y motivo de aplazamiento. |
| **`MINOR`** | **`Low`** | `CODE_SMELL`, `VULNERABILITY` menor | `warning` no bloqueante en `Spectral` (`operation-description`) | **BAJA (`Minor / P3`)** | Deuda técnica controlada; se atiende cuando es directo (p. ej. `BUG-DOC-008` en `api/openapi.yaml`) o se documenta como compromiso técnico aceptado (`go:S5332` para Mailpit local). |
| **`INFO`** | **`Info`** | `CODE_SMELL` informativo | `info` / `hint` | **Informativo (Sin impacto funcional)** | Trazabilidad opcional en notas o documentación. |

---

## 7. Capturas de verificación y Notas Técnicas

### Capturas en `docs/entrega3/evidencias/H2/capturas/`
- [`capturas/2026-11-01_h2_umbrales_y_deuda.png`](./capturas/2026-11-01_h2_umbrales_y_deuda.png) — Verificación reproducible de los dos umbrales (`0.8% < 20%` en duplicación y calificación `A <= C` en mantenibilidad) y registro de la primera medición de referencia de deuda técnica (`664 min` global / `60 min -> 0 min` en código de aplicación).
- [`capturas/2026-11-01_h2_hallazgos_y_commits.png`](./capturas/2026-11-01_h2_hallazgos_y_commits.png) — Verificación de las 6 correcciones en código con sus commits dedicados (`868e43d`, `bde3686`, `28967df`, `d469eb6`) y planes explícitos para los ítems remanentes.
- [`capturas/2026-11-01_h2_verificacion_make_check.png`](./capturas/2026-11-01_h2_verificacion_make_check.png) — Ejecución de linters, compilación y pruebas automatizadas de Go y Frontend (`23 OK · 0 FALLA · 0 OBS`).

### Hallazgos anotados en `docs/entrega3/NOTAS_TECNICAS.md`
Se añadió la **Nota Técnica #56** en [`docs/entrega3/NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md#56) documentando para **H3**, **I3** e **I5**:
1. El desglose de la primera medición de referencia (`duplicated_lines_density = 0.8%`, `sqale_rating = A [1.0]`, `sqale_index = 664 min` en el barrido global `ceecb01` frente a `60 min -> 0 min` de `CODE_SMELL` en el alcance de aplicación `api,cmd,internal,frontend/src`).
2. Las 6 correcciones aplicadas en H2 con sus commits (`868e43d`, `bde3686`, `28967df`, `d469eb6`), incluyendo el cierre definitivo de la **Nota Técnica #13** (`BUG-DOC-008` en `api/openapi.yaml`), para que en H3 figuren en la sección de *Correcciones realizadas durante este ejercicio* y no como deuda abierta.
3. El diagnóstico de las 33 ocurrencias de `go:S2077` en `internal/postgres/*.go` (concatenación de constantes de columnas en tiempo de compilación con parámetros `$1..$n`) y de `go:S5332` en `internal/mailer/smtp.go:89` (texto claro solo cuando `m.username == ""` para `mailpit` local), que pasan al inventario priorizado de `docs/entrega3/deuda-tecnica.md` en H3.

---

Este archivo no contiene credenciales, llaves ni secretos.
