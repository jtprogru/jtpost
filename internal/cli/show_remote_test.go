package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func newRemoteCmd(t *testing.T, server *httptest.Server) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("remote", server.URL, "")
	cmd.Flags().String("auth", "fake-token", "")
	return cmd
}

func TestShow_RemoteMode_Success(t *testing.T) {
	postID := uuid.MustParse("01900000-0000-7000-8000-000000000001")
	body := `{"id":"` + postID.String() + `","tenant_id":"00000000-0000-0000-0000-000000000000","author_id":"00000000-0000-0000-0000-000000000000","title":"Hi","slug":"hi","status":"draft","content":"x","revision":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`
	mux := http.NewServeMux()
	mux.HandleFunc("/posts/"+postID.String(), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cli, _, err := newAPIClient(newRemoteCmd(t, srv))
	if err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	if err := runShowRemote(context.Background(), cli, postID.String(), out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Hi") {
		t.Errorf("expected output to contain title, got %s", out.String())
	}
}

func TestShow_RemoteMode_404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/posts/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cli, _, err := newAPIClient(newRemoteCmd(t, srv))
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New().String()
	err = runShowRemote(context.Background(), cli, id, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestShow_RemoteMode_BadID(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runShowRemote(context.Background(), cli, "not-a-uuid", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "UUID") {
		t.Fatalf("expected UUID error, got %v", err)
	}
}

func TestStats_RemoteMode_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stats", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"draft":3,"published":7}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	out := &bytes.Buffer{}
	if err := runStatsRemote(context.Background(), cli, out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "draft") {
		t.Errorf("expected stats in output, got %s", out.String())
	}
}

func TestPlan_RemoteMode_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/plan", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	out := &bytes.Buffer{}
	if err := runPlanRemote(context.Background(), cli, out); err != nil {
		t.Fatal(err)
	}
}

func TestNext_RemoteMode_Success(t *testing.T) {
	postID := uuid.New()
	mux := http.NewServeMux()
	mux.HandleFunc("/next", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := `{"id":"` + postID.String() + `","tenant_id":"00000000-0000-0000-0000-000000000000","author_id":"00000000-0000-0000-0000-000000000000","title":"Next","slug":"next","status":"ready","content":"x","revision":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	out := &bytes.Buffer{}
	if err := runNextRemote(context.Background(), cli, out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Next") {
		t.Errorf("expected post in output")
	}
}

func TestNext_RemoteMode_404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/next", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, _, _ := newAPIClient(newRemoteCmd(t, srv))
	err := runNextRemote(context.Background(), cli, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "no next post") {
		t.Fatalf("expected no-next error, got %v", err)
	}
}
