package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestEdit_RemoteMode_Success(t *testing.T) {
	postID := uuid.New()
	mux := http.NewServeMux()
	mux.HandleFunc("/posts/"+postID.String(), func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["title"] != "NewTitle" {
			t.Errorf("expected title=NewTitle, got %v", body["title"])
		}
		if _, has := body["content"]; has {
			t.Errorf("content should not be in partial body")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + postID.String() + `","tenant_id":"00000000-0000-0000-0000-000000000000","author_id":"00000000-0000-0000-0000-000000000000","title":"NewTitle","slug":"hi","status":"draft","content":"x","revision":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","external":{}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	out := &bytes.Buffer{}
	if err := runEditRemote(context.Background(), cli, postID.String(), "NewTitle", "", "", nil, false, nil, out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "NewTitle") {
		t.Errorf("expected title in output, got %s", out.String())
	}
}

func TestEdit_RemoteMode_NoFields(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runEditRemote(context.Background(), cli, uuid.New().String(), "", "", "", nil, false, nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "no fields") {
		t.Fatalf("expected no-fields error, got %v", err)
	}
}

func TestEdit_RemoteMode_BadID(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runEditRemote(context.Background(), cli, "not-uuid", "T", "", "", nil, false, nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "UUID") {
		t.Fatalf("expected UUID error, got %v", err)
	}
}

func TestEdit_RemoteMode_404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/posts/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runEditRemote(context.Background(), cli, uuid.New().String(), "T", "", "", nil, false, nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestEdit_RemoteMode_TagsReplace(t *testing.T) {
	postID := uuid.New()
	mux := http.NewServeMux()
	mux.HandleFunc("/posts/"+postID.String(), func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		tags, ok := body["tags"].([]any)
		if !ok || len(tags) != 2 {
			t.Errorf("expected 2 tags, got %v", body["tags"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + postID.String() + `","tenant_id":"00000000-0000-0000-0000-000000000000","author_id":"00000000-0000-0000-0000-000000000000","title":"x","slug":"x","status":"draft","content":"x","revision":3,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","external":{}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	if err := runEditRemote(context.Background(), cli, postID.String(), "", "", "", []string{"go", "api"}, true, nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

func TestReadContentSource_Stdin(t *testing.T) {
	got, err := readContentSource("-", strings.NewReader("hello"))
	if err != nil || got != "hello" {
		t.Fatalf("expected hello, got %q err=%v", got, err)
	}
}

func TestReadContentSource_Empty(t *testing.T) {
	got, err := readContentSource("", nil)
	if err != nil || got != "" {
		t.Fatalf("expected empty, got %q err=%v", got, err)
	}
}
