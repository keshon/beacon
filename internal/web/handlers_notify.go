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

// apiNotifyTest sends a mock alert to the credentials supplied in the request
// body so users can verify Telegram/Discord delivery before saving the form.
func (s *Server) apiNotifyTest(w http.ResponseWriter, r *http.Request) {
	var req notifyTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "invalid JSON"})
		return
	}

	clientID := clientIP(r)
	alert := notify.TestAlert()

	switch strings.ToLower(strings.TrimSpace(req.Channel)) {
	case "telegram":
		if req.Telegram == nil {
			writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "telegram payload required"})
			return
		}
		token := strings.TrimSpace(req.Telegram.Token)
		chat := strings.TrimSpace(req.Telegram.ChatID)
		if token == "" || chat == "" {
			writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "token and chat_id are required"})
			return
		}
		if ok, wait := s.testLimit.AllowTelegram(clientID, token, chat); !ok {
			writeNotifyTest(w, http.StatusTooManyRequests, notifyTestResponse{
				Error:         "rate limited",
				RetryAfterSec: notify.RetryAfterSeconds(wait),
			})
			return
		}
		if err := notify.NewTelegram(token, chat).Send(alert); err != nil {
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
		if webhook == "" {
			writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "webhook is required"})
			return
		}
		if ok, wait := s.testLimit.AllowDiscord(clientID, webhook); !ok {
			writeNotifyTest(w, http.StatusTooManyRequests, notifyTestResponse{
				Error:         "rate limited",
				RetryAfterSec: notify.RetryAfterSeconds(wait),
			})
			return
		}
		if err := notify.NewDiscord(webhook).Send(alert); err != nil {
			writeNotifyTest(w, http.StatusBadGateway, notifyTestResponse{Error: err.Error()})
			return
		}
		writeNotifyTest(w, http.StatusOK, notifyTestResponse{OK: true})

	default:
		writeNotifyTest(w, http.StatusBadRequest, notifyTestResponse{Error: "channel must be telegram or discord"})
	}
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
