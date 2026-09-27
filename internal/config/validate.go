package config

import (
	"fmt"
	"strings"
)

// Issue #117. El criterio de aceptacion dice que «la aplicacion arranca solo
// con configuracion inyectada», y esa frase solo significa algo si arrancar sin
// ella falla.
//
// Sin estas comprobaciones, un despliegue mal configurado no se queja: arranca,
// atiende peticiones y funciona lo suficiente como para parecer correcto. La
// base seguiria con la contrasena de desarrollo, los correos de verificacion
// apuntarian a localhost y los archivos irian a un MinIO que en la nube no
// existe. Cada uno de esos fallos se descubre tarde y por un sintoma que no se
// parece a la causa.

// EnvironmentProduction es el valor de APP_ENV que activa las comprobaciones.
//
// Se eligio la lista blanca al reves --se comprueba en produccion, no se relaja
// en desarrollo-- para que el entorno por defecto sea el permisivo. Un
// desarrollador que levanta Docker Compose no deberia tener que configurar nada
// para empezar.
const EnvironmentProduction = "production"

// Valores por defecto que sirven en local y que en la nube significan que algo
// no se inyecto. Estan aqui, juntos, para que se lean como lo que son: una
// lista de cosas que no deben sobrevivir a un despliegue.
const (
	localDBPassword = "moocpassword"
	localS3Secret   = "minioadmin"
	localMailHost   = "mailpit"
)

// Validate comprueba que la configuracion no arrastra valores de desarrollo a
// un entorno que no lo es.
//
// Devuelve todos los problemas juntos y no el primero: quien despliega prefiere
// una lista de cuatro cosas que corregir a cuatro intentos de arranque.
func (c *Config) Validate() error {
	if !strings.EqualFold(c.Environment, EnvironmentProduction) {
		return nil
	}

	var problemas []string

	// La contrasena de la base viaja dentro de DATABASE_URL, asi que se busca
	// ahi. Que siga siendo la de docker-compose significa que TF_VAR_db_password
	// no llego al contenedor, o que se copio la cadena de conexion local.
	if strings.Contains(c.DatabaseURL, localDBPassword) {
		problemas = append(problemas,
			"DATABASE_URL conserva la contrasena de desarrollo: la de la nube vive en Secret Manager")
	}

	// En la nube el almacenamiento es el servicio administrado. Con minio, o
	// bien la migracion del issue C4 no se hizo, o el contenedor apunta a un
	// MinIO que no existe alli.
	if strings.EqualFold(c.StorageBackend, "minio") {
		problemas = append(problemas,
			"STORAGE_BACKEND=minio en produccion: la nube usa el almacenamiento administrado (gcs)")
	}
	if c.S3Bucket == "" || c.S3Bucket == "mooc-storage" {
		problemas = append(problemas,
			"S3_BUCKET conserva el valor de desarrollo (mooc-storage): en produccion debe ser el bucket administrado")
	}

	// Solo importa si de verdad se usa MinIO. Con gcs las credenciales vienen
	// de la cuenta de servicio adjunta a la VM y estos campos se ignoran.
	if strings.EqualFold(c.StorageBackend, "minio") && c.S3SecretKey == localS3Secret {
		problemas = append(problemas,
			"S3_SECRET_KEY conserva el valor de desarrollo")
	}

	// APP_BASE_URL arma los enlaces de activacion, de recuperacion de contrasena
	// y, desde el issue #113, el de verificacion de insignias. Si apunta a
	// localhost, nada de eso falla de forma visible: los correos se envian y los
	// enlaces sencillamente no abren.
	if c.AppBaseURL == "" {
		problemas = append(problemas, "APP_BASE_URL vacio: los enlaces de los correos no se pueden construir")
	} else if strings.Contains(c.AppBaseURL, "localhost") || strings.Contains(c.AppBaseURL, "127.0.0.1") {
		problemas = append(problemas,
			"APP_BASE_URL apunta a localhost: los enlaces de verificacion y de insignias no abririan para nadie")
	}

	// Mailpit es el buzon de desarrollo. En la nube el correo sale por un SMTP
	// real (issue #126), y ademas Google bloquea el puerto 25 saliente, asi que
	// el proveedor tiene que hablar por 587 o 465.
	if strings.Contains(strings.ToLower(c.SMTPHost), localMailHost) {
		problemas = append(problemas,
			"SMTP_HOST apunta a mailpit: en produccion el correo sale por un SMTP real")
	}
	if c.SMTPPort == 25 {
		problemas = append(problemas,
			"SMTP_PORT=25: Google bloquea ese puerto saliente en Compute Engine y no se puede abrir; usar 587 o 465")
	}

	if len(problemas) == 0 {
		return nil
	}
	return fmt.Errorf(
		"la configuracion de produccion conserva valores de desarrollo:\n  - %s",
		strings.Join(problemas, "\n  - "))
}
