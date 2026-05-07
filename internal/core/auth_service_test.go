package core

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// mockUserRepo — in-memory UserRepository для тестов.
type mockUserRepo struct {
	mu           sync.RWMutex
	byID         map[uuid.UUID]*User
	byEmail      map[string]*User // key = tenantID|email
	createErr    error
	getCallCount atomic.Int64
}

func newMockUsers() *mockUserRepo {
	return &mockUserRepo{
		byID:    map[uuid.UUID]*User{},
		byEmail: map[string]*User{},
	}
}

//nolint:funcorder // pure helper used by mock impls — keeping at top for readability.
func (m *mockUserRepo) emailKey(t uuid.UUID, e string) string { return t.String() + "|" + e }

func (m *mockUserRepo) GetByID(_ context.Context, id uuid.UUID) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.getCallCount.Add(1)
	u, ok := m.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByEmail(_ context.Context, t uuid.UUID, e string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.getCallCount.Add(1)
	u, ok := m.byEmail[m.emailKey(t, e)]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) Create(_ context.Context, u *User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return m.createErr
	}
	if _, ok := m.byEmail[m.emailKey(u.TenantID, u.Email)]; ok {
		return ErrAlreadyExists
	}
	m.byID[u.ID] = u
	m.byEmail[m.emailKey(u.TenantID, u.Email)] = u
	return nil
}

func (m *mockUserRepo) Update(_ context.Context, u *User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[u.ID]; !ok {
		return ErrNotFound
	}
	m.byID[u.ID] = u
	m.byEmail[m.emailKey(u.TenantID, u.Email)] = u
	return nil
}

func (m *mockUserRepo) Delete(_ context.Context, id uuid.UUID) error {
	u, ok := m.byID[id]
	if !ok {
		return ErrNotFound
	}
	delete(m.byID, id)
	delete(m.byEmail, m.emailKey(u.TenantID, u.Email))
	return nil
}

func (m *mockUserRepo) List(_ context.Context, t uuid.UUID) ([]*User, error) {
	out := []*User{}
	for _, u := range m.byID {
		if u.TenantID == t {
			out = append(out, u)
		}
	}
	return out, nil
}

func (m *mockUserRepo) Count(_ context.Context, t uuid.UUID) (int64, error) {
	var c int64
	for _, u := range m.byID {
		if u.TenantID == t {
			c++
		}
	}
	return c, nil
}

func (m *mockUserRepo) CountOwners(_ context.Context, t uuid.UUID) (int64, error) {
	var c int64
	for _, u := range m.byID {
		if u.TenantID == t && u.Role == RoleOwner {
			c++
		}
	}
	return c, nil
}

// mockTokenRepo.
type mockTokenRepo struct {
	byID         map[uuid.UUID]*APIToken
	byPrefix     map[string]*APIToken
	getCallCount atomic.Int64
}

func newMockTokens() *mockTokenRepo {
	return &mockTokenRepo{byID: map[uuid.UUID]*APIToken{}, byPrefix: map[string]*APIToken{}}
}

func (m *mockTokenRepo) GetByPrefix(_ context.Context, p string) (*APIToken, error) {
	m.getCallCount.Add(1)
	t, ok := m.byPrefix[p]
	if !ok {
		return nil, ErrNotFound
	}
	return t, nil
}

func (m *mockTokenRepo) Create(_ context.Context, t *APIToken) error {
	if _, ok := m.byPrefix[t.Prefix]; ok {
		return ErrAlreadyExists
	}
	m.byID[t.ID] = t
	m.byPrefix[t.Prefix] = t
	return nil
}

func (m *mockTokenRepo) Delete(_ context.Context, id uuid.UUID) error {
	t, ok := m.byID[id]
	if !ok {
		return ErrNotFound
	}
	delete(m.byID, id)
	delete(m.byPrefix, t.Prefix)
	return nil
}

