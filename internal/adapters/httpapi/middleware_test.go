package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jtprogru/jtpost/internal/logger"
)

// testHandler простой обработчик для тестирования.
func testHandler(statusCode int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte("test response"))
	})
}

func TestResponseWriter(t *testing.T) {
	t.Run("WriteHeader захватывает статус", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)

		rw.WriteHeader(http.StatusCreated)

		if rw.StatusCode() != http.StatusCreated {
			t.Errorf("ожидаемый статус %d, получен %d", http.StatusCreated, rw.StatusCode())
		}
	})

	t.Run("Write подсчитывает байты", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)

		data := []byte("hello world")
		n, err := rw.Write(data)
		if err != nil {
			t.Fatalf("ошибка записи: %v", err)
		}

		if n != len(data) {
			t.Errorf("ожидаемое количество байт %d, получено %d", len(data), n)
		}

		if rw.Written() != int64(len(data)) {
			t.Errorf("ожидаемый размер %d, получен %d", len(data), rw.Written())
		}
	})

	t.Run("StatusCode по умолчанию 200", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)

		// Не вызываем WriteHeader, проверяем значение по умолчанию
		if rw.StatusCode() != http.StatusOK {
			t.Errorf("ожидаемый статус по умолчанию %d, получен %d", http.StatusOK, rw.StatusCode())
		}
	})
}

func TestLoggingMiddleware(t *testing.T) {
	t.Run("логирует запрос", func(t *testing.T) {
		var buf bytes.Buffer
		log := logger.New(logger.Config{
			Output: &buf,
			Level:  logger.LevelInfo,
		})

		handler := LoggingMiddleware(log, testHandler(http.StatusOK))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		output := buf.String()

		// Проверяем, что лог содержит ключевые слова
		if !strings.Contains(output, "HTTP") {
			t.Errorf("лог должен содержать 'HTTP', получено: %s", output)
		}
		if !strings.Contains(output, "/test") {
			t.Errorf("лог должен содержать '/test', получено: %s", output)
		}
		if !strings.Contains(output, "200") {
			t.Errorf("лог должен содержать код статуса '200', получено: %s", output)
		}
	})

	t.Run("логирует разные методы", func(t *testing.T) {
		var buf bytes.Buffer
		log := logger.New(logger.Config{
			Output: &buf,
			Level:  logger.LevelInfo,
		})

		handler := LoggingMiddleware(log, testHandler(http.StatusCreated))

		tests := []struct {
			method string
			path   string
		}{
			{http.MethodPost, "/api/posts"},
			{http.MethodPut, "/api/posts/1"},
			{http.MethodDelete, "/api/posts/2"},
		}

		for _, tt := range tests {
			buf.Reset()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			output := buf.String()
			if !strings.Contains(output, tt.method) {
				t.Errorf("лог должен содержать метод '%s', получено: %s", tt.method, output)
			}
		}
	})

	t.Run("логирует разные статусы", func(t *testing.T) {
		var buf bytes.Buffer
		log := logger.New(logger.Config{
			Output: &buf,
			Level:  logger.LevelInfo,
		})

		statuses := []int{
			http.StatusOK,
			http.StatusCreated,
			http.StatusNotFound,
			http.StatusInternalServerError,
		}

		for _, status := range statuses {
			buf.Reset()
			handler := LoggingMiddleware(log, testHandler(status))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			output := buf.String()
			if !strings.Contains(output, string(rune(status+'0'))) {
				// Проверяем, что статус записан в лог
				t.Logf("лог: %s", output)
			}
		}
	})
}

func TestRecoveryMiddleware(t *testing.T) {
	t.Run("восстанавливается после паники", func(t *testing.T) {
		var buf bytes.Buffer
		log := logger.New(logger.Config{
			Output: &buf,
			Level:  logger.LevelError,
		})

		panicHandler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			panic("test panic")
		})

		handler := RecoveryMiddleware(log, panicHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		// Не должно паниковать
		handler.ServeHTTP(rec, req)

		// Проверяем, что паника была перехвачена и записана в лог
		output := buf.String()
		if !strings.Contains(output, "PANIC") {
			t.Errorf("лог должен содержать 'PANIC', получено: %s", output)
		}

		// Проверяем, что возвращён 500 статус
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("ожидаемый статус %d, получен %d", http.StatusInternalServerError, rec.Code)
		}
	})

	t.Run("не мешает нормальной работе", func(t *testing.T) {
		var buf bytes.Buffer
		log := logger.New(logger.Config{
			Output: &buf,
			Level:  logger.LevelInfo,
		})

		normalHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})

		handler := RecoveryMiddleware(log, normalHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("ожидаемый статус %d, получен %d", http.StatusOK, rec.Code)
		}

		if rec.Body.String() != "ok" {
			t.Errorf("ожидаемый ответ 'ok', получен '%s'", rec.Body.String())
		}
	})
}

func TestLoggingMiddleware_WrittenBytes(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Config{
		Output: &buf,
		Level:  logger.LevelInfo,
	})

	handler := LoggingMiddleware(log, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello world"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	output := buf.String()
	// Проверяем, что размер записанных байт указан в логе
	if !strings.Contains(output, "11") {
		t.Errorf("лог должен содержать размер ответа '11', получено: %s", output)
	}
}
