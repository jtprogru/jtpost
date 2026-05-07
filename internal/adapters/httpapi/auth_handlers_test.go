package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/httpapi/oapigen"
	"github.com/jtprogru/jtpost/internal/adapters/sqlite"
	"github.com/jtprogru/jtpost/internal/core"
)

// setupHandler создаёт SQLite-репо + cfg + AuthService + один user.
func setupHandler(t *testing.T) (*core.AuthService, *config.Config) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "auth.db")
	repo, err := sqlite.NewSQLitePostRepository(sqlite.Config{DSN: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	svc := core.NewAuthService(repo.Users(), repo.Tokens(), repo.Sessions(), core.NewMultiHasher(), core.SystemClock{})
	cfg := config.NewDefaultConfig()
	cfg.Auth.TenantDefault = uuid.New()
	cfg.Auth.AuthorDefault = uuid.New()
	cfg.Auth.SessionTTL = time.Hour
	cfg.Server.CookieSecure = false // для unit-тестов

	if _, err := svc.CreateUser(context.Background(), core.CreateUserInput{
		TenantID: cfg.Auth.TenantDefault,
		Email:    "owner@example.com",
		Password: "password123",
		Role:     core.RoleOwner,
	}); err != nil {
		t.Fatal(err)
	}
	return svc, cfg
}

func TestLoginHandler_Success(t *testing.T) {
	svc, cfg := setupHandler(t)
	body, _ := json.Marshal(oapigen.LoginRequest{Email: "owner@example.com", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	LoginHandler(svc, cfg, nil)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("session cookie not set")
	}
	if !sessionCookie.HttpOnly {
		t.Error("cookie must be HttpOnly")
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite=%v, want Lax", sessionCookie.SameSite)
	}
	if !strings.HasPrefix(sessionCookie.Value, "jts_") {
		t.Errorf("cookie value = %q, want jts_*", sessionCookie.Value)
	}
	var resp oapigen.LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.CsrfToken == "" || resp.UserId == uuid.Nil || resp.Role == "" {
		t.Errorf("response incomplete: %+v", resp)
	}
}

func TestLoginHandler_WrongPassword(t *testing.T) {
	svc, cfg := setupHandler(t)
	body, _ := json.Marshal(oapigen.LoginRequest{Email: "owner@example.com", Password: "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	LoginHandler(svc, cfg, nil)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", rec.Code)
	}
}

func TestLogoutHandler_NoCookie_Idempotent(t *testing.T) {
	svc, cfg := setupHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()
	LogoutHandler(svc, cfg, nil)(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status=%d, want 200", rec.Code)
	}
	// Должна быть Set-Cookie с MaxAge=-1.
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName && c.MaxAge < 0 {
			return
		}
	}
	t.Error("clear-cookie not set")
}

func TestCSRFHandler_NoSession_401(t *testing.T) {
	svc, _ := setupHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/csrf", nil)
	rec := httptest.NewRecorder()
	CSRFHandler(svc)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", rec.Code)
	}
}

func TestCSRFHandler_WithSession_NewCSRF(t *testing.T) {
	svc, cfg := setupHandler(t)
	// Login → получить session
	body, _ := json.Marshal(oapigen.LoginRequest{Email: "owner@example.com", Password: "password123"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	loginRec := httptest.NewRecorder()
	LoginHandler(svc, cfg, nil)(loginRec, loginReq)
	var loginResp oapigen.LoginResponse
	_ = json.Unmarshal(loginRec.Body.Bytes(), &loginResp)
	oldCSRF := loginResp.CsrfToken

	// Найти cookie
	var sessCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == SessionCookieName {
			sessCookie = c
		}
	}

	// Положить session в ctx через middleware (имитация)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/csrf", nil)
	req.AddCookie(sessCookie)
	rec := httptest.NewRecorder()
	chain := SessionMiddleware(svc)(CSRFHandler(svc))
	chain.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["csrf_token"] == "" || resp["csrf_token"] == oldCSRF {
		t.Errorf("new csrf must differ from old: old=%q new=%q", oldCSRF, resp["csrf_token"])
	}
	if rec.Header().Get(CSRFHeaderName) == "" {
		t.Error("X-CSRF-Token header missing")
	}
}

func TestE2E_LoginThenAuthenticated_GET(t *testing.T) {
	svc, cfg := setupHandler(t)

	// Login
	body, _ := json.Marshal(oapigen.LoginRequest{Email: "owner@example.com", Password: "password123"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	loginRec := httptest.NewRecorder()
	LoginHandler(svc, cfg, nil)(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d", loginRec.Code)
	}
	var sessCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == SessionCookieName {
			sessCookie = c
		}
	}

	// Защищённый GET через full chain
	called := false
	handler := fullChain(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		u, ok := core.UserFromContext(r.Context())
		if !ok {
			t.Error("user not in ctx")
		}
		if u != nil && u.Email != "owner@example.com" {
			t.Errorf("wrong user: %s", u.Email)
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.AddCookie(sessCookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Errorf("e2e: called=%v status=%d", called, rec.Code)
	}
}
