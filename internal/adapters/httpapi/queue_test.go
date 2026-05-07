package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/core"
)

// mockOutboxRepo минимальная реализация core.OutboxRepository для тестов.
type mockOutboxRepo struct {
	mu      sync.Mutex
	entries []*core.OutboxEntry
}

func (m *mockOutboxRepo) Enqueue(_ context.Context, e *core.OutboxEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	m.entries = append(m.entries, e)
	return nil
}
func (m *mockOutboxRepo) ClaimNext(context.Context, time.Time) (*core.OutboxEntry, error) {
	return nil, nil //nolint:nilnil // sentinel
}
func (m *mockOutboxRepo) MarkDone(context.Context, uuid.UUID, time.Time) error { return nil }
func (m *mockOutboxRepo) MarkRetry(context.Context, uuid.UUID, int, time.Time, string, time.Time) error {
	return nil
}
func (m *mockOutboxRepo) MarkFailed(context.Context, uuid.UUID, string, time.Time) error {
	return nil
}
func (m *mockOutboxRepo) List(context.Context, core.OutboxFilter) ([]*core.OutboxEntry, error) {
	return m.entries, nil
}
func (m *mockOutboxRepo) GetByID(context.Context, uuid.UUID) (*core.OutboxEntry, error) {
	return nil, core.ErrNotFound
}
func (m *mockOutboxRepo) SweepStuck(context.Context, time.Duration, time.Time) (int, error) {
	return 0, nil
}

func newTestServerWithOutbox(t *testing.T, outbox core.OutboxRepository) (*Server, *mockPostRepository) {
	t.Helper()
	repo := newMockPostRepository()
	service := core.NewPostService(repo, &mockClock{now: time.Now()})
	cfg := &config.Config{Auth: config.AuthConfig{TenantDefault: testTenant, AuthorDefault: testAuthor}}
	server := NewServerWithConfig(ServerConfig{
		Service: service,
		Outbox:  outbox,
		Config:  cfg,
	})
	return server, repo
}

func TestServer_QueuePost_Enqueues(t *testing.T) {
	outbox := &mockOutboxRepo{}
	server, repo := newTestServerWithOutbox(t, outbox)

	post := fixturePost("queue-1", testTenant, "Q", "q", core.StatusReady)
	post.Content = "x"
	_ = repo.Create(context.Background(), post)

	req := httptest.NewRequest(http.MethodPost, "/api/posts/"+post.ID.String()+"/queue", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body=%s)", w.Code, w.Body.String())
	}
	if len(outbox.entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(outbox.entries))
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "pending" {
		t.Errorf("expected pending status in response, got %v", resp["status"])
	}
}

func TestServer_QueuePost_NotFound(t *testing.T) {
	server, _ := newTestServerWithOutbox(t, &mockOutboxRepo{})
	req := httptest.NewRequest(http.MethodPost, "/api/posts/"+uuid.New().String()+"/queue", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestServer_QueuePost_NoOutbox_503(t *testing.T) {
	server, repo := newTestServerWithOutbox(t, nil)
	post := fixturePost("queue-2", testTenant, "Q", "q", core.StatusReady)
	post.Content = "x"
	_ = repo.Create(context.Background(), post)

	req := httptest.NewRequest(http.MethodPost, "/api/posts/"+post.ID.String()+"/queue", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}
