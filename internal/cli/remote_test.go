package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newTestCmd(t *testing.T, remote, auth string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("remote", remote, "")
	cmd.Flags().String("auth", auth, "")
	return cmd
}

func TestNewAPIClient_NoRemote(t *testing.T) {
	cmd := newTestCmd(t, "", "")
	cli, isRemote, err := newAPIClient(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if cli != nil || isRemote {
		t.Errorf("expected (nil, false), got cli=%v remote=%v", cli, isRemote)
	}
}

func TestNewAPIClient_RemoteWithAuth(t *testing.T) {
	cmd := newTestCmd(t, "http://localhost:8080", "jtpat_token")
	cli, isRemote, err := newAPIClient(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if cli == nil || !isRemote {
		t.Error("expected client + isRemote=true")
	}
}

func TestNewAPIClient_RemoteNoAuth_NoEnv(t *testing.T) {
	t.Setenv("JTPOST_AUTH_TOKEN", "")
	cmd := newTestCmd(t, "http://localhost:8080", "")
	_, _, err := newAPIClient(cmd)
	if err == nil || !strings.Contains(err.Error(), "auth required") {
		t.Fatalf("want 'auth required' error, got %v", err)
	}
}

func TestNewAPIClient_AuthFromEnv(t *testing.T) {
	t.Setenv("JTPOST_AUTH_TOKEN", "env-token")
	cmd := newTestCmd(t, "http://localhost:8080", "")
	cli, isRemote, err := newAPIClient(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if cli == nil || !isRemote {
		t.Error("expected client from env-token")
	}
}

func TestNewAPIClient_InvalidURL(t *testing.T) {
	cmd := newTestCmd(t, "not-a-url", "tok")
	_, _, err := newAPIClient(cmd)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestNewAPIClient_TrimsTrailingSlash(t *testing.T) {
	cmd := newTestCmd(t, "http://localhost:8080/", "tok")
	cli, isRemote, err := newAPIClient(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if cli == nil || !isRemote {
		t.Error("trim trailing slash should still build client")
	}
}
