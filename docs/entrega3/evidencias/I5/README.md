# Evidencia I5 — Tag `entrega-3` y registro del commit evaluado ([#235](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/235))

| | |
| :--- | :--- |
| **Bloque** | I — Documentación y cierre |
| **Rúbrica** | Documentación y deuda |
| **Responsable** | `@fredyxander` |
| **Depende de** | H2, I3, I4 — cerrados — **y de I6**, ver abajo |

**Estado: bloqueado por I6.** Todo lo demás está listo: la CI está verde en
`main` y 18 de los 19 issues con captura obligatoria pasan la verificación.

---

## El último paso de la entrega, y por qué va después de I6

`scripts/check_evidencias.sh` es criterio de aceptación de este issue: tiene que
salir con **código 0 sobre el commit que se taguea**. Hoy sale con 1:

```
==> 19 issue(s) con captura obligatoria
    ok       B3   2 archivo(s)      ...      ok       G6   7 archivo(s)
    FALTA    I6   la destruccion y la recreacion completa
==> Faltan 1 captura(s).
```

Así que el tag no se puede crear hasta que I6 haya capturado y mergeado. El
cuerpo de #236 dice «Depende de: I5» y está al revés; el razonamiento completo,
con las citas del enunciado, está en [`../I6/README.md`](../I6/README.md).

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

> Rellenar al crear el tag.

| | |
| :--- | :--- |
| Tag | `entrega-3` |
| SHA | |
| Fecha | |
| CI | |
| `check_evidencias.sh` | |
| Reporte de calidad correspondiente | |

El enunciado (p. 12) exige que el reporte de calidad esté «asociado al commit
final de la entrega 3». **Comprobar que el reporte de H2 y el inventario de
`deuda-tecnica.md` corresponden a este SHA**, no a uno anterior: si I6 añadió
commits después del último análisis, hay que relanzarlo.

---

## Capturas

> `capturas/AAAA-MM-DD_descripcion.png`. **Sin credenciales, tokens ni IP
> privadas.**

---

Este archivo no contiene credenciales, llaves ni secretos.
