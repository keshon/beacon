package notify

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
)

const (
	AlertModeRepeat = "repeat"
	AlertModeOnce   = "once"
)

// MaxTemplateLen caps custom template size.
const MaxTemplateLen = 2000

// ResolvedPolicy is the effective notification policy for one receiver.
type ResolvedPolicy struct {
	AlertMode string
	Templates config.MessageTemplates
}

// DefaultTemplates returns built-in down/recovered templates.
func DefaultTemplates() config.MessageTemplates {
	return config.DefaultMessageTemplates()
}

// DefaultAlertMode is used when nothing is configured.
func DefaultAlertMode() string {
	return AlertModeRepeat
}

// TemplateForStatus picks the template string for an alert status.
func (p ResolvedPolicy) TemplateForStatus(status string) string {
	switch status {
	case "recovered":
		return p.Templates.Recovered
	default:
		return p.Templates.Down
	}
}

// ResolveReceiverPolicy merges row policy with global notifications defaults.
// Row non-empty fields win; then global; then built-in defaults.
func ResolveReceiverPolicy(cfg *config.Config, row *config.ReceiverPolicy) ResolvedPolicy {
	def := DefaultTemplates()
	globalMode := ""
	globalTpl := config.MessageTemplates{}
	if cfg != nil {
		globalMode = cfg.Notifications.AlertMode
		globalTpl = cfg.Notifications.Templates
	}
	rowMode := ""
	rowTpl := config.MessageTemplates{}
	if row != nil {
		rowMode = row.AlertMode
		if row.Templates != nil {
			rowTpl = SanitizeTemplates(row.Templates)
		}
	}
	return ResolvedPolicy{
		AlertMode: mergeAlertMode(rowMode, globalMode, DefaultAlertMode()),
		Templates: mergeTemplates(def, globalTpl, rowTpl),
	}
}

// IsCustomTemplates reports whether effective templates differ from built-in defaults.
func IsCustomTemplates(p ResolvedPolicy) bool {
	def := DefaultTemplates()
	return p.Templates.Down != def.Down || p.Templates.Recovered != def.Recovered
}

func mergeAlertMode(rowMode, globalMode, fallback string) string {
	if s := strings.TrimSpace(rowMode); s != "" {
		if normalized := normalizeAlertMode(s); normalized != "" {
			return normalized
		}
	}
	if s := strings.TrimSpace(globalMode); s != "" {
		if normalized := normalizeAlertMode(s); normalized != "" {
			return normalized
		}
	}
	return fallback
}

func normalizeAlertMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case AlertModeRepeat, AlertModeOnce:
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return ""
	}
}

// mergeTemplates: for each field, prefer b (higher priority) if non-empty, else a, else def.
func mergeTemplates(def, a, b config.MessageTemplates) config.MessageTemplates {
	return config.MessageTemplates{
		Down:      pickTemplate(def.Down, a.Down, b.Down),
		Recovered: pickTemplate(def.Recovered, a.Recovered, b.Recovered),
	}
}

func pickTemplate(def, a, b string) string {
	if s := strings.TrimSpace(b); s != "" {
		return capTemplate(s)
	}
	if s := strings.TrimSpace(a); s != "" {
		return capTemplate(s)
	}
	return def
}

func capTemplate(s string) string {
	if len(s) > MaxTemplateLen {
		return s[:MaxTemplateLen]
	}
	return s
}

// SanitizeTemplates trims and caps template fields.
func SanitizeTemplates(t *config.MessageTemplates) config.MessageTemplates {
	if t == nil {
		return config.MessageTemplates{}
	}
	return config.MessageTemplates{
		Down:      capTemplate(strings.TrimSpace(t.Down)),
		Recovered: capTemplate(strings.TrimSpace(t.Recovered)),
	}
}

// PlaceholderInfo describes a template variable for the UI.
type PlaceholderInfo struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

// Placeholders returns the supported {{key}} list.
func Placeholders() []PlaceholderInfo {
	return []PlaceholderInfo{
		{Key: "name", Description: "Monitor name"},
		{Key: "target", Description: "URL or host:port"},
		{Key: "type", Description: "Check type (http, tcp)"},
		{Key: "status", Description: "Alert status (down, recovered, test)"},
		{Key: "error", Description: "Check error text"},
		{Key: "latency", Description: "Response latency"},
		{Key: "status_code", Description: "HTTP status code (0 if N/A)"},
		{Key: "time", Description: "Event time"},
		{Key: "message", Description: "Detail line (error or latency summary)"},
		{Key: "fail_count", Description: "Failed check count before down"},
	}
}

// ShouldSendDown returns whether a down alert should be delivered.
// Email always uses once semantics regardless of policy alert_mode.
func ShouldSendDown(policy ResolvedPolicy, isRepeat bool, channel string) bool {
	if channel == ChannelEmail {
		return !isRepeat
	}
	if !isRepeat {
		return true
	}
	return policy.AlertMode == AlertModeRepeat
}

// AlertDedup suppresses duplicate alerts for the same monitor status within a window.
type AlertDedup struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func NewAlertDedup() *AlertDedup {
	return &AlertDedup{last: make(map[string]time.Time)}
}

func alertDedupKey(monitorID, status string) string {
	return monitorID + "\x00" + status
}

// Allow reports whether an alert should be sent (false if duplicate within window).
func (d *AlertDedup) Allow(monitorID, status string, window time.Duration) bool {
	key := alertDedupKey(monitorID, status)
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if prev, ok := d.last[key]; ok && now.Sub(prev) < window {
		return false
	}
	d.last[key] = now
	return true
}

