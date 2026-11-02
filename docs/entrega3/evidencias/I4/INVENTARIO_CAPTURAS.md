# I4 — Inventario de capturas para el vídeo de sustentación ([#234](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/234))

El vídeo se monta sobre las capturas que cada issue guardó **mientras ocurría**, no volviendo a provocar los ciclos
(el issue lo pide así: relanzar un escalamiento cuesta dinero y después de I6 el entorno ya no existe). Esta tabla
cruza cada punto que la sustentación tiene que mostrar con lo que ya está en el repositorio, y dice qué falta y de
quién es, para reclamarlo **antes** de que se cierre el issue que lo produce.

**Inventario del 2026-11-01, 18:15 (hora de Bogotá).** `bash ./scripts/check_evidencias.sh --informe` daba 14 de 19
issues con captura obligatoria completa; con la de C7 (PR #296), 15.

## Lo que la sustentación tiene que mostrar, y con qué

| # | Punto del vídeo (I4 y enunciado) | Capturas disponibles | Estado |
| ---: | :--- | :--- | :---: |
| 1 | Arquitectura desplegada | diagramas de [`arquitectura.md`](../../arquitectura.md) §2.1 y §3.1; balanceador en [`C3/`](../C3/capturas/) (`balanceador-cadena-completa`, `certificado-valido`) | ✅ |
| 2 | Recorrido del estudiante desde el frontend | [`D2/`](../D2/capturas/): catálogo, inicio, aula con vídeo HLS, PDF, cuestionario aprobado, curso aprobado (nube, 29 y 31 oct) | ✅ |
| 3 | Recorrido del profesor desde el frontend | [`D3/`](../D3/capturas/): mis cursos, vídeo subido, editor, curso publicado (nube, 31 oct) | ✅ |
| 4 | **Recorrido conectado profesor → estudiante** | `D5/capturas/` **vacío**: su README remite a este vídeo | ❌ **falta** — ver abajo |
| 5 | Tráfico distribuido entre réplicas de API | [`C4/`](../C4/capturas/) (`dos-replicas-sanas`, `reparto-y-exclusion-terminal`); [`E1/`](../E1/capturas/) (`sesion-atendida-por-dos-instancias`) | ✅ |
| 6 | Incorporación y retiro automáticos — **API** | `G5/capturas/` **vacío** | ❌ **falta** — G5, sesión pendiente |
| 7 | Incorporación y retiro automáticos — **workers**, y procesamiento por workers nuevos | [`G6/`](../G6/capturas/) (ciclo, consumo por workers nuevos, drenaje y reducción, auditoría sin intervención manual); [`C6/`](../C6/capturas/) (`grupo-tamano-1-3-1`, `outstanding-sube-y-baja`) | ✅ |
| 8 | Recuperación segura de trabajos | [`C7/`](../C7/capturas/) (drenaje con trabajo activo, PR #296); [`E1/`](../E1/capturas/) (`worker-retirado-con-trabajo-activo-lo-termina-el-reemplazo`) | ✅ |
| 9 | PostgreSQL administrado, objetos y continuidad durante los cambios de capacidad | [`E1/`](../E1/capturas/) (`retiro-de-una-api-bajo-trafico-85-de-85`); [`E2/`](../E2/capturas/) (consistencia entre réplicas); [`C1/`](../C1/capturas/) (autohealing) | ✅ |
| 10 | Arranque automático de instancias | [`B3/`](../B3/capturas/) (`arranque-desde-metadata`, `arranque-hasta-listo`) | ✅ |
| 11 | Resultados comparativos | [`G3/`](../G3/capturas/) (`campana-completa-instancias-cpu-balanceador`); G4 y el informe de G7 | 🟡 **G4 y G7 pendientes** |
| 12 | Calidad y deuda técnica: línea base y plan | [`H1/`](../H1/capturas/), [`H2/`](../H2/capturas/), [`H3/`](../H3/capturas/) | ✅ |
| 13 | Seguridad de red | `F1/capturas/` **vacío**; identidades en [`F2/`](../F2/capturas/) | ❌ **falta** — F1, Tania |

## Lo que falta, de quién es y para cuándo

| Falta | Issue · responsable | Por qué no puede esperar |
| :--- | :--- | :--- |
| Ciclo de escalado de la **API**: las seis capturas del runbook | **G5** · @CrispisCas9 | Solo se puede capturar con el entorno encendido; después de I6 no existe. Es lo único que falta del punto 6 |
| **Barrido externo** con fecha | **F1** · @tmichelldiaz | Igual: entorno encendido |
| **Recorrido conectado** en vídeo corto (≤ 10 MiB) | **D5** · @tmichelldiaz | D5 está **cerrado** sin captura y `check_evidencias.sh` lo marca. Si se graba durante la sustentación, el clip se guarda también en `D5/capturas/`; pero tiene que grabarse **con el entorno encendido**, así que conviene hacerlo en la misma sesión que G5 o F1 |
| Gráficas por etapa del escenario 2 | **G4** · @tmichelldiaz | Entran en el informe de G7 y en el punto 11 |
| Destrucción y recreación | **I6** · @fredyxander | Es lo último; no bloquea el vídeo, que se graba antes |

## Cómo se repite este inventario

```bash
bash ./scripts/check_evidencias.sh --informe
```

y, para ver todas las capturas disponibles por issue:

```bash
for d in docs/entrega3/evidencias/*/capturas; do
  n=$(ls "$d" | grep -vc gitkeep); [ "$n" -gt 0 ] && echo "$(basename "$(dirname "$d")"): $n"
done
```

---

Este archivo no contiene credenciales, llaves ni secretos.
