package services

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Mailer sends plain-text email over SMTP. It is disabled unless Host and
// From are set; Port defaults to 587 (STARTTLS is negotiated by the smtp
// client). Configure with SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASSWORD and
// DIGEST_FROM.
type Mailer struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

// MailSender is the narrow interface the digest runner needs, so tests can
// fake delivery.
type MailSender interface {
	Enabled() bool
	Send(ctx context.Context, to, subject, body string) error
}

func (m *Mailer) Enabled() bool {
	return m != nil && m.Host != "" && m.From != ""
}

// Send delivers a plain-text message. Delivery is blocking with a 10s dial
// and 15s total timeout; callers run it off the request path.
func (m *Mailer) Send(_ context.Context, to, subject, body string) error {
	if !m.Enabled() {
		return fmt.Errorf("mailer not configured")
	}
	port := m.Port
	if port == "" {
		port = "587"
	}
	addr := net.JoinHostPort(m.Host, port)
	tlsConfig := &tls.Config{ServerName: m.Host, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	var err error
	// Port 465 is implicit TLS (e.g. smtp.mx.cloudflare.net); other ports
	// upgrade with STARTTLS below.
	if port == "465" {
		//allagio:outbound fixed-host configured SMTP server
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsConfig)
	} else {
		//allagio:outbound fixed-host configured SMTP server
		conn, err = (&net.Dialer{Timeout: 10 * time.Second}).Dial("tcp", addr)
	}
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	client, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if port != "465" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err = client.StartTLS(tlsConfig); err != nil {
				return err
			}
		}
	}
	if m.User != "" {
		if err = client.Auth(smtp.PlainAuth("", m.User, m.Password, m.Host)); err != nil {
			return err
		}
	}
	// MAIL FROM takes a bare mailbox; DIGEST_FROM may carry a display name
	// ("Allagio <digest@allagio.dev>") which belongs in the From: header only.
	envelope := m.From
	if parsed, parseErr := mail.ParseAddress(m.From); parseErr == nil {
		envelope = parsed.Address
	}
	if err = client.Mail(envelope); err != nil {
		return err
	}
	if err = client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	var msg strings.Builder
	fmt.Fprintf(&msg, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s", m.From, to, subject, body)
	if _, err = w.Write([]byte(msg.String())); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
