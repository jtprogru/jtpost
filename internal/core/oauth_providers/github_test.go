package oauth_providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jtprogru/jtpost/internal/core"
)

func TestGitHubProvider_Name(t *testing.T) {
	p := NewGitHubProvider("cid", "csec", "http://localhost/cb")
	if got := p.Name(); got != "github" {
		t.Fatalf("Name() = %q, want %q", got, "github")
	}
}

func TestGitHubProvider_AuthorizeURL(t *testing.T) {
	p := NewGitHubProvider("my-client-id", "my-client-secret", "http://localhost/cb")
	raw := p.AuthorizeURL("state-xyz")

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := u.Query()

	if got := q.Get("client_id"); got != "my-client-id" {
		t.Errorf("client_id = %q, want my-client-id", got)
	}
	if got := q.Get("redirect_uri"); got != "http://localhost/cb" {
		t.Errorf("redirect_uri = %q", got)
	}
	if got := q.Get("state"); got != "state-xyz" {
		t.Errorf("state = %q", got)
	}
	if got := q.Get("scope"); !strings.Contains(got, "user:email") {
		t.Errorf("scope = %q, want contains user:email", got)
	}
	// host должен быть github.com (default endpoint).
	if u.Host != "github.com" {
		t.Errorf("host = %q, want github.com", u.Host)
	}
}

// helper: запускает httptest-сервер, эмулирующий /user и /user/emails.
func newGitHubAPIServer(t *testing.T, userBody any, emailsBody any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got == "" {
			t.Errorf("missing Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(userBody)
	})
	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(emailsBody)
	})
	return httptest.NewServer(mux)
}

func TestGitHubProvider_FetchUserInfo_Mock(t *testing.T) {
	srv := newGitHubAPIServer(t,
		map[string]any{"id": 12345, "login": "octocat"},
		[]map[string]any{
			{"email": "octo@example.com", "primary": true, "verified": true},
			{"email": "old@example.com", "primary": false, "verified": true},
		},
	)
	defer srv.Close()

	p := NewGitHubProvider("cid", "csec", "http://localhost/cb")
	p.SetAPIURLs(srv.URL+"/user", srv.URL+"/user/emails")

	info, err := p.FetchUserInfo(context.Background(), "fake-token")
	if err != nil {
		t.Fatalf("FetchUserInfo: %v", err)
	}
	if info.ExternalID != "12345" {
		t.Errorf("ExternalID = %q, want 12345", info.ExternalID)
	}
	if info.Email != "octo@example.com" {
		t.Errorf("Email = %q, want octo@example.com", info.Email)
	}
	if info.DisplayName != "octocat" {
		t.Errorf("DisplayName = %q, want octocat", info.DisplayName)
	}
}

func TestGitHubProvider_FetchUserInfo_NoVerifiedEmail(t *testing.T) {
	srv := newGitHubAPIServer(t,
		map[string]any{"id": 7, "login": "u"},
		[]map[string]any{
			{"email": "x@example.com", "primary": true, "verified": false},
		},
	)
	defer srv.Close()

	p := NewGitHubProvider("cid", "csec", "http://localhost/cb")
	p.SetAPIURLs(srv.URL+"/user", srv.URL+"/user/emails")

	_, err := p.FetchUserInfo(context.Background(), "fake-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, core.ErrValidation) {
		t.Errorf("error not joined with ErrValidation: %v", err)
	}
}

func TestGitHubProvider_FetchUserInfo_UserAPI500(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := NewGitHubProvider("cid", "csec", "http://localhost/cb")
	p.SetAPIURLs(srv.URL+"/user", srv.URL+"/user/emails")

	_, err := p.FetchUserInfo(context.Background(), "fake-token")
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}
