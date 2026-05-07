package core

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"slices"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	tokenFormatPrefix = "jtpat_"
	prefixLen         = 8
	secretLen         = 24
	tokenSecretCost   = 6 // hardcoded — secret имеет ~140-bit entropy, не нужен высокий cost
	tokenRetryLimit   = 3 // на UNIQUE(prefix) collision

	tokenAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var tokenFormatRegex = regexp.MustCompile(`^jtpat_[A-Za-z0-9]{8}_[A-Za-z0-9]{24}$`) //nolint:gochecknoglobals

// AuthService инкапсулирует auth-логику: пользователи, PAT, RBAC.
type AuthService struct {
	users      UserRepository
	tokens     TokenRepository
	bcryptCost int
	clock      Clock
}

// NewAuthService создаёт AuthService.
func NewAuthService(users UserRepository, tokens TokenRepository, bcryptCost int, clock Clock) *AuthService {
	if clock == nil {
		clock = SystemClock{}
	}
	return &AuthService{
		users:      users,
		tokens:     tokens,
		bcryptCost: bcryptCost,
		clock:      clock,
	}
}

// CreateUser создаёт нового пользователя. Email и password валидируются;
// password хешируется через bcrypt с настроенным cost.
func (s *AuthService) CreateUser(ctx context.Context, in CreateUserInput) (*User, error) {
	if in.Email == "" || in.Password == "" {
		return nil, ErrValidation
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		return nil, fmt.Errorf("%w: invalid email", ErrValidation)
	}
	if len(in.Password) < 8 {
		return nil, fmt.Errorf("%w: password must be at least 8 characters", ErrValidation)
	}
	if in.Role == "" {
		in.Role = RoleAuthor
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	now := s.clock.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}
	user := &User{
		ID:           id,
		TenantID:     in.TenantID,
		Email:        in.Email,
		PasswordHash: string(hash),
		Role:         in.Role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// VerifyPassword находит пользователя по (tenant, email) и сверяет password.
// Любая ошибка (не найден / не совпало) — ErrUnauthorized, чтобы не утечь
// существование email.
func (s *AuthService) VerifyPassword(ctx context.Context, tenantID uuid.UUID, email, password string) (*User, error) {
	user, err := s.users.GetByEmail(ctx, tenantID, email)
	if err != nil {
		return nil, ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrUnauthorized
	}
	return user, nil
}

// IssueToken создаёт новый PAT. Возвращает IssuedToken где Raw показывается
// caller'у один раз.
func (s *AuthService) IssueToken(ctx context.Context, userID uuid.UUID, name string, expiresIn *time.Duration) (*IssuedToken, error) {
	now := s.clock.Now().UTC()
	var expiresAt *time.Time
	if expiresIn != nil {
		t := now.Add(*expiresIn)
		expiresAt = &t
	}

	for range tokenRetryLimit {
		prefix, secret, err := genTokenParts()
		if err != nil {
			return nil, fmt.Errorf("generate token parts: %w", err)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(secret), tokenSecretCost)
		if err != nil {
			return nil, fmt.Errorf("hash secret: %w", err)
		}
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate token uuid: %w", err)
		}
		tok := &APIToken{
			ID:         id,
			UserID:     userID,
			Prefix:     prefix,
			SecretHash: string(hash),
			Name:       name,
			CreatedAt:  now,
			ExpiresAt:  expiresAt,
		}
		if err := s.tokens.Create(ctx, tok); err != nil {
			if errors.Is(err, ErrAlreadyExists) {
				continue // retry на prefix collision
			}
			return nil, err
		}
		return &IssuedToken{
			Raw:   tokenFormatPrefix + prefix + "_" + secret,
			Token: tok,
		}, nil
	}
	return nil, fmt.Errorf("failed to issue token after %d retries", tokenRetryLimit)
}

// ValidateToken парсит raw, ищет в БД и проверяет hash + expiry.
func (s *AuthService) ValidateToken(ctx context.Context, raw string) (*User, Role, error) {
	if !tokenFormatRegex.MatchString(raw) {
		return nil, "", ErrUnauthorized
	}
	rest := raw[len(tokenFormatPrefix):]
	prefix := rest[:prefixLen]
	secret := rest[prefixLen+1:] // skip "_"

	tok, err := s.tokens.GetByPrefix(ctx, prefix)
	if err != nil {
		return nil, "", ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(tok.SecretHash), []byte(secret)); err != nil {
		return nil, "", ErrUnauthorized
	}
	if tok.ExpiresAt != nil && s.clock.Now().After(*tok.ExpiresAt) {
		return nil, "", ErrUnauthorized
	}
	user, err := s.users.GetByID(ctx, tok.UserID)
	if err != nil {
		return nil, "", ErrUnauthorized
	}
	// async update LastUsedAt — не блокировать запрос
	go func(tokID uuid.UUID, t time.Time) {
		_ = s.tokens.UpdateLastUsedAt(context.Background(), tokID, t)
	}(tok.ID, s.clock.Now().UTC())
	return user, user.Role, nil
}

// RevokeToken удаляет токен (hard delete).
func (s *AuthService) RevokeToken(ctx context.Context, tokenID uuid.UUID) error {
	return s.tokens.Delete(ctx, tokenID)
}

// AuthorizeOperation проверяет, что Role из ctx имеет указанный permission.
func (s *AuthService) AuthorizeOperation(ctx context.Context, perm Permission) error {
	role, ok := RoleFromContext(ctx)
	if !ok {
		return ErrUnauthorized
	}
	if slices.Contains(RolePermissions(role), perm) {
		return nil
	}
	return ErrForbidden
}

// genTokenParts генерирует random prefix (8 chars) и secret (24 chars) из
// crypto/rand над tokenAlphabet.
func genTokenParts() (prefix, secret string, err error) {
	prefix, err = randomString(prefixLen)
	if err != nil {
		return "", "", err
	}
	secret, err = randomString(secretLen)
	if err != nil {
		return "", "", err
	}
	return prefix, secret, nil
}

func randomString(n int) (string, error) {
	out := make([]byte, n)
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	alphaLen := byte(len(tokenAlphabet))
	for i := range n {
		out[i] = tokenAlphabet[buf[i]%alphaLen]
	}
	return string(out), nil
}
