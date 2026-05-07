package core

import (
	"context"
	"time"
)

// HistoryEntry — одна запись в истории изменений поста.
type HistoryEntry struct {
	Hash      string    // полный commit hash
	ShortHash string    // первые 8 символов
	Author    string    // имя коммитера
	Email     string    // e-mail коммитера
	Message   string    // commit message (одна строка, subject)
	At        time.Time // время коммита
}

// HistoryProvider — опциональный capability для PostRepository, возвращающий
// историю изменений поста. Реализуется storage backend'ами, у которых есть
// версионирование (gitrepo). Storage без versioning (fs/sqlite/postgres)
// этот интерфейс не реализует — webui-handler ответит 503.
type HistoryProvider interface {
	History(ctx context.Context, post *Post, limit int) ([]HistoryEntry, error)
}
