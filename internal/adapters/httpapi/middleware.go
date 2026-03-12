// Package httpapi предоставляет HTTP сервер для API jtpost.
package httpapi

import (
	"net/http"
	"time"

	"github.com/jtprogru/jtpost/internal/logger"
)

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
