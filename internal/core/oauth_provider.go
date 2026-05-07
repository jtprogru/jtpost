package core

import "context"

// OAuthProvider — абстракция конкретного OAuth2-провайдера.
// Используется OAuthService (F4c) для PKCE-флоу: построение authorize URL,
// обмен code → access_token и получение информации о пользователе.
type OAuthProvider interface {
	// Name возвращает строковый идентификатор провайдера (например, "github").
	// Используется как значение колонки oauth_accounts.provider.
	Name() string

	// AuthorizeURL возвращает URL для редиректа пользователя на страницу
	// согласия провайдера. state — anti-CSRF токен.
	AuthorizeURL(state string) string

	// Exchange обменивает authorization code на access_token.
	Exchange(ctx context.Context, code string) (accessToken string, err error)

	// FetchUserInfo получает профиль пользователя у провайдера по access_token.
	// Возвращает ErrValidation (joined), если необходимые поля недоступны
	// (например, нет primary verified email у GitHub).
	FetchUserInfo(ctx context.Context, accessToken string) (*OAuthUserInfo, error)
}
