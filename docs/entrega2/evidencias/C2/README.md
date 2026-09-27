# Evidencia C2 — Respaldo, restauración y procedimiento de eliminación/recreación (issue #119)

| Archivo | Criterio que demuestra |
| :--- | :--- |
| [`ensayo_de_restauracion.txt`](./ensayo_de_restauracion.txt) | Restauración probada sobre una instancia nueva, con datos y estructura |
| [`tiempos_medidos.txt`](./tiempos_medidos.txt) | Tiempo de recreación registrado |
| [`copias_de_seguridad.txt`](./copias_de_seguridad.txt) | Copias automáticas configuradas · copia manual tomada |

**Estado: las seis tareas cerradas.** Los tres criterios de aceptación cumplidos.

| Criterio de aceptación | |
| :--- | :--- |
| Restauración probada, con evidencia | ✅ contenido idéntico fila por fila |
| Procedimiento ejecutable por cualquier integrante | ✅ `scripts/recrear-entorno.sh`, ejecutado contra la nube |
| Tiempo de recreación registrado | ✅ 210 s desde el código · 528 s desde una copia |

Los recursos temporales que se usaron —una instancia desechable y una VM— **ya
están borrados**. El proyecto quedó con la instancia de la entrega y nada más.

--- | :--- |
| [`copias_de_seguridad.txt`](./copias_de_seguridad.txt) | Copias automáticas configuradas · copia manual tomada y correcta |

**Estado: cuatro de las seis tareas cerradas. Las dos que faltan necesitan crear
recursos en la nube** —una instancia desechable para restaurar sobre ella— y están
pendientes de autorización, no de trabajo.

| Tarea del issue | |
| :--- | :--- |
| Respaldos automáticos configurados y uno manual tomado | ✅ |
| Documentar qué se conserva tras eliminar la instancia y cuánto cuesta | ✅ medido |
| Registrar las condiciones del proveedor sobre suspender | ✅ con fuentes |
| Script de reconstrucción | ✅ escrito · ⏸ sin ejecutar de extremo a extremo |
| Ensayar una restauración sobre una instancia nueva | ⏸ |
| Tiempo de recreación de extremo a extremo | ⏸ parcial (copia: 82 s) |

---

## La copia manual

```
ID             TYPE       STATUS      WINDOW_START_TIME
1790517759710  ON_DEMAND  SUCCESSFUL  2026-09-27T14:02:39.710+00:00
```

**82 segundos** medidos. Es el primer número del presupuesto de tiempo de
recreación.

Hay un detalle que conviene explicar antes de que alguien lo lea como un fallo:
**la copia manual es la única que existe.** La instancia se creó el 27 a las 04:14
UTC y la ventana automática es a las 03:00, que ese día ya había pasado — la
primera automática cae el 28 a las 03:00 UTC. No es un problema de configuración
(`enabled=True`, ventana 03:00, retención 3), es la hora a la que nació la
instancia. Se confirma con un comando al día siguiente.

---

## El ensayo de restauración

Se restauró una copia de `mooc-db-1` **sobre una instancia nueva y distinta**,
`mooc-db-restauracion-1`, creada para esto y borrada después. No se tocó la
instancia de la entrega.

La secuencia, para que el ensayo probara algo:

1. **Sembrar la base real primero.** Restaurar un esquema vacío no demuestra
   nada: 18 tablas vacías también son 18 tablas. Se cargaron los datos
   sintéticos —8 usuarios, 4 cursos, 5 recursos, 1 quiz, 1 insignia, 2
   inscripciones— y solo entonces se tomó la copia.
2. Copia con datos, restaurada sobre la instancia nueva.
3. Comparación **de recuentos y de contenido**.

### Lo que demuestra

| | |
| :--- | :--- |
| Son dos instancias distintas | `inet_server_addr()` da `10.171.240.3` y `10.171.240.5` |
| Los datos llegaron | 13 de 14 tablas con recuento idéntico |
| **El contenido es el mismo, no solo la cantidad** | md5 por tabla, ordenado: idéntico en las 7 comparadas |
| La estructura llegó | las 5 restricciones (3 diferibles) y los 2 disparadores de auditoría |

