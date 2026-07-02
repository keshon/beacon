package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/notify"
	"github.com/keshon/beacon/internal/server/httpx"
)

type Notify struct {
	Cfg       *config.Config
	TestLimit *notify.RateLimiter
}

type notifyTestRequest struct {
	Channel  string `json:"channel"`
	Status   string `json:"status"`
	Template string `json:"template"`
	Telegram *struct {
		Token  string `json:"token"`
		ChatID string `json:"chat_id"`
	} `json:"telegram,omitempty"`
	Discord *struct {
		Webhook string `json:"webhook"`
	} `json:"discord,omitempty"`
	Email *struct {
		To string `json:"to"`
	} `json:"email,omitempty"`
	Webhook *struct {
		URL string `json:"url"`
	} `json:"webhook,omitempty"`
}

type notifyTestResponse struct {
	OK            bool   `json:"ok"`
	Error         string `json:"error,omitempty"`
	RetryAfterSec int    `json:"retry_after_sec,omitempty"`
}

func (h *Notify) Test(w http.ResponseWriter, r *http.Request) {
	var req notifyTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "invalid JSON"})
		return
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "down" && status != "recovered" {
		writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "status must be down or recovered"})
		return
	}
	tpl := strings.TrimSpace(req.Template)
	if tpl == "" {
		writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "template is required"})
		return
	}
	if len(tpl) > notify.MaxTemplateLen {
		tpl = tpl[:notify.MaxTemplateLen]
	}

	clientID := clientIP(r)
	ctx := notify.WithNodeFromConfig(notify.PreviewTemplateContext(status), h.Cfg)
	body := notify.RenderTemplate(tpl, ctx)
	alert := notify.Alert{
		MonitorName: ctx.MonitorName,
		Status:      status,
		Message:     ctx.Message,
		Body:        body,
		Time:        ctx.Time,
		Target:      ctx.Target,
		Type:        ctx.Type,
		StatusCode:  ctx.StatusCode,
		Latency:     ctx.Latency,
		FailCount:   ctx.FailCount,
	}

	switch strings.ToLower(strings.TrimSpace(req.Channel)) {
	case "telegram":
		if req.Telegram == nil {
			writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "telegram payload required"})
			return
		}
		token := strings.TrimSpace(req.Telegram.Token)
		chat := strings.TrimSpace(req.Telegram.ChatID)
		if chat == "" {
			writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "chat_id is required"})
			return
		}
		allowedToken, allowedChat, ok := h.Cfg.ResolveTelegramTestCredentials(token, chat)
		if !ok {
			if token == "" {
				writeNotifyTest(w, http.StatusForbidden, notifyTestResponse{Error: "credentials are not configured"})
				return
			}
			allowedToken, allowedChat = token, chat
		}
		if allowed, wait := h.TestLimit.AllowTelegram(clientID, allowedToken, allowedChat); !allowed {
			writeNotifyTest(w, http.StatusTooManyRequests, notifyTestResponse{
				Error:         "rate limited",
				RetryAfterSec: notify.RetryAfterSeconds(wait),
			})
			return
		}
		if err := notify.NewTelegram(allowedToken, allowedChat).Send(alert); err != nil {
			writeNotifyTest(w, http.StatusBadGateway, notifyTestResponse{Error: err.Error()})
			return
		}
		writeNotifyTest(w, http.StatusOK, notifyTestResponse{OK: true})

	case "discord":
		if req.Discord == nil {
			writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "discord payload required"})
			return
		}
		webhook := strings.TrimSpace(req.Discord.Webhook)
		allowedWebhook, ok := h.Cfg.ResolveDiscordTestWebhook(webhook)
		if !ok {
			if err := notify.ValidateWebhookURL(webhook); err != nil {
				writeNotifyTest(w, http.StatusForbidden, notifyTestResponse{Error: "webhook is not configured"})
				return
			}
			allowedWebhook = webhook
		}
		if allowed, wait := h.TestLimit.AllowDiscord(clientID, allowedWebhook); !allowed {
			writeNotifyTest(w, http.StatusTooManyRequests, notifyTestResponse{
				Error:         "rate limited",
				RetryAfterSec: notify.RetryAfterSeconds(wait),
			})
			return
		}
		if err := notify.NewDiscord(allowedWebhook).Send(alert); err != nil {
			writeNotifyTest(w, http.StatusBadGateway, notifyTestResponse{Error: err.Error()})
			return
		}
		writeNotifyTest(w, http.StatusOK, notifyTestResponse{OK: true})

	case "email":
		if req.Email == nil {
			writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "email payload required"})
			return
		}
		to := strings.TrimSpace(req.Email.To)
		if to == "" {
			writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "to is required"})
			return
		}
		target, ok := h.Cfg.ResolveEmailTestTarget(to)
		if !ok {
			if to == "" || !strings.Contains(to, "@") {
				writeNotifyTest(w, http.StatusForbidden, notifyTestResponse{Error: "email is not configured"})
				return
			}
			if !h.Cfg.Email.Enabled || strings.TrimSpace(h.Cfg.Email.SMTP.Host) == "" {
				writeNotifyTest(w, http.StatusForbidden, notifyTestResponse{Error: "email is not configured"})
				return
			}
			target = config.EmailTarget{To: to}
		}
		if allowed, wait := h.TestLimit.AllowEmail(clientID, to); !allowed {
			writeNotifyTest(w, http.StatusTooManyRequests, notifyTestResponse{
				Error:         "rate limited",
				RetryAfterSec: notify.RetryAfterSeconds(wait),
			})
			return
		}
		smtp := h.Cfg.EffectiveSMTP(target)
		if err := notify.NewEmail(smtp, target.To).Send(alert); err != nil {
			writeNotifyTest(w, http.StatusBadGateway, notifyTestResponse{Error: err.Error()})
			return
		}
		writeNotifyTest(w, http.StatusOK, notifyTestResponse{OK: true})

	case "webhook":
		if req.Webhook == nil {
			writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "webhook payload required"})
			return
		}
		rawURL := strings.TrimSpace(req.Webhook.URL)
		allowedURL, ok := h.Cfg.ResolveWebhookTestURL(rawURL)
		if !ok {
			if err := notify.ValidateWebhookURL(rawURL); err != nil {
				writeNotifyTest(w, http.StatusForbidden, notifyTestResponse{Error: "webhook is not configured"})
				return
			}
			allowedURL = rawURL
		}
		if allowed, wait := h.TestLimit.AllowWebhook(clientID, allowedURL); !allowed {
			writeNotifyTest(w, http.StatusTooManyRequests, notifyTestResponse{
				Error:         "rate limited",
				RetryAfterSec: notify.RetryAfterSeconds(wait),
			})
			return
		}
		var headers map[string]string
		for _, w := range h.Cfg.Webhook.Webhooks {
			if strings.TrimSpace(w.URL) == allowedURL {
				headers = w.Headers
				break
			}
		}
		if err := notify.NewWebhook(allowedURL, headers).Send(alert); err != nil {
			writeNotifyTest(w, http.StatusBadGateway, notifyTestResponse{Error: err.Error()})
			return
		}
		writeNotifyTest(w, http.StatusOK, notifyTestResponse{OK: true})

	default:
		writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "channel must be telegram, discord, email, or webhook"})
	}
}

func (h *Notify) Defaults(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, map[string]any{
		"alert_mode":   notify.DefaultAlertMode(),
		"templates":    notify.DefaultTemplates(),
		"placeholders": notify.Placeholders(),
	})
}

func writeNotifyTest(w http.ResponseWriter, status int, body notifyTestResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		if i := strings.IndexByte(xf, ','); i >= 0 {
			xf = xf[:i]
		}
		return strings.TrimSpace(xf)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
