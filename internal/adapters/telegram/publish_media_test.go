package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jtprogru/jtpost/internal/core"
)

// fakeTGServer возвращает httptest сервер, эмулирующий Telegram API.
// captured хранит последний URL path и payload для assertions.
type fakeTGServer struct {
	*httptest.Server

	lastPath        string
	lastPayload     map[string]any
	lastContentType string
	lastRawBody     []byte
}

func newFakeTG(t *testing.T) *fakeTGServer {
	t.Helper()
	f := &fakeTGServer{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.lastPath = r.URL.Path
		f.lastContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		f.lastRawBody = body
		f.lastPayload = nil
		_ = json.Unmarshal(body, &f.lastPayload)
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/sendMediaGroup") {
			_, _ = w.Write([]byte(`{"ok":true,"result":[{"message_id":42,"chat":{"id":-1001234567890,"username":"chan"}},{"message_id":43,"chat":{"id":-1001234567890,"username":"chan"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42,"chat":{"id":-1001234567890,"username":"chan"}}}`))
	}))
	t.Cleanup(f.Close)
	return f
}

func newTestPublisher(t *testing.T, siteURL string) (*Publisher, *fakeTGServer) {
	t.Helper()
	tg := newFakeTG(t)
	p := NewPublisher(Config{
		BotToken:    "test-token",
		ChannelID:   "-1001234567890",
		SiteBaseURL: siteURL,
	})
	p.baseURL = tg.URL + "/bot"
	return p, tg
}

func TestPublish_NoImages_UsesSendMessage(t *testing.T) {
	t.Parallel()
	p, tg := newTestPublisher(t, "")
	post := &core.Post{ID: mustParsePostID("a"), Title: "Hi", Content: "no images here"}
	if _, err := p.Publish(context.Background(), post); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.HasSuffix(tg.lastPath, "/sendMessage") {
		t.Errorf("expected sendMessage path, got %s", tg.lastPath)
	}
}

func TestPublish_AbsoluteImage_UsesSendPhoto(t *testing.T) {
	t.Parallel()
	p, tg := newTestPublisher(t, "")
	post := &core.Post{
		ID: mustParsePostID("a"), Title: "Cat", Content: "look ![](https://example.com/cat.jpg)",
	}
	if _, err := p.Publish(context.Background(), post); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.HasSuffix(tg.lastPath, "/sendPhoto") {
		t.Errorf("expected sendPhoto, got %s", tg.lastPath)
	}
	if got, _ := tg.lastPayload["photo"].(string); got != "https://example.com/cat.jpg" {
		t.Errorf("photo=%v", got)
	}
	caption, _ := tg.lastPayload["caption"].(string)
	if !strings.Contains(caption, "<b>Cat</b>") {
		t.Errorf("caption missing title: %q", caption)
	}
}

func TestPublish_RelativeImage_NoSiteURL_FallsBackToText(t *testing.T) {
	t.Parallel()
	p, tg := newTestPublisher(t, "")
	post := &core.Post{
		ID: mustParsePostID("a"), Title: "T", Content: "![](/ui/uploads/x.png)",
	}
	if _, err := p.Publish(context.Background(), post); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.HasSuffix(tg.lastPath, "/sendMessage") {
		t.Errorf("relative URL без siteURL должен fallback'нуть в sendMessage; path=%s", tg.lastPath)
	}
}

func TestPublish_RelativeImage_WithSiteURL_ResolvesToAbsolute(t *testing.T) {
	t.Parallel()
	p, tg := newTestPublisher(t, "https://my.site")
	post := &core.Post{
		ID: mustParsePostID("a"), Title: "T", Content: "![](/ui/uploads/x.png)",
	}
	if _, err := p.Publish(context.Background(), post); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.HasSuffix(tg.lastPath, "/sendPhoto") {
		t.Fatalf("expected sendPhoto, got %s", tg.lastPath)
	}
	if got, _ := tg.lastPayload["photo"].(string); got != "https://my.site/ui/uploads/x.png" {
		t.Errorf("photo=%v, want resolved absolute URL", got)
	}
}

