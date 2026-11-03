package push

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// NewMux arma las rutas del worker en modo Pub/Sub.
//
//	POST /pubsub/push   el mensaje; se confirma (204) solo cuando el trabajo termino y quedo guardado
//	GET  /healthz       el proceso respira
//	GET  /readyz        igual que /healthz: este worker no depende de una cola que pueda estar caida
//	GET  /metrics       las mismas metricas del worker de asynq
func NewMux(push, liveness, metrics http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("POST /pubsub/push", push)
	mux.Handle("GET /healthz", liveness)
	mux.Handle("GET /readyz", liveness)
	mux.Handle("GET /metrics", metrics)
	return mux
}

// Serve atiende en `ln` hasta que `ctx` termine y luego cierra con `grace` de plazo.
//
// Lo que siga en curso al cerrar se queda sin confirmar y Pub/Sub lo reentrega: no se espera a un trabajo
// de minutos. Devuelve el error del servidor si se detuvo por su cuenta, y nil si fue un cierre pedido.
func Serve(ctx context.Context, ln net.Listener, handler http.Handler, grace time.Duration) error {
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
