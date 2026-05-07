package core

import (
	"time"

	"github.com/google/uuid"
)

// OAuthAccount — связь между core.User и внешним OAuth-провайдером.
// Уникальность гарантируется парой (Provider, ExternalID).
type OAuthAccount struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Provider   string
	ExternalID string
	Email      string
	CreatedAt  time.Time
}

// OAuthUserInfo — нормализованная информация, возвращаемая OAuthProvider.FetchUserInfo.
// DisplayName опционален.
type OAuthUserInfo struct {
	ExternalID  string
	Email       string
	DisplayName string
}
