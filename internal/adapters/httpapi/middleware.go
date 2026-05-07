// Package httpapi предоставляет HTTP сервер для API jtpost.
package httpapi

import (
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

// BearerTokenMiddleware валидирует PAT из заголовка Authorization и кладёт
// User/TenantID/Role в context. При отсутствии или невалидном токене —
// возвращает HTTP 401 без передачи запроса handler'у.
func BearerTokenMiddleware(svc *core.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHdr := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(authHdr, prefix) {
				writeUnauthorized(w)
				return
			}
			raw := strings.TrimSpace(authHdr[len(prefix):])
			if raw == "" {
				writeUnauthorized(w)
				return
			}
			user, role, err := svc.ValidateToken(r.Context(), raw)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			ctx := r.Context()
			ctx = core.WithUser(ctx, user)
			ctx = core.WithTenant(ctx, user.TenantID)
			ctx = core.WithAuthor(ctx, user.ID)
			ctx = core.WithRole(ctx, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
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
