package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
	"github.com/jtprogru/jtpost/internal/logger"
	"golang.org/x/time/rate"
)

// RateLimitConfig конфигурация rate-limiting middleware.
type RateLimitConfig struct {
	// Enabled включает middleware. При false возвращается no-op handler.
	Enabled bool
	// RequestsPerMinute — sustained rate (token replenishment per minute).
	RequestsPerMinute int
	// Burst — максимальный пиковый объём в bucket (token capacity).
	Burst int
	// TrustProxyHeader — если true, источник берётся из X-Forwarded-For/X-Real-IP.
	// Включать ТОЛЬКО когда сервер за trusted reverse proxy.
	TrustProxyHeader bool
	// CleanupInterval — как часто GC-удаляются неактивные limiter'ы из памяти.
	// 0 → 5*time.Minute.
	CleanupInterval time.Duration
	// IdleTTL — limiter, не использованный дольше этого, удаляется GC.
	// 0 → 10*time.Minute.
	IdleTTL time.Duration
}

// rateLimitEntry — limiter с timestamp последнего hit'а для GC.
type rateLimitEntry struct {
	limiter *rate.Limiter
	lastHit time.Time
}

// rateLimiter — keyed-bucket limiter с фоновым GC stale entries.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rateLimitEntry
	rps     rate.Limit
	burst   int
	idleTTL time.Duration
	clock   func() time.Time
}

func newRateLimiter(rpm, burst int, idleTTL, cleanup time.Duration, stop <-chan struct{}) *rateLimiter {
	if rpm <= 0 {
		rpm = 60
	}
	if burst <= 0 {
		burst = rpm
	}
	if idleTTL <= 0 {
		idleTTL = 10 * time.Minute
	}
	if cleanup <= 0 {
		cleanup = 5 * time.Minute
	}
	rl := &rateLimiter{
		buckets: make(map[string]*rateLimitEntry),
		rps:     rate.Limit(float64(rpm) / 60.0),
		burst:   burst,
		idleTTL: idleTTL,
		clock:   time.Now,
	}
	go rl.gcLoop(cleanup, stop)
	return rl
}

func (rl *rateLimiter) get(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.clock()
	if e, ok := rl.buckets[key]; ok {
		e.lastHit = now
		return e.limiter
	}
	lim := rate.NewLimiter(rl.rps, rl.burst)
	rl.buckets[key] = &rateLimitEntry{limiter: lim, lastHit: now}
	return lim
}

func (rl *rateLimiter) gcLoop(interval time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			rl.gc()
		}
	}
}

func (rl *rateLimiter) gc() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := rl.clock().Add(-rl.idleTTL)
	for k, e := range rl.buckets {
		if e.lastHit.Before(cutoff) {
			delete(rl.buckets, k)
		}
	}
}

// rateLimitKey возвращает ключ-источник: User.ID при auth, иначе IP.
func rateLimitKey(r *http.Request, trustProxy bool) string {
	if u, ok := core.UserFromContext(r.Context()); ok && u != nil {
		return "u:" + u.ID.String()
	}
	return "ip:" + clientIP(r, trustProxy)
}

// clientIP извлекает IP клиента. С TrustProxyHeader=true уважает
// X-Forwarded-For (первый адрес) и X-Real-IP.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Первый адрес — original client.
			for i := range len(xff) {
				if xff[i] == ',' {
					return trimSpace(xff[:i])
				}
			}
			return trimSpace(xff)
		}
		if xr := r.Header.Get("X-Real-Ip"); xr != "" {
			return trimSpace(xr)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// RateLimitMiddleware — keyed token-bucket limiter. На превышении возвращает
// 429 с заголовками Retry-After и X-RateLimit-*. При cfg.Enabled=false —
// pass-through. stop — канал для остановки фонового GC (закрыть при shutdown).
func RateLimitMiddleware(cfg RateLimitConfig, log *logger.Logger, stop <-chan struct{}) func(http.Handler) http.Handler {
	if !cfg.Enabled {
		return func(next http.Handler) http.Handler { return next }
	}
	rl := newRateLimiter(cfg.RequestsPerMinute, cfg.Burst, cfg.IdleTTL, cfg.CleanupInterval, stop)
	limit := strconv.Itoa(cfg.RequestsPerMinute)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rateLimitKey(r, cfg.TrustProxyHeader)
			lim := rl.get(key)
			res := lim.Reserve()
			if !res.OK() {
				writeRateLimited(w, limit, "0", "60")
				if log != nil {
					log.Warn("rate-limit: rejected key=%s (no reservation)", key)
				}
				return
			}
			delay := res.Delay()
			if delay > 0 {
				res.Cancel()
				retry := max(int(delay.Seconds()+0.999), 1)
				w.Header().Set("X-Ratelimit-Limit", limit)
				w.Header().Set("X-Ratelimit-Remaining", "0")
				w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(retry))
				w.Header().Set("Retry-After", strconv.Itoa(retry))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate_limited"})
				if log != nil {
					log.Warn("rate-limit: throttled key=%s retry_after=%ds", key, retry)
				}
				return
			}
			tokens := max(int(lim.Tokens()), 0)
			w.Header().Set("X-Ratelimit-Limit", limit)
			w.Header().Set("X-Ratelimit-Remaining", strconv.Itoa(tokens))
			next.ServeHTTP(w, r)
		})
	}
}

func writeRateLimited(w http.ResponseWriter, limit, remaining, retryAfter string) {
	w.Header().Set("X-Ratelimit-Limit", limit)
	w.Header().Set("X-Ratelimit-Remaining", remaining)
	w.Header().Set("Retry-After", retryAfter)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate_limited"})
}
