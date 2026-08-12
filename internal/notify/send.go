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

type Alert struct {
	MonitorName string
	Status      string
	Message     string
	Body        string // rendered template; empty falls back to default templates
	Time        time.Time
	Target      string
	Type        string
	StatusCode  int
	Latency     time.Duration
	FailCount   int
}

type Notifier interface {
	Send(Alert) error
}

// AlertText returns the message body to send.
func AlertText(a Alert) string {
	if s := strings.TrimSpace(a.Body); s != "" {
		return s
	}
	status := a.Status
	if status != "recovered" {
		status = "down"
	}
	return BuildAlertBody(ResolvedPolicy{Templates: DefaultTemplates()}, status, TemplateContextFromAlert(a))
}

// staggerDelay spaces real fan-out sends so a single monitor flip with several
// recipients does not hit provider burst rate limits at once.
const staggerDelay = 250 * time.Millisecond

// Delivery outcomes, as reported to the recorder.
const (
	DeliverySent    = "sent"
	DeliveryFailed  = "failed"
	DeliverySkipped = "skipped"
)

// DeliveryEvent is one decision about one receiver.
//
// Every decision is reported, not just the sends — and the skips are the point.
// After an outage that nobody heard about, the question is "why was I not
// told", and silence is not an answer. "Suppressed: the policy sends once and
// it already had" is.
//
// Label is the display-safe name; the internal Key never travels here.
type DeliveryEvent struct {
	MonitorID string
	Channel   string
	Label     string
	// Kind is "down" or "recovered".
	Kind   string
	Status string
	Reason string
}

// SendResolved delivers an alert through resolved receivers with stagger and
// per-receiver dedup, reporting every decision to record.
func SendResolved(monitorID, status string, isRepeat bool, receivers []ResolvedReceiver, base Alert, tplCtx TemplateContext, emailGuard *EmailSendGuard, dedup *AlertDedup, dedupWindow time.Duration, logf func(string, ...any), record func(DeliveryEvent)) {
	kind := "down"
	if status == "recovered" {
		kind = "recovered"
	}
	report := func(r ResolvedReceiver, outcome, reason string) {
		if record == nil {
			return
		}
		record(DeliveryEvent{
			MonitorID: monitorID, Channel: r.Channel, Label: r.Label,
			Kind: kind, Status: outcome, Reason: reason,
		})
	}

	for i, r := range receivers {
		if r.Channel == ChannelEmail && emailGuard != nil && !emailGuard.Allow(monitorID, r.Key) {
			if logf != nil {
				logf("email cooldown skip [%s]", r.Key)
			}
			report(r, DeliverySkipped, "email cooldown is still running")
			continue
		}
		if status == "down" && !ShouldSendDown(r.Policy, isRepeat, r.Channel) {
			report(r, DeliverySkipped, "policy alerts once and it already did")
			continue
		}
		if dedup != nil && ((status == "down" && isRepeat) || status == "recovered") {
			if !dedup.Allow(monitorID, status, r.Key, dedupWindow) {
				report(r, DeliverySkipped, "the same alert went out moments ago")
				continue
			}
		}
		alert := base
		alert.Body = BuildAlertBody(r.Policy, status, tplCtx)
		if err := r.Notifier.Send(alert); err != nil {
			if logf != nil {
				logf("notify error [%s]: %v", r.Key, err)
			}
			report(r, DeliveryFailed, err.Error())
			continue
		}
		report(r, DeliverySent, "")
		if r.Channel == ChannelEmail && emailGuard != nil {
			emailGuard.RecordSuccess(monitorID, r.Key)
		}
		if i+1 < len(receivers) {
			time.Sleep(staggerDelay)
		}
	}
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
