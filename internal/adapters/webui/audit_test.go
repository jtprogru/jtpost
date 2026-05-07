package webui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/sqlite"
	"github.com/jtprogru/jtpost/internal/core"
)

func setupAuditUIHandler(t *testing.T) (*Handler, core.AuditRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "ui-audit.db")
	repo, err := sqlite.NewSQLitePostRepository(sqlite.Config{DSN: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	auditRepo := repo.AuditLog()
	cfg := config.NewDefaultConfig()
	cfg.Auth.Type = "token"
	cfg.Auth.TenantDefault = uuid.New()
	cfg.Server.CookieSecure = false
	svc := core.NewPostService(repo, core.SystemClock{})
	h := NewHandler(Config{Service: svc, AuditRepo: auditRepo, Cfg: cfg})
	return h, auditRepo
}

func TestUI_Audit_NoAuditStorage_503(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Auth.Type = "token"
	h := NewHandler(Config{Cfg: cfg})
	req := httptest.NewRequest(http.MethodGet, "/ui/audit", nil)
	req = req.WithContext(core.WithRole(req.Context(), core.RoleOwner))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", rec.Code)
	}
}

func TestUI_Audit_ForbiddenForNonOwner(t *testing.T) {
	h, _ := setupAuditUIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/ui/audit", nil)
	req = req.WithContext(core.WithRole(req.Context(), core.RoleEditor))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
}

func TestUI_Audit_OwnerSeesEntries(t *testing.T) {
	h, repo := setupAuditUIHandler(t)
	ctx := context.Background()
	for _, action := range []core.AuditAction{
		core.AuditAuthLoginSuccess,
		core.AuditPostCreated,
	} {
		_ = repo.Append(ctx, &core.AuditEntry{
			Action:    action,
			Outcome:   core.AuditOutcomeSuccess,
			ActorID:   uuid.New(),
			ActorType: core.AuditActorUser,
			IP:        "10.0.0.1",
		})
	}
	req := httptest.NewRequest(http.MethodGet, "/ui/audit", nil)
	req = req.WithContext(core.WithRole(req.Context(), core.RoleOwner))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"audit-table", "auth.login.success", "post.created", "10.0.0.1"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestUI_Audit_FilterByAction(t *testing.T) {
	h, repo := setupAuditUIHandler(t)
	ctx := context.Background()
	for _, action := range []core.AuditAction{
		core.AuditAuthLoginSuccess,
		core.AuditAuthLoginFail,
	} {
		_ = repo.Append(ctx, &core.AuditEntry{Action: action, Outcome: core.AuditOutcomeSuccess, ActorType: core.AuditActorAnonymous})
	}
	req := httptest.NewRequest(http.MethodGet, "/ui/audit?action=auth.login.fail", nil)
	req = req.WithContext(core.WithRole(req.Context(), core.RoleOwner))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	// Берём только tbody — select-options всегда содержат все варианты.
	tbody := body
	if i := strings.Index(body, "<tbody>"); i >= 0 {
		tbody = body[i:]
	}
	if !strings.Contains(tbody, "auth.login.fail") {
		t.Error("expected fail entry shown in table")
	}
	if strings.Contains(tbody, "auth.login.success") {
		t.Errorf("success entry must be filtered out from table. tbody=%s", tbody)
	}
}

func TestUI_Audit_EmptyState(t *testing.T) {
	h, _ := setupAuditUIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/ui/audit", nil)
	req = req.WithContext(core.WithRole(req.Context(), core.RoleOwner))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Нет audit-событий") {
		t.Error("expected empty state")
	}
}

func TestUI_Audit_MethodNotAllowed(t *testing.T) {
	h, _ := setupAuditUIHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/ui/audit", nil)
	req = req.WithContext(core.WithRole(req.Context(), core.RoleOwner))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want 405", rec.Code)
	}
}
