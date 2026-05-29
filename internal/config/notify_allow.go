package config

import "strings"

// ResolveTelegramTestCredentials returns token and chat ID allowed for notify test.
// Empty token in the request is filled from config when chat_id matches a stored target.
func (c *Config) ResolveTelegramTestCredentials(token, chatID string) (allowedToken, allowedChat string, ok bool) {
	if c == nil {
		return "", "", false
	}
	chatID = strings.TrimSpace(chatID)
	token = strings.TrimSpace(token)
	if chatID == "" {
		return "", "", false
	}
	for _, t := range c.Telegram.Targets {
		if strings.TrimSpace(t.ChatID) != chatID {
			continue
		}
		stored := strings.TrimSpace(t.Token)
		if stored == "" {
			continue
		}
		if token == "" || token == stored {
			return stored, chatID, true
		}
	}
	return "", "", false
}

// ResolveDiscordTestWebhook returns a webhook URL allowed for notify test.
func (c *Config) ResolveDiscordTestWebhook(webhook string) (allowed string, ok bool) {
	if c == nil {
		return "", false
	}
	webhook = strings.TrimSpace(webhook)
	if webhook == "" {
		return "", false
	}
	for _, w := range c.Discord.Webhooks {
		if strings.TrimSpace(w.Webhook) == webhook {
			return webhook, true
		}
	}
	return "", false
}
