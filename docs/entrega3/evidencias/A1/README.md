# Evidencia A1 — Rama base, CI y tablero del backlog ([#193](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/193))

| Tarea | |
| :--- | :---: |
| Partir del tag `entrega-2` y registrar el commit base | ✅ · §1 |
| Milestone y etiquetas nuevas | ✅ · §2 |
| CI verde en la rama base | ✅ · §3 |
| Backlog publicado con bloques, dependencias y responsables | ✅ · §4 |
| `docs/entrega3/NOTAS_TECNICAS.md` creado y sembrado | ✅ · §5 |
| Árbol `docs/entrega3/evidencias/<clave>/capturas/` | ✅ · §5 |
| Sección de Entrega 3 en el README | ✅ |

---

## 1. Commit base de la Entrega 3

| | |
| :--- | :--- |
| Tag de partida | `entrega-2` → commit `1827cdc` |
| Commit base de la Entrega 3 | `d7ac108` (`main`, 2026-10-25) |
| Commits desde el tag | 10 |
| Rama de trabajo de este issue | `docs/e3-a1-base` |

`entrega-2` es ancestro de `main`, comprobado con
`git merge-base --is-ancestor entrega-2 main`. La Entrega 3 parte del estado
evaluado en la anterior, sin reescribir historia.

## 2. Milestone y etiquetas

