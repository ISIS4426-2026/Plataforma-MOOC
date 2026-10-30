// Command queue-exporter publica en Cloud Monitoring las metricas de la cola
// compartida, desde un punto que NO pertenece a los grupos elasticos
// (C6, issue #209).
//
// # POR QUE EXISTE, SI EL WORKER YA PUBLICA ESAS METRICAS
//
// `internal/worker/queue_metrics.go` ya expone `worker.queue.pending`, `active`
// y `oldest_pending_age_seconds`, pero desde dentro de cada worker. Eso vale
// para observar y **no vale para autoescalar**, por tres razones:
//
//  1. **El autoescalador SUMA las series que devuelve su consulta.** Son
//     magnitudes de la cola, no de la instancia, asi que cada worker publica el
//     mismo valor global: con tres instancias el autoescalador veria el triple
//     de profundidad de la que hay y escalaria por su propio reflejo.
//  2. **Hay ventanas sin exportador** durante una rotacion o una recreacion,
//     justo cuando la situacion esta cambiando.
//  3. El enunciado lo prohibe: la cola «no debe ... depender su disponibilidad
//     de una maquina que el autoscaling pueda retirar», y la senal que gobierna
//     el autoscaling es parte de esa disponibilidad.
//
// POR QUE ESCRIBE A CLOUD MONITORING EN VEZ DE EXPONER /metrics
//
// Un exportador de Prometheus raspado por el agente de operaciones publicaria
// las series bajo el recurso `gce_instance` **de la VM de la cola**. Y el
// autoescalador de GCE, con el tipo de recurso `gce_instance`, exige una serie
// por cada instancia **del grupo** y **no admite filtrar por etiquetas de
// recurso**: nunca encontraria esta.
//
// Por eso se escriben directamente como `custom.googleapis.com/...` sobre el
// recurso `generic_node`, que si es filtrable. Es tambien la razon por la que
// B2 le dio `monitoring.metricWriter` a la cuenta de la cola.
//
// CONFIGURACION, toda por entorno
//
//	GOOGLE_CLOUD_PROJECT  proyecto donde se escriben las series (obligatoria)
//	REDIS_URL             la cola; por omision localhost:6379
//	QUEUE_NAMES           colas a observar, separadas por comas; por omision "default"
//	EXPORT_INTERVAL       cada cuanto se escribe; por omision 30s
//	NODE_ID               identifica al exportador en el recurso; por omision el hostname
//	GOOGLE_CLOUD_LOCATION ubicacion del recurso `generic_node`; por omision us-east1-b
//	METRIC_NAMESPACE      espacio de nombres por el que filtra el autoescalador;
//	                      por omision mooc-cola
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/hibiken/asynq"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/worker"
)

// Los cuatro tipos de metrica. `custom.googleapis.com/` es el prefijo que Cloud
// Monitoring reserva para metricas definidas por el usuario.
//
// Tres describen la cola y una gobierna el autoescalado. No es redundancia:
// `outstanding` = `pending` + `active`, y esa suma tiene que calcularse **aqui**
// porque el autoescalador de GCE solo sabe sumar series del **mismo** tipo de
// metrica. Dos tipos distintos no se pueden sumar en su consulta.
//
// POR QUE EL AUTOESCALADOR MIRA `outstanding` Y NO `pending`
//
// Con `pending` y dos trabajos por consumidor, una sola instancia oscila:
//
//	pending=4 -> pide 2 instancias -> la nueva toma 2 -> pending=2 -> pide 1
//	-> retira -> los trabajos vuelven a la cola -> pending=4 -> pide 2 ...
//
// El trabajo en curso desaparece de `pending` en el momento en que alguien lo
// empieza, asi que `pending` mide **lo que nadie ha tocado** y no la carga. Con
// `outstanding` el total no baja porque alguien se ponga a trabajar, solo porque
// el trabajo termine, y entonces la serie decrece de forma monotona durante un
// vaciado: exactamente lo que un autoescalador necesita para no perseguirse.
const (
	metricaPendientes = "custom.googleapis.com/mooc/queue/pending"
	metricaActivos    = "custom.googleapis.com/mooc/queue/active"
	metricaAntiguedad = "custom.googleapis.com/mooc/queue/oldest_pending_age_seconds"
	metricaPorAtender = "custom.googleapis.com/mooc/queue/outstanding"
)

func env(clave, pordefecto string) string {
	if v := strings.TrimSpace(os.Getenv(clave)); v != "" {
		return v
	}
	return pordefecto
}

func main() {
	proyecto := env("GOOGLE_CLOUD_PROJECT", "")
	if proyecto == "" {
		log.Fatal("falta GOOGLE_CLOUD_PROJECT: sin proyecto no hay donde escribir las series")
	}

	redisURL := env("REDIS_URL", "localhost:6379")
	intervalo, err := time.ParseDuration(env("EXPORT_INTERVAL", "30s"))
	if err != nil || intervalo < 10*time.Second {
		// Menos de diez segundos no mejora la decision del autoescalador --su
		// propio periodo de muestreo es mayor-- y multiplica el coste de
		// escritura de Cloud Monitoring, que se factura por punto.
		log.Fatalf("EXPORT_INTERVAL invalida o menor de 10s: %v", err)
	}

	hostname, _ := os.Hostname()
	nodo := env("NODE_ID", hostname)
	if nodo == "" {
		nodo = "cola"
	}

	colas := strings.Split(env("QUEUE_NAMES", "default"), ",")
	for i := range colas {
		colas[i] = strings.TrimSpace(colas[i])
	}

	redisOpt, err := worker.ParseRedisOpt(redisURL)
	if err != nil {
		log.Fatalf("REDIS_URL invalida: %v", err)
	}
	inspector := asynq.NewInspector(redisOpt)
	defer func() { _ = inspector.Close() }()

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	cliente, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		log.Fatalf("no se pudo abrir el cliente de Cloud Monitoring: %v", err)
	}
	defer func() { _ = cliente.Close() }()

	log.Printf("exportador de cola en marcha: proyecto=%s cola=%s colas=%s cada %s",
		proyecto, redisURL, strings.Join(colas, ","), intervalo)

	ticker := time.NewTicker(intervalo)
	defer ticker.Stop()

	// Una escritura inmediata para no dejar al autoescalador sin dato durante
	// el primer intervalo tras un arranque.
	exportar(ctx, cliente, inspector, proyecto, nodo, colas)

	for {
		select {
		case <-ctx.Done():
			log.Print("senal recibida: parando")
			return
		case <-ticker.C:
			exportar(ctx, cliente, inspector, proyecto, nodo, colas)
		}
	}
}

