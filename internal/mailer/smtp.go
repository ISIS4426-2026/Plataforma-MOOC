// Package mailer delivers transactional email over SMTP. In development it
// targets the Mailpit container declared in docker-compose.yml, where messages
// are inspected in a web UI instead of reaching real inboxes.
package mailer

import (
	"context"
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

// SMTPMailer is the SMTP implementation of domain.Mailer.
type SMTPMailer struct {
	addr     string
	from     string
	username string
	password string
}

var _ domain.Mailer = (*SMTPMailer)(nil)

func NewSMTPMailer(cfg *config.Config) *SMTPMailer {
	return &SMTPMailer{
		addr:     net.JoinHostPort(cfg.SMTPHost, strconv.Itoa(cfg.SMTPPort)),
		from:     cfg.SMTPFrom,
		username: cfg.SMTPUsername,
		password: cfg.SMTPPassword,
	}
}

// Send delivers a plain-text message.
//
// The SMTP conversation is driven explicitly rather than through smtp.SendMail
// because that helper negotiates STARTTLS whenever the server advertises it,
// which fails certificate verification against a container hostname like
// "mailpit". Here TLS is upgraded only when credentials are configured, which
// is the case that actually needs protecting.
func (m *SMTPMailer) Send(ctx context.Context, to string, subject string, body string) error {
	if err := validateHeaderValue(to); err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}
	if err := validateHeaderValue(subject); err != nil {
		return fmt.Errorf("invalid subject: %w", err)
	}

	dialer := &net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", m.addr)
	if err != nil {
		return fmt.Errorf("dial smtp server %s: %w", m.addr, err)
	}

	client, err := smtp.NewClient(conn, m.host())
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("start smtp session: %w", err)
	}
	defer client.Close()

	if m.username != "" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(nil); err != nil {
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
