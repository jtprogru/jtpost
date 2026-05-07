package core

import "errors"

// Ошибки репозитория.
var (
	ErrNotFound      = errors.New("post not found")
	ErrAlreadyExists = errors.New("post already exists")
	// ErrTenantMismatch возвращается при попытке изменить TenantID существующего
	// поста или при операции записи с несовпадением tenant scope.
	ErrTenantMismatch = errors.New("tenant mismatch")
)

// Ошибки валидации.
var (
	ErrEmptyTitle    = errors.New("title cannot be empty")
	ErrEmptySlug     = errors.New("slug cannot be empty")
	ErrInvalidStatus = errors.New("invalid status transition")
	// ErrInvalidTransition возвращается, когда переход между статусами не разрешён
	// согласно allowedTransitions. Отделена от ErrInvalidStatus, которая
	// сигнализирует о неизвестном значении статуса в принципе.
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrValidation        = errors.New("validation error")
)

// Ошибки публикации.
var (
	ErrPublishFailed     = errors.New("failed to publish")
	ErrNotReadyToPublish = errors.New("post is not ready to publish")
	// ErrPublishRetryExhausted возвращается worker'ом после исчерпания всех
	// попыток публикации.
	ErrPublishRetryExhausted = errors.New("publish retry attempts exhausted")
)

// Ошибки конфигурации.
var (
	ErrConfigNotFound   = errors.New("config file not found")
	ErrConfigInvalid    = errors.New("config file is invalid")
	ErrPostsDirNotFound = errors.New("posts directory not found")
)

// IsStatusTransitionValid проверяет допустимость перехода между статусами.
//
// Deprecated: используйте IsTransitionAllowed.
func IsStatusTransitionValid(from, to PostStatus) bool {
	return IsTransitionAllowed(from, to)
}
