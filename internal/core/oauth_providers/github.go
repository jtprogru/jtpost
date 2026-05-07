// Package oauth_providers содержит implementations конкретных OAuth-провайдеров,
// реализующих интерфейс core.OAuthProvider (F4c).
package oauth_providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"

	"github.com/jtprogru/jtpost/internal/core"
)

const (
	apiUserURL   = "https://api.github.com/user"
	apiEmailsURL = "https://api.github.com/user/emails"
)

// GitHubProvider реализует core.OAuthProvider для GitHub.
//
// Скоуп user:email необходим, чтобы получить primary verified email
// через /user/emails (поле email в /user может быть скрыто настройками
// приватности пользователя).
type GitHubProvider struct {
	cfg        *oauth2.Config
	httpClient *http.Client
	apiUser    string // overridable for tests
	apiEmails  string
}

// NewGitHubProvider конструирует провайдер с дефолтными GitHub endpoints
// и таймаутом 10s на HTTP-вызовы к GitHub API.
func NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider {
	return &GitHubProvider{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     githuboauth.Endpoint,
			Scopes:       []string{"user:email"},
		},
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiUser:    apiUserURL,
		apiEmails:  apiEmailsURL,
	}
}

// SetEndpoint используется в тестах для подмены OAuth-endpoint
// (authorize/token URL) на httptest-сервер.
func (p *GitHubProvider) SetEndpoint(authURL, tokenURL string) {
	p.cfg.Endpoint = oauth2.Endpoint{AuthURL: authURL, TokenURL: tokenURL}
}

// SetAPIURLs используется в тестах для подмены GitHub API URLs
// на httptest-сервер.
func (p *GitHubProvider) SetAPIURLs(userURL, emailsURL string) {
	p.apiUser = userURL
	p.apiEmails = emailsURL
}

// Name возвращает идентификатор провайдера для колонки oauth_accounts.provider.
func (p *GitHubProvider) Name() string { return "github" }

// AuthorizeURL строит URL для шага консента (включая client_id, redirect_uri,
// state и scopes).
func (p *GitHubProvider) AuthorizeURL(state string) string {
	return p.cfg.AuthCodeURL(state)
}

// Exchange обменивает authorization code на access_token GitHub.
// Использует внутренний http.Client (с таймаутом) через oauth2.HTTPClient.
func (p *GitHubProvider) Exchange(ctx context.Context, code string) (string, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
	tok, err := p.cfg.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("github exchange: %w", err)
	}
	return tok.AccessToken, nil
}

// FetchUserInfo получает профиль пользователя и primary verified email.
// Возвращает join(ErrValidation, ...) если у пользователя нет
// primary verified email.
func (p *GitHubProvider) FetchUserInfo(ctx context.Context, accessToken string) (*core.OAuthUserInfo, error) {
	user, err := p.fetchUser(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	email, err := p.fetchPrimaryVerifiedEmail(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	return &core.OAuthUserInfo{
		ExternalID:  strconv.FormatInt(user.ID, 10),
		Email:       email,
		DisplayName: user.Login,
	}, nil
}

type githubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (p *GitHubProvider) fetchUser(ctx context.Context, token string) (*githubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiUser, nil)
	if err != nil {
		return nil, fmt.Errorf("build /user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api /user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api /user: status %d", resp.StatusCode)
	}
	var u githubUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}
	return &u, nil
}

func (p *GitHubProvider) fetchPrimaryVerifiedEmail(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiEmails, nil)
	if err != nil {
		return "", fmt.Errorf("build /user/emails request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("github api /user/emails: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api /user/emails: status %d", resp.StatusCode)
	}
	var emails []githubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("decode emails: %w", err)
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", errors.Join(core.ErrValidation, errors.New("no primary verified email"))
}
