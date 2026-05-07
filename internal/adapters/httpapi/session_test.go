package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/sqlite"
	"github.com/jtprogru/jtpost/internal/core"
	"golang.org/x/crypto/bcrypt"
)

// fullChain собирает F4b chain: Bearer → Session → CSRF → RequireAuth.
func fullChain(svc *core.AuthService, h http.Handler) http.Handler {
	h = RequireAuthMiddleware()(h)
	h = CSRFMiddleware()(h)
	h = SessionMiddleware(svc)(h)
	h = BearerTokenMiddleware(svc)(h)
	return h
}

// setupSession создаёт SQLite-репо + user + login → возвращает svc, cookie value, csrf token, user.
func setupSession(t *testing.T) (*core.AuthService, string /*rawCookie*/, string /*csrf*/, *core.User) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "auth.db")
	repo, err := sqlite.NewSQLitePostRepository(sqlite.Config{DSN: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	svc := core.NewAuthService(repo.Users(), repo.Tokens(), repo.Sessions(), bcrypt.MinCost, core.SystemClock{})
	ctx := context.Background()
	tenantID := uuid.New()
	user, err := svc.CreateUser(ctx, core.CreateUserInput{
		TenantID: tenantID,
		Email:    "owner@example.com",
		Password: "password123",
		Role:     core.RoleOwner,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Login(ctx, core.LoginInput{
		TenantID: tenantID,
		Email:    "owner@example.com",
		Password: "password123",
	}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return svc, res.RawToken, res.CSRFToken, user
}

func TestSessionMiddleware_ValidCookie_CtxPopulated(t *testing.T) {
	svc, raw, _, user := setupSession(t)
	called := false
	handler := SessionMiddleware(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		u, ok := core.UserFromContext(r.Context())
		if !ok || u == nil || u.ID != user.ID {
			t.Errorf("user not in ctx: ok=%v u=%v", ok, u)
		}
		src, _ := core.AuthSourceFromContext(r.Context())
		if src != core.AuthSourceSession {
			t.Errorf("auth source = %s, want session", src)
		}
		if _, ok := core.SessionFromContext(r.Context()); !ok {
			t.Error("session not in ctx")
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: raw})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called {
		t.Error("handler must be invoked")
	}
}

func TestSessionMiddleware_NoCookie_FullChain_401(t *testing.T) {
	svc, _, _, _ := setupSession(t)
	handler := fullChain(svc, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler must not be invoked")
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", rec.Code)
	}
}

func TestSessionMiddleware_InvalidCookie_FullChain_401(t *testing.T) {
	svc, _, _, _ := setupSession(t)
	handler := fullChain(svc, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler must not be invoked")
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "jts_aaaaaaaa_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", rec.Code)
	}
}

func TestCSRFMiddleware_GET_Bypass(t *testing.T) {
	svc, raw, _, _ := setupSession(t)
	called := false
	handler := fullChain(svc, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: raw})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Errorf("GET should bypass CSRF: called=%v status=%d", called, rec.Code)
	}
}

func TestCSRFMiddleware_SessionPOST_ValidCSRF(t *testing.T) {
	svc, raw, csrf, _ := setupSession(t)
	called := false
	handler := fullChain(svc, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/posts", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: raw})
	req.Header.Set(CSRFHeaderName, csrf)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Errorf("POST with valid CSRF: called=%v status=%d", called, rec.Code)
	}
}

func TestCSRFMiddleware_SessionPOST_MissingCSRF_403(t *testing.T) {
	svc, raw, _, _ := setupSession(t)
	handler := fullChain(svc, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler must not be invoked")
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/posts", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: raw})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status=%d, want 403", rec.Code)
	}
}

func TestCSRFMiddleware_SessionPOST_WrongCSRF_403(t *testing.T) {
	svc, raw, _, _ := setupSession(t)
	handler := fullChain(svc, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler must not be invoked")
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/posts", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: raw})
	req.Header.Set(CSRFHeaderName, "wrong-csrf-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status=%d, want 403", rec.Code)
	}
}

func TestCSRFMiddleware_BearerPOST_NoCSRF_Pass(t *testing.T) {
	svc, validRaw, _ := setupBearer(t)
	called := false
	handler := fullChain(svc, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/posts", nil)
	req.Header.Set("Authorization", "Bearer "+validRaw)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Errorf("Bearer POST without CSRF: called=%v status=%d", called, rec.Code)
	}
}

func TestCSRFMiddleware_LoginEndpoint_Bypass(t *testing.T) {
	svc, _, _, _ := setupSession(t)
	called := false
	handler := fullChain(svc, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	// /api/auth/login is in skip-list for both CSRF and RequireAuth
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Errorf("login endpoint should bypass middleware: called=%v status=%d", called, rec.Code)
	}
}
