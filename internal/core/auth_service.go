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

	sessionFormatPrefix = "jts_"
	sessionSecretLen    = 32
	sessionSecretCost   = 4 // entropy 256-bit; bcrypt лишь защищает от leak БД
	csrfTokenLen        = 32

	tokenAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

//nolint:gochecknoglobals
var sessionFormatRegex = regexp.MustCompile(`^jts_[A-Za-z0-9]{8}_[A-Za-z0-9]{32}$`)

var tokenFormatRegex = regexp.MustCompile(`^jtpat_[A-Za-z0-9]{8}_[A-Za-z0-9]{24}$`) //nolint:gochecknoglobals

// AuthService инкапсулирует auth-логику: пользователи, PAT, sessions, RBAC.
type AuthService struct {
	users    UserRepository
	tokens   TokenRepository
	sessions SessionRepository
	hasher   PasswordHasher
	clock    Clock
}

// NewAuthService создаёт AuthService. sessions может быть nil. hasher по
// умолчанию `NewMultiHasher` если nil.
func NewAuthService(
	users UserRepository,
	tokens TokenRepository,
	sessions SessionRepository,
	hasher PasswordHasher,
	clock Clock,
) *AuthService {
	if clock == nil {
		clock = SystemClock{}
	}
	if hasher == nil {
		hasher = NewMultiHasher()
	}
	return &AuthService{
		users:    users,
		tokens:   tokens,
		sessions: sessions,
		hasher:   hasher,
		clock:    clock,
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
	hash, err := s.hasher.Hash(in.Password)
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
		PasswordHash: hash,
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
// Любая ошибка (не найден / не совпало / OAuth-only user) — ErrUnauthorized.
func (s *AuthService) VerifyPassword(ctx context.Context, tenantID uuid.UUID, email, password string) (*User, error) {
	user, err := s.users.GetByEmail(ctx, tenantID, email)
	if err != nil {
		return nil, ErrUnauthorized
	}
	if user.PasswordHash == "" {
		// OAuth-only user — нельзя логиниться через password.
		return nil, ErrUnauthorized
	}
	if err := s.hasher.Verify(user.PasswordHash, password); err != nil {
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

// Login верифицирует email+password и создаёт session. Возвращает LoginResult
// с .RawToken (cookie value, plaintext, показывается клиенту один раз).
// Token format: "jts_<8 prefix>_<32 secret>" — pattern зеркалит PAT для
// O(1) lookup по prefix.
func (s *AuthService) Login(ctx context.Context, in LoginInput, ttl time.Duration) (*LoginResult, error) {
	if s.sessions == nil {
		return nil, fmt.Errorf("sessions repository not configured")
	}
	user, err := s.VerifyPassword(ctx, in.TenantID, in.Email, in.Password)
	if err != nil {
		return nil, err
	}
	// Auto-rehash legacy bcrypt passwords в Argon2id silently.
	if s.hasher.NeedsRehash(user.PasswordHash) {
		go func(u User, password string) {
			newHash, hErr := s.hasher.Hash(password)
			if hErr != nil {
				return
			}
			u.PasswordHash = newHash
			u.UpdatedAt = s.clock.Now().UTC()
			_ = s.users.Update(context.Background(), &u) //nolint:contextcheck
		}(*user, in.Password)
	}
	return s.issueSessionForUserAt(ctx, user, ttl)
}

// IssueSessionForUser создаёт session для уже-аутентифицированного user
// (например после OAuth callback). Не делает password verify.
func (s *AuthService) IssueSessionForUser(ctx context.Context, user *User, ttl time.Duration) (*LoginResult, error) {
	if user == nil {
		return nil, ErrUnauthorized
	}
	return s.issueSessionForUserAt(ctx, user, ttl)
}

// issueSessionForUserAt — общая логика создания session.
func (s *AuthService) issueSessionForUserAt(ctx context.Context, user *User, ttl time.Duration) (*LoginResult, error) {
	if s.sessions == nil {
		return nil, fmt.Errorf("sessions repository not configured")
	}
	now := s.clock.Now().UTC()
	expiresAt := now.Add(ttl)

	csrf, err := randomString(csrfTokenLen)
	if err != nil {
		return nil, fmt.Errorf("generate csrf: %w", err)
	}

	for range tokenRetryLimit {
		prefix, err := randomString(prefixLen)
		if err != nil {
			return nil, fmt.Errorf("generate session prefix: %w", err)
		}
		secret, err := randomString(sessionSecretLen)
		if err != nil {
			return nil, fmt.Errorf("generate session secret: %w", err)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(secret), sessionSecretCost)
		if err != nil {
			return nil, fmt.Errorf("hash session secret: %w", err)
		}
		sid, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate session uuid: %w", err)
		}
		sess := &Session{
			ID:         sid,
			UserID:     user.ID,
			Prefix:     prefix,
			SecretHash: string(hash),
			CSRFToken:  csrf,
			CreatedAt:  now,
			ExpiresAt:  expiresAt,
		}
		if err := s.sessions.Create(ctx, sess); err != nil {
			if errors.Is(err, ErrAlreadyExists) {
				continue
			}
			return nil, err
		}
		return &LoginResult{
			RawToken:  sessionFormatPrefix + prefix + "_" + secret,
			CSRFToken: csrf,
			Session:   sess,
			User:      user,
		}, nil
	}
	return nil, fmt.Errorf("failed to issue session after %d retries", tokenRetryLimit)
}

// Logout удаляет session. Не-найденная session — no-op (idempotent).
func (s *AuthService) Logout(ctx context.Context, sessionID uuid.UUID) error {
	if s.sessions == nil {
		return fmt.Errorf("sessions repository not configured")
	}
	if err := s.sessions.Delete(ctx, sessionID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	return nil
}

// ValidateSession парсит raw cookie value, ищет session по prefix, проверяет
// secret_hash и expiry. Возвращает (user, role, session) или ErrUnauthorized.
func (s *AuthService) ValidateSession(ctx context.Context, raw string) (*User, Role, *Session, error) {
	if s.sessions == nil {
		return nil, "", nil, ErrUnauthorized
	}
	if !sessionFormatRegex.MatchString(raw) {
		return nil, "", nil, ErrUnauthorized
	}
	rest := raw[len(sessionFormatPrefix):]
	prefix := rest[:prefixLen]
	secret := rest[prefixLen+1:]

	sess, err := s.sessions.GetByPrefix(ctx, prefix)
	if err != nil {
		return nil, "", nil, ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(sess.SecretHash), []byte(secret)); err != nil {
		return nil, "", nil, ErrUnauthorized
	}
	if s.clock.Now().After(sess.ExpiresAt) {
		return nil, "", nil, ErrUnauthorized
	}
	user, err := s.users.GetByID(ctx, sess.UserID)
	if err != nil {
		return nil, "", nil, ErrUnauthorized
	}
	go func(id uuid.UUID, t time.Time) {
		_ = s.sessions.UpdateLastUsedAt(context.Background(), id, t) //nolint:contextcheck // intentional: detached lifecycle
	}(sess.ID, s.clock.Now().UTC())
	return user, user.Role, sess, nil
}

// RefreshCSRF генерирует новый CSRF-token для существующей session.
func (s *AuthService) RefreshCSRF(ctx context.Context, sessionID uuid.UUID) (string, error) {
	if s.sessions == nil {
		return "", fmt.Errorf("sessions repository not configured")
	}
	csrf, err := randomString(csrfTokenLen)
	if err != nil {
		return "", err
	}
	if err := s.sessions.UpdateCSRFToken(ctx, sessionID, csrf); err != nil {
		return "", err
	}
	return csrf, nil
}
