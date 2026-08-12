package config

import "strings"

// SecretsPresent indicates which secret fields are stored server-side.
type SecretsPresent struct {
	Password         bool   `json:"password"`
	TelegramTokens   []bool `json:"telegram_tokens"`
	DiscordWebhooks  []bool `json:"discord_webhooks"`
	EmailSMTP        bool   `json:"email_smtp"`
	EmailSMTPPerRow  []bool `json:"email_smtp_per_row"`
	WebhookURLs      []bool `json:"webhook_urls"`
	SyncToken        bool   `json:"sync_token"`
}

type PublicEmailTarget struct {
	To     string          `json:"to"`
	SMTP   *PublicSMTPConfig `json:"smtp,omitempty"`
	Policy *ReceiverPolicy `json:"policy,omitempty"`
}

type PublicSMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	From     string `json:"from"`
	TLS      string `json:"tls"`
}

type PublicEmailConfig struct {
	Enabled bool                `json:"enabled"`
	SMTP    PublicSMTPConfig    `json:"smtp"`
	Targets []PublicEmailTarget `json:"targets"`
}

type PublicWebhookReceiver struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Policy  *ReceiverPolicy   `json:"policy,omitempty"`
}

type PublicWebhookConfig struct {
	Enabled  bool                    `json:"enabled"`
	Webhooks []PublicWebhookReceiver `json:"webhooks"`
}

// PublicTelegramTarget is returned by GET /api/config (token omitted).
type PublicTelegramTarget struct {
	Token  string          `json:"token"`
	ChatID string          `json:"chat_id"`
	Policy *ReceiverPolicy `json:"policy,omitempty"`
}

// PublicDiscordReceiver is returned by GET /api/config (webhook omitted).
type PublicDiscordReceiver struct {
	Webhook string          `json:"webhook"`
	Policy  *ReceiverPolicy `json:"policy,omitempty"`
}

// PublicConfig is the redacted configuration returned to the web UI.
type PublicConfig struct {
	Listen                    string               `json:"listen"`
	Auth                      PublicAuthConfig     `json:"auth"`
	Notifications             NotificationsConfig  `json:"notifications"`
	Telegram                  PublicTelegramConfig `json:"telegram"`
	Discord                   PublicDiscordConfig  `json:"discord"`
	Email                     PublicEmailConfig    `json:"email"`
	Webhook                   PublicWebhookConfig  `json:"webhook"`
	Workers                   int                  `json:"workers"`
	DefaultInterval           int                  `json:"default_interval"`
	Network                   NetworkConfig        `json:"network"`
	Secrets                   SecretsPresent       `json:"secrets"`
	RequiresRestart           bool                 `json:"requires_restart"`
	RecommendedMinIntervalSec int                  `json:"recommended_min_interval_sec"`
}

type PublicAuthConfig struct {
	Username string `json:"username"`
}

type PublicTelegramConfig struct {
	Enabled bool                   `json:"enabled"`
	Targets []PublicTelegramTarget `json:"targets"`
}

type PublicDiscordConfig struct {
	Enabled  bool                    `json:"enabled"`
	Webhooks []PublicDiscordReceiver `json:"webhooks"`
}

