# Evidencia I5 — Tag `entrega-3` y registro del commit evaluado ([#235](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/235))

| | |
| :--- | :--- |
| **Bloque** | I — Documentación y cierre |
| **Rúbrica** | Documentación y deuda |
| **Responsable** | `@fredyxander` |
| **Depende de** | H2, I3, I4 — cerrados — **y de I6**, ver abajo |

**Estado: completo.** Tag publicado el 2026-11-02 sobre `9df67ea`,
con la CI en verde, `check_evidencias.sh` en 0 y el reporte de calidad
correspondiente al mismo commit.

---

## El último paso de la entrega, y por qué va después de I6

`scripts/check_evidencias.sh` es criterio de aceptación de este issue: tiene que
salir con **código 0 sobre el commit que se taguea**. Antes de I6 salía con 1,
y el único que faltaba era precisamente I6:

```
==> 19 issue(s) con captura obligatoria
    ok       B3   2 archivo(s)      ...      ok       G6   7 archivo(s)
    FALTA    I6   la destruccion y la recreacion completa
==> Faltan 1 captura(s).
```

Por eso el tag no podía crearse hasta que I6 capturara y mergeara. El cuerpo de
#236 decía «Depende de: I5» y estaba al revés —se corrigió en GitHub el
2026-11-02—; el razonamiento completo, con las citas del enunciado, está en
[`../I6/README.md`](../I6/README.md).

Con I6 cerrado, el script pasó a **código 0** y el tag se creó sobre ese commit.

---

## Procedimiento

Todo esto se hace **después** del merge de I6, sobre `main` limpio.

```bash
git checkout main && git pull
git status --short          # tiene que estar vacío

bash ./scripts/check_evidencias.sh    # DEBE salir con 0; si no, para aquí
```

> 📸 **Captura 1** → `capturas/AAAA-MM-DD_check-evidencias-en-cero.png`

Comprobar que la CI está verde en ese commit exacto:

```bash
SHA="$(git rev-parse HEAD)"
gh run list --repo ISIS4426-2026/Plataforma-MOOC --commit "$SHA" \
  --json conclusion,name,headSha
```

> 📸 **Captura 2** → `capturas/AAAA-MM-DD_ci-verde-en-el-commit.png`

Crear el tag y publicarlo:

```bash
git tag -a entrega-3 -m "Entrega 3 — Aplicación Elástica IaaS"
git push origin entrega-3
```

> 📸 **Captura 3** → `capturas/AAAA-MM-DD_tag-publicado.png`

Y registrar el SHA donde el enunciado lo pide (p. 13, «registrar el commit
evaluado»): en este README, en `docs/entrega3/deuda-tecnica.md` y en el
`README.md` de la raíz.

---

## Commit evaluado

| | |
| :--- | :--- |
| Tag | [`entrega-3`](https://github.com/ISIS4426-2026/Plataforma-MOOC/releases/tag/entrega-3), anotado, publicado en el remoto |
| SHA | `9df67ea2ca9fb2c29adf8030ed294d4f38adc4d3` |
| Fecha | 2026-11-02, 15:50 -05:00 |
| CI | [verde](https://github.com/ISIS4426-2026/Plataforma-MOOC/actions/runs/41522946344): `Build, Lint, Migrations & Unit Tests`, `Frontend`, `Terraform Format & Validate` y `SonarQube Static Analysis` |
| `check_evidencias.sh` | **código 0** — 19 de 19 issues con captura obligatoria |
| Reporte de calidad correspondiente | sí, ver abajo |

### El reporte de calidad sí corresponde a este commit

El enunciado (p. 12) exige que el reporte esté «asociado al commit final de la
entrega 3». Se cumple por dos caminos independientes:

1. **SonarQube analizó este commit.** La corrida de CI de `9df67ea`
   incluye el trabajo `SonarQube Static Analysis (API, Workers & Frontend)` y
   terminó en verde.
2. **La exportación versionada sigue siendo válida.** El JSON de la línea base
   (`evidencias/H3/reporte_sonarcloud_commit_final.json`) se exportó en
   `3e7d4c5`, y entre ese commit y el tag **no cambió ni un fichero de código de
   aplicación**:

   ```bash
   git diff --name-only 3e7d4c5..9df67ea -- internal/ cmd/ frontend/src api/ go.mod go.sum
   # sin salida: 0 ficheros
   ```

   Las métricas no pueden haber variado porque las fuentes analizadas son
   idénticas. Lo que entró después fue documentación y tres arreglos de guiones
   e infraestructura (`database.tf`, `recrear-entorno.sh`,
   `verificar_endpoints_nube.sh`), ninguno bajo `sonar.sources`.

---

## Capturas

> `capturas/AAAA-MM-DD_descripcion.png`. **Sin credenciales, tokens ni IP
> privadas.**

---

Este archivo no contiene credenciales, llaves ni secretos.
