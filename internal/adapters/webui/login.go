package webui

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/webui/components"
	"github.com/jtprogru/jtpost/internal/core"
)

// loginPageHandler — GET /ui/login. Если пользователь уже аутентифицирован —
// сразу redirect на /ui/.
func (h *Handler) loginPageHandler(w http.ResponseWriter, r *http.Request) {
	if u, _ := core.UserFromContext(r.Context()); u != nil {
		http.Redirect(w, r, "/ui/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.Login(components.LoginProps{}).Render(r.Context(), w); err != nil {
		h.log.Error("ui login render: %v", err)
	}
}

// loginSubmitHandler — POST /ui/login. Form-encoded; на успех ставит cookie
// и редиректит на /ui/. На ошибке рендерит форму с inline error.
func (h *Handler) loginSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if h.authSvc == nil || h.cfg == nil {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderLoginError(w, r, "", "invalid form")
		return
	}
	email := r.PostForm.Get("email")
	password := r.PostForm.Get("password")
	if email == "" || password == "" {
		h.renderLoginError(w, r, email, "email и password обязательны")
		return
	}

	ttl := h.cfg.Auth.SessionTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	res, err := h.authSvc.Login(r.Context(), core.LoginInput{
		TenantID: h.cfg.Auth.TenantDefault,
		Email:    email,
		Password: password,
	}, ttl)
	if err != nil {
		_ = h.auditSvc.Log(r.Context(), core.AuditEntry{
			Action:    core.AuditAuthLoginFail,
			Outcome:   core.AuditOutcomeFailure,
			ActorType: core.AuditActorAnonymous,
			TenantID:  h.cfg.Auth.TenantDefault,
			Metadata:  map[string]any{"email": email, "via": "webui"},
		})
		if errors.Is(err, core.ErrUnauthorized) {
			h.renderLoginError(w, r, email, "неверный email или пароль")
			return
		}
		h.renderLoginError(w, r, email, "ошибка входа, попробуйте ещё раз")
		return
	}
	setSessionCookie(w, h.cfg, res.RawToken, res.Session.ExpiresAt)
	_ = h.auditSvc.Log(r.Context(), core.AuditEntry{
		Action:    core.AuditAuthLoginSuccess,
		Outcome:   core.AuditOutcomeSuccess,
		ActorID:   res.User.ID,
		ActorType: core.AuditActorUser,
		TenantID:  res.User.TenantID,
		Metadata:  map[string]any{"via": "webui"},
	})
	http.Redirect(w, r, "/ui/", http.StatusSeeOther)
}

// logoutHandler — POST /ui/logout. Прерывает session, редиректит на login.
func (h *Handler) logoutHandler(w http.ResponseWriter, r *http.Request) {
	if h.authSvc == nil || h.cfg == nil {
		http.Redirect(w, r, "/ui/", http.StatusSeeOther)
		return
	}
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		if u, _, sess, vErr := h.authSvc.ValidateSession(r.Context(), c.Value); vErr == nil && sess != nil {
			actor := core.AuditActorUser
			if u == nil {
				actor = core.AuditActorAnonymous
			}
			var actorID uuid.UUID
			var tenantID uuid.UUID
			if u != nil {
				actorID = u.ID
				tenantID = u.TenantID
			}
			_ = h.authSvc.Logout(r.Context(), sess.ID)
			_ = h.auditSvc.Log(r.Context(), core.AuditEntry{
				Action:    core.AuditAuthLogout,
				Outcome:   core.AuditOutcomeSuccess,
				ActorID:   actorID,
				ActorType: actor,
				TenantID:  tenantID,
				Metadata:  map[string]any{"via": "webui"},
			})
		}
	}
	clearSessionCookie(w, h.cfg)
	http.Redirect(w, r, "/ui/login", http.StatusSeeOther)
}

func (h *Handler) renderLoginError(w http.ResponseWriter, r *http.Request, email, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	if err := components.Login(components.LoginProps{Email: email, Error: msg}).Render(r.Context(), w); err != nil {
		h.log.Error("ui login render error-state: %v", err)
	}
}

// SessionCookieName зеркалит httpapi.SessionCookieName — кросс-пакетная
// duplication по необходимости (избегаем circular import webui→httpapi).
const SessionCookieName = "jtpost_session"

func setSessionCookie(w http.ResponseWriter, cfg *config.Config, value string, expires time.Time) {
	c := &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.Server.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	}
	if cfg.Server.CookieDomain != "" {
		c.Domain = cfg.Server.CookieDomain
	}
	http.SetCookie(w, c)
}

func clearSessionCookie(w http.ResponseWriter, cfg *config.Config) {
	c := &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.Server.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
	if cfg.Server.CookieDomain != "" {
		c.Domain = cfg.Server.CookieDomain
	}
	http.SetCookie(w, c)
}

