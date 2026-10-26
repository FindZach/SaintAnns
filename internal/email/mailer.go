// Package email sends notification mail from the parish site. SMTP is
// optional — if the SMTP_HOST env var is empty the Mailer logs the message
// instead of sending it, so local development works without a mail account.
package email

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"olv-parish/internal/config"
)

// Mailer sends plain-text notifications via SMTP. Create it with the server's
// config and reuse it; it holds no connection, each Send dials fresh.
type Mailer struct {
	cfg config.EmailConfig
}

func New(cfg config.EmailConfig) *Mailer {
	return &Mailer{cfg: cfg}
}

// Enabled reports whether the mailer will actually send. When false, Send only
// logs — useful in dev or when the parish hasn't given us SMTP yet.
func (m *Mailer) Enabled() bool {
	return m != nil && m.cfg.SMTPHost != ""
}

// Send delivers one plain-text message. If the mailer is disabled, the message
// is written to the server log instead and a nil error is returned.
func (m *Mailer) Send(to, subject, body string) error {
	if m == nil {
		return errors.New("email: nil mailer")
	}
	from := m.cfg.FromAddress
	if from == "" {
		from = m.cfg.SMTPUsername
	}
	if from == "" {
		from = "noreply@olv-parish.local"
	}

	// Compose the message with CRLF line endings per RFC 5322.
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=utf-8\r\n\r\n")
	buf.WriteString(normalizeNewlines(body))

	if !m.Enabled() {
		log.Printf("Mailer (disabled; would send to %s):\n%s", to, buf.String())
		return nil
	}

	addr := fmt.Sprintf("%s:%d", m.cfg.SMTPHost, m.cfg.SMTPPort)
	var auth smtp.Auth
	if m.cfg.SMTPUsername != "" {
		auth = smtp.PlainAuth("", m.cfg.SMTPUsername, m.cfg.SMTPPassword, m.cfg.SMTPHost)
	}
	if err := smtp.SendMail(addr, auth, from, []string{to}, buf.Bytes()); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}
