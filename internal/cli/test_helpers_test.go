package cli

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/core"
)

// Фиксированные UUID для использования в тестах.
var (
	testTenant1 = uuid.MustParse("01900000-0000-7000-8000-000000000001")
	testAuthor1 = uuid.MustParse("01900000-0000-7000-8000-00000000000A")
)

// mustParsePostID парсит строку в PostID или генерирует UUID v5 из строки при ошибке.
// Используется только в тестах.
func mustParsePostID(s string) core.PostID {
	id, err := core.ParsePostID(s)
	if err != nil {
		u := uuid.NewSHA1(uuid.NameSpaceOID, []byte(s))
		return core.PostID(u)
	}
	return id
}

// fillTestPostDefaults дополняет пост обязательными полями (TenantID, AuthorID,
// CreatedAt, UpdatedAt, Revision), если они не заполнены. Используется только в тестах.
func fillTestPostDefaults(p *core.Post) {
	if p.TenantID == uuid.Nil {
		p.TenantID = testTenant1
	}
	if p.AuthorID == uuid.Nil {
		p.AuthorID = testAuthor1
	}
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	}
	if p.Revision == 0 {
		p.Revision = 1
	}
}

// writeTestConfig сохраняет тестовый конфиг с заполненными tenant/author по умолчанию
// и возвращает путь к файлу.
func writeTestConfig(t *testing.T, dir, postsDir string) string {
	t.Helper()
	cfg := config.NewDefaultConfig()
	cfg.PostsDir = postsDir
	cfg.Auth.TenantDefault = testTenant1
	cfg.Auth.AuthorDefault = testAuthor1
	configPath := filepath.Join(dir, ".jtpost.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}
	return configPath
}
