package middleware

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// CORS para el caso en que el frontend no comparta origen con la API.
//
// # Cuando hace falta, y cuando no
//
// Si el frontend se sirve detras del mismo balanceador que la API --misma
// maquina de origen para el navegador-- no hace falta nada de esto: no hay
// peticion entre origenes, no hay preflight y la cookie viaja como siempre.
// Esa es la configuracion recomendada, y con ella este middleware no llega a
// anadir una sola cabecera.
//
// Si D1 decide alojarlo aparte --un bucket con su propio dominio, por
// ejemplo-- entonces cada peticion del frontend es entre origenes y hacen
// falta tres cosas a la vez, no una:
//
//  1. CORS con `Allow-Credentials`, que es esto.
//  2. `SESSION_COOKIE_SAMESITE=none`, o el navegador no mandara la cookie.
//  3. El origen del frontend en CSRF_ALLOWED_ORIGINS.
//
// Faltando cualquiera de las tres, el frontend no se autentica. Es la clase de
// fallo que se diagnostica mal: el login responde 200, deja su cookie, y la
// siguiente peticion llega sin sesion.
//
// # Por que no hay comodin
//
// `Access-Control-Allow-Origin: *` es incompatible con `Allow-Credentials`
// segun la propia especificacion, y el navegador rechaza la respuesta. Pero
// ademas seria incorrecto aqui: la lista de origenes es la misma que sostiene
// la proteccion contra CSRF, y un comodin la anularia. Se devuelve el origen
// exacto, solo si esta en la lista.
type CORSConfig struct {
	// AllowedOrigins se compara exacto, con esquema y puerto. Vacio desactiva
	// el middleware entero.
	AllowedOrigins []string
}

const (
	headerOrigin          = "Origin"
	headerACAllowOrigin   = "Access-Control-Allow-Origin"
	headerACAllowMethods  = "Access-Control-Allow-Methods"
	headerACAllowHeaders  = "Access-Control-Allow-Headers"
	headerACAllowCreds    = "Access-Control-Allow-Credentials"
	headerACExposeHeaders = "Access-Control-Expose-Headers"
	headerACMaxAge        = "Access-Control-Max-Age"
	headerACReqHeaders    = "Access-Control-Request-Headers"
)

// corsMaxAge es cuanto puede el navegador recordar el resultado del preflight.
// Diez minutos: suficiente para que una sesion de uso no pague un OPTIONS por
// peticion, y corto para que un cambio de configuracion no tarde en notarse.
const corsMaxAge = 10 * time.Minute

// metodosPermitidos son los que la API usa de verdad. Enumerarlos en lugar de
// responder con lo que pida el navegador evita anunciar superficie que no
// existe.
var metodosPermitidos = []string{
	http.MethodGet, http.MethodPost, http.MethodPut,
	http.MethodPatch, http.MethodDelete, http.MethodOptions,
}

// cabecerasPermitidas son las que el frontend necesita enviar.
//
// Idempotency-Key esta porque el envio de un quiz la exige, y X-Request-ID
// porque es como una traza sobrevive desde el navegador hasta los workers.
var cabecerasPermitidas = []string{
	"Content-Type", "Authorization", "Idempotency-Key", HeaderRequestID,
}

// cabecerasExpuestas son las que el navegador deja leer al JavaScript. Sin
// esto, `fetch` no puede ver ninguna cabecera de respuesta salvo un punado de
// seguras, y el frontend no podria correlacionar una peticion con su traza ni
// saber que replica le contesto.
var cabecerasExpuestas = []string{HeaderRequestID, HeaderInstanceID}

// CORS responde a los preflight y marca las respuestas con el origen permitido.
func CORS(cfg CORSConfig) Middleware {
	permitidos := make([]string, 0, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		if o = strings.TrimSpace(o); o != "" {
			permitidos = append(permitidos, o)
		}
	}

	return func(next http.Handler) http.Handler {
		if len(permitidos) == 0 {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origen := r.Header.Get(headerOrigin)

			// Sin Origin no hay nada que decidir: o es una peticion del mismo
			// sitio, o no viene de un navegador.
			if origen == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Vary es obligatorio: la respuesta depende del origen, y sin esto
			// una cache intermedia podria servirle a un origen la respuesta
			// marcada para otro.
			w.Header().Add("Vary", headerOrigin)

			if !slices.Contains(permitidos, origen) {
				// No se responde un error: se omiten las cabeceras y el
				// navegador bloquea la lectura por su cuenta. Devolver 403
				// aqui convertiria en fallo del servidor algo que es una
				// politica del cliente.
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set(headerACAllowOrigin, origen)
			w.Header().Set(headerACAllowCreds, "true")
			w.Header().Set(headerACExposeHeaders, strings.Join(cabecerasExpuestas, ", "))

			if r.Method != http.MethodOptions || r.Header.Get("Access-Control-Request-Method") == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Preflight: se contesta aqui y no llega al enrutador.
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			w.Header().Set(headerACAllowMethods, strings.Join(metodosPermitidos, ", "))
			w.Header().Set(headerACAllowHeaders, cabecerasParaPreflight(r.Header.Get(headerACReqHeaders)))
			w.Header().Set(headerACMaxAge, strconv.Itoa(int(corsMaxAge.Seconds())))
			w.WriteHeader(http.StatusNoContent)
		})
	}
}

// cabecerasParaPreflight devuelve la interseccion entre lo que el navegador
// pide y lo que la API admite.
//
// Devolver lo pedido sin filtrar seria anunciar que se acepta cualquier
// cabecera, que es lo mismo que no tener lista.
func cabecerasParaPreflight(pedidas string) string {
	if strings.TrimSpace(pedidas) == "" {
		return strings.Join(cabecerasPermitidas, ", ")
	}

	admitidas := make([]string, 0, len(cabecerasPermitidas))
	for _, pedida := range strings.Split(pedidas, ",") {
		pedida = strings.TrimSpace(pedida)
		for _, permitida := range cabecerasPermitidas {
			if strings.EqualFold(pedida, permitida) {
				admitidas = append(admitidas, permitida)
				break
			}
		}
	}
	if len(admitidas) == 0 {
		return strings.Join(cabecerasPermitidas, ", ")
	}
	return strings.Join(admitidas, ", ")
}
