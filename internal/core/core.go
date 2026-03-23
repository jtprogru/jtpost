package core

// PostStatus статус поста в жизненном цикле.
type PostStatus string

const (
	StatusIdea      PostStatus = "idea"
	StatusDraft     PostStatus = "draft"
	StatusReady     PostStatus = "ready"
	StatusScheduled PostStatus = "scheduled"
	StatusPublished PostStatus = "published"
)

// StatusOrder — все статусы в порядке жизненного цикла.
var StatusOrder = []PostStatus{
	StatusIdea,
	StatusDraft,
	StatusReady,
	StatusScheduled,
	StatusPublished,
}
