# Evidencia I1 — `arquitectura.md`: modelo de componentes y modelo de despliegue ([#231](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/231))

El entregable es el documento, no esta carpeta: [`docs/entrega3/arquitectura.md`](../../arquitectura.md), secciones 1 a 4.

| Criterio de aceptación | Dónde |
| :--- | :--- |
| Los diagramas corresponden al despliegue efectivamente aplicado | §2.1, §2.4 y §3.1, contrastados con `infra/terraform/` en `main` (2026-11-01) y con las evidencias que cita cada fila. Nada previsto y no desplegado: el generador aparece marcado «solo durante las pruebas» |
| Los archivos fuente de los diagramas están en el repositorio | los bloques Mermaid del propio documento (§8) |
| La tabla de cambios cubre API, acceso, inicialización, workers, cola, frontend, persistencia, validación y calidad | §4, más una fila de red. Validación y calidad marcadas «en curso» con su fecha |
| Saltos de línea de Mermaid con `<br/>` | comprobado: ningún `\n` dentro de los bloques |
| Anotado en `NOTAS_TECNICAS.md` | [nota 60](../../NOTAS_TECNICAS.md#60) |

## Cómo se comprobaron los diagramas

Los tres bloques pasan el analizador de Mermaid 11 y se renderizaron con `mermaid-cli` (Chrome sin interfaz).
La topología del §3.1 se simplificó tras verla renderizada: con las flechas de Cloud NAT y de IAP hacia cada
máquina no se leía; ahora esas dos cosas, que valen para todas, se dicen en el texto bajo el dibujo.

```bash
npx -y @mermaid-js/mermaid-cli -i docs/entrega3/arquitectura.md -o arquitectura.svg
```

Las secciones 5, 6 y 7 son de I2 (#232) y quedaron como marcadores con sus títulos definitivos.

---

Este archivo no contiene credenciales, llaves ni secretos.
