# Evidencia I2 — Operación, recuperación, capacidad, costos y limitaciones (issue #137)

El documento es [`docs/entrega2/OPERACION_Y_CAPACIDAD.md`](../../OPERACION_Y_CAPACIDAD.md).

| Tarea / criterio | Dónde | |
| :--- | :--- | :---: |
| Despliegue, migración, verificación y reinicio | §2.2 a §2.6 | ✅ |
| Respaldo y reconstrucción (de C2) | §2.7 y §2.8 | ✅ |
| Ubicación de la configuración y manejo de secretos | §1 | ✅ |
| Configuración exacta y consumo observado frente a la estimación de B1 | §3.1, §3.2 y [`consumo_observado.md`](./consumo_observado.md) | ✅ |
| Puntos únicos de falla | §3.4 | ✅ |
| Cambios hacia una aplicación elástica | §3.5 | ✅ |
| Limitaciones del laboratorio | §3.6 | ✅ |
| **Un integrante que no desplegó puede reconstruir el entorno con el documento** | §4 | ⏳ falta hacer la prueba con una persona ajena |
| Consumo observado contrastado con la estimación fechada | §3.2: consumo físico y factura real (1,54 USD bruto, 15–27 sept.) frente a B1 (2026-09-25) | ✅ (releer la factura en 2–3 días por el retraso de facturación) |

## Pendientes, con quién puede cerrarlos

1. **Prueba de reconstrucción con una persona ajena** (§4): alguien que no haya desplegado sigue la §2.2 y anota dónde falla.
2. ~~Costo real de la factura~~ — leído (1,54 USD bruto hasta el 27 de sept.). **Releerlo pasados 2–3 días** y anotar la cifra definitiva en `consumo_observado.md`.
3. **Confirmar el presupuesto y las alertas** en la consola de Facturación.
4. **Enlazar el documento desde `README.md`** (lo hace I4).
5. ~~Reflejar en la §3.2 los números finales de H3~~ — hecho (§3.2 y §3.3, con las tres corridas del nivel de 200).

## Nunca

Credenciales, llaves ni secretos. Los datos de este directorio son tamaños, conteos y horas.
