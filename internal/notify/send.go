package notify

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/netpolicy"
)

// staggerDelay spaces real fan-out sends so a single monitor flip with several
// recipients does not hit provider burst rate limits at once.
const staggerDelay = 250 * time.Millisecond

// SendAll delivers alert through every notifier sequentially, pausing briefly
// between recipients. It returns the per-notifier errors in input order with
// nil entries on success.
func SendAll(notifiers []Notifier, a Alert) []error {
	if len(notifiers) == 0 {
		return nil
	}
	errs := make([]error, len(notifiers))
	for i, n := range notifiers {
		if i > 0 {
			time.Sleep(staggerDelay)
		}
		errs[i] = n.Send(a)
	}
	return errs
}

type TelegramNotifier struct {
	token  string
	chatID string
	client *http.Client
}

func NewTelegram(token, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		token:  token,
		chatID: chatID,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		},
	}
}

func (t *TelegramNotifier) Send(a Alert) error {
	text := AlertText(a)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.token)
	body, _ := json.Marshal(map[string]string{
		"chat_id": t.chatID,
		"text":    text,
	})
	resp, err := t.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API error: %d", resp.StatusCode)
	}
	return nil
}

type DiscordNotifier struct {
	webhook string
	client  *http.Client
}

func NewDiscord(webhook string) *DiscordNotifier {
	return &DiscordNotifier{
		webhook: webhook,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		},
	}
}

func (d *DiscordNotifier) Send(a Alert) error {
	text := AlertText(a)
	body, _ := json.Marshal(map[string]string{
		"content": text,
	})
	resp, err := d.client.Post(d.webhook, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook error: %d", resp.StatusCode)
	}
	return nil
}

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

type WebhookNotifier struct {
	url     string
	headers map[string]string
	client  *http.Client
}

func NewWebhook(rawURL string, headers map[string]string) *WebhookNotifier {
	h := make(map[string]string, len(headers))
	for k, v := range headers {
		h[k] = v
	}
	return &WebhookNotifier{
		url:     strings.TrimSpace(rawURL),
		headers: h,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		},
	}
}

func ValidateWebhookURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook URL must be http or https")
	}
	return netpolicy.ResolvePublicHost(u.Hostname())
}

func (w *WebhookNotifier) Send(a Alert) error {
	if err := ValidateWebhookURL(w.url); err != nil {
		return err
	}
	body := bytes.NewReader([]byte(AlertText(a)))
	req, err := http.NewRequest(http.MethodPost, w.url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook HTTP %d", resp.StatusCode)
	}
	return nil
}
