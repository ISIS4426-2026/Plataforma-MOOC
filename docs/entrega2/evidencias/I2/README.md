# Evidencia I2 — Operación, recuperación, capacidad, costos y limitaciones (issue #137)

El documento es [`docs/entrega2/OPERACION_Y_CAPACIDAD.md`](../../OPERACION_Y_CAPACIDAD.md).

| Tarea / criterio | Dónde | |
| :--- | :--- | :---: |
| Despliegue, migración, verificación y reinicio | §2.2 a §2.6 | ✅ |
| Respaldo y reconstrucción (de C2) | §2.7 y §2.8 | ✅ |
| Ubicación de la configuración y manejo de secretos | §1 | ✅ |
| Configuración exacta y consumo observado frente a la estimación de B1 | §3.1, §3.2 y [`consumo_observado.md`](./consumo_observado.md) | ⚠️ falta el costo real de la factura |
| Puntos únicos de falla | §3.4 | ✅ |
| Cambios hacia una aplicación elástica | §3.5 | ✅ |
| Limitaciones del laboratorio | §3.6 | ✅ |
| **Un integrante que no desplegó puede reconstruir el entorno con el documento** | §4 | ⏳ falta hacer la prueba con una persona ajena |
| Consumo observado contrastado con la estimación fechada | §3.2 | ⚠️ parcial (consumo físico sí; factura no) |

## Pendientes, con quién puede cerrarlos

1. **Prueba de reconstrucción con una persona ajena** (§4): alguien que no haya desplegado sigue la §2.2 y anota dónde falla.
2. **Costo real de la factura:** quien tenga acceso a Facturación pega el costo por servicio en `consumo_observado.md`.
3. **Confirmar el presupuesto y las alertas** en la consola de Facturación.
4. **Enlazar el documento desde `README.md`** (lo hace I4).
5. ~~Reflejar en la §3.2 los números finales de H3~~ — hecho (§3.2 y §3.3, con las tres corridas del nivel de 200).

## Nunca

Credenciales, llaves ni secretos. Los datos de este directorio son tamaños, conteos y horas.
