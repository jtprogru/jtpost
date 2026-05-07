package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
)

// fakeTGServerWith429 — отдаёт 429 первые `failures` раз, потом 200.
type fakeTGServerWith429 struct {
	*httptest.Server

	calls    atomic.Int32
	failures int32
}

func newFakeTGWith429(t *testing.T, failures int32, retryAfter int) *fakeTGServerWith429 {
	t.Helper()
	f := &fakeTGServerWith429{failures: failures}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := f.calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n <= f.failures {
			w.WriteHeader(http.StatusTooManyRequests)
			body := `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":` +
				itoaSimple(retryAfter) + `}}`
			_, _ = w.Write([]byte(body))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42,"chat":{"id":-1001234567890,"username":"chan"}}}`))
	}))
	t.Cleanup(f.Close)
	return f
}

func itoaSimple(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestPublisher_429_RetriesAndSucceeds(t *testing.T) {
	t.Parallel()
	tg := newFakeTGWith429(t, 1, 1) // первый ответ 429, retry_after=1s
	p := NewPublisher(Config{BotToken: "tok", ChannelID: "-1001234567890"})
	p.baseURL = tg.URL + "/bot"

	post := &core.Post{ID: mustParsePostID("a"), Title: "T", Content: "no images"}
	if _, err := p.Publish(context.Background(), post); err != nil {
		t.Fatalf("publish should succeed after retry: %v", err)
	}
	if got := tg.calls.Load(); got != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", got)
	}
}

func TestPublisher_429_RetryAfterTooLarge_ReturnsError(t *testing.T) {
	t.Parallel()
	tg := newFakeTGWith429(t, 5, 60) // 60s > telegramMaxRetryAfter
	p := NewPublisher(Config{BotToken: "tok", ChannelID: "-1001234567890"})
	p.baseURL = tg.URL + "/bot"

	post := &core.Post{ID: mustParsePostID("a"), Title: "T", Content: "no images"}
	_, err := p.Publish(context.Background(), post)
	if err == nil {
		t.Fatal("expected error when retry_after > limit")
	}
	var apiErr *telegramAPIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected telegramAPIError(429), got %T: %v", err, err)
	}
	// Только 1 попытка — не ретраим, потому что retry_after слишком большой.
	if got := tg.calls.Load(); got != 1 {
		t.Errorf("expected 1 call (no retry), got %d", got)
	}
}

func TestPublisher_429_ExceedsMaxRetries_ReturnsError(t *testing.T) {
	t.Parallel()
	// 5 failures > telegramMaxRetries (2 retries после первой попытки = 3 calls max)
	tg := newFakeTGWith429(t, 5, 1)
	p := NewPublisher(Config{BotToken: "tok", ChannelID: "-1001234567890"})
	p.baseURL = tg.URL + "/bot"

	post := &core.Post{ID: mustParsePostID("a"), Title: "T", Content: "x"}
	_, err := p.Publish(context.Background(), post)
	if err == nil {
		t.Fatal("expected error after exceeded retries")
	}
	if got := tg.calls.Load(); got != 3 { // 1 initial + 2 retries
		t.Errorf("expected 3 calls (1+2 retries), got %d", got)
	}
}

func TestPublisher_429_ContextCancelledDuringWait(t *testing.T) {
	t.Parallel()
	// retry_after=5s — за это время отменим ctx.
	tg := newFakeTGWith429(t, 5, 5)
	p := NewPublisher(Config{BotToken: "tok", ChannelID: "-1001234567890"})
	p.baseURL = tg.URL + "/bot"

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	post := &core.Post{ID: mustParsePostID("a"), Title: "T", Content: "x"}
	_, err := p.Publish(ctx, post)
	if err == nil {
		t.Fatal("expected error after ctx deadline")
	}
	// Ровно 1 успешный API-call (получивший 429); ретрай не дождался — ctx умер.
	if got := tg.calls.Load(); got != 1 {
		t.Errorf("expected 1 call (ctx cancelled before retry), got %d", got)
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int // seconds
	}{
		{"valid", `{"ok":false,"parameters":{"retry_after":5}}`, 5},
		{"missing", `{"ok":false,"description":"x"}`, 0},
		{"zero", `{"ok":false,"parameters":{"retry_after":0}}`, 0},
		{"invalid json", `not json`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseRetryAfter([]byte(tc.body))
			if int(got.Seconds()) != tc.want {
				t.Errorf("got %v, want %ds", got, tc.want)
			}
		})
	}
}
