package core

import "context"

// PostRepository интерфейс для хранения и извлечения постов.
type PostRepository interface {
	// GetByID возвращает пост по идентификатору.
	GetByID(ctx context.Context, id PostID) (*Post, error)
	// GetBySlug возвращает пост по slug.
	GetBySlug(ctx context.Context, slug string) (*Post, error)
	// List возвращает список постов с применением фильтров.
	List(ctx context.Context, filter PostFilter) ([]*Post, error)
	// Create создаёт новый пост.
	Create(ctx context.Context, post *Post) error
	// Update обновляет существующий пост.
	Update(ctx context.Context, post *Post) error
	// Delete удаляет пост.
	Delete(ctx context.Context, id PostID) error
}

// TransactionalRepository интерфейс для поддержки транзакций.
type TransactionalRepository interface {
	PostRepository
	// BeginTx начинает транзакцию и возвращает новый репозиторий в контексте транзакции.
	BeginTx(ctx context.Context) (context.Context, func() error, error)
}

// MigratableRepository интерфейс для поддержки миграции данных.
type MigratableRepository interface {
	PostRepository
	// ImportPosts импортирует посты из другого репозитория.
	ImportPosts(ctx context.Context, posts []*Post) error
	// Count возвращает количество постов в хранилище.
	Count(ctx context.Context) (int64, error)
}
