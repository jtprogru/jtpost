package core

import "slices"

// PostStatus статус поста в жизненном цикле.
type PostStatus string

// Допустимые статусы поста.
const (
	StatusIdea      PostStatus = "idea"
	StatusDraft     PostStatus = "draft"
	StatusReady     PostStatus = "ready"
	StatusScheduled PostStatus = "scheduled"
	StatusPublished PostStatus = "published"
	StatusArchived  PostStatus = "archived"
	StatusFailed    PostStatus = "failed"
)

// allowedTransitions определяет допустимые переходы между статусами.
//
// Переходы (всего 10):
//
//	из idea       в: draft
//	из draft      в: ready
//	из ready      в: scheduled, published
//	из scheduled  в: published, ready, failed
//	из failed     в: ready, archived
//	из published  в: archived
//	из archived   в: (нет переходов)
var allowedTransitions = map[PostStatus][]PostStatus{
	StatusIdea:      {StatusDraft},
	StatusDraft:     {StatusReady},
	StatusReady:     {StatusScheduled, StatusPublished},
	StatusScheduled: {StatusPublished, StatusReady, StatusFailed},
	StatusFailed:    {StatusReady, StatusArchived},
	StatusPublished: {StatusArchived},
	StatusArchived:  {},
}

// IsTransitionAllowed проверяет, разрешён ли переход from→to согласно
// allowedTransitions.
func IsTransitionAllowed(from, to PostStatus) bool {
	targets, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	return slices.Contains(targets, to)
}

// AllStatuses возвращает все статусы в порядке жизненного цикла.
func AllStatuses() []PostStatus {
	return []PostStatus{
		StatusIdea,
		StatusDraft,
		StatusReady,
		StatusScheduled,
		StatusPublished,
		StatusFailed,
		StatusArchived,
	}
}
