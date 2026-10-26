# Evidencia A6 — Criterios de medición y validación de métricas ([#198](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/198))

| Criterio de aceptación | |
| :--- | :---: |
| Documento de criterios escrito y fechado antes de la primera corrida | ✅ §1 |
| El script que agrega resultados falla si los percentiles no son monótonos | ✅ §3 |
| Toda corrida registra su URL de destino; sin destino no se publica | ✅ §4 |
| Ninguna métrica publicada se deriva de una constante sumada a otra | ✅ §5 |
| Los criterios aplican por igual a la línea base y a las corridas de la Entrega 3 | ✅ §1 |
| Anotado en `NOTAS_TECNICAS.md` | ✅ nota 25 |

Documento acordado:
[`capacity-planning/criterios_de_medicion.md`](../../../../capacity-planning/criterios_de_medicion.md),
fechado **2026-10-25**, antes de cualquier corrida de esta entrega.

---

## 1. El documento, y a qué se aplica

No redefine *qué* se mide —eso sigue en `escenario1.md` y `escenario2.md`— sino
**cómo se mide y qué puede publicarse**. Se aplica igual a la línea base
retrospectiva de A7 que a las corridas nuevas: si la línea base se midiera con
otras reglas, la comparación que pide la Entrega 3 no significaría nada.

Los umbrales de éxito y parada de los dos planes **no se tocan**. Lo que se
añade es lo que la infraestructura elástica trae consigo: degradación durante el
escalamiento, tiempo hasta que la capacidad nueva sirve, exclusión de las sondas
de salud y descarte de una corrida en la que el autosanado recreó una instancia.

## 2. Por qué no bastaba con escribirlo

Las reglas están en
[`scripts/capacity_criterios.py`](../../../../scripts/capacity_criterios.py) y se
ejecutan **dentro de los scripts que publican las tablas**. Un criterio que vive
solo en un documento se olvida a las dos corridas; y una revisión aparte es
exactamente lo que no ocurrió en la Entrega 2.

```
$ python scripts/capacity_criterios.py --autoprueba
ok  reproducido: con n=2 el estimador viejo da p50=5000 y p95=1000
    (el p95 era el valor minimo de la muestra)
ok  el estimador nuevo es monotono para n = 1..399
ok  con 8 observaciones el p95 sale 'n/d (n=8)', no un numero
ok  con 120 observaciones los tres percentiles se publican
ok  la puerta de monotonia aborta con los percentiles de la Entrega 2
ok  el destino de la Entrega 2 (http://api:8080) se clasifica como local

Todas las reglas se cumplen.
```

La autoprueba **reproduce el defecto** antes de comprobar la corrección. Si
alguien cambiara el estimador por otro que se cruza, la primera línea falla.

## 3. El origen de los p95 por debajo del p50

Había **tres estimadores de percentil distintos en cuatro archivos**:

| Archivo | p50 | p95 |
| :--- | :--- | :--- |
| `capacity_resumen_jtl.py` | ⌊p/100·n + 0,5⌉ − 1 | igual |
| `capacity_metricas_gcp.py` | — | igual que el anterior |
| `capacity_analisis_h3.py` | `v[n//2]` | `v[int(0,95·n)]` |
| `capacity_escenario2/main.go` | `v[n/2]` | `v[int((n−1)·0,95)]` |

El de la campaña del escenario 2 usa **una fórmula por percentil**. Con n=2 eso
da índice 1 para el p50 e índice **0** para el p95:

> **el p95 publicado era el valor mínimo de la muestra.**

Y n=2 no es un caso raro: los niveles del escenario 2 subían 1, 2, 4, 8 y 12
vídeos. Es justo el régimen donde el defecto aparece.

Ahora hay un estimador —rango más cercano con techo— replicado en Python y en
Go, monótono por construcción, y la comprobación corre en el mismo script que
genera las tablas.

### Lo que esto descubre de paso

La regla de observaciones mínimas (n ≥ 1/(1−X): 20 para un p95, 100 para un p99)
**invalida todos los p95 por etapa del escenario 2**, no solo los invertidos. Con
12 subidas como máximo, ningún nivel llega a 20 observaciones. No es que
estuvieran mal calculados —además lo estaban—: es que no existían.

Para **G4** eso es una decisión de diseño, no un detalle: o un nivel sube 20
vídeos o más, o sus etapas se publican con mediana, mínimo, máximo y
observaciones en crudo.

### Dos ventanas de agregación en la misma tabla

Encontrado al fijar las unidades: el resumen dividía el rendimiento por la
distancia entre los **inicios** de las peticiones —lo que deja fuera la última—
mientras publicaba como duración una ventana que **sí** incluía el calentamiento.
Ahora hay una sola ventana, declarada en cada resumen.

## 4. Destino registrado, y sin valor por omisión

El defecto más caro de la Entrega 2 no fue un cálculo: fue publicar como «nube»
una corrida cuyo destino era `http://api:8080`. Nada en la salida cruda decía
contra qué se había medido, así que nada lo contradecía.

