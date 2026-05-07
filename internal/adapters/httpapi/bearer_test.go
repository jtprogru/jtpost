package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/sqlite"
	"github.com/jtprogru/jtpost/internal/core"
	"golang.org/x/crypto/bcrypt"
)

// setupBearer создаёт SQLite-репо, AuthService, валидный owner и issued PAT.
func setupBearer(t *testing.T) (*core.AuthService, string /* validRaw */, *core.User) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "auth.db")
	repo, err := sqlite.NewSQLitePostRepository(sqlite.Config{DSN: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	svc := core.NewAuthService(repo.Users(), repo.Tokens(), bcrypt.MinCost, core.SystemClock{})
	ctx := context.Background()
	user, err := svc.CreateUser(ctx, core.CreateUserInput{
		TenantID: uuid.New(),
		Email:    "owner@example.com",
		Password: "password123",
		Role:     core.RoleOwner,
	})
	if err != nil {
		t.Fatal(err)
	}
	issued, err := svc.IssueToken(ctx, user.ID, "test-cli", nil)
	if err != nil {
		t.Fatal(err)
	}
	return svc, issued.Raw, user
}

func TestBearerMiddleware_NoHeader_401(t *testing.T) {
	svc, _, _ := setupBearer(t)
	mw := BearerTokenMiddleware(svc)
	handler := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler must not be invoked")
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", rec.Code)
	}
}

func TestBearerMiddleware_BadFormat_401(t *testing.T) {
	svc, _, _ := setupBearer(t)
	mw := BearerTokenMiddleware(svc)
	for _, hdr := range []string{"Basic xyz", "Bearer", "bearer abc", "TokenAuth jtpat_..."} {
		t.Run(hdr, func(t *testing.T) {
			handler := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("handler must not be invoked")
			}))
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.Header.Set("Authorization", hdr)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status=%d, want 401", rec.Code)
			}
		})
	}
}

func TestBearerMiddleware_InvalidToken_401(t *testing.T) {
	svc, _, _ := setupBearer(t)
	mw := BearerTokenMiddleware(svc)
	handler := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler must not be invoked")
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer jtpat_aaaaaaaa_bbbbbbbbbbbbbbbbbbbbbbbb")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", rec.Code)
	}
}

func TestBearerMiddleware_ValidToken_CtxPopulated(t *testing.T) {
	svc, validRaw, owner := setupBearer(t)
	mw := BearerTokenMiddleware(svc)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		ctx := r.Context()
		u, ok := core.UserFromContext(ctx)
		if !ok || u == nil || u.ID != owner.ID {
			t.Errorf("UserFromContext: ok=%v u=%+v", ok, u)
		}
		role, ok := core.RoleFromContext(ctx)
		if !ok || role != core.RoleOwner {
			t.Errorf("RoleFromContext: ok=%v role=%s", ok, role)
		}
		tID, ok := core.TenantFromContext(ctx)
		if !ok || tID != owner.TenantID {
			t.Errorf("TenantFromContext: ok=%v id=%s", ok, tID)
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+validRaw)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called {
		t.Error("handler must be invoked")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status=%d, want 200", rec.Code)
	}
}