// ToPublic returns API-safe config without secret values.
func (c *Config) ToPublic() PublicConfig {
	if c == nil {
		return PublicConfig{}
	}
	pub := PublicConfig{
		Listen:          c.Listen,
		Auth:            PublicAuthConfig{Username: c.Auth.Username},
		Notifications:   c.Notifications,
		Workers:         c.Workers,
		DefaultInterval: c.DefaultInterval,
		Network:         publicNetwork(c.Network),
		RequiresRestart: true,
		RecommendedMinIntervalSec: int(minIntervalProbeSeconds),
	}
	pub.Secrets.Password = c.Auth.PasswordHash != "" || c.Auth.Password != ""
	pub.Secrets.SyncToken = strings.TrimSpace(c.Network.SyncToken) != ""
	pub.Telegram.Enabled = c.Telegram.Enabled
	for _, t := range c.Telegram.Targets {
		pub.Secrets.TelegramTokens = append(pub.Secrets.TelegramTokens, t.Token != "")
		pub.Telegram.Targets = append(pub.Telegram.Targets, PublicTelegramTarget{
			Token:  MaskSecret(t.Token),
			ChatID: t.ChatID,
			Policy: t.Policy,
		})
	}
	pub.Discord.Enabled = c.Discord.Enabled
	for _, w := range c.Discord.Webhooks {
		pub.Secrets.DiscordWebhooks = append(pub.Secrets.DiscordWebhooks, w.Webhook != "")
		pub.Discord.Webhooks = append(pub.Discord.Webhooks, PublicDiscordReceiver{
			Webhook: MaskSecret(w.Webhook),
			Policy:  w.Policy,
		})
	}
	pub.Email.Enabled = c.Email.Enabled
	pub.Email.SMTP = publicSMTP(c.Email.SMTP)
	pub.Secrets.EmailSMTP = c.Email.SMTP.Password != "" || c.Email.SMTP.Username != ""
	for _, t := range c.Email.Targets {
		hasRowSMTP := t.SMTP != nil && strings.TrimSpace(t.SMTP.Password) != ""
		pub.Secrets.EmailSMTPPerRow = append(pub.Secrets.EmailSMTPPerRow, hasRowSMTP)
		row := PublicEmailTarget{To: t.To, Policy: t.Policy}
		if t.SMTP != nil && strings.TrimSpace(t.SMTP.Host) != "" {
			ps := publicSMTP(*t.SMTP)
			row.SMTP = &ps
		}
		pub.Email.Targets = append(pub.Email.Targets, row)
	}
	pub.Webhook.Enabled = c.Webhook.Enabled
	for _, w := range c.Webhook.Webhooks {
		pub.Secrets.WebhookURLs = append(pub.Secrets.WebhookURLs, w.URL != "")
		pub.Webhook.Webhooks = append(pub.Webhook.Webhooks, PublicWebhookReceiver{
			URL:     "",
			Headers: w.Headers,
			Policy:  w.Policy,
		})
	}
	return pub
}

// ToSettings returns full config for the authenticated settings UI (secrets included).
func (c *Config) ToSettings() PublicConfig {
	if c == nil {
		return PublicConfig{}
	}
	pub := PublicConfig{
		Listen:                    c.Listen,
		Auth:                      PublicAuthConfig{Username: c.Auth.Username},
		Notifications:             c.Notifications,
		Workers:                   c.Workers,
		DefaultInterval:           c.DefaultInterval,
		Network:                   c.Network,
		RecommendedMinIntervalSec: int(minIntervalProbeSeconds),
	}
	pub.Secrets.Password = c.Auth.PasswordHash != "" || c.Auth.Password != ""
	pub.Secrets.SyncToken = strings.TrimSpace(c.Network.SyncToken) != ""
	pub.Telegram.Enabled = c.Telegram.Enabled
	for _, t := range c.Telegram.Targets {
		pub.Secrets.TelegramTokens = append(pub.Secrets.TelegramTokens, t.Token != "")
		pub.Telegram.Targets = append(pub.Telegram.Targets, PublicTelegramTarget{
			Token:  t.Token,
			ChatID: t.ChatID,
			Policy: t.Policy,
		})
	}
	pub.Discord.Enabled = c.Discord.Enabled
	for _, w := range c.Discord.Webhooks {
		pub.Secrets.DiscordWebhooks = append(pub.Secrets.DiscordWebhooks, w.Webhook != "")
		pub.Discord.Webhooks = append(pub.Discord.Webhooks, PublicDiscordReceiver{
			Webhook: w.Webhook,
			Policy:  w.Policy,
		})
	}
	pub.Email.Enabled = c.Email.Enabled
	pub.Email.SMTP = PublicSMTPConfig{
		Host:     c.Email.SMTP.Host,
		Port:     c.Email.SMTP.Port,
		Username: c.Email.SMTP.Username,
		Password: c.Email.SMTP.Password,
		From:     c.Email.SMTP.From,
		TLS:      c.Email.SMTP.TLS,
	}
	pub.Secrets.EmailSMTP = c.Email.SMTP.Password != ""
	for _, t := range c.Email.Targets {
		hasRowSMTP := t.SMTP != nil && strings.TrimSpace(t.SMTP.Password) != ""
		pub.Secrets.EmailSMTPPerRow = append(pub.Secrets.EmailSMTPPerRow, hasRowSMTP)
		row := PublicEmailTarget{To: t.To, Policy: t.Policy}
		if t.SMTP != nil && strings.TrimSpace(t.SMTP.Host) != "" {
			s := SanitizeSMTPConfig(t.SMTP)
			row.SMTP = &PublicSMTPConfig{
				Host: s.Host, Port: s.Port, Username: s.Username,
				Password: s.Password, From: s.From, TLS: s.TLS,
			}
		}
		pub.Email.Targets = append(pub.Email.Targets, row)
	}
	pub.Webhook.Enabled = c.Webhook.Enabled
	for _, w := range c.Webhook.Webhooks {
		pub.Secrets.WebhookURLs = append(pub.Secrets.WebhookURLs, w.URL != "")
		pub.Webhook.Webhooks = append(pub.Webhook.Webhooks, PublicWebhookReceiver{
			URL:     w.URL,
			Headers: w.Headers,
			Policy:  w.Policy,
		})
	}
	return pub
}

