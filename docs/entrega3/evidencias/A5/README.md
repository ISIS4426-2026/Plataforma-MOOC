# Evidencia A5 — Archivos multimedia con los tres perfiles reales ([#197](https://github.com/ISIS4426-2026/Plataforma-MOOC/issues/197))

| Criterio de aceptación | |
| :--- | :---: |
| Los tres archivos existen y su `ffprobe` coincide campo por campo | ✅ §2 |
| Ningún archivo es un clip corto presentado como un perfil largo | ✅ §2 |
| Ningún archivo tiene la resolución inflada respecto de su original | ✅ §3 |
| La siembra es reproducible desde un script versionado | ✅ §4 |
| Coste y duración de una corrida con el perfil mayor, estimados | ✅ §5 |

---

## 1. Dónde estaba el defecto, exactamente

El revisor escribió: *«los perfiles declarados de 2, 10 y 30 minutos fueron
clips de 8, 20 y 40 segundos»*.

Lo primero que encontré es que **el sembrador nunca estuvo mal**.
`cmd/seed-media/main.go` declara `2*60`, `10*60` y `30*60` segundos, y la tabla
de costes de la Entrega 2 está calculada para esos tamaños. Lo que fallaba es
que **la campaña no usaba sus archivos**:

```go
// scripts/capacity_escenario2/main.go
flag.StringVar(&cfg.VideoCorto, "video-corto", "scratch/video_corto.mp4", ...)
```

Leía de `scratch/` lo que hubiera, y lo que había eran clips generados a mano
siguiendo el procedimiento de `EJECUCION_PRUEBAS_MULTIMEDIA.md`, que produce
**8 segundos**. Nada comprobaba la diferencia entre lo declarado y lo subido.

> En la misma línea está el otro defecto que marcó el revisor:
> `-base http://api:8080` como destino por omisión, que es Docker Compose. Lo
> cierra **A6**.

## 2. Los tres archivos, verificados campo por campo

`scripts/generar_media_prueba.sh` genera y **verifica**. Un archivo que no
coincide no se da por bueno: el script sale con error y dice en qué campo.

```
==> Verificando contra el perfil declarado
    ok  corto   120s  1280x720  h264/aac   2 MiB
    ok  medio   600s  1280x720  h264/aac  12 MiB
    ok  largo  1800s  1280x720  h264/aac  37 MiB

==> Los 3 perfil(es) coinciden campo por campo.
```

Y coinciden con la tabla que la Entrega 2 ya había declarado y costeado:

| Perfil | Declarado | Medido | Bitrate |
| :--- | ---: | ---: | ---: |
| Corto | 2 min · ~2,6 MB | 120,0 s · **2,60 MB** | 174 kbps |
| Medio | 10 min · ~13,0 MB | 600,0 s · **13,01 MB** | 174 kbps |
| Largo | 30 min · ~39,0 MB | 1800,0 s · **39,06 MB** | 174 kbps |

Las salidas completas de `ffprobe` están en este directorio, una por perfil.

**Lo que esto confirma:** la tabla de costes y el sembrador estaban bien. El
único eslabón roto era el archivo que la campaña subía.

## 3. Resolución nativa, sin inflar

Los tres se generan **nativos a 1280×720**, no reescalados desde algo menor. Un
720p agrandado desde 360p cuesta la misma CPU y no produce el trabajo que se
quiere medir, solo lo aparenta — el enunciado lo prohíbe explícitamente.

720p es además el techo exacto que declara `internal/transcode.DefaultLadder`,
así que la escalera 360p + 720p se aplica sin que nada se agrande.

## 4. Reproducible

```bash
bash ./scripts/generar_media_prueba.sh              # los tres
bash ./scripts/generar_media_prueba.sh largo        # solo uno
bash ./scripts/generar_media_prueba.sh --verificar  # no genera, solo comprueba
```

Usa los mismos parámetros de codificación que `cmd/seed-media`. Que coincidan
importa: si la campaña midiera archivos codificados de otra forma, estaría
midiendo otra cosa.