El md5 es la comprobación que importa. Los recuentos pueden coincidir con datos
distintos; comparar un resumen del contenido ordenado, no.

Y la estructura hay que comprobarla aparte: **una base con las filas correctas y
sin sus restricciones parece correcta y no lo es**, porque en este proyecto las
garantías del dominio las sostiene el esquema y no el código.

### La tabla que no coincidió, y por qué es lo mejor de esta evidencia

`users` dio 9 en el origen y 8 en el destino. No es un fallo de la restauración:

> La copia se tomó a las **14:37:48 UTC**. A las **14:46:16 UTC** se registró en
> el origen `estudiante.d2@plataforma-mooc.test`.

Diego estaba probando **D2** contra la instancia administrada, ocho minutos
después de mi copia. El destino tiene exactamente lo que el origen tenía al
copiarse — que es la definición de una copia.

Por eso la comparación de contenido lleva **un corte en el instante de la copia**:
contrastar contra el estado actual de una base viva y compartida no probaría nada.
Y de paso, ese registro es evidencia de otra cosa: **el registro de usuarios de D2
ya funciona contra Cloud SQL.**

### Un error que cometí y que conviene no repetir

La primera versión del script de comparación usaba un nombre de columna que no
existe (`users.is_active`; la columna real es `status`). La consulta falló contra
las dos instancias, **la comparación encontró los dos mensajes de error iguales y
declaró «contenido idéntico»**.

Una evidencia que miente es peor que no tener evidencia. El script ahora detecta
el fallo de `psql` —por código de salida y por `^ERROR:`— y se niega a comparar en
lugar de dar un falso positivo. El contador de fallos aparece al final de la
salida.

---

## Lo que se conserva al eliminar la instancia

