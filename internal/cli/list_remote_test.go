package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// mockServer возвращает httptest-сервер, отдающий заданное body+status на /api/v1/posts.
func mockListPostsServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/posts", func(w http.ResponseWriter, r *http.Request) {
		// validate auth
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
	return httptest.NewServer(mux)
}

func TestList_RemoteMode_Success(t *testing.T) {
	postID := uuid.MustParse("01900000-0000-7000-8000-000000000001")
	body := `[{"id":"` + postID.String() + `","tenant_id":"00000000-0000-0000-0000-000000000000","author_id":"00000000-0000-0000-0000-000000000000","title":"Hello","slug":"hello","status":"draft","content":"body","revision":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`
	srv := mockListPostsServer(t, http.StatusOK, body)
	defer srv.Close()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("remote", srv.URL, "")
	cmd.Flags().String("auth", "test-token", "")
	out := &bytes.Buffer{}
	cmd.SetOut(out)

	cli, isRemote, err := newAPIClient(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !isRemote {
		t.Fatal("expected isRemote=true")
	}
	if err := runListRemote(cmd, cli); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out.String())
	}
	if len(got) != 1 || got[0]["title"] != "Hello" {
		t.Errorf("unexpected output: %+v", got)
	}
}

func TestList_RemoteMode_401(t *testing.T) {
	srv := mockListPostsServer(t, http.StatusOK, `[]`)
	defer srv.Close()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("remote", srv.URL, "")
	cmd.Flags().String("auth", "", "") // empty → no auth header → server returns 401
	t.Setenv("JTPOST_AUTH_TOKEN", "")

	_, _, err := newAPIClient(cmd)
	// Пустой auth → newAPIClient вернёт ошибку до request'а.
	if err == nil || !strings.Contains(err.Error(), "auth required") {
		t.Fatalf("expected auth required error, got %v", err)
	}
}

func TestList_RemoteMode_RemoteReturns401(t *testing.T) {
	srv := mockListPostsServer(t, http.StatusUnauthorized, `{"error":"unauthorized"}`)
	defer srv.Close()

	// Override mock — реальный сервер отвергает любой токен.
	mux := http.NewServeMux()
	mux.HandleFunc("/posts", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	})
	srv2 := httptest.NewServer(mux)
	defer srv2.Close()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("remote", srv2.URL, "")
	cmd.Flags().String("auth", "fake-token", "")

	cli, _, err := newAPIClient(cmd)
	if err != nil {
		t.Fatal(err)
	}
	err = runListRemote(cmd, cli)
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}
