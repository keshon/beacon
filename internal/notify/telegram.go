package notify

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

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
	text := formatAlert(a)
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

func formatAlert(a Alert) string {
	if a.Status == "recovered" {
		return fmt.Sprintf("Site RECOVERED\n\n%s\n%s\nTime: %s",
			a.MonitorName, a.Message, a.Time.Format("2006-01-02 15:04"))
	}
	return fmt.Sprintf("Site DOWN\n\n%s\n%s\nTime: %s",
		a.MonitorName, a.Message, a.Time.Format("2006-01-02 15:04"))
}
