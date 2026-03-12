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