- Cada corrida escribe `destino.json` junto a sus datos crudos: URL efectiva,
  entorno, commit, generador y ventana UTC.
- El resumen **se niega a producir tablas** si falta, y marca
  `"publicable": false`.
- La clase del destino se **deriva del host**, no se declara: `api`, `minio`,
  `localhost` y `host.docker.internal` son `local`.
- Con `EXIGIR_NUBE=1` el resumen falla si el destino es local. `h3_nube.sh` lo
  activa solo en modo nube.
- **Ya no hay destino por omisión** en ninguna parte:

```
$ go run ./scripts/capacity_escenario2
Falta -base. No hay destino por omision: declara contra que mides.
  ej. -base https://<host> -hls-base https://storage.googleapis.com/<bucket> -entorno nube-entrega3
```

`h2_nube.sh` y `h3_nube.sh` tenían la IP de la VM de la Entrega 2 como valor por
omisión. Esa IP ya no es la de ningún entorno de la Entrega 3, y heredarla en
silencio es medir una cosa y publicar otra.

Lo mismo con la base de los derivados HLS: se adivinaba **comparando la URL con
esa IP literal**. Recreado el entorno, la campaña habría medido los derivados de
MinIO creyendo medir los del bucket.

## 5. Tres métricas derivadas de una constante, no una

La revisión marcó el tiempo al primer cuadro. Buscando cómo se producía
aparecieron otras dos, en la misma sección del informe y las tres presentadas
como medidas con un reproductor real:

| Métrica publicada | Cómo se calculaba |
| :--- | :--- |
| Tiempo al primer cuadro, 48 a 52 ms | latencia HTTP **+ 45 ms** |
| Margen de buffer, +5,99 s | **6,0** − latencia de segmento |
| Interrupciones, 0 eventos | un segmento tardando más de **6 s**, descargando a máxima velocidad |

La tercera es la más engañosa: con descarga sin pausa, cero interrupciones estaba
garantizado por aritmética. No se observó nada.

Ahora el margen de buffer se mide contra el instante en que la reproducción
necesita cada segmento, una interrupción es un margen negativo, y ambos salen
como «no aplica» cuando el patrón no tiene cadencia. El tiempo al primer cuadro
**no se publica**: la campaña mide transporte, y la cifra honesta se llama
*primer segmento descargado*. Cómo medirlo de verdad está en la §6 del documento
de criterios; lo implementa G4.

## 6. El consumo HLS declarado nunca ocurrió

`runHLSStreamingSimulation` recibía la cadencia y el número de estudiantes y **no
usaba ninguno de los dos**. Se ve sin interpretar nada: los dos nombres aparecen
exactamente una vez en todo el archivo, en la firma, y en ninguna línea más.

```
$ git show <tag-entrega-2>:scripts/capacity_escenario2/main.go | grep -n "pacingSec\|streamers"
773:func runHLSStreamingSimulation(client *httpClient, hlsBase, stableID string, pacingSec float64, streamers int) HLSStreamResult {
```

Un solo lector secuencial, sin pausas, leyendo **tres** segmentos. Los niveles
declaraban 2, 10, 25, 50 y 100 estudiantes con cadencia de 6,0 s, y el informe
publicó esas cifras.

Ahora cada estudiante es una goroutine, la pausa se aplica cuando el patrón es
`cadencia`, y la duración del segmento **se lee del `#EXTINF` de la lista** en
vez de suponerse. El 6,0 supuesto se equivocaba un 18 %: los segmentos duran
~7,06 s (nota técnica 24).

Con el patrón corregido, el consumo HLS pasa a ser la **única** parte del
escenario 2 con observaciones de sobra para un p95: 25 estudiantes por 34
segmentos son 850 medidas, frente a las 8 de las etapas de subida.

## 7. Y aquí nacieron los clips de 8 segundos

Buscando quién producía los archivos que A5 corrigió:

```bash
# scripts/run_capacity_escenario2.sh, antes de este issue
echo "    Generando video_corto.mp4 (8s, 1280x720, 25fps)..."
ffmpeg ... -i "testsrc=size=1280x720:rate=25:duration=8" ...
```

El propio ejecutor de la campaña los generaba, con las duraciones escritas a
mano, mientras el plan declaraba 2, 10 y 30 minutos. **Nadie comparaba una cosa
con la otra.** A5 arregló el generador bueno; esto era el que se usaba.

Ahora delega en `generar_media_prueba.sh`, que genera **y verifica** campo por
campo, y la corrida no arranca si un archivo no es lo que dice.

## 8. Lo que falta

Nada de este issue. Las reglas no se pueden *demostrar sobre una corrida real*
hasta que haya entorno: eso ocurre en **A7**, que es la primera que las ejecuta
de principio a fin, y en **G3** y **G4**, que publican con ellas.

Lo que sí queda hecho y verificable sin entorno es la autoprueba, los guardas de
destino y el que la campaña se niegue a correr sin declarar contra qué mide.

---

Este archivo no contiene credenciales, llaves ni secretos.
