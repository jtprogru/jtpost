package fsrepo

import (
	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
)

// mustParsePostID парсит строку в PostID или паникует при ошибке.
// Используется только в тестах.
func mustParsePostID(s string) core.PostID {
	id, err := core.ParsePostID(s)
	if err != nil {
		// Fallback: генерируем UUID из строки используя SHA1
		u := uuid.NewSHA1(uuid.NameSpaceOID, []byte(s))
		return core.PostID(u)
	}
	return id
}