const (
	minIntervalTelegram = 20 * time.Second
	minIntervalDiscord  = 60 * time.Second
	minIntervalWebhook  = 30 * time.Second
	minIntervalProbe    = 5 * time.Second
)

// RecommendedMinInterval returns a conservative minimum check interval for a monitor.
// Email is excluded (once-only delivery with separate send guard).
func RecommendedMinInterval(cfg *config.Config, m *monitor.Monitor) time.Duration {
	recvs := BuildReceivers(cfg, m)
	if len(recvs) == 0 {
		return minIntervalProbe
	}
	var max time.Duration
	hasRepeat := false
	for _, r := range recvs {
		if r.Channel == ChannelEmail {
			continue
		}
		floor := channelIntervalFloor(r.Channel)
		if floor > max {
			max = floor
		}
		if r.Policy.AlertMode == AlertModeRepeat {
			hasRepeat = true
		}
	}
	if !hasRepeat {
		return minIntervalProbe
	}
	if max == 0 {
		return minIntervalProbe
	}
	return max
}

func channelIntervalFloor(channel string) time.Duration {
	switch channel {
	case ChannelTelegram:
		return minIntervalTelegram
	case ChannelDiscord:
		return minIntervalDiscord
	case ChannelWebhook:
		return minIntervalWebhook
	default:
		return 0
	}
}

// IntervalWarnings returns human-readable warnings when monitor interval is below recommended.
func IntervalWarnings(cfg *config.Config, m *monitor.Monitor) []string {
	if m == nil {
		return nil
	}
	rec := RecommendedMinInterval(cfg, m)
	if m.Interval <= 0 {
		return nil
	}
	if m.Interval >= rec {
		return nil
	}
	return []string{
		"check interval " + m.Interval.String() + " is below recommended minimum " + rec.String() + " for repeat-mode notification channels",
	}
}

// EmailSendGuard limits production email frequency per destination.
type EmailSendGuard struct {
	last map[string]time.Time
}

func NewEmailSendGuard() *EmailSendGuard {
	return &EmailSendGuard{last: make(map[string]time.Time)}
}

const emailSafeInterval = 60 * time.Second

func (g *EmailSendGuard) Allow(monitorID, recipient string) bool {
	key := monitorID + "\x00" + recipient
	now := time.Now()
	if prev, ok := g.last[key]; ok && now.Sub(prev) < emailSafeInterval {
		return false
	}
	return true
}

func (g *EmailSendGuard) RecordSuccess(monitorID, recipient string) {
	key := monitorID + "\x00" + recipient
	g.last[key] = time.Now()
}

// Default cooldowns are conservative compared to provider limits so a user
// mashing the Test button cannot get a real bot banned.
const (
	telegramTestCooldown = 3 * time.Second
	discordTestCooldown  = 5 * time.Second
	emailTestCooldown    = 10 * time.Second
	webhookTestCooldown  = 5 * time.Second
	clientTestWindow     = time.Minute
	clientTestBudget     = 10
)

// RateLimiter is an in-memory limiter for outbound test notifications. It
// enforces:
//   - a per-destination cooldown so the same chat/webhook cannot be hammered
//   - a per-client burst budget keyed by an opaque client id (e.g. IP)
//
// It is safe for concurrent use.
type RateLimiter struct {
	mu      sync.Mutex
	destAt  map[string]time.Time
	clients map[string][]time.Time
}

// NewRateLimiter constructs an empty limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		destAt:  make(map[string]time.Time),
		clients: make(map[string][]time.Time),
	}
}

// AllowTelegram reserves a slot for sending a test message to the given
// Telegram destination from clientID. retryAfter is the wait time until the
// next attempt is allowed when ok is false.
func (r *RateLimiter) AllowTelegram(clientID, token, chatID string) (ok bool, retryAfter time.Duration) {
	return r.allow(clientID, "tg:"+hashKey(token+"|"+chatID), telegramTestCooldown)
}

// AllowDiscord reserves a slot for sending a test message to the given Discord
// webhook from clientID.
func (r *RateLimiter) AllowDiscord(clientID, webhook string) (ok bool, retryAfter time.Duration) {
	return r.allow(clientID, "dc:"+hashKey(webhook), discordTestCooldown)
}

func (r *RateLimiter) AllowEmail(clientID, to string) (ok bool, retryAfter time.Duration) {
	return r.allow(clientID, "em:"+hashKey(strings.ToLower(strings.TrimSpace(to))), emailTestCooldown)
}

func (r *RateLimiter) AllowWebhook(clientID, url string) (ok bool, retryAfter time.Duration) {
	return r.allow(clientID, "wh:"+hashKey(url), webhookTestCooldown)
}

func (r *RateLimiter) allow(clientID, destKey string, cooldown time.Duration) (bool, time.Duration) {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()

	if last, ok := r.destAt[destKey]; ok {
		if wait := cooldown - now.Sub(last); wait > 0 {
			return false, wait
		}
	}

	if clientID != "" {
		hits := r.clients[clientID]
		cutoff := now.Add(-clientTestWindow)
		fresh := hits[:0]
		for _, t := range hits {
			if t.After(cutoff) {
				fresh = append(fresh, t)
			}
		}
		if len(fresh) >= clientTestBudget {
			wait := clientTestWindow - now.Sub(fresh[0])
			if wait < 0 {
				wait = 0
			}
			r.clients[clientID] = fresh
			return false, wait
		}
		r.clients[clientID] = append(fresh, now)
	}

	r.destAt[destKey] = now
	return true, 0
}

// RetryAfterSeconds rounds wait up to the next whole second for HTTP response
// bodies.
func RetryAfterSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int(math.Ceil(d.Seconds()))
}

func hashKey(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}
