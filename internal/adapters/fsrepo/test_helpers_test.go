package fsrepo

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
)

// Фиксированные UUID для использования в тестах.
//
//nolint:gochecknoglobals // shared test constants
var (
	testTenant1 = uuid.MustParse("01900000-0000-7000-8000-000000000001")
	testTenant2 = uuid.MustParse("01900000-0000-7000-8000-000000000002")
	testAuthor1 = uuid.MustParse("01900000-0000-7000-8000-00000000000A")
	testAuthor2 = uuid.MustParse("01900000-0000-7000-8000-00000000000B")
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

// newTestPost создаёт полностью валидный пост с заполненными обязательными полями.
// Tenant и Author по умолчанию — testTenant1/testAuthor1; меняются опциональными override-функциями.
func newTestPost(t *testing.T, opts ...func(*core.Post)) *core.Post {
	t.Helper()
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	p := &core.Post{
		ID:        mustParsePostID("post-default"),
		TenantID:  testTenant1,
		AuthorID:  testAuthor1,
		Title:     "Default Test Post",
		Slug:      "default-test-post",
		Status:    core.StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
		Tags:      []string{},
		Content:   "Test content",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}
