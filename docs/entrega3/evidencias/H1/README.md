# Evidencia H1 — SonarQube en GitHub Actions sobre API, workers y frontend ([#228](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/228))

**Bloque:** H — Calidad del código y deuda técnica  
**Rúbrica:** Calidad del código  
**Responsable:** @DiegoOrtizRuiz  
**Depende de:** A1 (#193)  
**Desbloquea:** H2 (#229), H3 (#230)

| Tarea / Criterio de aceptación | Estado | Sección / Evidencia |
| :--- | :---: | :--- |
| Añadir el análisis a [`.github/workflows/ci.yml`](../../../../.github/workflows/ci.yml) conservando `lint`, `build`, `migrations`, `tests` y verificaciones de `frontend` | ✅ | §1, [`b_pipeline_y_secretos.txt`](./b_pipeline_y_secretos.txt) |
| Alcance sobre el **código total del proyecto** —API, workers y frontend—, no solo sobre el código nuevo | ✅ | §2, [`sonar-project.properties`](../../../../sonar-project.properties), [`a_alcance_y_exclusiones.txt`](./a_alcance_y_exclusiones.txt) |
| Publicar la cobertura de pruebas al analizador (`coverage.out` en Go y `frontend/coverage/lcov.info` en Frontend) | ✅ | §3, [`c_cobertura_y_reporte.txt`](./c_cobertura_y_reporte.txt) |
| Documentar herramienta y versión, perfil de reglas, alcance y exclusiones justificadas una por una, sin excluir código de aplicación | ✅ | §2, §4 |
| Registrar fecha del análisis y commit evaluado | ✅ | §5 |
| Credenciales del analizador como secretos del repositorio (`${{ secrets.SONAR_TOKEN }}`), nunca en el workflow | ✅ | §1, [`b_pipeline_y_secretos.txt`](./b_pipeline_y_secretos.txt) |
| Anotar hallazgos transversales en [`docs/entrega3/NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md) (Nota Técnica #55) | ✅ | §6 |

---

## 1. Integración en `.github/workflows/ci.yml` y conservación de la automatización previa

El análisis estático se integró directamente en [`.github/workflows/ci.yml`](../../../../.github/workflows/ci.yml) como un cuarto trabajo (`sonarqube`) que depende de la finalización exitosa de `ci` (Go) y `frontend` (`needs: [ci, frontend]`).

### 1.1. Trabajos del pipeline y preservación de controles previos

| Job en `ci.yml` | Runner | Qué verifica y conserva | Artefacto emitido |
| :--- | :--- | :--- | :--- |
| `terraform` (`Terraform Format & Validate`) | `ubuntu-24.04` | `terraform fmt -check -recursive -diff`, `init -backend=false` y `validate` sobre `infra/terraform`. | — |
| `ci` (`Build, Lint, Migrations & Unit Tests`) | `ubuntu-24.04` | Contenedores de servicio `postgres:16-alpine` y `redis:7-alpine`; `make lint` (`gofmt`, `go vet`, `golangci-lint v1.64.8`, `Spectral` OpenAPI 3.1), `make build` (`cmd/api` y `cmd/worker`), `make test-migrations`, `make test` (`go test -v -coverprofile=coverage.out -covermode=atomic ./...`) y `make check`. | `go-coverage` (`coverage.out`) |
| `frontend` (`Frontend`) | `ubuntu-24.04` | `npm ci --no-audit --no-fund`, `npm run test:coverage` (`vitest run` + generación LCOV), `npm run build` (`tsc --noEmit && vite build`) y `npm run check:bundle` (0 secretos ni IPs privadas en el bundle). | `frontend-coverage` (`frontend/coverage/lcov.info`) |
| `sonarqube` (`SonarQube Static Analysis`) | `ubuntu-24.04` | `actions/checkout@v5` con `fetch-depth: 0` (*blame* completo), descarga de `coverage.out` y `frontend/coverage/lcov.info`, validación de existencia de ambos reportes y ejecución de `SonarSource/sonarqube-scan-action@v5` contra **SonarQube Cloud**. | Resumen y enlace en `GITHUB_STEP_SUMMARY` |

### 1.2. Atención de la Nota Técnica #15 (obsolescencias de CI)

Aprovechando la edición de `.github/workflows/ci.yml` prevista en la [Nota Técnica #15](../../NOTAS_TECNICAS.md#15):
- Se fijaron todos los jobs en `ubuntu-24.04` en lugar de `ubuntu-latest` para evitar cambios imprevistos de herramientas del sistema durante la ventana de migración de GitHub Actions a Ubuntu 26.
- Se actualizaron `actions/checkout@v4 → actions/checkout@v5` y `hashicorp/setup-terraform@v3 → hashicorp/setup-terraform@v4`, eliminando los avisos de deprecación de Node.js 20 en el runner.

### 1.3. Gestión segura de credenciales

- El token de autenticación con SonarQube Cloud se inyecta exclusivamente mediante el secreto de repositorio `SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}`.
- Ni [`.github/workflows/ci.yml`](../../../../.github/workflows/ci.yml) ni [`sonar-project.properties`](../../../../sonar-project.properties) contienen credenciales, tokens ni llaves literales (verificado automáticamente en [`b_pipeline_y_secretos.txt`](./b_pipeline_y_secretos.txt)).

---

## 2. Herramienta, versión, perfiles de reglas y alcance sobre el código total

### 2.1. Ficha técnica del analizador

| Parámetro | Valor configurado |
| :--- | :--- |
| **Herramienta** | **SonarQube Cloud (SonarCloud)** — `https://sonarcloud.io` |
| **Acción y Scanner en CI** | `SonarSource/sonarqube-scan-action@v5` (`sonar-scanner-cli` 6.x oficial de SonarSource) |
| **Organización (`sonar.organization`)** | `isis4426-2026` (parametrizable vía `vars.SONAR_ORGANIZATION`) |
| **Clave de proyecto (`sonar.projectKey`)** | `ISIS4426-2026_Plataforma-MOOC` (parametrizable vía `vars.SONAR_PROJECT_KEY`) |
| **Modalidad de evaluación** | **Código total del proyecto (*Overall Code*)**, no únicamente código nuevo |
| **Perfiles de reglas (*Quality Profiles*)** | **Sonar way** (perfil oficial por omisión de SonarSource) para **Go**, **TypeScript / JavaScript** y **CSS** |
| **Umbrales de referencia de la Entrega 3** | Duplicación de código $\le 20\%$; Mantenibilidad (`sqale_rating`) calificación **C o mejor (`A`, `B` o `C`)**; registro de la deuda técnica total estimada (`sqale_index`) en minutos/horas |

### 2.2. Alcance sobre el código total de aplicación (`sonar.sources`)

La propiedad `sonar.sources=api,cmd,internal,frontend/src` en [`sonar-project.properties`](../../../../sonar-project.properties) abarca el **100 % del código de aplicación** de las tres capas del sistema:

| Capa / Directorio | Archivos productivos | Líneas productivas | Archivos de test (`sonar.tests`) | Líneas de test | Componentes incluidos |
| :--- | ---: | ---: | ---: | ---: | :--- |
| **Contrato embebido (`api/`)** | 1 | 14 | 0 | 0 | `api/embed.go` (exposición del contrato OpenAPI 3.1 en la API). |
| **Puntos de entrada (`cmd/`)** | 4 | 1 479 | 0 | 0 | `cmd/api` (servidor HTTP), `cmd/worker` (consumidor `asynq` + `ffmpeg`), `cmd/queue-exporter` (exportador de métricas a Cloud Monitoring) y `cmd/seed-media`. |
| **Backend modular: API y Workers (`internal/`)** | 95 | 16 463 | 58 | 14 092 | `admin`, `auth`, `cache`, `config`, `course`, `domain`, `enrollment`, `http` (`handler`, `middleware`), `mailer`, `media`, `observability`, `postgres`, `progress`, `quiz`, `storage`, `structure`, `transcode` y `worker` (`handler`, `task`). |
| **Migraciones (`migrations/`)** | 0 | 0 | 1 | 197 | `migrations/migrations_test.go` (validación estructural, reversibilidad y ejecución en PostgreSQL). |
| **Frontend web (`frontend/src/`)** | 35 | 3 726 | 4 | 196 | `api/`, `auth/`, `components/`, `hooks/`, `lib/`, `pages/`, `professor/`, `student/`, `config.ts`, `main.tsx`, `runtime.ts` y `styles.css`. |
| **Total analizado** | **135** | **21 682** | **63** | **14 485** | **API + Workers + Frontend completos (36 167 líneas entre código productivo y tests)** |

---

## 3. Publicación de cobertura de pruebas al analizador

SonarQube Cloud recibe los reportes de cobertura de pruebas tanto del backend en Go como del frontend en React/TypeScript:

1. **Cobertura de Go (`sonar.go.coverage.reportPaths=coverage.out`)**:
   - El objetivo `make test` en [`Makefile`](../../../../Makefile) ejecuta `go test -v -coverprofile=coverage.out -covermode=atomic ./...` contra los contenedores de servicio `postgres:16-alpine` y `redis:7-alpine` del job `ci`.
   - El archivo `coverage.out` se transfiere al job `sonarqube` mediante `actions/upload-artifact@v4` (`name: go-coverage`).
2. **Cobertura del Frontend (`sonar.javascript.lcov.reportPaths=frontend/coverage/lcov.info`)**:
   - El script `npm run test:coverage` en [`frontend/package.json`](../../../../frontend/package.json) ejecuta `NODE_V8_COVERAGE=coverage/v8 vitest run && node scripts/v8-to-lcov.mjs`.
   - [`frontend/scripts/v8-to-lcov.mjs`](../../../../frontend/scripts/v8-to-lcov.mjs) transforma los perfiles nativos V8 de Node.js en un archivo estándar `frontend/coverage/lcov.info` con rutas `SF:frontend/src/...` relativas a la raíz del repositorio y números de línea `DA:` validados contra cada archivo `.ts`/`.tsx` real, sin añadir dependencias externas ni alterar `frontend/package-lock.json`.

---

## 4. Exclusiones justificadas una por una

Ninguna exclusión en [`sonar-project.properties`](../../../../sonar-project.properties) retira código de aplicación de la API, de los workers ni del frontend:

| # | Patrón en `sonar.exclusions` | Categoría | Justificación técnica individual |
| :---: | :--- | :--- | :--- |
| 1 | `**/*_test.go` | Separación `sources` vs `tests` | En Go los archivos `*_test.go` residen en los mismos paquetes que el código productivo (`internal/`). SonarQube exige excluirlos de `sonar.sources` cuando están declarados en `sonar.test.inclusions` para evitar el fallo `File can't be indexed twice`. Siguen siendo analizados como tests. |
| 2 | `frontend/src/**/*.test.ts` | Separación `sources` vs `tests` | Archivos de pruebas unitarias de Vitest (`client.test.ts`, `names.test.ts`, `runtime.test.ts`) ubicados junto a los módulos bajo prueba en `frontend/src/`. Analizados bajo `sonar.tests`. |
| 3 | `frontend/src/**/*.test.tsx` | Separación `sources` vs `tests` | Archivo de prueba de componentes React (`lib.test.tsx`) ubicado en `frontend/src/lib/`. Analizado bajo `sonar.tests`. |
| 4 | `frontend/node_modules/**` | Dependencias de terceros | Paquetes externos descargados por `npm ci`; no son código propio del proyecto. |
| 5 | `frontend/dist/**` | Artefactos compilados | Salida transpilada y minificada generada por `vite build`. Analizar código minificado distorsionaría métricas de complejidad y duplicación. |
| 6 | `frontend/coverage/**` | Reportes generados | Directorio de salida donde se depositan los JSON de cobertura V8 y `lcov.info`. |
| 7 | `bin/**` | Binarios compilados | Ejecutables compilados de `cmd/api` y `cmd/worker`. |
| 8 | `docs/**` | Documentación y evidencias | Documentos de arquitectura, colecciones Postman y evidencias de las entregas. |
| 9 | `capacity-planning/**` | Informes y planes de carga | Planes JMeter (`.jmx`), resultados `.jtl` e informes de pruebas de capacidad. |
| 10 | `scratch/**` | Archivos temporales | Videos sintéticos `.mp4` generados con `ffmpeg` para pruebas E2E y de escalamiento. |
| 11 | `scripts/**` | Guiones operativos y arneses | Scripts de automatización de campañas de carga (`G1`–`G6`), observación de GCP y verificadores de evidencias. No forman parte de los binarios ni del bundle desplegado en producción. |
| 12 | `infra/**` | Infraestructura como código | Módulos de Terraform y scripts de provisión de VM, validados por su propio job `Terraform Format & Validate` en `.github/workflows/ci.yml`. |

---

## 5. Registro de ejecución y verificación reproducible

| Campo | Valor |
| :--- | :--- |
| **Fecha de configuración y verificación** | `2026-11-01` |
| **Rama de trabajo** | `H1-SonarQube` |
| **Commit base de la rama** | `ceecb0181216eef755796f2d77584510f80168b4` |
| **Workflow de GitHub Actions** | [`.github/workflows/ci.yml`](https://github.com/ISIS4426-2026/Plataforma-MOOC/actions/workflows/ci.yml) |
| **Proyecto en SonarQube Cloud** | [`https://sonarcloud.io/project/overview?id=ISIS4426-2026_Plataforma-MOOC`](https://sonarcloud.io/project/overview?id=ISIS4426-2026_Plataforma-MOOC) |
| **Script verificador y exportador** | [`scripts/h1_verificar_y_exportar_sonar.sh`](../../../../scripts/h1_verificar_y_exportar_sonar.sh) (`27 OK · 0 FALLA · 0 OBS`) |

### Comandos de verificación y exportación del reporte

```bash
# 1. Verificar alcance, exclusiones justificadas, pipeline CI, cobertura LCOV y ausencia de secretos:
bash ./scripts/h1_verificar_y_exportar_sonar.sh

# 2. Exportar el reporte JSON verificable desde la Web API de SonarQube Cloud tras la corrida del pipeline:
bash ./scripts/h1_verificar_y_exportar_sonar.sh --exportar-sonar
```

### Capturas de verificación (`docs/entrega3/evidencias/H1/capturas/`)

- [`capturas/2026-11-01_h1_alcance_y_exclusiones.png`](./capturas/2026-11-01_h1_alcance_y_exclusiones.png) — Verificación del alcance completo (`api`, `cmd`, `internal`, `frontend/src`: 135 archivos productivos y 63 archivos de test) y de las 12 exclusiones justificadas.
- [`capturas/2026-11-01_h1_pipeline_ci_y_secretos.png`](./capturas/2026-11-01_h1_pipeline_ci_y_secretos.png) — Verificación de los 4 jobs en `.github/workflows/ci.yml`, preservación de todos los pasos previos, atención de la Nota Técnica #15 y uso exclusivo de `${{ secrets.SONAR_TOKEN }}`.
- [`capturas/2026-11-01_h1_resumen_verificacion.png`](./capturas/2026-11-01_h1_resumen_verificacion.png) — Consolidado de los 27 controles automatizados (`27 OK · 0 FALLA · 0 OBS`).

---

## 6. Hallazgos anotados en `docs/entrega3/NOTAS_TECNICAS.md`

Se registró la **Nota Técnica #55** en [`docs/entrega3/NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md#55) documentando:
1. La necesidad de excluir `**/*_test.go` y `frontend/src/**/*.test.ts(x)` de `sonar.sources` mediante `sonar.exclusions` cuando los tests conviven en las mismas carpetas que el código productivo y se declaran en `sonar.test.inclusions` (de lo contrario el scanner falla con `File can't be indexed twice`).
2. La generación de cobertura LCOV del frontend (`frontend/scripts/v8-to-lcov.mjs`) a partir de `NODE_V8_COVERAGE` sin modificar `frontend/package-lock.json` y garantizando rutas `SF:frontend/src/...` relativas a la raíz del repositorio.
3. La desactivación obligatoria de *Automatic Analysis* en SonarQube Cloud para que el proyecto acepte ejecuciones desde GitHub Actions con reportes de cobertura.
4. El cierre de las dos obsolescencias de CI señaladas en la Nota Técnica #15 (`ubuntu-24.04`, `actions/checkout@v5`, `hashicorp/setup-terraform@v4`), evitando que entren como deuda pendiente en H3.

---

Este archivo no contiene credenciales, llaves ni secretos.