func TestPublish_LongContent_TruncatesCaption(t *testing.T) {
	t.Parallel()
	p, tg := newTestPublisher(t, "")
	long := strings.Repeat("ab cd ", 250) // > 1024 chars
	post := &core.Post{
		ID:      mustParsePostID("a"),
		Title:   "Long",
		Content: "![](https://x/y.png)\n\n" + long,
	}
	if _, err := p.Publish(context.Background(), post); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// Последний запрос должен быть sendMessage (followup для длинного body).
	if !strings.HasSuffix(tg.lastPath, "/sendMessage") {
		t.Errorf("expected followup sendMessage; last path=%s", tg.lastPath)
	}
}

func TestPublish_PrivateUpload_UsesMultipart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	relDir := filepath.Join("2026", "05")
	if err := os.MkdirAll(filepath.Join(dir, relDir), 0o755); err != nil {
		t.Fatal(err)
	}
	imgBytes := []byte("\x89PNG\r\n\x1a\n fake png bytes")
	if err := os.WriteFile(filepath.Join(dir, relDir, "abc.png"), imgBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	tg := newFakeTG(t)
	p := NewPublisher(Config{
		BotToken:    "tok",
		ChannelID:   "-1001234567890",
		UploadDir:   dir,
		UploadRoute: "/ui/uploads/",
	})
	p.baseURL = tg.URL + "/bot"

	post := &core.Post{ID: mustParsePostID("a"), Title: "T", Content: "![](/ui/uploads/2026/05/abc.png)"}
	if _, err := p.Publish(context.Background(), post); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.HasSuffix(tg.lastPath, "/sendPhoto") {
		t.Errorf("expected sendPhoto, got %s", tg.lastPath)
	}
	if !strings.HasPrefix(tg.lastContentType, "multipart/form-data") {
		t.Errorf("expected multipart, got Content-Type=%q", tg.lastContentType)
	}
	if !strings.Contains(string(tg.lastRawBody), "abc.png") {
		t.Errorf("multipart body должен содержать имя файла")
	}
}

func TestPublish_PrivateUpload_PrefersMultipartOverSiteURL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "2026"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026", "x.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tg := newFakeTG(t)
	p := NewPublisher(Config{
		BotToken:    "tok",
		ChannelID:   "-1001234567890",
		SiteBaseURL: "https://my.site",
		UploadDir:   dir,
		UploadRoute: "/ui/uploads/",
	})
	p.baseURL = tg.URL + "/bot"

	post := &core.Post{ID: mustParsePostID("a"), Title: "T", Content: "![](/ui/uploads/2026/x.png)"}
	if _, err := p.Publish(context.Background(), post); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.HasPrefix(tg.lastContentType, "multipart/form-data") {
		t.Errorf("multipart должен иметь приоритет; CT=%q", tg.lastContentType)
	}
}

func TestPublish_PrivateUpload_TraversalRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tg := newFakeTG(t)
	p := NewPublisher(Config{
		BotToken: "tok", ChannelID: "-1001234567890",
		UploadDir: dir, UploadRoute: "/ui/uploads/",
	})
	p.baseURL = tg.URL + "/bot"
	post := &core.Post{ID: mustParsePostID("a"), Title: "T", Content: "![](/ui/uploads/../../etc/passwd)"}
	if _, err := p.Publish(context.Background(), post); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// Traversal не должен пройти — fallback к sendMessage.
	if !strings.HasSuffix(tg.lastPath, "/sendMessage") {
		t.Errorf("traversal должен fallback в sendMessage; got %s", tg.lastPath)
	}
}

