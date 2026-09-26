# Evidencia B1 — Configuración efectiva y marco de costos (issue #114)

Las decisiones están en [`../../CONFIGURACION_Y_COSTOS.md`](../../CONFIGURACION_Y_COSTOS.md).
Aquí van las capturas que las respaldan.

## Estado

| Archivo | Qué demuestra | |
| :--- | :--- | :--- |
| `presupuesto_y_alertas1..4.PNG` | El presupuesto paso a paso: *Alerts only*, **Custom range**, **Savings desmarcadas**, 50 USD y los cuatro umbrales | ✅ |
| `creditos_y_expiracion.png` | Saldo 50,00 de 50,00 USD y vigencia hasta 2027-08-19 | ✅ |
| `calculadora_estimacion.csv` | Desglose por SKU con **fecha efectiva embebida** (2026-09-26T18:17:45Z) y total 133,72805 USD | ✅ |
| `calculadora_estimacion.png` | Total mensual en pantalla | ✅ |
| `bitacora_configuracion.txt` | Registro del procedimiento seguido y las decisiones tomadas en cada paso | ✅ |
| `latencia_us-east1.txt` | Latencia desde Bogotá: `us-east1` 74 ms, la mejor de todas; São Paulo 178 ms | ✅ |
| `latencia_gcping.png` | La tabla de gcping.com tal como se midió | ✅ |

Los números **ya están volcados** en
[`../../CONFIGURACION_Y_COSTOS.md`](../../CONFIGURACION_Y_COSTOS.md) y contrastados
uno a uno contra el CSV.

El CSV es la evidencia más fuerte de las tres: trae el precio por SKU, la región de
cada línea y la fecha efectiva, así que la estimación queda fechada y verificable
sin depender de ninguna captura.

## La estimación, ya calculada

**Enlace verificable:** <https://cloud.google.com/products/calculator?dl=CjhDaVEwTVRWbE9HRTJaUzB6TVRFNExUUTBZalV0WWpNeE15MDRObUZsWlRKbE1tWmhPVGdRQVE9PRAOGiRFNDVDOUIwOS1EODRDLTRGRTUtOTk5MC0xRkZCMDk1NTU4QjI>

Es la evidencia principal de esta línea: el evaluador lo abre y comprueba cada
renglón, sin depender de una captura.



Consultada el 2026-09-26 con precios de lista, sin vincular la cuenta de
facturación para que sea reproducible. Si hay que rehacerla, esta es la
configuración exacta:

| Línea | Configuración | USD/mes |
| :--- | :--- | ---: |
| Web Server | Compute Engine, `us-east1`, `e2-highcpu-2`, 730 h, disco 30 GiB balanced, OS libre, Regular, sin CUD | 39,11 |
| Worker Server | Idéntica | 39,11 |
| Cloud SQL | PostgreSQL Enterprise, `us-east1`, 1 vCPU / 3,75 GiB, **zona única sin HA**, SSD 10 GiB, 730 h | 51,01 |
| Cloud Storage | Region `us-east1`, Standard, 3 GiB, clase A 0,001 M, clase B 0,25 M | 0,08 |
| IP | 2 IPv4 en uso en VM estándar, `us-east1` | 3,65 |
| Egreso | Internet, Premium tier, `us-east1` → Sudamérica, 5 GiB | 0,76 |
| **TOTAL** | | **133,73** |

Cotizado aparte y **no** incluido: Cloud NAT 4,90 · egreso de 192 GiB 36,29 ·
2 IPv4 reservadas con las VMs apagadas 14,59.

> Cuidado con las unidades en Cloud Storage: los campos de operaciones son **en
> millones**. 250 000 operaciones se escriben `0.25`, no `250000`.

## Por qué la captura del presupuesto debe mostrar el ajuste de créditos

Si el presupuesto calcula el costo **incluyendo** los créditos promocionales, el
cupón absorbe el consumo, el costo ronda cero y las alertas no se disparan hasta
que el crédito ya se agotó. La captura tiene que dejar constancia de que se
configuró **excluyendo** los créditos, o la evidencia mostraría un presupuesto que
no avisa de nada.

## Nunca

Credenciales, llaves, archivos de cuentas de servicio, `.env`, cadenas de
conexión con contraseña ni tokens. El docente aclaró que a eso se refiere el
enunciado con «las evidencias no deben aparecer en el repositorio».

Identificadores de proyecto y de recursos sí pueden aparecer. En las capturas de
facturación conviene revisar que no se vea el número completo de la cuenta de
facturación ni datos de pago.
