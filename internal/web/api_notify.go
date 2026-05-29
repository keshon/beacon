package web

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/keshon/beacon/internal/notify"
)

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
}

type notifyTestResponse struct {
	OK            bool   `json:"ok"`
	Error         string `json:"error,omitempty"`
	RetryAfterSec int    `json:"retry_after_sec,omitempty"`
}

// apiNotifyTest renders the supplied template with preview placeholders and
// sends it to the credentials in the request (used from receiver policy modal).
func (s *Server) apiNotifyTest(w http.ResponseWriter, r *http.Request) {
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
	ctx := notify.PreviewTemplateContext(status)
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
		allowedToken, allowedChat, ok := s.cfg.ResolveTelegramTestCredentials(token, chat)
		if !ok {
			writeNotifyTest(w, http.StatusForbidden, notifyTestResponse{Error: "credentials are not configured"})
			return
		}
		if allowed, wait := s.testLimit.AllowTelegram(clientID, allowedToken, allowedChat); !allowed {
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
		allowedWebhook, ok := s.cfg.ResolveDiscordTestWebhook(webhook)
		if !ok {
			writeNotifyTest(w, http.StatusForbidden, notifyTestResponse{Error: "webhook is not configured"})
			return
		}
		if allowed, wait := s.testLimit.AllowDiscord(clientID, allowedWebhook); !allowed {
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

	default:
		writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "channel must be telegram or discord"})
	}
}

func (s *Server) apiNotifyDefaults(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, map[string]any{
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

// clientIP returns a best-effort identifier for per-client rate limiting. It
// prefers X-Forwarded-For (first hop) and falls back to RemoteAddr.
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

