package config

import (
	"fmt"
	"net"
	"net/url"
	"slices"
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

	// Dominio del remitente por defecto del Compose de produccion. **No esta
	// registrado**, comprobado con una consulta DNS: no resuelve. Un correo
	// enviado desde ahi lo rechaza el proveedor --exige que el remitente este
	// verificado-- o lo descarta el destinatario por SPF.
	//
	// El equipo decidio no comprar dominio (issue #126), asi que el remitente es
	// una direccion real verificada como remitente unico en el proveedor. Este
	// valor solo sobrevive a un despliegue si nadie lo configuro.
	//
	// Si algun dia se registra el dominio, esta constante se borra.
	unregisteredMailDomain = "plataforma-mooc.online"
)

// Modos de sslmode que cifran de verdad, para el issue #118 (C1).
//
// Los tres que faltan --disable, allow y prefer-- comparten un problema: el
// cifrado queda a criterio del servidor. Contra la instancia de Cloud SQL, que
// esta en ENCRYPTED_ONLY, «prefer» acabaria cifrando igual, y eso es
// precisamente lo que lo hace peligroso: funciona, asi que nadie lo corrige, y
// el dia que la cadena de conexion apunte a otro servidor viajaria en claro sin
// que cambie ni una linea de codigo.
var sslModesCifrados = []string{"require", "verify-ca", "verify-full"}

// sslModeDe extrae el sslmode de una cadena de conexion.
//
// Funciona con las dos formas que admite libpq --la URL
// (`postgres://...?sslmode=x`) y la de pares clave=valor (`host=... sslmode=x`)--
// porque no parsea: busca el parametro y lee hasta el siguiente separador. Para
// una comprobacion de arranque eso es mas robusto que un parseo, que fallaria
// entero por una contrasena con caracteres raros.
func sslModeDe(dsn string) string {
	const clave = "sslmode="

	i := strings.Index(dsn, clave)
	if i < 0 {
		return ""
	}

	valor := dsn[i+len(clave):]
	if fin := strings.IndexAny(valor, "&? "); fin >= 0 {
		valor = valor[:fin]
	}
	return strings.ToLower(strings.TrimSpace(valor))
}

// Validate comprueba que la configuracion no arrastra valores de desarrollo a
// un entorno que no lo es. Es la comprobacion de la API, que sirve HTTP y envia
// correo, asi que exige tambien lo que hace falta para eso.
//
// Devuelve todos los problemas juntos y no el primero: quien despliega prefiere
// una lista de cuatro cosas que corregir a cuatro intentos de arranque.
func (c *Config) Validate() error {
	return c.validar(true)
}

// ValidateWorker es la misma comprobacion para el proceso que no sirve HTTP ni
// manda correo.
//
// Existe porque la version unica era imposible de satisfacer. El worker exigia
// SMTP_PASSWORD, y mail.tf le niega ese secreto a proposito --«el worker no
// aparece a proposito, y no es un olvido: procesa video y no manda mensajes»--,
// asi que la unica forma de arrancarlo en produccion era darle una credencial
// que la infraestructura decidio que no debia tener, o inventarse un valor
// falso para enganar al validador.
//
// Lo encontro G3: el worker llevaba en bucle de reinicio desde que se recrearon
// las VMs, quejandose de APP_BASE_URL, CSRF y SMTP --tres cosas que no usa--
// mientras la cola se llenaba de tareas que nadie procesaba. Un arranque que
// solo se puede lograr contradiciendo el modelo de permisos no es una
// comprobacion, es un obstaculo. Ver la nota 20 de
// docs/entrega2/NOTAS_TECNICAS.md.
//
// Lo que si comparte con la API se sigue comprobando entero: la base, su
// cifrado, el almacenamiento y el tamano de los pools. Ahi un valor de
// desarrollo tiene exactamente las mismas consecuencias en los dos procesos.
func (c *Config) ValidateWorker() error {
	return c.validar(false)
}

