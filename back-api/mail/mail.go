package mail

import (
	"fmt"
	"mime"
	"net/smtp"
	"strings"

	"github.com/fedutinova/smartheart/back-api/config"
)

type Sender struct {
	host    string
	port    int
	user    string
	pass    string
	address string // bare email for SMTP envelope
	header  string // formatted From header value
}

func NewSender(cfg config.SMTPConfig) *Sender {
	header := cfg.From
	if cfg.FromName != "" {
		header = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("UTF-8", cfg.FromName), cfg.From)
	}

	return &Sender{
		host:    cfg.Host,
		port:    cfg.Port,
		user:    cfg.User,
		pass:    cfg.Password,
		address: cfg.From,
		header:  header,
	}
}

func (s *Sender) Send(to, subject, htmlBody string) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	var msg strings.Builder
	fmt.Fprintf(&msg, "From: %s\r\n", s.header)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", subject))
	fmt.Fprintf(&msg, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&msg, "Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	auth := smtp.PlainAuth("", s.user, s.pass, s.host)
	return smtp.SendMail(addr, auth, s.address, []string{to}, []byte(msg.String()))
}
