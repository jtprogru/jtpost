package httpapi

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/core"
)

// OAuthStateCookieName — cookie с state-token для CSRF-protection OAuth-flow.
const OAuthStateCookieName = "jtpost_oauth_state"

// OAuthStateMaxAge — TTL state cookie (10 минут).
const OAuthStateMaxAge = 600

// OAuthHandler обрабатывает /api/auth/oauth/{provider} routes.
type OAuthHandler struct {
	oauthSvc *core.OAuthService
	authSvc  *core.AuthService
	auditSvc *core.AuditService // nil-safe
	cfg      *config.Config
}

// NewOAuthHandler создаёт handler.
func NewOAuthHandler(oauthSvc *core.OAuthService, authSvc *core.AuthService, audit *core.AuditService, cfg *config.Config) *OAuthHandler {
	return &OAuthHandler{oauthSvc: oauthSvc, authSvc: authSvc, auditSvc: audit, cfg: cfg}
}

// ServeHTTP — единая точка входа для path-routing /api/auth/oauth/{provider}[/callback].
func (h *OAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/auth/oauth/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	provider := parts[0]
	if provider == "" {
		http.NotFound(w, r)
		return
	}
	if !h.oauthSvc.HasProvider(provider) {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 2 && parts[1] == "callback" {
		h.handleCallback(w, r, provider)
		return
	}
	if len(parts) == 1 {
		h.handleInitiate(w, r, provider)
		return
	}
	http.NotFound(w, r)
}

func (h *OAuthHandler) handleInitiate(w http.ResponseWriter, r *http.Request, provider string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authURL, state, err := h.oauthSvc.BuildAuthorizeURL(provider)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "oauth_init_failed")
		return
	}
	stateCookie := &http.Cookie{
		Name:     OAuthStateCookieName,
		Value:    state,
		Path:     "/api/auth/oauth/",
		HttpOnly: true,
		Secure:   h.cfg.Server.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   OAuthStateMaxAge,
	}
	if h.cfg.Server.CookieDomain != "" {
		stateCookie.Domain = h.cfg.Server.CookieDomain
	}
	http.SetCookie(w, stateCookie)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *OAuthHandler) handleCallback(w http.ResponseWriter, r *http.Request, provider string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stateCookie, err := r.Cookie(OAuthStateCookieName)
	if err != nil || stateCookie.Value == "" {
		writeJSONError(w, http.StatusBadRequest, "state_missing")
		return
	}
	queryState := r.URL.Query().Get("state")
	if subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(queryState)) != 1 {
		writeJSONError(w, http.StatusBadRequest, "state_mismatch")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeJSONError(w, http.StatusBadRequest, "code_missing")
		return
	}

	// Очистить state-cookie.
	clearStateCookie := &http.Cookie{
		Name:     OAuthStateCookieName,
		Value:    "",
		Path:     "/api/auth/oauth/",
		HttpOnly: true,
		Secure:   h.cfg.Server.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
	if h.cfg.Server.CookieDomain != "" {
		clearStateCookie.Domain = h.cfg.Server.CookieDomain
	}
	http.SetCookie(w, clearStateCookie)

	user, err := h.oauthSvc.HandleCallback(r.Context(), provider, code)
	if err != nil {
		_ = h.auditSvc.Log(r.Context(), core.AuditEntry{
			Action:    core.AuditAuthOAuthLogin,
			Outcome:   core.AuditOutcomeFailure,
			ActorType: core.AuditActorAnonymous,
			Metadata:  map[string]any{"provider": provider, "reason": err.Error()},
		})
		if errors.Is(err, core.ErrValidation) {
			writeJSONError(w, http.StatusBadRequest, "oauth_user_info_invalid")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "oauth_failed")
		return
	}

	ttl := h.cfg.Auth.SessionTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	res, err := h.authSvc.IssueSessionForUser(r.Context(), user, ttl)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "session_issue_failed")
		return
	}
	setSessionCookie(w, h.cfg, res.RawToken, res.Session.ExpiresAt)
	_ = h.auditSvc.Log(r.Context(), core.AuditEntry{
		Action:    core.AuditAuthOAuthLogin,
		Outcome:   core.AuditOutcomeSuccess,
		ActorID:   user.ID,
		ActorType: core.AuditActorUser,
		TenantID:  user.TenantID,
		Metadata:  map[string]any{"provider": provider},
	})
	http.Redirect(w, r, "/", http.StatusFound)
}