// validar reune las comprobaciones. sirveHTTP distingue a la API del worker: no
// es un "modo estricto" y uno laxo, son dos procesos con superficies distintas.
func (c *Config) validar(sirveHTTP bool) error {
	var problemas []string

	// Esta si se comprueba en todo entorno, porque en ninguno es intencionada:
	// database/sql recorta en silencio el maximo de conexiones inactivas al de
	// abiertas, asi que pedir mas inactivas que abiertas no da error, no hace
	// nada y deja a quien lo configuro creyendo que si.
	if c.DBMaxIdleConns > c.DBMaxOpenConns {
		problemas = append(problemas, fmt.Sprintf(
			"DB_MAX_IDLE_CONNS=%d es mayor que DB_MAX_OPEN_CONNS=%d: database/sql lo recorta en silencio",
			c.DBMaxIdleConns, c.DBMaxOpenConns))
	}

	if c.WorkerConcurrency < 0 {
		problemas = append(problemas, fmt.Sprintf(
			"WORKER_CONCURRENCY=%d no puede ser negativo", c.WorkerConcurrency))
	}

	if !strings.EqualFold(c.Environment, EnvironmentProduction) {
		if len(problemas) == 0 {
			return nil
		}
		return unError(problemas)
	}

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

	if sirveHTTP {
		problemas = append(problemas, c.problemasDeSuperficieHTTP()...)
	}

	// La instancia de Cloud SQL del issue #118 no tiene IP publica y esta en
	// ENCRYPTED_ONLY, asi que rechaza las conexiones en claro. Sin esta
	// comprobacion, el sintoma de una cadena con `sslmode=disable` es un fallo
	// de conexion en el arranque cuyo mensaje no menciona el cifrado.
	if modo := sslModeDe(c.DatabaseURL); !slices.Contains(sslModesCifrados, modo) {
		if modo == "" {
			problemas = append(problemas,
				"DATABASE_URL no lleva sslmode: Cloud SQL exige conexion cifrada; usar sslmode=require")
		} else {
			problemas = append(problemas, fmt.Sprintf(
				"DATABASE_URL usa sslmode=%s, que deja el cifrado a criterio del servidor; usar require, verify-ca o verify-full",
				modo))
		}
	}

	// Cero significa «sin limite» en database/sql, y un pool sin limite contra
	// una instancia con max_connections declarado es la forma de descubrir el
	// limite bajo carga (nota 11 de NOTAS_TECNICAS.md). El reparto de las 100
	// conexiones esta en infra/terraform/database.tf.
	if c.DBMaxOpenConns <= 0 {
		problemas = append(problemas,
			"DB_MAX_OPEN_CONNS sin limite: la instancia administrada tiene un tope de conexiones y lo comparten la API y el worker")
	}

	if len(problemas) == 0 {
		return nil
	}
	return unError(problemas)
}

func httpsOrigin(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	return parsed.Scheme + "://" + parsed.Host, true
}

// unError junta los problemas en un solo mensaje. Quien despliega prefiere una
// lista que corregir a un arranque fallido por cada cosa.
func unError(problemas []string) error {
	return fmt.Errorf(
		"la configuracion de produccion conserva valores de desarrollo:\n  - %s",
		strings.Join(problemas, "\n  - "))
}