func (m *mockTokenRepo) ListByUser(_ context.Context, uid uuid.UUID) ([]*APIToken, error) {
	out := []*APIToken{}
	for _, t := range m.byID {
		if t.UserID == uid {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *mockTokenRepo) UpdateLastUsedAt(_ context.Context, id uuid.UUID, t time.Time) error {
	tok, ok := m.byID[id]
	if !ok {
		return ErrNotFound
	}
	tok.LastUsedAt = &t
	return nil
}

// authClock.
type authClock struct{ now time.Time }

func (c *authClock) Now() time.Time { return c.now }

func newAuthSvc(t *testing.T) (*AuthService, *mockUserRepo, *mockTokenRepo, *authClock) {
	t.Helper()
	users := newMockUsers()
	tokens := newMockTokens()
	clk := &authClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	return NewAuthService(users, tokens, newMockSessions(), NewMultiHasher(), clk), users, tokens, clk
}

// mockSessionRepo — in-memory SessionRepository для тестов.
type mockSessionRepo struct {
	byID     map[uuid.UUID]*Session
	byPrefix map[string]*Session
}

func newMockSessions() *mockSessionRepo {
	return &mockSessionRepo{byID: map[uuid.UUID]*Session{}, byPrefix: map[string]*Session{}}
}

func (m *mockSessionRepo) GetByPrefix(_ context.Context, p string) (*Session, error) {
	s, ok := m.byPrefix[p]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

func (m *mockSessionRepo) Create(_ context.Context, s *Session) error {
	if _, ok := m.byPrefix[s.Prefix]; ok {
		return ErrAlreadyExists
	}
	m.byID[s.ID] = s
	m.byPrefix[s.Prefix] = s
	return nil
}

func (m *mockSessionRepo) Delete(_ context.Context, id uuid.UUID) error {
	s, ok := m.byID[id]
	if !ok {
		return ErrNotFound
	}
	delete(m.byID, id)
	delete(m.byPrefix, s.Prefix)
	return nil
}

func (m *mockSessionRepo) DeleteByUser(_ context.Context, uid uuid.UUID) error {
	for id, s := range m.byID {
		if s.UserID == uid {
			delete(m.byID, id)
			delete(m.byPrefix, s.Prefix)
		}
	}
	return nil
}

func (m *mockSessionRepo) UpdateLastUsedAt(_ context.Context, id uuid.UUID, t time.Time) error {
	s, ok := m.byID[id]
	if !ok {
		return ErrNotFound
	}
	s.LastUsedAt = &t
	return nil
}

func (m *mockSessionRepo) UpdateCSRFToken(_ context.Context, id uuid.UUID, csrf string) error {
	s, ok := m.byID[id]
	if !ok {
		return ErrNotFound
	}
	s.CSRFToken = csrf
	return nil
}

func TestRolePermissions(t *testing.T) {
	tt := []struct {
		role   Role
		expect int // count
	}{
		{RoleOwner, 6},
		{RoleEditor, 4},
		{RoleAuthor, 2},
		{RoleViewer, 0},
		{Role("unknown"), 0},
	}
	for _, tc := range tt {
		t.Run(string(tc.role), func(t *testing.T) {
			got := RolePermissions(tc.role)
			if len(got) != tc.expect {
				t.Errorf("RolePermissions(%s) = %d, want %d", tc.role, len(got), tc.expect)
			}
		})
	}
}

func TestAuthService_CreateUser_Success(t *testing.T) {
	svc, users, _, _ := newAuthSvc(t)
	ctx := context.Background()
	tenantID := uuid.New()
	user, err := svc.CreateUser(ctx, CreateUserInput{
		TenantID: tenantID,
		Email:    "alice@example.com",
		Password: "password123",
		Role:     RoleOwner,
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "alice@example.com" || user.Role != RoleOwner {
		t.Errorf("got %+v", user)
	}
	// F4c: password hashes теперь Argon2id (не bcrypt cost-based).
	if !strings.HasPrefix(user.PasswordHash, "$argon2id$") {
		t.Errorf("password hash format %q, want $argon2id$ prefix", user.PasswordHash)
	}
	if _, ok := users.byID[user.ID]; !ok {
		t.Error("user not stored")
	}
}

func TestAuthService_CreateUser_ShortPassword(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	_, err := svc.CreateUser(context.Background(), CreateUserInput{
		TenantID: uuid.New(),
		Email:    "x@y.com",
		Password: "short",
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestAuthService_CreateUser_InvalidEmail(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	_, err := svc.CreateUser(context.Background(), CreateUserInput{
		TenantID: uuid.New(),
		Email:    "not-an-email",
		Password: "password123",
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestAuthService_CreateUser_EmailCollision(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	tenantID := uuid.New()
	in := CreateUserInput{TenantID: tenantID, Email: "a@x.com", Password: "password123", Role: RoleOwner}
	if _, err := svc.CreateUser(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateUser(context.Background(), in); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("want ErrAlreadyExists, got %v", err)
	}
}

func TestAuthService_VerifyPassword_Success(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	ctx := context.Background()
	tenantID := uuid.New()
	_, err := svc.CreateUser(ctx, CreateUserInput{TenantID: tenantID, Email: "a@x.com", Password: "password123", Role: RoleOwner})
	if err != nil {
		t.Fatal(err)
	}
	user, err := svc.VerifyPassword(ctx, tenantID, "a@x.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "a@x.com" {
		t.Error("wrong user")
	}
}

func TestAuthService_VerifyPassword_Wrong(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	ctx := context.Background()
	tenantID := uuid.New()
	_, _ = svc.CreateUser(ctx, CreateUserInput{TenantID: tenantID, Email: "a@x.com", Password: "password123", Role: RoleOwner})
	_, err := svc.VerifyPassword(ctx, tenantID, "a@x.com", "wrong-password")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestAuthService_VerifyPassword_UserNotFound(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	_, err := svc.VerifyPassword(context.Background(), uuid.New(), "ghost@x.com", "password123")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized (no leak), got %v", err)
	}
}

func TestAuthService_IssueToken_Format(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	ctx := context.Background()
	user, _ := svc.CreateUser(ctx, CreateUserInput{TenantID: uuid.New(), Email: "a@x.com", Password: "password123", Role: RoleOwner})
	issued, err := svc.IssueToken(ctx, user.ID, "test-cli", nil)
	if err != nil {
		t.Fatal(err)
	}
	rgx := regexp.MustCompile(`^jtpat_[A-Za-z0-9]{8}_[A-Za-z0-9]{24}$`)
	if !rgx.MatchString(issued.Raw) {
		t.Errorf("token format mismatch: %q", issued.Raw)
	}
	if issued.Token == nil || issued.Token.UserID != user.ID {
		t.Error("token metadata mismatch")
	}
	if cost, _ := bcrypt.Cost([]byte(issued.Token.SecretHash)); cost != tokenSecretCost {
		t.Errorf("secret hash cost=%d, want %d", cost, tokenSecretCost)
	}
}

func TestAuthService_ValidateToken_RoundTrip(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	ctx := context.Background()
	user, _ := svc.CreateUser(ctx, CreateUserInput{TenantID: uuid.New(), Email: "a@x.com", Password: "password123", Role: RoleEditor})
	issued, _ := svc.IssueToken(ctx, user.ID, "cli", nil)
	got, role, err := svc.ValidateToken(ctx, issued.Raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != user.ID {
		t.Errorf("user mismatch")
	}
	if role != RoleEditor {
		t.Errorf("role=%s, want editor", role)
	}
}

func TestAuthService_ValidateToken_Expired(t *testing.T) {
	svc, _, _, clk := newAuthSvc(t)
	ctx := context.Background()
	user, _ := svc.CreateUser(ctx, CreateUserInput{TenantID: uuid.New(), Email: "a@x.com", Password: "password123", Role: RoleOwner})
	dur := 1 * time.Hour
	issued, _ := svc.IssueToken(ctx, user.ID, "cli", &dur)
	// продвигаем clock на 2 часа вперёд
	clk.now = clk.now.Add(2 * time.Hour)
	_, _, err := svc.ValidateToken(ctx, issued.Raw)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expired token: want ErrUnauthorized, got %v", err)
	}
}

func TestAuthService_ValidateToken_BadFormat_NoSQL(t *testing.T) {
	svc, _, tokens, _ := newAuthSvc(t)
	before := tokens.getCallCount.Load()
	for _, raw := range []string{"", "bad-token", "Bearer xyz", "jtpat_short", "jtpat_xxxxxxxx_short"} {
		_, _, err := svc.ValidateToken(context.Background(), raw)
		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("raw=%q: want ErrUnauthorized, got %v", raw, err)
		}
	}
	after := tokens.getCallCount.Load()
	if after != before {
		t.Errorf("invalid format must not hit DB: GetByPrefix called %d times", after-before)
	}
}

func TestAuthService_RevokeToken(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	ctx := context.Background()
	user, _ := svc.CreateUser(ctx, CreateUserInput{TenantID: uuid.New(), Email: "a@x.com", Password: "password123", Role: RoleOwner})
	issued, _ := svc.IssueToken(ctx, user.ID, "cli", nil)
	if err := svc.RevokeToken(ctx, issued.Token.ID); err != nil {
		t.Fatal(err)
	}
	_, _, err := svc.ValidateToken(ctx, issued.Raw)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("revoked: want ErrUnauthorized, got %v", err)
	}
}

func TestAuthService_AuthorizeOperation(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	tt := []struct {
		role    Role
		perm    Permission
		wantErr error
	}{
		{RoleOwner, PermUsersManage, nil},
		{RoleEditor, PermPostsCreate, nil},
		{RoleEditor, PermUsersManage, ErrForbidden},
		{RoleAuthor, PermPostsDelete, ErrForbidden},
		{RoleViewer, PermPostsCreate, ErrForbidden},
	}
	for _, tc := range tt {
		t.Run(string(tc.role)+"_"+string(tc.perm), func(t *testing.T) {
			ctx := WithRole(context.Background(), tc.role)
			err := svc.AuthorizeOperation(ctx, tc.perm)
			if tc.wantErr == nil && err != nil {
				t.Errorf("want nil, got %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestAuthService_AuthorizeOperation_NoRoleInCtx(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	err := svc.AuthorizeOperation(context.Background(), PermPostsCreate)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	ctx := context.Background()
	tenantID := uuid.New()
	_, err := svc.CreateUser(ctx, CreateUserInput{TenantID: tenantID, Email: "a@x.com", Password: "password123", Role: RoleOwner})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Login(ctx, LoginInput{TenantID: tenantID, Email: "a@x.com", Password: "password123"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if res.Session == nil || res.User == nil || res.RawToken == "" || res.CSRFToken == "" {
		t.Fatalf("incomplete LoginResult: %+v", res)
	}
	rgx := regexp.MustCompile(`^jts_[A-Za-z0-9]{8}_[A-Za-z0-9]{32}$`)
	if !rgx.MatchString(res.RawToken) {
		t.Errorf("RawToken format mismatch: %q", res.RawToken)
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	ctx := context.Background()
	tenantID := uuid.New()
	_, _ = svc.CreateUser(ctx, CreateUserInput{TenantID: tenantID, Email: "a@x.com", Password: "password123", Role: RoleOwner})
	_, err := svc.Login(ctx, LoginInput{TenantID: tenantID, Email: "a@x.com", Password: "wrong"}, time.Hour)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestAuthService_ValidateSession_Roundtrip(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	ctx := context.Background()
	tenantID := uuid.New()
	user, _ := svc.CreateUser(ctx, CreateUserInput{TenantID: tenantID, Email: "a@x.com", Password: "password123", Role: RoleEditor})
	res, _ := svc.Login(ctx, LoginInput{TenantID: tenantID, Email: "a@x.com", Password: "password123"}, time.Hour)
	gotUser, role, sess, err := svc.ValidateSession(ctx, res.RawToken)
	if err != nil {
		t.Fatal(err)
	}
	if gotUser.ID != user.ID || role != RoleEditor || sess.ID != res.Session.ID {
		t.Errorf("unexpected: user=%v role=%s sess=%v", gotUser.ID, role, sess.ID)
	}
}

func TestAuthService_ValidateSession_Expired(t *testing.T) {
	svc, _, _, clk := newAuthSvc(t)
	ctx := context.Background()
	tenantID := uuid.New()
	_, _ = svc.CreateUser(ctx, CreateUserInput{TenantID: tenantID, Email: "a@x.com", Password: "password123", Role: RoleOwner})
	res, _ := svc.Login(ctx, LoginInput{TenantID: tenantID, Email: "a@x.com", Password: "password123"}, time.Hour)
	clk.now = clk.now.Add(2 * time.Hour)
	_, _, _, err := svc.ValidateSession(ctx, res.RawToken)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expired session: want ErrUnauthorized, got %v", err)
	}
}

func TestAuthService_ValidateSession_BadFormat(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	for _, raw := range []string{"", "bad", "jtpat_aaaaaaaa_bbbbbbbbbbbbbbbbbbbbbbbb", "jts_short"} {
		_, _, _, err := svc.ValidateSession(context.Background(), raw)
		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("raw=%q: want ErrUnauthorized, got %v", raw, err)
		}
	}
}

func TestAuthService_Logout(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	ctx := context.Background()
	tenantID := uuid.New()
	_, _ = svc.CreateUser(ctx, CreateUserInput{TenantID: tenantID, Email: "a@x.com", Password: "password123", Role: RoleOwner})
	res, _ := svc.Login(ctx, LoginInput{TenantID: tenantID, Email: "a@x.com", Password: "password123"}, time.Hour)
	if err := svc.Logout(ctx, res.Session.ID); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := svc.ValidateSession(ctx, res.RawToken)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("after logout: want ErrUnauthorized, got %v", err)
	}
}

func TestAuthService_Logout_Idempotent(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	if err := svc.Logout(context.Background(), uuid.New()); err != nil {
		t.Fatalf("logout of unknown id should be no-op, got %v", err)
	}
}

func TestAuthService_RefreshCSRF(t *testing.T) {
	svc, _, _, _ := newAuthSvc(t)
	ctx := context.Background()
	tenantID := uuid.New()
	_, _ = svc.CreateUser(ctx, CreateUserInput{TenantID: tenantID, Email: "a@x.com", Password: "password123", Role: RoleOwner})
	res, _ := svc.Login(ctx, LoginInput{TenantID: tenantID, Email: "a@x.com", Password: "password123"}, time.Hour)
	oldCSRF := res.Session.CSRFToken
	newCSRF, err := svc.RefreshCSRF(ctx, res.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if newCSRF == oldCSRF || newCSRF == "" {
		t.Errorf("CSRF should change: old=%q new=%q", oldCSRF, newCSRF)
	}
}

func TestAuthService_VerifyPassword_OAuthOnly_Rejected(t *testing.T) {
	svc, users, _, _ := newAuthSvc(t)
	tenantID := uuid.New()
	id, _ := uuid.NewV7()
	users.byID[id] = &User{
		ID:           id,
		TenantID:     tenantID,
		Email:        "oauth@example.com",
		PasswordHash: "", // OAuth-only user
		Role:         RoleAuthor,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	users.byEmail[users.emailKey(tenantID, "oauth@example.com")] = users.byID[id]
	_, err := svc.VerifyPassword(context.Background(), tenantID, "oauth@example.com", "anything")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("OAuth-only user verifyPassword: want ErrUnauthorized, got %v", err)
	}
}

func TestAuthService_Login_RehashLegacy(t *testing.T) {
	svc, users, _, _ := newAuthSvc(t)
	tenantID := uuid.New()
	// Создать user с manual bcrypt-hash (имитация F4a-data).
	bcryptHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	id, _ := uuid.NewV7()
	users.byID[id] = &User{
		ID:           id,
		TenantID:     tenantID,
		Email:        "old@example.com",
		PasswordHash: string(bcryptHash),
		Role:         RoleOwner,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	users.byEmail[users.emailKey(tenantID, "old@example.com")] = users.byID[id]

	// Login через bcrypt-hashed password — должен success.
	res, err := svc.Login(context.Background(), LoginInput{
		TenantID: tenantID,
		Email:    "old@example.com",
		Password: "password123",
	}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if res.User == nil {
		t.Fatal("user nil")
	}
	// Async rehash — ждём короткое время.
	for range 50 {
		users.mu.RLock()
		hash := users.byID[id].PasswordHash
		users.mu.RUnlock()
		if strings.HasPrefix(hash, "$argon2id$") {
			return // rehash прошёл
		}
		time.Sleep(20 * time.Millisecond)
	}
	users.mu.RLock()
	finalHash := users.byID[id].PasswordHash
	users.mu.RUnlock()
	t.Errorf("rehash не произошёл: hash=%q", finalHash)
}

func TestAuthService_IssueSessionForUser_Roundtrip(t *testing.T) {
	svc, users, _, _ := newAuthSvc(t)
	user := &User{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		Email:    "x@y.com",
		Role:     RoleEditor,
	}
	users.byID[user.ID] = user
	users.byEmail[users.emailKey(user.TenantID, user.Email)] = user

	res, err := svc.IssueSessionForUser(context.Background(), user, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if res.Session == nil || res.RawToken == "" {
		t.Fatal("incomplete result")
	}
	gotUser, role, _, err := svc.ValidateSession(context.Background(), res.RawToken)
	if err != nil {
		t.Fatal(err)
	}
	if gotUser.ID != user.ID || role != RoleEditor {
		t.Errorf("user/role mismatch: %v / %s", gotUser.ID, role)
	}
}