const minIntervalProbeSeconds = 5

func publicNetwork(n NetworkConfig) NetworkConfig {
	out := n
	out.SyncToken = ""
	return out
}

func publicSMTP(s SMTPConfig) PublicSMTPConfig {
	s = SanitizeSMTPConfig(&s)
	return PublicSMTPConfig{
		Host:     s.Host,
		Port:     s.Port,
		Username: s.Username,
		From:     s.From,
		TLS:      s.TLS,
	}
}

// MergeSecrets applies patch semantics: empty secret fields keep existing values.
func MergeSecrets(existing, incoming *Config, sec Sections) error {
	if existing == nil || incoming == nil {
		return nil
	}
	if sec.Has("auth") {
		if pw := strings.TrimSpace(incoming.Auth.Password); pw != "" {
			if err := existing.Auth.SetPassword(pw); err != nil {
				return err
			}
		}
		existing.Auth.Username = incoming.Auth.Username
		if existing.Auth.Username == "" {
			existing.Auth.Username = "admin"
		}
	}

	if sec.Has("telegram") {
		existing.Telegram.Enabled = incoming.Telegram.Enabled
		existing.Telegram.Targets = mergeTelegramTargets(existing.Telegram.Targets, incoming.Telegram.Targets)
	}
	if sec.Has("discord") {
		existing.Discord.Enabled = incoming.Discord.Enabled
		existing.Discord.Webhooks = mergeDiscordWebhooks(existing.Discord.Webhooks, incoming.Discord.Webhooks)
	}
	if sec.Has("email") {
		existing.Email.Enabled = incoming.Email.Enabled
		existing.Email.SMTP = mergeSMTPConfig(existing.Email.SMTP, incoming.Email.SMTP)
		existing.Email.Targets = mergeEmailTargets(existing.Email.Targets, incoming.Email.Targets)
	}
	if sec.Has("webhook") {
		existing.Webhook.Enabled = incoming.Webhook.Enabled
		existing.Webhook.Webhooks = mergeWebhookReceivers(existing.Webhook.Webhooks, incoming.Webhook.Webhooks)
	}
	if sec.Has("network") {
		if tok := strings.TrimSpace(incoming.Network.SyncToken); tok != "" {
			existing.Network.SyncToken = tok
		}
	}
	return nil
}

func mergeSMTPConfig(existing, incoming SMTPConfig) SMTPConfig {
	out := SanitizeSMTPConfig(&incoming)
	if out.Host == "" {
		out.Host = existing.Host
	}
	if out.Port <= 0 {
		out.Port = existing.Port
	}
	if out.Username == "" {
		out.Username = existing.Username
	}
	if out.Password == "" {
		out.Password = existing.Password
	}
	if out.From == "" {
		out.From = existing.From
	}
	if strings.TrimSpace(incoming.TLS) == "" {
		out.TLS = existing.TLS
	}
	return SanitizeSMTPConfig(&out)
}

func mergeEmailTargets(existing, incoming []EmailTarget) []EmailTarget {
	byTo := make(map[string]EmailTarget, len(existing))
	for _, t := range existing {
		byTo[t.To] = t
	}
	out := make([]EmailTarget, 0, len(incoming))
	for _, t := range incoming {
		to := strings.TrimSpace(t.To)
		if to == "" {
			continue
		}
		prev := byTo[to]
		smtp := t.SMTP
		if smtp != nil {
			merged := mergeSMTPConfig(prevSMTPOrZero(prev), *smtp)
			if merged.Host != "" {
				smtp = &merged
			} else {
				smtp = prev.SMTP
			}
		} else {
			smtp = prev.SMTP
		}
		out = append(out, EmailTarget{
			To:     to,
			SMTP:   smtp,
			Policy: SanitizeReceiverPolicy(t.Policy),
		})
		if len(out) >= MaxReceivers {
			break
		}
	}
	return out
}