func exportar(ctx context.Context, cliente *monitoring.MetricClient, inspector *asynq.Inspector,
	proyecto, nodo string, colas []string) {

	ahora := timestamppb.New(time.Now())
	series := make([]*monitoringpb.TimeSeries, 0, len(colas)*3)

	for _, q := range colas {
		var pendientes, activos int64
		var antiguedad float64

		// Una cola que nunca recibio una tarea todavia no existe en Redis, y el
		// Inspector lo reporta como error en vez de como cola vacia. Es el
		// estado esperado justo despues de un despliegue, no un fallo: publicar
		// ceros es exactamente lo que esa cola vale.
		//
		// Importa para el autoescalador: sin punto no hay dato, y sin dato no
		// decide. Con un cero decide que no hace falta escalar, que es correcto.
		if info, err := inspector.GetQueueInfo(q); err == nil {
			pendientes = int64(info.Pending)
			activos = int64(info.Active)
			antiguedad = info.Latency.Seconds()
		}

		recurso := recursoDe(proyecto, nodo, q)
		series = append(series,
			serieEntera(metricaPendientes, recurso, ahora, pendientes),
			serieEntera(metricaActivos, recurso, ahora, activos),
			serieDoble(metricaAntiguedad, recurso, ahora, antiguedad),
			serieEntera(metricaPorAtender, recurso, ahora, pendientes+activos),
		)
	}

	peticion := &monitoringpb.CreateTimeSeriesRequest{
		Name:       "projects/" + proyecto,
		TimeSeries: series,
	}

	escritura, cancelar := context.WithTimeout(ctx, 20*time.Second)
	defer cancelar()

	if err := cliente.CreateTimeSeries(escritura, peticion); err != nil {
		// No es fatal: un fallo de escritura no debe tumbar el exportador, o la
		// siguiente ventana tampoco tendria dato. Se registra para que el fallo
		// sea visible en los registros de la instancia.
		if !errors.Is(err, context.Canceled) {
			log.Printf("no se pudieron escribir las series: %v", err)
		}
	}
}

// recursoDe describe DONDE se mide, y es lo que el autoescalador filtra.
//
// `generic_node` y no `gce_instance`: con `gce_instance` el autoescalador exige
// una serie por instancia del grupo y no deja filtrar por etiquetas de recurso,
// asi que nunca veria una serie publicada desde la VM de la cola.
//
// El espacio de nombres viene del entorno, y no es pereza:
//
// Es la etiqueta por la que filtra el autoescalador. `worker_autoscaler.tf`
// escribe el filtro y `queue.tf` escribe esta variable, los dos desde el mismo
// `local.queue_metric_namespace`: asi no pueden divergir.
//
// Si estuviera fija en este fichero, cambiarla en un sitio y no en el otro
// dejaria al autoescalador mirando una consulta que no devuelve nada -- y una
// consulta vacia no es un error visible, es un grupo que deja de escalar en
// silencio. Los NOMBRES de las metricas si son constantes de aqui: nombran un
// esquema, no un despliegue.
func recursoDe(proyecto, nodo, cola string) *monitoredrespb.MonitoredResource {
	return &monitoredrespb.MonitoredResource{
		Type: "generic_node",
		Labels: map[string]string{
			"project_id": proyecto,
			"location":   env("GOOGLE_CLOUD_LOCATION", "us-east1-b"),
			"namespace":  env("METRIC_NAMESPACE", "mooc-cola"),
			"node_id":    nodo + "/" + cola,
		},
	}
}

func serieEntera(tipo string, recurso *monitoredrespb.MonitoredResource,
	cuando *timestamppb.Timestamp, valor int64) *monitoringpb.TimeSeries {

	return &monitoringpb.TimeSeries{
		Metric:   &metricpb.Metric{Type: tipo},
		Resource: recurso,
		Points: []*monitoringpb.Point{{
			Interval: &monitoringpb.TimeInterval{EndTime: cuando},
			Value: &monitoringpb.TypedValue{
				Value: &monitoringpb.TypedValue_Int64Value{Int64Value: valor},
			},
		}},
	}
}

func serieDoble(tipo string, recurso *monitoredrespb.MonitoredResource,
	cuando *timestamppb.Timestamp, valor float64) *monitoringpb.TimeSeries {

	return &monitoringpb.TimeSeries{
		Metric:   &metricpb.Metric{Type: tipo},
		Resource: recurso,
		Points: []*monitoringpb.Point{{
			Interval: &monitoringpb.TimeInterval{EndTime: cuando},
			Value: &monitoringpb.TypedValue{
				Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: valor},
			},
		}},
	}
}
