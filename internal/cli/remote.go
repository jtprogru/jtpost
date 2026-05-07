package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
	"github.com/spf13/cobra"
)

// newAPIClient возвращает client для remote-mode.
// Returns (nil, false, nil) если --remote не задан → caller использует local mode.
// Returns (client, true, nil) при success.
// Returns (nil, false, err) при misconfiguration.
func newAPIClient(cmd *cobra.Command) (*apiclient.ClientWithResponses, bool, error) {
	remote, _ := cmd.Flags().GetString("remote")
	if remote == "" {
		return nil, false, nil
	}
	remote = strings.TrimRight(remote, "/")
	if _, err := url.ParseRequestURI(remote); err != nil {
		return nil, false, fmt.Errorf("invalid --remote URL %q: %w", remote, err)
	}
	auth, _ := cmd.Flags().GetString("auth")
	if auth == "" {
		auth = os.Getenv("JTPOST_AUTH_TOKEN")
	}
	if auth == "" {
		return nil, false, fmt.Errorf("--auth required when using --remote (or set JTPOST_AUTH_TOKEN env)")
	}
	cli, err := apiclient.NewClientWithResponses(remote, apiclient.WithRequestEditorFn(
		func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+auth)
			return nil
		},
	))
	if err != nil {
		return nil, false, fmt.Errorf("apiclient init: %w", err)
	}
	return cli, true, nil
}

// runRemote — общий wrapper для remote-mode CLI команд. Если --remote не задан,
// возвращает (false, nil) — caller продолжает в local-mode. Иначе вызывает fn
// с готовым client + ctx и возвращает (true, fn-error).
func runRemote(cmd *cobra.Command, fn func(ctx context.Context, cli *apiclient.ClientWithResponses) error) (bool, error) {
	cli, isRemote, err := newAPIClient(cmd)
	if err != nil {
		return false, err
	}
	if !isRemote {
		return false, nil
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	return true, fn(ctx, cli)
}
