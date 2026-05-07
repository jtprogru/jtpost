package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/core"
)

func newTestServerOutboxOnly(outbox core.OutboxRepository) *Server {
	cfg := &config.Config{Auth: config.AuthConfig{TenantDefault: testTenant, AuthorDefault: testAuthor}}
	repo := newMockPostRepository()
	service := core.NewPostService(repo, &mockClock{now: time.Now()})
	return NewServerWithConfig(ServerConfig{Service: service, Outbox: outbox, Config: cfg})
}

func mustEnqueue(t *testing.T, box core.OutboxRepository, status core.OutboxStatus) *core.OutboxEntry {
	t.Helper()
	e := &core.OutboxEntry{
		PostID: core.PostID(uuid.New()), TenantID: testTenant,
		Kind: core.OutboxKindPublish, Status: status, MaxAttempts: 3,
		NextAttemptAt: time.Now().UTC(),
	}
	if err := box.Enqueue(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestServer_ListOutbox_Filter(t *testing.T) {
	box := &mockOutboxRepo{}
	mustEnqueue(t, box, core.OutboxStatusPending)
	mustEnqueue(t, box, core.OutboxStatusPending)
	srv := newTestServerOutboxOnly(box)

	req := httptest.NewRequest(http.MethodGet, "/api/outbox?status=pending&limit=10", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp []outboxView
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 entries, got %d", len(resp))
	}
}

func TestServer_GetOutboxByID(t *testing.T) {
	box := &mockOutboxRepo{}
	e := mustEnqueue(t, box, core.OutboxStatusPending)
	srv := newTestServerOutboxOnly(box)

	req := httptest.NewRequest(http.MethodGet, "/api/outbox/"+e.ID.String(), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var v outboxView
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	if v.ID != e.ID {
		t.Errorf("got id %s, want %s", v.ID, e.ID)
	}
}

func TestServer_GetOutboxByID_NotFound(t *testing.T) {
	srv := newTestServerOutboxOnly(&mockOutboxRepo{})
	req := httptest.NewRequest(http.MethodGet, "/api/outbox/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestServer_RetryOutbox_ResetsToPending(t *testing.T) {
	box := &mockOutboxRepo{}
	e := mustEnqueue(t, box, core.OutboxStatusFailed)
	e.Attempts = 3
	e.LastError = "old error"
	srv := newTestServerOutboxOnly(box)

	req := httptest.NewRequest(http.MethodPost, "/api/outbox/"+e.ID.String()+"/retry", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if e.Status != core.OutboxStatusPending {
		t.Errorf("expected pending after retry, got %s", e.Status)
	}
	if e.Attempts != 0 {
		t.Errorf("expected attempts reset to 0, got %d", e.Attempts)
	}
}

func TestServer_RetryOutbox_DoneEntry_Conflict(t *testing.T) {
	box := &mockOutboxRepo{}
	e := mustEnqueue(t, box, core.OutboxStatusDone)
	srv := newTestServerOutboxOnly(box)
	req := httptest.NewRequest(http.MethodPost, "/api/outbox/"+e.ID.String()+"/retry", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 for done entry, got %d", w.Code)
	}
}

func TestServer_OutboxEndpoints_NoOutbox_503(t *testing.T) {
	srv := newTestServerOutboxOnly(nil)
	for _, p := range []string{"/api/outbox", "/api/outbox/" + uuid.New().String()} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("path %s: expected 503, got %d", p, w.Code)
		}
	}
}

func TestServer_OutboxV1Alias(t *testing.T) {
	box := &mockOutboxRepo{}
	e := mustEnqueue(t, box, core.OutboxStatusPending)
	srv := newTestServerOutboxOnly(box)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/outbox/"+e.ID.String(), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("v1 alias failed: status=%d body=%s", w.Code, w.Body.String())
	}
}
