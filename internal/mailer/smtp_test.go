package mailer

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
)

// Issue #126. Estas pruebas existen por una garantía concreta: cuando hay
// credenciales configuradas, no se envían sin cifrar.
//
// Hasta este issue el cliente intentaba STARTTLS solo si el servidor lo
// anunciaba y, si no lo anunciaba, seguía en claro con la contraseña dentro.
// `smtp.PlainAuth` de la biblioteca estándar lo frena por su cuenta, pero
// depender de eso significa depender de un detalle de implementación ajeno para
// una propiedad de seguridad nuestra.

// servidorFalso habla el mínimo de SMTP necesario para llegar al punto de
// decisión: saluda, responde al EHLO con las extensiones que se le indiquen, y
// registra los comandos que recibe.
//
// Se escribió a mano en lugar de usar una biblioteca porque lo que se prueba es
// exactamente la negociación, y un servidor de mentira que la simplifique
// dejaría de probar lo que importa.
type servidorFalso struct {
	extensiones []string // las que anuncia tras el EHLO
	comandos    []string // los que recibió, para inspeccionarlos
	direccion   string
	errLectura  error // un cierre inesperado de la conexion, por si el cliente falla raro
	cerrar      func()
}

func nuevoServidorFalso(t *testing.T, extensiones []string) *servidorFalso {
	t.Helper()

	// 127.0.0.1 y no "localhost": smtp.PlainAuth trata localhost como caso de
	// confianza y permitiría autenticar sin cifrado, que es justo la excepción
	// que estas pruebas no quieren tocar.
	escucha, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("no se pudo abrir el servidor de prueba: %v", err)
	}

	s := &servidorFalso{
		extensiones: extensiones,
		direccion:   escucha.Addr().String(),
		cerrar:      func() { _ = escucha.Close() },
	}

	go func() {
		conn, err := escucha.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

		escribir := func(formato string, args ...any) {
			fmt.Fprintf(conn, formato+"\r\n", args...)
		}
		escribir("220 servidorfalso ESMTP")

		lector := bufio.NewScanner(conn)
		for lector.Scan() {
			linea := lector.Text()
			s.comandos = append(s.comandos, linea)

			switch {
			case strings.HasPrefix(strings.ToUpper(linea), "EHLO"):
				// La última línea de una respuesta multilínea lleva espacio en
				// lugar de guion; sin eso el cliente se queda esperando.
				if len(s.extensiones) == 0 {
					escribir("250 servidorfalso")
					continue
				}
				escribir("250-servidorfalso")
				for i, ext := range s.extensiones {
					if i == len(s.extensiones)-1 {
						escribir("250 %s", ext)
					} else {
						escribir("250-%s", ext)
					}
				}
			case strings.HasPrefix(strings.ToUpper(linea), "DATA"):
				// DATA no se responde con 250: el servidor pide el cuerpo con un
				// 354, lo consume hasta una linea con un solo punto, y entonces
				// confirma. Saltarse ese paso hacia fallar el envio con un error
				// que no tenia nada que ver con lo que se estaba probando.
				escribir("354 adelante, termina con un punto")
				for lector.Scan() {
					if lector.Text() == "." {
						break
					}
				}
				escribir("250 mensaje aceptado")
			case strings.HasPrefix(strings.ToUpper(linea), "QUIT"):
				escribir("221 adios")
				return
			default:
				escribir("250 vale")
			}
		}

		// Un error del lector aqui no puede fallar la prueba --esta en otra
		// goroutine-- pero dejarlo sin mirar oculta un cierre inesperado de la
		// conexion, que se manifestaria como un fallo confuso en el cliente.
		if err := lector.Err(); err != nil {
			s.errLectura = err
		}
	}()

	t.Cleanup(s.cerrar)
	return s
}

func (s *servidorFalso) hostPuerto(t *testing.T) (string, int) {
	t.Helper()
	host, puerto, err := net.SplitHostPort(s.direccion)
	if err != nil {
		t.Fatalf("direccion inesperada %q: %v", s.direccion, err)
	}
	n, err := strconv.Atoi(puerto)
	if err != nil {
		t.Fatalf("puerto inesperado %q: %v", puerto, err)
	}
	return host, n
}