Si no hay `ffmpeg` en el PATH tira de Docker, así que no hace falta instalarlo
—en este equipo no estaba— y además todos usan la misma versión, lo que hace
los tiempos comparables entre máquinas.

Los `.mp4` **no se versionan**: el largo son 39 MB y `scratch/` está en
`.gitignore`. Lo versionado es el script. Las salidas de `ffprobe` sí se
conservan, porque pesan 4 KB.

## 5. Coste y duración, medidos antes de lanzar

### Transcodificación

Medido de verdad, no estimado: la escalera real (360p + 720p, mismos argumentos
que `internal/transcode`) sobre el perfil corto, acotado a **1 vCPU**:

```
=== transcodificando el perfil CORTO a la escalera real, 1 vCPU ===
real    2m21.701s
```

**141 segundos para un vídeo de 120 — unas 1,2 veces el tiempo real.**
Extrapolando linealmente:

| Perfil | Transcodificación en 1 vCPU |
| :--- | ---: |
| Corto | ~2,4 min |
| Medio | ~12 min |
| Largo | **~35 min** |

### Esto invalida las tasas de servicio de la Entrega 2

El resumen del escenario 2 declara una **tasa de servicio de 12,29 a 123,87
vídeos/min**. Con los perfiles reales, un worker de 1 vCPU rinde **0,43
vídeos/min** en el perfil corto.

La diferencia es de **30 a 290 veces**. No es un error de medición: es
aritméticamente imposible transcodificar 124 vídeos de dos minutos en un minuto
con dos vCPU. Esos números salen de los clips de 8 segundos, y **no se pueden
comparar con nada que se mida con perfiles reales**. A7 tiene que rehacerlos.

### Duración de una corrida con el perfil largo

El plan del escenario 2 declara el Nivel 3 con inyección de **~3,0 vídeos/min**
mezclando los tres perfiles, y dos workers de 1 vCPU.

| | |
| :--- | :--- |
| Tasa de servicio real, dos workers, perfil corto | ~0,85 vídeos/min |
| Tasa de llegada declarada en Nivel 3 | 3,0 vídeos/min |
| Acumulación | ~2,2 vídeos/min |

Diez minutos de inyección dejan ~22 vídeos en cola. Drenarlos cuesta **~26
minutos más** si todos fueran cortos; con el perfil largo en la mezcla, cada uno
añade 35 minutos de una vCPU.

> **Una corrida de Nivel 3 con el perfil largo se mide en horas, no en minutos.**
> Hay que decidirlo al planificar G4: o se reduce la tasa de inyección, o se
> acota cuántos largos entran, o se asume la duración y el coste de cómputo.
> Con los clips de 8 segundos esto no se veía.

### Almacenamiento

Medido sobre el perfil corto: **33 MB de derivados** (14 MB en 360p, 20 MB en
720p) frente a los 53 MB que estimaba la tabla. La estimación era conservadora
porque suponía el bitrate completo de la escalera.

### Un defecto encontrado de paso: el recuento de segmentos

La tabla declara `2 × ⌈D/6⌉` segmentos, 40 para el perfil corto. **Salen 34**,
17 por rendición.

El worker fija `-g` a `segmentSeconds()*30 = 180` fotogramas, asumiendo 30 fps.
El vídeo va a **25 fps**, así que el GOP dura 7,2 s y los segmentos se alinean a
fotograma clave: ~7,06 s en vez de 6.

Consecuencia: **el recuento de objetos y la estimación de almacenamiento se
pasan un 20 %**. No rompe nada —los segmentos son válidos y el reproductor los
consume igual—, pero la cifra declarada no es la real. Se arregla calculando el
GOP con los fps de verdad, o corrigiendo la declaración. Queda para H3.

## 6. Lo que falta

**Sembrar en el bucket.** El entorno está apagado, así que los archivos existen
y están verificados en local pero no subidos. El procedimiento es el de
`cmd/seed-media`, que ya declara estos mismos tres perfiles; se ejecuta en el
encendido de **A7**.

---

Este archivo no contiene credenciales, llaves ni secretos.
