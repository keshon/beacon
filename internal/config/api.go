package config

import "strings"

// SecretsPresent indicates which secret fields are stored server-side.
type SecretsPresent struct {
	Password        bool `json:"password"`
	TelegramTokens  []bool `json:"telegram_tokens"`
	DiscordWebhooks []bool `json:"discord_webhooks"`
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
	Listen          string              `json:"listen"`
	Auth            PublicAuthConfig    `json:"auth"`
	Notifications   NotificationsConfig `json:"notifications"`
	Telegram        PublicTelegramConfig `json:"telegram"`
	Discord         PublicDiscordConfig  `json:"discord"`
	Workers         int                 `json:"workers"`
	DefaultInterval int                 `json:"default_interval"`
	Network         NetworkConfig       `json:"network"`
	Secrets         SecretsPresent      `json:"secrets"`
	RequiresRestart bool                `json:"requires_restart"`
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
		Network:         c.Network,
		RequiresRestart: true,
	}
	pub.Secrets.Password = c.Auth.PasswordHash != "" || c.Auth.Password != ""
	pub.Telegram.Enabled = c.Telegram.Enabled
	for _, t := range c.Telegram.Targets {
		pub.Secrets.TelegramTokens = append(pub.Secrets.TelegramTokens, t.Token != "")
		pub.Telegram.Targets = append(pub.Telegram.Targets, PublicTelegramTarget{
			Token:  "",
			ChatID: t.ChatID,
			Policy: t.Policy,
		})
	}
	pub.Discord.Enabled = c.Discord.Enabled
	for _, w := range c.Discord.Webhooks {
		pub.Secrets.DiscordWebhooks = append(pub.Secrets.DiscordWebhooks, w.Webhook != "")
		pub.Discord.Webhooks = append(pub.Discord.Webhooks, PublicDiscordReceiver{
			Webhook: "",
			Policy:  w.Policy,
		})
	}
	return pub
}

// MergeSecrets applies patch semantics: empty secret fields keep existing values.
func MergeSecrets(existing, incoming *Config) error {
	if existing == nil || incoming == nil {
		return nil
	}
	if pw := strings.TrimSpace(incoming.Auth.Password); pw != "" {
		if err := existing.Auth.SetPassword(pw); err != nil {
			return err
		}
	}
	existing.Auth.Username = incoming.Auth.Username
	if existing.Auth.Username == "" {
		existing.Auth.Username = "admin"
	}

	existing.Telegram.Enabled = incoming.Telegram.Enabled
	existing.Discord.Enabled = incoming.Discord.Enabled
	existing.Telegram.Targets = mergeTelegramTargets(existing.Telegram.Targets, incoming.Telegram.Targets)
	existing.Discord.Webhooks = mergeDiscordWebhooks(existing.Discord.Webhooks, incoming.Discord.Webhooks)
	return nil
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
		if token == "" && chat != "" {
			if prev, ok := byChat[chat]; ok {
				token = prev.Token
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
	out := make([]DiscordReceiver, 0, len(incoming))
	for i, w := range incoming {
		webhook := strings.TrimSpace(w.Webhook)
		if webhook == "" && i < len(existing) {
			webhook = existing[i].Webhook
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

// ApplyNonSecret copies non-secret settings from incoming onto existing.
func ApplyNonSecret(existing, incoming *Config) {
	existing.Listen = incoming.Listen
	existing.Workers = incoming.Workers
	existing.DefaultInterval = incoming.DefaultInterval
	existing.Notifications = incoming.Notifications
	existing.Network = incoming.Network
}
