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

func TestDelete_RemoteMode_Success(t *testing.T) {
	postID := uuid.New()
	mux := http.NewServeMux()
	mux.HandleFunc("/posts/"+postID.String(), func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cli, _, err := newAPIClient(newRemoteCmd(t, srv))
	if err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	if err := runDeleteRemote(context.Background(), cli, postID.String(), out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "удалён") {
		t.Errorf("expected success message, got %s", out.String())
	}
}

func TestDelete_RemoteMode_401(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/posts/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runDeleteRemote(context.Background(), cli, uuid.New().String(), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestDelete_RemoteMode_404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/posts/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runDeleteRemote(context.Background(), cli, uuid.New().String(), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestDelete_RemoteMode_BadID(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runDeleteRemote(context.Background(), cli, "not-uuid", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "UUID") {
		t.Fatalf("expected UUID error, got %v", err)
	}
}