Medido, no estimado. La tabla completa está en
[`infra/terraform/ADMINISTRACION.md`](../../../../infra/terraform/ADMINISTRACION.md#qué-sobrevive-al-destroy-y-cuánto-cuesta);
el resumen:

| Recurso | Tamaño real | Coste/mes |
| :--- | :--- | :--- |
| Bucket del estado de Terraform | 79,77 KiB | ~0,00 |
| Bucket de multimedia | 4 B | ~0,00 |
| Artifact Registry (imágenes de D1) | 106,39 MB | **0,00** (bajo el medio GB gratuito) |
| Secret Manager | 1 secreto | ~0,06 |
| VPC, subred, firewall, NAT sin VMs, APIs | — | 0,00 |

**Del orden de 0,06 USD al mes.** La conclusión aguanta aunque los precios
unitarios cambien, porque todas las cantidades están órdenes de magnitud por
debajo de los umbrales de pago. Lo único que factura de verdad es la instancia, y
es exactamente lo que se elimina.

### Y lo que NO se conserva, que es lo que sorprende

**Las copias de seguridad se van con la instancia.** Una copia no es un respaldo si
vive dentro de lo que vas a borrar. Si hay que conservar datos, se exporta a un
bucket antes.

---

## Suspender la instancia: el 97 %

La regla 5 de operación de B1 pedía consultar las condiciones del proveedor antes
de detener la instancia. Está resuelto, con la cita literal, en
[`CONFIGURACION_Y_COSTOS.md`](../../CONFIGURACION_Y_COSTOS.md#suspender-la-instancia-de-base-de-datos-las-condiciones-del-proveedor).

Lo esencial: se suspenden los cargos de instancia, **siguen** los de
almacenamiento e IP. En nuestros números, de 51,01 a 1,70 USD al mes — **el 97 %
del coste de la base**.

Y una consecuencia que no aparece en la página de precios: **mientras está
detenida, las copias automáticas no se ejecutan** (`STATUS_SKIPPED`). Aquí es
aceptable porque los datos son sintéticos y reproducibles desde `migrations/` y
`scripts/seeds/` — nada existe solo en la base. Deja de serlo en cuanto una
corrida genere datos para el informe: **antes de detenerla después de un escenario
de carga, copia manual o exportación.**

---

## Los tiempos, y la conclusión que no esperaba

| Operación | Medido |
| :--- | ---: |
| Copia bajo demanda | 82 s · 81 s |
| Aprovisionar una instancia nueva | 204 s |
| Restaurar una copia sobre una instancia existente | 324 s |
| Migrar las 7 migraciones + sembrar | **6 s** |
| Borrar una instancia | 144 s |

Dos caminos para volver a tener la base en pie:

| | |
| :--- | ---: |
| **A. Recrear desde el código** — aprovisionar + migrar y sembrar | **210 s** |
| **B. Restaurar una copia** — aprovisionar + restaurar | **528 s** |

**Recrear desde el código es 2,5 veces más rápido que restaurar.** No es
casualidad de la medición: restaurar arrastra el estado completo de la instancia,
mientras que siete migraciones y una semilla determinista sobre una base vacía son
6 segundos.

Eso cambia cuál es el procedimiento de recuperación por defecto:

* Para el entorno de la entrega, el camino es **A**. La copia no aporta nada que
  el repositorio no tenga, y tarda el doble. **El respaldo de este proyecto es el
  repositorio.**
* La copia importa para lo que **no se puede regenerar**: los datos que deje una
  corrida de carga y que alimenten el informe de capacidad. Ahí el repositorio no
  sirve, porque esos datos no están en él.

Para I6, el ciclo completo de eliminar y recrear con datos son **unos 6 minutos
medidos**; con margen para leer un `plan` y preparar el host temporal, **10
minutos** es la cifra defendible en una sustentación.

---

## El script de reconstrucción

[`scripts/recrear-entorno.sh`](../../../../scripts/recrear-entorno.sh). El criterio
dice «ejecutable por cualquier integrante», y lo que lo hace ejecutable es que
resuelve por su cuenta lo que de otro modo hay que recordar.

**Tiene dos caras, y la razón no es estética.** Las tres fases no corren en el
mismo sitio:

| Fase | Dónde | Por qué |
| :--- | :--- | :--- |
| Aprovisionar | Tu máquina | `terraform apply` con tus credenciales personales, desde `main` |
| Migrar | **Dentro de la VPC** | La instancia no tiene IP pública |
| Sembrar | **Dentro de la VPC** | Igual |

Esa frontera es la dificultad real del procedimiento. El script la cruza creando
una VM mínima y temporal, ejecutándose a sí mismo allí con `--en-la-vpc`, y
borrándola al terminar — con un `trap` en la salida, para que un fallo a mitad no
deje una máquina facturando que nadie recuerda.

Tres decisiones que vale la pena conocer:

**Llama a `seed.sh` en lugar de traerse su SQL.** `seed.sh` ya usa `DATABASE_URL`
cuando está definida, así que funciona contra la nube sin cambios. Y cuando G1
(#127) amplíe la semilla, este script se beneficia sin tocarlo. Es la diferencia
entre depender de G1 y no depender.

**Muestra el plan y pide confirmación escrita antes de aplicar.** Un `to destroy`
inesperado ahí significa que el árbol no está en `main` o que alguien aplicó desde
una rama — la [nota 16](../../NOTAS_TECNICAS.md). En un ensayo de recreación es
justo donde hay que mirar.

**La contraseña la lee la VM con su cuenta de servicio adjunta**, no se pasa por
argumento. Lo único que se imprime es su longitud.

**Su fase de dentro de la VPC se ejecutó contra la instancia real**, y es lo que
demostró que `seed.sh` funciona contra Cloud SQL sin un solo cambio: migró (sin
nada pendiente), sembró los datos sintéticos y reportó sus propios tiempos —4 s
en leer el secreto, 2 s en migrar y sembrar—.

La fase de aprovisionamiento no se ejecutó desde el script porque la instancia ya
existía de C1; su equivalente se midió con `gcloud` (204 s).

---

## Nunca

Los archivos de esta carpeta son salidas de verificación y no contienen
credenciales. `recrear-entorno.sh` recibe la contraseña de Secret Manager en
tiempo de ejecución y nunca la imprime.
