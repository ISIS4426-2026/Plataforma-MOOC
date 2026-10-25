package handler

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

// Liveness y readiness contestan preguntas distintas, y mezclarlas tiene
// consecuencias caras en un grupo administrado.
//
// # Por que estan separadas
//
// El grupo de instancias repara --es decir, destruye y recrea-- la maquina que
// no supera su comprobacion de salud. El balanceador, en cambio, solo deja de
// enviarle trafico.
//
// Si la comprobacion que usa el grupo dependiera de la base de datos, un
// parpadeo de Cloud SQL haria fallar a las tres replicas a la vez y el grupo
// las recrearia todas. Un problema transitorio de una dependencia compartida se
// convertiria en una caida completa, y ademas la capacidad nueva arrancaria
// contra la misma base que sigue sin responder.
//
// De ahi el reparto:
//
//   - /api/v1/live    el proceso esta vivo y sirve. Sin dependencias. Es lo que
//     mira el autohealing del grupo.
//   - /api/v1/ready   esta replica puede atender una peticion de verdad. Prueba
//     la base. Es lo que mira el balanceador.
//
// # Por que el cuerpo no dice nada
//
// El criterio de aceptacion pide que la respuesta no revele dependencias,
// versiones ni topologia. Quien consulta estas dos rutas es una sonda
// automatica que solo mira el codigo de estado, asi que el cuerpo se reduce al
// minimo. El detalle --que dependencia fallo-- vive en /api/v1/health, que es
// la vista interna, y en el log.

// readinessTimeout acota la prueba de la dependencia.
//
// Mas corto que healthCheckTimeout a proposito: el balanceador reintenta cada
// pocos segundos, y una sonda que tarda mas que su propio intervalo apila
// peticiones en una instancia que ya esta en apuros.
const readinessTimeout = 1500 * time.Millisecond

// Readiness describe si la instancia puede atender.
type Readiness struct {
	Status string `json:"status"`
}

// NewLivenessHandler construye /api/v1/live.
//
// Responde 200 mientras el proceso sirva. No prueba nada: si este manejador
// llega a ejecutarse, el proceso esta vivo, acepta conexiones y su mux
// funciona. Es exactamente lo que el autohealing necesita saber.
func NewLivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		RespondWithJSON(w, http.StatusOK, Readiness{Status: "pass"})
	}
}

// NewReadinessHandler construye /api/v1/ready.
//
// ready es la bandera que el servidor levanta cuando termina de montarse. Antes
// de eso la respuesta es 503 aunque la base conteste: durante esa ventana el
// proceso vive pero todavia no puede atender, y mandarle trafico del
// balanceador seria repartir errores.
func NewReadinessHandler(db Pinger, ready *atomic.Bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ready != nil && !ready.Load() {
			RespondWithJSON(w, http.StatusServiceUnavailable, Readiness{Status: "fail"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			RespondWithJSON(w, http.StatusServiceUnavailable, Readiness{Status: "fail"})
			return
		}

		RespondWithJSON(w, http.StatusOK, Readiness{Status: "pass"})
	}
}
