// Package httpapi предоставляет HTTP сервер для API jtpost.
package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/jtprogru/jtpost/internal/logger"
)

// TenantFromConfigMiddleware — middleware-заглушка под F4. Извлекает tenant_default
// и author_default из конфига и кладёт их в context. Реальный auth — в F4.
func TenantFromConfigMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if cfg != nil {
				ctx = core.WithTenant(ctx, cfg.Auth.TenantDefault)
				ctx = core.WithAuthor(ctx, cfg.Auth.AuthorDefault)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// responseWriter обёртка над http.ResponseWriter для перехвата статуса.
type responseWriter struct {
	http.ResponseWriter

	statusCode int
	written    int64
}

// newResponseWriter создаёт новый responseWriter.
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // по умолчанию 200
	}
}

// WriteHeader перехватывает код статуса.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write перехватывает запись данных.
func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// StatusCode возвращает код статуса.
func (rw *responseWriter) StatusCode() int {
	return rw.statusCode
}

// Written возвращает количество записанных байт.
func (rw *responseWriter) Written() int64 {
	return rw.written
}

// LoggingMiddleware middleware для логирования HTTP запросов.
func LoggingMiddleware(log *logger.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Создаём обёртку для перехвата статуса
		rw := newResponseWriter(w)

		// Вызываем следующий обработчик
		next.ServeHTTP(rw, r)

		// Логируем запрос
		duration := time.Since(start)
		log.Info(
			"HTTP %s %s %d %d %v",
			r.Method,
			r.URL.Path,
			rw.StatusCode(),
			rw.Written(),
			duration.Round(time.Millisecond),
		)
	})
}

// BearerTokenMiddleware валидирует PAT из заголовка Authorization и при success
// populates ctx (User/TenantID/Author/Role/AuthSource=bearer). На failure —
// soft-pass (next вызывается без populated ctx; финальный RequireAuthMiddleware
// решает 401). Это позволяет составлять цепочку Bearer → Session → RequireAuth.
func BearerTokenMiddleware(svc *core.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHdr := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(authHdr, prefix) {
				next.ServeHTTP(w, r)
				return
			}
			raw := strings.TrimSpace(authHdr[len(prefix):])
			if raw == "" {
				next.ServeHTTP(w, r)
				return
			}
			user, role, err := svc.ValidateToken(r.Context(), raw)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := r.Context()
			ctx = core.WithUser(ctx, user)
			ctx = core.WithTenant(ctx, user.TenantID)
			ctx = core.WithAuthor(ctx, user.ID)
			ctx = core.WithRole(ctx, role)
			ctx = core.WithAuthSource(ctx, core.AuthSourceBearer)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionCookieName — имя cookie с session-token.
const SessionCookieName = "jtpost_session"

// CSRFHeaderName — имя header'а для double-submit CSRF token.
// Используется через canonical-form `X-Csrf-Token` (http.Header автоматически
// приводит к canonical при Get/Set).
const CSRFHeaderName = "X-Csrf-Token"

// SessionMiddleware валидирует session-cookie. Если Bearer уже установил
// ctx.User — пропускает (Bearer wins, REQ-4.3). При success — populates
// ctx (User/Tenant/Author/Role/Session/AuthSource=session). На failure —
// soft-pass (как BearerTokenMiddleware).
func SessionMiddleware(svc *core.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := core.UserFromContext(r.Context()); ok {
				next.ServeHTTP(w, r)
				return
			}
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			user, role, sess, err := svc.ValidateSession(r.Context(), cookie.Value)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := r.Context()
			ctx = core.WithUser(ctx, user)
			ctx = core.WithTenant(ctx, user.TenantID)
			ctx = core.WithAuthor(ctx, user.ID)
			ctx = core.WithRole(ctx, role)
			ctx = core.WithSession(ctx, sess)
			ctx = core.WithAuthSource(ctx, core.AuthSourceSession)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// csrfSkipPaths — endpoints, для которых CSRF не проверяется.
//
//nolint:gochecknoglobals
var csrfSkipPaths = map[string]struct{}{
	"/api/auth/login":  {},
	"/api/auth/csrf":   {},
	"/api/auth/logout": {},
	"/ui/login":        {},
	"/ui/logout":       {},
}

// csrfSkipPrefixes — пути префикс-skip для CSRF (OAuth callback).
//
//nolint:gochecknoglobals
var csrfSkipPrefixes = []string{
	"/api/auth/oauth/",
}

// CSRFMiddleware применяет double-submit-pattern для state-changing запросов
// с auth.source=session. Bearer-only клиенты — CSRF-immune (REQ-5.2).
func CSRFMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			if _, skip := csrfSkipPaths[r.URL.Path]; skip {
				next.ServeHTTP(w, r)
				return
			}
			for _, prefix := range csrfSkipPrefixes {
				if strings.HasPrefix(r.URL.Path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}
			src, _ := core.AuthSourceFromContext(r.Context())
			if src != core.AuthSourceSession {
				next.ServeHTTP(w, r)
				return
			}
			sess, ok := core.SessionFromContext(r.Context())
			if !ok || sess == nil {
				writeCSRFInvalid(w)
				return
			}
			provided := r.Header.Get(CSRFHeaderName)
			if subtle.ConstantTimeCompare([]byte(provided), []byte(sess.CSRFToken)) != 1 {
				writeCSRFInvalid(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requireAuthSkipPaths — endpoints, проходящие БЕЗ auth (login).
//
//nolint:gochecknoglobals
var requireAuthSkipPaths = map[string]struct{}{
	"/api/auth/login":  {},
	"/api/auth/logout": {},
}

// requireAuthSkipPrefixes — path-prefix, проходящие БЕЗ auth (OAuth-flow,
// UI login/static — UI handlers сами решают что показывать анонимам).
//
//nolint:gochecknoglobals
var requireAuthSkipPrefixes = []string{
	"/api/auth/oauth/",
	"/ui/login",
	"/ui/static/",
}

// RequireAuthMiddleware — финальный gate. Если в ctx нет User — 401.
// Skip-list: login, logout (logout идемпотентен), /api/auth/oauth/* (OAuth flow).
func RequireAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, skip := requireAuthSkipPaths[r.URL.Path]; skip {
				next.ServeHTTP(w, r)
				return
			}
			for _, prefix := range requireAuthSkipPrefixes {
				if strings.HasPrefix(r.URL.Path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}
			if _, ok := core.UserFromContext(r.Context()); !ok {
				writeUnauthorized(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeCSRFInvalid(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "csrf_invalid"})
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}

// AuditContextMiddleware кладёт в ctx IP клиента и User-Agent — необходимо
// AuditService.Log для заполнения полей. trustProxy=true — берёт IP из
// X-Forwarded-For/X-Real-IP (только за trusted reverse proxy).
func AuditContextMiddleware(trustProxy bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := core.WithAuditContext(r.Context(), core.AuditContext{
				IP:        clientIP(r, trustProxy),
				UserAgent: r.UserAgent(),
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RecoveryMiddleware middleware для восстановления после паник.
func RecoveryMiddleware(log *logger.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("PANIC recovered: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
