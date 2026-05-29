package notify

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"sync"
	"time"
)

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
