package config_test

import (
	"strings"
	"testing"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
)

// produccionValida es una configuracion de nube correcta. Cada prueba parte de
// ella y estropea una sola cosa, para que el fallo señale a esa cosa y no a la
// suma de varias.
func produccionValida() *config.Config {
	return &config.Config{
		Environment:        config.EnvironmentProduction,
		DatabaseURL:        "postgres://moocuser:UnaContrasenaDeVerdad@10.0.0.3:5432/moocdb?sslmode=require",
		StorageBackend:     "gcs",
		S3Bucket:           "plataforma-mooc-media",
		AppBaseURL:         "https://mooc.example.com",
		CSRFAllowedOrigins: []string{"https://mooc.example.com"},
		TrustedProxyIP:     "172.30.0.2",
		SMTPHost:           "smtp.sendgrid.net",
		SMTPPort:           587,

		// El pool dimensionado contra el max_connections de la instancia del
		// issue #118. Ver el reparto en infra/terraform/database.tf.
		DBMaxOpenConns: 25,
		DBMaxIdleConns: 25,
	}
}

func TestValidateAceptaUnaConfiguracionDeNubeCorrecta(t *testing.T) {
	if err := produccionValida().Validate(); err != nil {
		t.Fatalf("una configuración de nube correcta fue rechazada: %v", err)
	}
}

// En desarrollo no se comprueba nada: quien levanta Docker Compose no debería
// tener que configurar nada para empezar.
func TestValidateNoEstorbaEnDesarrollo(t *testing.T) {
	cfg := &config.Config{
		Environment:    "development",
		DatabaseURL:    "postgres://moocuser:moocpassword@postgres:5432/moocdb?sslmode=disable",
		StorageBackend: "minio",
		S3SecretKey:    "minioadmin",
		AppBaseURL:     "http://localhost:8080",
		SMTPHost:       "mailpit",
		SMTPPort:       1025,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("la configuración local fue rechazada en desarrollo: %v", err)
	}
}

func TestValidateRechazaValoresDeDesarrolloEnProduccion(t *testing.T) {
	casos := map[string]struct {
		estropear func(*config.Config)
		esperado  string
	}{
		"la contraseña de la base sigue siendo la local": {
			func(c *config.Config) {
				c.DatabaseURL = "postgres://moocuser:moocpassword@postgres:5432/moocdb?sslmode=disable"
			},
			"DATABASE_URL",
		},
		"el almacenamiento sigue apuntando a MinIO": {
			func(c *config.Config) { c.StorageBackend = "minio" },
			"STORAGE_BACKEND",
		},
		"el bucket sigue siendo el local": {
			func(c *config.Config) { c.S3Bucket = "mooc-storage" },
			"S3_BUCKET",
		},
		"los enlaces de los correos apuntan a localhost": {
			func(c *config.Config) { c.AppBaseURL = "http://localhost:8080" },
			"APP_BASE_URL",
		},
		"la URL publica no usa HTTPS": {
			func(c *config.Config) { c.AppBaseURL = "http://mooc.example.com" },
			"APP_BASE_URL",
		},
		"no hay origen público configurado": {
			func(c *config.Config) { c.AppBaseURL = "" },
			"APP_BASE_URL",
		},
		"la lista CSRF esta vacia": {
			func(c *config.Config) { c.CSRFAllowedOrigins = nil },
			"CSRF_ALLOWED_ORIGINS",
		},
		"la lista CSRF no incluye el origen publico": {
			func(c *config.Config) { c.CSRFAllowedOrigins = []string{"https://otro.example.com"} },
			"CSRF_ALLOWED_ORIGINS",
		},
		"la lista CSRF contiene una ruta y no un origen": {
			func(c *config.Config) { c.CSRFAllowedOrigins = []string{"https://mooc.example.com/app"} },
			"CSRF_ALLOWED_ORIGINS",
		},
		"el correo sigue saliendo por mailpit": {
			func(c *config.Config) { c.SMTPHost = "mailpit" },
			"SMTP_HOST",
		},
		"el SMTP usa el puerto que Google bloquea": {
			func(c *config.Config) { c.SMTPPort = 25 },
			"SMTP_PORT",
		},
		"la conexion a la base no exige cifrado": {
			func(c *config.Config) {
				c.DatabaseURL = "postgres://moocuser:UnaContrasena@10.0.0.3:5432/moocdb?sslmode=disable"
			},
			"sslmode",
		},
		"la conexion a la base deja el cifrado a criterio del servidor": {
			func(c *config.Config) {
				c.DatabaseURL = "postgres://moocuser:UnaContrasena@10.0.0.3:5432/moocdb?sslmode=prefer"
			},
			"sslmode",
		},
		"la cadena de conexion no menciona el cifrado": {
			func(c *config.Config) {
				c.DatabaseURL = "postgres://moocuser:UnaContrasena@10.0.0.3:5432/moocdb"
			},
			"sslmode",
		},
		"el pool no tiene limite frente a una instancia que si lo tiene": {
			func(c *config.Config) { c.DBMaxOpenConns = 0; c.DBMaxIdleConns = 0 },
			"DB_MAX_OPEN_CONNS",
		},
	}

	for nombre, caso := range casos {
		t.Run(nombre, func(t *testing.T) {
			cfg := produccionValida()
			caso.estropear(cfg)

			err := cfg.Validate()

			if err == nil {
				t.Fatalf("la configuración se aceptó pese a que %s", nombre)
			}
			if !strings.Contains(err.Error(), caso.esperado) {
				t.Errorf("el error no menciona %s, así que quien despliega no sabrá qué corregir:\n%v",
					caso.esperado, err)
			}
		})
	}
}