// problemasDeSuperficieHTTP reune lo que solo le toca al proceso que atiende
// peticiones y manda correo: los enlaces publicos, el proxy de confianza, el
// CSRF y las credenciales del proveedor SMTP. Nada de esto lo usa el worker, y
// la del correo ni siquiera puede leerla.
func (c *Config) problemasDeSuperficieHTTP() []string {
	var problemas []string

	// APP_BASE_URL arma los enlaces de activacion, de recuperacion de contrasena
	// y, desde el issue #113, el de verificacion de insignias. Si apunta a
	// localhost, nada de eso falla de forma visible: los correos se envian y los
	// enlaces sencillamente no abren.
	appOrigin, appOriginOK := httpsOrigin(c.AppBaseURL)
	if c.AppBaseURL == "" {
		problemas = append(problemas, "APP_BASE_URL vacio: los enlaces de los correos no se pueden construir")
	} else if strings.Contains(c.AppBaseURL, "localhost") || strings.Contains(c.AppBaseURL, "127.0.0.1") {
		problemas = append(problemas, "APP_BASE_URL apunta a localhost: los enlaces publicos no abririan para nadie")
	} else if !appOriginOK {
		problemas = append(problemas, "APP_BASE_URL debe ser un origen HTTPS publico sin rutas")
	}
	if net.ParseIP(c.TrustedProxyIP) == nil {
		problemas = append(problemas, "TRUSTED_PROXY_IP debe ser la IP interna fija del proxy")
	}

	if len(c.CSRFAllowedOrigins) == 0 {
		problemas = append(problemas, "CSRF_ALLOWED_ORIGINS vacio en produccion")
	} else {
		containsAppOrigin := false
		for _, origin := range c.CSRFAllowedOrigins {
			parsedOrigin, ok := httpsOrigin(origin)
			if !ok || parsedOrigin != origin {
				problemas = append(problemas, "CSRF_ALLOWED_ORIGINS debe contener solo origenes HTTPS sin rutas")
				break
			}
			if appOriginOK && origin == appOrigin {
				containsAppOrigin = true
			}
		}
		if appOriginOK && !containsAppOrigin {
			problemas = append(problemas, "CSRF_ALLOWED_ORIGINS debe incluir el origen de APP_BASE_URL")
		}
	}

	// Mailpit es el buzon de desarrollo. En la nube el correo sale por un SMTP
	// real (issue #126), y ademas Google bloquea el puerto 25 saliente. Brevo
	// soporta 587 (preferido), 465 y 2525 (alterno para redes restrictivas).
	if strings.Contains(strings.ToLower(c.SMTPHost), localMailHost) {
		problemas = append(problemas,
			"SMTP_HOST apunta a mailpit: en produccion el correo sale por un SMTP real")
	}
	if c.SMTPPort == 25 {
		problemas = append(problemas,
			"SMTP_PORT=25: Google bloquea ese puerto saliente en Compute Engine y no se puede abrir; usar 587, 465 o 2525")
	} else if !slices.Contains([]int{587, 465, 2525}, c.SMTPPort) {
		problemas = append(problemas,
			"SMTP_PORT no esta permitido en produccion: usar 587, 465 o 2525")
	}

	// Issue #126. Sin credenciales, internal/mailer/smtp.go omite la
	// autenticacion por completo, el proveedor rechaza el mensaje y el fallo es
	// invisible donde importa: **el registro funciona** --el usuario queda
	// creado-- y el correo de verificacion no llega nunca. Quien se registra ve
	// una cuenta que no puede activar y nadie ve un error.
	//
	// Es el mismo patron que el resto de este archivo: un valor que en local es
	// correcto --Mailpit no autentica-- y en la nube significa que algo no se
	// inyecto.
	if c.SMTPUsername == "" {
		problemas = append(problemas,
			"SMTP_USERNAME vacio: sin credenciales el proveedor rechaza el correo y el registro parece funcionar sin que llegue nada")
	}
	if c.SMTPPassword == "" {
		problemas = append(problemas,
			"SMTP_PASSWORD vacio: la contrasena del proveedor vive en Secret Manager y se inyecta en ejecucion")
	}

	// El remitente tiene que ser el que este verificado en el proveedor. La
	// aplicacion no puede comprobar eso, pero si puede descartar los dos casos
	// que garantizan un fallo: vacio, y el dominio de ejemplo que no existe.
	if c.SMTPFrom == "" {
		problemas = append(problemas, "SMTP_FROM vacio: hace falta el remitente verificado en el proveedor")
	} else if strings.Contains(strings.ToLower(c.SMTPFrom), unregisteredMailDomain) {
		problemas = append(problemas, fmt.Sprintf(
			"SMTP_FROM usa %s, que no es un dominio registrado: el proveedor exige un remitente verificado",
			unregisteredMailDomain))
	} else if !strings.Contains(c.SMTPFrom, "@") {
		problemas = append(problemas, "SMTP_FROM no parece una direccion de correo")
	}

	return problemas
}
