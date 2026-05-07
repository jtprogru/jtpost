package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
)

// runShowRemote реализует `jtpost show <id> --remote ...` через apiclient.GetPost.
// Slug в remote-mode не поддерживается (endpoint /posts/{id} принимает только UUID).
func runShowRemote(ctx context.Context, cli *apiclient.ClientWithResponses, idArg string, out io.Writer) error {
	id, err := uuid.Parse(idArg)
	if err != nil {
		return fmt.Errorf("--remote requires UUID id (slug не поддержан в remote-mode): %w", err)
	}
	resp, err := cli.GetPostWithResponse(ctx, id)
	if err != nil {
		return fmt.Errorf("remote API call failed: %w", err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		// proceed
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized: invalid or expired --auth token")
	case http.StatusNotFound:
		return fmt.Errorf("post not found: %s", id)
	default:
		return fmt.Errorf("remote API returned %d", resp.StatusCode())
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("empty response body")
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(*resp.JSON200)
}
