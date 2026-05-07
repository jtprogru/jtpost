package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jtprogru/jtpost/internal/adapters/httpapi/oapigen"
)

// TestAPIV1_LoginAliasWorks — POST /api/v1/auth/login работает идентично
// /api/auth/login (sanity check для v1 alias mechanism).
func TestAPIV1_LoginAliasWorks(t *testing.T) {
	svc, cfg, _ := setupHandler(t)
	server := NewServerWithConfig(ServerConfig{
		AuthService: svc,
		Config:      cfg,
	})

	body, _ := json.Marshal(oapigen.LoginRequest{
		Email:    "owner@example.com",
		Password: "password123",
	})

	for _, path := range []string{"/api/auth/login", "/api/v1/auth/login"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("path=%s status=%d, want 200", path, rec.Code)
			}
			// Проверим что cookie установлен.
			found := false
			for _, c := range rec.Result().Cookies() {
				if c.Name == SessionCookieName && strings.HasPrefix(c.Value, "jts_") {
					found = true
				}
			}
			if !found {
				t.Errorf("path=%s: session cookie не выставлен", path)
			}
		})
	}
}
