package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// Liveness y readiness contestan preguntas distintas tambien en el worker, y
// mezclarlas tiene el mismo coste que en la API (ver A2, internal/http/handler/
// readiness.go):
//
//   - /healthz   el proceso vive. No prueba dependencias, asi que una cola caida
//                no hace que el worker se declare muerto: seguiria vivo y
//                volveria a consumir en cuanto la cola vuelva.
//   - /readyz    este worker **puede trabajar de verdad**: alcanza la cola
//                compartida. Es lo que pide C5 como criterio de salud, «proceso
//                vivo y conectado a la cola compartida».
//
// POR QUE EL AUTOHEALING MIRA /readyz Y NO /healthz
//
// Un worker que vive pero no alcanza la cola no consume nada: esta ocupando una
// instancia sin hacer trabajo, y eso es justo lo que el grupo tiene que reparar.
// Con /healthz esa instancia pasaria la comprobacion para siempre.
//
// La contrapartida, que hay que saber: si la cola cae, **todas** las instancias
// del grupo se declaran no sanas a la vez y el grupo intentara recrearlas todas.
// No las arregla --el problema esta fuera-- pero tampoco empeora el servicio,
// porque sin cola no hay trabajo que hacer. La cola es un punto unico de fallo
// declarado desde B1 (nota tecnica 30), y esto es una consecuencia mas de eso.

// readinessTimeout acota la prueba de la cola. Mas largo que esto y la
// comprobacion de salud expira antes (timeout_sec = 5 en el health check), con
// lo que el resultado seria el mismo pero sin diagnostico.
const readinessTimeout = 2 * time.Second

// NewLivenessHandler responde 200 mientras el proceso atienda. No prueba nada
// mas, y es deliberado.
func NewLivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escribir(w, http.StatusOK, map[string]string{"status": "pass"})
	})
}

// NewReadinessHandler construye /readyz sobre la misma conexion que usa el
// worker para consumir, no sobre una nueva: comprobar una conexion distinta de
// la que trabaja puede dar sano mientras el consumo esta roto.
func NewReadinessHandler(opt asynq.RedisConnOpt) http.Handler {
	cliente, _ := opt.MakeRedisClient().(redis.UniversalClient)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cliente == nil {
			escribir(w, http.StatusServiceUnavailable, map[string]string{
				"status": "fail",
				"queue":  "sin cliente de cola",
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()

		if err := cliente.Ping(ctx).Err(); err != nil {
			// El error no se devuelve al cliente: la respuesta de una sonda la
			// lee un balanceador o un autohealer, y el detalle va al registro.
			escribir(w, http.StatusServiceUnavailable, map[string]string{
				"status": "fail",
				"queue":  "down",
			})
			return
		}

		escribir(w, http.StatusOK, map[string]string{
			"status": "pass",
			"queue":  "up",
		})
	})
}

func escribir(w http.ResponseWriter, codigo int, cuerpo map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(codigo)
	_ = json.NewEncoder(w).Encode(cuerpo)
}
