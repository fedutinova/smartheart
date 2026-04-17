package mail

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/fedutinova/smartheart/back-api/config"
)

// Sender sends emails via SMTP.
type Sender struct {
	host string
	port int
	user string
	pass string
	from string
}

// NewSender creates a Sender from SMTP config.
func NewSender(cfg config.SMTPConfig) *Sender {
	return &Sender{
		host: cfg.Host,
		port: cfg.Port,
		user: cfg.User,
		pass: cfg.Password,
		from: cfg.From,
	}
}

// Send sends an HTML email to the given recipient.
func (s *Sender) Send(to, subject, htmlBody string) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	var msg strings.Builder
	fmt.Fprintf(&msg, "From: %s\r\n", s.from)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %s\r\n", subject)
	fmt.Fprintf(&msg, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&msg, "Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	auth := smtp.PlainAuth("", s.user, s.pass, s.host)
	return smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg.String()))
}
