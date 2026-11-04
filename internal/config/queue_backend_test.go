package config_test

import (
	"strings"
	"testing"
)

func TestValidateAceptaLosDosBackendsDeCola(t *testing.T) {
	for _, backend := range []string{"", "asynq"} {
		cfg := produccionValida()
		cfg.QueueBackend = backend
		if err := cfg.Validate(); err != nil {
			t.Errorf("QUEUE_BACKEND=%q fue rechazado: %v", backend, err)
		}
	}

	cfg := produccionValida()
	cfg.QueueBackend = "pubsub"
	cfg.PubSubTopic = "projects/demo/topics/media-jobs"
	if err := cfg.Validate(); err != nil {
		t.Errorf("pubsub con topic fue rechazado: %v", err)
	}
}

func TestValidateRechazaUnBackendDeColaDesconocido(t *testing.T) {
	cfg := produccionValida()
	cfg.QueueBackend = "kafka"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "QUEUE_BACKEND") {
		t.Fatalf("se esperaba un error de QUEUE_BACKEND, hubo: %v", err)
	}
}

// Solo la API publica: sin topic no puede entregar trabajos, pero el worker solo recibe y no lo necesita.
func TestValidatePubSubExigeTopicSoloALaAPI(t *testing.T) {
	cfg := produccionValida()
	cfg.QueueBackend = "pubsub"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PUBSUB_TOPIC") {
		t.Fatalf("la API arrancó con pubsub y sin topic: %v", err)
	}

	worker := workerEnProduccion()
	worker.QueueBackend = "pubsub"
	if err := worker.ValidateWorker(); err != nil {
		t.Errorf("el worker no necesita el topic: %v", err)
	}
}

// Entrega 5: el topic de documentos solo existe en Pub/Sub.
func TestValidateExigePubSubParaElTopicDeDocumentos(t *testing.T) {
	cfg := produccionValida()
	cfg.PubSubDocumentsTopic = "projects/demo/topics/media-documents"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PUBSUB_DOCUMENTS_TOPIC") {
		t.Fatalf("la API arranco con un topic de documentos y sin Pub/Sub: %v", err)
	}

	cfg.QueueBackend = "pubsub"
	cfg.PubSubTopic = "projects/demo/topics/media-jobs"
	if err := cfg.Validate(); err != nil {
		t.Errorf("con Pub/Sub el topic de documentos es valido: %v", err)
	}
}
