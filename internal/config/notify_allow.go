package config

import "strings"

// ResolveTelegramTestCredentials returns token and chat ID allowed for notify test.
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

// ResolveEmailTestTarget returns SMTP and recipient allowed for notify test.
func (c *Config) ResolveEmailTestTarget(to string) (target EmailTarget, ok bool) {
	if c == nil {
		return EmailTarget{}, false
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return EmailTarget{}, false
	}
	for _, t := range c.Email.Targets {
		if strings.EqualFold(strings.TrimSpace(t.To), to) {
			return t, true
		}
	}
	return EmailTarget{}, false
}

// ResolveWebhookTestURL returns a generic webhook URL allowed for notify test.
func (c *Config) ResolveWebhookTestURL(rawURL string) (allowed string, ok bool) {
	if c == nil {
		return "", false
	}
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", false
	}
	for _, w := range c.Webhook.Webhooks {
		if strings.TrimSpace(w.URL) == rawURL {
			return rawURL, true
		}
	}
	return "", false
}
