package httpapi

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

// fakeOAuthProvider — простой mock для OAuthService.
type fakeOAuthProvider struct {
	authorizeURL string
	exchangeRet  string
	exchangeErr  error
	userInfo     *core.OAuthUserInfo
	userInfoErr  error
}

func (p *fakeOAuthProvider) Name() string { return "github" }
func (p *fakeOAuthProvider) AuthorizeURL(state string) string {
	return p.authorizeURL + "?state=" + state
}
func (p *fakeOAuthProvider) Exchange(_ context.Context, _ string) (string, error) {
	if p.exchangeErr != nil {
		return "", p.exchangeErr
	}
	return p.exchangeRet, nil
}
func (p *fakeOAuthProvider) FetchUserInfo(_ context.Context, _ string) (*core.OAuthUserInfo, error) {
	if p.userInfoErr != nil {
		return nil, p.userInfoErr
	}
	return p.userInfo, nil
}

func setupOAuth(t *testing.T) (*OAuthHandler, *core.AuthService, *config.Config, *fakeOAuthProvider) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "auth.db")
	repo, err := sqlite.NewSQLitePostRepository(sqlite.Config{DSN: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	authSvc := core.NewAuthService(repo.Users(), repo.Tokens(), repo.Sessions(), core.NewMultiHasher(), core.SystemClock{})
	cfg := config.NewDefaultConfig()
	cfg.Auth.TenantDefault = uuid.New()
	cfg.Server.CookieSecure = false

	provider := &fakeOAuthProvider{
		authorizeURL: "https://github.com/login/oauth/authorize",
		exchangeRet:  "fake-token",
		userInfo: &core.OAuthUserInfo{
			ExternalID:  "12345",
			Email:       "octo@example.com",
			DisplayName: "octocat",
		},
	}
	oauthSvc := core.NewOAuthService(
		map[string]core.OAuthProvider{"github": provider},
		repo.Users(),
		repo.OAuthAccounts(),
		cfg.Auth.TenantDefault,
		core.RoleAuthor,
		core.SystemClock{},
	)
	return NewOAuthHandler(oauthSvc, authSvc, cfg), authSvc, cfg, provider
}

func TestOAuthHandler_Initiate_RedirectAndCookie(t *testing.T) {
	h, _, _, _ := setupOAuth(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/github", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d, want 302", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://github.com/login/oauth/authorize") {
		t.Errorf("location=%q", loc)
	}
	var stateCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == OAuthStateCookieName {
			stateCookie = c
		}
	}
	if stateCookie == nil || stateCookie.Value == "" {
		t.Fatal("state cookie not set")
	}
	if !stateCookie.HttpOnly {
		t.Error("state cookie must be HttpOnly")
	}
	if !strings.Contains(loc, "state="+stateCookie.Value) {
		t.Errorf("redirect must contain state from cookie")
	}
}

func TestOAuthHandler_Initiate_UnknownProvider_404(t *testing.T) {
	h, _, _, _ := setupOAuth(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/google", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", rec.Code)
	}
}

func TestOAuthHandler_Callback_StateMissing_400(t *testing.T) {
	h, _, _, _ := setupOAuth(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/github/callback?code=abc&state=xyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec.Code)
	}
}

func TestOAuthHandler_Callback_StateMismatch_400(t *testing.T) {
	h, _, _, _ := setupOAuth(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/github/callback?code=abc&state=wrong", nil)
	req.AddCookie(&http.Cookie{Name: OAuthStateCookieName, Value: "real-state"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", rec.Code)
	}
}

func TestOAuthHandler_Callback_Success_SessionCookie_Redirect(t *testing.T) {
	h, _, _, _ := setupOAuth(t)
	// Initiate чтобы получить state.
	initReq := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/github", nil)
	initRec := httptest.NewRecorder()
	h.ServeHTTP(initRec, initReq)
	var state string
	for _, c := range initRec.Result().Cookies() {
		if c.Name == OAuthStateCookieName {
			state = c.Value
		}
	}
	if state == "" {
		t.Fatal("no state cookie")
	}

	// Callback с тем же state.
	cbReq := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/github/callback?code=fake-code&state="+state, nil)
	cbReq.AddCookie(&http.Cookie{Name: OAuthStateCookieName, Value: state})
	cbRec := httptest.NewRecorder()
	h.ServeHTTP(cbRec, cbReq)

	if cbRec.Code != http.StatusFound {
		t.Errorf("status=%d, want 302", cbRec.Code)
	}
	if cbRec.Header().Get("Location") != "/" {
		t.Errorf("location=%q, want /", cbRec.Header().Get("Location"))
	}
	// Должны быть две cookies: state cleared (Max-Age=-1) + session set.
	var sessionSet bool
	for _, c := range cbRec.Result().Cookies() {
		if c.Name == SessionCookieName && c.Value != "" {
			sessionSet = true
		}
	}
	if !sessionSet {
		t.Error("session cookie not set after OAuth callback")
	}
}