Milestone **`Entrega 3 - Aplicacion elastica IaaS`** (#3).

Etiquetas creadas, además de las que ya existían:

| Etiqueta | Para |
| :--- | :--- |
| `entrega-3` | los 44 issues de la entrega |
| `area:frontend` | interfaz web de estudiante y profesor |
| `area:escalamiento` | MIG, autoscaling y balanceador |
| `area:calidad` | análisis estático y deuda técnica |

## 3. Verificación de la CI

**Las seis comprobaciones de `.github/workflows/ci.yml` pasan**, reproducidas en
local sobre `d7ac108` con el mismo entorno que usa la CI:

| Comprobación | Resultado |
| :--- | :--- |
| `go fmt` | ✅ 0 archivos sin formatear |
| `go vet` | ✅ 0 hallazgos |
| `make lint` — golangci-lint v1.64.8 + Spectral 6.17.0 | ✅ 0 errores |
| `make build` | ✅ binarios de API y worker |
| `go test ./...` | ✅ todos los paquetes, `internal/worker` incluido |
| `go test ./migrations/...` | ✅ estructura, reversibilidad y ejecución en vivo |
| `terraform fmt -check -recursive` | ✅ |

Spectral deja **2 avisos, 0 errores**: `paths./modules/{module_id}.put` y
`paths./units/{unit_id}.put` no tienen `description`. La regla es *warning*, así
que no rompe la CI, pero es deuda real del contrato de la API y queda anotada
para H3.

Reproducción del entorno de la CI en una máquina de desarrollo:

```bash
docker run -d --rm -p 6379:6379 redis:7-alpine
docker run -d --rm -p 5432:5432 \
  -e POSTGRES_USER=moocuser -e POSTGRES_PASSWORD=moocpassword -e POSTGRES_DB=moocdb \
  postgres:16-alpine
export DATABASE_URL='postgres://moocuser:moocpassword@127.0.0.1:5432/moocdb?sslmode=disable'
export REDIS_URL='127.0.0.1:6379'
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
npm install -g @stoplight/spectral-cli
```

Sin Redis, los 8 tests de `internal/worker` fallan con
`dial tcp [::1]:6379: connection refused`; sin `DATABASE_URL`,
`TestMigrationsRunCleanlyOnPostgres` se salta en vez de ejecutarse. Ninguna de
las dos cosas es un fallo del código, pero conviene saberlo antes de dar por
rota una corrida local.

En Windows, `make lint` falla con `bash: D:\Fredy\Uniandes\Materias\Cloud: No
such file or directory` porque la ruta del proyecto lleva espacios y `make` no
los cita al invocar el script. Se evita llamando directamente a
`bash ./scripts/lint.sh`.

### Ejecución en GitHub Actions

| | |
| :--- | :--- |
| Ejecución | [#40215607831](https://github.com/ISIS4426-2026/Plataforma-MOOC/actions/runs/40215607831) — **success** |
| Commit | `39d947e` (rama `docs/e3-a1-base`) |
| PR | [#237](https://github.com/ISIS4426-2026/Plataforma-MOOC/pull/237) |
| `Terraform Format & Validate` | ✅ |
| `Build, Lint, Migrations & Unit Tests` | ✅ |

Dos avisos del entorno de ejecución, ajenos al proyecto y que conviene tener
presentes porque acabarán rompiendo la CI si no se atienden:

- `actions/checkout@v4` y `hashicorp/setup-terraform@v3` declaran Node.js 20, ya
  obsoleto, y el runner los fuerza a Node.js 24.
- La etiqueta `ubuntu-latest` migra a Ubuntu 26 a partir del 19 de octubre de
  2026.

Ninguno de los dos falla hoy. Quedan anotados para el inventario de deuda de H3.

## 4. Backlog

**44 issues, [#193](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/193)–[#236](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/236)**,
en nueve bloques. Cada uno lleva responsable, dependencias explícitas en los dos
sentidos, tareas, criterios de aceptación, los documentos y archivos del
repositorio que hay que mirar antes de empezar, y su directorio de evidencia.

| Bloque | | Issues |
| :--- | :--- | :---: |
| A | Preparación de la aplicación y de la línea base | 7 |
| B | Servicios compartidos fuera de los grupos elásticos | 4 |
| C | Grupos elásticos y balanceador | 7 |
| D | Frontend mínimo funcional | 5 |
| E | Integridad y estado compartido | 3 |
| F | Seguridad, operación y costos | 3 |
| G | Pruebas de carga comparativas | 6 |
| H | Calidad del código y deuda técnica | 3 |
| I | Documentación y cierre | 6 |

| Responsable | Issues | |
| :--- | :--- | :---: |
| @fredyxander | A1–A7, B1, B2, C1–C4, I5, I6 | 15 |
| @DiegoOrtizRuiz | B3, B4, C5–C7, E1, E3, F2, G1, G6, H1–H3, I2 | 14 |
| @tmichelldiaz | D1–D5, G3, G4 | 7 |
| @CrispisCas9 | E2, F1, F3, G5, G7, I1, I3, I4 | 8 |

El reparto está hecho por franja vertical, de modo que cada quien construye y
verifica lo suyo. Tres dependencias se relajaron a propósito para que nadie
arranque bloqueado: **D1** parte del contrato que ya fija `api/openapi.yaml` en
vez de esperar al balanceador, **H1** arranca con alcance Go y añade el frontend
después, y **B3** escribe el grueso del arranque antes de que cierren A2, A3 y
B1.

Arranque sin esperas: @fredyxander A1, A2, A3, A5, A6 y B1 · @DiegoOrtizRuiz B3 y
H1 · @tmichelldiaz D1. **@CrispisCas9 no tiene frente propio** hasta que aterricen
B2, C3, C4 o D2; si eso resulta incómodo, el ajuste limpio es pasarle A6.

## 5. Notas técnicas y árbol de evidencias

[`docs/entrega3/NOTAS_TECNICAS.md`](../../NOTAS_TECNICAS.md) queda sembrado con
diez hallazgos que ya se conocen, cada uno con la columna de a qué issues afecta.
Los cuatro que conviene leer antes de tocar nada:

1. **Un `apply` desde una rama borra el trabajo de otro, y no falla al hacerlo.**
   Heredado de la Entrega 2 y peor ahora: dos franjas completas sobre la misma
   configuración de Terraform y un solo estado remoto.
2. **La línea base del escenario 2 no sirve** — medida contra Docker Compose, con
   perfiles falsos y percentiles inconsistentes. La rehacen A5, A6 y A7.
3. **La caché de sesiones es local a cada VM**, así que con varias réplicas un
   logout no se propaga. Lo arregla A4.
4. **Las capturas no se pueden rehacer después de I6**, que es lo que justifica
   el criterio que bloquea el cierre en 18 issues.

El árbol `docs/entrega3/evidencias/` queda creado con un directorio por issue y
`capturas/` en los 18 que lo necesitan.
[Su README](../README.md) lleva el índice completo con responsable y qué hay que
capturar en cada uno.

---

Este archivo no contiene credenciales, llaves ni secretos.
