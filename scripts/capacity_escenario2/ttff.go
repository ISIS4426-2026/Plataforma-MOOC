package main

// G4 (#224): lo que la campana de la Entrega 2 no tenia y la revision de A7 pidio.
//
//   - el tiempo al primer cuadro, medido por un reproductor (Chromium sin ventana + hls.js), no sumado;
//   - un video YA PROCESADO para el consumo HLS, de modo que los estudiantes no dependan de la cola
//     (en A7, los niveles 2 a 4 no descargaron ni un segmento porque sus videos seguian encolados);
//   - un detector de atasco que mira el SISTEMA y no un recurso: en una cola saturada un trabajo puede
//     esperar su turno una hora sin cambiar de estado, y eso no es estar colgado.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os/exec"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// ---- atasco del sistema --------------------------------------------------------------------------------

var ultimoAvanceSistema atomic.Int64

func marcarAvance() { ultimoAvanceSistema.Store(time.Now().UnixNano()) }

func sinAvanceDesde() time.Duration {
	v := ultimoAvanceSistema.Load()
	if v == 0 {
		return 0
	}
	return time.Since(time.Unix(0, v))
}

// limiteAtascoSistema: tiempo sin que NINGUN trabajo cambie de estado. Un video largo transcodifica en
// ~30 min mientras los demas esperan, asi que el plazo tiene que ser mayor que eso.
const limiteAtascoSistema = 45 * time.Minute

// ---- tiempo al primer cuadro ----------------------------------------------------------------------------

type MuestraTTFF struct {
	TTFFMs           float64 `json:"ttff_ms"`
	ManifiestoMs     float64 `json:"manifiesto_ms"`
	VarianteMs       float64 `json:"variante_ms"`
	Segmento0Ms      float64 `json:"segmento0_ms"`
	DecodificacionMs float64 `json:"decodificacion_ms"`
	Playing          float64 `json:"playing"`
	Waiting          float64 `json:"waiting"`
	Stalled          float64 `json:"stalled"`
	Error            string  `json:"error"`
	Navegador        string  `json:"navegador"`
}

type ResultadoTTFF struct {
	Solicitadas int
	Muestras    []MuestraTTFF
	Fallidas    int
	Navegador   string
}

// medirTTFF lanza el contenedor del medidor n veces, una tras otra (cada medicion es una sesion nueva de
// navegador, sin cache). Devuelve solo lo observado: una muestra que fallo cuenta como fallida, no como cero.
func medirTTFF(imagen, masterURL string, n int) ResultadoTTFF {
	r := ResultadoTTFF{Solicitadas: n}
	for i := 0; i < n; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		cmd := exec.CommandContext(ctx, "docker", "run", "--rm", imagen, masterURL)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		cancel()
		linea := strings.TrimSpace(out.String())
		if idx := strings.LastIndex(linea, "\n"); idx >= 0 {
			linea = linea[idx+1:]
		}
		var m MuestraTTFF
		if err != nil || json.Unmarshal([]byte(linea), &m) != nil || m.Error != "" || m.TTFFMs <= 0 {
			r.Fallidas++
			motivo := m.Error
			if motivo == "" && err != nil {
				motivo = err.Error()
			}
			log.Printf("    TTFF muestra %d/%d fallida: %s", i+1, n, motivo)
			continue
		}
		if r.Navegador == "" {
			r.Navegador = m.Navegador
		}
		r.Muestras = append(r.Muestras, m)
	}
	return r
}

func valoresTTFF(ms []MuestraTTFF, f func(MuestraTTFF) float64) []float64 {
	v := make([]float64, 0, len(ms))
	for _, m := range ms {
		v = append(v, f(m))
	}
	sort.Float64s(v)
	return v
}

// percentilFloat usa el mismo estimador que el resto de la campana (el de capacity_criterios.py): el
// rango mas cercano. Con pocas muestras un percentil alto es el maximo, y por eso se publica n/d.
func percentilFloat(ordenados []float64, p int) float64 {
	if len(ordenados) == 0 {
		return math.NaN()
	}
	k := int(math.Ceil(float64(p)/100*float64(len(ordenados)))) - 1
	if k < 0 {
		k = 0
	}
	if k >= len(ordenados) {
		k = len(ordenados) - 1
	}
	return ordenados[k]
}

// textoPercentilTTFF aplica las reglas de A6: un p95 necesita 20 observaciones y un p99, 100.
func textoPercentilTTFF(v []float64, p int) string {
	minimo := map[int]int{50: 5, 95: 20, 99: 100}[p]
	if len(v) < minimo {
		return fmt.Sprintf("n/d (n=%d)", len(v))
	}
	return fmt.Sprintf("%.0f ms", percentilFloat(v, p))
}

func (r ResultadoTTFF) Texto() string {
	if len(r.Muestras) == 0 {
		return fmt.Sprintf("sin muestras (%d solicitadas, %d fallidas)", r.Solicitadas, r.Fallidas)
	}
	t := valoresTTFF(r.Muestras, func(m MuestraTTFF) float64 { return m.TTFFMs })
	man := valoresTTFF(r.Muestras, func(m MuestraTTFF) float64 { return m.ManifiestoMs })
	seg := valoresTTFF(r.Muestras, func(m MuestraTTFF) float64 { return m.Segmento0Ms })
	dec := valoresTTFF(r.Muestras, func(m MuestraTTFF) float64 { return m.DecodificacionMs })
	return fmt.Sprintf("n=%d (fallidas %d) | primer cuadro: mediana %s, min %.0f, max %.0f ms | manifiesto %.0f, segmento 0 %.0f, decodificacion %.0f ms (medianas)",
		len(t), r.Fallidas, textoPercentilTTFF(t, 50), t[0], t[len(t)-1],
		percentilFloat(man, 50), percentilFloat(seg, 50), percentilFloat(dec, 50))
}
