package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestPublish_RemoteMode_Success(t *testing.T) {
	postID := uuid.New()
	mux := http.NewServeMux()
	mux.HandleFunc("/posts/"+postID.String()+"/publish", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		body := `{"id":"` + postID.String() + `","tenant_id":"00000000-0000-0000-0000-000000000000","author_id":"00000000-0000-0000-0000-000000000000","title":"Hi","slug":"hi","status":"published","content":"x","revision":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","external":{}}`
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	out := &bytes.Buffer{}
	if err := runPublishRemote(context.Background(), cli, postID.String(), out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "опубликован") {
		t.Errorf("expected success message, got %s", out.String())
	}
}

func TestPublish_RemoteMode_401(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/posts/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runPublishRemote(context.Background(), cli, uuid.New().String(), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestPublish_RemoteMode_404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/posts/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runPublishRemote(context.Background(), cli, uuid.New().String(), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestPublish_RemoteMode_Conflict(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/posts/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"already published"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runPublishRemote(context.Background(), cli, uuid.New().String(), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("expected conflict, got %v", err)
	}
}