// La llave de MinIO solo importa si de verdad se usa MinIO. Con gcs las
// credenciales vienen de la cuenta adjunta a la VM y ese campo se ignora.
func TestValidateIgnoraLaLlaveDeMinIOCuandoElBackendEsGCS(t *testing.T) {
	cfg := produccionValida()
	cfg.S3SecretKey = "minioadmin"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("se rechazó por una llave de MinIO que con gcs no se usa: %v", err)
	}
}

// Quien despliega prefiere una lista de cuatro cosas que corregir a cuatro
// intentos de arranque.
func TestValidateReportaTodosLosProblemasJuntos(t *testing.T) {
	cfg := produccionValida()
	cfg.DatabaseURL = "postgres://moocuser:moocpassword@postgres:5432/moocdb"
	cfg.StorageBackend = "minio"
	cfg.S3Bucket = "mooc-storage"
	cfg.S3SecretKey = "minioadmin"
	cfg.AppBaseURL = "http://localhost:8080"
	cfg.CSRFAllowedOrigins = nil
	cfg.SMTPHost = "mailpit"
	cfg.SMTPPort = 25

	err := cfg.Validate()
	if err == nil {
		t.Fatal("una configuración enteramente de desarrollo fue aceptada en producción")
	}

	for _, esperado := range []string{"DATABASE_URL", "STORAGE_BACKEND", "S3_BUCKET", "S3_SECRET_KEY", "APP_BASE_URL", "CSRF_ALLOWED_ORIGINS", "SMTP_HOST", "SMTP_PORT"} {
		if !strings.Contains(err.Error(), esperado) {
			t.Errorf("el error omite %s: quien despliega tendría que arrancar otra vez para descubrirlo", esperado)
		}
	}
}

// Issue #118 (C1). Los tres modos que cifran de verdad valen; se prueban los
// tres porque verify-ca y verify-full son el endurecimiento natural de este
// despliegue y seria absurdo que el arranque los rechazara.
func TestValidateAceptaLosModosDeSSLQueCifran(t *testing.T) {
	for _, modo := range []string{"require", "verify-ca", "verify-full"} {
		t.Run(modo, func(t *testing.T) {
			cfg := produccionValida()
			cfg.DatabaseURL = "postgres://moocuser:UnaContrasena@10.0.0.3:5432/moocdb?sslmode=" + modo

			if err := cfg.Validate(); err != nil {
				t.Fatalf("sslmode=%s fue rechazado pese a que cifra: %v", modo, err)
			}
		})
	}
}

// libpq admite dos formas de cadena de conexion y la comprobacion tiene que
// ver ambas: si solo entendiera la URL, pasar a la forma de pares
// clave=valor desactivaria la comprobacion sin que nada lo avisara.
func TestValidateLeeElSslmodeEnLaFormaDePares(t *testing.T) {
	cfg := produccionValida()
	cfg.DatabaseURL = "host=10.0.0.3 port=5432 user=moocuser password=UnaContrasena dbname=moocdb sslmode=disable"

	err := cfg.Validate()

	if err == nil {
		t.Fatal("se acepto una cadena de pares clave=valor con sslmode=disable")
	}
	if !strings.Contains(err.Error(), "sslmode") {
		t.Errorf("el error no menciona sslmode:\n%v", err)
	}
}

// Una contrasena con `&` o con `?` dentro no debe confundir la extraccion del
// sslmode. Es el caso por el que la comprobacion busca el parametro en lugar de
// parsear la cadena: un parseo estricto fallaria entero por la contrasena.
func TestValidateNoSeConfundeConUnaContrasenaConSimbolos(t *testing.T) {
	cfg := produccionValida()
	cfg.DatabaseURL = "postgres://moocuser:a?b&c=d@10.0.0.3:5432/moocdb?sslmode=require"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("una contrasena con simbolos rompio la lectura del sslmode: %v", err)
	}
}

// Pedir mas conexiones inactivas que abiertas no da error en database/sql: las
// recorta en silencio. Es la clase de configuracion que parece aplicada y no lo
// esta, asi que se comprueba en todo entorno y no solo en produccion.
func TestValidateRechazaMasConexionesInactivasQueAbiertas(t *testing.T) {
	cfg := &config.Config{
		Environment:    "development",
		DBMaxOpenConns: 10,
		DBMaxIdleConns: 25,
	}

	err := cfg.Validate()

	if err == nil {
		t.Fatal("se acepto un pool con mas conexiones inactivas que abiertas")
	}
	if !strings.Contains(err.Error(), "DB_MAX_IDLE_CONNS") {
		t.Errorf("el error no menciona DB_MAX_IDLE_CONNS:\n%v", err)
	}
}

func TestValidateRechazaWorkerConcurrencyNegativa(t *testing.T) {
	cfg := produccionValida()
	cfg.WorkerConcurrency = -1

	err := cfg.Validate()
	if err == nil {
		t.Fatal("se acepto una concurrencia de workers negativa")
	}
	if !strings.Contains(err.Error(), "WORKER_CONCURRENCY") {
		t.Errorf("el error no menciona WORKER_CONCURRENCY:\n%v", err)
	}
}
