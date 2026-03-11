package notify

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

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
	text := formatAlert(a)
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
