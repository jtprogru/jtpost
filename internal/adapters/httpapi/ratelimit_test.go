package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRateLimitMiddleware_DisabledIsPassthrough(t *testing.T) {
	t.Parallel()
	stop := make(chan struct{})
	defer close(stop)
	mw := RateLimitMiddleware(RateLimitConfig{Enabled: false}, nil, stop)
	h := mw(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("X-Ratelimit-Limit") != "" {
		t.Fatalf("expected no rate-limit headers when disabled")
	}
}

func TestRateLimitMiddleware_ThrottlesAfterBurst(t *testing.T) {
	t.Parallel()
	stop := make(chan struct{})
	defer close(stop)
	cfg := RateLimitConfig{Enabled: true, RequestsPerMinute: 60, Burst: 3}
	mw := RateLimitMiddleware(cfg, nil, stop)
	h := mw(okHandler())

	allowed := 0
	throttled := 0
	for range 10 {
		req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
		req.RemoteAddr = "10.0.0.42:9999"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		switch rr.Code {
		case http.StatusOK:
			allowed++
		case http.StatusTooManyRequests:
			throttled++
			if rr.Header().Get("Retry-After") == "" {
				t.Fatalf("expected Retry-After header on 429")
			}
			if rr.Header().Get("X-Ratelimit-Remaining") != "0" {
				t.Fatalf("expected X-Ratelimit-Remaining=0, got %q", rr.Header().Get("X-Ratelimit-Remaining"))
			}
		default:
			t.Fatalf("unexpected status %d", rr.Code)
		}
	}
	if allowed != 3 {
		t.Fatalf("expected exactly 3 allowed (burst), got %d (throttled=%d)", allowed, throttled)
	}
	if throttled == 0 {
		t.Fatalf("expected some 429 responses")
	}
}

func TestRateLimitMiddleware_PerKeyIsolation(t *testing.T) {
	t.Parallel()
	stop := make(chan struct{})
	defer close(stop)
	cfg := RateLimitConfig{Enabled: true, RequestsPerMinute: 60, Burst: 2}
	mw := RateLimitMiddleware(cfg, nil, stop)
	h := mw(okHandler())

	// IP A исчерпывает burst.
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1111"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("A request %d: expected 200, got %d", i, rr.Code)
		}
	}
	// IP B должен иметь свой bucket.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:2222"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("B request: expected 200 (separate bucket), got %d", rr.Code)
	}
	// IP A теперь throttled.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1111"
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("A overflow: expected 429, got %d", rr.Code)
	}
}

func TestRateLimitMiddleware_KeyByUserWhenAuthenticated(t *testing.T) {
	t.Parallel()
	stop := make(chan struct{})
	defer close(stop)
	cfg := RateLimitConfig{Enabled: true, RequestsPerMinute: 60, Burst: 1}
	mw := RateLimitMiddleware(cfg, nil, stop)
	h := mw(okHandler())

	user := &core.User{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111")}

	// Один user.ID — два разных RemoteAddr → один bucket.
	for i, addr := range []string{"10.0.0.1:1", "10.0.0.2:2"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = addr
		req = req.WithContext(core.WithUser(req.Context(), user))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if i == 0 && rr.Code != http.StatusOK {
			t.Fatalf("first user request: expected 200, got %d", rr.Code)
		}
		if i == 1 && rr.Code != http.StatusTooManyRequests {
			t.Fatalf("second user request (different IP, same user): expected 429, got %d", rr.Code)
		}
	}
}

func TestRateLimitMiddleware_TrustProxyXFF(t *testing.T) {
	t.Parallel()
	stop := make(chan struct{})
	defer close(stop)
	cfg := RateLimitConfig{Enabled: true, RequestsPerMinute: 60, Burst: 1, TrustProxyHeader: true}
	mw := RateLimitMiddleware(cfg, nil, stop)
	h := mw(okHandler())

	// Same RemoteAddr (proxy), different X-Forwarded-For → разные buckets.
	for i, xff := range []string{"203.0.113.1, 10.0.0.1", "203.0.113.2"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1"
		req.Header.Set("X-Forwarded-For", xff)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("XFF %d (%q): expected 200, got %d", i, xff, rr.Code)
		}
	}
}

func TestRateLimiter_GCRemovesIdleEntries(t *testing.T) {
	t.Parallel()
	stop := make(chan struct{})
	defer close(stop)
	rl := newRateLimiter(60, 5, time.Hour, time.Hour, stop)

	now := time.Now()
	rl.clock = func() time.Time { return now }
	rl.get("k1")
	rl.get("k2")
	if len(rl.buckets) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(rl.buckets))
	}

	// Сдвигаем clock на > idleTTL.
	rl.clock = func() time.Time { return now.Add(2 * time.Hour) }
	rl.gc()
	if len(rl.buckets) != 0 {
		t.Fatalf("expected GC to remove all stale buckets, %d remain", len(rl.buckets))
	}
}
