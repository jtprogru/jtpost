package core

import "context"

// Publisher интерфейс для публикации поста на платформу.
type Publisher interface {
	// Platform возвращает платформу, которую поддерживает publisher.
	Platform() Platform
	// Publish публикует пост и возвращает обновлённую версию с ExternalLinks.
	Publish(ctx context.Context, post *Post) (*Post, error)
}
