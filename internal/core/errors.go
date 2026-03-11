package core

import "errors"

// Ошибки репозитория.
var (
	ErrNotFound      = errors.New("post not found")
	ErrAlreadyExists = errors.New("post already exists")
)

// Ошибки валидации.
var (
	ErrEmptyTitle     = errors.New("title cannot be empty")
	ErrEmptySlug      = errors.New("slug cannot be empty")
	ErrInvalidStatus  = errors.New("invalid status transition")
	ErrInvalidPlatform = errors.New("invalid platform")
	ErrValidation     = errors.New("validation error")
)

// Ошибки публикации.
var (
	ErrPublishFailed     = errors.New("failed to publish")
	ErrUnknownPlatform   = errors.New("unknown platform")
	ErrNotReadyToPublish = errors.New("post is not ready to publish")
)

// Ошибки конфигурации.
var (
	ErrConfigNotFound    = errors.New("config file not found")
	ErrConfigInvalid     = errors.New("config file is invalid")
	ErrPostsDirNotFound  = errors.New("posts directory not found")
)

// IsStatusTransitionValid проверяет допустимость перехода между статусами.
func IsStatusTransitionValid(from, to PostStatus) bool {
	statusIndex := make(map[PostStatus]int)
	for i, s := range StatusOrder {
		statusIndex[s] = i
	}

	fromIdx, fromOk := statusIndex[from]
	toIdx, toOk := statusIndex[to]

	if !fromOk || !toOk {
		return false
	}

	// Можно переходить только вперёд по статусам
	return toIdx > fromIdx
}
