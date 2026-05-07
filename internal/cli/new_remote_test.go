package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNew_RemoteMode_Success(t *testing.T) {
	postID := uuid.New()
	mux := http.NewServeMux()
	mux.HandleFunc("/posts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["title"] != "Hello" {
			t.Errorf("expected title=Hello, got %v", body["title"])
		}
		if body["content"] != "C" {
			t.Errorf("expected content=C, got %v", body["content"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		resp := `{"id":"` + postID.String() + `","tenant_id":"00000000-0000-0000-0000-000000000000","author_id":"00000000-0000-0000-0000-000000000000","title":"Hello","slug":"hello","status":"draft","content":"C","revision":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","external":{}}`
		_, _ = io.WriteString(w, resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	out := &bytes.Buffer{}
	stdin := strings.NewReader("C")
	if err := runNewRemote(context.Background(), cli, "Hello", "", "-", []string{"go"}, stdin, out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Hello") {
		t.Errorf("expected title in output, got %s", out.String())
	}
}

func TestNew_RemoteMode_NoTitle(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runNewRemote(context.Background(), cli, "", "", "", nil, nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "title") {
		t.Fatalf("expected title error, got %v", err)
	}
}

func TestNew_RemoteMode_400(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/posts", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid slug"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runNewRemote(context.Background(), cli, "T", "", "", nil, nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "validation") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestNew_RemoteMode_401(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/posts", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runNewRemote(context.Background(), cli, "T", "", "", nil, nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}
