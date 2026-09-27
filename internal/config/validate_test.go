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
		Environment:    config.EnvironmentProduction,
		DatabaseURL:    "postgres://moocuser:UnaContrasenaDeVerdad@10.0.0.3:5432/moocdb?sslmode=require",
		StorageBackend: "gcs",
		S3Bucket:       "plataforma-mooc-media",
		AppBaseURL:     "https://mooc.example.com",
		SMTPHost:       "smtp.sendgrid.net",
		SMTPPort:       587,
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
		"los enlaces de los correos apuntan a localhost": {
			func(c *config.Config) { c.AppBaseURL = "http://localhost:8080" },
			"APP_BASE_URL",
		},
		"no hay origen público configurado": {
			func(c *config.Config) { c.AppBaseURL = "" },
			"APP_BASE_URL",
		},
		"el correo sigue saliendo por mailpit": {
			func(c *config.Config) { c.SMTPHost = "mailpit" },
			"SMTP_HOST",
		},
		"el SMTP usa el puerto que Google bloquea": {
			func(c *config.Config) { c.SMTPPort = 25 },
			"SMTP_PORT",
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
	cfg.S3SecretKey = "minioadmin"
	cfg.AppBaseURL = "http://localhost:8080"
	cfg.SMTPHost = "mailpit"
	cfg.SMTPPort = 25

	err := cfg.Validate()
	if err == nil {
		t.Fatal("una configuración enteramente de desarrollo fue aceptada en producción")
	}

	for _, esperado := range []string{"DATABASE_URL", "STORAGE_BACKEND", "S3_SECRET_KEY", "APP_BASE_URL", "SMTP_HOST", "SMTP_PORT"} {
		if !strings.Contains(err.Error(), esperado) {
			t.Errorf("el error omite %s: quien despliega tendría que arrancar otra vez para descubrirlo", esperado)
		}
	}
}
