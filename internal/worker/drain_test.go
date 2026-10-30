package worker

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker/task"
	"github.com/hibiken/asynq"
)

// El plazo configurado manda sobre el de asynq, y un valor no positivo no lo
// toca. Importa la segunda mitad: un cero accidental --una variable de entorno
// vacia-- no puede convertir el drenaje en «aborta en el acto».
func TestWithShutdownTimeout(t *testing.T) {
	cfg := &config.Config{Environment: "test", RedisURL: "localhost:6379"}

	casos := []struct {
		nombre   string
		opcion   []WorkerOption
		esperado time.Duration
	}{
		{"sin configurar manda el de asynq", nil, 8 * time.Second},
		{"configurado manda el nuestro", []WorkerOption{WithShutdownTimeout(90 * time.Second)}, 90 * time.Second},
		{"cero se ignora", []WorkerOption{WithShutdownTimeout(0)}, 8 * time.Second},
		{"negativo se ignora", []WorkerOption{WithShutdownTimeout(-5 * time.Second)}, 8 * time.Second},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			we, err := NewWorkerEngine(cfg, c.opcion...)
			if err != nil {
				t.Fatalf("no se pudo crear el motor: %v", err)
			}
			if got := we.drainTimeout(); got != c.esperado {
				t.Errorf("drainTimeout() = %s, se esperaba %s", got, c.esperado)
			}
		})
	}
}

// El comportamiento que C7 tiene que garantizar: un worker que se retira con un
// trabajo en curso **devuelve** ese trabajo a la cola y **no gasta un
// reintento**.
//
// Que no gaste reintento no es un detalle. Una instancia retirada por el
// autoescalador no ha fallado en nada, y si cada retirada consumiera uno de los
// tres intentos, tres reducciones seguidas mandarian a la cola de muertos un
// trabajo perfectamente sano.
func TestWorkerEngine_DrenajeDevuelveLaTareaSinGastarReintento(t *testing.T) {
	const cola = "c7-drenaje"
	cfg := &config.Config{Environment: "test", RedisURL: redisDePrueba(t)}

	limpiarCola(t, cfg.RedisURL, cola)

	// El manejador avisa de que empezo y se queda bloqueado hasta que lo
	// cancelan: es un transcodificado de 1 711 s en miniatura.
	empezo := make(chan struct{}, 1)
	we, err := NewWorkerEngine(cfg,
		WithConcurrency(1),
		WithQueues(map[string]int{cola: 1}),
		WithShutdownTimeout(500*time.Millisecond),
		WithMediaProcessor(asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
			select {
			case empezo <- struct{}{}:
			default:
			}
			<-ctx.Done()
			return ctx.Err()
		})),
	)
	if err != nil {
		t.Fatalf("no se pudo crear el motor: %v", err)
	}

	ctx, cancelar := context.WithCancel(context.Background())
	fin := make(chan error, 1)
	go func() { fin <- we.Start(ctx) }()

	cliente, err := NewClient(cfg.RedisURL)
	if err != nil {
		t.Fatalf("no se pudo crear el cliente: %v", err)
	}
	defer cliente.Close()

	tarea, err := task.NewMediaProcessTask("c7-recurso", "hls_transcode", "originals/c7.mp4")
	if err != nil {
		t.Fatalf("no se pudo construir la tarea: %v", err)
	}
	if _, err := cliente.Enqueue(context.Background(), tarea, asynq.Queue(cola)); err != nil {
		t.Fatalf("no se pudo encolar: %v", err)
	}

	select {
	case <-empezo:
	case <-time.After(10 * time.Second):
		cancelar()
		t.Fatal("el worker nunca empezo la tarea")
	}

	cancelar()
	select {
	case err := <-fin:
		if err != nil {
			t.Fatalf("el drenaje devolvio error: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("el drenaje no termino")
	}

	inspector, err := cliente.Inspector()
	if err != nil {
		t.Fatalf("no se pudo crear el inspector: %v", err)
	}
	defer inspector.Close()

	pendientes, err := inspector.ListPendingTasks(cola)
	if err != nil {
		t.Fatalf("no se pudieron listar las pendientes: %v", err)
	}
	if len(pendientes) != 1 {
		t.Fatalf("la cola %s tiene %d pendientes, se esperaba 1: el drenaje perdio la tarea", cola, len(pendientes))
	}
	if n := pendientes[0].Retried; n != 0 {
		t.Errorf("la tarea devuelta lleva %d reintentos, se esperaba 0: un drenaje no es un fallo", n)
	}

	limpiarCola(t, cfg.RedisURL, cola)
}

// redisDePrueba salta la prueba si no hay un Redis escuchando, en vez de
// fallar. La integracion con Redis la cubre CI, que si lo levanta; en una
// maquina de desarrollo sin el, un fallo aqui solo anade ruido a una suite que
// ya tiene dos pruebas en esa situacion.
func redisDePrueba(t *testing.T) string {
	t.Helper()

	url := "localhost:6379"
	conn, err := net.DialTimeout("tcp", url, time.Second)
	if err != nil {
		t.Skipf("no hay Redis en %s: %v", url, err)
	}
	conn.Close()
	return url
}

// limpiarCola deja la cola vacia para que la prueba no dependa de lo que
// dejara una corrida anterior.
func limpiarCola(t *testing.T, redisURL, cola string) {
	t.Helper()

	cliente, err := NewClient(redisURL)
	if err != nil {
		t.Fatalf("no se pudo crear el cliente de limpieza: %v", err)
	}
	defer cliente.Close()

	inspector, err := cliente.Inspector()
	if err != nil {
		t.Fatalf("no se pudo crear el inspector de limpieza: %v", err)
	}
	defer inspector.Close()

	// Una cola que todavia no existe no es un error aqui: es el caso normal la
	// primera vez que esta prueba corre contra un Redis limpio.
	//
	// Las activas no se pueden borrar --asynq no lo permite, y es correcto: una
	// tarea activa la tiene un worker en la mano--. Si quedara alguna de una
	// corrida anterior, su lease de 30 s vence y la recupera el recoverer.
	if _, err := inspector.DeleteAllPendingTasks(cola); err != nil {
		t.Logf("no se pudo vaciar %s (puede que aun no exista): %v", cola, err)
	}
}