func prevSMTPOrZero(t EmailTarget) SMTPConfig {
	if t.SMTP != nil {
		return *t.SMTP
	}
	return SMTPConfig{}
}

func mergeWebhookReceivers(existing, incoming []WebhookReceiver) []WebhookReceiver {
	byURL := make(map[string]WebhookReceiver, len(existing))
	ordered := make([]string, 0, len(existing))
	for _, w := range existing {
		if w.URL != "" {
			byURL[w.URL] = w
			ordered = append(ordered, w.URL)
		}
	}
	out := make([]WebhookReceiver, 0, len(incoming))
	for i, w := range incoming {
		url := strings.TrimSpace(w.URL)
		if url == "" && i < len(ordered) {
			url = ordered[i]
		}
		if url == "" {
			continue
		}
		prev := byURL[url]
		headers := w.Headers
		if len(headers) == 0 && len(prev.Headers) > 0 {
			headers = prev.Headers
		}
		out = append(out, WebhookReceiver{
			URL:     url,
			Headers: headers,
			Policy:  SanitizeReceiverPolicy(w.Policy),
		})
		if len(out) >= MaxReceivers {
			break
		}
	}
	return out
}

func mergeTelegramTargets(existing, incoming []TelegramTarget) []TelegramTarget {
	byChat := make(map[string]TelegramTarget, len(existing))
	for _, t := range existing {
		byChat[t.ChatID] = t
	}
	out := make([]TelegramTarget, 0, len(incoming))
	for _, t := range incoming {
		token := strings.TrimSpace(t.Token)
		chat := strings.TrimSpace(t.ChatID)
		if chat != "" {
			if prev, ok := byChat[chat]; ok {
				if token == "" || SecretUnchanged(token, prev.Token) {
					token = prev.Token
				}
			}
		}
		if token == "" || chat == "" {
			continue
		}
		out = append(out, TelegramTarget{
			Token:  token,
			ChatID: chat,
			Policy: SanitizeReceiverPolicy(t.Policy),
		})
		if len(out) >= MaxReceivers {
			break
		}
	}
	return out
}

func mergeDiscordWebhooks(existing, incoming []DiscordReceiver) []DiscordReceiver {
	ordered := make([]string, 0, len(existing))
	for _, w := range existing {
		if w.Webhook != "" {
			ordered = append(ordered, w.Webhook)
		}
	}
	out := make([]DiscordReceiver, 0, len(incoming))
	for i, w := range incoming {
		webhook := strings.TrimSpace(w.Webhook)
		var stored string
		if i < len(ordered) {
			stored = ordered[i]
		}
		if webhook == "" && stored != "" {
			webhook = stored
		} else if webhook != "" && stored != "" && SecretUnchanged(webhook, stored) {
			webhook = stored
		}
		if webhook == "" {
			continue
		}
		out = append(out, DiscordReceiver{
			Webhook: webhook,
			Policy:  SanitizeReceiverPolicy(w.Policy),
		})
		if len(out) >= MaxReceivers {
			break
		}
	}
	return out
}

// ApplyNonSecret copies non-secret settings from incoming onto existing,
// touching only the sections the patch mentioned.
func ApplyNonSecret(existing, incoming *Config, sec Sections) {
	if sec.Has("listen") {
		existing.Listen = incoming.Listen
	}
	if sec.Has("workers") {
		existing.Workers = incoming.Workers
	}
	if sec.Has("default_interval") {
		existing.DefaultInterval = incoming.DefaultInterval
	}
	if sec.Has("notifications") {
		mergeNotifications(&existing.Notifications, incoming.Notifications)
	}
	if sec.Has("network") {
		existing.Network = mergeNetworkConfig(existing.Network, incoming.Network)
	}
}

func mergeNotifications(existing *NotificationsConfig, incoming NotificationsConfig) {
	if incoming.AlertMode != "" {
		existing.AlertMode = incoming.AlertMode
	}
	if incoming.Templates.Down != "" {
		existing.Templates.Down = incoming.Templates.Down
	}
	if incoming.Templates.Recovered != "" {
		existing.Templates.Recovered = incoming.Templates.Recovered
	}
}

func mergeNetworkConfig(existing, incoming NetworkConfig) NetworkConfig {
	out := incoming
	if existing.NodeID != "" {
		out.NodeID = existing.NodeID
	} else if out.NodeID == "" {
		out.NodeID = existing.NodeID
	}
	out.SyncToken = existing.SyncToken
	return out
}