func TestPublish_MultipleImages_UsesMediaGroup(t *testing.T) {
	t.Parallel()
	p, tg := newTestPublisher(t, "")
	post := &core.Post{
		ID:    mustParsePostID("a"),
		Title: "Trip",
		Content: "Day 1 ![](https://x/a.jpg) and ![](https://x/b.jpg) and ![](https://x/c.jpg)",
	}
	if _, err := p.Publish(context.Background(), post); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.HasSuffix(tg.lastPath, "/sendMediaGroup") {
		t.Errorf("expected sendMediaGroup, got %s", tg.lastPath)
	}
	if !strings.HasPrefix(tg.lastContentType, "multipart/form-data") {
		t.Errorf("expected multipart, got %q", tg.lastContentType)
	}
	body := string(tg.lastRawBody)
	for _, want := range []string{"a.jpg", "b.jpg", "c.jpg", "Trip"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestPublish_MediaGroup_LimitTo10(t *testing.T) {
	t.Parallel()
	p, tg := newTestPublisher(t, "")
	var sb strings.Builder
	for i := range 12 {
		fmt.Fprintf(&sb, "![](https://x/%d.jpg) ", i)
	}
	post := &core.Post{ID: mustParsePostID("a"), Title: "Many", Content: sb.String()}
	if _, err := p.Publish(context.Background(), post); err != nil {
		t.Fatalf("publish: %v", err)
	}
	body := string(tg.lastRawBody)
	// Должны попасть 0..9, НЕ должны 10, 11.
	if !strings.Contains(body, "9.jpg") {
		t.Error("9.jpg должен быть в body")
	}
	if strings.Contains(body, "10.jpg") || strings.Contains(body, "11.jpg") {
		t.Error("10.jpg/11.jpg выходят за лимит 10 и не должны передаваться")
	}
}

func TestPublish_MediaGroup_MixedSources(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "2026"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026", "local.png"), []byte("fakebytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	tg := newFakeTG(t)
	p := NewPublisher(Config{
		BotToken: "tok", ChannelID: "-1001234567890",
		UploadDir: dir, UploadRoute: "/ui/uploads/",
	})
	p.baseURL = tg.URL + "/bot"

	post := &core.Post{
		ID:      mustParsePostID("a"),
		Title:   "Mix",
		Content: "![](https://x/remote.jpg) plus ![](/ui/uploads/2026/local.png)",
	}
	if _, err := p.Publish(context.Background(), post); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.HasSuffix(tg.lastPath, "/sendMediaGroup") {
		t.Errorf("expected sendMediaGroup, got %s", tg.lastPath)
	}
	body := string(tg.lastRawBody)
	if !strings.Contains(body, "remote.jpg") {
		t.Error("remote URL должен быть в media JSON")
	}
	if !strings.Contains(body, "attach://photo1") {
		t.Error("local-файл должен быть как attach://photoN reference")
	}
	if !strings.Contains(body, "local.png") {
		t.Error("multipart часть с local.png должна присутствовать")
	}
}

func TestFirstResolvableImage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		siteURL string
		imgs    []MDImage
		want    string
	}{
		{"empty", "", nil, ""},
		{"absolute https", "", []MDImage{{URL: "https://x/y.png"}}, "https://x/y.png"},
		{"relative no siteURL", "", []MDImage{{URL: "/ui/uploads/x.png"}}, ""},
		{"relative with siteURL", "https://s", []MDImage{{URL: "/ui/uploads/x.png"}}, "https://s/ui/uploads/x.png"},
		{"siteURL trailing slash", "https://s/", []MDImage{{URL: "/x.png"}}, "https://s/x.png"},
		{"non-rooted relative skipped", "https://s", []MDImage{{URL: "rel.png"}}, ""},
		{"absolute precedes relative", "", []MDImage{{URL: "/skip.png"}, {URL: "https://x/y.png"}}, "https://x/y.png"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := NewPublisher(Config{SiteBaseURL: tc.siteURL})
			if got := p.firstResolvableImage(tc.imgs); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