// La garantía: si el servidor no ofrece STARTTLS y hay credenciales, no se envía.
func TestSendSeNiegaAAutenticarSinStartTLS(t *testing.T) {
	servidor := nuevoServidorFalso(t, nil) // no anuncia STARTTLS
	host, puerto := servidor.hostPuerto(t)

	m := NewSMTPMailer(&config.Config{
		SMTPHost:     host,
		SMTPPort:     puerto,
		SMTPFrom:     "no-reply@ejemplo.test",
		SMTPUsername: "usuario",
		SMTPPassword: "secreto",
	})

	err := m.Send(context.Background(), "alguien@ejemplo.test", "Asunto", "Cuerpo")

	if err == nil {
		t.Fatal("se envió contra un servidor sin STARTTLS teniendo credenciales configuradas")
	}
	if !strings.Contains(err.Error(), "STARTTLS") {
		t.Errorf("el error no menciona STARTTLS, así que quien despliega no sabrá qué corregir:\n%v", err)
	}

	// Lo que de verdad importa: la contraseña no llegó a salir.
	for _, c := range servidor.comandos {
		if strings.HasPrefix(strings.ToUpper(c), "AUTH") {
			t.Errorf("se envió un comando AUTH sin cifrado: %q", c)
		}
	}
}

// Sin credenciales sí se permite la sesión en claro: es el caso de Mailpit en
// desarrollo, donde no hay nada que proteger y exigir TLS obligaría a montar
// certificados para un contenedor local.
func TestSendPermiteSesionEnClaroSinCredenciales(t *testing.T) {
	servidor := nuevoServidorFalso(t, nil)
	host, puerto := servidor.hostPuerto(t)

	m := NewSMTPMailer(&config.Config{
		SMTPHost: host,
		SMTPPort: puerto,
		SMTPFrom: "no-reply@ejemplo.test",
	})

	if err := m.Send(context.Background(), "alguien@ejemplo.test", "Asunto", "Cuerpo"); err != nil {
		t.Fatalf("falló el envío sin credenciales, que es el caso de Mailpit: %v", err)
	}

	var vioDatos bool
	for _, c := range servidor.comandos {
		if strings.HasPrefix(strings.ToUpper(c), "DATA") {
			vioDatos = true
		}
		if strings.HasPrefix(strings.ToUpper(c), "AUTH") {
			t.Errorf("se intentó autenticar sin credenciales: %q", c)
		}
	}
	if !vioDatos {
		t.Error("el servidor no recibió DATA: el mensaje no se entregó")
	}
}

// El puerto 465 cambia el protocolo, no una opción: la sesión va cifrada desde
// el primer byte. Contra un servidor que habla en claro, el saludo TLS no
// encaja y el envío tiene que fallar en lugar de degradarse a texto plano.
func TestSendConPuerto465NoSeDegradaATextoPlano(t *testing.T) {
	servidor := nuevoServidorFalso(t, nil)
	host, _ := servidor.hostPuerto(t)

	// Se fuerza el 465 contra un servidor que no habla TLS. Da igual que el
	// puerto real del servidor falso sea otro: lo que se comprueba es que el
	// cliente intenta TLS directo, y para eso basta con que el destino no lo
	// hable.
	m := NewSMTPMailer(&config.Config{
		SMTPHost:     host,
		SMTPPort:     465,
		SMTPFrom:     "no-reply@ejemplo.test",
		SMTPUsername: "usuario",
		SMTPPassword: "secreto",
	})
	// Keep the protocol selected by port 465 but direct the connection to the
	// ephemeral test listener. Otherwise a connection refusal on the real port
	// would make the test pass without proving that TLS is attempted.
	m.addr = servidor.direccion

	ctx, cancelar := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelar()

	err := m.Send(ctx, "alguien@ejemplo.test", "Asunto", "Cuerpo")
	if err == nil {
		t.Fatal("el envío por 465 tuvo éxito contra un servidor que no habla TLS")
	}

	// Y no se queda colgado: el contexto se respeta, que es la razón de usar
	// tls.Dialer en lugar de tls.Dial.
	if ctx.Err() == context.DeadlineExceeded {
		t.Error("el envío agotó el contexto en lugar de fallar: el dial no respeta el tiempo límite")
	}
}
