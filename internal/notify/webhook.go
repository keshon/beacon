package notify

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/netpolicy"
)

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
