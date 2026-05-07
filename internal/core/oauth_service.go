package core

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OAuthService инкапсулирует OAuth flow: build authorize URL, обработка
// callback, account linking. Использует OAuthProvider для конкретных
// провайдеров (GitHub/Google/Yandex).
type OAuthService struct {
	providers       map[string]OAuthProvider
	users           UserRepository
	oauthAccounts   OAuthAccountRepository
	defaultTenantID uuid.UUID
	defaultRole     Role
	clock           Clock
}

// NewOAuthService создаёт OAuthService.
func NewOAuthService(
	providers map[string]OAuthProvider,
	users UserRepository,
	oauthAccounts OAuthAccountRepository,
	defaultTenantID uuid.UUID,
	defaultRole Role,
	clock Clock,
) *OAuthService {
	if clock == nil {
		clock = SystemClock{}
	}
	if defaultRole == "" {
		defaultRole = RoleAuthor
	}
	return &OAuthService{
		providers:       providers,
		users:           users,
		oauthAccounts:   oauthAccounts,
		defaultTenantID: defaultTenantID,
		defaultRole:     defaultRole,
		clock:           clock,
	}
}

// HasProvider проверяет регистрацию провайдера.
func (s *OAuthService) HasProvider(name string) bool {
	_, ok := s.providers[name]
	return ok
}

// BuildAuthorizeURL генерирует random state и возвращает provider's authorize URL.
func (s *OAuthService) BuildAuthorizeURL(provider string) (authorizeURL, state string, err error) {
	p, ok := s.providers[provider]
	if !ok {
		return "", "", fmt.Errorf("%w: oauth provider %q not configured", ErrConfigInvalid, provider)
	}
	state, err = generateOAuthState()
	if err != nil {
		return "", "", err
	}
	return p.AuthorizeURL(state), state, nil
}

// HandleCallback exchange code → token → user info → link/create user.
// Возвращает User для последующей выдачи session.
func (s *OAuthService) HandleCallback(ctx context.Context, provider, code string) (*User, error) {
	p, ok := s.providers[provider]
	if !ok {
		return nil, fmt.Errorf("%w: oauth provider %q not configured", ErrConfigInvalid, provider)
	}

	accessToken, err := p.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth exchange: %w", err)
	}
	info, err := p.FetchUserInfo(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("oauth user info: %w", err)
	}
	if info.Email == "" || info.ExternalID == "" {
		return nil, errors.Join(ErrValidation, errors.New("oauth provider returned incomplete user info"))
	}

	// 1. Уже существующий oauth_account — re-login.
	existing, err := s.oauthAccounts.GetByExternalID(ctx, provider, info.ExternalID)
	if err == nil && existing != nil {
		user, uErr := s.users.GetByID(ctx, existing.UserID)
		if uErr != nil {
			return nil, fmt.Errorf("oauth-linked user not found: %w", uErr)
		}
		return user, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	// 2. Существующий user с тем же email — link.
	user, err := s.users.GetByEmail(ctx, s.defaultTenantID, info.Email)
	if err == nil && user != nil {
		if err := s.createOAuthAccount(ctx, user.ID, provider, info); err != nil {
			return nil, err
		}
		return user, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	// 3. Новый user (OAuth-only).
	now := s.clock.Now().UTC()
	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate user uuid: %w", err)
	}
	newUser := &User{
		ID:           uid,
		TenantID:     s.defaultTenantID,
		Email:        info.Email,
		PasswordHash: "", // OAuth-only
		Role:         s.defaultRole,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("create oauth user: %w", err)
	}
	if err := s.createOAuthAccount(ctx, newUser.ID, provider, info); err != nil {
		return nil, err
	}
	return newUser, nil
}

func (s *OAuthService) createOAuthAccount(ctx context.Context, userID uuid.UUID, provider string, info *OAuthUserInfo) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate oauth_account uuid: %w", err)
	}
	a := &OAuthAccount{
		ID:         id,
		UserID:     userID,
		Provider:   provider,
		ExternalID: info.ExternalID,
		Email:      info.Email,
		CreatedAt:  s.clock.Now().UTC(),
	}
	if err := s.oauthAccounts.Create(ctx, a); err != nil {
		return fmt.Errorf("create oauth_account: %w", err)
	}
	return nil
}

func generateOAuthState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// _ keep time-import alive (used implicitly via clock).
var _ = time.Now
