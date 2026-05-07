package webui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/sqlite"
	"github.com/jtprogru/jtpost/internal/core"
)

// setupAuthHandler — Handler с реальными SQLite users/sessions + 1 user.
func setupAuthHandler(t *testing.T) *Handler {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "ui-auth.db")
	repo, err := sqlite.NewSQLitePostRepository(sqlite.Config{DSN: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	auth := core.NewAuthService(repo.Users(), repo.Tokens(), repo.Sessions(), core.NewMultiHasher(), core.SystemClock{})
	cfg := config.NewDefaultConfig()
	cfg.Auth.Type = "token"
	cfg.Auth.TenantDefault = uuid.New()
	cfg.Auth.SessionTTL = time.Hour
	cfg.Server.CookieSecure = false
	if _, err := auth.CreateUser(context.Background(), core.CreateUserInput{
		TenantID: cfg.Auth.TenantDefault,
		Email:    "owner@example.com",
		Password: "password123",
		Role:     core.RoleOwner,
	}); err != nil {
		t.Fatal(err)
	}

	svc := core.NewPostService(repo, core.SystemClock{})
	return NewHandler(Config{Service: svc, Auth: auth, Cfg: cfg})
}

func TestUI_Login_GET_RendersForm(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/ui/login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"<form", "name=\"email\"", "name=\"password\"", "Log in"} {
		if !strings.Contains(body, want) {
			t.Errorf("login form missing %q", want)
		}
	}
}

func TestUI_Login_POST_SuccessSetsCookieAndRedirects(t *testing.T) {
	h := setupAuthHandler(t)
	form := url.Values{"email": {"owner@example.com"}, "password": {"password123"}}
	req := httptest.NewRequest(http.MethodPost, "/ui/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	if rec.Header().Get("Location") != "/ui/" {
		t.Errorf("redirect to %q, want /ui/", rec.Header().Get("Location"))
	}
	var sess *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName {
			sess = c
		}
	}
	if sess == nil {
		t.Fatal("session cookie not set")
	}
	if !strings.HasPrefix(sess.Value, "jts_") {
		t.Errorf("cookie value %q, want jts_* prefix", sess.Value)
	}
	if !sess.HttpOnly {
		t.Error("cookie must be HttpOnly")
	}
}

func TestUI_Login_POST_WrongPasswordRendersError(t *testing.T) {
	h := setupAuthHandler(t)
	form := url.Values{"email": {"owner@example.com"}, "password": {"WRONG"}}
	req := httptest.NewRequest(http.MethodPost, "/ui/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "form-error") {
		t.Errorf("expected form-error block in body")
	}
	// Email сохраняется в форме.
	if !strings.Contains(body, "owner@example.com") {
		t.Errorf("expected email pre-filled in form")
	}
}

func TestUI_Login_POST_MissingFieldsError(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/ui/login", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "обязательны") {
		t.Errorf("expected validation error message")
	}
}

func TestUI_Login_GET_AuthenticatedRedirects(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/ui/login", nil)
	user := &core.User{ID: uuid.New(), Email: "x@y", TenantID: uuid.New()}
	req = req.WithContext(core.WithUser(req.Context(), user))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect when already authed, got %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/ui/" {
		t.Errorf("redirect to %q", rec.Header().Get("Location"))
	}
}

func TestUI_Logout_ClearsCookieAndRedirects(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/ui/logout", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	if rec.Header().Get("Location") != "/ui/login" {
		t.Errorf("redirect to %q, want /ui/login", rec.Header().Get("Location"))
	}
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("session cookie not cleared (MaxAge<0)")
	}
}

func TestUI_Login_GET_MethodNotAllowed(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/ui/login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want 405", rec.Code)
	}
}

func TestUI_Login_NoAuthService_503(t *testing.T) {
	h := NewHandler(Config{}) // no Auth/Cfg
	form := url.Values{"email": {"x"}, "password": {"y"}}
	req := httptest.NewRequest(http.MethodPost, "/ui/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", rec.Code)
	}
}

func TestUI_Dashboard_AnonRedirectsToLogin_WhenAuthEnabled(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/ui/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	if rec.Header().Get("Location") != "/ui/login" {
		t.Errorf("redirect to %q", rec.Header().Get("Location"))
	}
}
