// Package mailer delivers transactional email over SMTP. In development it
// targets the Mailpit container declared in docker-compose.yml, where messages
// are inspected in a web UI instead of reaching real inboxes.
package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/config"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// dialTimeout keeps a stuck mail server from holding a request open. Sending is
// on the critical path of registration, so it must fail rather than hang.
const dialTimeout = 10 * time.Second

// implicitTLSPort es el puerto en el que la sesion va cifrada desde el primer
// byte, sin STARTTLS (issue #126).
//
// Importa porque los dos puertos hablan protocolos distintos: en el 587 se abre
// en claro y se asciende con STARTTLS, y en el 465 hay que negociar TLS antes de
// decir nada. Marcar el numero de puerto y no una opcion de configuracion evita
// una combinacion imposible --465 sin cifrado, 587 con TLS desde el principio--
// y una pregunta menos para quien despliega.
//
// Sin esto, configurar el 465 no da un error de cifrado: la conexion se queda
// esperando hasta agotar el tiempo, porque el servidor aguarda un saludo TLS que
// nunca llega.
const implicitTLSPort = 465

// SMTPMailer is the SMTP implementation of domain.Mailer.
type SMTPMailer struct {
	addr     string
	from     string
	username string
	password string

	// implicitTLS distingue el 465 del 587. Ver implicitTLSPort.
	implicitTLS bool
}

var _ domain.Mailer = (*SMTPMailer)(nil)

func NewSMTPMailer(cfg *config.Config) *SMTPMailer {
	return &SMTPMailer{
		addr:        net.JoinHostPort(cfg.SMTPHost, strconv.Itoa(cfg.SMTPPort)),
		from:        cfg.SMTPFrom,
		username:    cfg.SMTPUsername,
		password:    cfg.SMTPPassword,
		implicitTLS: cfg.SMTPPort == implicitTLSPort,
	}
}

// Send delivers a plain-text message.
//
// The SMTP conversation is driven explicitly rather than through smtp.SendMail
// because that helper negotiates STARTTLS whenever the server advertises it,
// which fails certificate verification against a container hostname like
// "mailpit". Here TLS is upgraded only when credentials are configured, which
// is the case that actually needs protecting.
//
// Issue #126. Cuando hay credenciales, el cifrado dejo de ser opcional. Antes se
// intentaba STARTTLS solo si el servidor lo anunciaba y, si no lo anunciaba, la
// sesion continuaba en claro con la contrasena dentro. Go lo frena por su cuenta
// --smtp.PlainAuth se niega a autenticar sin cifrado-- pero el error que produce
// habla de una conexion sin cifrar y no dice que el servidor no ofrecio STARTTLS,
// que es la causa. Ahora se comprueba aqui y el mensaje lo dice.
func (m *SMTPMailer) Send(ctx context.Context, to string, subject string, body string) error {
	if err := validateHeaderValue(to); err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}
	if err := validateHeaderValue(subject); err != nil {
		return fmt.Errorf("invalid subject: %w", err)
	}

	conn, err := m.dial(ctx)
	if err != nil {
		return err
	}

	client, err := smtp.NewClient(conn, m.host())
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("start smtp session: %w", err)
	}
	defer client.Close()

	if m.username != "" {
		// En el 465 la conexion ya viene cifrada; en el 587 hay que ascenderla.
		if !m.implicitTLS {
			ok, _ := client.Extension("STARTTLS")
			if !ok {
				return fmt.Errorf(
					"el servidor %s no ofrece STARTTLS y hay credenciales configuradas: "+
						"enviarlas sin cifrar no es aceptable; revisa el puerto (587 con STARTTLS, 465 con TLS directo)",
					m.addr)
			}
			if err := client.StartTLS(&tls.Config{
				MinVersion: tls.VersionTLS12,
				ServerName: m.host(),
			}); err != nil {
				return fmt.Errorf("start tls: %w", err)
			}
		}

		auth := smtp.PlainAuth("", m.username, m.password, m.host())
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp authentication: %w", err)
		}
	}

	if err := client.Mail(m.from); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}

	if _, err := writer.Write([]byte(m.buildMessage(to, subject, body))); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close message: %w", err)
	}

	return client.Quit()
}

// dial abre la conexion, cifrada desde el principio si el puerto es el de TLS
// implicito.
func (m *SMTPMailer) dial(ctx context.Context) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: dialTimeout}

	if !m.implicitTLS {
		conn, err := dialer.DialContext(ctx, "tcp", m.addr)
		if err != nil {
			return nil, fmt.Errorf("dial smtp server %s: %w", m.addr, err)
		}
		return conn, nil
	}

	// tls.Dialer respeta el contexto, a diferencia de tls.Dial. Importa porque
	// enviar correo esta en el camino critico del registro: si el servidor no
	// responde, la peticion tiene que fallar y no quedarse colgada.
	tlsDialer := &tls.Dialer{
		NetDialer: dialer,
		Config: &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: m.host(),
		},
	}
	conn, err := tlsDialer.DialContext(ctx, "tcp", m.addr)
	if err != nil {
		return nil, fmt.Errorf("dial smtp server %s con TLS directo: %w", m.addr, err)
	}
	return conn, nil
}

func (m *SMTPMailer) host() string {
	host, _, err := net.SplitHostPort(m.addr)
	if err != nil {
		return m.addr
	}
	return host
}

// buildMessage assembles an RFC 5322 message. The subject is Q-encoded so
// accented Spanish text survives transports that are not 8-bit clean.
func (m *SMTPMailer) buildMessage(to, subject, body string) string {
	var msg strings.Builder

	fmt.Fprintf(&msg, "From: %s\r\n", m.from)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&msg, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return msg.String()
}

// validateHeaderValue rejects CR and LF, which would otherwise let a crafted
// address or subject inject extra headers or a second message body.
func validateHeaderValue(value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("value contains line breaks")
	}
	return nil
}
