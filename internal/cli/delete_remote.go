package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/apiclient"
)

// runDeleteRemote реализует `jtpost delete <id> --remote ...` через apiclient.
func runDeleteRemote(ctx context.Context, cli *apiclient.ClientWithResponses, idArg string, out io.Writer) error {
	id, err := uuid.Parse(idArg)
	if err != nil {
		return fmt.Errorf("--remote requires UUID id: %w", err)
	}
	resp, err := cli.DeletePostWithResponse(ctx, id)
	if err != nil {
		return fmt.Errorf("remote API call failed: %w", err)
	}
	switch resp.StatusCode() {
	case http.StatusNoContent, http.StatusOK:
		fmt.Fprintf(out, "✅ Пост удалён: %s\n", id)
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized: invalid or expired --auth token")
	case http.StatusForbidden:
		return fmt.Errorf("forbidden")
	case http.StatusNotFound:
		return fmt.Errorf("post not found: %s", id)
	default:
		return fmt.Errorf("server error: %d", resp.StatusCode())
	}
}
