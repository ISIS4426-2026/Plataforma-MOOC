package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
)

// HeaderInstanceID carries the identity of the replica that answered.
//
// Con una sola maquina esto no hacia falta. Con un grupo de 1 a 3 instancias
// detras de un balanceador, sin el no hay forma de demostrar que el trafico se
// reparte --que es lo que piden C4 y el escenario 3A-- ni de leer un log y
// saber cual de las replicas lo escribio.
//
// El enunciado lo admite explicitamente: «para identificar los destinos se
// podran utilizar logs correlacionados o un identificador tecnico de instancia
// en las evidencias».
const HeaderInstanceID = "X-Instance-ID"

// InstanceID identifica a la replica que atiende, de forma estable mientras el
// proceso viva y distinta entre replicas.
//
// # Por que no se publica el nombre de la maquina tal cual
//
// El nombre que GCE le da a una instancia de un grupo administrado delata el
// nombre del grupo y el sufijo que lo numera. Eso es topologia interna, y el
// criterio de aceptacion de este issue pide que la respuesta no la revele.
//
// Asi que lo que sale al exterior es un resumen corto del nombre: basta para
// distinguir una replica de otra y para agrupar peticiones por destino, y no
// dice nada de como se llama la maquina ni cuantas hay.
//
// El valor completo si va al log, donde solo lo lee el equipo. Esa asimetria es
// deliberada: correlacionar en la consola de Cloud Logging necesita el nombre
// real; la evidencia que se publica, no.
type InstanceID struct {
	// Public es el resumen que viaja en la cabecera.
	Public string
	// Private es el nombre tal cual, para los logs.
	Private string
}

// NewInstanceID resuelve la identidad de esta replica.
//
// Prioridad: la variable de entorno que fije el script de arranque, y si no, el
// nombre de la maquina. En GCE el arranque puede poner ahi el id numerico que
// devuelve el servidor de metadatos, que es estable entre reinicios del
// contenedor y unico en el proyecto.
//
// Nunca devuelve un valor vacio: una replica sin identidad es justamente la que
// no se puede rastrear.
func NewInstanceID(fromEnv string) InstanceID {
	name := strings.TrimSpace(fromEnv)
	if name == "" {
		if host, err := os.Hostname(); err == nil {
			name = strings.TrimSpace(host)
		}
	}
	if name == "" {
		name = "desconocida"
	}

	sum := sha256.Sum256([]byte(name))
	return InstanceID{Public: hex.EncodeToString(sum[:])[:12], Private: name}
}

// WithInstanceID anade la cabecera a toda respuesta.
//
// Se coloca en la cadena antes que nada para que tambien la lleven las
// respuestas que rechaza un middleware --un 403 de CSRF o un 429 del
// limitador--, que son justo las que hay que poder atribuir a una replica
// cuando algo va mal en una corrida de carga.
func WithInstanceID(id InstanceID) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(HeaderInstanceID, id.Public)
			next.ServeHTTP(w, r)
		})
	}
}
