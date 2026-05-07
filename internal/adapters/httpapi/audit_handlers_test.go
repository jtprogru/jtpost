package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/sqlite"
	"github.com/jtprogru/jtpost/internal/core"
)

func setupAuditRepo(t *testing.T) *sqlite.AuditLogRepository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	repo, err := sqlite.NewSQLitePostRepository(sqlite.Config{DSN: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo.AuditLog()
}

func TestAuditHandler_ForbiddenForNonOwner(t *testing.T) {
	repo := setupAuditRepo(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	req = req.WithContext(core.WithRole(req.Context(), core.RoleEditor))
	rec := httptest.NewRecorder()
	AuditHandler(repo)(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
}

func TestAuditHandler_OwnerCanList(t *testing.T) {
	repo := setupAuditRepo(t)
	ctx := context.Background()
	for i, action := range []core.AuditAction{core.AuditAuthLoginSuccess, core.AuditPostCreated} {
		_ = repo.Append(ctx, &core.AuditEntry{
			Action:    action,
			Outcome:   core.AuditOutcomeSuccess,
			ActorID:   uuid.New(),
			ActorType: core.AuditActorUser,
			Metadata:  map[string]any{"i": i},
		})
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	req = req.WithContext(core.WithRole(req.Context(), core.RoleOwner))
	rec := httptest.NewRecorder()
	AuditHandler(repo)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Entries []jsonAuditEntry `json:"entries"`
		Count   int              `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Count != 2 || len(resp.Entries) != 2 {
		t.Fatalf("expected 2 entries, got count=%d len=%d", resp.Count, len(resp.Entries))
	}
}

func TestAuditHandler_FilterByAction(t *testing.T) {
	repo := setupAuditRepo(t)
	ctx := context.Background()
	for _, action := range []core.AuditAction{
		core.AuditAuthLoginSuccess,
		core.AuditAuthLoginFail,
		core.AuditAuthLoginFail,
	} {
		_ = repo.Append(ctx, &core.AuditEntry{
			Action: action, Outcome: core.AuditOutcomeSuccess, ActorType: core.AuditActorAnonymous,
		})
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?action=auth.login.fail", nil)
	req = req.WithContext(core.WithRole(req.Context(), core.RoleOwner))
	rec := httptest.NewRecorder()
	AuditHandler(repo)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var resp struct {
		Entries []jsonAuditEntry `json:"entries"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Entries) != 2 {
		t.Fatalf("expected 2 fails, got %d", len(resp.Entries))
	}
}

func TestAuditHandler_LimitClamped(t *testing.T) {
	repo := setupAuditRepo(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?limit=99999", nil)
	req = req.WithContext(core.WithRole(req.Context(), core.RoleOwner))
	rec := httptest.NewRecorder()
	AuditHandler(repo)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAuditHandler_MethodNotAllowed(t *testing.T) {
	repo := setupAuditRepo(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit", nil)
	req = req.WithContext(core.WithRole(req.Context(), core.RoleOwner))
	rec := httptest.NewRecorder()
	AuditHandler(repo)(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}
