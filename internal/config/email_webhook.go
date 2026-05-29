package config

import "strings"

// SMTPConfig holds outbound mail server settings.
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	From     string `json:"from"`
	TLS      string `json:"tls,omitempty"` // starttls | ssl | plain
}

// EmailTarget is a single email destination.
type EmailTarget struct {
	To     string          `json:"to"`
	SMTP   *SMTPConfig     `json:"smtp,omitempty"`
	Policy *ReceiverPolicy `json:"policy,omitempty"`
}

// EmailConfig holds global email notification settings.
type EmailConfig struct {
	Enabled bool          `json:"enabled"`
	SMTP    SMTPConfig    `json:"smtp"`
	Targets []EmailTarget `json:"targets"`
}

// WebhookReceiver is a generic HTTP webhook destination (not Discord).
type WebhookReceiver struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Policy  *ReceiverPolicy   `json:"policy,omitempty"`
}

// WebhookConfig holds global generic webhook settings.
type WebhookConfig struct {
	Enabled  bool              `json:"enabled"`
	Webhooks []WebhookReceiver `json:"webhooks"`
}

// SanitizeEmailTargets trims and caps email targets.
func SanitizeEmailTargets(in []EmailTarget) []EmailTarget {
	if len(in) == 0 {
		return nil
	}
	out := make([]EmailTarget, 0, len(in))
	for _, t := range in {
		to := strings.TrimSpace(t.To)
		if to == "" {
			continue
		}
		et := EmailTarget{To: to, Policy: SanitizeReceiverPolicy(t.Policy)}
		if t.SMTP != nil {
			s := SanitizeSMTPConfig(t.SMTP)
			if s.Host != "" {
				et.SMTP = &s
			}
		}
		out = append(out, et)
		if len(out) >= MaxReceivers {
			break
		}
	}
	return out
}

// SanitizeWebhookReceivers trims and caps webhook receivers.
func SanitizeWebhookReceivers(in []WebhookReceiver) []WebhookReceiver {
	if len(in) == 0 {
		return nil
	}
	out := make([]WebhookReceiver, 0, len(in))
	for _, w := range in {
		url := strings.TrimSpace(w.URL)
		if url == "" {
			continue
		}
		headers := make(map[string]string, len(w.Headers))
		for k, v := range w.Headers {
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			if k != "" && v != "" {
				headers[k] = v
			}
		}
		if len(headers) > 5 {
			trimmed := make(map[string]string, 5)
			n := 0
			for k, v := range headers {
				trimmed[k] = v
				n++
				if n >= 5 {
					break
				}
			}
			headers = trimmed
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

func SanitizeSMTPConfig(in *SMTPConfig) SMTPConfig {
	if in == nil {
		return SMTPConfig{}
	}
	out := SMTPConfig{
		Host:     strings.TrimSpace(in.Host),
		Port:     in.Port,
		Username: strings.TrimSpace(in.Username),
		Password: strings.TrimSpace(in.Password),
		From:     strings.TrimSpace(in.From),
		TLS:      strings.ToLower(strings.TrimSpace(in.TLS)),
	}
	if out.Port <= 0 {
		out.Port = 587
	}
	if out.TLS == "" {
		out.TLS = "starttls"
	}
	return out
}

// EffectiveSMTP returns per-target SMTP or global default.
func (c *Config) EffectiveSMTP(target EmailTarget) SMTPConfig {
	if target.SMTP != nil && strings.TrimSpace(target.SMTP.Host) != "" {
		return SanitizeSMTPConfig(target.SMTP)
	}
	if c != nil {
		return SanitizeSMTPConfig(&c.Email.SMTP)
	}
	return SMTPConfig{}
}
