package notify

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/config"
)

type EmailNotifier struct {
	smtp config.SMTPConfig
	to   string
}

func NewEmail(smtpCfg config.SMTPConfig, to string) *EmailNotifier {
	return &EmailNotifier{smtp: smtpCfg, to: strings.TrimSpace(to)}
}

func (e *EmailNotifier) Send(a Alert) error {
	if e.to == "" {
		return fmt.Errorf("email recipient is empty")
	}
	host := strings.TrimSpace(e.smtp.Host)
	if host == "" {
		return fmt.Errorf("smtp host is not configured")
	}
	port := e.smtp.Port
	if port <= 0 {
		port = 587
	}
	from := strings.TrimSpace(e.smtp.From)
	if from == "" {
		from = strings.TrimSpace(e.smtp.Username)
	}
	if from == "" {
		return fmt.Errorf("smtp from address is not configured")
	}
	subject := fmt.Sprintf("Beacon: %s %s", a.MonitorName, strings.ToUpper(a.Status))
	body := AlertText(a)
	msg := buildPlainEmail(from, e.to, subject, body)
	addr := fmt.Sprintf("%s:%d", host, port)
	auth := smtpAuth(e.smtp)
	switch strings.ToLower(e.smtp.TLS) {
	case "ssl":
		return sendSMTPS(addr, auth, from, []string{e.to}, msg)
	default:
		return smtp.SendMail(addr, auth, from, []string{e.to}, msg)
	}
}

func smtpAuth(c config.SMTPConfig) smtp.Auth {
	user := strings.TrimSpace(c.Username)
	if user == "" {
		return nil
	}
	host := strings.TrimSpace(c.Host)
	return smtp.PlainAuth("", user, c.Password, host)
}

func buildPlainEmail(from, to, subject, body string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return []byte(b.String())
}

func sendSMTPS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// EmailSendGuard limits production email frequency per destination.
type EmailSendGuard struct {
	last map[string]time.Time
}

func NewEmailSendGuard() *EmailSendGuard {
	return &EmailSendGuard{last: make(map[string]time.Time)}
}

const emailSafeInterval = 60 * time.Second

func (g *EmailSendGuard) Allow(key string) bool {
	now := time.Now()
	if prev, ok := g.last[key]; ok && now.Sub(prev) < emailSafeInterval {
		return false
	}
	g.last[key] = now
	return true
}
