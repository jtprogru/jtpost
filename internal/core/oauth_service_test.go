package core

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// mockOAuthAccountRepo
type mockOAuthAccountRepo struct {
	byID    map[uuid.UUID]*OAuthAccount
	byExtID map[string]*OAuthAccount // key = provider|external_id
}

func newMockOAuthAccounts() *mockOAuthAccountRepo {
	return &mockOAuthAccountRepo{byID: map[uuid.UUID]*OAuthAccount{}, byExtID: map[string]*OAuthAccount{}}
}

func (m *mockOAuthAccountRepo) extKey(provider, ext string) string { return provider + "|" + ext }

func (m *mockOAuthAccountRepo) GetByExternalID(_ context.Context, provider, ext string) (*OAuthAccount, error) {
	a, ok := m.byExtID[m.extKey(provider, ext)]
	if !ok {
		return nil, ErrNotFound
	}
	return a, nil
}

func (m *mockOAuthAccountRepo) Create(_ context.Context, a *OAuthAccount) error {
	if _, ok := m.byExtID[m.extKey(a.Provider, a.ExternalID)]; ok {
		return ErrAlreadyExists
	}
	m.byID[a.ID] = a
	m.byExtID[m.extKey(a.Provider, a.ExternalID)] = a
	return nil
}

func (m *mockOAuthAccountRepo) ListByUser(_ context.Context, uid uuid.UUID) ([]*OAuthAccount, error) {
	out := []*OAuthAccount{}
	for _, a := range m.byID {
		if a.UserID == uid {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *mockOAuthAccountRepo) Delete(_ context.Context, id uuid.UUID) error {
	a, ok := m.byID[id]
	if !ok {
		return ErrNotFound
	}
	delete(m.byID, id)
	delete(m.byExtID, m.extKey(a.Provider, a.ExternalID))
	return nil
}

// mockOAuthProvider
type mockOAuthProvider struct {
	name        string
	authorize   string
	exchangeRet string
	exchangeErr error
	userInfo    *OAuthUserInfo
	userInfoErr error
	exchangeCnt atomic.Int64
}

func (p *mockOAuthProvider) Name() string { return p.name }
func (p *mockOAuthProvider) AuthorizeURL(state string) string {
	return p.authorize + "?state=" + state
}
func (p *mockOAuthProvider) Exchange(_ context.Context, _ string) (string, error) {
	p.exchangeCnt.Add(1)
	if p.exchangeErr != nil {
		return "", p.exchangeErr
	}
	return p.exchangeRet, nil
}
func (p *mockOAuthProvider) FetchUserInfo(_ context.Context, _ string) (*OAuthUserInfo, error) {
	if p.userInfoErr != nil {
		return nil, p.userInfoErr
	}
	return p.userInfo, nil
}

func newOAuthSvc(t *testing.T, info *OAuthUserInfo) (*OAuthService, *mockUserRepo, *mockOAuthAccountRepo, *mockOAuthProvider, uuid.UUID) {
	t.Helper()
	users := newMockUsers()
	oauth := newMockOAuthAccounts()
	provider := &mockOAuthProvider{
		name:        "github",
		authorize:   "https://github.com/login/oauth/authorize",
		exchangeRet: "fake-token",
		userInfo:    info,
	}
	tenantID := uuid.New()
	clk := &authClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	svc := NewOAuthService(
		map[string]OAuthProvider{"github": provider},
		users,
		oauth,
		tenantID,
		RoleAuthor,
		clk,
	)
	return svc, users, oauth, provider, tenantID
}

func TestOAuthService_BuildAuthorizeURL(t *testing.T) {
	svc, _, _, _, _ := newOAuthSvc(t, nil)
	url, state, err := svc.BuildAuthorizeURL("github")
	if err != nil {
		t.Fatal(err)
	}
	if state == "" {
		t.Fatal("state empty")
	}
	if !strings.Contains(url, "state="+state) {
		t.Errorf("url=%q does not contain state=%s", url, state)
	}
}

func TestOAuthService_BuildAuthorizeURL_UnknownProvider(t *testing.T) {
	svc, _, _, _, _ := newOAuthSvc(t, nil)
	_, _, err := svc.BuildAuthorizeURL("google")
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("want ErrConfigInvalid, got %v", err)
	}
}

func TestOAuthService_HandleCallback_NewUser(t *testing.T) {
	svc, users, oauth, _, _ := newOAuthSvc(t, &OAuthUserInfo{
		ExternalID:  "12345",
		Email:       "octo@example.com",
		DisplayName: "octocat",
	})
	user, err := svc.HandleCallback(context.Background(), "github", "fake-code")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "octo@example.com" || user.PasswordHash != "" || user.Role != RoleAuthor {
		t.Errorf("new user mismatch: %+v", user)
	}
	if _, ok := users.byID[user.ID]; !ok {
		t.Error("user not stored")
	}
	if a, ok := oauth.byExtID["github|12345"]; !ok || a.UserID != user.ID {
		t.Error("oauth_account not linked")
	}
}

func TestOAuthService_HandleCallback_ExistingOAuth(t *testing.T) {
	svc, users, oauth, _, tenantID := newOAuthSvc(t, &OAuthUserInfo{
		ExternalID:  "12345",
		Email:       "octo@example.com",
		DisplayName: "octocat",
	})
	// Pre-existing user + oauth_account.
	uid, _ := uuid.NewV7()
	users.byID[uid] = &User{ID: uid, TenantID: tenantID, Email: "octo@example.com", Role: RoleOwner}
	users.byEmail[users.emailKey(tenantID, "octo@example.com")] = users.byID[uid]
	aid, _ := uuid.NewV7()
	a := &OAuthAccount{ID: aid, UserID: uid, Provider: "github", ExternalID: "12345", Email: "octo@example.com"}
	oauth.byID[aid] = a
	oauth.byExtID["github|12345"] = a

	user, err := svc.HandleCallback(context.Background(), "github", "fake-code")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != uid {
		t.Errorf("expected re-login of existing user, got %v", user.ID)
	}
}

func TestOAuthService_HandleCallback_LinkByEmail(t *testing.T) {
	svc, users, oauth, _, tenantID := newOAuthSvc(t, &OAuthUserInfo{
		ExternalID:  "99999",
		Email:       "alice@example.com",
		DisplayName: "alice",
	})
	// Pre-existing local user (без oauth_account).
	uid, _ := uuid.NewV7()
	users.byID[uid] = &User{ID: uid, TenantID: tenantID, Email: "alice@example.com", Role: RoleEditor, PasswordHash: "$argon2id$..."}
	users.byEmail[users.emailKey(tenantID, "alice@example.com")] = users.byID[uid]

	user, err := svc.HandleCallback(context.Background(), "github", "fake-code")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != uid {
		t.Errorf("expected link to existing user")
	}
	if _, ok := oauth.byExtID["github|99999"]; !ok {
		t.Error("oauth_account not created during link")
	}
}

func TestOAuthService_HandleCallback_NoVerifiedEmail(t *testing.T) {
	svc, _, _, provider, _ := newOAuthSvc(t, nil)
	provider.userInfoErr = errors.Join(ErrValidation, errors.New("no primary verified email"))
	_, err := svc.HandleCallback(context.Background(), "github", "fake-code")
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestOAuthService_HandleCallback_UnknownProvider(t *testing.T) {
	svc, _, _, _, _ := newOAuthSvc(t, nil)
	_, err := svc.HandleCallback(context.Background(), "google", "fake-code")
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("want ErrConfigInvalid, got %v", err)
	}
}
