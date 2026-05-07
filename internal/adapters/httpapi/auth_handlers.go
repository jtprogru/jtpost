package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/httpapi/oapigen"
	"github.com/jtprogru/jtpost/internal/core"
)

// LoginRequest и LoginResponse объявлены в `oapigen` (generated из
// api/openapi.yaml). См. internal/adapters/httpapi/oapigen/types.gen.go.

// LoginHandler handles POST /api/auth/login.
func LoginHandler(svc *core.AuthService, cfg *config.Config, audit *core.AuditService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req oapigen.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body")
			return
		}
		// Revoke existing session if cookie присутствует.
		if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
			if _, _, sess, vErr := svc.ValidateSession(r.Context(), c.Value); vErr == nil && sess != nil {
				_ = svc.Logout(r.Context(), sess.ID)
			}
		}
		ttl := cfg.Auth.SessionTTL
		if ttl <= 0 {
			ttl = 24 * time.Hour
		}
		res, err := svc.Login(r.Context(), core.LoginInput{
			TenantID: cfg.Auth.TenantDefault,
			Email:    string(req.Email),
			Password: req.Password,
		}, ttl)
		if err != nil {
			_ = audit.Log(r.Context(), core.AuditEntry{
				Action:    core.AuditAuthLoginFail,
				Outcome:   core.AuditOutcomeFailure,
				ActorType: core.AuditActorAnonymous,
				TenantID:  cfg.Auth.TenantDefault,
				Metadata:  map[string]any{"email": string(req.Email)},
			})
			if errors.Is(err, core.ErrUnauthorized) {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "login_failed")
			return
		}
		setSessionCookie(w, cfg, res.RawToken, res.Session.ExpiresAt)
		_ = audit.Log(r.Context(), core.AuditEntry{
			Action:    core.AuditAuthLoginSuccess,
			Outcome:   core.AuditOutcomeSuccess,
			ActorID:   res.User.ID,
			ActorType: core.AuditActorUser,
			TenantID:  res.User.TenantID,
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(oapigen.LoginResponse{
			CsrfToken: res.CSRFToken,
			UserId:    res.User.ID,
			Role:      oapigen.Role(res.User.Role),
			ExpiresAt: res.Session.ExpiresAt.UTC(),
		})
	}
}

// LogoutHandler handles POST /api/auth/logout. Idempotent.
func LogoutHandler(svc *core.AuthService, cfg *config.Config, audit *core.AuditService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var actorID = uuid.Nil
		var tenantID = uuid.Nil
		if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
			if u, _, sess, vErr := svc.ValidateSession(r.Context(), c.Value); vErr == nil && sess != nil {
				if u != nil {
					actorID = u.ID
					tenantID = u.TenantID
				}
				_ = svc.Logout(r.Context(), sess.ID)
			}
		}
		clearSessionCookie(w, cfg)
		_ = audit.Log(r.Context(), core.AuditEntry{
			Action:    core.AuditAuthLogout,
			Outcome:   core.AuditOutcomeSuccess,
			ActorID:   actorID,
			ActorType: core.AuditActorUser,
			TenantID:  tenantID,
		})
		w.WriteHeader(http.StatusOK)
	}
}

// CSRFHandler handles POST /api/auth/csrf — refresh CSRF-token.
func CSRFHandler(svc *core.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sess, ok := core.SessionFromContext(r.Context())
		if !ok || sess == nil {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		newCSRF, err := svc.RefreshCSRF(r.Context(), sess.ID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "csrf_failed")
			return
		}
		w.Header().Set(CSRFHeaderName, newCSRF)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"csrf_token": newCSRF})
	}
}

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

// _ keeps "context" import alive when only used implicitly.
var _ = context.Background
